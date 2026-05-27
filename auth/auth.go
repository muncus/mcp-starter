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
	"golang.org/x/oauth2/google"
	analyticsdata "google.golang.org/api/analyticsdata/v1alpha"
	"google.golang.org/api/sheets/v4"
)

//go:embed interstitial.html
var interstitialHTML []byte

var tmpl = template.Must(template.New("interstitial").Parse(string(interstitialHTML)))

type TemplateData struct {
	Success      bool
	Error        bool
	ErrorMessage string
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
		config := &oauth2.Config{
			ClientID:     viper.GetString("oauth.client_id"),
			ClientSecret: viper.GetString("oauth.client_secret"),
			Endpoint:     google.Endpoint,
			RedirectURL:  "http://localhost:8080",
		}
		if config.ClientID == "" || config.ClientSecret == "" {
			log.Fatalf("oauth.client_id and oauth.client_secret must be set in config file.")
		}

		token, err := getTokenFromWeb(config)
		if err != nil {
			return fmt.Errorf("failed to get token from web: %w", err)
		}
		viper.Set("oauth.token", token)

		if err := viper.WriteConfig(); err != nil {
			return err
		}

		fmt.Println("\nAuthentication successful. Token saved.")
		return nil
	}
}

func Command() *cli.Command {
	return &cli.Command{
		Name:   "auth",
		Usage:  "Authenticate with Google APIs",
		Action: AuthAction(),
	}
}

func GetClient() (*http.Client, error) {
	config := &oauth2.Config{
		ClientID:     viper.GetString("oauth.client_id"),
		ClientSecret: viper.GetString("oauth.client_secret"),
		Endpoint:     google.Endpoint,
		Scopes:       []string{sheets.SpreadsheetsScope, analyticsdata.AnalyticsReadonlyScope, sheets.DriveReadonlyScope},
	}

	var token oauth2.Token
	if err := viper.UnmarshalKey("oauth.token", &token); err != nil {
		return nil, fmt.Errorf("failed to unmarshal oauth.token: %w", err)
	}

	return config.Client(context.Background(), &token), nil
}

func getTokenFromWeb(config *oauth2.Config) (*oauth2.Token, error) {
	type tokenResult struct {
		token *oauth2.Token
		err   error
	}
	resultChan := make(chan tokenResult)

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
				})
				// Send error back but don't block
				go func() { resultChan <- tokenResult{err: err} }()
				return
			}

			renderTemplate(w, TemplateData{
				Success: true,
			})
			go func() { resultChan <- tokenResult{token: tok} }()
			return
		}

		if errStr := query.Get("error"); errStr != "" {
			renderTemplate(w, TemplateData{
				Error:        true,
				ErrorMessage: fmt.Sprintf("Authorization error from Google: %s", errStr),
			})
			go func() { resultChan <- tokenResult{err: fmt.Errorf("authorization error from Google: %s", errStr)} }()
			return
		}

		// Otherwise, serve the scope selection form
		renderTemplate(w, TemplateData{})
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

