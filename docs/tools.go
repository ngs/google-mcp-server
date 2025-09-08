package docs

import (
	"context"
	"encoding/json"
	"fmt"

	"go.ngs.io/google-mcp-server/server"
)

// Handler implements the ServiceHandler interface for Docs
type Handler struct {
	client  *Client
	updater *DocumentUpdater
}

// NewHandler creates a new Docs handler
func NewHandler(client *Client) *Handler {
	return &Handler{
		client:  client,
		updater: NewDocumentUpdater(client),
	}
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
		{
			Name:        "docs_document_batch_update",
			Description: "Perform batch updates on a document with formatting",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"document_id": {
						Type:        "string",
						Description: "Document ID",
					},
					"requests": {
						Type:        "array",
						Description: "Array of batch update requests",
						Items: &server.Property{
							Type: "object",
							Properties: map[string]server.Property{
								"type": {
									Type:        "string",
									Description: "Request type: insertText, updateParagraphStyle, updateTextStyle, deleteContentRange",
								},
								"text": {
									Type:        "string",
									Description: "Text to insert (for insertText)",
								},
								"location": {
									Type:        "number",
									Description: "Index location for operation",
								},
								"startIndex": {
									Type:        "number",
									Description: "Start index for range operations",
								},
								"endIndex": {
									Type:        "number",
									Description: "End index for range operations",
								},
								"paragraphStyle": {
									Type:        "object",
									Description: "Paragraph style settings",
									Properties: map[string]server.Property{
										"namedStyleType": {
											Type:        "string",
											Description: "Named style type: NORMAL_TEXT, TITLE, HEADING_1, HEADING_2, etc.",
										},
										"alignment": {
											Type:        "string",
											Description: "Text alignment: START, CENTER, END, JUSTIFIED",
										},
									},
								},
								"textStyle": {
									Type:        "object",
									Description: "Text style settings",
									Properties: map[string]server.Property{
										"bold": {
											Type:        "boolean",
											Description: "Bold text",
										},
										"italic": {
											Type:        "boolean",
											Description: "Italic text",
										},
										"underline": {
											Type:        "boolean",
											Description: "Underlined text",
										},
										"fontSize": {
											Type:        "number",
											Description: "Font size in points",
										},
									},
								},
							},
						},
					},
				},
				Required: []string{"document_id", "requests"},
			},
		},
		{
			Name:        "docs_document_format",
			Description: "Format document content with rich text styles using Markdown",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"document_id": {
						Type:        "string",
						Description: "Document ID",
					},
					"markdown_content": {
						Type:        "string",
						Description: "Markdown formatted content to convert to rich text",
					},
					"mode": {
						Type:        "string",
						Description: "Update mode: 'append' (default) or 'replace'",
					},
				},
				Required: []string{"document_id", "markdown_content"},
			},
		},
		{
			Name:        "docs_document_update_html",
			Description: "Update document with HTML content",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"document_id": {
						Type:        "string",
						Description: "Document ID",
					},
					"html_content": {
						Type:        "string",
						Description: "HTML content to convert to rich text",
					},
					"mode": {
						Type:        "string",
						Description: "Update mode: 'append' (default) or 'replace'",
					},
				},
				Required: []string{"document_id", "html_content"},
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

	case "docs_document_batch_update":
		var args struct {
			DocumentID string                   `json:"document_id"`
			Requests   []map[string]interface{} `json:"requests"`
		}
		if err := json.Unmarshal(arguments, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}

		// Convert requests to Google Docs API format
		requests, err := h.convertBatchRequests(args.Requests)
		if err != nil {
			return nil, fmt.Errorf("failed to convert requests: %w", err)
		}

		// Execute batch update
		response, err := h.client.BatchUpdate(args.DocumentID, requests)
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

	case "docs_document_format":
		var args struct {
			DocumentID      string `json:"document_id"`
			MarkdownContent string `json:"markdown_content"`
			Mode            string `json:"mode"`
		}
		if err := json.Unmarshal(arguments, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}

		// Default to append mode
		if args.Mode == "" {
			args.Mode = "append"
		}

		// Use the DocumentUpdater to handle markdown conversion and API calls
		response, err := h.updater.UpdateWithMarkdown(ctx, args.DocumentID, args.MarkdownContent, args.Mode)
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

	case "docs_document_update_html":
		var args struct {
			DocumentID  string `json:"document_id"`
			HTMLContent string `json:"html_content"`
			Mode        string `json:"mode"`
		}
		if err := json.Unmarshal(arguments, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}

		// Default to append mode
		if args.Mode == "" {
			args.Mode = "append"
		}

		// Use the DocumentUpdater to handle HTML conversion and API calls
		response, err := h.updater.UpdateWithHTML(ctx, args.DocumentID, args.HTMLContent, args.Mode)
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
