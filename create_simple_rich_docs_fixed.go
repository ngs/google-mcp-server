package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
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
	fmt.Println("Creating Google Docs document with basic formatting...")
	doc, err := docsClient.CreateDocument("Claude Code Instructions for Google MCP Server (Rich Text)")
	if err != nil {
		log.Fatalf("Failed to create document: %v", err)
	}

	fmt.Printf("Document created successfully!\n")
	fmt.Printf("Document ID: %s\n", doc.DocumentId)
	fmt.Printf("Document URL: https://docs.google.com/document/d/%s/edit\n", doc.DocumentId)

	// Parse the markdown content and create structured requests
	fmt.Println("Adding formatted content...")
	
	lines := strings.Split(string(content), "\n")
	var requests []*docsv1.Request
	currentIndex := int64(1)

	for _, line := range lines {
		var request *docsv1.Request
		var textStyle *docsv1.TextStyle
		var paragraphStyle *docsv1.ParagraphStyle

		// Handle different markdown elements
		if strings.HasPrefix(line, "# ") {
			// Main heading
			text := strings.TrimPrefix(line, "# ") + "\n"
			request = &docsv1.Request{
				InsertText: &docsv1.InsertTextRequest{
					Location: &docsv1.Location{Index: currentIndex},
					Text:     text,
				},
			}
			textStyle = &docsv1.TextStyle{
				Bold:     true,
				FontSize: &docsv1.Dimension{Magnitude: 20, Unit: "PT"},
			}
			paragraphStyle = &docsv1.ParagraphStyle{
				NamedStyleType: "TITLE",
			}
			
		} else if strings.HasPrefix(line, "## ") {
			// Section heading
			text := strings.TrimPrefix(line, "## ") + "\n"
			request = &docsv1.Request{
				InsertText: &docsv1.InsertTextRequest{
					Location: &docsv1.Location{Index: currentIndex},
					Text:     text,
				},
			}
			textStyle = &docsv1.TextStyle{
				Bold:     true,
				FontSize: &docsv1.Dimension{Magnitude: 16, Unit: "PT"},
			}
			paragraphStyle = &docsv1.ParagraphStyle{
				NamedStyleType: "HEADING_1",
			}
			
		} else if strings.HasPrefix(line, "### ") {
			// Subsection heading
			text := strings.TrimPrefix(line, "### ") + "\n"
			request = &docsv1.Request{
				InsertText: &docsv1.InsertTextRequest{
					Location: &docsv1.Location{Index: currentIndex},
					Text:     text,
				},
			}
			textStyle = &docsv1.TextStyle{
				Bold:     true,
				FontSize: &docsv1.Dimension{Magnitude: 14, Unit: "PT"},
			}
			paragraphStyle = &docsv1.ParagraphStyle{
				NamedStyleType: "HEADING_2",
			}
			
		} else if strings.HasPrefix(line, "- ") {
			// Bullet point
			text := "• " + strings.TrimPrefix(line, "- ") + "\n"
			request = &docsv1.Request{
				InsertText: &docsv1.InsertTextRequest{
					Location: &docsv1.Location{Index: currentIndex},
					Text:     text,
				},
			}
			paragraphStyle = &docsv1.ParagraphStyle{
				IndentFirstLine: &docsv1.Dimension{Magnitude: 18, Unit: "PT"},
				IndentStart:     &docsv1.Dimension{Magnitude: 18, Unit: "PT"},
			}
			
		} else if strings.Contains(line, "**") {
			// Bold text (simplified)
			text := strings.ReplaceAll(line, "**", "") + "\n"
			request = &docsv1.Request{
				InsertText: &docsv1.InsertTextRequest{
					Location: &docsv1.Location{Index: currentIndex},
					Text:     text,
				},
			}
			textStyle = &docsv1.TextStyle{
				Bold: true,
			}
			
		} else if strings.HasPrefix(line, "```") {
			// Skip code block markers
			continue
			
		} else {
			// Regular text
			text := line + "\n"
			request = &docsv1.Request{
				InsertText: &docsv1.InsertTextRequest{
					Location: &docsv1.Location{Index: currentIndex},
					Text:     text,
				},
			}
		}

		// Add the insert request
		if request != nil {
			requests = append(requests, request)
			textLength := int64(len(request.InsertText.Text))
			
			// Add text style if specified
			if textStyle != nil {
				requests = append(requests, &docsv1.Request{
					UpdateTextStyle: &docsv1.UpdateTextStyleRequest{
						Range: &docsv1.Range{
							StartIndex: currentIndex,
							EndIndex:   currentIndex + textLength - 1,
						},
						TextStyle: textStyle,
						Fields:    "*",
					},
				})
			}
			
			// Add paragraph style if specified
			if paragraphStyle != nil {
				requests = append(requests, &docsv1.Request{
					UpdateParagraphStyle: &docsv1.UpdateParagraphStyleRequest{
						Range: &docsv1.Range{
							StartIndex: currentIndex,
							EndIndex:   currentIndex + textLength,
						},
						ParagraphStyle: paragraphStyle,
						Fields:         "*",
					},
				})
			}
			
			currentIndex += textLength
		}
	}

	// Execute the batch update
	if len(requests) > 0 {
		_, err = docsClient.BatchUpdate(doc.DocumentId, requests)
		if err != nil {
			log.Printf("Failed to apply formatting: %v", err)
			// Fallback to plain text
			fmt.Println("Falling back to plain text...")
			_, err = docsClient.UpdateDocument(doc.DocumentId, string(content), "replace")
			if err != nil {
				log.Fatalf("Failed to update document with plain text: %v", err)
			}
			fmt.Println("Plain text content added successfully!")
		} else {
			fmt.Println("Rich text formatting applied successfully!")
		}
	}

	fmt.Printf("Final Document URL: https://docs.google.com/document/d/%s/edit\n", doc.DocumentId)
}