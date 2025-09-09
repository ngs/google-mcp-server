package drive

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"go.ngs.io/google-mcp-server/auth"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

// MultiAccountClient manages drive operations across multiple accounts
type MultiAccountClient struct {
	accountManager *auth.AccountManager
	clients        map[string]*Client
	mu             sync.RWMutex
}

// NewMultiAccountClient creates a new multi-account drive client
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
func (mac *MultiAccountClient) SearchAcrossAccounts(ctx context.Context, query string) (map[string][]*drive.File, error) {
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

			fileList, err := client.service.Files.List().Q(query).Do()
			if err != nil {
				mu.Lock()
				errors = append(errors, fmt.Errorf("%s: %w", email, err))
				mu.Unlock()
				return
			}

			mu.Lock()
			results[email] = fileList.Files
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

			call := client.service.Files.List()
			if parentID != "" {
				call = call.Q(fmt.Sprintf("'%s' in parents", parentID))
			}
			if pageSize > 0 {
				call = call.PageSize(pageSize)
			}

			fileList, err := call.Do()
			if err != nil {
				mu.Lock()
				errors = append(errors, fmt.Errorf("%s: %w", email, err))
				mu.Unlock()
				return
			}

			mu.Lock()
			results[email] = fileList.Files
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

// UploadFileWithAccount uploads a file with a specific account
func (mac *MultiAccountClient) UploadFileWithAccount(ctx context.Context, email string, file *drive.File, content []byte) (*drive.File, error) {
	mac.mu.RLock()
	_, exists := mac.clients[email]
	mac.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("no client for account %s", email)
	}

	// Implementation would use client.service.Files.Create(...).Media(bytes.NewReader(content)).Do()
	// Simplified for now - full implementation will be added when needed
	return nil, fmt.Errorf("upload implementation needed")
}
