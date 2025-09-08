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

type ContentElement struct {
	Text       string
	Type       string // "title", "heading1", "heading2", "heading3", "paragraph", "bullet", "numbered", "code", "bold"
	Level      int    // for lists and headings
	IsCode     bool
	IsBold     bool
}

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
	fmt.Println("Creating Google Docs document with rich text formatting...")
	doc, err := docsClient.CreateDocument("Claude Code Instructions for Google MCP Server")
	if err != nil {
		log.Fatalf("Failed to create document: %v", err)
	}

	fmt.Printf("Document created successfully!\n")
	fmt.Printf("Document ID: %s\n", doc.DocumentId)
	fmt.Printf("Document URL: https://docs.google.com/document/d/%s/edit\n", doc.DocumentId)

	// Parse and apply rich text formatting
	fmt.Println("Applying rich text formatting...")
	err = applyRichTextFormatting(docsClient, doc.DocumentId, string(content))
	if err != nil {
		log.Fatalf("Failed to apply formatting: %v", err)
	}

	fmt.Println("Rich text formatting applied successfully!")
	fmt.Printf("Final Document URL: https://docs.google.com/document/d/%s/edit\n", doc.DocumentId)
}

func applyRichTextFormatting(docsClient *docs.Client, documentId string, content string) error {
	// Parse content into structured elements
	elements := parseContentElements(content)
	
	// Build the text content first
	var textBuilder strings.Builder
	var elementMap []struct {
		Element ContentElement
		Start   int64
		End     int64
	}
	
	position := int64(0)
	
	for _, element := range elements {
		if element.Text == "" {
			// Empty line
			text := "\n"
			textBuilder.WriteString(text)
			start := position
			position += int64(len(text))
			elementMap = append(elementMap, struct {
				Element ContentElement
				Start   int64
				End     int64
			}{element, start, position})
			continue
		}
		
		text := element.Text + "\n"
		textBuilder.WriteString(text)
		start := position
		position += int64(len(text))
		elementMap = append(elementMap, struct {
			Element ContentElement
			Start   int64
			End     int64
		}{element, start, position})
	}
	
	// Insert all text content
	allText := textBuilder.String()
	if allText != "" {
		_, err := docsClient.UpdateDocument(documentId, allText, "replace")
		if err != nil {
			return fmt.Errorf("failed to insert text: %w", err)
		}
	}
	
	// Apply formatting in small, safe batches
	return applyFormattingInBatches(docsClient, documentId, elementMap)
}

func parseContentElements(content string) []ContentElement {
	var elements []ContentElement
	lines := strings.Split(content, "\n")
	
	inCodeBlock := false
	
	for _, line := range lines {
		element := ContentElement{}
		
		// Handle code blocks
		if strings.HasPrefix(line, "```") {
			inCodeBlock = !inCodeBlock
			if line != "```" {
				element.Type = "code"
				element.Text = "Code: " + strings.TrimPrefix(line, "```")
				element.IsCode = true
			} else {
				continue
			}
		} else if inCodeBlock {
			element.Type = "code"
			element.Text = line
			element.IsCode = true
		} else if strings.HasPrefix(line, "# ") {
			element.Type = "title"
			element.Text = strings.TrimPrefix(line, "# ")
			element.IsBold = true
		} else if strings.HasPrefix(line, "## ") {
			element.Type = "heading1"
			element.Text = strings.TrimPrefix(line, "## ")
			element.Level = 1
			element.IsBold = true
		} else if strings.HasPrefix(line, "### ") {
			element.Type = "heading2"
			element.Text = strings.TrimPrefix(line, "### ")
			element.Level = 2
			element.IsBold = true
		} else if strings.HasPrefix(line, "#### ") {
			element.Type = "heading3"
			element.Text = strings.TrimPrefix(line, "#### ")
			element.Level = 3
			element.IsBold = true
		} else if strings.HasPrefix(line, "- ") {
			element.Type = "bullet"
			element.Text = "• " + strings.TrimPrefix(line, "- ")
			element.Level = 0
		} else if strings.HasPrefix(line, "  - ") {
			element.Type = "bullet"
			element.Text = "  ◦ " + strings.TrimPrefix(line, "  - ")
			element.Level = 1
		} else if regexp.MustCompile(`^\d+\.\s+`).MatchString(line) {
			element.Type = "numbered"
			element.Text = line
			element.Level = 0
		} else if strings.TrimSpace(line) == "" {
			element.Type = "paragraph"
			element.Text = ""
		} else {
			element.Type = "paragraph"
			element.Text = line
		}
		
		// Handle bold text
		if strings.Contains(element.Text, "**") {
			element.IsBold = true
			element.Text = strings.ReplaceAll(element.Text, "**", "")
		}
		
		// Handle inline code
		if strings.Contains(element.Text, "`") {
			element.IsCode = true
			element.Text = strings.ReplaceAll(element.Text, "`", "")
		}
		
		elements = append(elements, element)
	}
	
	return elements
}

func applyFormattingInBatches(docsClient *docs.Client, documentId string, elementMap []struct {
	Element ContentElement
	Start   int64
	End     int64
}) error {
	
	var requests []*docsv1.Request
	batchSize := 5 // Small batch size to avoid API errors
	
	for _, item := range elementMap {
		element := item.Element
		
		if element.Text == "" {
			continue
		}
		
		// Calculate safe text bounds (exclude newline from text styling)
		textStart := item.Start + 1 // Move to position 1-based indexing
		textEnd := item.End        // End position for text (includes newline)
		textStyleEnd := textEnd - 1 // Exclude newline from text styling
		
		if textStyleEnd <= textStart {
			continue
		}
		
		// Determine styling
		var textStyle *docsv1.TextStyle
		var paragraphStyle *docsv1.ParagraphStyle
		
		switch element.Type {
		case "title":
			textStyle = &docsv1.TextStyle{
				Bold:     true,
				FontSize: &docsv1.Dimension{Magnitude: 20, Unit: "PT"},
			}
			paragraphStyle = &docsv1.ParagraphStyle{
				NamedStyleType: "TITLE",
			}
			
		case "heading1":
			textStyle = &docsv1.TextStyle{
				Bold:     true,
				FontSize: &docsv1.Dimension{Magnitude: 16, Unit: "PT"},
			}
			paragraphStyle = &docsv1.ParagraphStyle{
				NamedStyleType: "HEADING_1",
			}
			
		case "heading2":
			textStyle = &docsv1.TextStyle{
				Bold:     true,
				FontSize: &docsv1.Dimension{Magnitude: 14, Unit: "PT"},
			}
			paragraphStyle = &docsv1.ParagraphStyle{
				NamedStyleType: "HEADING_2",
			}
			
		case "heading3":
			textStyle = &docsv1.TextStyle{
				Bold:     true,
				FontSize: &docsv1.Dimension{Magnitude: 12, Unit: "PT"},
			}
			paragraphStyle = &docsv1.ParagraphStyle{
				NamedStyleType: "HEADING_3",
			}
			
		case "bullet":
			indentLevel := float64(element.Level * 18)
			paragraphStyle = &docsv1.ParagraphStyle{
				IndentFirstLine: &docsv1.Dimension{Magnitude: indentLevel, Unit: "PT"},
				IndentStart:     &docsv1.Dimension{Magnitude: indentLevel + 18, Unit: "PT"},
			}
			
		case "numbered":
			paragraphStyle = &docsv1.ParagraphStyle{
				IndentFirstLine: &docsv1.Dimension{Magnitude: 0, Unit: "PT"},
				IndentStart:     &docsv1.Dimension{Magnitude: 18, Unit: "PT"},
			}
			
		case "code":
			textStyle = &docsv1.TextStyle{
				WeightedFontFamily: &docsv1.WeightedFontFamily{
					FontFamily: "Courier New",
					Weight:     400,
				},
				BackgroundColor: &docsv1.OptionalColor{
					Color: &docsv1.Color{
						RgbColor: &docsv1.RgbColor{
							Red:   0.96,
							Green: 0.96,
							Blue:  0.96,
						},
					},
				},
			}
		}
		
		// Apply bold or code styling
		if element.IsBold && textStyle != nil {
			textStyle.Bold = true
		} else if element.IsBold {
			textStyle = &docsv1.TextStyle{Bold: true}
		}
		
		if element.IsCode && textStyle != nil {
			textStyle.WeightedFontFamily = &docsv1.WeightedFontFamily{
				FontFamily: "Courier New",
				Weight:     400,
			}
		} else if element.IsCode {
			textStyle = &docsv1.TextStyle{
				WeightedFontFamily: &docsv1.WeightedFontFamily{
					FontFamily: "Courier New",
					Weight:     400,
				},
			}
		}
		
		// Add text style request
		if textStyle != nil {
			requests = append(requests, &docsv1.Request{
				UpdateTextStyle: &docsv1.UpdateTextStyleRequest{
					Range: &docsv1.Range{
						StartIndex: textStart,
						EndIndex:   textStyleEnd,
					},
					TextStyle: textStyle,
					Fields:    "*",
				},
			})
		}
		
		// Add paragraph style request
		if paragraphStyle != nil {
			requests = append(requests, &docsv1.Request{
				UpdateParagraphStyle: &docsv1.UpdateParagraphStyleRequest{
					Range: &docsv1.Range{
						StartIndex: textStart,
						EndIndex:   textEnd,
					},
					ParagraphStyle: paragraphStyle,
					Fields:         "*",
				},
			})
		}
		
		// Execute batch when we reach batch size
		if len(requests) >= batchSize {
			_, err := docsClient.BatchUpdate(documentId, requests[:batchSize])
			if err != nil {
				log.Printf("Warning: Failed to apply formatting batch: %v", err)
			}
			requests = requests[batchSize:]
		}
	}
	
	// Execute remaining requests
	if len(requests) > 0 {
		_, err := docsClient.BatchUpdate(documentId, requests)
		if err != nil {
			log.Printf("Warning: Failed to apply final formatting batch: %v", err)
		}
	}
	
	return nil
}