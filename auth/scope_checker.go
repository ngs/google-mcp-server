package auth

import (
	"context"
	"fmt"
	"strings"

	"golang.org/x/oauth2"
	oauth2v2 "google.golang.org/api/oauth2/v2"
	"google.golang.org/api/option"
)

// RequiredScopes defines the required OAuth scopes for each service
var RequiredScopes = map[string][]string{
	"calendar": {
		"https://www.googleapis.com/auth/calendar",
		"https://www.googleapis.com/auth/calendar.events",
	},
	"drive": {
		"https://www.googleapis.com/auth/drive",
		"https://www.googleapis.com/auth/drive.file",
	},
	"gmail": {
		"https://www.googleapis.com/auth/gmail.readonly",
		"https://www.googleapis.com/auth/gmail.modify",
	},
	"sheets": {
		"https://www.googleapis.com/auth/spreadsheets",
	},
	"docs": {
		"https://www.googleapis.com/auth/documents",
	},
	"slides": {
		"https://www.googleapis.com/auth/presentations",
	},
}

// ScopeError represents an error when required scopes are missing
type ScopeError struct {
	Service        string
	RequiredScopes []string
	CurrentScopes  []string
	Account        string
}

func (e *ScopeError) Error() string {
	missing := getMissingScopes(e.RequiredScopes, e.CurrentScopes)
	return fmt.Sprintf(
		"Missing required OAuth scopes for %s service.\n"+
			"Account: %s\n"+
			"Required scopes: %s\n"+
			"Missing scopes: %s\n"+
			"Please re-authenticate with: accounts_add or accounts_refresh",
		e.Service,
		e.Account,
		strings.Join(e.RequiredScopes, ", "),
		strings.Join(missing, ", "),
	)
}

// CheckScopes verifies if an account has the required scopes for a service
func (am *AccountManager) CheckScopes(ctx context.Context, account *Account, service string) error {
	requiredScopes, ok := RequiredScopes[service]
	if !ok {
		// If no specific scopes defined for service, assume it's okay
		return nil
	}

	// Get current token scopes
	currentScopes, err := am.GetTokenScopes(ctx, account)
	if err != nil {
		return fmt.Errorf("failed to get token scopes: %w", err)
	}

	// Check if all required scopes are present
	if !hasAllScopes(requiredScopes, currentScopes) {
		return &ScopeError{
			Service:        service,
			RequiredScopes: requiredScopes,
			CurrentScopes:  currentScopes,
			Account:        account.Email,
		}
	}

	return nil
}

// GetTokenScopes retrieves the scopes associated with an account's token
func (am *AccountManager) GetTokenScopes(ctx context.Context, account *Account) ([]string, error) {
	if account.Token == nil {
		return nil, fmt.Errorf("no token available for account: %s", account.Email)
	}

	// Create a temporary client with the token
	tokenSource := am.oauthConfig.TokenSource(ctx, account.Token)
	httpClient := oauth2.NewClient(ctx, tokenSource)

	// Use the tokeninfo endpoint to get scope information
	oauth2Service, err := oauth2v2.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("failed to create oauth2 service: %w", err)
	}

	tokenInfo, err := oauth2Service.Tokeninfo().AccessToken(account.Token.AccessToken).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get token info: %w", err)
	}

	// Split the scope string into individual scopes
	if tokenInfo.Scope != "" {
		return strings.Split(tokenInfo.Scope, " "), nil
	}

	return []string{}, nil
}

// RefreshAccountWithScopes refreshes an account's authentication with additional scopes
func (am *AccountManager) RefreshAccountWithScopes(ctx context.Context, email string, additionalScopes []string) error {
	account, err := am.GetAccount(email)
	if err != nil {
		return err
	}

	// Combine existing and additional scopes
	currentScopes, _ := am.GetTokenScopes(ctx, account)
	allScopes := append(currentScopes, additionalScopes...)
	allScopes = removeDuplicates(allScopes)

	// Update OAuth config with new scopes
	newConfig := *am.oauthConfig
	newConfig.Scopes = allScopes

	// Force re-authentication with new scopes
	account.Token = nil
	if err := am.saveAccount(account); err != nil {
		return fmt.Errorf("failed to clear token for re-authentication: %w", err)
	}

	return fmt.Errorf("account %s needs re-authentication with additional scopes. Please use accounts_add or accounts_refresh", email)
}

// Helper functions

func hasAllScopes(required, current []string) bool {
	currentMap := make(map[string]bool)
	for _, scope := range current {
		currentMap[scope] = true
	}

	for _, scope := range required {
		if !currentMap[scope] {
			return false
		}
	}

	return true
}

func getMissingScopes(required, current []string) []string {
	currentMap := make(map[string]bool)
	for _, scope := range current {
		currentMap[scope] = true
	}

	missing := []string{}
	for _, scope := range required {
		if !currentMap[scope] {
			missing = append(missing, scope)
		}
	}

	return missing
}

func removeDuplicates(scopes []string) []string {
	seen := make(map[string]bool)
	result := []string{}

	for _, scope := range scopes {
		if !seen[scope] {
			seen[scope] = true
			result = append(result, scope)
		}
	}

	return result
}

// IsScopeError checks if an error is a ScopeError
func IsScopeError(err error) bool {
	_, ok := err.(*ScopeError)
	return ok
}

// IsAPIDisabledError checks if an error indicates an API is disabled
func IsAPIDisabledError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "SERVICE_DISABLED") ||
		strings.Contains(errStr, "has not been used in project") ||
		strings.Contains(errStr, "accessNotConfigured")
}