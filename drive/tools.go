package drive

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"

	"go.ngs.io/google-mcp-server/server"
)

// Handler implements the ServiceHandler interface for Drive
type Handler struct {
	client *Client
}

// NewHandler creates a new Drive handler
func NewHandler(client *Client) *Handler {
	return &Handler{client: client}
}

// GetTools returns the available Drive tools
func (h *Handler) GetTools() []server.Tool {
	return []server.Tool{
		{
			Name:        "drive_files_list",
			Description: "List files and folders in Google Drive",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"parent_id": {
						Type:        "string",
						Description: "Parent folder ID (optional, defaults to root)",
					},
					"page_size": {
						Type:        "number",
						Description: "Number of files to return (max 1000)",
					},
				},
			},
		},
		{
			Name:        "drive_files_search",
			Description: "Search for files in Google Drive",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"name": {
						Type:        "string",
						Description: "File name to search for",
					},
					"mime_type": {
						Type:        "string",
						Description: "MIME type to filter by",
					},
					"modified_after": {
						Type:        "string",
						Description: "Modified after date (RFC3339 format)",
					},
				},
			},
		},
		{
			Name:        "drive_file_download",
			Description: "Download a file from Google Drive",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"file_id": {
						Type:        "string",
						Description: "File ID to download",
					},
				},
				Required: []string{"file_id"},
			},
		},
		{
			Name:        "drive_file_upload",
			Description: "Upload a file to Google Drive",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"name": {
						Type:        "string",
						Description: "File name",
					},
					"content": {
						Type:        "string",
						Description: "File content (base64 encoded for binary files)",
					},
					"mime_type": {
						Type:        "string",
						Description: "MIME type of the file",
					},
					"parent_id": {
						Type:        "string",
						Description: "Parent folder ID (optional)",
					},
				},
				Required: []string{"name", "content"},
			},
		},
		{
			Name:        "drive_file_get_metadata",
			Description: "Get metadata for a file",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"file_id": {
						Type:        "string",
						Description: "File ID",
					},
				},
				Required: []string{"file_id"},
			},
		},
		{
			Name:        "drive_file_update_metadata",
			Description: "Update file metadata",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"file_id": {
						Type:        "string",
						Description: "File ID",
					},
					"name": {
						Type:        "string",
						Description: "New file name",
					},
					"description": {
						Type:        "string",
						Description: "New file description",
					},
				},
				Required: []string{"file_id"},
			},
		},
		{
			Name:        "drive_folder_create",
			Description: "Create a new folder",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"name": {
						Type:        "string",
						Description: "Folder name",
					},
					"parent_id": {
						Type:        "string",
						Description: "Parent folder ID (optional)",
					},
				},
				Required: []string{"name"},
			},
		},
		{
			Name:        "drive_file_move",
			Description: "Move a file to another folder",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"file_id": {
						Type:        "string",
						Description: "File ID to move",
					},
					"new_parent_id": {
						Type:        "string",
						Description: "New parent folder ID",
					},
				},
				Required: []string{"file_id", "new_parent_id"},
			},
		},
		{
			Name:        "drive_file_copy",
			Description: "Copy a file",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"file_id": {
						Type:        "string",
						Description: "File ID to copy",
					},
					"new_name": {
						Type:        "string",
						Description: "Name for the copy",
					},
				},
				Required: []string{"file_id"},
			},
		},
		{
			Name:        "drive_file_delete",
			Description: "Permanently delete a file",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"file_id": {
						Type:        "string",
						Description: "File ID to delete",
					},
				},
				Required: []string{"file_id"},
			},
		},
		{
			Name:        "drive_file_trash",
			Description: "Move a file to trash",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"file_id": {
						Type:        "string",
						Description: "File ID to trash",
					},
				},
				Required: []string{"file_id"},
			},
		},
		{
			Name:        "drive_file_restore",
			Description: "Restore a file from trash",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"file_id": {
						Type:        "string",
						Description: "File ID to restore",
					},
				},
				Required: []string{"file_id"},
			},
		},
		{
			Name:        "drive_shared_link_create",
			Description: "Create a shareable link for a file",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"file_id": {
						Type:        "string",
						Description: "File ID",
					},
					"role": {
						Type:        "string",
						Description: "Permission role (reader, writer, commenter)",
						Enum:        []string{"reader", "writer", "commenter"},
					},
				},
				Required: []string{"file_id", "role"},
			},
		},
		{
			Name:        "drive_permissions_list",
			Description: "List permissions for a file",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"file_id": {
						Type:        "string",
						Description: "File ID",
					},
				},
				Required: []string{"file_id"},
			},
		},
		{
			Name:        "drive_permissions_create",
			Description: "Grant permission to a user",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"file_id": {
						Type:        "string",
						Description: "File ID",
					},
					"email": {
						Type:        "string",
						Description: "User email address",
					},
					"role": {
						Type:        "string",
						Description: "Permission role (reader, writer, commenter)",
						Enum:        []string{"reader", "writer", "commenter"},
					},
				},
				Required: []string{"file_id", "email", "role"},
			},
		},
		{
			Name:        "drive_permissions_delete",
			Description: "Remove a permission",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"file_id": {
						Type:        "string",
						Description: "File ID",
					},
					"permission_id": {
						Type:        "string",
						Description: "Permission ID to remove",
					},
				},
				Required: []string{"file_id", "permission_id"},
			},
		},
	}
}

// HandleToolCall handles a tool call for Drive service
func (h *Handler) HandleToolCall(ctx context.Context, name string, arguments json.RawMessage) (interface{}, error) {
	switch name {
	case "drive_files_list":
		var args struct {
			ParentID string  `json:"parent_id"`
			PageSize float64 `json:"page_size"`
		}
		if err := json.Unmarshal(arguments, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
		return h.handleFilesList(ctx, args.ParentID, int64(args.PageSize))

	case "drive_files_search":
		var args struct {
			Name          string `json:"name"`
			MimeType      string `json:"mime_type"`
			ModifiedAfter string `json:"modified_after"`
		}
		if err := json.Unmarshal(arguments, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
		return h.handleFilesSearch(ctx, args.Name, args.MimeType, args.ModifiedAfter)

	case "drive_file_download":
		var args struct {
			FileID string `json:"file_id"`
		}
		if err := json.Unmarshal(arguments, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
		return h.handleFileDownload(ctx, args.FileID)

	case "drive_file_upload":
		var args struct {
			Name     string `json:"name"`
			Content  string `json:"content"`
			MimeType string `json:"mime_type"`
			ParentID string `json:"parent_id"`
		}
		if err := json.Unmarshal(arguments, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
		return h.handleFileUpload(ctx, args.Name, args.Content, args.MimeType, args.ParentID)

	case "drive_file_get_metadata":
		var args struct {
			FileID string `json:"file_id"`
		}
		if err := json.Unmarshal(arguments, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
		return h.handleFileGetMetadata(ctx, args.FileID)

	case "drive_file_update_metadata":
		var args struct {
			FileID      string `json:"file_id"`
			Name        string `json:"name"`
			Description string `json:"description"`
		}
		if err := json.Unmarshal(arguments, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
		return h.handleFileUpdateMetadata(ctx, args.FileID, args.Name, args.Description)

	case "drive_folder_create":
		var args struct {
			Name     string `json:"name"`
			ParentID string `json:"parent_id"`
		}
		if err := json.Unmarshal(arguments, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
		return h.handleFolderCreate(ctx, args.Name, args.ParentID)

	case "drive_file_move":
		var args struct {
			FileID      string `json:"file_id"`
			NewParentID string `json:"new_parent_id"`
		}
		if err := json.Unmarshal(arguments, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
		return h.handleFileMove(ctx, args.FileID, args.NewParentID)

	case "drive_file_copy":
		var args struct {
			FileID  string `json:"file_id"`
			NewName string `json:"new_name"`
		}
		if err := json.Unmarshal(arguments, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
		return h.handleFileCopy(ctx, args.FileID, args.NewName)

	case "drive_file_delete":
		var args struct {
			FileID string `json:"file_id"`
		}
		if err := json.Unmarshal(arguments, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
		return h.handleFileDelete(ctx, args.FileID)

	case "drive_file_trash":
		var args struct {
			FileID string `json:"file_id"`
		}
		if err := json.Unmarshal(arguments, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
		return h.handleFileTrash(ctx, args.FileID)

	case "drive_file_restore":
		var args struct {
			FileID string `json:"file_id"`
		}
		if err := json.Unmarshal(arguments, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
		return h.handleFileRestore(ctx, args.FileID)

	case "drive_shared_link_create":
		var args struct {
			FileID string `json:"file_id"`
			Role   string `json:"role"`
		}
		if err := json.Unmarshal(arguments, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
		return h.handleSharedLinkCreate(ctx, args.FileID, args.Role)

	case "drive_permissions_list":
		var args struct {
			FileID string `json:"file_id"`
		}
		if err := json.Unmarshal(arguments, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
		return h.handlePermissionsList(ctx, args.FileID)

	case "drive_permissions_create":
		var args struct {
			FileID string `json:"file_id"`
			Email  string `json:"email"`
			Role   string `json:"role"`
		}
		if err := json.Unmarshal(arguments, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
		return h.handlePermissionsCreate(ctx, args.FileID, args.Email, args.Role)

	case "drive_permissions_delete":
		var args struct {
			FileID       string `json:"file_id"`
			PermissionID string `json:"permission_id"`
		}
		if err := json.Unmarshal(arguments, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
		return h.handlePermissionsDelete(ctx, args.FileID, args.PermissionID)

	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

// Tool handler implementations
func (h *Handler) handleFilesList(ctx context.Context, parentID string, pageSize int64) (interface{}, error) {
	if pageSize <= 0 {
		pageSize = 100
	}

	files, err := h.client.ListFiles("", pageSize, parentID)
	if err != nil {
		return nil, err
	}

	return formatFiles(files), nil
}

func (h *Handler) handleFilesSearch(ctx context.Context, name, mimeType, modifiedAfter string) (interface{}, error) {
	files, err := h.client.SearchFiles(name, mimeType, modifiedAfter)
	if err != nil {
		return nil, err
	}

	return formatFiles(files), nil
}

func (h *Handler) handleFileDownload(ctx context.Context, fileID string) (interface{}, error) {
	var buf bytes.Buffer
	err := h.client.DownloadFile(fileID, &buf)
	if err != nil {
		return nil, err
	}

	// Return base64 encoded content
	return map[string]interface{}{
		"file_id": fileID,
		"content": base64.StdEncoding.EncodeToString(buf.Bytes()),
		"size":    buf.Len(),
	}, nil
}

func (h *Handler) handleFileUpload(ctx context.Context, name, content, mimeType, parentID string) (interface{}, error) {
	// Decode base64 content if needed
	var reader io.Reader
	if content != "" {
		decoded, err := base64.StdEncoding.DecodeString(content)
		if err != nil {
			// Try as plain text if base64 decode fails
			reader = bytes.NewReader([]byte(content))
		} else {
			reader = bytes.NewReader(decoded)
		}
	}

	if mimeType == "" {
		mimeType = "text/plain"
	}

	file, err := h.client.UploadFile(name, mimeType, reader, parentID)
	if err != nil {
		return nil, err
	}

	return formatFile(file), nil
}

func (h *Handler) handleFileGetMetadata(ctx context.Context, fileID string) (interface{}, error) {
	file, err := h.client.GetFile(fileID)
	if err != nil {
		return nil, err
	}

	return formatFile(file), nil
}

func (h *Handler) handleFileUpdateMetadata(ctx context.Context, fileID, name, description string) (interface{}, error) {
	file, err := h.client.UpdateFileMetadata(fileID, name, description)
	if err != nil {
		return nil, err
	}

	return formatFile(file), nil
}

func (h *Handler) handleFolderCreate(ctx context.Context, name, parentID string) (interface{}, error) {
	folder, err := h.client.CreateFolder(name, parentID)
	if err != nil {
		return nil, err
	}

	return formatFile(folder), nil
}

func (h *Handler) handleFileMove(ctx context.Context, fileID, newParentID string) (interface{}, error) {
	file, err := h.client.MoveFile(fileID, newParentID)
	if err != nil {
		return nil, err
	}

	return formatFile(file), nil
}

func (h *Handler) handleFileCopy(ctx context.Context, fileID, newName string) (interface{}, error) {
	file, err := h.client.CopyFile(fileID, newName)
	if err != nil {
		return nil, err
	}

	return formatFile(file), nil
}

func (h *Handler) handleFileDelete(ctx context.Context, fileID string) (interface{}, error) {
	err := h.client.DeleteFile(fileID)
	if err != nil {
		return nil, err
	}

	return map[string]string{"status": "deleted", "file_id": fileID}, nil
}

func (h *Handler) handleFileTrash(ctx context.Context, fileID string) (interface{}, error) {
	err := h.client.TrashFile(fileID)
	if err != nil {
		return nil, err
	}

	return map[string]string{"status": "trashed", "file_id": fileID}, nil
}

func (h *Handler) handleFileRestore(ctx context.Context, fileID string) (interface{}, error) {
	err := h.client.RestoreFile(fileID)
	if err != nil {
		return nil, err
	}

	return map[string]string{"status": "restored", "file_id": fileID}, nil
}

func (h *Handler) handleSharedLinkCreate(ctx context.Context, fileID, role string) (interface{}, error) {
	link, err := h.client.CreateShareLink(fileID, role)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"file_id": fileID,
		"link":    link,
		"role":    role,
	}, nil
}

func (h *Handler) handlePermissionsList(ctx context.Context, fileID string) (interface{}, error) {
	permissions, err := h.client.ListPermissions(fileID)
	if err != nil {
		return nil, err
	}

	result := make([]map[string]interface{}, len(permissions))
	for i, perm := range permissions {
		result[i] = map[string]interface{}{
			"id":           perm.Id,
			"type":         perm.Type,
			"role":         perm.Role,
			"emailAddress": perm.EmailAddress,
		}
	}

	return result, nil
}

func (h *Handler) handlePermissionsCreate(ctx context.Context, fileID, email, role string) (interface{}, error) {
	permission, err := h.client.CreatePermission(fileID, email, role)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"id":           permission.Id,
		"type":         permission.Type,
		"role":         permission.Role,
		"emailAddress": permission.EmailAddress,
	}, nil
}

func (h *Handler) handlePermissionsDelete(ctx context.Context, fileID, permissionID string) (interface{}, error) {
	err := h.client.DeletePermission(fileID, permissionID)
	if err != nil {
		return nil, err
	}

	return map[string]string{"status": "deleted", "permission_id": permissionID}, nil
}

// formatFile formats a drive file for response
func formatFile(file interface{}) map[string]interface{} {
	data := make(map[string]interface{})
	jsonData, _ := json.Marshal(file)
	_ = json.Unmarshal(jsonData, &data)
	return data
}

// formatFiles formats multiple drive files for response
func formatFiles(files interface{}) []map[string]interface{} {
	var result []map[string]interface{}
	jsonData, _ := json.Marshal(files)
	_ = json.Unmarshal(jsonData, &result)
	return result
}
