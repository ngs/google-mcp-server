package auth

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"golang.org/x/oauth2"
)

// OAuthCallbackServer handles OAuth callbacks
type OAuthCallbackServer struct {
	config       *oauth2.Config
	server       *http.Server
	authCodeChan chan string
	errorChan    chan error
	port         int
}

// NewOAuthCallbackServer creates a new OAuth callback server
func NewOAuthCallbackServer(config *oauth2.Config) *OAuthCallbackServer {
	return &OAuthCallbackServer{
		config:       config,
		authCodeChan: make(chan string, 1),
		errorChan:    make(chan error, 1),
		port:         8080,
	}
}

// StartAndWaitForCallback starts the server and waits for OAuth callback
func (s *OAuthCallbackServer) StartAndWaitForCallback(ctx context.Context) (*oauth2.Token, error) {
	// Find available port
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", s.port))
	if err != nil {
		// Try random port
		listener, err = net.Listen("tcp", ":0")
		if err != nil {
			return nil, fmt.Errorf("failed to start callback server: %w", err)
		}
		s.port = listener.Addr().(*net.TCPAddr).Port
	}

	// Update redirect URI with actual port
	s.config.RedirectURL = fmt.Sprintf("http://localhost:%d/callback", s.port)

	// Setup HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", s.handleCallback)
	mux.HandleFunc("/", s.handleRoot)

	s.server = &http.Server{
		Handler: mux,
	}

	// Start server in background
	go func() {
		if err := s.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			s.errorChan <- err
		}
	}()

	// Generate auth URL
	authURL := s.config.AuthCodeURL("state", oauth2.AccessTypeOffline)

	// Log to stderr to be visible in MCP context
	fmt.Fprintf(os.Stderr, "\n=== OAuth Authentication Required ===\n")
	fmt.Fprintf(os.Stderr, "Please visit this URL to authenticate:\n%s\n\n", authURL)
	fmt.Fprintf(os.Stderr, "Waiting for authentication (timeout: 5 minutes)...\n")

	// Wait for callback or timeout
	select {
	case authCode := <-s.authCodeChan:
		// Exchange code for token
		token, err := s.config.Exchange(ctx, authCode)
		if err != nil {
			return nil, fmt.Errorf("failed to exchange auth code: %w", err)
		}

		// Shutdown server
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.server.Shutdown(shutdownCtx)

		return token, nil

	case err := <-s.errorChan:
		return nil, err

	case <-ctx.Done():
		// Shutdown server
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.server.Shutdown(shutdownCtx)

		return nil, ctx.Err()

	case <-time.After(5 * time.Minute):
		// Shutdown server
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.server.Shutdown(shutdownCtx)

		return nil, fmt.Errorf("authentication timeout")
	}
}

// handleCallback handles the OAuth callback
func (s *OAuthCallbackServer) handleCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		errMsg := r.URL.Query().Get("error")
		if errMsg == "" {
			errMsg = "no authorization code received"
		}

		w.WriteHeader(http.StatusBadRequest)
		_, _ = fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
    <title>Authentication Failed</title>
    <style>
        body { font-family: Arial, sans-serif; text-align: center; padding: 50px; }
        .error { color: #d9534f; }
    </style>
</head>
<body>
    <h1 class="error">Authentication Failed</h1>
    <p>Error: %s</p>
    <p>Please close this window and try again.</p>
</body>
</html>`, errMsg)

		s.errorChan <- fmt.Errorf("OAuth error: %s", errMsg)
		return
	}

	// Send success response
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
    <title>Authentication Successful</title>
    <style>
        body { font-family: Arial, sans-serif; text-align: center; padding: 50px; }
        .success { color: #5cb85c; }
    </style>
</head>
<body>
    <h1 class="success">Authentication Successful!</h1>
    <p>You can now close this window and return to the application.</p>
    <script>
        setTimeout(function() {
            window.close();
        }, 3000);
    </script>
</body>
</html>`)

	// Send auth code to channel
	s.authCodeChan <- code
}

// handleRoot handles the root path
func (s *OAuthCallbackServer) handleRoot(w http.ResponseWriter, r *http.Request) {
	// Generate auth URL
	authURL := s.config.AuthCodeURL("state", oauth2.AccessTypeOffline)

	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
    <title>Google MCP Server - OAuth</title>
    <style>
        body { font-family: Arial, sans-serif; text-align: center; padding: 50px; }
        .button { 
            background-color: #4285f4; 
            color: white; 
            padding: 12px 24px; 
            text-decoration: none; 
            border-radius: 4px;
            display: inline-block;
            margin-top: 20px;
        }
    </style>
</head>
<body>
    <h1>Google MCP Server Authentication</h1>
    <p>Click the button below to authenticate with Google:</p>
    <a href="%s" class="button">Authenticate with Google</a>
</body>
</html>`, authURL)
}

// GetAuthURL returns the OAuth authorization URL
func (s *OAuthCallbackServer) GetAuthURL() string {
	return s.config.AuthCodeURL("state", oauth2.AccessTypeOffline)
}

// GetCallbackURL returns the callback URL
func (s *OAuthCallbackServer) GetCallbackURL() string {
	return fmt.Sprintf("http://localhost:%d/callback", s.port)
}
