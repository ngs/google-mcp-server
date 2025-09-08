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
