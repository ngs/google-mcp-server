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

// Token returns a valid token, persisting it when it differs from the last
// one written.
func (s *persistingTokenSource) Token() (*oauth2.Token, error) {
	for {
		s.mu.Lock()
		base, gen := s.base, s.gen
		s.mu.Unlock()

		tok, err := base.Token()

		s.mu.Lock()
		// The source was replaced while this token was being fetched. Use the
		// replacement instead, so the client neither fails with the old
		// source's error nor keeps sending a token superseded by RefreshToken.
		if gen != s.gen {
			s.mu.Unlock()
			continue
		}
		if err != nil {
			s.mu.Unlock()
			return nil, err
		}
		if tok.AccessToken != s.last {
			// Advance last only after a successful save, so a failed write is
			// retried on the next request while the token is still valid.
			if err := s.am.setAccountToken(s.account, tok); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to save refreshed token for %s: %v\n", s.account.Email, err)
			} else {
				s.last = tok.AccessToken
			}
		}
		s.mu.Unlock()
		return tok, nil
	}
}

// replace switches the source to start from token and persists it.
func (s *persistingTokenSource) replace(token *oauth2.Token) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.base = s.am.reuseTokenSource(s.ctx, token)
	s.gen++
	if err := s.am.setAccountToken(s.account, token); err != nil {
		return err
	}
	s.last = token.AccessToken
	return nil
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

	// Unlike oauth2.NewClient, do not wrap src in another ReuseTokenSource:
	// src already reuses valid tokens, and a second cache in the transport
	// would keep serving a token after RefreshToken replaced it and would
	// skip retrying a failed save.
	client := &http.Client{}
	if cc, ok := ctx.Value(oauth2.HTTPClient).(*http.Client); ok && cc != nil {
		client.Transport = cc.Transport
		client.CheckRedirect = cc.CheckRedirect
		client.Jar = cc.Jar
		client.Timeout = cc.Timeout
	}
	client.Transport = &oauth2.Transport{Source: src, Base: client.Transport}
	return client
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
