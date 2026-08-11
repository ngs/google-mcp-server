package main

import (
	"context"
	"testing"

	"go.ngs.io/google-mcp-server/auth"
	"go.ngs.io/google-mcp-server/config"
	"go.ngs.io/google-mcp-server/server"
)

func TestInit(t *testing.T) {
	// Basic test to ensure the package can be imported
	// This is a placeholder test
	t.Log("Main package initialized successfully")
}

// TestRegisterServicesWithNilOAuth ensures that multi-account mode (no default
// OAuth client) registers services without panicking.
func TestRegisterServicesWithNilOAuth(t *testing.T) {
	// Isolate the account manager from the real home directory
	t.Setenv("HOME", t.TempDir())

	ctx := context.Background()

	cfg := &config.Config{
		Services: config.ServicesConfig{
			Accounts: config.AccountsConfig{Enabled: true},
			Calendar: config.CalendarConfig{Enabled: true},
			Drive:    config.DriveConfig{Enabled: true},
			Gmail:    config.GmailConfig{Enabled: true},
			Sheets:   config.SheetsConfig{Enabled: true},
			Docs:     config.DocsConfig{Enabled: true},
			Slides:   config.SlidesConfig{Enabled: true},
		},
	}

	accountManager, err := auth.NewAccountManager(ctx, cfg.OAuth)
	if err != nil {
		t.Fatalf("Failed to create account manager: %v", err)
	}

	srv := server.NewMCPServer(cfg)

	if err := registerServices(ctx, srv, accountManager, nil, cfg); err != nil {
		t.Fatalf("registerServices returned an error with a nil OAuth client: %v", err)
	}
}
