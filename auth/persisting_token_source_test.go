package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
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

func readAccountFile(t *testing.T, path string) *Account {
	t.Helper()
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatalf("read account file: %v", err)
	}
	stored := &Account{}
	if err := json.Unmarshal(data, stored); err != nil {
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
	if tok := account.CurrentToken(); tok.AccessToken != "fresh-1" || !tok.Valid() {
		t.Fatalf("in-memory token not updated: %+v", tok)
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

func getAuthorization(t *testing.T, client *http.Client, url string) string {
	t.Helper()
	resp, err := client.Get(url)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(body)
}

func TestRefreshTokenUpdatesExistingHTTPClient(t *testing.T) {
	am, issued := newTestAccountManager(t)
	expired := &oauth2.Token{
		AccessToken:  "stale",
		RefreshToken: "refresh-me",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(-time.Hour),
	}
	account := newTestAccount(am, expired)
	api := newAPIServer(t)

	// A service built before RefreshToken keeps this client.
	cached := account.OAuthClient.GetHTTPClient()

	if err := am.RefreshToken(context.Background(), account.Email); err != nil {
		t.Fatalf("RefreshToken: %v", err)
	}

	if got := getAuthorization(t, cached, api.URL); got != "Bearer fresh-1" {
		t.Fatalf("cached client sent %q, want the refreshed token", got)
	}
	if got := getAuthorization(t, account.OAuthClient.GetHTTPClient(), api.URL); got != "Bearer fresh-1" {
		t.Fatalf("current client sent %q, want the refreshed token", got)
	}
	if got := atomic.LoadInt32(issued); got != 1 {
		t.Fatalf("token endpoint called %d times, want 1", got)
	}
	if stored := readAccountFile(t, account.TokenFile); stored.Token.AccessToken != "fresh-1" {
		t.Fatalf("stored token not updated: %q", stored.Token.AccessToken)
	}
}

func TestRefreshAfterRemoveDoesNotRecreateAccountFile(t *testing.T) {
	am, issued := newTestAccountManager(t)
	expired := &oauth2.Token{
		AccessToken:  "stale",
		RefreshToken: "refresh-me",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(-time.Hour),
	}
	account := newTestAccount(am, expired)
	api := newAPIServer(t)
	cached := account.OAuthClient.GetHTTPClient()

	if err := am.RemoveAccount(account.Email); err != nil {
		t.Fatalf("RemoveAccount: %v", err)
	}

	if got := getAuthorization(t, cached, api.URL); got != "Bearer fresh-1" {
		t.Fatalf("cached client sent %q, want a refreshed token", got)
	}
	if got := atomic.LoadInt32(issued); got != 1 {
		t.Fatalf("token endpoint called %d times, want 1", got)
	}
	if _, err := os.Stat(account.TokenFile); !os.IsNotExist(err) {
		t.Fatalf("removed account file was recreated (stat err: %v)", err)
	}
}

func TestStaleClientDoesNotOverwriteReplacementAccount(t *testing.T) {
	am, _ := newTestAccountManager(t)
	expired := &oauth2.Token{
		AccessToken:  "stale",
		RefreshToken: "refresh-me",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(-time.Hour),
	}
	old := newTestAccount(am, expired)
	api := newAPIServer(t)
	cached := old.OAuthClient.GetHTTPClient()

	// The same email is added again with new credentials.
	replacement := newTestAccount(am, &oauth2.Token{
		AccessToken:  "replacement",
		RefreshToken: "replacement-refresh",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(time.Hour),
	})
	if err := am.saveAccount(replacement); err != nil {
		t.Fatalf("save replacement: %v", err)
	}

	if got := getAuthorization(t, cached, api.URL); got != "Bearer fresh-1" {
		t.Fatalf("cached client sent %q, want a refreshed token", got)
	}

	stored := readAccountFile(t, replacement.TokenFile)
	if stored.Token.AccessToken != "replacement" || stored.Token.RefreshToken != "replacement-refresh" {
		t.Fatalf("replacement credentials overwritten: %+v", stored.Token)
	}
}

func TestConcurrentTokenReadsDuringRefresh(t *testing.T) {
	am, _ := newTestAccountManager(t)
	expired := &oauth2.Token{
		AccessToken:  "stale",
		RefreshToken: "refresh-me",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(-time.Hour),
	}
	account := newTestAccount(am, expired)
	api := newAPIServer(t)
	client := account.OAuthClient.GetHTTPClient()

	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			resp, err := client.Get(api.URL)
			if err != nil {
				t.Errorf("request: %v", err)
				return
			}
			_ = resp.Body.Close()
		}()
		go func() {
			defer wg.Done()
			for _, a := range am.ListAccounts() {
				_ = a.CurrentToken().Valid()
				_ = a.LastUsedAt()
			}
			if _, err := am.GetAccount(account.Email); err != nil {
				t.Errorf("GetAccount: %v", err)
			}
		}()
	}
	if err := am.RefreshToken(context.Background(), account.Email); err != nil {
		t.Fatalf("RefreshToken: %v", err)
	}
	wg.Wait()

	if tok := account.CurrentToken(); tok == nil || !tok.Valid() {
		t.Fatalf("token not valid after concurrent refreshes: %+v", tok)
	}
}

func TestFailedSaveIsRetriedOnNextRequest(t *testing.T) {
	am, issued := newTestAccountManager(t)
	expired := &oauth2.Token{
		AccessToken:  "stale",
		RefreshToken: "refresh-me",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(-time.Hour),
	}
	account := newTestAccount(am, expired)
	// Point the account file into a directory that does not exist yet, so the
	// first save fails.
	dir := filepath.Join(am.configDir, "missing")
	account.TokenFile = filepath.Join(dir, "user_at_example_com.json")
	api := newAPIServer(t)
	client := account.OAuthClient.GetHTTPClient()

	if got := getAuthorization(t, client, api.URL); got != "Bearer fresh-1" {
		t.Fatalf("first request sent %q", got)
	}
	if _, err := os.Stat(account.TokenFile); !os.IsNotExist(err) {
		t.Fatalf("account file unexpectedly written (stat err: %v)", err)
	}

	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if got := getAuthorization(t, client, api.URL); got != "Bearer fresh-1" {
		t.Fatalf("second request sent %q", got)
	}
	if got := atomic.LoadInt32(issued); got != 1 {
		t.Fatalf("token endpoint called %d times, want 1", got)
	}
	if stored := readAccountFile(t, account.TokenFile); stored.Token.AccessToken != "fresh-1" {
		t.Fatalf("stored token not updated after retry: %q", stored.Token.AccessToken)
	}
}

func TestInFlightRefreshUsesReplacedToken(t *testing.T) {
	am, _ := newTestAccountManager(t)

	// The first token request (the client's automatic refresh) blocks until
	// released; later requests (RefreshToken) answer immediately.
	var calls int32
	started := make(chan struct{})
	release := make(chan struct{})
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			close(started)
			<-release
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"access_token":"fresh-%d","token_type":"Bearer","expires_in":3600}`, n)
	}))
	t.Cleanup(tokenServer.Close)
	am.oauthConfig.Endpoint.TokenURL = tokenServer.URL

	expired := &oauth2.Token{
		AccessToken:  "stale",
		RefreshToken: "refresh-me",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(-time.Hour),
	}
	account := newTestAccount(am, expired)
	api := newAPIServer(t)
	client := account.OAuthClient.GetHTTPClient()

	result := make(chan string, 1)
	go func() {
		resp, err := client.Get(api.URL)
		if err != nil {
			result <- "error: " + err.Error()
			return
		}
		defer func() { _ = resp.Body.Close() }()
		body, _ := io.ReadAll(resp.Body)
		result <- string(body)
	}()

	<-started
	if err := am.RefreshToken(context.Background(), account.Email); err != nil {
		t.Fatalf("RefreshToken: %v", err)
	}
	close(release)

	if got := <-result; got != "Bearer fresh-2" {
		t.Fatalf("in-flight request sent %q, want the token installed by RefreshToken", got)
	}
	if got := getAuthorization(t, client, api.URL); got != "Bearer fresh-2" {
		t.Fatalf("later request sent %q, want the token installed by RefreshToken", got)
	}
	if stored := readAccountFile(t, account.TokenFile); stored.Token.AccessToken != "fresh-2" {
		t.Fatalf("stored token = %q, want fresh-2", stored.Token.AccessToken)
	}
}
