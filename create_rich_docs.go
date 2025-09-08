package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"

	"go.ngs.io/google-mcp-server/config"
	"go.ngs.io/google-mcp-server/auth"
	"go.ngs.io/google-mcp-server/docs"
)

func main() {
	ctx := context.Background()

	// Read CLAUDE.md file
	content, err := ioutil.ReadFile("CLAUDE.md")
	if err != nil {
		log.Fatalf("Failed to read CLAUDE.md: %v", err)
	}

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Check if OAuth credentials are configured
	if cfg.OAuth.ClientID == "" || cfg.OAuth.ClientSecret == "" {
		log.Printf("OAuth credentials not found in config. Checking environment variables...")
		log.Printf("Please set GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET environment variables")
		log.Printf("Or create a config file with OAuth credentials")
		log.Printf("You can create a Google Cloud project and enable the Docs API:")
		log.Printf("https://console.developers.google.com/")
		return
	}

	// Initialize OAuth client
	oauth, err := auth.NewOAuthClient(ctx, cfg.OAuth)
	if err != nil {
		log.Fatalf("Failed to create OAuth client: %v", err)
	}

	// Create docs client
	docsClient, err := docs.NewClient(ctx, oauth)
	if err != nil {
		log.Fatalf("Failed to create docs client: %v", err)
	}

	// Create the document
	fmt.Println("Creating Google Docs document with rich text formatting...")
	doc, err := docsClient.CreateDocument("Claude Code Instructions for Google MCP Server (Rich Text)")
	if err != nil {
		log.Fatalf("Failed to create document: %v", err)
	}

	fmt.Printf("Document created successfully!\n")
	fmt.Printf("Document ID: %s\n", doc.DocumentId)
	fmt.Printf("Document URL: https://docs.google.com/document/d/%s/edit\n", doc.DocumentId)

	// Create DocumentUpdater instance
	updater := docs.NewDocumentUpdater(docsClient)

	// Apply markdown formatting using the document formatter
	fmt.Println("Adding rich text content to the document...")
	_, err = updater.UpdateWithMarkdown(ctx, doc.DocumentId, string(content), "replace")
	if err != nil {
		log.Fatalf("Failed to format document with markdown: %v", err)
	}

	fmt.Println("Rich text content added successfully!")
	fmt.Printf("Final Document URL: https://docs.google.com/document/d/%s/edit\n", doc.DocumentId)
}