package slides

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"time"
	"unicode/utf16"

	"google.golang.org/api/option"
	"google.golang.org/api/slides/v1"
)

type FormatRange struct {
	Start  int
	End    int
	Style  *slides.TextStyle
	Fields string
}

type Client struct {
	service *slides.Service
}

func NewClient(ctx context.Context, client *http.Client) (*Client, error) {
	service, err := slides.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("failed to create slides service: %w", err)
	}
	return &Client{service: service}, nil
}

func (c *Client) CreatePresentation(title string) (*slides.Presentation, error) {
	presentation := &slides.Presentation{
		Title: title,
	}
	return c.service.Presentations.Create(presentation).Do()
}

func (c *Client) GetPresentation(presentationId string) (*slides.Presentation, error) {
	return c.service.Presentations.Get(presentationId).Do()
}

// GetLayoutId gets the layout ID by name from a presentation
func (c *Client) GetLayoutId(presentationId string, layoutName string) (string, error) {
	presentation, err := c.GetPresentation(presentationId)
	if err != nil {
		return "", err
	}

	// Iterate through layouts to find the one with matching name
	for _, layout := range presentation.Layouts {
		if layout.LayoutProperties != nil && layout.LayoutProperties.Name == layoutName {
			return layout.ObjectId, nil
		}
	}

	// If exact match not found, try common layout names
	layoutMapping := map[string][]string{
		"TITLE_AND_BODY": {"Title and body", "Title and Body", "TITLE_AND_BODY"},
		"TITLE_ONLY":     {"Title only", "Title Only", "TITLE_ONLY"},
		"BLANK":          {"Blank", "BLANK"},
		"TITLE":          {"Title", "TITLE", "Title slide", "Title Slide"},
	}

	if alternatives, exists := layoutMapping[layoutName]; exists {
		for _, alt := range alternatives {
			for _, layout := range presentation.Layouts {
				if layout.LayoutProperties != nil && layout.LayoutProperties.Name == alt {
					return layout.ObjectId, nil
				}
			}
		}
	}

	// Return first layout as fallback
	if len(presentation.Layouts) > 0 {
		return presentation.Layouts[0].ObjectId, nil
	}

	return "", fmt.Errorf("no layout found with name: %s", layoutName)
}

func (c *Client) ListPresentations() ([]*slides.Presentation, error) {
	// Note: Slides API doesn't have a direct list method like Drive
	// This would typically be done through Drive API
	return nil, fmt.Errorf("use Drive API to list presentations")
}

func (c *Client) CreateSlide(presentationId string, insertionIndex int) (*slides.BatchUpdatePresentationResponse, error) {
	createSlideReq := &slides.CreateSlideRequest{}

	// Only set InsertionIndex if it's >= 0
	// If not set, the slide will be appended to the end
	if insertionIndex >= 0 {
		createSlideReq.InsertionIndex = int64(insertionIndex)
	}

	requests := []*slides.Request{
		{
			CreateSlide: createSlideReq,
		},
	}

	req := &slides.BatchUpdatePresentationRequest{
		Requests: requests,
	}

	return c.service.Presentations.BatchUpdate(presentationId, req).Do()
}

// CreateSlideWithLayout creates a new slide with a specific layout
func (c *Client) CreateSlideWithLayout(presentationId string, layoutId string, insertionIndex int) (*slides.BatchUpdatePresentationResponse, error) {
	createSlideReq := &slides.CreateSlideRequest{
		SlideLayoutReference: &slides.LayoutReference{
			LayoutId: layoutId,
		},
	}

	// Only set InsertionIndex if it's >= 0
	// If not set, the slide will be appended to the end
	if insertionIndex >= 0 {
		createSlideReq.InsertionIndex = int64(insertionIndex)
	}

	requests := []*slides.Request{
		{
			CreateSlide: createSlideReq,
		},
	}

	req := &slides.BatchUpdatePresentationRequest{
		Requests: requests,
	}

	return c.service.Presentations.BatchUpdate(presentationId, req).Do()
}

func (c *Client) DeleteSlide(presentationId string, slideId string) (*slides.BatchUpdatePresentationResponse, error) {
	requests := []*slides.Request{
		{
			DeleteObject: &slides.DeleteObjectRequest{
				ObjectId: slideId,
			},
		},
	}

	req := &slides.BatchUpdatePresentationRequest{
		Requests: requests,
	}

	return c.service.Presentations.BatchUpdate(presentationId, req).Do()
}

func (c *Client) DuplicateSlide(presentationId string, slideId string) (*slides.BatchUpdatePresentationResponse, error) {
	requests := []*slides.Request{
		{
			DuplicateObject: &slides.DuplicateObjectRequest{
				ObjectId: slideId,
			},
		},
	}

	req := &slides.BatchUpdatePresentationRequest{
		Requests: requests,
	}

	return c.service.Presentations.BatchUpdate(presentationId, req).Do()
}

// ReplaceAllTextInShape replaces all text in existing shapes on a slide
// processMarkdownText processes markdown formatting and returns clean text with format ranges
func (c *Client) processMarkdownText(text string) (string, []FormatRange) {
	// Temporarily disable all formatting to avoid character counting issues
	// Just remove markdown markers without applying any formatting
	result := text

	// Remove bold markers (**text** or __text__)
	boldRegex := regexp.MustCompile(`(\*\*|__)([^*_]+?)(\*\*|__)`)
	result = boldRegex.ReplaceAllString(result, "$2")

	// Remove code block markers (```code```)
	codeBlockRegex := regexp.MustCompile("(?s)```[^\\n]*\\n([^`]+?)```")
	result = codeBlockRegex.ReplaceAllString(result, "$1")

	// Remove inline code markers (`code`)
	inlineCodeRegex := regexp.MustCompile("`([^`]+?)`")
	result = inlineCodeRegex.ReplaceAllString(result, "$1")

	// Remove italic markers (*text*)
	italicRegex := regexp.MustCompile(`\*([^*]+?)\*`)
	result = italicRegex.ReplaceAllString(result, "$1")

	// Return empty format ranges for now
	return result, []FormatRange{}
}

func (c *Client) ReplaceAllTextInSlide(presentationId string, slideId string, oldText, newText string) (*slides.BatchUpdatePresentationResponse, error) {
	requests := []*slides.Request{
		{
			ReplaceAllText: &slides.ReplaceAllTextRequest{
				ContainsText: &slides.SubstringMatchCriteria{
					Text:      oldText,
					MatchCase: false,
				},
				ReplaceText:   newText,
				PageObjectIds: []string{slideId},
			},
		},
	}

	req := &slides.BatchUpdatePresentationRequest{
		Requests: requests,
	}

	return c.service.Presentations.BatchUpdate(presentationId, req).Do()
}

// InsertTextInPlaceholder inserts text into a placeholder shape
func (c *Client) InsertTextInPlaceholder(presentationId string, shapeId string, text string) (*slides.BatchUpdatePresentationResponse, error) {
	// Process markdown formatting properly for placeholders
	processedText, formatRanges := c.processMarkdownTextWithFormatting(text)

	requests := []*slides.Request{
		{
			InsertText: &slides.InsertTextRequest{
				ObjectId:       shapeId,
				InsertionIndex: 0,
				Text:           processedText,
			},
		},
	}

	// Apply formatting ranges (bold, italic)
	for _, fr := range formatRanges {
		startIdx := int64(fr.Start)
		endIdx := int64(fr.End)
		requests = append(requests, &slides.Request{
			UpdateTextStyle: &slides.UpdateTextStyleRequest{
				ObjectId: shapeId,
				TextRange: &slides.Range{
					Type:       "FIXED_RANGE",
					StartIndex: &startIdx,
					EndIndex:   &endIdx,
				},
				Style:  fr.Style,
				Fields: fr.Fields,
			},
		})
	}

	// Don't apply Courier New font to all text anymore
	// Code blocks and inline code are handled in processMarkdownTextWithFormatting

	req := &slides.BatchUpdatePresentationRequest{
		Requests: requests,
	}

	return c.service.Presentations.BatchUpdate(presentationId, req).Do()
}

// processMarkdownTextWithFormatting processes markdown with proper formatting for placeholders
func (c *Client) processMarkdownTextWithFormatting(text string) (string, []FormatRange) {
	var formatRanges []FormatRange
	result := text

	// Process bold text (**text** or __text__)
	boldRegex := regexp.MustCompile(`\*\*([^*]+?)\*\*|__([^_]+?)__`)
	for {
		match := boldRegex.FindStringSubmatchIndex(result)
		if match == nil {
			break
		}

		var cleanText string
		if match[2] != -1 { // ** match
			cleanText = result[match[2]:match[3]]
		} else { // __ match
			cleanText = result[match[4]:match[5]]
		}

		// Calculate positions in the current processed text (before removing markers)
		// Google Slides API uses UTF-16 code units, not runes
		beforeText := result[:match[0]]
		start := len([]uint16(utf16.Encode([]rune(beforeText))))
		end := start + len([]uint16(utf16.Encode([]rune(cleanText))))

		// Add format range
		formatRanges = append(formatRanges, FormatRange{
			Start: start,
			End:   end,
			Style: &slides.TextStyle{
				Bold: true,
			},
			Fields: "bold",
		})

		// Calculate markers length in UTF-16 code units before modifying result
		fullMatch := result[match[0]:match[1]]
		markersLength := len([]uint16(utf16.Encode([]rune(fullMatch)))) - len([]uint16(utf16.Encode([]rune(cleanText)))) // Total UTF-16 code units of markers removed

		// Replace in result (remove markers, keep content)
		result = result[:match[0]] + cleanText + result[match[1]:]

		// Adjust existing format ranges for the removed markers
		for i := range formatRanges[:len(formatRanges)-1] { // Don't adjust the one we just added
			if formatRanges[i].Start > start {
				formatRanges[i].Start -= markersLength
				formatRanges[i].End -= markersLength
			}
		}
	}

	// Process italic text (*text*)
	italicRegex := regexp.MustCompile(`\*([^*]+?)\*`)
	for {
		match := italicRegex.FindStringSubmatchIndex(result)
		if match == nil {
			break
		}

		// Extract text without markers
		cleanText := result[match[2]:match[3]]

		// Calculate positions using UTF-16 code units
		beforeText := result[:match[0]]
		start := len([]uint16(utf16.Encode([]rune(beforeText))))
		end := start + len([]uint16(utf16.Encode([]rune(cleanText))))

		// Add format range
		formatRanges = append(formatRanges, FormatRange{
			Start: start,
			End:   end,
			Style: &slides.TextStyle{
				Italic: true,
			},
			Fields: "italic",
		})

		// Replace in result
		result = result[:match[0]] + cleanText + result[match[1]:]

		// Adjust existing format ranges
		markersRemoved := 2 // Two asterisks (in UTF-16 code units)
		for i := range formatRanges[:len(formatRanges)-1] {
			if formatRanges[i].Start > start {
				formatRanges[i].Start -= markersRemoved
				formatRanges[i].End -= markersRemoved
			}
		}
	}

	// Process inline code (`code`) - apply Courier New font
	inlineCodeRegex := regexp.MustCompile("`([^`]+?)`")
	for {
		match := inlineCodeRegex.FindStringSubmatchIndex(result)
		if match == nil {
			break
		}

		// Extract text without markers
		cleanText := result[match[2]:match[3]]

		// Calculate positions using UTF-16 code units
		beforeText := result[:match[0]]
		start := len([]uint16(utf16.Encode([]rune(beforeText))))
		end := start + len([]uint16(utf16.Encode([]rune(cleanText))))

		// Add format range for Courier New font
		formatRanges = append(formatRanges, FormatRange{
			Start: start,
			End:   end,
			Style: &slides.TextStyle{
				FontFamily: "Courier New",
			},
			Fields: "fontFamily",
		})

		// Replace in result
		result = result[:match[0]] + cleanText + result[match[1]:]

		// Adjust existing format ranges
		markersRemoved := 2 // Two backticks (in UTF-16 code units)
		for i := range formatRanges[:len(formatRanges)-1] {
			if formatRanges[i].Start > start {
				formatRanges[i].Start -= markersRemoved
				formatRanges[i].End -= markersRemoved
			}
		}
	}

	// Process links [text](url)
	linkRegex := regexp.MustCompile(`\[([^\]]+?)\]\(([^)]+?)\)`)
	for {
		match := linkRegex.FindStringSubmatchIndex(result)
		if match == nil {
			break
		}

		// Extract link text and URL
		linkText := result[match[2]:match[3]]
		linkURL := result[match[4]:match[5]]

		// Calculate positions in the current processed text using UTF-16 code units
		beforeText := result[:match[0]]
		start := len([]uint16(utf16.Encode([]rune(beforeText))))
		end := start + len([]uint16(utf16.Encode([]rune(linkText))))

		// Add format range for link
		formatRanges = append(formatRanges, FormatRange{
			Start: start,
			End:   end,
			Style: &slides.TextStyle{
				Link: &slides.Link{
					Url: linkURL,
				},
			},
			Fields: "link",
		})

		// Calculate markdown length in UTF-16 code units before modifying result
		fullMatch := result[match[0]:match[1]]
		markersLength := len([]uint16(utf16.Encode([]rune(fullMatch)))) - len([]uint16(utf16.Encode([]rune(linkText)))) // Total UTF-16 code units of markdown removed

		// Replace in result (keep only link text, remove markdown)
		result = result[:match[0]] + linkText + result[match[1]:]

		// Adjust existing format ranges for the removed markdown
		for i := range formatRanges[:len(formatRanges)-1] { // Don't adjust the one we just added
			if formatRanges[i].Start > start {
				formatRanges[i].Start -= markersLength
				formatRanges[i].End -= markersLength
			}
		}
	}

	// Process code blocks (```code```) - apply Courier New to entire content
	codeBlockRegex := regexp.MustCompile("(?s)```[^\\n]*\\n([^`]+?)```")
	for {
		match := codeBlockRegex.FindStringSubmatchIndex(result)
		if match == nil {
			break
		}

		// Extract code content
		codeContent := result[match[2]:match[3]]

		// Calculate position for the code content after removing markers
		beforeText := result[:match[0]]
		start := len([]uint16(utf16.Encode([]rune(beforeText))))
		end := start + len([]uint16(utf16.Encode([]rune(codeContent))))

		// Add format range for entire code block
		formatRanges = append(formatRanges, FormatRange{
			Start: start,
			End:   end,
			Style: &slides.TextStyle{
				FontFamily: "Courier New",
			},
			Fields: "fontFamily",
		})

		// Calculate markers length before modifying result
		fullMatch := result[match[0]:match[1]]
		markersLength := len([]uint16(utf16.Encode([]rune(fullMatch)))) - len([]uint16(utf16.Encode([]rune(codeContent))))

		// Replace code block with just the content
		result = result[:match[0]] + codeContent + result[match[1]:]

		// Adjust existing format ranges
		for i := range formatRanges[:len(formatRanges)-1] {
			if formatRanges[i].Start > start {
				formatRanges[i].Start -= markersLength
				formatRanges[i].End -= markersLength
			}
		}
	}

	return result, formatRanges
}

// DeleteTextInPlaceholder deletes existing text in a placeholder
func (c *Client) DeleteTextInPlaceholder(presentationId string, shapeId string) (*slides.BatchUpdatePresentationResponse, error) {
	requests := []*slides.Request{
		{
			DeleteText: &slides.DeleteTextRequest{
				ObjectId: shapeId,
				TextRange: &slides.Range{
					Type: "ALL",
				},
			},
		},
	}

	req := &slides.BatchUpdatePresentationRequest{
		Requests: requests,
	}

	return c.service.Presentations.BatchUpdate(presentationId, req).Do()
}

func (c *Client) AddTextBox(presentationId string, slideId string, text string, x, y, width, height float64) (*slides.BatchUpdatePresentationResponse, error) {
	// Validate dimensions to avoid "affine transform is not invertible" error
	if width <= 0 {
		width = 400 // Default width
	}
	if height <= 0 {
		height = 100 // Default height
	}

	elementId := fmt.Sprintf("textbox_%s", generateId())

	// Process markdown formatting (bold and italic)
	processedText, formatRanges := c.processMarkdownText(text)

	requests := []*slides.Request{
		{
			CreateShape: &slides.CreateShapeRequest{
				ObjectId:  elementId,
				ShapeType: "TEXT_BOX",
				ElementProperties: &slides.PageElementProperties{
					PageObjectId: slideId,
					Size: &slides.Size{
						Width: &slides.Dimension{
							Magnitude: width,
							Unit:      "PT",
						},
						Height: &slides.Dimension{
							Magnitude: height,
							Unit:      "PT",
						},
					},
					Transform: &slides.AffineTransform{
						ScaleX:     1.0,
						ScaleY:     1.0,
						TranslateX: x,
						TranslateY: y,
						Unit:       "PT",
					},
				},
			},
		},
		{
			InsertText: &slides.InsertTextRequest{
				ObjectId:       elementId,
				InsertionIndex: 0,
				Text:           processedText,
			},
		},
	}

	// Skip text styling for now to avoid character counting issues
	_ = formatRanges

	req := &slides.BatchUpdatePresentationRequest{
		Requests: requests,
	}

	resp, err := c.service.Presentations.BatchUpdate(presentationId, req).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to add text box: %w", err)
	}

	// Check if all requests were successful
	if resp != nil && len(resp.Replies) != len(requests) {
		return resp, fmt.Errorf("some requests failed: expected %d replies, got %d", len(requests), len(resp.Replies))
	}

	return resp, nil
}

func (c *Client) AddCodeTextBox(presentationId string, slideId string, text string, x, y, width, height float64) (*slides.BatchUpdatePresentationResponse, error) {
	// Validate dimensions
	if width <= 0 {
		width = 400
	}
	if height <= 0 {
		height = 100
	}

	elementId := fmt.Sprintf("codebox_%s", generateId())

	// Remove markdown markers but don't apply formatting
	processedText, _ := c.processMarkdownText(text)

	requests := []*slides.Request{
		{
			CreateShape: &slides.CreateShapeRequest{
				ObjectId:  elementId,
				ShapeType: "TEXT_BOX",
				ElementProperties: &slides.PageElementProperties{
					PageObjectId: slideId,
					Size: &slides.Size{
						Width: &slides.Dimension{
							Magnitude: width,
							Unit:      "PT",
						},
						Height: &slides.Dimension{
							Magnitude: height,
							Unit:      "PT",
						},
					},
					Transform: &slides.AffineTransform{
						ScaleX:     1.0,
						ScaleY:     1.0,
						TranslateX: x,
						TranslateY: y,
						Unit:       "PT",
					},
				},
			},
		},
		{
			InsertText: &slides.InsertTextRequest{
				ObjectId:       elementId,
				InsertionIndex: 0,
				Text:           processedText,
			},
		},
		{
			UpdateTextStyle: &slides.UpdateTextStyleRequest{
				ObjectId: elementId,
				TextRange: &slides.Range{
					Type: "ALL",
				},
				Style: &slides.TextStyle{
					FontFamily: "Courier New",
				},
				Fields: "fontFamily",
			},
		},
	}

	req := &slides.BatchUpdatePresentationRequest{
		Requests: requests,
	}

	return c.service.Presentations.BatchUpdate(presentationId, req).Do()
}

func (c *Client) AddImage(presentationId string, slideId string, imageUrl string, x, y, width, height float64) (*slides.BatchUpdatePresentationResponse, error) {
	elementId := fmt.Sprintf("image_%s", generateId())

	requests := []*slides.Request{
		{
			CreateImage: &slides.CreateImageRequest{
				ObjectId: elementId,
				Url:      imageUrl,
				ElementProperties: &slides.PageElementProperties{
					PageObjectId: slideId,
					Size: &slides.Size{
						Width: &slides.Dimension{
							Magnitude: width,
							Unit:      "PT",
						},
						Height: &slides.Dimension{
							Magnitude: height,
							Unit:      "PT",
						},
					},
					Transform: &slides.AffineTransform{
						ScaleX:     1.0,
						ScaleY:     1.0,
						TranslateX: x,
						TranslateY: y,
						Unit:       "PT",
					},
				},
			},
		},
	}

	req := &slides.BatchUpdatePresentationRequest{
		Requests: requests,
	}

	return c.service.Presentations.BatchUpdate(presentationId, req).Do()
}

func (c *Client) AddTable(presentationId string, slideId string, rows, columns int, x, y, width, height float64) (*slides.BatchUpdatePresentationResponse, error) {
	// Validate dimensions to avoid "affine transform is not invertible" error
	if width <= 0 {
		width = 400 // Default width
	}
	if height <= 0 {
		height = 200 // Default height
	}
	if rows <= 0 {
		rows = 1
	}
	if columns <= 0 {
		columns = 1
	}

	elementId := fmt.Sprintf("table_%s", generateId())

	requests := []*slides.Request{
		{
			CreateTable: &slides.CreateTableRequest{
				ObjectId: elementId,
				Rows:     int64(rows),
				Columns:  int64(columns),
				ElementProperties: &slides.PageElementProperties{
					PageObjectId: slideId,
					Size: &slides.Size{
						Width: &slides.Dimension{
							Magnitude: width,
							Unit:      "PT",
						},
						Height: &slides.Dimension{
							Magnitude: height,
							Unit:      "PT",
						},
					},
					Transform: &slides.AffineTransform{
						ScaleX:     1.0,
						ScaleY:     1.0,
						TranslateX: x,
						TranslateY: y,
						Unit:       "PT",
					},
				},
			},
		},
	}

	req := &slides.BatchUpdatePresentationRequest{
		Requests: requests,
	}

	return c.service.Presentations.BatchUpdate(presentationId, req).Do()
}

func (c *Client) AddShape(presentationId string, slideId string, shapeType string, x, y, width, height float64) (*slides.BatchUpdatePresentationResponse, error) {
	// Validate dimensions to avoid "affine transform is not invertible" error
	if width <= 0 {
		width = 100 // Default width
	}
	if height <= 0 {
		height = 100 // Default height
	}

	elementId := fmt.Sprintf("shape_%s", generateId())

	requests := []*slides.Request{
		{
			CreateShape: &slides.CreateShapeRequest{
				ObjectId:  elementId,
				ShapeType: shapeType,
				ElementProperties: &slides.PageElementProperties{
					PageObjectId: slideId,
					Size: &slides.Size{
						Width: &slides.Dimension{
							Magnitude: width,
							Unit:      "PT",
						},
						Height: &slides.Dimension{
							Magnitude: height,
							Unit:      "PT",
						},
					},
					Transform: &slides.AffineTransform{
						ScaleX:     1.0,
						ScaleY:     1.0,
						TranslateX: x,
						TranslateY: y,
						Unit:       "PT",
					},
				},
			},
		},
	}

	req := &slides.BatchUpdatePresentationRequest{
		Requests: requests,
	}

	return c.service.Presentations.BatchUpdate(presentationId, req).Do()
}

func (c *Client) ApplyTemplate(presentationId string, templateId string) (*slides.BatchUpdatePresentationResponse, error) {
	// This would require fetching the template and applying its layouts
	// Complex operation that might need multiple API calls
	return nil, fmt.Errorf("template application not yet implemented")
}

func (c *Client) SetSlideLayout(presentationId string, slideId string, layoutId string) (*slides.BatchUpdatePresentationResponse, error) {
	requests := []*slides.Request{
		{
			UpdatePageProperties: &slides.UpdatePagePropertiesRequest{
				ObjectId: slideId,
				PageProperties: &slides.PageProperties{
					PageBackgroundFill: &slides.PageBackgroundFill{},
				},
				Fields: "pageBackgroundFill",
			},
		},
	}

	req := &slides.BatchUpdatePresentationRequest{
		Requests: requests,
	}

	return c.service.Presentations.BatchUpdate(presentationId, req).Do()
}

func (c *Client) BatchUpdate(presentationId string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
	req := &slides.BatchUpdatePresentationRequest{
		Requests: requests,
	}

	return c.service.Presentations.BatchUpdate(presentationId, req).Do()
}

// InsertTextInTableCell inserts text into a specific table cell
func (c *Client) InsertTextInTableCell(presentationId string, tableId string, row, col int, text string) (*slides.BatchUpdatePresentationResponse, error) {
	requests := []*slides.Request{
		{
			InsertText: &slides.InsertTextRequest{
				ObjectId: tableId,
				CellLocation: &slides.TableCellLocation{
					RowIndex:    int64(row),
					ColumnIndex: int64(col),
				},
				InsertionIndex: 0,
				Text:           text,
			},
		},
	}

	req := &slides.BatchUpdatePresentationRequest{
		Requests: requests,
	}

	return c.service.Presentations.BatchUpdate(presentationId, req).Do()
}

// ApplyCodeFormattingToPlaceholder applies Courier New font to specific text ranges in a placeholder
func (c *Client) ApplyCodeFormattingToPlaceholder(presentationId string, shapeId string, codeRanges []struct {
	start int
	end   int
}) error {
	var requests []*slides.Request

	for _, cr := range codeRanges {
		startIdx := int64(cr.start)
		endIdx := int64(cr.end)
		requests = append(requests, &slides.Request{
			UpdateTextStyle: &slides.UpdateTextStyleRequest{
				ObjectId: shapeId,
				TextRange: &slides.Range{
					Type:       "FIXED_RANGE",
					StartIndex: &startIdx,
					EndIndex:   &endIdx,
				},
				Style: &slides.TextStyle{
					FontFamily: "Courier New",
				},
				Fields: "fontFamily",
			},
		})
	}

	if len(requests) == 0 {
		return nil
	}

	req := &slides.BatchUpdatePresentationRequest{
		Requests: requests,
	}

	_, err := c.service.Presentations.BatchUpdate(presentationId, req).Do()
	return err
}

func generateId() string {
	// Simple ID generator - in production, use UUID or similar
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
