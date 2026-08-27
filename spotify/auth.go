package spotify

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/url"
	"net/http"
	"os"
	"path/filepath"
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
	tlsCertFile := utils.GetEnv("SPOTIFY_TLS_CERT_FILE", "./data/spotify-callback-cert.pem")
	tlsKeyFile := utils.GetEnv("SPOTIFY_TLS_KEY_FILE", "./data/spotify-callback-key.pem")
	if err := ensureTLSCertificateFiles(tlsCertFile, tlsKeyFile); err != nil {
		fmt.Println("Failed to prepare TLS certificate files:", err)
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

	func ensureTLSCertificateFiles(certFile, keyFile string) error {
		certExists := fileExists(certFile)
		keyExists := fileExists(keyFile)
		if certExists && keyExists {
			return nil
		}
		return generateSelfSignedCertificate(certFile, keyFile)
	}

	func fileExists(path string) bool {
		_, err := os.Stat(path)
		return err == nil
	}

	func generateSelfSignedCertificate(certFile, keyFile string) error {
		privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return err
		}

		serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
		serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
		if err != nil {
			return err
		}

		template := x509.Certificate{
			SerialNumber: serialNumber,
			Subject: pkix.Name{
				CommonName: "localhost",
			},
			NotBefore:             time.Now().Add(-1 * time.Hour),
			NotAfter:              time.Now().Add(365 * 24 * time.Hour),
			KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
			ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
			BasicConstraintsValid: true,
			DNSNames:              []string{"localhost"},
			IPAddresses:           []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
		}

		derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
		if err != nil {
			return err
		}

		if err := os.MkdirAll(filepath.Dir(certFile), 0o755); err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(keyFile), 0o755); err != nil {
			return err
		}

		certOut, err := os.OpenFile(certFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
		if err != nil {
			return err
		}
		defer certOut.Close()

		if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
			return err
		}

		keyOut, err := os.OpenFile(keyFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
		if err != nil {
			return err
		}
		defer keyOut.Close()

		privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
		if err := pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: privateKeyBytes}); err != nil {
			return err
		}

		utils.Logger.Infof("Generated self-signed TLS certificate for Spotify callback server: cert=%s key=%s", certFile, keyFile)
		return nil
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
