package main

import (
	"context"
	"strings"
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

// TestVersionStringTracksServerVersion guards against --version drifting from
// the real version, which it previously did (it was pinned at v0.1.0 while the
// project shipped 0.3.0).
func TestVersionStringTracksServerVersion(t *testing.T) {
	got := versionString()

	if want := "google-mcp-server v" + server.VERSION; got != want {
		t.Errorf("versionString() = %q, want %q", got, want)
	}
	if strings.Contains(got, "v0.1.0") && server.VERSION != "0.1.0" {
		t.Errorf("versionString() still reports a hardcoded v0.1.0: %q", got)
	}
}
