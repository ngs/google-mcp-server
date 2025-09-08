package docs

import (
	"context"
	"encoding/json"
	"fmt"

	"go.ngs.io/google-mcp-server/server"
)

// Handler implements the ServiceHandler interface for Docs
type Handler struct {
	client *Client
}

// NewHandler creates a new Docs handler
func NewHandler(client *Client) *Handler {
	return &Handler{client: client}
}

// GetTools returns the available Docs tools
func (h *Handler) GetTools() []server.Tool {
	return []server.Tool{
		{
			Name:        "docs_document_get",
			Description: "Get document content",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"document_id": {
						Type:        "string",
						Description: "Document ID",
					},
				},
				Required: []string{"document_id"},
			},
		},
		{
			Name:        "docs_document_create",
			Description: "Create a new document",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"title": {
						Type:        "string",
						Description: "Document title",
					},
				},
				Required: []string{"title"},
			},
		},
		{
			Name:        "docs_document_update",
			Description: "Update document content (append text or replace all content)",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"document_id": {
						Type:        "string",
						Description: "Document ID",
					},
					"content": {
						Type:        "string",
						Description: "Text content to add to the document",
					},
					"mode": {
						Type:        "string",
						Description: "Update mode: 'append' (default) or 'replace'",
					},
				},
				Required: []string{"document_id", "content"},
			},
		},
	}
}

// HandleToolCall handles a tool call for Docs service
func (h *Handler) HandleToolCall(ctx context.Context, name string, arguments json.RawMessage) (interface{}, error) {
	switch name {
	case "docs_document_get":
		var args struct {
			DocumentID string `json:"document_id"`
		}
		if err := json.Unmarshal(arguments, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
		doc, err := h.client.GetDocument(args.DocumentID)
		if err != nil {
			return nil, err
		}

		// Format document for response
		result := map[string]interface{}{
			"documentId": doc.DocumentId,
			"title":      doc.Title,
		}

		// Extract text content and structure from body
		if doc.Body != nil && doc.Body.Content != nil {
			var textContent string
			var elements []map[string]interface{}
			
			for _, element := range doc.Body.Content {
				elemData := map[string]interface{}{
					"startIndex": element.StartIndex,
					"endIndex":   element.EndIndex,
				}
				
				if element.Paragraph != nil {
					paragraph := element.Paragraph
					paragraphData := map[string]interface{}{}
					
					// Add paragraph style information
					if paragraph.ParagraphStyle != nil {
						styleData := map[string]interface{}{}
						if paragraph.ParagraphStyle.NamedStyleType != "" {
							styleData["namedStyleType"] = paragraph.ParagraphStyle.NamedStyleType
						}
						if paragraph.ParagraphStyle.HeadingId != "" {
							styleData["headingId"] = paragraph.ParagraphStyle.HeadingId
						}
						if paragraph.ParagraphStyle.IndentStart != nil {
							styleData["indentStart"] = paragraph.ParagraphStyle.IndentStart.Magnitude
						}
						if len(styleData) > 0 {
							paragraphData["style"] = styleData
						}
					}
					
					// Add bullet/list information
					if paragraph.Bullet != nil {
						bulletData := map[string]interface{}{
							"nestingLevel": paragraph.Bullet.NestingLevel,
						}
						if paragraph.Bullet.ListId != "" {
							bulletData["listId"] = paragraph.Bullet.ListId
						}
						paragraphData["bullet"] = bulletData
					}
					
					// Add text runs with their styles
					var textRuns []map[string]interface{}
					for _, elem := range paragraph.Elements {
						if elem.TextRun != nil {
							textRun := elem.TextRun
							textContent += textRun.Content
							
							runData := map[string]interface{}{
								"content": textRun.Content,
							}
							
							// Add text style information
							if textRun.TextStyle != nil {
								style := textRun.TextStyle
								styleData := map[string]interface{}{}
								
								if style.Bold {
									styleData["bold"] = true
								}
								if style.Italic {
									styleData["italic"] = true
								}
								if style.Underline {
									styleData["underline"] = true
								}
								if style.Strikethrough {
									styleData["strikethrough"] = true
								}
								if style.WeightedFontFamily != nil && style.WeightedFontFamily.FontFamily != "" {
									styleData["fontFamily"] = style.WeightedFontFamily.FontFamily
								}
								if style.ForegroundColor != nil && style.ForegroundColor.Color != nil && style.ForegroundColor.Color.RgbColor != nil {
									rgb := style.ForegroundColor.Color.RgbColor
									styleData["color"] = map[string]float64{
										"red":   rgb.Red,
										"green": rgb.Green,
										"blue":  rgb.Blue,
									}
								}
								
								if len(styleData) > 0 {
									runData["style"] = styleData
								}
							}
							
							textRuns = append(textRuns, runData)
						}
					}
					
					if len(textRuns) > 0 {
						paragraphData["textRuns"] = textRuns
					}
					
					if len(paragraphData) > 0 {
						elemData["paragraph"] = paragraphData
					}
				}
				
				elements = append(elements, elemData)
			}
			
			result["content"] = textContent
			result["elements"] = elements
		}

		return result, nil

	case "docs_document_create":
		var args struct {
			Title string `json:"title"`
		}
		if err := json.Unmarshal(arguments, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
		doc, err := h.client.CreateDocument(args.Title)
		if err != nil {
			return nil, err
		}

		// Format created document response
		result := map[string]interface{}{
			"documentId": doc.DocumentId,
			"title":      doc.Title,
			"revisionId": doc.RevisionId,
		}
		return result, nil

	case "docs_document_update":
		var args struct {
			DocumentID string `json:"document_id"`
			Content    string `json:"content"`
			Mode       string `json:"mode"`
		}
		if err := json.Unmarshal(arguments, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}

		// Default to append mode
		if args.Mode == "" {
			args.Mode = "append"
		}

		// Update the document
		response, err := h.client.UpdateDocument(args.DocumentID, args.Content, args.Mode)
		if err != nil {
			return nil, err
		}

		// Format response
		result := map[string]interface{}{
			"documentId": response.DocumentId,
			"replies":    len(response.Replies),
			"success":    true,
		}
		return result, nil

	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

// GetResources returns the available Docs resources
func (h *Handler) GetResources() []server.Resource {
	return []server.Resource{}
}

// HandleResourceCall handles a resource call for Docs service
func (h *Handler) HandleResourceCall(ctx context.Context, uri string) (interface{}, error) {
	return nil, fmt.Errorf("no resources available for docs")
}
