package auth

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/spf13/viper"
	"github.com/urfave/cli/v3"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	analyticsdata "google.golang.org/api/analyticsdata/v1alpha"
	"google.golang.org/api/sheets/v4"
)

func AuthAction() func(ctx context.Context, cmd *cli.Command) error {
	return func(ctx context.Context, cmd *cli.Command) error {
		config := &oauth2.Config{
			ClientID:     viper.GetString("oauth.client_id"),
			ClientSecret: viper.GetString("oauth.client_secret"),
			Endpoint:     google.Endpoint,
			RedirectURL:  "http://localhost:8080",
			Scopes:       []string{sheets.SpreadsheetsScope, analyticsdata.AnalyticsReadonlyScope, sheets.DriveReadonlyScope, "https://www.googleapis.com/auth/cloud-platform"},
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

		fmt.Println("Authentication successful. Token saved.")
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
		Endpoint: google.Endpoint,
		Scopes:   []string{sheets.SpreadsheetsScope, analyticsdata.AnalyticsReadonlyScope, sheets.DriveReadonlyScope},
	}

	var token oauth2.Token
	viper.GetString("oauth.token")

	return config.Client(context.Background(), &token), nil
}

func getTokenFromWeb(config *oauth2.Config) (*oauth2.Token, error) {
	authCodeChan := make(chan string)
	srv := &http.Server{Addr: ":8080"}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		authCodeChan <- code
		fmt.Fprintf(w, "Authentication successful! You can close this window.")
	})

	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("ListenAndServe(): %v", err)
		}
	}()

	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser to authorize the application: \n%v\n", authURL)

	authCode := <-authCodeChan

	if err := srv.Shutdown(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to shutdown server: %w", err)
	}

	tok, err := config.Exchange(context.Background(), authCode)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve token from web: %v", err)
	}
	return tok, nil
}
