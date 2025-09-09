package drive

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"go.ngs.io/google-mcp-server/auth"
	"go.ngs.io/google-mcp-server/server"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

// MultiAccountClient manages Drive operations across multiple accounts
type MultiAccountClient struct {
	accountManager *auth.AccountManager
	clients        map[string]*Client
	mu             sync.RWMutex
}

// NewMultiAccountClient creates a new multi-account Drive client
func NewMultiAccountClient(ctx context.Context, accountManager *auth.AccountManager) (*MultiAccountClient, error) {
	mac := &MultiAccountClient{
		accountManager: accountManager,
		clients:        make(map[string]*Client),
	}

	// Initialize clients for all accounts
	for email, oauthClient := range accountManager.GetAllOAuthClients() {
		service, err := drive.NewService(ctx, option.WithHTTPClient(oauthClient.GetHTTPClient()))
		if err != nil {
			fmt.Printf("Warning: failed to create drive service for %s: %v\n", email, err)
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
		service, err := drive.NewService(ctx, option.WithHTTPClient(account.OAuthClient.GetHTTPClient()))
		if err != nil {
			return nil, "", fmt.Errorf("failed to create drive service: %w", err)
		}

		newClient := &Client{service: service}
		mac.mu.Lock()
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

// SearchAcrossAccounts searches for files across all accounts
func (mac *MultiAccountClient) SearchAcrossAccounts(ctx context.Context, query string, pageSize int64) (map[string][]*drive.File, error) {
	results := make(map[string][]*drive.File)
	var wg sync.WaitGroup
	var mu sync.Mutex
	errors := make([]error, 0)

	mac.mu.RLock()
	clients := make(map[string]*Client)
	for email, client := range mac.clients {
		clients[email] = client
	}
	mac.mu.RUnlock()

	for email, client := range clients {
		wg.Add(1)
		go func(email string, client *Client) {
			defer wg.Done()

			// SearchFiles expects (name, mimeType, modifiedAfter)
			// For cross-account search, we'll use ListFiles with the query directly
			files, err := client.ListFiles(query, pageSize, "")
			if err != nil {
				mu.Lock()
				errors = append(errors, fmt.Errorf("%s: %w", email, err))
				mu.Unlock()
				return
			}

			mu.Lock()
			results[email] = files
			mu.Unlock()
		}(email, client)
	}

	wg.Wait()

	// If all searches failed, return the first error
	if len(errors) == len(clients) && len(errors) > 0 {
		return nil, errors[0]
	}

	return results, nil
}

// ListFilesAcrossAccounts lists files from all accounts
func (mac *MultiAccountClient) ListFilesAcrossAccounts(ctx context.Context, parentID string, pageSize int64) (map[string][]*drive.File, error) {
	results := make(map[string][]*drive.File)
	var wg sync.WaitGroup
	var mu sync.Mutex
	errors := make([]error, 0)

	mac.mu.RLock()
	clients := make(map[string]*Client)
	for email, client := range mac.clients {
		clients[email] = client
	}
	mac.mu.RUnlock()

	for email, client := range clients {
		wg.Add(1)
		go func(email string, client *Client) {
			defer wg.Done()

			files, err := client.ListFiles("", pageSize, parentID)
			if err != nil {
				mu.Lock()
				errors = append(errors, fmt.Errorf("%s: %w", email, err))
				mu.Unlock()
				return
			}

			mu.Lock()
			results[email] = files
			mu.Unlock()
		}(email, client)
	}

	wg.Wait()

	// If all searches failed, return the first error
	if len(errors) == len(clients) && len(errors) > 0 {
		return nil, errors[0]
	}

	return results, nil
}

// MultiAccountHandler handles Drive operations with multi-account support
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
		fmt.Printf("Warning: failed to initialize multi-account client: %v\n", err)
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

// GetTools returns the available Drive tools with multi-account support
func (h *MultiAccountHandler) GetTools() []server.Tool {
	// Get original tools from handler
	if h.handler != nil {
		tools := h.handler.GetTools()

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

		// Add new multi-account specific tools
		tools = append(tools, server.Tool{
			Name:        "drive_files_list_all_accounts",
			Description: "List files from all authenticated accounts",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"parent_id": {
						Type:        "string",
						Description: "Parent folder ID (optional, defaults to root)",
					},
					"page_size": {
						Type:        "number",
						Description: "Number of files per account (max 1000)",
					},
				},
			},
		})

		return tools
	}

	// Return empty if no handler
	return []server.Tool{}
}

// HandleToolCall handles a tool call for Drive service with multi-account support
func (h *MultiAccountHandler) HandleToolCall(ctx context.Context, name string, arguments json.RawMessage) (interface{}, error) {
	// Handle multi-account specific tools
	if name == "drive_files_list_all_accounts" {
		var args struct {
			ParentID string  `json:"parent_id"`
			PageSize float64 `json:"page_size"`
		}
		if err := json.Unmarshal(arguments, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}

		pageSize := int64(args.PageSize)
		if pageSize <= 0 {
			pageSize = 100
		}

		// List files across all accounts
		results, err := h.multiClient.ListFilesAcrossAccounts(ctx, args.ParentID, pageSize)
		if err != nil {
			return nil, err
		}

		// Format results
		formattedResults := make(map[string]interface{})
		totalFiles := 0
		for email, files := range results {
			fileList := make([]map[string]interface{}, len(files))
			for i, file := range files {
				fileInfo := map[string]interface{}{
					"id":           file.Id,
					"name":         file.Name,
					"mimeType":     file.MimeType,
					"size":         file.Size,
					"modifiedTime": file.ModifiedTime,
				}
				if file.WebViewLink != "" {
					fileInfo["webViewLink"] = file.WebViewLink
				}
				if len(file.Parents) > 0 {
					fileInfo["parents"] = file.Parents
				}
				if file.ThumbnailLink != "" {
					fileInfo["thumbnailLink"] = file.ThumbnailLink
				}
				if file.IconLink != "" {
					fileInfo["iconLink"] = file.IconLink
				}
				fileList[i] = fileInfo
			}
			formattedResults[email] = map[string]interface{}{
				"files": fileList,
				"count": len(files),
			}
			totalFiles += len(files)
		}

		return map[string]interface{}{
			"accounts":      formattedResults,
			"total_count":   totalFiles,
			"account_count": len(results),
		}, nil
	}

	// For other tools, check if account parameter is provided
	var accountHint string
	if arguments != nil {
		var args map[string]interface{}
		if err := json.Unmarshal(arguments, &args); err == nil {
			if account, ok := args["account"].(string); ok {
				accountHint = account
			}
		}
	}

	// Try to get client for the specified account
	if accountHint != "" || h.multiClient != nil {
		client, accountUsed, err := h.multiClient.GetClientForContext(ctx, accountHint)
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

	return nil, fmt.Errorf("no handler available for tool: %s", name)
}

// GetResources returns the available Drive resources
func (h *MultiAccountHandler) GetResources() []server.Resource {
	if h.handler != nil {
		return h.handler.GetResources()
	}
	return []server.Resource{}
}

// HandleResourceCall handles a resource call for Drive service
func (h *MultiAccountHandler) HandleResourceCall(ctx context.Context, uri string) (interface{}, error) {
	// For now, delegate to original handler
	if h.handler != nil {
		return h.handler.HandleResourceCall(ctx, uri)
	}
	return nil, fmt.Errorf("no handler available for resource: %s", uri)
}
