package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"go.ngs.io/google-mcp-server/auth"
	"go.ngs.io/google-mcp-server/calendar"
	"go.ngs.io/google-mcp-server/config"
	"go.ngs.io/google-mcp-server/docs"
	"go.ngs.io/google-mcp-server/drive"
	"go.ngs.io/google-mcp-server/gmail"
	"go.ngs.io/google-mcp-server/photos"
	"go.ngs.io/google-mcp-server/server"
	"go.ngs.io/google-mcp-server/sheets"
)

func main() {
	// Set up logging immediately with no buffering
	log.SetOutput(os.Stderr)
	log.SetFlags(0) // Remove flags for cleaner MCP output

	// Check for version flag
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Println("google-mcp-server v0.1.0")
		os.Exit(0)
	}

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}
	// Configuration loaded successfully

	// Initialize OAuth client
	ctx := context.Background()
	log.Println("[DEBUG] Creating OAuth client...")
	oauthClient, err := auth.NewOAuthClient(ctx, cfg.OAuth)
	if err != nil {
		log.Fatalf("Failed to initialize OAuth client: %v", err)
	}
	log.Println("[DEBUG] OAuth client created successfully")

	// Check if HTTP client is available
	log.Println("[DEBUG] About to get HTTP client...")
	httpClient := oauthClient.GetHTTPClient()
	log.Println("[DEBUG] Got HTTP client")
	if httpClient == nil {
		log.Println("[WARNING] HTTP client is nil after OAuth initialization")
	} else {
		log.Println("[DEBUG] HTTP client is ready")
	}

	// Initialize MCP server
	mcpServer := server.NewMCPServer(cfg)

	// Register services before starting the server
	log.Println("[INFO] Starting service registration...")
	if err := registerServices(ctx, mcpServer, oauthClient, cfg); err != nil {
		log.Printf("[WARNING] Some services failed to register: %v", err)
	} else {
		log.Println("[INFO] All services registered successfully")
	}

	// Start the server (blocks until shutdown)
	if err := mcpServer.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func registerServices(ctx context.Context, srv *server.MCPServer, oauth *auth.OAuthClient, cfg *config.Config) error {
	// Use a short timeout for service initialization to prevent blocking
	initTimeout := 5 * time.Second

	// Add delay between service initializations to avoid conflicts
	serviceDelay := 100 * time.Millisecond

	// Initialize and register Calendar service
	if cfg.Services.Calendar.Enabled {
		log.Println("[DEBUG] Initializing Calendar service...")
		initCtx, cancel := context.WithTimeout(ctx, initTimeout)
		calendarClient, err := calendar.NewClient(initCtx, oauth)
		cancel()
		if err != nil {
			log.Printf("[ERROR] Failed to initialize Calendar client: %v\n", err)
			// Continue without Calendar service
		} else {
			calendarHandler := calendar.NewHandler(calendarClient)
			srv.RegisterService("calendar", calendarHandler)
			log.Println("[DEBUG] Calendar service registered")
		}
		// Add delay before next service
		time.Sleep(serviceDelay)
	}

	// Initialize and register Drive service
	if cfg.Services.Drive.Enabled {
		log.Println("[DEBUG] Initializing Drive service...")
		// Add timeout context for initialization
		initCtx, cancel := context.WithTimeout(ctx, initTimeout)
		driveClient, err := drive.NewClient(initCtx, oauth)
		cancel()
		if err != nil {
			log.Printf("[ERROR] Failed to initialize Drive client: %v\n", err)
			// Continue without Drive service instead of failing
		} else {
			driveHandler := drive.NewHandler(driveClient)
			srv.RegisterService("drive", driveHandler)
			log.Println("[DEBUG] Drive service registered")
		}
		// Add delay before next service
		time.Sleep(serviceDelay)
	}

	// Initialize and register Gmail service
	if cfg.Services.Gmail.Enabled {
		// Initialize Gmail service
		initCtx, cancel := context.WithTimeout(ctx, initTimeout)
		gmailClient, err := gmail.NewClient(initCtx, oauth)
		cancel()
		if err != nil {
			// Failed to initialize Gmail client, continue without it
		} else {
			gmailHandler := gmail.NewHandler(gmailClient)
			srv.RegisterService("gmail", gmailHandler)
			// Gmail service registered
		}
		// Add delay before next service
		time.Sleep(serviceDelay)
	}

	// Initialize and register Photos service
	if cfg.Services.Photos.Enabled {
		// Initialize Photos service
		initCtx, cancel := context.WithTimeout(ctx, initTimeout)
		photosClient, err := photos.NewClient(initCtx, oauth)
		cancel()
		if err != nil {
			// Failed to initialize Photos client, continue without it
		} else {
			photosHandler := photos.NewHandler(photosClient)
			srv.RegisterService("photos", photosHandler)
			// Photos service registered
		}
		// Add delay before next service
		time.Sleep(serviceDelay)
	}

	// Initialize and register Sheets service
	if cfg.Services.Sheets.Enabled {
		// Initialize Sheets service
		initCtx, cancel := context.WithTimeout(ctx, initTimeout)
		sheetsClient, err := sheets.NewClient(initCtx, oauth)
		cancel()
		if err != nil {
			// Failed to initialize Sheets client, continue without it
		} else {
			sheetsHandler := sheets.NewHandler(sheetsClient)
			srv.RegisterService("sheets", sheetsHandler)
			// Sheets service registered
		}
		// Add delay before next service
		time.Sleep(serviceDelay)
	}

	// Initialize and register Docs service
	if cfg.Services.Docs.Enabled {
		// Initialize Docs service
		initCtx, cancel := context.WithTimeout(ctx, initTimeout)
		docsClient, err := docs.NewClient(initCtx, oauth)
		cancel()
		if err != nil {
			// Failed to initialize Docs client, continue without it
		} else {
			docsHandler := docs.NewHandler(docsClient)
			srv.RegisterService("docs", docsHandler)
			// Docs service registered
		}
	}

	return nil
}

func init() {
	// Logging is now set up in main()
}
