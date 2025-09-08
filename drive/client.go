package drive

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/yuin/goldmark"
	emoji "github.com/yuin/goldmark-emoji"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	goldmarkhtml "github.com/yuin/goldmark/renderer/html"
	"go.ngs.io/google-mcp-server/auth"
	"google.golang.org/api/drive/v3"
)

// Client wraps the Google Drive API client
type Client struct {
	service *drive.Service
}

// NewClient creates a new Drive client
func NewClient(ctx context.Context, oauth *auth.OAuthClient) (*Client, error) {
	service, err := drive.NewService(ctx, oauth.GetClientOption())
	if err != nil {
		return nil, fmt.Errorf("failed to create drive service: %w", err)
	}

	return &Client{
		service: service,
	}, nil
}

// ListFiles lists files and folders
func (c *Client) ListFiles(query string, pageSize int64, parentID string) ([]*drive.File, error) {
	// Log the request for debugging
	log.Printf("[Drive] ListFiles called with query=%q, pageSize=%d, parentID=%q", query, pageSize, parentID)

	// Create a new context for the API call
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	call := c.service.Files.List().
		Fields("files(id, name, mimeType, size, modifiedTime, parents, webViewLink, iconLink, thumbnailLink)")

	// Build the query
	finalQuery := query
	if parentID != "" {
		parentQuery := fmt.Sprintf("'%s' in parents", parentID)
		if finalQuery != "" {
			finalQuery = finalQuery + " and " + parentQuery
		} else {
			finalQuery = parentQuery
		}
	} else if finalQuery == "" {
		// If no parent and no query, limit to root folder only to avoid fetching everything
		finalQuery = "'root' in parents and trashed = false"
	}

	if finalQuery != "" {
		log.Printf("[Drive] Final query: %q", finalQuery)
		call = call.Q(finalQuery)
	}

	if pageSize > 0 {
		call = call.PageSize(pageSize)
	} else {
		// Set default page size if not specified
		pageSize = 10 // Reduced default to 10 for faster response
		call = call.PageSize(pageSize)
	}

	log.Printf("[Drive] Making API call with pageSize=%d", pageSize)

	// Don't use Pages() method as it fetches ALL pages which can cause timeouts
	// Instead, fetch only one page based on the specified pageSize
	fileList, err := call.Context(ctx).Do()
	if err != nil {
		log.Printf("[Drive] API call failed: %v", err)
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	log.Printf("[Drive] API call successful, got %d files", len(fileList.Files))
	return fileList.Files, nil
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
	defer func() { _ = resp.Body.Close() }()

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
	defer func() { _ = file.Close() }()

	// Detect MIME type (simplified - in production use proper detection)
	mimeType := "application/octet-stream"

	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	return c.UploadFile(info.Name(), mimeType, file, parentID)
}

// convertMarkdownToHTML converts markdown content to HTML using goldmark with all extensions
func convertMarkdownToHTML(markdown string) (string, error) {
	// Create goldmark instance with all extensions
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,            // GitHub Flavored Markdown
			extension.Footnote,       // Footnotes
			extension.DefinitionList, // Definition lists
			extension.Typographer,    // Typography replacements
			extension.Linkify,        // Auto-linkify URLs
			extension.Strikethrough,  // Strikethrough text
			extension.TaskList,       // Task lists
			extension.Table,          // Tables
			emoji.Emoji,              // Emoji support
			highlighting.NewHighlighting(
				highlighting.WithStyle("github"),
				highlighting.WithFormatOptions(
					chromahtml.WithClasses(false),
					chromahtml.PreventSurroundingPre(false),
					chromahtml.WithLineNumbers(false),
					chromahtml.LineNumbersInTable(false),
				),
			),
			meta.Meta, // YAML frontmatter
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(), // Auto-generate heading IDs
			parser.WithAttribute(),     // Allow attributes
			parser.WithBlockParsers(),
			parser.WithInlineParsers(),
			parser.WithParagraphTransformers(),
		),
		goldmark.WithRendererOptions(
			goldmarkhtml.WithHardWraps(), // Preserve line breaks
			goldmarkhtml.WithXHTML(),     // Generate XHTML
			goldmarkhtml.WithUnsafe(),    // Allow raw HTML
		),
	)

	var buf bytes.Buffer
	// Wrap in HTML document structure for Google Docs
	buf.WriteString(`<!DOCTYPE html>
<html>
<head>
<meta charset="UTF-8">
<style>
body { font-family: Arial, sans-serif; line-height: 1.6; margin: 20px; }
pre { 
  background-color: rgb(243, 243, 243);
  padding: 16px; 
  border-radius: 8px; 
  overflow-x: auto; 
  border: 1px solid rgb(220, 220, 220);
  display: block;
  font-family: 'Courier New', Consolas, Monaco, monospace;
  font-size: 14px;
  line-height: 1.4;
  white-space: pre;
}
pre code {
  background-color: transparent;
  padding: 0;
  border-radius: 0;
  color: #24292e;
  font-size: inherit;
}
code { 
  background-color: rgb(243, 243, 243);
  padding: 2px 6px; 
  border-radius: 3px; 
  font-family: 'Courier New', Consolas, Monaco, monospace; 
  font-size: 0.9em;
  color: #24292e;
  border: 1px solid rgb(220, 220, 220);
}
blockquote { border-left: 4px solid #ddd; margin: 0; padding-left: 16px; color: #666; }
table { border-collapse: collapse; width: 100%; margin: 15px 0; }
th, td { border: 1px solid #ddd; padding: 8px; text-align: left; }
th { background-color: #f2f2f2; font-weight: bold; }
tr:nth-child(even) { background-color: #f9f9f9; }
h1, h2, h3, h4, h5, h6 { margin-top: 24px; margin-bottom: 16px; font-weight: 600; }
ul, ol { margin: 10px 0; padding-left: 30px; }
li { margin: 5px 0; }
a { color: #0066cc; text-decoration: none; }
a:hover { text-decoration: underline; }
.task-list-item { list-style-type: none; }
.task-list-item input { margin-right: 8px; }
.highlight { background-color: transparent; }
.highlight pre { background-color: rgb(243, 243, 243); }
</style>
</head>
<body>
`)

	// Convert markdown to HTML
	if err := md.Convert([]byte(markdown), &buf); err != nil {
		return "", fmt.Errorf("failed to convert markdown to HTML: %w", err)
	}

	buf.WriteString(`
</body>
</html>`)

	return buf.String(), nil
}

// UploadMarkdownAsDoc uploads markdown content as a Google Doc
func (c *Client) UploadMarkdownAsDoc(ctx context.Context, name, markdown, parentID string) (*drive.File, error) {
	// Convert markdown to HTML
	htmlContent, err := convertMarkdownToHTML(markdown)
	if err != nil {
		return nil, fmt.Errorf("failed to convert markdown: %w", err)
	}

	// Create file metadata
	file := &drive.File{
		Name:     name,
		MimeType: "application/vnd.google-apps.document",
	}

	if parentID != "" {
		file.Parents = []string{parentID}
	}

	// Upload as Google Doc
	reader := strings.NewReader(htmlContent)
	driveFile, err := c.service.Files.Create(file).
		Media(reader).
		Fields("id, name, mimeType, size, modifiedTime, parents, webViewLink, iconLink, thumbnailLink, createdTime").
		Context(ctx).
		Do()

	if err != nil {
		return nil, fmt.Errorf("failed to upload file: %w", err)
	}

	return driveFile, nil
}

// ReplaceDocWithMarkdown replaces a Google Doc's content with converted markdown
func (c *Client) ReplaceDocWithMarkdown(ctx context.Context, fileID, markdown string) (*drive.File, error) {
	// First, get the file metadata to ensure it's a Google Doc
	file, err := c.service.Files.Get(fileID).
		Fields("id, name, mimeType").
		Context(ctx).
		Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get file metadata: %w", err)
	}

	// Check if it's a Google Doc
	if file.MimeType != "application/vnd.google-apps.document" {
		return nil, fmt.Errorf("file is not a Google Doc (mimeType: %s)", file.MimeType)
	}

	// Convert markdown to HTML
	htmlContent, err := convertMarkdownToHTML(markdown)
	if err != nil {
		return nil, fmt.Errorf("failed to convert markdown: %w", err)
	}

	// Update the file content
	reader := strings.NewReader(htmlContent)
	updatedFile, err := c.service.Files.Update(fileID, &drive.File{}).
		Media(reader).
		Fields("id, name, mimeType, size, modifiedTime, parents, webViewLink, iconLink, thumbnailLink").
		Context(ctx).
		Do()

	if err != nil {
		return nil, fmt.Errorf("failed to update file: %w", err)
	}

	return updatedFile, nil
}
