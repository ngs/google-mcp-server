package slides

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"go.ngs.io/google-mcp-server/auth"
	"go.ngs.io/google-mcp-server/server"
)

type Service struct {
	authManager *auth.AccountManager
}

func NewService(authManager *auth.AccountManager) *Service {
	return &Service{
		authManager: authManager,
	}
}

func (s *Service) GetTools() []server.Tool {
	return []server.Tool{
		{
			Name:        "slides_presentation_create",
			Description: "Create a new Google Slides presentation",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"title": {
						Type:        "string",
						Description: "Presentation title",
					},
					"account": {
						Type:        "string",
						Description: "Email address of the account to use (optional)",
					},
				},
				Required: []string{"title"},
			},
		},
		{
			Name:        "slides_presentation_get",
			Description: "Get Google Slides presentation metadata",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"presentation_id": {
						Type:        "string",
						Description: "Presentation ID",
					},
					"account": {
						Type:        "string",
						Description: "Email address of the account to use (optional)",
					},
				},
				Required: []string{"presentation_id"},
			},
		},
		{
			Name:        "slides_slide_create",
			Description: "Create a new slide in a presentation",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"presentation_id": {
						Type:        "string",
						Description: "Presentation ID",
					},
					"insertion_index": {
						Type:        "number",
						Description: "Position to insert the slide (0-based)",
					},
					"account": {
						Type:        "string",
						Description: "Email address of the account to use (optional)",
					},
				},
				Required: []string{"presentation_id"},
			},
		},
		{
			Name:        "slides_slide_delete",
			Description: "Delete a slide from a presentation",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"presentation_id": {
						Type:        "string",
						Description: "Presentation ID",
					},
					"slide_id": {
						Type:        "string",
						Description: "Slide ID to delete",
					},
					"account": {
						Type:        "string",
						Description: "Email address of the account to use (optional)",
					},
				},
				Required: []string{"presentation_id", "slide_id"},
			},
		},
		{
			Name:        "slides_slide_duplicate",
			Description: "Duplicate a slide in a presentation",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"presentation_id": {
						Type:        "string",
						Description: "Presentation ID",
					},
					"slide_id": {
						Type:        "string",
						Description: "Slide ID to duplicate",
					},
					"account": {
						Type:        "string",
						Description: "Email address of the account to use (optional)",
					},
				},
				Required: []string{"presentation_id", "slide_id"},
			},
		},
		{
			Name:        "slides_markdown_create",
			Description: "Create a new presentation from Markdown content with automatic pagination",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"title": {
						Type:        "string",
						Description: "Presentation title",
					},
					"markdown": {
						Type:        "string",
						Description: "Markdown content (use --- for page breaks)",
					},
					"account": {
						Type:        "string",
						Description: "Email address of the account to use (optional)",
					},
				},
				Required: []string{"title", "markdown"},
			},
		},
		{
			Name:        "slides_markdown_update",
			Description: "Update an existing presentation with Markdown content",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"presentation_id": {
						Type:        "string",
						Description: "Presentation ID to update",
					},
					"markdown": {
						Type:        "string",
						Description: "Markdown content (use --- for page breaks)",
					},
					"account": {
						Type:        "string",
						Description: "Email address of the account to use (optional)",
					},
				},
				Required: []string{"presentation_id", "markdown"},
			},
		},
		{
			Name:        "slides_markdown_append",
			Description: "Append slides from Markdown to an existing presentation",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"presentation_id": {
						Type:        "string",
						Description: "Presentation ID",
					},
					"markdown": {
						Type:        "string",
						Description: "Markdown content to append",
					},
					"account": {
						Type:        "string",
						Description: "Email address of the account to use (optional)",
					},
				},
				Required: []string{"presentation_id", "markdown"},
			},
		},
		{
			Name:        "slides_add_text",
			Description: "Add a text box to a slide",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"presentation_id": {
						Type:        "string",
						Description: "Presentation ID",
					},
					"slide_id": {
						Type:        "string",
						Description: "Slide ID",
					},
					"text": {
						Type:        "string",
						Description: "Text content",
					},
					"x": {
						Type:        "number",
						Description: "X position in points",
					},
					"y": {
						Type:        "number",
						Description: "Y position in points",
					},
					"width": {
						Type:        "number",
						Description: "Width in points",
					},
					"height": {
						Type:        "number",
						Description: "Height in points",
					},
					"account": {
						Type:        "string",
						Description: "Email address of the account to use (optional)",
					},
				},
				Required: []string{"presentation_id", "slide_id", "text"},
			},
		},
		{
			Name:        "slides_add_image",
			Description: "Add an image to a slide",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"presentation_id": {
						Type:        "string",
						Description: "Presentation ID",
					},
					"slide_id": {
						Type:        "string",
						Description: "Slide ID",
					},
					"image_url": {
						Type:        "string",
						Description: "Image URL",
					},
					"x": {
						Type:        "number",
						Description: "X position in points",
					},
					"y": {
						Type:        "number",
						Description: "Y position in points",
					},
					"width": {
						Type:        "number",
						Description: "Width in points",
					},
					"height": {
						Type:        "number",
						Description: "Height in points",
					},
					"account": {
						Type:        "string",
						Description: "Email address of the account to use (optional)",
					},
				},
				Required: []string{"presentation_id", "slide_id", "image_url"},
			},
		},
		{
			Name:        "slides_add_table",
			Description: "Add a table to a slide",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"presentation_id": {
						Type:        "string",
						Description: "Presentation ID",
					},
					"slide_id": {
						Type:        "string",
						Description: "Slide ID",
					},
					"rows": {
						Type:        "number",
						Description: "Number of rows",
					},
					"columns": {
						Type:        "number",
						Description: "Number of columns",
					},
					"x": {
						Type:        "number",
						Description: "X position in points",
					},
					"y": {
						Type:        "number",
						Description: "Y position in points",
					},
					"width": {
						Type:        "number",
						Description: "Width in points",
					},
					"height": {
						Type:        "number",
						Description: "Height in points",
					},
					"account": {
						Type:        "string",
						Description: "Email address of the account to use (optional)",
					},
				},
				Required: []string{"presentation_id", "slide_id", "rows", "columns"},
			},
		},
		{
			Name:        "slides_add_shape",
			Description: "Add a shape to a slide",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"presentation_id": {
						Type:        "string",
						Description: "Presentation ID",
					},
					"slide_id": {
						Type:        "string",
						Description: "Slide ID",
					},
					"shape_type": {
						Type:        "string",
						Description: "Shape type (e.g., RECTANGLE, ELLIPSE, TRIANGLE)",
					},
					"x": {
						Type:        "number",
						Description: "X position in points",
					},
					"y": {
						Type:        "number",
						Description: "Y position in points",
					},
					"width": {
						Type:        "number",
						Description: "Width in points",
					},
					"height": {
						Type:        "number",
						Description: "Height in points",
					},
					"account": {
						Type:        "string",
						Description: "Email address of the account to use (optional)",
					},
				},
				Required: []string{"presentation_id", "slide_id", "shape_type"},
			},
		},
		{
			Name:        "slides_set_layout",
			Description: "Set the layout of a slide",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"presentation_id": {
						Type:        "string",
						Description: "Presentation ID",
					},
					"slide_id": {
						Type:        "string",
						Description: "Slide ID",
					},
					"layout_id": {
						Type:        "string",
						Description: "Layout ID",
					},
					"account": {
						Type:        "string",
						Description: "Email address of the account to use (optional)",
					},
				},
				Required: []string{"presentation_id", "slide_id", "layout_id"},
			},
		},
		{
			Name:        "slides_export_pdf",
			Description: "Export presentation as PDF (returns download URL)",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"presentation_id": {
						Type:        "string",
						Description: "Presentation ID",
					},
					"account": {
						Type:        "string",
						Description: "Email address of the account to use (optional)",
					},
				},
				Required: []string{"presentation_id"},
			},
		},
		{
			Name:        "slides_share",
			Description: "Create a shareable link for a presentation",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"presentation_id": {
						Type:        "string",
						Description: "Presentation ID",
					},
					"role": {
						Type:        "string",
						Description: "Permission role (reader, writer, commenter)",
						Enum:        []string{"reader", "writer", "commenter"},
					},
					"account": {
						Type:        "string",
						Description: "Email address of the account to use (optional)",
					},
				},
				Required: []string{"presentation_id", "role"},
			},
		},
	}
}

func (s *Service) HandleToolCall(ctx context.Context, name string, arguments json.RawMessage) (interface{}, error) {
	var args map[string]interface{}
	if err := json.Unmarshal(arguments, &args); err != nil {
		return nil, err
	}

	// Get account email if specified
	accountEmail, _ := args["account"].(string)

	// Get account and HTTP client
	var account *auth.Account
	var httpClient *http.Client

	if accountEmail != "" {
		var err error
		account, err = s.authManager.GetAccount(accountEmail)
		if err != nil {
			return nil, fmt.Errorf("failed to get account: %w", err)
		}
	} else {
		// Use first available account
		accounts := s.authManager.ListAccounts()
		if len(accounts) == 0 {
			return nil, fmt.Errorf("no authenticated accounts available. Please authenticate using accounts_add")
		}
		account = accounts[0]
	}

	// Check if account has required scopes for Slides
	if err := s.authManager.CheckScopes(ctx, account, "slides"); err != nil {
		if scopeErr, ok := err.(*auth.ScopeError); ok {
			return nil, fmt.Errorf("%v\n\nTo fix this, run: accounts_refresh (and select account: %s)", scopeErr, account.Email)
		}
		return nil, err
	}

	if account.OAuthClient == nil {
		return nil, fmt.Errorf("no OAuth client for account: %s. Please re-authenticate using accounts_refresh", account.Email)
	}

	httpClient = account.OAuthClient.GetHTTPClient()

	// Create Slides client
	client, err := NewClient(ctx, httpClient)
	if err != nil {
		// Check if this is an API disabled error
		if auth.IsAPIDisabledError(err) {
			return nil, fmt.Errorf(
				"Google Slides API is not enabled for this project.\n"+
					"Please enable it at: https://console.cloud.google.com/apis/library/slides.googleapis.com\n"+
					"Then re-authenticate using: accounts_refresh (account: %s)",
				account.Email,
			)
		}
		return nil, err
	}

	switch name {
	case "slides_presentation_create":
		title, _ := args["title"].(string)
		presentation, err := client.CreatePresentation(title)
		if err != nil {
			// Check if this is an API disabled error
			if auth.IsAPIDisabledError(err) {
				return nil, fmt.Errorf(
					"Google Slides API is not enabled for this project.\n"+
						"Account: %s\n"+
						"Please enable it at: https://console.cloud.google.com/apis/library/slides.googleapis.com\n"+
						"After enabling, wait a few minutes then re-authenticate using: accounts_refresh",
					account.Email,
				)
			}
			return nil, err
		}
		return map[string]interface{}{
			"presentation_id": presentation.PresentationId,
			"title":           presentation.Title,
			"url":             fmt.Sprintf("https://docs.google.com/presentation/d/%s/edit", presentation.PresentationId),
		}, nil

	case "slides_presentation_get":
		presentationId, _ := args["presentation_id"].(string)
		presentation, err := client.GetPresentation(presentationId)
		if err != nil {
			return nil, err
		}

		// Build detailed slide information
		slides := make([]map[string]interface{}, 0, len(presentation.Slides))
		for i, slide := range presentation.Slides {
			slideInfo := map[string]interface{}{
				"index":    i + 1,
				"slide_id": slide.ObjectId,
				"layout":   "",
				"title":    "",
				"elements": []map[string]interface{}{},
			}

			// Get layout information
			if slide.SlideProperties != nil && slide.SlideProperties.LayoutObjectId != "" {
				for _, layout := range presentation.Layouts {
					if layout.ObjectId == slide.SlideProperties.LayoutObjectId {
						if layout.LayoutProperties != nil {
							slideInfo["layout"] = layout.LayoutProperties.Name
						}
						break
					}
				}
			}

			// Extract elements information
			elements := make([]map[string]interface{}, 0)
			for _, element := range slide.PageElements {
				elementInfo := map[string]interface{}{
					"element_id": element.ObjectId,
					"type":       "",
				}

				if element.Shape != nil {
					elementInfo["type"] = "shape"
					if element.Shape.ShapeType != "" {
						elementInfo["shape_type"] = element.Shape.ShapeType
					}
					if element.Shape.Text != nil && len(element.Shape.Text.TextElements) > 0 {
						text := ""
						fontFamily := ""
						for _, textElement := range element.Shape.Text.TextElements {
							if textElement.TextRun != nil {
								text += textElement.TextRun.Content
								if textElement.TextRun.Style != nil && textElement.TextRun.Style.FontFamily != "" {
									fontFamily = textElement.TextRun.Style.FontFamily
								}
							}
						}
						elementInfo["text"] = text
						if fontFamily != "" {
							elementInfo["font_family"] = fontFamily
						}
					}
					if element.Shape.Placeholder != nil {
						elementInfo["placeholder_type"] = element.Shape.Placeholder.Type
						if element.Shape.Placeholder.Type == "TITLE" || element.Shape.Placeholder.Type == "CENTERED_TITLE" {
							slideInfo["title"] = elementInfo["text"]
						}
					}
				} else if element.Table != nil {
					elementInfo["type"] = "table"
					elementInfo["rows"] = element.Table.Rows
					elementInfo["columns"] = element.Table.Columns
				} else if element.Image != nil {
					elementInfo["type"] = "image"
					if element.Image.ContentUrl != "" {
						elementInfo["url"] = element.Image.ContentUrl
					}
				}

				elements = append(elements, elementInfo)
			}
			slideInfo["elements"] = elements
			slides = append(slides, slideInfo)
		}

		return map[string]interface{}{
			"presentation_id": presentation.PresentationId,
			"title":           presentation.Title,
			"slides_count":    len(presentation.Slides),
			"url":             fmt.Sprintf("https://docs.google.com/presentation/d/%s/edit", presentation.PresentationId),
			"slides":          slides,
		}, nil

	case "slides_slide_create":
		presentationId, _ := args["presentation_id"].(string)
		insertionIndex := 0
		if idx, ok := args["insertion_index"].(float64); ok {
			insertionIndex = int(idx)
		}

		resp, err := client.CreateSlide(presentationId, insertionIndex)
		if err != nil {
			return nil, err
		}

		if len(resp.Replies) > 0 && resp.Replies[0].CreateSlide != nil {
			return map[string]interface{}{
				"slide_id": resp.Replies[0].CreateSlide.ObjectId,
			}, nil
		}
		return map[string]interface{}{"success": true}, nil

	case "slides_slide_delete":
		presentationId, _ := args["presentation_id"].(string)
		slideId, _ := args["slide_id"].(string)

		_, err := client.DeleteSlide(presentationId, slideId)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"success": true}, nil

	case "slides_slide_duplicate":
		presentationId, _ := args["presentation_id"].(string)
		slideId, _ := args["slide_id"].(string)

		resp, err := client.DuplicateSlide(presentationId, slideId)
		if err != nil {
			return nil, err
		}

		if len(resp.Replies) > 0 && resp.Replies[0].DuplicateObject != nil {
			return map[string]interface{}{
				"new_slide_id": resp.Replies[0].DuplicateObject.ObjectId,
			}, nil
		}
		return map[string]interface{}{"success": true}, nil

	case "slides_markdown_create":
		title, _ := args["title"].(string)
		markdown, _ := args["markdown"].(string)

		// Create new presentation
		presentation, err := client.CreatePresentation(title)
		if err != nil {
			return nil, err
		}

		// Convert markdown and create slides
		converter := NewMarkdownConverter(client, presentation.PresentationId)
		slides, err := converter.CreateSlidesFromMarkdown(markdown)
		if err != nil {
			return nil, err
		}

		return map[string]interface{}{
			"presentation_id": presentation.PresentationId,
			"title":           presentation.Title,
			"slides_created":  len(slides),
			"url":             fmt.Sprintf("https://docs.google.com/presentation/d/%s/edit", presentation.PresentationId),
		}, nil

	case "slides_markdown_update":
		presentationId, _ := args["presentation_id"].(string)
		markdown, _ := args["markdown"].(string)

		converter := NewMarkdownConverter(client, presentationId)
		err := converter.UpdateSlidesFromMarkdown(markdown)
		if err != nil {
			return nil, err
		}

		return map[string]interface{}{
			"presentation_id": presentationId,
			"success":         true,
		}, nil

	case "slides_markdown_append":
		presentationId, _ := args["presentation_id"].(string)
		markdown, _ := args["markdown"].(string)

		converter := NewMarkdownConverter(client, presentationId)
		slides, err := converter.CreateSlidesFromMarkdown(markdown)
		if err != nil {
			return nil, err
		}

		return map[string]interface{}{
			"presentation_id": presentationId,
			"slides_added":    len(slides),
			"success":         true,
		}, nil

	case "slides_add_text":
		presentationId, _ := args["presentation_id"].(string)
		slideId, _ := args["slide_id"].(string)
		text, _ := args["text"].(string)

		x := getFloatOrDefault(args, "x", 50)
		y := getFloatOrDefault(args, "y", 50)
		width := getFloatOrDefault(args, "width", 300)
		height := getFloatOrDefault(args, "height", 100)

		_, err := client.AddTextBox(presentationId, slideId, text, x, y, width, height)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"success": true}, nil

	case "slides_add_image":
		presentationId, _ := args["presentation_id"].(string)
		slideId, _ := args["slide_id"].(string)
		imageUrl, _ := args["image_url"].(string)

		x := getFloatOrDefault(args, "x", 50)
		y := getFloatOrDefault(args, "y", 50)
		width := getFloatOrDefault(args, "width", 400)
		height := getFloatOrDefault(args, "height", 300)

		_, err := client.AddImage(presentationId, slideId, imageUrl, x, y, width, height)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"success": true}, nil

	case "slides_add_table":
		presentationId, _ := args["presentation_id"].(string)
		slideId, _ := args["slide_id"].(string)

		rows := int(getFloatOrDefault(args, "rows", 3))
		columns := int(getFloatOrDefault(args, "columns", 3))
		x := getFloatOrDefault(args, "x", 50)
		y := getFloatOrDefault(args, "y", 50)
		width := getFloatOrDefault(args, "width", 400)
		height := getFloatOrDefault(args, "height", 200)

		_, err := client.AddTable(presentationId, slideId, rows, columns, x, y, width, height)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"success": true}, nil

	case "slides_add_shape":
		presentationId, _ := args["presentation_id"].(string)
		slideId, _ := args["slide_id"].(string)
		shapeType, _ := args["shape_type"].(string)

		x := getFloatOrDefault(args, "x", 50)
		y := getFloatOrDefault(args, "y", 50)
		width := getFloatOrDefault(args, "width", 100)
		height := getFloatOrDefault(args, "height", 100)

		_, err := client.AddShape(presentationId, slideId, shapeType, x, y, width, height)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"success": true}, nil

	case "slides_set_layout":
		presentationId, _ := args["presentation_id"].(string)
		slideId, _ := args["slide_id"].(string)
		layoutId, _ := args["layout_id"].(string)

		_, err := client.SetSlideLayout(presentationId, slideId, layoutId)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"success": true}, nil

	case "slides_export_pdf":
		presentationId, _ := args["presentation_id"].(string)

		// Generate export URL
		exportUrl := fmt.Sprintf("https://docs.google.com/presentation/d/%s/export/pdf", presentationId)

		return map[string]interface{}{
			"presentation_id": presentationId,
			"export_url":      exportUrl,
		}, nil

	case "slides_share":
		// This would typically use Drive API for sharing
		presentationId, _ := args["presentation_id"].(string)
		role, _ := args["role"].(string)

		// For now, just return the public URL
		shareUrl := fmt.Sprintf("https://docs.google.com/presentation/d/%s/edit?usp=sharing", presentationId)

		return map[string]interface{}{
			"presentation_id": presentationId,
			"share_url":       shareUrl,
			"role":            role,
			"note":            "Use Drive API for actual permission management",
		}, nil

	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

func getFloatOrDefault(args map[string]interface{}, key string, defaultValue float64) float64 {
	if val, ok := args[key].(float64); ok {
		return val
	}
	return defaultValue
}
