package spotify

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"net/http"
	"os"
	"radio-to-spotify/utils"
	"time"

	"github.com/zmb3/spotify/v2"
	spotifyauth "github.com/zmb3/spotify/v2/auth"
	"golang.org/x/oauth2"
)

var (
	authenticator *spotifyauth.Authenticator
	tokenFile     = "./data/.token"
	token         *oauth2.Token
)

// initializeAuthenticator initializes the Spotify authenticator
func initializeAuthenticator() {
	clientID := utils.GetEnv("SPOTIFY_ID", "")
	clientSecret := utils.GetEnv("SPOTIFY_SECRET", "")
	redirectURL := utils.GetEnv("SPOTIFY_REDIRECT_URL", "https://localhost:8080/callback")

	if clientID == "" || clientSecret == "" {
		fmt.Println("Please set SPOTIFY_ID and SPOTIFY_SECRET environment variables")
		os.Exit(1)
	}

	parsedRedirectURL, err := url.Parse(redirectURL)
	if err != nil || parsedRedirectURL.Scheme != "https" {
		fmt.Println("SPOTIFY_REDIRECT_URL must be a valid HTTPS URL")
		os.Exit(1)
	}

	utils.Logger.Debugf("Initializing Spotify authenticator with client ID: %s and redirect URL: %s", clientID, redirectURL)

	authenticator = spotifyauth.New(
		spotifyauth.WithClientID(clientID),
		spotifyauth.WithClientSecret(clientSecret),
		spotifyauth.WithRedirectURL(redirectURL),
		spotifyauth.WithScopes(
			spotifyauth.ScopeUserReadPrivate,
			spotifyauth.ScopePlaylistModifyPublic,
			spotifyauth.ScopePlaylistModifyPrivate,
		),
	)
}

func getAuthToken() (*oauth2.Token, error) {
	defer saveTokenToFile(tokenFile, token) // Save token to file when function exits
	initializeAuthenticator()               // Initialize authenticator

	token, err := loadTokenFromFile(tokenFile)
	if err != nil {
		utils.Logger.Debug("Error loading token: ", err)
	}
	token, err = authenticator.RefreshToken(context.Background(), token)
	if err == nil && token.Valid() {
		utils.Logger.Debug("Using existing token")
		return token, nil
	}
	if err != nil {
		utils.Logger.Debug("Error refreshing token: ", err)
	}

	http.HandleFunc("/callback", completeAuth)
	serverAddress := ":" + utils.GetEnv("SPOTIFY_PORT", "8999")
	tlsCertFile := utils.GetEnv("SPOTIFY_TLS_CERT_FILE", "")
	tlsKeyFile := utils.GetEnv("SPOTIFY_TLS_KEY_FILE", "")
	if tlsCertFile == "" || tlsKeyFile == "" {
		fmt.Println("Please set SPOTIFY_TLS_CERT_FILE and SPOTIFY_TLS_KEY_FILE environment variables")
		os.Exit(1)
	}
	go func() {
		if err := http.ListenAndServeTLS(serverAddress, tlsCertFile, tlsKeyFile, nil); err != nil {
			utils.Logger.Error("Error starting Spotify callback HTTPS server: ", err)
		}
	}()

	url := authenticator.AuthURL("state-token")
	fmt.Println("Please log in to Spotify by visiting the following page in your browser:", url)

	for {
		time.Sleep(100 * time.Millisecond)
		token, err := loadTokenFromFile(tokenFile)
		if err == nil && token.Valid() {
			return token, nil
		}
	}
}

func completeAuth(w http.ResponseWriter, r *http.Request) {
	utils.Logger.Debugf("Callback received: %s", r.URL.String())
	tok, err := authenticator.Token(context.Background(), "state-token", r)
	if err != nil {
		http.Error(w, "Couldn't get token ", http.StatusForbidden)
		utils.Logger.Error(err)
		return
	}
	if st := r.FormValue("state"); st != "state-token" {
		http.NotFound(w, r)
		utils.Logger.Errorf("State mismatch: %s != %s\n", st, "state-token")
		return
	}

	client := spotify.New(authenticator.Client(context.Background(), tok))
	_, err = client.CurrentUser(context.Background())
	if err != nil {
		http.Error(w, "Couldn't get user", http.StatusForbidden)
		utils.Logger.Error(err)
		return
	}
	utils.Logger.Debug("Login Completed!")
	saveTokenToFile(tokenFile, tok)

	fmt.Fprintf(w, `<script>window.close();</script>`)
}

func saveTokenToFile(path string, token *oauth2.Token) error {
	utils.Logger.Debugf("Saving token to file: %s", path)
	defer utils.Logger.Debug("Token saved to file: ", path)
	if token == nil {
		return nil
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	return json.NewEncoder(file).Encode(token)
}

func loadTokenFromFile(path string) (*oauth2.Token, error) {
	utils.Logger.Debugf("Loading token from file: %s", path)
	defer utils.Logger.Debug("Token loaded from file: ", path)
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	defer file.Close()

	var token oauth2.Token
	err = json.NewDecoder(file).Decode(&token)
	if err != nil {
		return nil, err
	}

	return &token, nil
}

func getClient() (*spotify.Client, error) {
	token, err := getAuthToken()
	if err != nil {
		return nil, err
	}

	client := spotify.New(authenticator.Client(context.Background(), token), spotify.WithRetry(true))
	return client, nil
}

func (s *SpotifyService) UpdateSession() error {
	_, err := s.client.CurrentUser(context.Background())
	return err
}

func (s *SpotifyService) CheckHealth() (bool, string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := s.client.CurrentUser(ctx)
	if err != nil {
		return false, "Spotify service is unavailable"
	}
	return true, "Spotify service is working"
}
