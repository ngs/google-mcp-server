package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"regexp"
	"strings"

	"go.ngs.io/google-mcp-server/config"
	"go.ngs.io/google-mcp-server/auth"
	"go.ngs.io/google-mcp-server/docs"
	docsv1 "google.golang.org/api/docs/v1"
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
	fmt.Println("Creating Google Docs document...")
	doc, err := docsClient.CreateDocument("Claude Code Instructions for Google MCP Server (Formatted)")
	if err != nil {
		log.Fatalf("Failed to create document: %v", err)
	}

	fmt.Printf("Document created successfully!\n")
	fmt.Printf("Document ID: %s\n", doc.DocumentId)
	fmt.Printf("Document URL: https://docs.google.com/document/d/%s/edit\n", doc.DocumentId)

	// Clean up the markdown and insert as plain text first
	cleanContent := cleanMarkdown(string(content))
	
	// Insert all content as plain text first
	fmt.Println("Adding content...")
	_, err = docsClient.UpdateDocument(doc.DocumentId, cleanContent, "replace")
	if err != nil {
		log.Fatalf("Failed to add content: %v", err)
	}

	// Now apply basic formatting to specific sections
	fmt.Println("Applying basic formatting...")
	err = applyBasicFormatting(docsClient, doc.DocumentId, cleanContent)
	if err != nil {
		log.Printf("Failed to apply formatting: %v", err)
		fmt.Println("Document created with plain text content")
	} else {
		fmt.Println("Basic formatting applied successfully!")
	}

	fmt.Printf("Final Document URL: https://docs.google.com/document/d/%s/edit\n", doc.DocumentId)
}

func cleanMarkdown(content string) string {
	// Remove markdown syntax but keep the structure
	content = strings.ReplaceAll(content, "```bash", "")
	content = strings.ReplaceAll(content, "```go", "")
	content = strings.ReplaceAll(content, "```", "")
	
	// Convert headers to simple text
	content = regexp.MustCompile(`^### (.+)$`).ReplaceAllString(content, "$1")
	content = regexp.MustCompile(`^## (.+)$`).ReplaceAllString(content, "$1")
	content = regexp.MustCompile(`^# (.+)$`).ReplaceAllString(content, "$1")
	
	// Convert bullets
	content = regexp.MustCompile(`^- (.+)$`).ReplaceAllString(content, "• $1")
	
	// Remove bold markdown
	content = strings.ReplaceAll(content, "**", "")
	
	return content
}

func applyBasicFormatting(docsClient *docs.Client, documentId, content string) error {
	// Find positions of titles and headers
	var requests []*docsv1.Request
	
	lines := strings.Split(content, "\n")
	position := int64(1)
	
	for _, line := range lines {
		lineLen := int64(len(line) + 1) // +1 for newline
		
		// Apply title formatting to the first line (main title)
		if position == 1 && strings.TrimSpace(line) != "" {
			requests = append(requests, &docsv1.Request{
				UpdateTextStyle: &docsv1.UpdateTextStyleRequest{
					Range: &docsv1.Range{
						StartIndex: position,
						EndIndex:   position + int64(len(line)),
					},
					TextStyle: &docsv1.TextStyle{
						Bold:     true,
						FontSize: &docsv1.Dimension{Magnitude: 18, Unit: "PT"},
					},
					Fields: "bold,fontSize",
				},
			})
		}
		
		// Apply formatting to section headers (lines that look like headers)
		if strings.Contains(line, "Overview") || 
		   strings.Contains(line, "Guidelines") ||
		   strings.Contains(line, "Implementation") ||
		   strings.Contains(line, "Tasks") ||
		   strings.Contains(line, "Rate Limits") ||
		   strings.Contains(line, "Security") ||
		   strings.Contains(line, "Issues") ||
		   strings.Contains(line, "Debugging") ||
		   strings.Contains(line, "Structure") ||
		   strings.Contains(line, "Contact") ||
		   strings.Contains(line, "Release") ||
		   strings.Contains(line, "Performance") ||
		   strings.Contains(line, "Error Handling") {
			
			if strings.TrimSpace(line) != "" && !strings.HasPrefix(line, "•") {
				requests = append(requests, &docsv1.Request{
					UpdateTextStyle: &docsv1.UpdateTextStyleRequest{
						Range: &docsv1.Range{
							StartIndex: position,
							EndIndex:   position + int64(len(line)),
						},
						TextStyle: &docsv1.TextStyle{
							Bold:     true,
							FontSize: &docsv1.Dimension{Magnitude: 14, Unit: "PT"},
						},
						Fields: "bold,fontSize",
					},
				})
			}
		}
		
		position += lineLen
	}
	
	// Apply the requests
	if len(requests) > 0 {
		_, err := docsClient.BatchUpdate(documentId, requests)
		return err
	}
	
	return nil
}