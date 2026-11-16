package auth

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sync"

	"golang.org/x/oauth2"
)

// persistingTokenSource wraps an account's token source and writes every newly
// issued token back to the account file. Without it, tokens refreshed on demand
// by the HTTP client only live in memory, so the stored expiry goes stale and
// the account is reported as inactive even though API calls keep working.
type persistingTokenSource struct {
	base    oauth2.TokenSource
	am      *AccountManager
	account *Account

	mu   sync.Mutex
	last string // access token most recently persisted
}

// Token returns a valid token, persisting it when it differs from the last one.
func (s *persistingTokenSource) Token() (*oauth2.Token, error) {
	tok, err := s.base.Token()
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if tok.AccessToken == s.last {
		return tok, nil
	}
	s.last = tok.AccessToken

	if err := s.am.setAccountToken(s.account, tok); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to save refreshed token for %s: %v\n", s.account.Email, err)
	}
	return tok, nil
}

// newAccountHTTPClient builds an authenticated HTTP client for an account whose
// refreshed tokens are written back to the account file.
func (am *AccountManager) newAccountHTTPClient(ctx context.Context, account *Account) *http.Client {
	src := &persistingTokenSource{
		base:    oauth2.ReuseTokenSource(account.Token, am.oauthConfig.TokenSource(ctx, account.Token)),
		am:      am,
		account: account,
	}
	if account.Token != nil {
		src.last = account.Token.AccessToken
	}
	return oauth2.NewClient(ctx, src)
}

// setAccountToken replaces an account's token in memory and on disk.
func (am *AccountManager) setAccountToken(account *Account, token *oauth2.Token) error {
	am.saveMu.Lock()
	defer am.saveMu.Unlock()

	account.Token = token
	if account.OAuthClient != nil {
		account.OAuthClient.mu.Lock()
		account.OAuthClient.token = token
		account.OAuthClient.mu.Unlock()
	}
	return am.saveAccountLocked(account)
}
