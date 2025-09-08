package docs

import (
	"fmt"
	"regexp"
	"strings"

	"google.golang.org/api/docs/v1"
)

// convertBatchRequests converts simplified batch requests to Google Docs API format
func (h *Handler) convertBatchRequests(requests []map[string]interface{}) ([]*docs.Request, error) {
	var docsRequests []*docs.Request

	for _, req := range requests {
		reqType, ok := req["type"].(string)
		if !ok {
			return nil, fmt.Errorf("request type is required")
		}

		switch reqType {
		case "insertText":
			text, _ := req["text"].(string)
			location, _ := req["location"].(float64)

			docsRequests = append(docsRequests, &docs.Request{
				InsertText: &docs.InsertTextRequest{
					Text: text,
					Location: &docs.Location{
						Index: int64(location),
					},
				},
			})

		case "deleteContentRange":
			startIndex, _ := req["startIndex"].(float64)
			endIndex, _ := req["endIndex"].(float64)

			docsRequests = append(docsRequests, &docs.Request{
				DeleteContentRange: &docs.DeleteContentRangeRequest{
					Range: &docs.Range{
						StartIndex: int64(startIndex),
						EndIndex:   int64(endIndex),
					},
				},
			})

		case "updateParagraphStyle":
			startIndex, _ := req["startIndex"].(float64)
			endIndex, _ := req["endIndex"].(float64)

			paragraphStyle := &docs.ParagraphStyle{}
			fields := []string{}

			if style, ok := req["paragraphStyle"].(map[string]interface{}); ok {
				if namedStyle, ok := style["namedStyleType"].(string); ok {
					paragraphStyle.NamedStyleType = namedStyle
					fields = append(fields, "namedStyleType")
				}
				if alignment, ok := style["alignment"].(string); ok {
					paragraphStyle.Alignment = alignment
					fields = append(fields, "alignment")
				}
			}

			docsRequests = append(docsRequests, &docs.Request{
				UpdateParagraphStyle: &docs.UpdateParagraphStyleRequest{
					Range: &docs.Range{
						StartIndex: int64(startIndex),
						EndIndex:   int64(endIndex),
					},
					ParagraphStyle: paragraphStyle,
					Fields:         strings.Join(fields, ","),
				},
			})

		case "updateTextStyle":
			startIndex, _ := req["startIndex"].(float64)
			endIndex, _ := req["endIndex"].(float64)

			textStyle := &docs.TextStyle{}
			fields := []string{}

			if style, ok := req["textStyle"].(map[string]interface{}); ok {
				if bold, ok := style["bold"].(bool); ok {
					textStyle.Bold = bold
					fields = append(fields, "bold")
				}
				if italic, ok := style["italic"].(bool); ok {
					textStyle.Italic = italic
					fields = append(fields, "italic")
				}
				if underline, ok := style["underline"].(bool); ok {
					textStyle.Underline = underline
					fields = append(fields, "underline")
				}
				if fontSize, ok := style["fontSize"].(float64); ok {
					textStyle.FontSize = &docs.Dimension{
						Magnitude: fontSize,
						Unit:      "PT",
					}
					fields = append(fields, "fontSize")
				}
			}

			docsRequests = append(docsRequests, &docs.Request{
				UpdateTextStyle: &docs.UpdateTextStyleRequest{
					Range: &docs.Range{
						StartIndex: int64(startIndex),
						EndIndex:   int64(endIndex),
					},
					TextStyle: textStyle,
					Fields:    strings.Join(fields, ","),
				},
			})

		default:
			return nil, fmt.Errorf("unknown request type: %s", reqType)
		}
	}

	return docsRequests, nil
}

// markdownToDocsRequests converts markdown content to Google Docs API requests
func (h *Handler) markdownToDocsRequests(documentID string, markdown string, mode string) ([]*docs.Request, error) {
	var requests []*docs.Request
	var currentIndex int64 = 1

	// If replace mode, first clear the document
	if mode == "replace" {
		doc, err := h.client.GetDocument(documentID)
		if err != nil {
			return nil, fmt.Errorf("failed to get document for replacement: %w", err)
		}

		// Find the end index of the document content
		endIndex := int64(1)
		if doc.Body != nil && doc.Body.Content != nil && len(doc.Body.Content) > 0 {
			lastElement := doc.Body.Content[len(doc.Body.Content)-1]
			if lastElement.EndIndex > 0 {
				endIndex = lastElement.EndIndex - 1
			}
		}

		// Delete existing content if any
		if endIndex > 1 {
			requests = append(requests, &docs.Request{
				DeleteContentRange: &docs.DeleteContentRangeRequest{
					Range: &docs.Range{
						StartIndex: 1,
						EndIndex:   endIndex,
					},
				},
			})
		}
	} else {
		// Append mode: find where to append
		doc, err := h.client.GetDocument(documentID)
		if err != nil {
			return nil, fmt.Errorf("failed to get document for appending: %w", err)
		}

		if doc.Body != nil && doc.Body.Content != nil && len(doc.Body.Content) > 0 {
			lastElement := doc.Body.Content[len(doc.Body.Content)-1]
			if lastElement.EndIndex > 0 {
				currentIndex = lastElement.EndIndex - 1
			}
		}
	}

	// Parse markdown and create formatted requests
	lines := strings.Split(markdown, "\n")
	var pendingRequests []pendingRequest
	var inCodeBlock bool
	var codeBlockContent strings.Builder

	for _, line := range lines {
		// Handle code blocks
		if strings.HasPrefix(line, "```") {
			if inCodeBlock {
				// End of code block
				codeText := codeBlockContent.String() + "\n"
				requests = append(requests, &docs.Request{
					InsertText: &docs.InsertTextRequest{
						Text: codeText,
						Location: &docs.Location{
							Index: currentIndex,
						},
					},
				})

				// Apply code block styling
				pendingRequests = append(pendingRequests, pendingRequest{
					startIndex: currentIndex,
					endIndex:   currentIndex + int64(len(codeText)),
					lineType:   "codeBlock",
				})

				currentIndex += int64(len(codeText))
				inCodeBlock = false
				codeBlockContent.Reset()
			} else {
				// Start of code block
				inCodeBlock = true
			}
			continue
		}

		if inCodeBlock {
			// Inside code block
			if codeBlockContent.Len() > 0 {
				codeBlockContent.WriteString("\n")
			}
			codeBlockContent.WriteString(line)
		} else {
			// Process normal markdown line
			parsed := parseMarkdownLine(line)

			// Handle inline formatting
			if parsed.lineType == "normal" || parsed.lineType == "list" || parsed.lineType == "numberedList" {
				// Process inline code and other formatting
				textParts := processInlineFormatting(parsed.text)
				for _, part := range textParts {
					requests = append(requests, &docs.Request{
						InsertText: &docs.InsertTextRequest{
							Text: part.text,
							Location: &docs.Location{
								Index: currentIndex,
							},
						},
					})

					if part.style != nil {
						pendingRequests = append(pendingRequests, pendingRequest{
							startIndex: currentIndex,
							endIndex:   currentIndex + int64(len(part.text)),
							lineType:   "inline",
							style:      part.style,
						})
					}

					currentIndex += int64(len(part.text))
				}

				// Add newline
				requests = append(requests, &docs.Request{
					InsertText: &docs.InsertTextRequest{
						Text: "\n",
						Location: &docs.Location{
							Index: currentIndex,
						},
					},
				})

				// Apply paragraph-level styling
				if parsed.lineType == "list" || parsed.lineType == "numberedList" {
					pendingRequests = append(pendingRequests, pendingRequest{
						startIndex: currentIndex,
						endIndex:   currentIndex + 1,
						lineType:   parsed.lineType,
					})
				}

				currentIndex++
			} else {
				// Handle headers and other block elements
				text := parsed.text
				if text != "" || parsed.lineType == "blank" {
					if parsed.lineType != "blank" {
						text += "\n"
					} else {
						text = "\n"
					}

					requests = append(requests, &docs.Request{
						InsertText: &docs.InsertTextRequest{
							Text: text,
							Location: &docs.Location{
								Index: currentIndex,
							},
						},
					})

					// Store formatting to apply after text insertion
					if parsed.lineType != "normal" && parsed.lineType != "blank" {
						pendingRequests = append(pendingRequests, pendingRequest{
							startIndex: currentIndex,
							endIndex:   currentIndex + int64(len(text)),
							lineType:   parsed.lineType,
							style:      parsed.style,
						})
					}

					currentIndex += int64(len(text))
				}
			}
		}
	}

	// Apply all formatting after text insertion
	for _, pending := range pendingRequests {
		requests = append(requests, createStyleRequests(pending)...)
	}

	return requests, nil
}

type parsedLine struct {
	text     string
	lineType string
	style    map[string]interface{}
}

type pendingRequest struct {
	startIndex int64
	endIndex   int64
	lineType   string
	style      map[string]interface{}
}

type textPart struct {
	text  string
	style map[string]interface{}
}

func parseMarkdownLine(line string) parsedLine {
	// Check for headers
	if match := regexp.MustCompile(`^# (.*)`).FindStringSubmatch(line); match != nil {
		return parsedLine{
			text:     match[1],
			lineType: "heading1",
		}
	}
	if match := regexp.MustCompile(`^## (.*)`).FindStringSubmatch(line); match != nil {
		return parsedLine{
			text:     match[1],
			lineType: "heading2",
		}
	}
	if match := regexp.MustCompile(`^### (.*)`).FindStringSubmatch(line); match != nil {
		return parsedLine{
			text:     match[1],
			lineType: "heading3",
		}
	}
	if match := regexp.MustCompile(`^#### (.*)`).FindStringSubmatch(line); match != nil {
		return parsedLine{
			text:     match[1],
			lineType: "heading4",
		}
	}
	if match := regexp.MustCompile(`^##### (.*)`).FindStringSubmatch(line); match != nil {
		return parsedLine{
			text:     match[1],
			lineType: "heading5",
		}
	}
	if match := regexp.MustCompile(`^###### (.*)`).FindStringSubmatch(line); match != nil {
		return parsedLine{
			text:     match[1],
			lineType: "heading6",
		}
	}

	// Check for list items
	if match := regexp.MustCompile(`^[-*] (.*)`).FindStringSubmatch(line); match != nil {
		return parsedLine{
			text:     "• " + match[1],
			lineType: "list",
		}
	}
	if match := regexp.MustCompile(`^(\d+)\. (.*)`).FindStringSubmatch(line); match != nil {
		return parsedLine{
			text:     match[1] + ". " + match[2],
			lineType: "numberedList",
		}
	}

	// Check for code blocks (simplified - just indent them)
	if strings.HasPrefix(line, "```") {
		return parsedLine{
			text:     "",
			lineType: "codeDelimiter",
		}
	}

	// Check for blank lines
	if strings.TrimSpace(line) == "" {
		return parsedLine{
			text:     "",
			lineType: "blank",
		}
	}

	// Normal text with inline formatting
	return parsedLine{
		text:     line,
		lineType: "normal",
	}
}

func createStyleRequests(pending pendingRequest) []*docs.Request {
	var requests []*docs.Request

	switch pending.lineType {
	case "heading1":
		requests = append(requests, &docs.Request{
			UpdateParagraphStyle: &docs.UpdateParagraphStyleRequest{
				Range: &docs.Range{
					StartIndex: pending.startIndex,
					EndIndex:   pending.endIndex,
				},
				ParagraphStyle: &docs.ParagraphStyle{
					NamedStyleType: "TITLE",
				},
				Fields: "namedStyleType",
			},
		})

	case "heading2":
		requests = append(requests, &docs.Request{
			UpdateParagraphStyle: &docs.UpdateParagraphStyleRequest{
				Range: &docs.Range{
					StartIndex: pending.startIndex,
					EndIndex:   pending.endIndex,
				},
				ParagraphStyle: &docs.ParagraphStyle{
					NamedStyleType: "HEADING_1",
				},
				Fields: "namedStyleType",
			},
		})

	case "heading3":
		requests = append(requests, &docs.Request{
			UpdateParagraphStyle: &docs.UpdateParagraphStyleRequest{
				Range: &docs.Range{
					StartIndex: pending.startIndex,
					EndIndex:   pending.endIndex,
				},
				ParagraphStyle: &docs.ParagraphStyle{
					NamedStyleType: "HEADING_2",
				},
				Fields: "namedStyleType",
			},
		})

	case "heading4":
		requests = append(requests, &docs.Request{
			UpdateParagraphStyle: &docs.UpdateParagraphStyleRequest{
				Range: &docs.Range{
					StartIndex: pending.startIndex,
					EndIndex:   pending.endIndex,
				},
				ParagraphStyle: &docs.ParagraphStyle{
					NamedStyleType: "HEADING_3",
				},
				Fields: "namedStyleType",
			},
		})

	case "heading5":
		requests = append(requests, &docs.Request{
			UpdateParagraphStyle: &docs.UpdateParagraphStyleRequest{
				Range: &docs.Range{
					StartIndex: pending.startIndex,
					EndIndex:   pending.endIndex,
				},
				ParagraphStyle: &docs.ParagraphStyle{
					NamedStyleType: "HEADING_4",
				},
				Fields: "namedStyleType",
			},
		})

	case "heading6":
		requests = append(requests, &docs.Request{
			UpdateParagraphStyle: &docs.UpdateParagraphStyleRequest{
				Range: &docs.Range{
					StartIndex: pending.startIndex,
					EndIndex:   pending.endIndex,
				},
				ParagraphStyle: &docs.ParagraphStyle{
					NamedStyleType: "HEADING_5",
				},
				Fields: "namedStyleType",
			},
		})

	case "codeBlock":
		// Apply monospace font and background color for code blocks
		requests = append(requests, &docs.Request{
			UpdateTextStyle: &docs.UpdateTextStyleRequest{
				Range: &docs.Range{
					StartIndex: pending.startIndex,
					EndIndex:   pending.endIndex,
				},
				TextStyle: &docs.TextStyle{
					WeightedFontFamily: &docs.WeightedFontFamily{
						FontFamily: "Courier New",
						Weight:     400,
					},
					FontSize: &docs.Dimension{
						Magnitude: 10,
						Unit:      "PT",
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
				},
				Fields: "weightedFontFamily,fontSize,backgroundColor",
			},
		})

	case "inline":
		// Apply inline formatting (bold, italic, code)
		if pending.style != nil {
			textStyle := &docs.TextStyle{}
			fields := []string{}

			if bold, ok := pending.style["bold"].(bool); ok && bold {
				textStyle.Bold = true
				fields = append(fields, "bold")
			}

			if italic, ok := pending.style["italic"].(bool); ok && italic {
				textStyle.Italic = true
				fields = append(fields, "italic")
			}

			if code, ok := pending.style["code"].(bool); ok && code {
				textStyle.WeightedFontFamily = &docs.WeightedFontFamily{
					FontFamily: "Courier New",
					Weight:     400,
				}
				textStyle.BackgroundColor = &docs.OptionalColor{
					Color: &docs.Color{
						RgbColor: &docs.RgbColor{
							Red:   0.95,
							Green: 0.95,
							Blue:  0.95,
						},
					},
				}
				fields = append(fields, "weightedFontFamily", "backgroundColor")
			}

			if len(fields) > 0 {
				requests = append(requests, &docs.Request{
					UpdateTextStyle: &docs.UpdateTextStyleRequest{
						Range: &docs.Range{
							StartIndex: pending.startIndex,
							EndIndex:   pending.endIndex,
						},
						TextStyle: textStyle,
						Fields:    strings.Join(fields, ","),
					},
				})
			}
		}
	}

	return requests
}

// processInlineFormatting processes inline markdown formatting (bold, italic, code)
func processInlineFormatting(text string) []textPart {
	var parts []textPart
	var currentPos int

	// Regular expressions for inline formatting
	inlineCodeRegex := regexp.MustCompile("`([^`]+)`")
	boldRegex := regexp.MustCompile(`\*\*([^*]+)\*\*`)
	italicRegex := regexp.MustCompile(`\*([^*]+)\*`)

	// Process inline code first (highest priority)
	for {
		match := inlineCodeRegex.FindStringSubmatchIndex(text[currentPos:])
		if match == nil {
			break
		}

		// Add text before the match
		if match[0] > 0 {
			parts = append(parts, textPart{
				text: text[currentPos : currentPos+match[0]],
			})
		}

		// Add the code text with styling
		parts = append(parts, textPart{
			text: text[currentPos+match[2] : currentPos+match[3]],
			style: map[string]interface{}{
				"code": true,
			},
		})

		currentPos += match[1]
	}

	// Add remaining text
	if currentPos < len(text) {
		remainingText := text[currentPos:]

		// Process bold and italic in the remaining text
		processedText := processFormattingInText(remainingText, boldRegex, "bold")
		for _, part := range processedText {
			if part.style == nil {
				// Check for italic
				italicParts := processFormattingInText(part.text, italicRegex, "italic")
				parts = append(parts, italicParts...)
			} else {
				parts = append(parts, part)
			}
		}
	}

	// If no parts were created, return the original text
	if len(parts) == 0 {
		parts = append(parts, textPart{text: text})
	}

	return parts
}

// processFormattingInText processes a single type of formatting in text
func processFormattingInText(text string, regex *regexp.Regexp, styleType string) []textPart {
	var parts []textPart
	var currentPos int

	for {
		match := regex.FindStringSubmatchIndex(text[currentPos:])
		if match == nil {
			break
		}

		// Add text before the match
		if match[0] > 0 {
			parts = append(parts, textPart{
				text: text[currentPos : currentPos+match[0]],
			})
		}

		// Add the formatted text with styling
		parts = append(parts, textPart{
			text: text[currentPos+match[2] : currentPos+match[3]],
			style: map[string]interface{}{
				styleType: true,
			},
		})

		currentPos += match[1]
	}

	// Add remaining text
	if currentPos < len(text) {
		parts = append(parts, textPart{
			text: text[currentPos:],
		})
	}

	// If no parts were created, return the original text
	if len(parts) == 0 {
		parts = append(parts, textPart{text: text})
	}

	return parts
}
