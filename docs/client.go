package docs

import (
	"context"
	"fmt"

	"go.ngs.io/google-mcp-server/auth"
	"google.golang.org/api/docs/v1"
)

// Client wraps the Google Docs API client
type Client struct {
	service *docs.Service
}

// Ensure Client implements ClientInterface
var _ ClientInterface = (*Client)(nil)

// NewClient creates a new Docs client
func NewClient(ctx context.Context, oauth *auth.OAuthClient) (*Client, error) {
	service, err := docs.NewService(ctx, oauth.GetClientOption())
	if err != nil {
		return nil, fmt.Errorf("failed to create docs service: %w", err)
	}

	return &Client{
		service: service,
	}, nil
}

// GetDocument gets a document by ID
func (c *Client) GetDocument(documentID string) (*docs.Document, error) {
	doc, err := c.service.Documents.Get(documentID).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get document: %w", err)
	}
	return doc, nil
}

// CreateDocument creates a new document
func (c *Client) CreateDocument(title string) (*docs.Document, error) {
	doc := &docs.Document{
		Title: title,
	}
	created, err := c.service.Documents.Create(doc).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to create document: %w", err)
	}
	return created, nil
}

// BatchUpdate performs batch updates on a document
func (c *Client) BatchUpdate(documentID string, requests []*docs.Request) (*docs.BatchUpdateDocumentResponse, error) {
	batchUpdate := &docs.BatchUpdateDocumentRequest{
		Requests: requests,
	}
	response, err := c.service.Documents.BatchUpdate(documentID, batchUpdate).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to batch update: %w", err)
	}
	return response, nil
}

// UpdateDocument updates a document's content
func (c *Client) UpdateDocument(documentID string, content string, mode string) (*docs.BatchUpdateDocumentResponse, error) {
	var requests []*docs.Request

	if mode == "replace" {
		// First, get the document to find the end index
		doc, err := c.GetDocument(documentID)
		if err != nil {
			return nil, fmt.Errorf("failed to get document for replacement: %w", err)
		}

		// Find the end index of the document content
		endIndex := int64(1) // Default to 1 if document is empty
		if doc.Body != nil && doc.Body.Content != nil && len(doc.Body.Content) > 0 {
			lastElement := doc.Body.Content[len(doc.Body.Content)-1]
			if lastElement.EndIndex > 0 {
				endIndex = lastElement.EndIndex - 1 // Subtract 1 to avoid the final newline
			}
		}

		// Delete existing content (if any)
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

		// Insert new content at the beginning
		requests = append(requests, &docs.Request{
			InsertText: &docs.InsertTextRequest{
				Location: &docs.Location{
					Index: 1,
				},
				Text: content,
			},
		})
	} else {
		// Append mode: get the document to find where to append
		doc, err := c.GetDocument(documentID)
		if err != nil {
			return nil, fmt.Errorf("failed to get document for appending: %w", err)
		}

		// Find the end index to append content
		appendIndex := int64(1) // Default to 1 if document is empty
		if doc.Body != nil && doc.Body.Content != nil && len(doc.Body.Content) > 0 {
			lastElement := doc.Body.Content[len(doc.Body.Content)-1]
			if lastElement.EndIndex > 0 {
				appendIndex = lastElement.EndIndex - 1 // Insert before the final newline
			}
		}

		// Insert text at the end
		requests = append(requests, &docs.Request{
			InsertText: &docs.InsertTextRequest{
				Location: &docs.Location{
					Index: appendIndex,
				},
				Text: content,
			},
		})
	}

	return c.BatchUpdate(documentID, requests)
}
