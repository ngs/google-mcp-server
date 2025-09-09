package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	oauth2api "google.golang.org/api/oauth2/v2"
	"google.golang.org/api/option"
)

// AccountManager manages multiple Google accounts
type AccountManager struct {
	accounts    map[string]*Account
	configDir   string
	oauthConfig *oauth2.Config
	mu          sync.RWMutex
}

// Account represents a single authenticated Google account
type Account struct {
	Email       string        `json:"email"`
	Name        string        `json:"name"`
	Picture     string        `json:"picture"`
	Token       *oauth2.Token `json:"token"`
	LastUsed    time.Time     `json:"last_used"`
	TokenFile   string        `json:"token_file"`
	OAuthClient *OAuthClient  `json:"-"`
}

// NewAccountManager creates a new account manager
func NewAccountManager(ctx context.Context, oauthConfig OAuthConfig) (*AccountManager, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".google-mcp-accounts")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	// Create OAuth2 config
	oauth2Config := &oauth2.Config{
		ClientID:     oauthConfig.ClientID,
		ClientSecret: oauthConfig.ClientSecret,
		Endpoint:     google.Endpoint,
		RedirectURL:  oauthConfig.RedirectURI,
		Scopes:       oauthConfig.Scopes,
	}

	if oauth2Config.RedirectURL == "" {
		oauth2Config.RedirectURL = "http://localhost:8080/callback"
	}

	if len(oauth2Config.Scopes) == 0 {
		oauth2Config.Scopes = DefaultScopes()
	}

	am := &AccountManager{
		accounts:    make(map[string]*Account),
		configDir:   configDir,
		oauthConfig: oauth2Config,
	}

	// Load existing accounts
	if err := am.loadAccounts(ctx); err != nil {
		// Log error but don't fail - accounts can be added later
		fmt.Fprintf(os.Stderr, "Warning: failed to load existing accounts: %v\n", err)
	}

	// Check for legacy token and migrate it
	if len(am.accounts) == 0 {
		if err := am.migrateLegacyToken(ctx, oauthConfig); err == nil {
			fmt.Fprintf(os.Stderr, "[INFO] Migrated legacy token to multi-account format\n")
		}
	}

	return am, nil
}

// loadAccounts loads all existing account tokens
func (am *AccountManager) loadAccounts(ctx context.Context) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	files, err := os.ReadDir(am.configDir)
	if err != nil {
		return fmt.Errorf("failed to read config directory: %w", err)
	}

	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".json") {
			continue
		}

		tokenFile := filepath.Join(am.configDir, file.Name())
		data, err := os.ReadFile(tokenFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to read token file %s: %v\n", tokenFile, err)
			continue
		}

		var account Account
		if err := json.Unmarshal(data, &account); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to parse token file %s: %v\n", tokenFile, err)
			continue
		}

		account.TokenFile = tokenFile

		// Create OAuth client for this account
		oauthClient := &OAuthClient{
			config:     am.oauthConfig,
			token:      account.Token,
			tokenFile:  tokenFile,
			httpClient: am.oauthConfig.Client(ctx, account.Token),
		}
		account.OAuthClient = oauthClient

		// Get user info if not available
		if account.Email == "" {
			if err := am.updateUserInfo(ctx, &account); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to get user info: %v\n", err)
			}
		}

		if account.Email != "" {
			am.accounts[account.Email] = &account
		}
	}

	return nil
}

// AddAccount adds a new account or updates existing one
func (am *AccountManager) AddAccount(ctx context.Context, token *oauth2.Token) (*Account, error) {
	am.mu.Lock()
	defer am.mu.Unlock()

	// Create temporary OAuth client to get user info
	tempClient := am.oauthConfig.Client(ctx, token)

	// Get user info
	oauth2Service, err := oauth2api.NewService(ctx, option.WithHTTPClient(tempClient))
	if err != nil {
		return nil, fmt.Errorf("failed to create oauth2 service: %w", err)
	}

	userInfo, err := oauth2Service.Userinfo.Get().Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}

	// Create or update account
	account := &Account{
		Email:    userInfo.Email,
		Name:     userInfo.Name,
		Picture:  userInfo.Picture,
		Token:    token,
		LastUsed: time.Now(),
	}

	// Set token file path
	safeEmail := strings.ReplaceAll(account.Email, "@", "_at_")
	safeEmail = strings.ReplaceAll(safeEmail, ".", "_")
	account.TokenFile = filepath.Join(am.configDir, fmt.Sprintf("%s.json", safeEmail))

	// Create OAuth client for this account
	account.OAuthClient = &OAuthClient{
		config:     am.oauthConfig,
		token:      token,
		tokenFile:  account.TokenFile,
		httpClient: tempClient,
	}

	// Save account
	if err := am.saveAccount(account); err != nil {
		return nil, fmt.Errorf("failed to save account: %w", err)
	}

	am.accounts[account.Email] = account
	return account, nil
}

// saveAccount saves an account to disk
func (am *AccountManager) saveAccount(account *Account) error {
	data, err := json.MarshalIndent(account, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal account: %w", err)
	}

	if err := os.WriteFile(account.TokenFile, data, 0600); err != nil {
		return fmt.Errorf("failed to write token file: %w", err)
	}

	return nil
}

// updateUserInfo updates user information for an account
func (am *AccountManager) updateUserInfo(ctx context.Context, account *Account) error {
	if account.OAuthClient == nil || account.OAuthClient.httpClient == nil {
		return fmt.Errorf("no OAuth client available")
	}

	oauth2Service, err := oauth2api.NewService(ctx, option.WithHTTPClient(account.OAuthClient.httpClient))
	if err != nil {
		return fmt.Errorf("failed to create oauth2 service: %w", err)
	}

	userInfo, err := oauth2Service.Userinfo.Get().Do()
	if err != nil {
		return fmt.Errorf("failed to get user info: %w", err)
	}

	account.Email = userInfo.Email
	account.Name = userInfo.Name
	account.Picture = userInfo.Picture

	return am.saveAccount(account)
}

// GetAccount returns an account by email
func (am *AccountManager) GetAccount(email string) (*Account, error) {
	am.mu.RLock()
	defer am.mu.RUnlock()

	account, exists := am.accounts[email]
	if !exists {
		return nil, fmt.Errorf("account not found: %s", email)
	}

	// Update last used time
	account.LastUsed = time.Now()
	go func() {
		if err := am.saveAccount(account); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to update last used time for %s: %v\n", email, err)
		}
	}()

	return account, nil
}

// ListAccounts returns all accounts
func (am *AccountManager) ListAccounts() []*Account {
	am.mu.RLock()
	defer am.mu.RUnlock()

	accounts := make([]*Account, 0, len(am.accounts))
	for _, account := range am.accounts {
		accounts = append(accounts, account)
	}

	return accounts
}

// GetAccountForContext attempts to determine the appropriate account based on context
func (am *AccountManager) GetAccountForContext(ctx context.Context, hint string) (*Account, error) {
	am.mu.RLock()
	defer am.mu.RUnlock()

	// If hint contains an email address, try to match it
	if strings.Contains(hint, "@") {
		for email, account := range am.accounts {
			if strings.Contains(hint, email) {
				account.LastUsed = time.Now()
				go func() {
					if err := am.saveAccount(account); err != nil {
						fmt.Fprintf(os.Stderr, "Warning: failed to update last used time for %s: %v\n", email, err)
					}
				}()
				return account, nil
			}
		}
	}

	// If hint contains a domain, try to match accounts from that domain
	if strings.Contains(hint, ".") {
		for email, account := range am.accounts {
			domain := strings.Split(email, "@")[1]
			if strings.Contains(hint, domain) {
				account.LastUsed = time.Now()
				go func() {
					if err := am.saveAccount(account); err != nil {
						fmt.Fprintf(os.Stderr, "Warning: failed to update last used time for %s: %v\n", email, err)
					}
				}()
				return account, nil
			}
		}
	}

	// If only one account exists, use it
	if len(am.accounts) == 1 {
		for email, account := range am.accounts {
			account.LastUsed = time.Now()
			go func() {
				if err := am.saveAccount(account); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to update last used time for %s: %v\n", email, err)
				}
			}()
			return account, nil
		}
	}

	// If no clear match, return error listing available accounts
	if len(am.accounts) == 0 {
		return nil, fmt.Errorf("no authenticated accounts available")
	}

	var accountList []string
	for email := range am.accounts {
		accountList = append(accountList, email)
	}

	return nil, fmt.Errorf("multiple accounts available, please specify: %s", strings.Join(accountList, ", "))
}

// RemoveAccount removes an account
func (am *AccountManager) RemoveAccount(email string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	account, exists := am.accounts[email]
	if !exists {
		return fmt.Errorf("account not found: %s", email)
	}

	// Remove token file
	if err := os.Remove(account.TokenFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove token file: %w", err)
	}

	delete(am.accounts, email)
	return nil
}

// RefreshToken refreshes the token for an account
func (am *AccountManager) RefreshToken(ctx context.Context, email string) error {
	account, err := am.GetAccount(email)
	if err != nil {
		return err
	}

	tokenSource := am.oauthConfig.TokenSource(ctx, account.Token)
	newToken, err := tokenSource.Token()
	if err != nil {
		return fmt.Errorf("failed to refresh token: %w", err)
	}

	account.Token = newToken
	account.OAuthClient.token = newToken

	return am.saveAccount(account)
}

// GetOAuthConfig returns the OAuth configuration
func (am *AccountManager) GetOAuthConfig() *oauth2.Config {
	return am.oauthConfig
}

// GetAllOAuthClients returns OAuth clients for all accounts (for cross-account operations)
func (am *AccountManager) GetAllOAuthClients() map[string]*OAuthClient {
	am.mu.RLock()
	defer am.mu.RUnlock()

	clients := make(map[string]*OAuthClient)
	for email, account := range am.accounts {
		if account.OAuthClient != nil {
			clients[email] = account.OAuthClient
		}
	}
	return clients
}

// expandPath expands ~ to home directory
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		homeDir, _ := os.UserHomeDir()
		return filepath.Join(homeDir, path[2:])
	}
	return path
}

// migrateLegacyToken migrates a legacy token file to the multi-account format
func (am *AccountManager) migrateLegacyToken(ctx context.Context, oauthConfig OAuthConfig) error {
	// Check for legacy token file
	tokenFile := expandPath(oauthConfig.TokenFile)
	if tokenFile == "" {
		homeDir, _ := os.UserHomeDir()
		tokenFile = filepath.Join(homeDir, ".google-mcp-token.json")
	}

	// Check if file exists
	if _, err := os.Stat(tokenFile); os.IsNotExist(err) {
		return fmt.Errorf("no legacy token file found")
	}

	// Read the token
	data, err := os.ReadFile(tokenFile)
	if err != nil {
		return fmt.Errorf("failed to read legacy token: %w", err)
	}

	var token oauth2.Token
	if err := json.Unmarshal(data, &token); err != nil {
		return fmt.Errorf("failed to parse legacy token: %w", err)
	}

	// Add as a new account
	account, err := am.AddAccount(ctx, &token)
	if err != nil {
		return fmt.Errorf("failed to migrate account: %w", err)
	}

	fmt.Fprintf(os.Stderr, "[INFO] Successfully migrated account: %s\n", account.Email)
	return nil
}
