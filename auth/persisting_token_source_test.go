package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

// newTestAccountManager returns an AccountManager whose token endpoint is a
// local server that issues "fresh-<n>" access tokens valid for one hour.
func newTestAccountManager(t *testing.T) (*AccountManager, *int32) {
	t.Helper()

	var issued int32
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&issued, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"access_token":"fresh-%d","token_type":"Bearer","expires_in":3600}`, n)
	}))
	t.Cleanup(tokenServer.Close)

	am := &AccountManager{
		accounts:  make(map[string]*Account),
		configDir: t.TempDir(),
		oauthConfig: &oauth2.Config{
			ClientID:     "id",
			ClientSecret: "secret",
			Endpoint:     oauth2.Endpoint{TokenURL: tokenServer.URL, AuthStyle: oauth2.AuthStyleInParams},
		},
	}
	return am, &issued
}

func newTestAccount(am *AccountManager, token *oauth2.Token) *Account {
	account := &Account{
		Email:     "user@example.com",
		Token:     token,
		TokenFile: filepath.Join(am.configDir, "user_at_example_com.json"),
	}
	account.OAuthClient = &OAuthClient{config: am.oauthConfig, token: token, tokenFile: account.TokenFile}
	account.OAuthClient.httpClient = am.newAccountHTTPClient(context.Background(), account)
	am.accounts[account.Email] = account
	return account
}

func readAccountFile(t *testing.T, path string) Account {
	t.Helper()
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatalf("read account file: %v", err)
	}
	var stored Account
	if err := json.Unmarshal(data, &stored); err != nil {
		t.Fatalf("parse account file: %v", err)
	}
	return stored
}

func newAPIServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(r.Header.Get("Authorization")))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestAccountHTTPClientPersistsRefreshedToken(t *testing.T) {
	am, issued := newTestAccountManager(t)
	expired := &oauth2.Token{
		AccessToken:  "stale",
		RefreshToken: "refresh-me",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(-time.Hour),
	}
	account := newTestAccount(am, expired)
	api := newAPIServer(t)

	resp, err := account.OAuthClient.GetHTTPClient().Get(api.URL)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	_ = resp.Body.Close()

	if got := atomic.LoadInt32(issued); got != 1 {
		t.Fatalf("token endpoint called %d times, want 1", got)
	}
	if account.Token.AccessToken != "fresh-1" || !account.Token.Valid() {
		t.Fatalf("in-memory token not updated: %+v", account.Token)
	}
	if account.OAuthClient.token.AccessToken != "fresh-1" {
		t.Fatalf("OAuthClient token not updated: %q", account.OAuthClient.token.AccessToken)
	}

	stored := readAccountFile(t, account.TokenFile)
	if stored.Email != "user@example.com" {
		t.Errorf("account file lost its email: %q", stored.Email)
	}
	if stored.Token == nil || stored.Token.AccessToken != "fresh-1" {
		t.Fatalf("stored token not updated: %+v", stored.Token)
	}
	if stored.Token.RefreshToken != "refresh-me" {
		t.Errorf("refresh token not kept: %q", stored.Token.RefreshToken)
	}
	if !stored.Token.Expiry.After(time.Now()) {
		t.Errorf("stored expiry not in the future: %v", stored.Token.Expiry)
	}
}

func TestAccountHTTPClientSkipsWriteForUnchangedToken(t *testing.T) {
	am, issued := newTestAccountManager(t)
	valid := &oauth2.Token{
		AccessToken:  "still-good",
		RefreshToken: "refresh-me",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(time.Hour),
	}
	account := newTestAccount(am, valid)
	api := newAPIServer(t)

	for i := 0; i < 3; i++ {
		resp, err := account.OAuthClient.GetHTTPClient().Get(api.URL)
		if err != nil {
			t.Fatalf("request: %v", err)
		}
		_ = resp.Body.Close()
	}

	if got := atomic.LoadInt32(issued); got != 0 {
		t.Fatalf("token endpoint called %d times, want 0", got)
	}
	if _, err := os.Stat(account.TokenFile); !os.IsNotExist(err) {
		t.Fatalf("account file written for an unchanged token (stat err: %v)", err)
	}
}

func TestRefreshTokenRebuildsHTTPClient(t *testing.T) {
	am, issued := newTestAccountManager(t)
	expired := &oauth2.Token{
		AccessToken:  "stale",
		RefreshToken: "refresh-me",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(-time.Hour),
	}
	account := newTestAccount(am, expired)
	api := newAPIServer(t)

	if err := am.RefreshToken(context.Background(), account.Email); err != nil {
		t.Fatalf("RefreshToken: %v", err)
	}

	resp, err := account.OAuthClient.GetHTTPClient().Get(api.URL)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	body := make([]byte, 64)
	n, _ := resp.Body.Read(body)
	_ = resp.Body.Close()

	if got := string(body[:n]); got != "Bearer fresh-1" {
		t.Fatalf("API saw %q, want the refreshed token", got)
	}
	if got := atomic.LoadInt32(issued); got != 1 {
		t.Fatalf("token endpoint called %d times, want 1", got)
	}
	if stored := readAccountFile(t, account.TokenFile); stored.Token.AccessToken != "fresh-1" {
		t.Fatalf("stored token not updated: %q", stored.Token.AccessToken)
	}
}
