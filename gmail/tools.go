package gmail

import (
	"context"
	"encoding/json"
	"fmt"

	"go.ngs.io/google-mcp-server/server"
)

// Handler implements the ServiceHandler interface for Gmail
type Handler struct {
	client *Client
}

// NewHandler creates a new Gmail handler
func NewHandler(client *Client) *Handler {
	return &Handler{client: client}
}

// GetTools returns the available Gmail tools
func (h *Handler) GetTools() []server.Tool {
	return []server.Tool{
		{
			Name:        "gmail_messages_list",
			Description: "List email messages",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"query": {
						Type:        "string",
						Description: "Search query (e.g., 'from:user@example.com')",
					},
					"max_results": {
						Type:        "number",
						Description: "Maximum number of results",
					},
				},
			},
		},
		{
			Name:        "gmail_message_get",
			Description: "Get email message details",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"message_id": {
						Type:        "string",
						Description: "Message ID",
					},
				},
				Required: []string{"message_id"},
			},
		},
	}
}

// HandleToolCall handles a tool call for Gmail service
func (h *Handler) HandleToolCall(ctx context.Context, name string, arguments json.RawMessage) (interface{}, error) {
	switch name {
	case "gmail_messages_list":
		var args struct {
			Query      string  `json:"query"`
			MaxResults float64 `json:"max_results"`
		}
		if err := json.Unmarshal(arguments, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
		messages, err := h.client.ListMessages(args.Query, int64(args.MaxResults))
		if err != nil {
			return nil, err
		}

		// Format messages for response
		messageList := make([]map[string]interface{}, len(messages))
		for i, msg := range messages {
			messageList[i] = map[string]interface{}{
				"id":       msg.Id,
				"threadId": msg.ThreadId,
			}
		}
		return map[string]interface{}{
			"messages": messageList,
		}, nil

	case "gmail_message_get":
		var args struct {
			MessageID string `json:"message_id"`
		}
		if err := json.Unmarshal(arguments, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
		message, err := h.client.GetMessage(args.MessageID)
		if err != nil {
			return nil, err
		}

		// Format message for response
		result := map[string]interface{}{
			"id":           message.Id,
			"threadId":     message.ThreadId,
			"labelIds":     message.LabelIds,
			"snippet":      message.Snippet,
			"historyId":    message.HistoryId,
			"internalDate": message.InternalDate,
			"sizeEstimate": message.SizeEstimate,
		}

		// Extract headers for easier access
		if message.Payload != nil && message.Payload.Headers != nil {
			headers := make(map[string]string)
			for _, header := range message.Payload.Headers {
				headers[header.Name] = header.Value
			}
			result["headers"] = headers

			// Add body if available
			if message.Payload.Body != nil && message.Payload.Body.Data != "" {
				result["body"] = message.Payload.Body.Data
			}
		}

		return result, nil

	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

// GetResources returns the available Gmail resources
func (h *Handler) GetResources() []server.Resource {
	return []server.Resource{
		{
			URI:         "gmail://inbox",
			Name:        "Inbox",
			Description: "Gmail inbox messages",
			MimeType:    "application/json",
		},
	}
}

// HandleResourceCall handles a resource call for Gmail service
func (h *Handler) HandleResourceCall(ctx context.Context, uri string) (interface{}, error) {
	if uri == "gmail://inbox" {
		messages, err := h.client.ListMessages("in:inbox", 20)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"messages": messages,
			"count":    len(messages),
		}, nil
	}
	return nil, fmt.Errorf("unknown resource: %s", uri)
}
