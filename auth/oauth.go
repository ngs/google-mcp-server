package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/pkg/browser"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
)

// OAuthClient manages OAuth2 authentication for Google APIs
type OAuthClient struct {
	config       *oauth2.Config
	token        *oauth2.Token
	tokenFile    string
	httpClient   *http.Client
	mu           sync.RWMutex
	refreshTimer *time.Timer
}

// OAuthConfig holds OAuth configuration
type OAuthConfig struct {
	ClientID     string   `json:"client_id"`
	ClientSecret string   `json:"client_secret"`
	RedirectURI  string   `json:"redirect_uri"`
	TokenFile    string   `json:"token_file"`
	Scopes       []string `json:"scopes"`
}

// NewOAuthClient creates a new OAuth client
func NewOAuthClient(ctx context.Context, config OAuthConfig) (*OAuthClient, error) {
	if config.ClientID == "" || config.ClientSecret == "" {
		return nil, fmt.Errorf("client ID and client secret are required")
	}

	// Set default scopes if not provided
	if len(config.Scopes) == 0 {
		config.Scopes = DefaultScopes()
	}

	// Set default redirect URI if not provided
	if config.RedirectURI == "" {
		config.RedirectURI = "http://localhost:8080/callback"
	}

	// Set default token file if not provided
	if config.TokenFile == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		config.TokenFile = filepath.Join(homeDir, ".google-mcp-token.json")
	}

	oauthConfig := &oauth2.Config{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		RedirectURL:  config.RedirectURI,
		Scopes:       config.Scopes,
		Endpoint:     google.Endpoint,
	}

	client := &OAuthClient{
		config:    oauthConfig,
		tokenFile: config.TokenFile,
	}

	// Try to load existing token
	if err := client.loadToken(); err == nil && client.token != nil {
		// Token loaded successfully, create HTTP client
		client.httpClient = oauthConfig.Client(ctx, client.token)
		client.startTokenRefresh(ctx)
		return client, nil
	}

	// No valid token, need to authenticate
	if err := client.authenticate(ctx); err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	client.startTokenRefresh(ctx)
	return client, nil
}

// DefaultScopes returns the default set of OAuth scopes
func DefaultScopes() []string {
	return []string{
		"https://www.googleapis.com/auth/calendar",
		"https://www.googleapis.com/auth/drive",
		"https://www.googleapis.com/auth/gmail.modify",
		"https://www.googleapis.com/auth/spreadsheets",
		"https://www.googleapis.com/auth/documents",
		"https://www.googleapis.com/auth/userinfo.email",
		"https://www.googleapis.com/auth/userinfo.profile",
	}
}

// GetHTTPClient returns the authenticated HTTP client
func (c *OAuthClient) GetHTTPClient() *http.Client {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.httpClient
}

// GetClientOption returns the Google API client option
func (c *OAuthClient) GetClientOption() option.ClientOption {
	return option.WithHTTPClient(c.GetHTTPClient())
}

// authenticate performs the OAuth2 authentication flow
func (c *OAuthClient) authenticate(ctx context.Context) error {
	// Generate authorization URL
	authURL := c.config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)

	fmt.Printf("Opening browser for authentication...\n")
	fmt.Printf("If browser doesn't open, visit this URL:\n%s\n", authURL)

	// Open browser
	if err := browser.OpenURL(authURL); err != nil {
		fmt.Printf("Failed to open browser: %v\n", err)
	}

	// Start local server to handle callback
	codeChan := make(chan string, 1)
	errChan := make(chan error, 1)

	server := &http.Server{
		Addr: ":8080",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			code := r.URL.Query().Get("code")
			if code == "" {
				errChan <- fmt.Errorf("no authorization code received")
				http.Error(w, "No authorization code received", http.StatusBadRequest)
				return
			}

			codeChan <- code
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprintf(w, `<html><body>
				<h1>Authentication successful!</h1>
				<p>You can close this window and return to the terminal.</p>
				<script>window.close()</script>
			</body></html>`)
		}),
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// Wait for authorization code or error
	var code string
	select {
	case code = <-codeChan:
		// Success
	case err := <-errChan:
		return err
	case <-time.After(5 * time.Minute):
		return fmt.Errorf("authentication timeout")
	}

	// Shut down the server
	if err := server.Shutdown(ctx); err != nil {
		fmt.Printf("Warning: failed to shutdown callback server: %v\n", err)
	}

	// Exchange authorization code for token
	token, err := c.config.Exchange(ctx, code)
	if err != nil {
		return fmt.Errorf("failed to exchange authorization code: %w", err)
	}

	c.mu.Lock()
	c.token = token
	c.httpClient = c.config.Client(ctx, token)
	c.mu.Unlock()

	// Save token for future use
	if err := c.saveToken(); err != nil {
		fmt.Printf("Warning: failed to save token: %v\n", err)
	}

	fmt.Println("Authentication successful!")
	return nil
}

// loadToken loads the OAuth token from file
func (c *OAuthClient) loadToken() error {
	file, err := os.Open(c.tokenFile)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	token := &oauth2.Token{}
	if err := json.NewDecoder(file).Decode(token); err != nil {
		return err
	}

	// Check if token is expired
	if token.Expiry.Before(time.Now()) && token.RefreshToken == "" {
		return fmt.Errorf("token expired and no refresh token available")
	}

	c.mu.Lock()
	c.token = token
	c.mu.Unlock()

	return nil
}

// saveToken saves the OAuth token to file
func (c *OAuthClient) saveToken() error {
	c.mu.RLock()
	token := c.token
	c.mu.RUnlock()

	if token == nil {
		return fmt.Errorf("no token to save")
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(c.tokenFile)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create token directory: %w", err)
	}

	file, err := os.OpenFile(c.tokenFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create token file: %w", err)
	}
	defer func() { _ = file.Close() }()

	if err := json.NewEncoder(file).Encode(token); err != nil {
		return fmt.Errorf("failed to encode token: %w", err)
	}

	return nil
}

// startTokenRefresh starts automatic token refresh
func (c *OAuthClient) startTokenRefresh(ctx context.Context) {
	c.mu.Lock()

	if c.refreshTimer != nil {
		c.refreshTimer.Stop()
	}

	if c.token == nil || c.token.RefreshToken == "" {
		c.mu.Unlock()
		return
	}

	// Calculate time until token expires
	timeUntilExpiry := time.Until(c.token.Expiry)
	if timeUntilExpiry <= 0 {
		c.mu.Unlock()
		// Token already expired, refresh immediately
		go c.refreshToken(ctx)
		return
	}

	// Refresh token 5 minutes before expiry
	refreshTime := timeUntilExpiry - 5*time.Minute
	if refreshTime <= 0 {
		refreshTime = 1 * time.Second
	}

	c.refreshTimer = time.AfterFunc(refreshTime, func() {
		c.refreshToken(ctx)
	})
	c.mu.Unlock()
}

// refreshToken refreshes the OAuth token
func (c *OAuthClient) refreshToken(ctx context.Context) {
	c.mu.RLock()
	currentToken := c.token
	c.mu.RUnlock()

	if currentToken == nil || currentToken.RefreshToken == "" {
		return
	}

	tokenSource := c.config.TokenSource(ctx, currentToken)
	newToken, err := tokenSource.Token()
	if err != nil {
		fmt.Printf("Warning: failed to refresh token: %v\n", err)
		return
	}

	c.mu.Lock()
	c.token = newToken
	c.mu.Unlock()

	// Save the new token
	if err := c.saveToken(); err != nil {
		fmt.Printf("Warning: failed to save refreshed token: %v\n", err)
	}

	c.mu.Lock()

	// Schedule next refresh
	// Check if token is already expired
	timeUntilExpiry := time.Until(c.token.Expiry)
	if timeUntilExpiry <= 0 {
		// Token already expired again, refresh immediately
		c.mu.Unlock()
		go func() {
			// Small delay to ensure current function completes
			time.Sleep(100 * time.Millisecond)
			c.refreshToken(ctx)
		}()
		return
	}

	// Set up timer for next refresh (while still holding the lock)
	if c.refreshTimer != nil {
		c.refreshTimer.Stop()
	}

	refreshTime := timeUntilExpiry - 5*time.Minute
	if refreshTime <= 0 {
		refreshTime = 1 * time.Second
	}

	c.refreshTimer = time.AfterFunc(refreshTime, func() {
		c.refreshToken(ctx)
	})
	c.mu.Unlock()
}

// Revoke revokes the current OAuth token
func (c *OAuthClient) Revoke(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.token == nil {
		return nil
	}

	// Revoke token via Google API
	req, err := http.NewRequestWithContext(ctx, "POST",
		fmt.Sprintf("https://oauth2.googleapis.com/revoke?token=%s", c.token.AccessToken),
		nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	// Remove local token file
	if err := os.Remove(c.tokenFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove token file: %w", err)
	}

	c.token = nil
	c.httpClient = nil

	return nil
}
