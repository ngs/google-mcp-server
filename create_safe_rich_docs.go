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
	doc, err := docsClient.CreateDocument("Claude Code Instructions for Google MCP Server (Final)")
	if err != nil {
		log.Fatalf("Failed to create document: %v", err)
	}

	fmt.Printf("Document created successfully!\n")
	fmt.Printf("Document ID: %s\n", doc.DocumentId)
	fmt.Printf("Document URL: https://docs.google.com/document/d/%s/edit\n", doc.DocumentId)

	// Process content safely step by step
	fmt.Println("Processing and formatting content safely...")
	err = processContentSafely(docsClient, doc.DocumentId, string(content))
	if err != nil {
		log.Fatalf("Failed to process content: %v", err)
	}

	fmt.Println("Content processed and formatted successfully!")
	fmt.Printf("Final Document URL: https://docs.google.com/document/d/%s/edit\n", doc.DocumentId)
}

func processContentSafely(docsClient *docs.Client, documentId, content string) error {
	// Clean and structure the markdown content
	processedContent := cleanAndStructureMarkdown(content)
	
	// Insert the cleaned content first
	_, err := docsClient.UpdateDocument(documentId, processedContent, "replace")
	if err != nil {
		return fmt.Errorf("failed to insert content: %w", err)
	}
	
	// Get the document to check actual content length
	doc, err := docsClient.GetDocument(documentId)
	if err != nil {
		return fmt.Errorf("failed to get document: %w", err)
	}
	
	// Apply safe formatting
	err = applySafeFormatting(docsClient, documentId, doc, processedContent)
	if err != nil {
		log.Printf("Warning: Some formatting could not be applied: %v", err)
		// Continue anyway - at least we have the content
	}
	
	return nil
}

func cleanAndStructureMarkdown(content string) string {
	lines := strings.Split(content, "\n")
	var result strings.Builder
	
	for _, line := range lines {
		// Convert headers to structured text
		if strings.HasPrefix(line, "# ") {
			result.WriteString("═══ " + strings.TrimPrefix(line, "# ") + " ═══\n")
		} else if strings.HasPrefix(line, "## ") {
			result.WriteString("▌" + strings.TrimPrefix(line, "## ") + "\n")
		} else if strings.HasPrefix(line, "### ") {
			result.WriteString("◆ " + strings.TrimPrefix(line, "### ") + "\n")
		} else if strings.HasPrefix(line, "- ") {
			result.WriteString("• " + strings.TrimPrefix(line, "- ") + "\n")
		} else if strings.HasPrefix(line, "  - ") {
			result.WriteString("  ◦ " + strings.TrimPrefix(line, "  - ") + "\n")
		} else if regexp.MustCompile(`^\d+\.\s+`).MatchString(line) {
			// Keep numbered lists as is
			result.WriteString(line + "\n")
		} else if strings.HasPrefix(line, "```") {
			// Mark code blocks
			if line == "```" {
				result.WriteString("────────────────────────────────────\n")
			} else {
				result.WriteString("CODE: " + strings.TrimPrefix(line, "```") + "\n")
				result.WriteString("────────────────────────────────────\n")
			}
		} else {
			// Regular text - clean up bold markdown
			cleanLine := strings.ReplaceAll(line, "**", "")
			cleanLine = strings.ReplaceAll(cleanLine, "`", "")
			result.WriteString(cleanLine + "\n")
		}
	}
	
	return result.String()
}

func applySafeFormatting(docsClient *docs.Client, documentId string, doc *docsv1.Document, processedContent string) error {
	lines := strings.Split(processedContent, "\n")
	currentIndex := int64(1)
	
	var requests []*docsv1.Request
	
	for _, line := range lines {
		lineLength := int64(len(line) + 1) // +1 for newline
		
		// Skip empty lines
		if strings.TrimSpace(line) == "" {
			currentIndex += lineLength
			continue
		}
		
		// Apply formatting based on line markers
		var textStyle *docsv1.TextStyle
		var shouldStyle bool = false
		
		if strings.HasPrefix(line, "═══") && strings.HasSuffix(line, "═══") {
			// Main title
			textStyle = &docsv1.TextStyle{
				Bold:     true,
				FontSize: &docsv1.Dimension{Magnitude: 18, Unit: "PT"},
			}
			shouldStyle = true
		} else if strings.HasPrefix(line, "▌") {
			// Section header
			textStyle = &docsv1.TextStyle{
				Bold:     true,
				FontSize: &docsv1.Dimension{Magnitude: 14, Unit: "PT"},
			}
			shouldStyle = true
		} else if strings.HasPrefix(line, "◆") {
			// Subsection header
			textStyle = &docsv1.TextStyle{
				Bold:     true,
				FontSize: &docsv1.Dimension{Magnitude: 12, Unit: "PT"},
			}
			shouldStyle = true
		} else if strings.HasPrefix(line, "CODE:") {
			// Code marker
			textStyle = &docsv1.TextStyle{
				Bold:   true,
				Italic: true,
			}
			shouldStyle = true
		} else if strings.Contains(line, "────────") {
			// Code block separator
			textStyle = &docsv1.TextStyle{
				WeightedFontFamily: &docsv1.WeightedFontFamily{
					FontFamily: "Consolas",
					Weight:     400,
				},
			}
			shouldStyle = true
		}
		
		// Only apply formatting if we have text and it's safe to do so
		if shouldStyle && textStyle != nil && lineLength > 1 {
			// Make sure we don't exceed document bounds
			endIndex := currentIndex + lineLength - 2 // -2 to exclude newline and be safe
			if endIndex > currentIndex {
				requests = append(requests, &docsv1.Request{
					UpdateTextStyle: &docsv1.UpdateTextStyleRequest{
						Range: &docsv1.Range{
							StartIndex: currentIndex,
							EndIndex:   endIndex,
						},
						TextStyle: textStyle,
						Fields:    "*",
					},
				})
			}
		}
		
		currentIndex += lineLength
	}
	
	// Apply formatting in small batches to avoid API limits
	batchSize := 10
	for i := 0; i < len(requests); i += batchSize {
		end := i + batchSize
		if end > len(requests) {
			end = len(requests)
		}
		
		batch := requests[i:end]
		if len(batch) > 0 {
			_, err := docsClient.BatchUpdate(documentId, batch)
			if err != nil {
				return fmt.Errorf("failed to apply formatting batch %d-%d: %w", i, end, err)
			}
		}
	}
	
	return nil
}