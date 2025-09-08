package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"go.ngs.io/google-mcp-server/auth"
)

// Config represents the application configuration
type Config struct {
	OAuth    auth.OAuthConfig `json:"oauth"`
	Services ServicesConfig   `json:"services"`
	Global   GlobalConfig     `json:"global"`
}

// ServicesConfig represents configuration for all services
type ServicesConfig struct {
	Calendar CalendarConfig `json:"calendar"`
	Drive    DriveConfig    `json:"drive"`
	Gmail    GmailConfig    `json:"gmail"`
	Photos   PhotosConfig   `json:"photos"`
	Sheets   SheetsConfig   `json:"sheets"`
	Docs     DocsConfig     `json:"docs"`
}

// CalendarConfig represents Calendar service configuration
type CalendarConfig struct {
	Enabled          bool   `json:"enabled"`
	DefaultCalendar  string `json:"default_calendar,omitempty"`
	ReminderMinutes  int    `json:"reminder_minutes,omitempty"`
	TimeZone         string `json:"time_zone,omitempty"`
}

// DriveConfig represents Drive service configuration
type DriveConfig struct {
	Enabled        bool   `json:"enabled"`
	DefaultFolder  string `json:"default_folder,omitempty"`
	ChunkSize      int    `json:"chunk_size,omitempty"`
	MaxRetries     int    `json:"max_retries,omitempty"`
}

// GmailConfig represents Gmail service configuration
type GmailConfig struct {
	Enabled        bool     `json:"enabled"`
	SendLimit      int      `json:"send_limit,omitempty"`
	DefaultLabels  []string `json:"default_labels,omitempty"`
	Signature      string   `json:"signature,omitempty"`
	MaxResults     int      `json:"max_results,omitempty"`
}

// PhotosConfig represents Photos service configuration
type PhotosConfig struct {
	Enabled        bool   `json:"enabled"`
	UploadQuality  string `json:"upload_quality,omitempty"`
	AutoBackup     bool   `json:"auto_backup,omitempty"`
	MaxBatchSize   int    `json:"max_batch_size,omitempty"`
}

// SheetsConfig represents Sheets service configuration
type SheetsConfig struct {
	Enabled         bool   `json:"enabled"`
	DefaultRange    string `json:"default_range,omitempty"`
	BatchSize       int    `json:"batch_size,omitempty"`
	NumberFormat    string `json:"number_format,omitempty"`
	DateFormat      string `json:"date_format,omitempty"`
}

// DocsConfig represents Docs service configuration
type DocsConfig struct {
	Enabled        bool   `json:"enabled"`
	DefaultFormat  string `json:"default_format,omitempty"`
	TemplateFolder string `json:"template_folder,omitempty"`
}

// GlobalConfig represents global configuration
type GlobalConfig struct {
	LogLevel       string `json:"log_level,omitempty"`
	Timeout        int    `json:"timeout,omitempty"`
	RetryCount     int    `json:"retry_count,omitempty"`
	RetryDelay     int    `json:"retry_delay,omitempty"`
	MaxConcurrency int    `json:"max_concurrency,omitempty"`
}

// Load loads configuration from various sources
func Load() (*Config, error) {
	cfg := &Config{
		Services: ServicesConfig{
			Calendar: CalendarConfig{Enabled: true},
			Drive:    DriveConfig{Enabled: true},
			Gmail:    GmailConfig{Enabled: true},
			Photos:   PhotosConfig{Enabled: true},
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

	// Try to load from environment variables
	if err := cfg.loadFromEnv(); err != nil {
		return nil, fmt.Errorf("failed to load environment variables: %w", err)
	}

	// Try to load from config file
	configPaths := []string{
		"config.json",
		"config.local.json",
		filepath.Join(os.Getenv("HOME"), ".google-mcp-server", "config.json"),
		"/etc/google-mcp-server/config.json",
	}

	for _, path := range configPaths {
		if err := cfg.loadFromFile(path); err == nil {
			break
		}
	}

	// Validate configuration
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Set defaults
	cfg.setDefaults()

	return cfg, nil
}

// loadFromEnv loads configuration from environment variables
func (c *Config) loadFromEnv() error {
	// OAuth settings
	if clientID := os.Getenv("GOOGLE_CLIENT_ID"); clientID != "" {
		c.OAuth.ClientID = clientID
	}
	if clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET"); clientSecret != "" {
		c.OAuth.ClientSecret = clientSecret
	}
	if redirectURI := os.Getenv("GOOGLE_REDIRECT_URI"); redirectURI != "" {
		c.OAuth.RedirectURI = redirectURI
	}
	if tokenFile := os.Getenv("GOOGLE_TOKEN_FILE"); tokenFile != "" {
		c.OAuth.TokenFile = tokenFile
	}

	// Service enable/disable flags
	if os.Getenv("DISABLE_CALENDAR") == "true" {
		c.Services.Calendar.Enabled = false
	}
	if os.Getenv("DISABLE_DRIVE") == "true" {
		c.Services.Drive.Enabled = false
	}
	if os.Getenv("DISABLE_GMAIL") == "true" {
		c.Services.Gmail.Enabled = false
	}
	if os.Getenv("DISABLE_PHOTOS") == "true" {
		c.Services.Photos.Enabled = false
	}
	if os.Getenv("DISABLE_SHEETS") == "true" {
		c.Services.Sheets.Enabled = false
	}
	if os.Getenv("DISABLE_DOCS") == "true" {
		c.Services.Docs.Enabled = false
	}

	// Global settings
	if logLevel := os.Getenv("LOG_LEVEL"); logLevel != "" {
		c.Global.LogLevel = logLevel
	}

	return nil
}

// loadFromFile loads configuration from a JSON file
func (c *Config) loadFromFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(c); err != nil {
		return fmt.Errorf("failed to decode config file %s: %w", path, err)
	}

	return nil
}

// validate validates the configuration
func (c *Config) validate() error {
	// OAuth validation is done in the OAuth client
	// Just check if at least one service is enabled
	if !c.Services.Calendar.Enabled &&
		!c.Services.Drive.Enabled &&
		!c.Services.Gmail.Enabled &&
		!c.Services.Photos.Enabled &&
		!c.Services.Sheets.Enabled &&
		!c.Services.Docs.Enabled {
		return fmt.Errorf("at least one service must be enabled")
	}

	return nil
}

// setDefaults sets default values for configuration
func (c *Config) setDefaults() {
	// Gmail defaults
	if c.Services.Gmail.Enabled {
		if c.Services.Gmail.SendLimit == 0 {
			c.Services.Gmail.SendLimit = 250 // Gmail daily limit
		}
		if c.Services.Gmail.MaxResults == 0 {
			c.Services.Gmail.MaxResults = 100
		}
	}

	// Drive defaults
	if c.Services.Drive.Enabled {
		if c.Services.Drive.ChunkSize == 0 {
			c.Services.Drive.ChunkSize = 5 * 1024 * 1024 // 5MB chunks
		}
		if c.Services.Drive.MaxRetries == 0 {
			c.Services.Drive.MaxRetries = 3
		}
	}

	// Photos defaults
	if c.Services.Photos.Enabled {
		if c.Services.Photos.UploadQuality == "" {
			c.Services.Photos.UploadQuality = "original"
		}
		if c.Services.Photos.MaxBatchSize == 0 {
			c.Services.Photos.MaxBatchSize = 50
		}
	}

	// Sheets defaults
	if c.Services.Sheets.Enabled {
		if c.Services.Sheets.BatchSize == 0 {
			c.Services.Sheets.BatchSize = 1000
		}
		if c.Services.Sheets.DateFormat == "" {
			c.Services.Sheets.DateFormat = "2006-01-02"
		}
	}

	// Calendar defaults
	if c.Services.Calendar.Enabled {
		if c.Services.Calendar.TimeZone == "" {
			c.Services.Calendar.TimeZone = "UTC"
		}
		if c.Services.Calendar.ReminderMinutes == 0 {
			c.Services.Calendar.ReminderMinutes = 10
		}
	}
}

// SaveExample saves an example configuration file
func SaveExample(path string) error {
	example := &Config{
		OAuth: auth.OAuthConfig{
			ClientID:     "YOUR_CLIENT_ID.apps.googleusercontent.com",
			ClientSecret: "YOUR_CLIENT_SECRET",
			RedirectURI:  "http://localhost:8080/callback",
			TokenFile:    "~/.google-mcp-token.json",
			Scopes:       auth.DefaultScopes(),
		},
		Services: ServicesConfig{
			Calendar: CalendarConfig{
				Enabled:         true,
				DefaultCalendar: "primary",
				ReminderMinutes: 10,
				TimeZone:        "America/New_York",
			},
			Drive: DriveConfig{
				Enabled:       true,
				DefaultFolder: "root",
				ChunkSize:     5242880,
				MaxRetries:    3,
			},
			Gmail: GmailConfig{
				Enabled:       true,
				SendLimit:     250,
				DefaultLabels: []string{"INBOX"},
				MaxResults:    100,
			},
			Photos: PhotosConfig{
				Enabled:       true,
				UploadQuality: "original",
				AutoBackup:    false,
				MaxBatchSize:  50,
			},
			Sheets: SheetsConfig{
				Enabled:      true,
				DefaultRange: "A1",
				BatchSize:    1000,
				NumberFormat: "#,##0.00",
				DateFormat:   "2006-01-02",
			},
			Docs: DocsConfig{
				Enabled:       true,
				DefaultFormat: "text",
			},
		},
		Global: GlobalConfig{
			LogLevel:       "info",
			Timeout:        300,
			RetryCount:     3,
			RetryDelay:     1000,
			MaxConcurrency: 10,
		},
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(example)
}