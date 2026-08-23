package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

func TestDefaultScopes(t *testing.T) {
	scopes := DefaultScopes()

	if len(scopes) == 0 {
		t.Error("DefaultScopes returned empty slice")
	}

	expectedScopes := []string{
		"https://www.googleapis.com/auth/calendar",
		"https://www.googleapis.com/auth/drive",
		"https://www.googleapis.com/auth/gmail.modify",
		"https://www.googleapis.com/auth/spreadsheets",
		"https://www.googleapis.com/auth/documents",
		"https://www.googleapis.com/auth/presentations",
		"https://www.googleapis.com/auth/userinfo.email",
		"https://www.googleapis.com/auth/userinfo.profile",
	}

	if len(scopes) != len(expectedScopes) {
		t.Errorf("Expected %d scopes, got %d", len(expectedScopes), len(scopes))
	}

	// Check each scope
	scopeMap := make(map[string]bool)
	for _, scope := range scopes {
		scopeMap[scope] = true
	}

	for _, expected := range expectedScopes {
		if !scopeMap[expected] {
			t.Errorf("Missing expected scope: %s", expected)
		}
	}
}

func TestOAuthConfig(t *testing.T) {
	config := OAuthConfig{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		RedirectURI:  "http://localhost:8080/callback",
		TokenFile:    filepath.Join(os.TempDir(), "test-token.json"),
		Scopes:       DefaultScopes(),
	}

	if config.ClientID != "test-client-id" {
		t.Errorf("Expected ClientID to be 'test-client-id', got %s", config.ClientID)
	}

	if config.ClientSecret != "test-client-secret" {
		t.Errorf("Expected ClientSecret to be 'test-client-secret', got %s", config.ClientSecret)
	}
}

func TestNewOAuthClientWithoutAuth(t *testing.T) {
	// Skip this test on Windows or CI environments to avoid OAuth flow
	if os.Getenv("CI") != "" {
		t.Skip("Skipping OAuth test in CI environment")
	}

	ctx := context.Background()

	// Test with missing credentials (should fail gracefully)
	config := OAuthConfig{
		ClientID:     "",
		ClientSecret: "",
	}

	_, err := NewOAuthClient(ctx, config)
	if err == nil {
		t.Error("Expected error with empty credentials")
	}
}

// writeTestToken writes a token file that loadToken accepts and returns its path.
func writeTestToken(t *testing.T, dir string) string {
	t.Helper()

	path := filepath.Join(dir, "token.json")
	token := &oauth2.Token{
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(time.Hour),
	}

	data, err := json.Marshal(token)
	if err != nil {
		t.Fatalf("Failed to marshal token: %v", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("Failed to write token file: %v", err)
	}

	return path
}

// TestOAuthClientConstructors covers the interactive/non-interactive split.
// Only cases with a stored token may use the interactive constructor: a missing
// token there would open a browser and block.
func TestOAuthClientConstructors(t *testing.T) {
	tests := []struct {
		name        string
		interactive bool
		withToken   bool
		malformed   bool
		wantErr     bool
		errContains string
	}{
		{name: "non-interactive with token", interactive: false, withToken: true},
		{name: "non-interactive without token", interactive: false, withToken: false, wantErr: true, errContains: "no stored token"},
		{name: "non-interactive with malformed token", interactive: false, malformed: true, wantErr: true, errContains: "failed to load stored token"},
		{name: "interactive with token", interactive: true, withToken: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			tokenFile := filepath.Join(tempDir, "token.json")
			if tt.withToken {
				tokenFile = writeTestToken(t, tempDir)
			}
			if tt.malformed {
				if err := os.WriteFile(tokenFile, []byte("{not json"), 0600); err != nil {
					t.Fatalf("Failed to write malformed token file: %v", err)
				}
			}

			config := OAuthConfig{
				ClientID:     "test-client-id",
				ClientSecret: "test-client-secret",
				TokenFile:    tokenFile,
			}

			ctx := context.Background()
			var (
				client *OAuthClient
				err    error
			)
			if tt.interactive {
				client, err = NewOAuthClient(ctx, config)
			} else {
				client, err = NewOAuthClientNonInteractive(ctx, config)
			}

			if tt.wantErr {
				if err == nil {
					t.Fatal("Expected an error, got nil")
				}
				if !strings.Contains(err.Error(), tokenFile) {
					t.Errorf("Expected the error to mention the token file %s, got %v", tokenFile, err)
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("Expected the error to contain %q, got %v", tt.errContains, err)
				}
				if client != nil {
					t.Error("Expected a nil client alongside the error")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if client == nil {
				t.Fatal("Expected a client, got nil")
			}
			if client.GetHTTPClient() == nil {
				t.Error("Expected an authenticated HTTP client")
			}
		})
	}
}

// TestNewOAuthClientNonInteractiveDoesNotBlock guards the headless startup path:
// a missing token must fail immediately rather than wait on a browser callback.
func TestNewOAuthClientNonInteractiveDoesNotBlock(t *testing.T) {
	config := OAuthConfig{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		TokenFile:    filepath.Join(t.TempDir(), "missing-token.json"),
	}

	done := make(chan error, 1)
	go func() {
		_, err := NewOAuthClientNonInteractive(context.Background(), config)
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("Expected an error for a missing token file")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("NewOAuthClientNonInteractive blocked with no stored token")
	}
}

func TestRedirectURIPort(t *testing.T) {
	tests := []struct {
		name        string
		redirectURI string
		want        int
	}{
		{"explicit port", "http://localhost:51011/callback", 51011},
		{"default port", "http://localhost:8080/callback", 8080},
		{"no port", "http://localhost/callback", defaultCallbackPort},
		{"empty", "", defaultCallbackPort},
		{"not a URL", "://nope", defaultCallbackPort},
		{"out of range", "http://localhost:99999/callback", defaultCallbackPort},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := redirectURIPort(tt.redirectURI); got != tt.want {
				t.Errorf("redirectURIPort(%q) = %d, want %d", tt.redirectURI, got, tt.want)
			}
		})
	}
}

func TestStartCallbackListenerUsesPreferredPort(t *testing.T) {
	preferredPort := freePort(t)

	listener, err := startCallbackListener(preferredPort)
	if err != nil {
		t.Fatalf("startCallbackListener failed: %v", err)
	}
	defer func() { _ = listener.Close() }()

	addr := listener.Addr().(*net.TCPAddr)
	if addr.Port != preferredPort {
		t.Errorf("Expected port %d, got %d", preferredPort, addr.Port)
	}
	if !addr.IP.Equal(net.IPv4(127, 0, 0, 1)) {
		t.Errorf("Expected listener bound to 127.0.0.1, got %s", addr.IP)
	}
}

// TestAuthenticatePortFallback verifies that a busy preferred port makes the
// callback fall back to an ephemeral port, and that the authorization URL
// advertises the port we actually listen on.
func TestAuthenticatePortFallback(t *testing.T) {
	busyPort := freePort(t)
	blocker, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", busyPort))
	if err != nil {
		t.Fatalf("Failed to occupy port %d: %v", busyPort, err)
	}
	defer func() { _ = blocker.Close() }()

	client := &OAuthClient{
		config: &oauth2.Config{
			ClientID:     "test-id",
			ClientSecret: "test-secret",
			RedirectURL:  fmt.Sprintf("http://localhost:%d/callback", busyPort),
			Scopes:       DefaultScopes(),
			Endpoint:     google.Endpoint,
		},
	}

	listener, authURL, err := client.prepareCallback()
	if err != nil {
		t.Fatalf("prepareCallback failed: %v", err)
	}
	defer func() { _ = listener.Close() }()

	addr := listener.Addr().(*net.TCPAddr)
	if addr.Port == busyPort {
		t.Fatalf("Expected fallback to a different port, got the busy port %d", busyPort)
	}
	if !addr.IP.Equal(net.IPv4(127, 0, 0, 1)) {
		t.Errorf("Expected listener bound to 127.0.0.1, got %s", addr.IP)
	}

	// The authorization URL must point at the port we actually listen on
	parsed, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("Failed to parse authorization URL: %v", err)
	}
	redirectURI, err := url.Parse(parsed.Query().Get("redirect_uri"))
	if err != nil {
		t.Fatalf("Failed to parse redirect_uri: %v", err)
	}
	if redirectURI.Port() != strconv.Itoa(addr.Port) {
		t.Errorf("redirect_uri port is %s, want %d", redirectURI.Port(), addr.Port)
	}
	if redirectURI.Hostname() != addr.IP.String() {
		t.Errorf("redirect_uri host is %s, want the bound address %s", redirectURI.Hostname(), addr.IP)
	}

	// The fallback port must actually be reachable
	conn, err := net.Dial("tcp", addr.String())
	if err != nil {
		t.Fatalf("Failed to connect to callback listener: %v", err)
	}
	_ = conn.Close()
}

// freePort returns a port that is free at the moment it is called.
func freePort(t *testing.T) int {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to find a free port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatalf("Failed to release port %d: %v", port, err)
	}

	return port
}

func TestNilOAuthClientAccessors(t *testing.T) {
	var client *OAuthClient

	if httpClient := client.GetHTTPClient(); httpClient != nil {
		t.Errorf("Expected nil HTTP client from nil receiver, got %v", httpClient)
	}

	if opt := client.GetClientOption(); opt == nil {
		t.Error("Expected a client option from nil receiver, got nil")
	}
}

func TestTokenFilePath(t *testing.T) {
	tempDir := t.TempDir()
	tokenFile := filepath.Join(tempDir, "test-token.json")

	config := OAuthConfig{
		ClientID:     "test-id",
		ClientSecret: "test-secret",
		TokenFile:    tokenFile,
	}

	// Verify the path is correctly set
	if config.TokenFile != tokenFile {
		t.Errorf("Expected token file path %s, got %s", tokenFile, config.TokenFile)
	}

	// Verify the directory exists or can be created
	dir := filepath.Dir(config.TokenFile)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Errorf("Token file directory does not exist: %s", dir)
	}
}
