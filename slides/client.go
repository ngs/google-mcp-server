package slides

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"time"

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
	var formatRanges []FormatRange
	result := text
	offset := 0

	// Process bold text (**text** or __text__)
	boldRegex := regexp.MustCompile(`(\*\*|__)([^*_]+?)(\*\*|__)`)
	for {
		match := boldRegex.FindStringSubmatchIndex(result)
		if match == nil {
			break
		}
		
		// Check if the markers match (both ** or both __)
		if result[match[2]:match[3]] != result[match[6]:match[7]] {
			// Skip mismatched markers
			result = result[:match[0]+1] + result[match[0]+1:]
			continue
		}
		
		// Extract text without markers
		cleanText := result[match[4]:match[5]]
		start := match[0] + offset
		end := start + len(cleanText)
		
		// Add format range
		formatRanges = append(formatRanges, FormatRange{
			Start: start,
			End:   end,
			Style: &slides.TextStyle{
				Bold: true,
			},
			Fields: "bold",
		})
		
		// Replace in result
		result = result[:match[0]] + cleanText + result[match[7]:]
		offset -= (match[7] - match[0] - len(cleanText))
	}

	// Process italic text (*text* or _text_)
	// Must handle single asterisk/underscore carefully to avoid conflicts with bold
	italicRegex := regexp.MustCompile(`(?:^|[^*_])([*_])([^*_]+?)([*_])(?:[^*_]|$)`)
	processedResult := ""
	lastEnd := 0
	
	for {
		match := italicRegex.FindStringSubmatchIndex(result[lastEnd:])
		if match == nil {
			processedResult += result[lastEnd:]
			break
		}
		
		// Adjust indices for substring search
		for i := range match {
			if i > 0 {
				match[i] += lastEnd
			}
		}
		
		// Check if the markers match
		if result[match[2]:match[3]] != result[match[6]:match[7]] {
			// Skip mismatched markers
			processedResult += result[lastEnd:lastEnd+1]
			lastEnd++
			continue
		}
		
		// Extract text without markers
		cleanText := result[match[4]:match[5]]
		realStart := match[2]
		
		// Add the part before the match
		processedResult += result[lastEnd:realStart]
		
		// Add format range (adjust for already processed text)
		formatRanges = append(formatRanges, FormatRange{
			Start: len(processedResult),
			End:   len(processedResult) + len(cleanText),
			Style: &slides.TextStyle{
				Italic: true,
			},
			Fields: "italic",
		})
		
		// Add the clean text
		processedResult += cleanText
		
		// Update lastEnd to skip past the closing marker
		lastEnd = match[7]
	}
	
	// Handle case where no italic formatting was found
	if processedResult == "" {
		processedResult = result
	}

	return processedResult, formatRanges
}

func (c *Client) ReplaceAllTextInSlide(presentationId string, slideId string, oldText, newText string) (*slides.BatchUpdatePresentationResponse, error) {
	requests := []*slides.Request{
		{
			ReplaceAllText: &slides.ReplaceAllTextRequest{
				ContainsText: &slides.SubstringMatchCriteria{
					Text:      oldText,
					MatchCase: false,
				},
				ReplaceText: newText,
				PageObjectIds: []string{slideId},
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

	// Add text style requests for bold and italic
	for _, fr := range formatRanges {
		startIdx := int64(fr.Start)
		endIdx := int64(fr.End)
		requests = append(requests, &slides.Request{
			UpdateTextStyle: &slides.UpdateTextStyleRequest{
				ObjectId: elementId,
				TextRange: &slides.Range{
					Type:       "FIXED_RANGE",
					StartIndex: &startIdx,
					EndIndex:   &endIdx,
				},
				Style: fr.Style,
				Fields: fr.Fields,
			},
		})
	}

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

func generateId() string {
	// Simple ID generator - in production, use UUID or similar
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
