package auth

import (
	"context"
	_ "embed"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/spf13/viper"
	"github.com/urfave/cli/v3"
	"golang.org/x/oauth2"

	_ "github.com/muncus/mcp-starter/auth/providers/google"
)

//go:embed interstitial.html
var interstitialHTML []byte

var tmpl = template.Must(template.New("interstitial").Parse(string(interstitialHTML)))

type TemplateData struct {
	Success      bool
	Error        bool
	ErrorMessage string
	Scopes       []Scope
}

func renderTemplate(w http.ResponseWriter, data TemplateData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err := tmpl.Execute(w, data)
	if err != nil {
		log.Printf("failed to execute template: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func AuthAction() func(ctx context.Context, cmd *cli.Command) error {
	return func(ctx context.Context, cmd *cli.Command) error {
		providerName := cmd.String("provider")
		if providerName == "" {
			providerName = "google"
		}

		provider, ok := GetProvider(providerName)
		if !ok {
			return fmt.Errorf("unknown OAuth provider: %s", providerName)
		}

		clientIDKey := fmt.Sprintf("oauth.%s.client_id", providerName)
		clientSecretKey := fmt.Sprintf("oauth.%s.client_secret", providerName)

		clientID := viper.GetString(clientIDKey)
		clientSecret := viper.GetString(clientSecretKey)

		// Legacy back-compat for default Google configuration
		if providerName == "google" {
			if clientID == "" {
				clientID = viper.GetString("oauth.client_id")
			}
			if clientSecret == "" {
				clientSecret = viper.GetString("oauth.client_secret")
			}
		}

		if clientID == "" || clientSecret == "" {
			log.Fatalf("OAuth credentials for '%s' must be configured under 'oauth.%s.client_id' and 'oauth.%s.client_secret'.", providerName, providerName, providerName)
		}

		provider.ClientID = clientID
		provider.ClientSecret = clientSecret

		token, err := getTokenFromWeb(&provider)
		if err != nil {
			return fmt.Errorf("failed to get token from web: %w", err)
		}

		tokenKey := fmt.Sprintf("oauth.%s.token", providerName)
		viper.Set(tokenKey, token)

		// Legacy back-compat
		if providerName == "google" {
			viper.Set("oauth.token", token)
		}

		if err := viper.WriteConfig(); err != nil {
			return err
		}

		fmt.Println("\nAuthentication successful. Token saved.")
		return nil
	}
}

func Command() *cli.Command {
	return &cli.Command{
		Name:  "auth",
		Usage: "Authenticate with Google APIs",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "provider",
				Aliases: []string{"p"},
				Usage:   "OAuth provider to authenticate with (e.g. google)",
				Value:   "google",
			},
		},
		Action: AuthAction(),
	}
}

func GetClient() (*http.Client, error) {
	providerName := "google"
	provider, ok := GetProvider(providerName)
	if !ok {
		return nil, fmt.Errorf("default provider 'google' not registered")
	}

	clientID := viper.GetString("oauth.google.client_id")
	clientSecret := viper.GetString("oauth.google.client_secret")
	if clientID == "" {
		clientID = viper.GetString("oauth.client_id")
	}
	if clientSecret == "" {
		clientSecret = viper.GetString("oauth.client_secret")
	}

	config := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     provider.Endpoint,
	}

	var token oauth2.Token
	tokenKey := "oauth.google.token"
	if !viper.IsSet(tokenKey) {
		tokenKey = "oauth.token"
	}

	if err := viper.UnmarshalKey(tokenKey, &token); err != nil {
		return nil, fmt.Errorf("failed to unmarshal oauth.token: %w", err)
	}

	return config.Client(context.Background(), &token), nil
}

func getTokenFromWeb(provider *Provider) (*oauth2.Token, error) {
	type tokenResult struct {
		token *oauth2.Token
		err   error
	}
	resultChan := make(chan tokenResult)

	config := &oauth2.Config{
		ClientID:     provider.ClientID,
		ClientSecret: provider.ClientSecret,
		Endpoint:     provider.Endpoint,
		RedirectURL:  provider.RedirectURL,
	}

	var mu sync.Mutex
	var activeConfig = *config // local copy of config structure

	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		query := r.URL.Query()
		if code := query.Get("code"); code != "" {
			mu.Lock()
			cfg := activeConfig
			mu.Unlock()

			tok, err := cfg.Exchange(r.Context(), code)
			if err != nil {
				renderTemplate(w, TemplateData{
					Error:        true,
					ErrorMessage: fmt.Sprintf("Failed to exchange code for token: %v", err),
					Scopes:       provider.Scopes,
				})
				// Send error back but don't block
				go func() { resultChan <- tokenResult{err: err} }()
				return
			}

			renderTemplate(w, TemplateData{
				Success: true,
				Scopes:  provider.Scopes,
			})
			go func() { resultChan <- tokenResult{token: tok} }()
			return
		}

		if errStr := query.Get("error"); errStr != "" {
			renderTemplate(w, TemplateData{
				Error:        true,
				ErrorMessage: fmt.Sprintf("Authorization error from OAuth provider: %s", errStr),
				Scopes:       provider.Scopes,
			})
			go func() { resultChan <- tokenResult{err: fmt.Errorf("authorization error from OAuth provider: %s", errStr)} }()
			return
		}

		// Otherwise, serve the scope selection form dynamically
		renderTemplate(w, TemplateData{
			Scopes: provider.Scopes,
		})
	})

	mux.HandleFunc("/authorize", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if err := r.ParseForm(); err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		scopes := r.PostForm["scopes"]
		if len(scopes) == 0 {
			renderTemplate(w, TemplateData{
				Error:        true,
				ErrorMessage: "Please select at least one scope to authorize.",
				Scopes:       provider.Scopes,
			})
			return
		}

		mu.Lock()
		activeConfig.Scopes = scopes
		authURL := activeConfig.AuthCodeURL(
			"state-token",
			oauth2.AccessTypeOffline,
			oauth2.SetAuthURLParam("prompt", "consent"),
		)
		mu.Unlock()

		http.Redirect(w, r, authURL, http.StatusSeeOther)
	})

	srv := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Printf("server error: %v", err)
		}
	}()

	fmt.Println("Please open your browser and visit: http://localhost:8080")

	res := <-resultChan

	// Shutdown the server gracefully
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)

	return res.token, res.err
}


