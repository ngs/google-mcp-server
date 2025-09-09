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
			Description: "Create a new plain text document (for Markdown use drive_markdown_upload instead)",
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
			Description: "Update plain text document content - append or replace (for Markdown use drive_markdown_replace)",
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

		// Extract text content from body
		if doc.Body != nil && doc.Body.Content != nil {
			var textContent string
			for _, element := range doc.Body.Content {
				if element.Paragraph != nil {
					for _, elem := range element.Paragraph.Elements {
						if elem.TextRun != nil {
							textContent += elem.TextRun.Content
						}
					}
				}
			}
			result["content"] = textContent
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
