package auth

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"

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
