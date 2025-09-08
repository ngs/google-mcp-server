package photos

import (
	"context"
	"encoding/json"
	"fmt"

	"go.ngs.io/google-mcp-server/server"
)

// Handler implements the ServiceHandler interface for Photos
type Handler struct {
	client *Client
}

// NewHandler creates a new Photos handler
func NewHandler(client *Client) *Handler {
	return &Handler{client: client}
}

// GetTools returns the available Photos tools
func (h *Handler) GetTools() []server.Tool {
	return []server.Tool{
		{
			Name:        "photos_albums_list",
			Description: "List photo albums",
			InputSchema: server.InputSchema{
				Type:       "object",
				Properties: map[string]server.Property{},
			},
		},
		{
			Name:        "photos_album_get",
			Description: "Get album details",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"album_id": {
						Type:        "string",
						Description: "Album ID",
					},
				},
				Required: []string{"album_id"},
			},
		},
	}
}

// HandleToolCall handles a tool call for Photos service
func (h *Handler) HandleToolCall(ctx context.Context, name string, arguments json.RawMessage) (interface{}, error) {
	switch name {
	case "photos_albums_list":
		albums, err := h.client.ListAlbums()
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"albums": albums,
		}, nil

	case "photos_album_get":
		var args struct {
			AlbumID string `json:"album_id"`
		}
		if err := json.Unmarshal(arguments, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
		album, err := h.client.GetAlbum(args.AlbumID)
		if err != nil {
			return nil, err
		}
		return album, nil

	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

// GetResources returns the available Photos resources
func (h *Handler) GetResources() []server.Resource {
	return []server.Resource{
		{
			URI:         "photos://albums",
			Name:        "Photo Albums",
			Description: "List of photo albums",
			MimeType:    "application/json",
		},
	}
}

// HandleResourceCall handles a resource call for Photos service
func (h *Handler) HandleResourceCall(ctx context.Context, uri string) (interface{}, error) {
	if uri == "photos://albums" {
		albums, err := h.client.ListAlbums()
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"albums": albums,
			"count":  len(albums),
		}, nil
	}
	return nil, fmt.Errorf("unknown resource: %s", uri)
}
