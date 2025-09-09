package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigDefaults(t *testing.T) {
	cfg := &Config{
		Services: ServicesConfig{
			Calendar: CalendarConfig{Enabled: true},
			Drive:    DriveConfig{Enabled: true},
			Gmail:    GmailConfig{Enabled: true},
			Sheets:   SheetsConfig{Enabled: true},
			Docs:     DocsConfig{Enabled: true},
		},
		Global: GlobalConfig{
			LogLevel:       "info",
			Timeout:        300,
			RetryCount:     3,
			RetryDelay:     1000,
			MaxConcurrency: 10,
		},
	}

	cfg.setDefaults()

	// Test Gmail defaults
	if cfg.Services.Gmail.SendLimit != 250 {
		t.Errorf("Expected Gmail SendLimit to be 250, got %d", cfg.Services.Gmail.SendLimit)
	}
	if cfg.Services.Gmail.MaxResults != 100 {
		t.Errorf("Expected Gmail MaxResults to be 100, got %d", cfg.Services.Gmail.MaxResults)
	}

	// Test Drive defaults
	if cfg.Services.Drive.ChunkSize != 5*1024*1024 {
		t.Errorf("Expected Drive ChunkSize to be 5MB, got %d", cfg.Services.Drive.ChunkSize)
	}
	if cfg.Services.Drive.MaxRetries != 3 {
		t.Errorf("Expected Drive MaxRetries to be 3, got %d", cfg.Services.Drive.MaxRetries)
	}

	// Test Sheets defaults
	if cfg.Services.Sheets.BatchSize != 1000 {
		t.Errorf("Expected Sheets BatchSize to be 1000, got %d", cfg.Services.Sheets.BatchSize)
	}
	if cfg.Services.Sheets.DateFormat != "2006-01-02" {
		t.Errorf("Expected Sheets DateFormat to be '2006-01-02', got %s", cfg.Services.Sheets.DateFormat)
	}

	// Test Calendar defaults
	if cfg.Services.Calendar.TimeZone != "UTC" {
		t.Errorf("Expected Calendar TimeZone to be 'UTC', got %s", cfg.Services.Calendar.TimeZone)
	}
	if cfg.Services.Calendar.ReminderMinutes != 10 {
		t.Errorf("Expected Calendar ReminderMinutes to be 10, got %d", cfg.Services.Calendar.ReminderMinutes)
	}
}

func TestConfigValidation(t *testing.T) {
	// Test with no services enabled
	cfg := &Config{
		Services: ServicesConfig{
			Calendar: CalendarConfig{Enabled: false},
			Drive:    DriveConfig{Enabled: false},
			Gmail:    GmailConfig{Enabled: false},
			Sheets:   SheetsConfig{Enabled: false},
			Docs:     DocsConfig{Enabled: false},
		},
	}

	err := cfg.validate()
	if err == nil {
		t.Error("Expected validation error when no services are enabled")
	}

	// Test with at least one service enabled
	cfg.Services.Calendar.Enabled = true
	err = cfg.validate()
	if err != nil {
		t.Errorf("Expected no validation error with Calendar enabled, got: %v", err)
	}
}

func TestSaveExample(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test-example.json")

	err := SaveExample(testFile)
	if err != nil {
		t.Fatalf("Failed to save example config: %v", err)
	}

	// Check if file exists
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Error("Example config file was not created")
	}

	// Clean up
	_ = os.Remove(testFile)
}
