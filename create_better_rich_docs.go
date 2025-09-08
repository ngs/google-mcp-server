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

type Element struct {
	Text           string
	Type           string // title, heading1, heading2, heading3, paragraph, bullet, numbered, code
	Level          int    // for headings and lists
	Bold           bool
	Code           bool
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
	fmt.Println("Creating Google Docs document with proper formatting...")
	doc, err := docsClient.CreateDocument("Claude Code Instructions for Google MCP Server (Enhanced Formatting)")
	if err != nil {
		log.Fatalf("Failed to create document: %v", err)
	}

	fmt.Printf("Document created successfully!\n")
	fmt.Printf("Document ID: %s\n", doc.DocumentId)
	fmt.Printf("Document URL: https://docs.google.com/document/d/%s/edit\n", doc.DocumentId)

	// Parse markdown content
	fmt.Println("Parsing markdown content...")
	elements := parseMarkdown(string(content))

	// Apply formatting in two phases: first insert all text, then apply formatting
	fmt.Println("Applying enhanced formatting...")
	err = applyEnhancedFormatting(docsClient, doc.DocumentId, elements)
	if err != nil {
		log.Fatalf("Failed to apply formatting: %v", err)
	}

	fmt.Println("Enhanced formatting applied successfully!")
	fmt.Printf("Final Document URL: https://docs.google.com/document/d/%s/edit\n", doc.DocumentId)
}

func parseMarkdown(content string) []Element {
	var elements []Element
	lines := strings.Split(content, "\n")
	
	inCodeBlock := false
	
	for _, line := range lines {
		element := Element{}
		
		// Handle code blocks
		if strings.HasPrefix(line, "```") {
			inCodeBlock = !inCodeBlock
			continue
		}
		
		if inCodeBlock {
			element.Type = "code"
			element.Text = line
			elements = append(elements, element)
			continue
		}
		
		// Handle headings
		if strings.HasPrefix(line, "# ") {
			element.Type = "title"
			element.Text = strings.TrimPrefix(line, "# ")
			element.Bold = true
		} else if strings.HasPrefix(line, "## ") {
			element.Type = "heading1"
			element.Text = strings.TrimPrefix(line, "## ")
			element.Bold = true
			element.Level = 1
		} else if strings.HasPrefix(line, "### ") {
			element.Type = "heading2"
			element.Text = strings.TrimPrefix(line, "### ")
			element.Bold = true
			element.Level = 2
		} else if strings.HasPrefix(line, "#### ") {
			element.Type = "heading3"
			element.Text = strings.TrimPrefix(line, "#### ")
			element.Bold = true
			element.Level = 3
		} else if regexp.MustCompile(`^\d+\.\s+`).MatchString(line) {
			// Numbered list
			element.Type = "numbered"
			re := regexp.MustCompile(`^\d+\.\s+(.*)`)
			matches := re.FindStringSubmatch(line)
			if len(matches) > 1 {
				element.Text = matches[1]
			}
			element.Level = 0 // base level
		} else if strings.HasPrefix(line, "- ") {
			// Bullet list
			element.Type = "bullet"
			element.Text = strings.TrimPrefix(line, "- ")
			element.Level = 0 // base level
		} else if strings.HasPrefix(line, "  - ") {
			// Nested bullet list level 1
			element.Type = "bullet"
			element.Text = strings.TrimPrefix(line, "  - ")
			element.Level = 1
		} else if strings.HasPrefix(line, "    - ") {
			// Nested bullet list level 2
			element.Type = "bullet"
			element.Text = strings.TrimPrefix(line, "    - ")
			element.Level = 2
		} else if strings.TrimSpace(line) == "" {
			// Empty line - create paragraph break
			element.Type = "paragraph"
			element.Text = ""
		} else {
			// Regular paragraph
			element.Type = "paragraph"
			element.Text = line
		}
		
		// Handle bold text within content
		if strings.Contains(element.Text, "**") {
			if strings.Count(element.Text, "**") >= 2 {
				element.Bold = true
				element.Text = strings.ReplaceAll(element.Text, "**", "")
			}
		}
		
		// Handle inline code
		if strings.Contains(element.Text, "`") && strings.Count(element.Text, "`") >= 2 {
			element.Code = true
			element.Text = strings.ReplaceAll(element.Text, "`", "")
		}
		
		elements = append(elements, element)
	}
	
	return elements
}

func applyEnhancedFormatting(docsClient *docs.Client, documentId string, elements []Element) error {
	// First phase: Insert all text content
	var insertRequests []*docsv1.Request
	currentIndex := int64(1)
	
	// Build the complete text content
	var allText strings.Builder
	var elementPositions []struct {
		Element   Element
		StartPos  int64
		EndPos    int64
	}
	
	for _, element := range elements {
		if element.Text == "" && element.Type == "paragraph" {
			// Empty line
			text := "\n"
			allText.WriteString(text)
			elementPositions = append(elementPositions, struct{
				Element   Element
				StartPos  int64
				EndPos    int64
			}{element, currentIndex, currentIndex + int64(len(text))})
			currentIndex += int64(len(text))
			continue
		}
		
		if strings.TrimSpace(element.Text) == "" {
			continue
		}
		
		// For bullet and numbered lists, add bullet symbol or number
		text := element.Text
		if element.Type == "bullet" {
			text = "• " + text
		} else if element.Type == "numbered" {
			// We'll let the user add numbers manually or use Google Docs' numbering
			text = text
		}
		
		text += "\n"
		
		startPos := currentIndex
		allText.WriteString(text)
		endPos := currentIndex + int64(len(text))
		
		elementPositions = append(elementPositions, struct{
			Element   Element
			StartPos  int64
			EndPos    int64
		}{element, startPos, endPos})
		
		currentIndex = endPos
	}
	
	// Insert all text at once
	if allText.Len() > 0 {
		insertRequests = append(insertRequests, &docsv1.Request{
			InsertText: &docsv1.InsertTextRequest{
				Location: &docsv1.Location{Index: 1},
				Text:     allText.String(),
			},
		})
		
		_, err := docsClient.BatchUpdate(documentId, insertRequests)
		if err != nil {
			return fmt.Errorf("failed to insert text: %w", err)
		}
	}
	
	// Second phase: Apply formatting
	var formatRequests []*docsv1.Request
	
	for _, pos := range elementPositions {
		element := pos.Element
		
		if strings.TrimSpace(element.Text) == "" && element.Type == "paragraph" {
			continue // Skip empty paragraphs
		}
		
		// Calculate text range (excluding newline for text styling)
		textStart := pos.StartPos
		textEnd := pos.EndPos - 1 // Exclude newline character from text styling
		paragraphEnd := pos.EndPos // Include newline for paragraph styling
		
		if element.Type == "bullet" {
			textStart += 2 // Skip the "• " prefix
		}
		
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
			indentLevel := float64(element.Level * 18) // 18 points per level
			paragraphStyle = &docsv1.ParagraphStyle{
				IndentFirstLine: &docsv1.Dimension{Magnitude: indentLevel, Unit: "PT"},
				IndentStart:     &docsv1.Dimension{Magnitude: indentLevel + 18, Unit: "PT"},
			}
			
		case "numbered":
			indentLevel := float64(element.Level * 18) // 18 points per level
			paragraphStyle = &docsv1.ParagraphStyle{
				IndentFirstLine: &docsv1.Dimension{Magnitude: indentLevel, Unit: "PT"},
				IndentStart:     &docsv1.Dimension{Magnitude: indentLevel + 18, Unit: "PT"},
			}
			
		case "code":
			textStyle = &docsv1.TextStyle{
				WeightedFontFamily: &docsv1.WeightedFontFamily{
					FontFamily: "Consolas",
					Weight:     400,
				},
				BackgroundColor: &docsv1.OptionalColor{
					Color: &docsv1.Color{
						RgbColor: &docsv1.RgbColor{
							Red:   0.95,
							Green: 0.95,
							Blue:  0.95,
						},
					},
				},
			}
		}
		
		// Apply additional styling for bold text
		if element.Bold && textStyle != nil {
			textStyle.Bold = true
		} else if element.Bold {
			textStyle = &docsv1.TextStyle{Bold: true}
		}
		
		// Apply additional styling for code text
		if element.Code {
			if textStyle != nil {
				textStyle.WeightedFontFamily = &docsv1.WeightedFontFamily{
					FontFamily: "Consolas",
					Weight:     400,
				}
			} else {
				textStyle = &docsv1.TextStyle{
					WeightedFontFamily: &docsv1.WeightedFontFamily{
						FontFamily: "Consolas",
						Weight:     400,
					},
				}
			}
		}
		
		// Add text style request
		if textStyle != nil && textEnd > textStart {
			formatRequests = append(formatRequests, &docsv1.Request{
				UpdateTextStyle: &docsv1.UpdateTextStyleRequest{
					Range: &docsv1.Range{
						StartIndex: textStart,
						EndIndex:   textEnd,
					},
					TextStyle: textStyle,
					Fields:    "*",
				},
			})
		}
		
		// Add paragraph style request
		if paragraphStyle != nil {
			formatRequests = append(formatRequests, &docsv1.Request{
				UpdateParagraphStyle: &docsv1.UpdateParagraphStyleRequest{
					Range: &docsv1.Range{
						StartIndex: pos.StartPos,
						EndIndex:   paragraphEnd,
					},
					ParagraphStyle: paragraphStyle,
					Fields:         "*",
				},
			})
		}
	}
	
	// Execute formatting requests
	if len(formatRequests) > 0 {
		_, err := docsClient.BatchUpdate(documentId, formatRequests)
		if err != nil {
			return fmt.Errorf("failed to apply formatting: %w", err)
		}
	}
	
	return nil
}