package drive

import (
	"context"
	"fmt"
	"strings"

	"go.ngs.io/google-mcp-server/server"
)

// GetResources returns the available Drive resources
func (h *Handler) GetResources() []server.Resource {
	return []server.Resource{
		{
			URI:         "drive://root",
			Name:        "Drive Root",
			Description: "Root folder of Google Drive",
			MimeType:    "application/json",
		},
		{
			URI:         "drive://recent",
			Name:        "Recent Files",
			Description: "Recently accessed files",
			MimeType:    "application/json",
		},
		{
			URI:         "drive://starred",
			Name:        "Starred Files",
			Description: "Starred files and folders",
			MimeType:    "application/json",
		},
		{
			URI:         "drive://trash",
			Name:        "Trash",
			Description: "Files in trash",
			MimeType:    "application/json",
		},
	}
}

// HandleResourceCall handles a resource call for Drive service
func (h *Handler) HandleResourceCall(ctx context.Context, uri string) (interface{}, error) {
	if !strings.HasPrefix(uri, "drive://") {
		return nil, fmt.Errorf("invalid drive URI: %s", uri)
	}
	
	path := strings.TrimPrefix(uri, "drive://")
	
	switch path {
	case "root":
		return h.getRootFiles(ctx)
	case "recent":
		return h.getRecentFiles(ctx)
	case "starred":
		return h.getStarredFiles(ctx)
	case "trash":
		return h.getTrashedFiles(ctx)
	default:
		return nil, fmt.Errorf("unknown drive resource: %s", uri)
	}
}

func (h *Handler) getRootFiles(ctx context.Context) (interface{}, error) {
	files, err := h.client.ListFiles("'root' in parents and trashed = false", 100, "")
	if err != nil {
		return nil, fmt.Errorf("failed to get root files: %w", err)
	}
	
	return map[string]interface{}{
		"location": "root",
		"files":    formatFiles(files),
		"count":    len(files),
	}, nil
}

func (h *Handler) getRecentFiles(ctx context.Context) (interface{}, error) {
	files, err := h.client.ListFiles("trashed = false", 20, "")
	if err != nil {
		return nil, fmt.Errorf("failed to get recent files: %w", err)
	}
	
	return map[string]interface{}{
		"location": "recent",
		"files":    formatFiles(files),
		"count":    len(files),
	}, nil
}

func (h *Handler) getStarredFiles(ctx context.Context) (interface{}, error) {
	files, err := h.client.ListFiles("starred = true and trashed = false", 100, "")
	if err != nil {
		return nil, fmt.Errorf("failed to get starred files: %w", err)
	}
	
	return map[string]interface{}{
		"location": "starred",
		"files":    formatFiles(files),
		"count":    len(files),
	}, nil
}

func (h *Handler) getTrashedFiles(ctx context.Context) (interface{}, error) {
	files, err := h.client.ListFiles("trashed = true", 100, "")
	if err != nil {
		return nil, fmt.Errorf("failed to get trashed files: %w", err)
	}
	
	return map[string]interface{}{
		"location": "trash",
		"files":    formatFiles(files),
		"count":    len(files),
	}, nil
}