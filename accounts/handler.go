package accounts

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"go.ngs.io/google-mcp-server/auth"
	"go.ngs.io/google-mcp-server/server"
	"golang.org/x/oauth2"
)

// Handler implements account management tools
type Handler struct {
	accountManager *auth.AccountManager
}

// NewHandler creates a new accounts handler
func NewHandler(accountManager *auth.AccountManager) *Handler {
	return &Handler{
		accountManager: accountManager,
	}
}

// GetTools returns the available account management tools
func (h *Handler) GetTools() []server.Tool {
	return []server.Tool{
		{
			Name:        "accounts_list",
			Description: "List all authenticated Google accounts",
			InputSchema: server.InputSchema{
				Type:       "object",
				Properties: map[string]server.Property{},
			},
		},
		{
			Name:        "accounts_details",
			Description: "Get detailed information about a specific account",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"email": {
						Type:        "string",
						Description: "Email address of the account (optional, shows all if not specified)",
					},
				},
			},
		},
		{
			Name:        "accounts_add",
			Description: "Add a new Google account (initiates OAuth flow)",
			InputSchema: server.InputSchema{
				Type:       "object",
				Properties: map[string]server.Property{},
			},
		},
		{
			Name:        "accounts_remove",
			Description: "Remove an authenticated account",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"email": {
						Type:        "string",
						Description: "Email address of the account to remove",
					},
				},
				Required: []string{"email"},
			},
		},
		{
			Name:        "accounts_refresh",
			Description: "Refresh authentication token for an account",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"email": {
						Type:        "string",
						Description: "Email address of the account to refresh",
					},
				},
				Required: []string{"email"},
			},
		},
	}
}

// HandleToolCall handles a tool call for account management
func (h *Handler) HandleToolCall(ctx context.Context, name string, arguments json.RawMessage) (interface{}, error) {
	switch name {
	case "accounts_list":
		return h.handleAccountsList(ctx)

	case "accounts_details":
		var args struct {
			Email string `json:"email"`
		}
		if err := json.Unmarshal(arguments, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
		return h.handleAccountsDetails(ctx, args.Email)

	case "accounts_add":
		return h.handleAccountsAdd(ctx)

	case "accounts_remove":
		var args struct {
			Email string `json:"email"`
		}
		if err := json.Unmarshal(arguments, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
		return h.handleAccountsRemove(ctx, args.Email)

	case "accounts_refresh":
		var args struct {
			Email string `json:"email"`
		}
		if err := json.Unmarshal(arguments, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
		return h.handleAccountsRefresh(ctx, args.Email)

	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

// handleAccountsList lists all authenticated accounts
func (h *Handler) handleAccountsList(ctx context.Context) (interface{}, error) {
	accounts := h.accountManager.ListAccounts()

	// Sort by last used time
	sort.Slice(accounts, func(i, j int) bool {
		return accounts[i].LastUsed.After(accounts[j].LastUsed)
	})

	type AccountSummary struct {
		Email    string    `json:"email"`
		Name     string    `json:"name"`
		LastUsed time.Time `json:"last_used"`
		Active   bool      `json:"active"`
	}

	summaries := make([]AccountSummary, len(accounts))
	for i, account := range accounts {
		summaries[i] = AccountSummary{
			Email:    account.Email,
			Name:     account.Name,
			LastUsed: account.LastUsed,
			Active:   account.Token != nil && account.Token.Valid(),
		}
	}

	return map[string]interface{}{
		"accounts": summaries,
		"count":    len(summaries),
	}, nil
}

// handleAccountsDetails shows detailed information about accounts
func (h *Handler) handleAccountsDetails(ctx context.Context, email string) (interface{}, error) {
	if email == "" {
		// Show all accounts with details
		accounts := h.accountManager.ListAccounts()

		type AccountDetails struct {
			Email       string    `json:"email"`
			Name        string    `json:"name"`
			Picture     string    `json:"picture,omitempty"`
			LastUsed    time.Time `json:"last_used"`
			TokenExpiry time.Time `json:"token_expiry,omitempty"`
			Scopes      []string  `json:"scopes,omitempty"`
			Active      bool      `json:"active"`
		}

		details := make([]AccountDetails, len(accounts))
		for i, account := range accounts {
			detail := AccountDetails{
				Email:    account.Email,
				Name:     account.Name,
				Picture:  account.Picture,
				LastUsed: account.LastUsed,
				Active:   account.Token != nil && account.Token.Valid(),
			}

			if account.Token != nil {
				detail.TokenExpiry = account.Token.Expiry
				// Extract scopes from token if available
				if scope, ok := account.Token.Extra("scope").(string); ok {
					detail.Scopes = strings.Split(scope, " ")
				}
			}

			details[i] = detail
		}

		return map[string]interface{}{
			"accounts": details,
			"count":    len(details),
		}, nil
	}

	// Show specific account details
	account, err := h.accountManager.GetAccount(email)
	if err != nil {
		return nil, err
	}

	result := map[string]interface{}{
		"email":     account.Email,
		"name":      account.Name,
		"picture":   account.Picture,
		"last_used": account.LastUsed,
		"active":    account.Token != nil && account.Token.Valid(),
	}

	if account.Token != nil {
		result["token_expiry"] = account.Token.Expiry
		result["token_valid"] = account.Token.Valid()

		// Extract scopes from token if available
		if scope, ok := account.Token.Extra("scope").(string); ok {
			result["scopes"] = strings.Split(scope, " ")
		}
	}

	return result, nil
}

// handleAccountsAdd initiates OAuth flow to add a new account
func (h *Handler) handleAccountsAdd(ctx context.Context) (interface{}, error) {
	// Get OAuth config from account manager
	config := h.accountManager.GetOAuthConfig()

	// For MCP context, we'll start the server in background and return the URL
	// The user needs to open the URL manually
	callbackServer := auth.NewOAuthCallbackServer(config)

	// Start the server in a goroutine
	go func() {
		token, err := callbackServer.StartAndWaitForCallback(context.Background())
		if err != nil {
			fmt.Fprintf(os.Stderr, "OAuth authentication failed: %v\n", err)
			return
		}

		// Add the account
		account, err := h.accountManager.AddAccount(context.Background(), token)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to add account: %v\n", err)
			return
		}

		fmt.Fprintf(os.Stderr, "Successfully added account: %s\n", account.Email)
	}()

	// Wait a moment for server to start
	time.Sleep(100 * time.Millisecond)

	// Return the auth URL for the user to open
	authURL := config.AuthCodeURL("state", oauth2.AccessTypeOffline)

	return map[string]interface{}{
		"message":      "OAuth server started. Please open the URL below in your browser to authenticate",
		"auth_url":     authURL,
		"callback_url": callbackServer.GetCallbackURL(),
		"instructions": []string{
			"1. Open the auth_url in your browser",
			"2. Log in with the Google account you want to add",
			"3. Grant the requested permissions",
			"4. The account will be automatically added when authentication completes",
			"5. Check the server logs for confirmation",
		},
		"note": "The OAuth server will timeout after 5 minutes if no authentication is received",
	}, nil
}

// handleAccountsRemove removes an account
func (h *Handler) handleAccountsRemove(ctx context.Context, email string) (interface{}, error) {
	if err := h.accountManager.RemoveAccount(email); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"message": fmt.Sprintf("Account %s has been removed", email),
		"success": true,
	}, nil
}

// handleAccountsRefresh refreshes the token for an account
func (h *Handler) handleAccountsRefresh(ctx context.Context, email string) (interface{}, error) {
	if err := h.accountManager.RefreshToken(ctx, email); err != nil {
		return nil, err
	}

	account, err := h.accountManager.GetAccount(email)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"message":      fmt.Sprintf("Token refreshed for account %s", email),
		"email":        account.Email,
		"token_expiry": account.Token.Expiry,
		"token_valid":  account.Token.Valid(),
	}, nil
}

// GetResources returns the available resources
func (h *Handler) GetResources() []server.Resource {
	return []server.Resource{
		{
			URI:         "accounts://list",
			Name:        "Authenticated Accounts",
			Description: "List of all authenticated Google accounts",
			MimeType:    "application/json",
		},
	}
}

// HandleResourceCall handles a resource call
func (h *Handler) HandleResourceCall(ctx context.Context, uri string) (interface{}, error) {
	if uri == "accounts://list" {
		return h.handleAccountsList(ctx)
	}
	return nil, fmt.Errorf("unknown resource: %s", uri)
}
