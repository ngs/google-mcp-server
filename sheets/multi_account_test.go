package sheets

import (
	"context"
	"testing"

	"go.ngs.io/google-mcp-server/auth"
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

// TestGetToolsWithoutDefaultClient covers the multi-account-only setup, where
// no default OAuth client exists and the Sheets tools must still be listed.
func TestGetToolsWithoutDefaultClient(t *testing.T) {
	handler := NewMultiAccountHandler(newTestAccountManager(t), nil)

	tools := handler.GetTools()
	if len(tools) == 0 {
		t.Fatal("Expected Sheets tools without a default client, got none")
	}

	byName := make(map[string]bool, len(tools))
	for _, tool := range tools {
		byName[tool.Name] = true
	}
	for _, name := range []string{"sheets_spreadsheet_get", "sheets_values_get", "sheets_values_update"} {
		if !byName[name] {
			t.Errorf("Expected tool %s to be listed", name)
		}
	}

	if _, ok := tools[0].InputSchema.Properties["account"]; !ok {
		t.Errorf("Expected tool %s to accept an account parameter", tools[0].Name)
	}
}

// TestGetToolsDoesNotMutateDefaults ensures the account parameter injection
// does not leak into the shared tool definitions.
func TestGetToolsDoesNotMutateDefaults(t *testing.T) {
	handler := NewMultiAccountHandler(newTestAccountManager(t), nil)
	handler.GetTools()

	for _, tool := range defaultSheetsTools() {
		if _, ok := tool.InputSchema.Properties["account"]; ok {
			t.Errorf("Tool %s gained an account parameter in the shared definitions", tool.Name)
		}
	}
}
