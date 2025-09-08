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
	fmt.Println("Creating Google Docs document with markdown formatting...")
	doc, err := docsClient.CreateDocument("Claude Code Instructions for Google MCP Server (Markdown Formatted)")
	if err != nil {
		log.Fatalf("Failed to create document: %v", err)
	}

	fmt.Printf("Document created successfully!\n")
	fmt.Printf("Document ID: %s\n", doc.DocumentId)
	fmt.Printf("Document URL: https://docs.google.com/document/d/%s/edit\n", doc.DocumentId)

	// Create DocumentUpdater to use the markdown processing functionality
	updater := docs.NewDocumentUpdater(docsClient)

	// Use the existing markdown processor to convert and format the content
	fmt.Println("Processing markdown content...")
	_, err = updater.UpdateWithMarkdown(ctx, doc.DocumentId, string(content), "replace")
	if err != nil {
		log.Printf("Markdown processing failed: %v", err)
		fmt.Println("Falling back to plain text...")
		
		// If markdown processing fails, add plain text with basic formatting
		_, err = docsClient.UpdateDocument(doc.DocumentId, string(content), "replace")
		if err != nil {
			log.Fatalf("Failed to update document with plain text: %v", err)
		}
		fmt.Println("Plain text content added successfully!")
	} else {
		fmt.Println("Markdown formatting applied successfully!")
	}

	fmt.Printf("Final Document URL: https://docs.google.com/document/d/%s/edit\n", doc.DocumentId)
}