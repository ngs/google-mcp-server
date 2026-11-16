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
//
// One persistingTokenSource backs the account's HTTP client for its whole
// lifetime. RefreshToken swaps the underlying source in place (replace) rather
// than building a new client, so API services that cached the client keep
// using the current token.
//
// Lock order: persistingTokenSource.mu, AccountManager.saveMu,
// AccountManager.mu, then Account.mu / OAuthClient.mu.
type persistingTokenSource struct {
	ctx     context.Context
	am      *AccountManager
	account *Account

	mu   sync.Mutex
	base oauth2.TokenSource
	gen  uint64 // incremented by replace
	last string // access token most recently persisted
}

// Token returns a valid token, persisting it when it differs from the last one.
func (s *persistingTokenSource) Token() (*oauth2.Token, error) {
	s.mu.Lock()
	base, gen := s.base, s.gen
	s.mu.Unlock()

	tok, err := base.Token()
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// The source was replaced while this token was being fetched; the newer
	// token has already been persisted, so do not overwrite it.
	if gen != s.gen {
		return tok, nil
	}
	if tok.AccessToken == s.last {
		return tok, nil
	}
	s.last = tok.AccessToken

	if err := s.am.setAccountToken(s.account, tok); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to save refreshed token for %s: %v\n", s.account.Email, err)
	}
	return tok, nil
}

// replace switches the source to start from token and persists it.
func (s *persistingTokenSource) replace(token *oauth2.Token) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.base = s.am.reuseTokenSource(s.ctx, token)
	s.gen++
	s.last = token.AccessToken
	return s.am.setAccountToken(s.account, token)
}

func (am *AccountManager) reuseTokenSource(ctx context.Context, token *oauth2.Token) oauth2.TokenSource {
	return oauth2.ReuseTokenSource(token, am.oauthConfig.TokenSource(ctx, token))
}

// newAccountHTTPClient builds an authenticated HTTP client for an account whose
// refreshed tokens are written back to the account file.
func (am *AccountManager) newAccountHTTPClient(ctx context.Context, account *Account) *http.Client {
	token := account.CurrentToken()
	src := &persistingTokenSource{
		ctx:     ctx,
		am:      am,
		account: account,
		base:    am.reuseTokenSource(ctx, token),
	}
	if token != nil {
		src.last = token.AccessToken
	}
	account.tokenSource = src
	return oauth2.NewClient(ctx, src)
}

// setAccountToken replaces an account's token in memory and, while the account
// is still registered, on disk. A client of a removed or replaced account may
// still refresh its token; writing that token would recreate the removed
// account's file or overwrite the replacement's credentials.
func (am *AccountManager) setAccountToken(account *Account, token *oauth2.Token) error {
	am.saveMu.Lock()
	defer am.saveMu.Unlock()

	account.setToken(token)
	if account.OAuthClient != nil {
		account.OAuthClient.mu.Lock()
		account.OAuthClient.token = token
		account.OAuthClient.mu.Unlock()
	}
	if !am.isRegistered(account) {
		return nil
	}
	return am.saveAccountLocked(account)
}
