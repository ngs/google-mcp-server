package gmail

import (
	"context"
	"fmt"

	"go.ngs.io/google-mcp-server/auth"
	"google.golang.org/api/gmail/v1"
)

// Client wraps the Google Gmail API client
type Client struct {
	service *gmail.Service
	ctx     context.Context
}

// NewClient creates a new Gmail client
func NewClient(ctx context.Context, oauth *auth.OAuthClient) (*Client, error) {
	service, err := gmail.NewService(ctx, oauth.GetClientOption())
	if err != nil {
		return nil, fmt.Errorf("failed to create gmail service: %w", err)
	}

	return &Client{
		service: service,
		ctx:     ctx,
	}, nil
}

// ListMessages lists messages
func (c *Client) ListMessages(query string, maxResults int64) ([]*gmail.Message, error) {
	call := c.service.Users.Messages.List("me")
	if query != "" {
		call = call.Q(query)
	}
	if maxResults > 0 {
		call = call.MaxResults(maxResults)
	}

	response, err := call.Do()
	if err != nil {
		return nil, fmt.Errorf("failed to list messages: %w", err)
	}

	return response.Messages, nil
}

// GetMessage gets a message by ID
func (c *Client) GetMessage(messageID string) (*gmail.Message, error) {
	message, err := c.service.Users.Messages.Get("me", messageID).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get message: %w", err)
	}
	return message, nil
}