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
			request = &docs.Request{
				InsertText: &docs.InsertTextRequest{
					Location: &docs.Location{Index: currentIndex},
					Text:     text,
				},
			}
			textStyle = &docs.TextStyle{
				Bold:     true,
				FontSize: &docs.Dimension{Magnitude: 14, Unit: "PT"},
			}
			paragraphStyle = &docs.ParagraphStyle{
				NamedStyleType: "HEADING_2",
			}
			
		} else if strings.HasPrefix(line, "- ") {
			// Bullet point
			text := "• " + strings.TrimPrefix(line, "- ") + "\n"
			request = &docs.Request{
				InsertText: &docs.InsertTextRequest{
					Location: &docs.Location{Index: currentIndex},
					Text:     text,
				},
			}
			paragraphStyle = &docs.ParagraphStyle{
				IndentFirstLine: &docs.Dimension{Magnitude: 18, Unit: "PT"},
				IndentStart:     &docs.Dimension{Magnitude: 18, Unit: "PT"},
			}
			
		} else if strings.HasPrefix(line, "```") {
			// Code block (simplified - just make monospace)
			if line == "```" {
				continue // Skip empty code block markers
			}
			text := line + "\n"
			request = &docs.Request{
				InsertText: &docs.InsertTextRequest{
					Location: &docs.Location{Index: currentIndex},
					Text:     text,
				},
			}
			textStyle = &docs.TextStyle{
				WeightedFontFamily: &docs.WeightedFontFamily{
					FontFamily: "Consolas",
					Weight:     400,
				},
				BackgroundColor: &docs.OptionalColor{
					Color: &docs.Color{
						RgbColor: &docs.RgbColor{
							Red:   0.95,
							Green: 0.95,
							Blue:  0.95,
						},
					},
				},
			}
			
		} else {
			// Regular text
			text := line + "\n"
			if text == "\n" && currentIndex > 1 {
				// Add paragraph break
				text = "\n"
			}
			request = &docs.Request{
				InsertText: &docs.InsertTextRequest{
					Location: &docs.Location{Index: currentIndex},
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
				requests = append(requests, &docs.Request{
					UpdateTextStyle: &docs.UpdateTextStyleRequest{
						Range: &docs.Range{
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
				requests = append(requests, &docs.Request{
					UpdateParagraphStyle: &docs.UpdateParagraphStyleRequest{
						Range: &docs.Range{
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
			log.Fatalf("Failed to apply formatting: %v", err)
		}
	}

	fmt.Println("Rich text formatting applied successfully!")
	fmt.Printf("Final Document URL: https://docs.google.com/document/d/%s/edit\n", doc.DocumentId)
}