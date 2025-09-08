package docs

import (
	"fmt"
	"regexp"
	"strings"

	"google.golang.org/api/docs/v1"
)

// MarkdownConverter converts Markdown text to Google Docs structured requests
type MarkdownConverter struct {
	currentIndex int64
	numberedListRegex *regexp.Regexp
}

// NewMarkdownConverter creates a new MarkdownConverter instance
func NewMarkdownConverter(startIndex int64) *MarkdownConverter {
	return &MarkdownConverter{
		currentIndex: startIndex,
		numberedListRegex: regexp.MustCompile(`^\d+\. `),
	}
}

// ConvertToRequests converts markdown text to Google Docs BatchUpdate requests
func (mc *MarkdownConverter) ConvertToRequests(markdown string) []*docs.Request {
	requests := make([]*docs.Request, 0)

	// Handle empty input
	if markdown == "" {
		return requests
	}

	// Process the markdown line by line to handle different elements
	lines := strings.Split(markdown, "\n")
	previousWasHeading := false
	previousWasList := false

	for i, line := range lines {
		// Skip empty lines at the beginning of processing
		if i == 0 && strings.TrimSpace(line) == "" && len(lines) > 1 {
			continue
		}

		lineIsHeading := false
		lineIsList := false
		isHeading := strings.HasPrefix(strings.TrimSpace(line), "#")
		trimmedLine := strings.TrimSpace(line)
		isList := strings.HasPrefix(trimmedLine, "- ") || strings.HasPrefix(trimmedLine, "* ") ||
			regexp.MustCompile(`^\d+\. `).MatchString(trimmedLine)

		// Add newline before line if not the first one
		// Don't add for headings or lists as they handle their own
		shouldAddNewline := i > 0 && !isHeading && !isList
		if shouldAddNewline {
			line = "\n" + line
		}

		// Process different markdown elements
		if strings.HasPrefix(strings.TrimSpace(line), "# ") {
			// Always reset style before heading if coming from another style
			if previousWasHeading || previousWasList {
				requests = append(requests, mc.insertStyleBreak()...)
			}
			requests = append(requests, mc.createHeadingRequests(line, 1)...)
			lineIsHeading = true
		} else if strings.HasPrefix(strings.TrimSpace(line), "## ") {
			if previousWasHeading || previousWasList {
				requests = append(requests, mc.insertStyleBreak()...)
			}
			requests = append(requests, mc.createHeadingRequests(line, 2)...)
			lineIsHeading = true
		} else if strings.HasPrefix(strings.TrimSpace(line), "### ") {
			if previousWasHeading || previousWasList {
				requests = append(requests, mc.insertStyleBreak()...)
			}
			requests = append(requests, mc.createHeadingRequests(line, 3)...)
			lineIsHeading = true
		} else if strings.HasPrefix(strings.TrimSpace(line), "#### ") {
			if previousWasHeading || previousWasList {
				requests = append(requests, mc.insertStyleBreak()...)
			}
			requests = append(requests, mc.createHeadingRequests(line, 4)...)
			lineIsHeading = true
		} else if strings.HasPrefix(strings.TrimSpace(line), "##### ") {
			if previousWasHeading || previousWasList {
				requests = append(requests, mc.insertStyleBreak()...)
			}
			requests = append(requests, mc.createHeadingRequests(line, 5)...)
			lineIsHeading = true
		} else if strings.HasPrefix(strings.TrimSpace(line), "###### ") {
			if previousWasHeading || previousWasList {
				requests = append(requests, mc.insertStyleBreak()...)
			}
			requests = append(requests, mc.createHeadingRequests(line, 6)...)
			lineIsHeading = true
		} else if strings.HasPrefix(strings.TrimSpace(line), "- ") || strings.HasPrefix(strings.TrimSpace(line), "* ") {
			if previousWasHeading {
				requests = append(requests, mc.insertStyleBreak()...)
			}
			requests = append(requests, mc.createBulletListRequests(line)...)
			lineIsList = true
		} else if mc.numberedListRegex.MatchString(strings.TrimSpace(line)) {
			if previousWasHeading {
				requests = append(requests, mc.insertStyleBreak()...)
			}
			requests = append(requests, mc.createNumberedListRequests(line)...)
			lineIsList = true
		} else if strings.HasPrefix(strings.TrimSpace(line), "> ") {
			if previousWasHeading || previousWasList {
				requests = append(requests, mc.insertStyleBreak()...)
			}
			requests = append(requests, mc.createBlockquoteRequests(line)...)
		} else if strings.HasPrefix(strings.TrimSpace(line), "```") {
			if previousWasHeading || previousWasList {
				requests = append(requests, mc.insertStyleBreak()...)
			}
			// Handle code blocks - for simplicity, treat as plain text with monospace font
			requests = append(requests, mc.createCodeBlockRequests(line)...)
		} else {
			// Regular paragraph with inline formatting
			if previousWasHeading || previousWasList {
				requests = append(requests, mc.createNormalParagraphRequests(line)...)
			} else {
				requests = append(requests, mc.createParagraphRequests(line)...)
			}
		}

		previousWasHeading = lineIsHeading
		previousWasList = lineIsList
	}

	return requests
}

// insertStyleBreak inserts a paragraph break to reset styling
func (mc *MarkdownConverter) insertStyleBreak() []*docs.Request {
	var requests []*docs.Request
	
	// Insert a newline
	requests = append(requests, &docs.Request{
		InsertText: &docs.InsertTextRequest{
			Location: &docs.Location{Index: mc.currentIndex},
			Text:     "\n",
		},
	})
	mc.currentIndex += 1
	
	// Reset to normal text style
	requests = append(requests, &docs.Request{
		UpdateParagraphStyle: &docs.UpdateParagraphStyleRequest{
			Range: &docs.Range{
				StartIndex: mc.currentIndex - 1,
				EndIndex:   mc.currentIndex,
			},
			ParagraphStyle: &docs.ParagraphStyle{
				NamedStyleType: "NORMAL_TEXT",
			},
			Fields: "namedStyleType",
		},
	})
	
	return requests
}

// createHeadingRequests creates requests for heading elements
func (mc *MarkdownConverter) createHeadingRequests(line string, level int) []*docs.Request {
	var requests []*docs.Request

	// Extract heading text (remove # markers)
	headingText := strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(line), "#"))
	
	// Add newline before if needed
	if mc.currentIndex > 1 {
		requests = append(requests, &docs.Request{
			InsertText: &docs.InsertTextRequest{
				Location: &docs.Location{Index: mc.currentIndex},
				Text:     "\n",
			},
		})
		mc.currentIndex += 1
	}

	startIndex := mc.currentIndex

	// Insert the heading text
	requests = append(requests, &docs.Request{
		InsertText: &docs.InsertTextRequest{
			Location: &docs.Location{Index: mc.currentIndex},
			Text:     headingText,
		},
	})

	mc.currentIndex += int64(len(headingText))

	// Apply heading style to the heading text only
	namedStyleType := fmt.Sprintf("HEADING_%d", level)
	requests = append(requests, &docs.Request{
		UpdateParagraphStyle: &docs.UpdateParagraphStyleRequest{
			Range: &docs.Range{
				StartIndex: startIndex,
				EndIndex:   mc.currentIndex,
			},
			ParagraphStyle: &docs.ParagraphStyle{
				NamedStyleType: namedStyleType,
			},
			Fields: "namedStyleType",
		},
	})

	return requests
}

// createBulletListRequests creates requests for bullet list items
func (mc *MarkdownConverter) createBulletListRequests(line string) []*docs.Request {
	var requests []*docs.Request

	// Add newline if not the first item in document
	if mc.currentIndex > 1 {
		requests = append(requests, &docs.Request{
			InsertText: &docs.InsertTextRequest{
				Location: &docs.Location{Index: mc.currentIndex},
				Text:     "\n",
			},
		})
		mc.currentIndex += 1
	}

	// Extract list item text (remove - or * markers)
	listText := strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(line), "-*"))

	startIndex := mc.currentIndex

	// Insert the list item text
	requests = append(requests, &docs.Request{
		InsertText: &docs.InsertTextRequest{
			Location: &docs.Location{Index: mc.currentIndex},
			Text:     listText,
		},
	})

	mc.currentIndex += int64(len(listText))

	// Apply bullet list style
	requests = append(requests, &docs.Request{
		CreateParagraphBullets: &docs.CreateParagraphBulletsRequest{
			Range: &docs.Range{
				StartIndex: startIndex,
				EndIndex:   mc.currentIndex,
			},
			BulletPreset: "BULLET_DISC_CIRCLE_SQUARE",
		},
	})

	return requests
}

// createNumberedListRequests creates requests for numbered list items
func (mc *MarkdownConverter) createNumberedListRequests(line string) []*docs.Request {
	var requests []*docs.Request

	// Add newline if not the first item in document
	if mc.currentIndex > 1 {
		requests = append(requests, &docs.Request{
			InsertText: &docs.InsertTextRequest{
				Location: &docs.Location{Index: mc.currentIndex},
				Text:     "\n",
			},
		})
		mc.currentIndex += 1
	}

	// Extract list item text (remove number and dot)
	re := regexp.MustCompile(`^\d+\.\s*`)
	listText := re.ReplaceAllString(strings.TrimSpace(line), "")

	startIndex := mc.currentIndex

	// Insert the list item text
	requests = append(requests, &docs.Request{
		InsertText: &docs.InsertTextRequest{
			Location: &docs.Location{Index: mc.currentIndex},
			Text:     listText,
		},
	})

	mc.currentIndex += int64(len(listText))

	// Apply numbered list style
	requests = append(requests, &docs.Request{
		CreateParagraphBullets: &docs.CreateParagraphBulletsRequest{
			Range: &docs.Range{
				StartIndex: startIndex,
				EndIndex:   mc.currentIndex,
			},
			BulletPreset: "NUMBERED_DECIMAL_ALPHA_ROMAN",
		},
	})

	return requests
}

// createBlockquoteRequests creates requests for blockquote elements
func (mc *MarkdownConverter) createBlockquoteRequests(line string) []*docs.Request {
	var requests []*docs.Request

	// Extract blockquote text (remove > marker)
	quoteText := strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(line), ">"))
	if strings.HasPrefix(line, "\n") {
		quoteText = "\n" + quoteText
	}

	startIndex := mc.currentIndex

	// Insert the blockquote text
	requests = append(requests, &docs.Request{
		InsertText: &docs.InsertTextRequest{
			Location: &docs.Location{Index: mc.currentIndex},
			Text:     quoteText,
		},
	})

	mc.currentIndex += int64(len(quoteText))

	// Apply blockquote style (indent)
	requests = append(requests, &docs.Request{
		UpdateParagraphStyle: &docs.UpdateParagraphStyleRequest{
			Range: &docs.Range{
				StartIndex: startIndex,
				EndIndex:   mc.currentIndex,
			},
			ParagraphStyle: &docs.ParagraphStyle{
				IndentStart: &docs.Dimension{
					Magnitude: 36, // 0.5 inch
					Unit:      "PT",
				},
				SpacingMode: "NEVER_COLLAPSE",
			},
			Fields: "indentStart,spacingMode",
		},
	})

	// Make text italic - ensure it applies to the actual text
	requests = append(requests, &docs.Request{
		UpdateTextStyle: &docs.UpdateTextStyleRequest{
			Range: &docs.Range{
				StartIndex: startIndex,
				EndIndex:   mc.currentIndex,
			},
			TextStyle: &docs.TextStyle{
				Italic: true,
			},
			Fields: "italic",
		},
	})

	// Add a newline to end the blockquote
	requests = append(requests, &docs.Request{
		InsertText: &docs.InsertTextRequest{
			Location: &docs.Location{Index: mc.currentIndex},
			Text:     "\n",
		},
	})
	mc.currentIndex += 1

	return requests
}

// createCodeBlockRequests creates requests for code block elements
func (mc *MarkdownConverter) createCodeBlockRequests(line string) []*docs.Request {
	var requests []*docs.Request

	// For code block markers, just insert them as-is with monospace font
	startIndex := mc.currentIndex

	// Insert the code block text
	requests = append(requests, &docs.Request{
		InsertText: &docs.InsertTextRequest{
			Location: &docs.Location{Index: mc.currentIndex},
			Text:     line,
		},
	})

	mc.currentIndex += int64(len(line))

	// Apply monospace font
	requests = append(requests, &docs.Request{
		UpdateTextStyle: &docs.UpdateTextStyleRequest{
			Range: &docs.Range{
				StartIndex: startIndex,
				EndIndex:   mc.currentIndex,
			},
			TextStyle: &docs.TextStyle{
				WeightedFontFamily: &docs.WeightedFontFamily{
					FontFamily: "Consolas",
				},
			},
			Fields: "weightedFontFamily",
		},
	})

	return requests
}

// createParagraphRequests creates requests for regular paragraphs with inline formatting
func (mc *MarkdownConverter) createParagraphRequests(line string) []*docs.Request {
	var requests []*docs.Request

	// Process inline formatting: **bold**, *italic*, `code`, [link](url)
	processedText, inlineRequests := mc.processInlineFormatting(line)

	// Insert the processed text
	requests = append(requests, &docs.Request{
		InsertText: &docs.InsertTextRequest{
			Location: &docs.Location{Index: mc.currentIndex},
			Text:     processedText,
		},
	})

	// Apply inline formatting
	requests = append(requests, inlineRequests...)

	mc.currentIndex += int64(len(processedText))

	return requests
}

// createNormalParagraphRequests creates requests for normal paragraphs with style reset
func (mc *MarkdownConverter) createNormalParagraphRequests(line string) []*docs.Request {
	var requests []*docs.Request

	// Process inline formatting: **bold**, *italic*, `code`, [link](url)
	processedText, inlineRequests := mc.processInlineFormatting(line)

	startIndex := mc.currentIndex

	// Insert the processed text
	requests = append(requests, &docs.Request{
		InsertText: &docs.InsertTextRequest{
			Location: &docs.Location{Index: mc.currentIndex},
			Text:     processedText,
		},
	})

	mc.currentIndex += int64(len(processedText))

	// Delete any existing bullets/lists
	requests = append(requests, &docs.Request{
		DeleteParagraphBullets: &docs.DeleteParagraphBulletsRequest{
			Range: &docs.Range{
				StartIndex: startIndex,
				EndIndex:   mc.currentIndex,
			},
		},
	})

	// Reset paragraph style to normal
	requests = append(requests, &docs.Request{
		UpdateParagraphStyle: &docs.UpdateParagraphStyleRequest{
			Range: &docs.Range{
				StartIndex: startIndex,
				EndIndex:   mc.currentIndex,
			},
			ParagraphStyle: &docs.ParagraphStyle{
				NamedStyleType: "NORMAL_TEXT",
			},
			Fields: "namedStyleType",
		},
	})

	// Apply inline formatting after style reset
	requests = append(requests, inlineRequests...)

	return requests
}

// processInlineFormatting processes inline markdown formatting and returns cleaned text and formatting requests
func (mc *MarkdownConverter) processInlineFormatting(text string) (string, []*docs.Request) {
	var requests []*docs.Request
	cleanText := text
	offset := int64(0)

	// Process **bold** text
	boldRegex := regexp.MustCompile(`\*\*(.*?)\*\*`)
	boldMatches := boldRegex.FindAllStringSubmatchIndex(cleanText, -1)
	for i := len(boldMatches) - 1; i >= 0; i-- {
		match := boldMatches[i]
		start, end := match[0], match[1]
		innerStart, innerEnd := match[2], match[3]

		// Replace **text** with text
		boldText := cleanText[innerStart:innerEnd]
		cleanText = cleanText[:start] + boldText + cleanText[end:]

		// Add bold formatting request
		adjustedStart := mc.currentIndex + int64(start) - offset
		adjustedEnd := adjustedStart + int64(len(boldText))

		requests = append([]*docs.Request{{
			UpdateTextStyle: &docs.UpdateTextStyleRequest{
				Range: &docs.Range{
					StartIndex: adjustedStart,
					EndIndex:   adjustedEnd,
				},
				TextStyle: &docs.TextStyle{
					Bold: true,
				},
				Fields: "bold",
			},
		}}, requests...)

		offset += int64(4) // ** at start and end
	}

	// Process *italic* text (but not **bold**)
	italicRegex := regexp.MustCompile(`(?:\*([^*]+?)\*)|(?:_([^_]+?)_)`)
	italicMatches := italicRegex.FindAllStringSubmatchIndex(cleanText, -1)
	for i := len(italicMatches) - 1; i >= 0; i-- {
		match := italicMatches[i]
		start, end := match[0], match[1]

		// Find which group matched (group 1 for *, group 2 for _)
		var innerStart, innerEnd int
		if match[2] != -1 { // * group matched
			innerStart, innerEnd = match[2], match[3]
		} else { // _ group matched
			innerStart, innerEnd = match[4], match[5]
		}

		// Replace *text* or _text_ with text
		italicText := cleanText[innerStart:innerEnd]
		cleanText = cleanText[:start] + italicText + cleanText[end:]

		// Add italic formatting request
		adjustedStart := mc.currentIndex + int64(start) - offset
		adjustedEnd := adjustedStart + int64(len(italicText))

		requests = append([]*docs.Request{{
			UpdateTextStyle: &docs.UpdateTextStyleRequest{
				Range: &docs.Range{
					StartIndex: adjustedStart,
					EndIndex:   adjustedEnd,
				},
				TextStyle: &docs.TextStyle{
					Italic: true,
				},
				Fields: "italic",
			},
		}}, requests...)

		offset += int64(2) // * or _ at start and end
	}

	// Process `code` text
	codeRegex := regexp.MustCompile("`([^`]+?)`")
	codeMatches := codeRegex.FindAllStringSubmatchIndex(cleanText, -1)
	for i := len(codeMatches) - 1; i >= 0; i-- {
		match := codeMatches[i]
		start, end := match[0], match[1]
		innerStart, innerEnd := match[2], match[3]

		// Replace `code` with code
		codeText := cleanText[innerStart:innerEnd]
		cleanText = cleanText[:start] + codeText + cleanText[end:]

		// Add monospace formatting request
		adjustedStart := mc.currentIndex + int64(start) - offset
		adjustedEnd := adjustedStart + int64(len(codeText))

		requests = append([]*docs.Request{{
			UpdateTextStyle: &docs.UpdateTextStyleRequest{
				Range: &docs.Range{
					StartIndex: adjustedStart,
					EndIndex:   adjustedEnd,
				},
				TextStyle: &docs.TextStyle{
					WeightedFontFamily: &docs.WeightedFontFamily{
						FontFamily: "Consolas",
					},
				},
				Fields: "weightedFontFamily",
			},
		}}, requests...)

		offset += int64(2) // ` at start and end
	}

	// Process [link](url) - convert to just link text for now
	// (Google Docs API link creation is more complex and requires additional setup)
	linkRegex := regexp.MustCompile(`\[([^\]]+?)\]\([^)]+?\)`)
	linkMatches := linkRegex.FindAllStringSubmatchIndex(cleanText, -1)
	for i := len(linkMatches) - 1; i >= 0; i-- {
		match := linkMatches[i]
		start, end := match[0], match[1]
		innerStart, innerEnd := match[2], match[3]

		// Replace [text](url) with text
		linkText := cleanText[innerStart:innerEnd]
		cleanText = cleanText[:start] + linkText + cleanText[end:]

		// For now, just make it blue and underlined to indicate it was a link
		adjustedStart := mc.currentIndex + int64(start) - offset
		adjustedEnd := adjustedStart + int64(len(linkText))

		requests = append([]*docs.Request{{
			UpdateTextStyle: &docs.UpdateTextStyleRequest{
				Range: &docs.Range{
					StartIndex: adjustedStart,
					EndIndex:   adjustedEnd,
				},
				TextStyle: &docs.TextStyle{
					ForegroundColor: &docs.OptionalColor{
						Color: &docs.Color{
							RgbColor: &docs.RgbColor{
								Blue:  1.0,
								Green: 0.0,
								Red:   0.0,
							},
						},
					},
					Underline: true,
				},
				Fields: "foregroundColor,underline",
			},
		}}, requests...)

		// Calculate offset based on original match length minus link text length
		originalLength := end - start
		offset += int64(originalLength - len(linkText))
	}

	return cleanText, requests
}
