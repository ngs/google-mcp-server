package drive

import (
	"context"
	"fmt"
	"io"
	"os"

	"go.ngs.io/google-mcp-server/auth"
	"google.golang.org/api/drive/v3"
)

// Client wraps the Google Drive API client
type Client struct {
	service *drive.Service
	ctx     context.Context
}

// NewClient creates a new Drive client
func NewClient(ctx context.Context, oauth *auth.OAuthClient) (*Client, error) {
	service, err := drive.NewService(ctx, oauth.GetClientOption())
	if err != nil {
		return nil, fmt.Errorf("failed to create drive service: %w", err)
	}

	return &Client{
		service: service,
		ctx:     ctx,
	}, nil
}

// ListFiles lists files and folders
func (c *Client) ListFiles(query string, pageSize int64, parentID string) ([]*drive.File, error) {
	call := c.service.Files.List().
		Fields("nextPageToken, files(id, name, mimeType, size, modifiedTime, parents, webViewLink, iconLink, thumbnailLink)")
	
	if query != "" {
		call = call.Q(query)
	}
	if pageSize > 0 {
		call = call.PageSize(pageSize)
	}
	if parentID != "" {
		call = call.Q(fmt.Sprintf("'%s' in parents", parentID))
	}

	var files []*drive.File
	err := call.Pages(c.ctx, func(page *drive.FileList) error {
		files = append(files, page.Files...)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	return files, nil
}

// SearchFiles searches for files
func (c *Client) SearchFiles(name, mimeType string, modifiedAfter string) ([]*drive.File, error) {
	query := ""
	if name != "" {
		query = fmt.Sprintf("name contains '%s'", name)
	}
	if mimeType != "" {
		if query != "" {
			query += " and "
		}
		query += fmt.Sprintf("mimeType = '%s'", mimeType)
	}
	if modifiedAfter != "" {
		if query != "" {
			query += " and "
		}
		query += fmt.Sprintf("modifiedTime > '%s'", modifiedAfter)
	}

	return c.ListFiles(query, 100, "")
}

// GetFile gets file metadata
func (c *Client) GetFile(fileID string) (*drive.File, error) {
	file, err := c.service.Files.Get(fileID).
		Fields("id, name, mimeType, size, modifiedTime, parents, webViewLink, iconLink, thumbnailLink, permissions").
		Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get file: %w", err)
	}
	return file, nil
}

// DownloadFile downloads a file
func (c *Client) DownloadFile(fileID string, writer io.Writer) error {
	resp, err := c.service.Files.Get(fileID).Download()
	if err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	_, err = io.Copy(writer, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// UploadFile uploads a file
func (c *Client) UploadFile(name string, mimeType string, reader io.Reader, parentID string) (*drive.File, error) {
	file := &drive.File{
		Name:     name,
		MimeType: mimeType,
	}
	
	if parentID != "" {
		file.Parents = []string{parentID}
	}

	call := c.service.Files.Create(file)
	if reader != nil {
		call = call.Media(reader)
	}

	created, err := call.Do()
	if err != nil {
		return nil, fmt.Errorf("failed to upload file: %w", err)
	}

	return created, nil
}

// UpdateFileMetadata updates file metadata
func (c *Client) UpdateFileMetadata(fileID, name, description string) (*drive.File, error) {
	file := &drive.File{}
	if name != "" {
		file.Name = name
	}
	if description != "" {
		file.Description = description
	}

	updated, err := c.service.Files.Update(fileID, file).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to update file metadata: %w", err)
	}

	return updated, nil
}

// CreateFolder creates a folder
func (c *Client) CreateFolder(name string, parentID string) (*drive.File, error) {
	folder := &drive.File{
		Name:     name,
		MimeType: "application/vnd.google-apps.folder",
	}
	
	if parentID != "" {
		folder.Parents = []string{parentID}
	}

	created, err := c.service.Files.Create(folder).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to create folder: %w", err)
	}

	return created, nil
}

// MoveFile moves a file to a different folder
func (c *Client) MoveFile(fileID, newParentID string) (*drive.File, error) {
	// Get current parents
	file, err := c.GetFile(fileID)
	if err != nil {
		return nil, err
	}

	// Remove from current parents and add to new parent
	var removeParents string
	if len(file.Parents) > 0 {
		removeParents = file.Parents[0]
	}

	updated, err := c.service.Files.Update(fileID, &drive.File{}).
		AddParents(newParentID).
		RemoveParents(removeParents).
		Fields("id, parents").
		Do()
	
	if err != nil {
		return nil, fmt.Errorf("failed to move file: %w", err)
	}

	return updated, nil
}

// CopyFile copies a file
func (c *Client) CopyFile(fileID, newName string) (*drive.File, error) {
	copy := &drive.File{}
	if newName != "" {
		copy.Name = newName
	}

	copied, err := c.service.Files.Copy(fileID, copy).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to copy file: %w", err)
	}

	return copied, nil
}

// DeleteFile deletes a file
func (c *Client) DeleteFile(fileID string) error {
	err := c.service.Files.Delete(fileID).Do()
	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}
	return nil
}

// TrashFile moves a file to trash
func (c *Client) TrashFile(fileID string) error {
	_, err := c.service.Files.Update(fileID, &drive.File{Trashed: true}).Do()
	if err != nil {
		return fmt.Errorf("failed to trash file: %w", err)
	}
	return nil
}

// RestoreFile restores a file from trash
func (c *Client) RestoreFile(fileID string) error {
	_, err := c.service.Files.Update(fileID, &drive.File{Trashed: false}).Do()
	if err != nil {
		return fmt.Errorf("failed to restore file: %w", err)
	}
	return nil
}

// CreateShareLink creates a shareable link
func (c *Client) CreateShareLink(fileID string, role string) (string, error) {
	permission := &drive.Permission{
		Type: "anyone",
		Role: role, // "reader", "writer", etc.
	}

	_, err := c.service.Permissions.Create(fileID, permission).Do()
	if err != nil {
		return "", fmt.Errorf("failed to create share link: %w", err)
	}

	file, err := c.GetFile(fileID)
	if err != nil {
		return "", err
	}

	return file.WebViewLink, nil
}

// ListPermissions lists file permissions
func (c *Client) ListPermissions(fileID string) ([]*drive.Permission, error) {
	permissions, err := c.service.Permissions.List(fileID).
		Fields("permissions(id, type, role, emailAddress)").
		Do()
	if err != nil {
		return nil, fmt.Errorf("failed to list permissions: %w", err)
	}

	return permissions.Permissions, nil
}

// CreatePermission creates a permission
func (c *Client) CreatePermission(fileID, email, role string) (*drive.Permission, error) {
	permission := &drive.Permission{
		Type:         "user",
		Role:         role,
		EmailAddress: email,
	}

	created, err := c.service.Permissions.Create(fileID, permission).
		SendNotificationEmail(true).
		Do()
	if err != nil {
		return nil, fmt.Errorf("failed to create permission: %w", err)
	}

	return created, nil
}

// DeletePermission deletes a permission
func (c *Client) DeletePermission(fileID, permissionID string) error {
	err := c.service.Permissions.Delete(fileID, permissionID).Do()
	if err != nil {
		return fmt.Errorf("failed to delete permission: %w", err)
	}
	return nil
}

// ExportFile exports a Google Workspace file
func (c *Client) ExportFile(fileID, mimeType string) (io.ReadCloser, error) {
	resp, err := c.service.Files.Export(fileID, mimeType).Download()
	if err != nil {
		return nil, fmt.Errorf("failed to export file: %w", err)
	}
	return resp.Body, nil
}

// UploadFileFromPath uploads a file from filesystem path
func (c *Client) UploadFileFromPath(filePath string, parentID string) (*drive.File, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Detect MIME type (simplified - in production use proper detection)
	mimeType := "application/octet-stream"
	
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	return c.UploadFile(info.Name(), mimeType, file, parentID)
}