package sheets

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"

	"go.ngs.io/google-mcp-server/auth"
	"go.ngs.io/google-mcp-server/server"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

// MultiAccountClient manages Sheets operations across multiple accounts
type MultiAccountClient struct {
	accountManager *auth.AccountManager
	clients        map[string]*Client
	mu             sync.RWMutex
}

// NewMultiAccountClient creates a new multi-account Sheets client
func NewMultiAccountClient(ctx context.Context, accountManager *auth.AccountManager) (*MultiAccountClient, error) {
	mac := &MultiAccountClient{
		accountManager: accountManager,
		clients:        make(map[string]*Client),
	}

	// Initialize clients for all accounts
	for email, oauthClient := range accountManager.GetAllOAuthClients() {
		service, err := sheets.NewService(ctx, option.WithHTTPClient(oauthClient.GetHTTPClient()))
		if err != nil {
			log.Printf("[WARNING] Failed to create sheets service for %s: %v\n", email, err)
			continue
		}
		mac.clients[email] = &Client{service: service}
	}

	return mac, nil
}

// GetClientForContext returns the appropriate client based on context hints
func (mac *MultiAccountClient) GetClientForContext(ctx context.Context, hint string) (*Client, string, error) {
	// First try to get a specific account based on the hint
	account, err := mac.accountManager.GetAccountForContext(ctx, hint)
	if err == nil && account != nil {
		mac.mu.RLock()
		client, exists := mac.clients[account.Email]
		mac.mu.RUnlock()

		if exists {
			return client, account.Email, nil
		}

		// Create client on demand if not exists
		service, err := sheets.NewService(ctx, option.WithHTTPClient(account.OAuthClient.GetHTTPClient()))
		if err != nil {
			return nil, "", fmt.Errorf("failed to create sheets service: %w", err)
		}

		mac.mu.Lock()
		// Another goroutine may have created the client while we were building
		// ours, so re-check under the write lock and keep the existing one.
		if existing, ok := mac.clients[account.Email]; ok {
			mac.mu.Unlock()
			return existing, account.Email, nil
		}
		newClient := &Client{service: service}
		mac.clients[account.Email] = newClient
		mac.mu.Unlock()

		return newClient, account.Email, nil
	}

	// If context is unclear but only one account exists, use it
	accounts := mac.accountManager.ListAccounts()
	if len(accounts) == 1 {
		email := accounts[0].Email
		mac.mu.RLock()
		client, exists := mac.clients[email]
		mac.mu.RUnlock()

		if exists {
			return client, email, nil
		}
	}

	// Return error with available accounts
	if len(accounts) == 0 {
		return nil, "", fmt.Errorf("no authenticated accounts available")
	}

	var accountList []string
	for _, acc := range accounts {
		accountList = append(accountList, acc.Email)
	}

	return nil, "", fmt.Errorf("please specify account: %s", strings.Join(accountList, ", "))
}

// MultiAccountHandler handles Sheets operations with multi-account support
type MultiAccountHandler struct {
	multiClient *MultiAccountClient
	handler     *Handler // Original handler for backward compatibility
}

// NewMultiAccountHandler creates a new handler with multi-account support
func NewMultiAccountHandler(accountManager *auth.AccountManager, defaultClient *Client) *MultiAccountHandler {
	ctx := context.Background()
	multiClient, err := NewMultiAccountClient(ctx, accountManager)
	if err != nil {
		// Log error but continue with limited functionality
		log.Printf("[WARNING] Failed to initialize multi-account sheets client: %v\n", err)
		multiClient = &MultiAccountClient{
			accountManager: accountManager,
			clients:        make(map[string]*Client),
		}
	}

	// Create original handler for backward compatibility
	var handler *Handler
	if defaultClient != nil {
		handler = NewHandler(defaultClient)
	}

	return &MultiAccountHandler{
		multiClient: multiClient,
		handler:     handler,
	}
}

// GetTools returns the available Sheets tools with an added account parameter.
// The tool list does not depend on a default OAuth client being available, so
// the tools stay visible in multi-account-only setups.
func (h *MultiAccountHandler) GetTools() []server.Tool {
	tools := defaultSheetsTools()

	// Add account parameter to existing tools
	for i := range tools {
		if tools[i].InputSchema.Properties == nil {
			tools[i].InputSchema.Properties = make(map[string]server.Property)
		}
		tools[i].InputSchema.Properties["account"] = server.Property{
			Type:        "string",
			Description: "Email address of the account to use (optional)",
		}
	}

	return tools
}

// HandleToolCall handles a tool call for Sheets service with multi-account support
func (h *MultiAccountHandler) HandleToolCall(ctx context.Context, name string, arguments json.RawMessage) (interface{}, error) {
	// Check if account parameter is provided
	var accountHint string
	if arguments != nil {
		var args map[string]interface{}
		if err := json.Unmarshal(arguments, &args); err == nil {
			if account, ok := args["account"].(string); ok {
				accountHint = account
			}
		}
	}

	// Try to get client for the specified account (or the sole account)
	var accountErr error
	if h.multiClient != nil {
		client, accountUsed, err := h.multiClient.GetClientForContext(ctx, accountHint)
		accountErr = err
		if err == nil {
			// Create a temporary handler with the selected client
			tempHandler := NewHandler(client)
			result, err := tempHandler.HandleToolCall(ctx, name, arguments)
			if err != nil {
				return nil, err
			}

			// Add account information to result if it's a map
			if resultMap, ok := result.(map[string]interface{}); ok {
				resultMap["account"] = accountUsed
			}

			return result, nil
		}
	}

	// Fall back to original handler for backward compatibility
	if h.handler != nil {
		return h.handler.HandleToolCall(ctx, name, arguments)
	}

	// Without a default client, surface why no account could be selected
	if accountErr != nil {
		return nil, accountErr
	}

	return nil, fmt.Errorf("no handler available for tool: %s", name)
}

// GetResources returns the available Sheets resources
func (h *MultiAccountHandler) GetResources() []server.Resource {
	if h.handler != nil {
		return h.handler.GetResources()
	}
	return []server.Resource{}
}

// HandleResourceCall handles a resource call for Sheets service
func (h *MultiAccountHandler) HandleResourceCall(ctx context.Context, uri string) (interface{}, error) {
	if h.handler != nil {
		return h.handler.HandleResourceCall(ctx, uri)
	}
	return nil, fmt.Errorf("no handler available for resource: %s", uri)
}
