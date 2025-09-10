package gmail

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"go.ngs.io/google-mcp-server/auth"
	"go.ngs.io/google-mcp-server/server"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

// MultiAccountClient manages Gmail operations across multiple accounts
type MultiAccountClient struct {
	accountManager *auth.AccountManager
	clients        map[string]*Client
	mu             sync.RWMutex
}

// NewMultiAccountClient creates a new multi-account Gmail client
func NewMultiAccountClient(ctx context.Context, accountManager *auth.AccountManager) (*MultiAccountClient, error) {
	mac := &MultiAccountClient{
		accountManager: accountManager,
		clients:        make(map[string]*Client),
	}

	// Initialize clients for all accounts
	for email, oauthClient := range accountManager.GetAllOAuthClients() {
		service, err := gmail.NewService(ctx, option.WithHTTPClient(oauthClient.GetHTTPClient()))
		if err != nil {
			fmt.Printf("Warning: failed to create gmail service for %s: %v\n", email, err)
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
		service, err := gmail.NewService(ctx, option.WithHTTPClient(account.OAuthClient.GetHTTPClient()))
		if err != nil {
			return nil, "", fmt.Errorf("failed to create gmail service: %w", err)
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

// SearchAcrossAccounts searches for messages across all accounts
func (mac *MultiAccountClient) SearchAcrossAccounts(ctx context.Context, query string, maxResults int64) (map[string][]*gmail.Message, error) {
	results := make(map[string][]*gmail.Message)
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

			messages, err := client.ListMessages(query, maxResults)
			if err != nil {
				mu.Lock()
				errors = append(errors, fmt.Errorf("%s: %w", email, err))
				mu.Unlock()
				return
			}

			mu.Lock()
			results[email] = messages
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

// MultiAccountHandler handles Gmail operations with multi-account support
type MultiAccountHandler struct {
	multiClient *MultiAccountClient
	client      *Client // Default client for backward compatibility
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

	return &MultiAccountHandler{
		multiClient: multiClient,
		client:      defaultClient,
	}
}

// GetTools returns the available Gmail tools with multi-account support
func (h *MultiAccountHandler) GetTools() []server.Tool {
	return []server.Tool{
		{
			Name:        "gmail_messages_list",
			Description: "List email messages",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"query": {
						Type:        "string",
						Description: "Search query (e.g., 'from:user@example.com')",
					},
					"max_results": {
						Type:        "number",
						Description: "Maximum number of results",
					},
					"account": {
						Type:        "string",
						Description: "Email address of the account to use (optional)",
					},
				},
			},
		},
		{
			Name:        "gmail_message_get",
			Description: "Get email message details",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"message_id": {
						Type:        "string",
						Description: "Message ID",
					},
					"account": {
						Type:        "string",
						Description: "Email address of the account to use (optional)",
					},
				},
				Required: []string{"message_id"},
			},
		},
		{
			Name:        "gmail_messages_list_all_accounts",
			Description: "List messages from all authenticated accounts",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"query": {
						Type:        "string",
						Description: "Search query (e.g., 'is:unread')",
					},
					"max_results": {
						Type:        "number",
						Description: "Maximum number of results per account",
					},
				},
			},
		},
	}
}

// HandleToolCall handles a tool call for Gmail service with multi-account support
func (h *MultiAccountHandler) HandleToolCall(ctx context.Context, name string, arguments json.RawMessage) (interface{}, error) {
	switch name {
	case "gmail_messages_list":
		var args struct {
			Query      string  `json:"query"`
			MaxResults float64 `json:"max_results"`
			Account    string  `json:"account"`
		}
		if err := json.Unmarshal(arguments, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}

		// Try to get client for specified account
		client, accountUsed, err := h.multiClient.GetClientForContext(ctx, args.Account)
		if err != nil {
			// Fall back to default client if available
			if h.client != nil {
				client = h.client
				accountUsed = "default"
			} else {
				return nil, err
			}
		}

		messages, err := client.ListMessages(args.Query, int64(args.MaxResults))
		if err != nil {
			return nil, err
		}

		// Format messages for response
		messageList := make([]map[string]interface{}, len(messages))
		for i, msg := range messages {
			messageList[i] = map[string]interface{}{
				"id":       msg.Id,
				"threadId": msg.ThreadId,
			}
		}
		return map[string]interface{}{
			"messages": messageList,
			"account":  accountUsed,
		}, nil

	case "gmail_message_get":
		var args struct {
			MessageID string `json:"message_id"`
			Account   string `json:"account"`
		}
		if err := json.Unmarshal(arguments, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}

		// Try to get client for specified account
		client, accountUsed, err := h.multiClient.GetClientForContext(ctx, args.Account)
		if err != nil {
			// Fall back to default client if available
			if h.client != nil {
				client = h.client
				accountUsed = "default"
			} else {
				return nil, err
			}
		}

		message, err := client.GetMessage(args.MessageID)
		if err != nil {
			return nil, err
		}

		// Format message for response
		result := map[string]interface{}{
			"id":           message.Id,
			"threadId":     message.ThreadId,
			"labelIds":     message.LabelIds,
			"snippet":      message.Snippet,
			"historyId":    message.HistoryId,
			"internalDate": message.InternalDate,
			"sizeEstimate": message.SizeEstimate,
			"account":      accountUsed,
		}

		// Extract headers for easier access
		if message.Payload != nil && message.Payload.Headers != nil {
			headers := make(map[string]string)
			for _, header := range message.Payload.Headers {
				headers[header.Name] = header.Value
			}
			result["headers"] = headers

			// Add body if available
			if message.Payload.Body != nil && message.Payload.Body.Data != "" {
				result["body"] = message.Payload.Body.Data
			}
		}

		return result, nil

	case "gmail_messages_list_all_accounts":
		var args struct {
			Query      string  `json:"query"`
			MaxResults float64 `json:"max_results"`
		}
		if err := json.Unmarshal(arguments, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}

		// Default query to inbox if not specified
		query := args.Query
		if query == "" {
			query = "in:inbox"
		}

		// Search across all accounts
		results, err := h.multiClient.SearchAcrossAccounts(ctx, query, int64(args.MaxResults))
		if err != nil {
			return nil, err
		}

		// Format results
		formattedResults := make(map[string]interface{})
		totalMessages := 0
		for email, messages := range results {
			messageList := make([]map[string]interface{}, len(messages))
			for i, msg := range messages {
				messageList[i] = map[string]interface{}{
					"id":       msg.Id,
					"threadId": msg.ThreadId,
				}
			}
			formattedResults[email] = map[string]interface{}{
				"messages": messageList,
				"count":    len(messages),
			}
			totalMessages += len(messages)
		}

		return map[string]interface{}{
			"accounts":      formattedResults,
			"total_count":   totalMessages,
			"account_count": len(results),
		}, nil

	default:
		// Fall back to original handler for backward compatibility
		if h.client != nil {
			handler := &Handler{client: h.client}
			return handler.HandleToolCall(ctx, name, arguments)
		}
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

// GetResources returns the available Gmail resources
func (h *MultiAccountHandler) GetResources() []server.Resource {
	return []server.Resource{
		{
			URI:         "gmail://inbox",
			Name:        "Inbox",
			Description: "Gmail inbox messages",
			MimeType:    "application/json",
		},
	}
}

// HandleResourceCall handles a resource call for Gmail service
func (h *MultiAccountHandler) HandleResourceCall(ctx context.Context, uri string) (interface{}, error) {
	if uri == "gmail://inbox" {
		// List inbox messages from all accounts
		results, err := h.multiClient.SearchAcrossAccounts(ctx, "in:inbox", 20)
		if err != nil {
			// Fall back to default client if available
			if h.client != nil {
				messages, err := h.client.ListMessages("in:inbox", 20)
				if err != nil {
					return nil, err
				}
				return map[string]interface{}{
					"messages": messages,
					"count":    len(messages),
				}, nil
			}
			return nil, err
		}

		// Format results
		formattedResults := make(map[string]interface{})
		totalMessages := 0
		for email, messages := range results {
			formattedResults[email] = map[string]interface{}{
				"messages": messages,
				"count":    len(messages),
			}
			totalMessages += len(messages)
		}

		return map[string]interface{}{
			"accounts":    formattedResults,
			"total_count": totalMessages,
		}, nil
	}
	return nil, fmt.Errorf("unknown resource: %s", uri)
}
