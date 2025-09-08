package main

import (
	"context"
	"fmt"
	"log"
	"os"

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
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize OAuth client
	ctx := context.Background()
	oauthClient, err := auth.NewOAuthClient(ctx, cfg.OAuth)
	if err != nil {
		log.Fatalf("Failed to initialize OAuth client: %v", err)
	}

	// Initialize MCP server
	mcpServer := server.NewMCPServer(cfg)

	// Register service handlers
	if err := registerServices(ctx, mcpServer, oauthClient, cfg); err != nil {
		log.Fatalf("Failed to register services: %v", err)
	}

	// Start the server
	if err := mcpServer.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func registerServices(ctx context.Context, srv *server.MCPServer, oauth *auth.OAuthClient, cfg *config.Config) error {
	// Initialize and register Calendar service
	if cfg.Services.Calendar.Enabled {
		calendarClient, err := calendar.NewClient(ctx, oauth)
		if err != nil {
			return fmt.Errorf("failed to initialize Calendar client: %w", err)
		}
		calendarHandler := calendar.NewHandler(calendarClient)
		srv.RegisterService("calendar", calendarHandler)
	}

	// Initialize and register Drive service
	if cfg.Services.Drive.Enabled {
		driveClient, err := drive.NewClient(ctx, oauth)
		if err != nil {
			return fmt.Errorf("failed to initialize Drive client: %w", err)
		}
		driveHandler := drive.NewHandler(driveClient)
		srv.RegisterService("drive", driveHandler)
	}

	// Initialize and register Gmail service
	if cfg.Services.Gmail.Enabled {
		gmailClient, err := gmail.NewClient(ctx, oauth)
		if err != nil {
			return fmt.Errorf("failed to initialize Gmail client: %w", err)
		}
		gmailHandler := gmail.NewHandler(gmailClient)
		srv.RegisterService("gmail", gmailHandler)
	}

	// Initialize and register Photos service
	if cfg.Services.Photos.Enabled {
		photosClient, err := photos.NewClient(ctx, oauth)
		if err != nil {
			return fmt.Errorf("failed to initialize Photos client: %w", err)
		}
		photosHandler := photos.NewHandler(photosClient)
		srv.RegisterService("photos", photosHandler)
	}

	// Initialize and register Sheets service
	if cfg.Services.Sheets.Enabled {
		sheetsClient, err := sheets.NewClient(ctx, oauth)
		if err != nil {
			return fmt.Errorf("failed to initialize Sheets client: %w", err)
		}
		sheetsHandler := sheets.NewHandler(sheetsClient)
		srv.RegisterService("sheets", sheetsHandler)
	}

	// Initialize and register Docs service
	if cfg.Services.Docs.Enabled {
		docsClient, err := docs.NewClient(ctx, oauth)
		if err != nil {
			return fmt.Errorf("failed to initialize Docs client: %w", err)
		}
		docsHandler := docs.NewHandler(docsClient)
		srv.RegisterService("docs", docsHandler)
	}

	return nil
}

func init() {
	// Set up logging
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	
	// Check for version flag
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Println("google-mcp-server v0.1.0")
		os.Exit(0)
	}
}