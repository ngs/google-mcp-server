package main

import (
	"context"
	"encoding/json"
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

	// Create handler to use the format functionality
	handler := docs.NewHandler(docsClient)

	// Create the document first
	fmt.Println("Creating Google Docs document...")
	doc, err := docsClient.CreateDocument("Claude Code Instructions for Google MCP Server (Formatted)")
	if err != nil {
		log.Fatalf("Failed to create document: %v", err)
	}

	fmt.Printf("Document created successfully!\n")
	fmt.Printf("Document ID: %s\n", doc.DocumentId)
	fmt.Printf("Document URL: https://docs.google.com/document/d/%s/edit\n", doc.DocumentId)

	// Create the args for the format tool call
	args := map[string]interface{}{
		"document_id":      doc.DocumentId,
		"markdown_content": string(content),
		"mode":            "replace",
	}

	// Use the format tool to add rich text
	fmt.Println("Applying markdown formatting...")
	result, err := handler.HandleToolCall(ctx, "docs_document_format", mustMarshal(args))
	if err != nil {
		// If formatting fails, try simple text update
		fmt.Printf("Formatting failed (%v), falling back to plain text...\n", err)
		_, err = docsClient.UpdateDocument(doc.DocumentId, string(content), "replace")
		if err != nil {
			log.Fatalf("Failed to update document with plain text: %v", err)
		}
		fmt.Println("Plain text content added successfully!")
	} else {
		fmt.Printf("Formatting successful: %+v\n", result)
		fmt.Println("Rich text content added successfully!")
	}

	fmt.Printf("Final Document URL: https://docs.google.com/document/d/%s/edit\n", doc.DocumentId)
}

func mustMarshal(v interface{}) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}