package drive

import (
	"context"
	"testing"

	"go.ngs.io/google-mcp-server/auth"
	"go.ngs.io/google-mcp-server/server"
)

func newTestAccountManager(t *testing.T) *auth.AccountManager {
	t.Helper()

	// Keep the account manager away from the real home directory
	t.Setenv("HOME", t.TempDir())

	am, err := auth.NewAccountManager(context.Background(), auth.OAuthConfig{})
	if err != nil {
		t.Fatalf("Failed to create account manager: %v", err)
	}

	return am
}

func toolNames(tools []server.Tool) map[string]server.Tool {
	byName := make(map[string]server.Tool, len(tools))
	for _, tool := range tools {
		byName[tool.Name] = tool
	}
	return byName
}

// TestGetToolsWithoutDefaultClient covers the multi-account-only setup, where
// no default OAuth client exists and the Drive tools must still be listed.
func TestGetToolsWithoutDefaultClient(t *testing.T) {
	handler := NewMultiAccountHandler(newTestAccountManager(t), nil)

	tools := handler.GetTools()
	if len(tools) == 0 {
		t.Fatal("Expected Drive tools without a default client, got none")
	}

	byName := toolNames(tools)
	for _, name := range []string{"drive_files_list", "drive_files_list_all_accounts"} {
		if _, ok := byName[name]; !ok {
			t.Errorf("Expected tool %s to be listed", name)
		}
	}

	if _, ok := byName["drive_files_list"].InputSchema.Properties["account"]; !ok {
		t.Error("Expected drive_files_list to accept an account parameter")
	}
}

func TestGetToolsWithDefaultClient(t *testing.T) {
	handler := NewMultiAccountHandler(newTestAccountManager(t), &Client{})

	tools := handler.GetTools()
	withoutClient := NewMultiAccountHandler(newTestAccountManager(t), nil).GetTools()

	if len(tools) != len(withoutClient) {
		t.Errorf("Expected the same number of tools with and without a default client, got %d and %d",
			len(tools), len(withoutClient))
	}

	// The multi-account list is the base tool set plus the all-accounts tool
	if len(tools) != len(defaultDriveTools())+1 {
		t.Errorf("Expected %d tools, got %d", len(defaultDriveTools())+1, len(tools))
	}

	byName := toolNames(tools)
	for _, name := range []string{"drive_files_list", "drive_files_list_all_accounts"} {
		if _, ok := byName[name]; !ok {
			t.Errorf("Expected tool %s to be listed", name)
		}
	}
}

// TestGetToolsDoesNotMutateDefaults ensures the account parameter injection
// does not leak into the shared tool definitions.
func TestGetToolsDoesNotMutateDefaults(t *testing.T) {
	handler := NewMultiAccountHandler(newTestAccountManager(t), nil)
	handler.GetTools()

	for _, tool := range defaultDriveTools() {
		if _, ok := tool.InputSchema.Properties["account"]; ok {
			t.Errorf("Tool %s gained an account parameter in the shared definitions", tool.Name)
		}
	}
}
