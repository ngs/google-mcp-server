package drive

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

// recordingTransport records every request and its body, and answers with a
// fixed JSON response.
type recordingTransport struct {
	responseBody string
	requests     []*http.Request
	bodies       []string
}

func (t *recordingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.requests = append(t.requests, req)
	body := ""
	if req.Body != nil {
		data, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		body = string(data)
	}
	t.bodies = append(t.bodies, body)
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(t.responseBody)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}, nil
}

func (t *recordingTransport) lastRequest(tb testing.TB) (*http.Request, string) {
	tb.Helper()
	if len(t.requests) == 0 {
		tb.Fatal("Expected a request to be sent, got none")
	}
	return t.requests[len(t.requests)-1], t.bodies[len(t.bodies)-1]
}

func newCommentsTestHandler(t *testing.T, responseBody string) (*Handler, *recordingTransport) {
	t.Helper()
	rt := &recordingTransport{responseBody: responseBody}
	service, err := drive.NewService(context.Background(), option.WithHTTPClient(&http.Client{Transport: rt}))
	if err != nil {
		t.Fatalf("Failed to create Drive service: %v", err)
	}
	return NewHandler(&Client{service: service}), rt
}

func callTool(t *testing.T, h *Handler, name string, args map[string]interface{}) (map[string]interface{}, error) {
	t.Helper()
	raw, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("Failed to marshal arguments: %v", err)
	}
	result, err := h.HandleToolCall(context.Background(), name, raw)
	if err != nil {
		return nil, err
	}
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected a map result, got %T", result)
	}
	return resultMap, nil
}

const commentListResponse = `{
	"nextPageToken": "next-token",
	"comments": [
		{
			"id": "c1",
			"author": {"displayName": "Alice", "me": true},
			"content": "Please fix this",
			"createdTime": "2026-10-01T00:00:00Z",
			"modifiedTime": "2026-10-01T00:00:00Z",
			"resolved": false,
			"anchor": "kix.abc",
			"quotedFileContent": {"mimeType": "text/html", "value": "the quoted text"},
			"replies": [
				{"id": "r1", "author": {"displayName": "Bob"}, "content": "Done", "action": "resolve"}
			]
		},
		{
			"id": "c2",
			"author": {"displayName": "Bob"},
			"content": "Already handled",
			"resolved": true
		}
	]
}`

func TestCommentsList(t *testing.T) {
	h, rt := newCommentsTestHandler(t, commentListResponse)

	result, err := callTool(t, h, "drive_comments_list", map[string]interface{}{
		"file_id":    "file123",
		"page_size":  50,
		"page_token": "page-token",
	})
	if err != nil {
		t.Fatalf("drive_comments_list error = %v", err)
	}

	req, _ := rt.lastRequest(t)
	if req.Method != http.MethodGet {
		t.Errorf("Expected GET, got %s", req.Method)
	}
	if !strings.HasSuffix(req.URL.Path, "/files/file123/comments") {
		t.Errorf("Unexpected request path: %s", req.URL.Path)
	}
	query := req.URL.Query()
	if fields := query.Get("fields"); !strings.Contains(fields, "comments(") || !strings.Contains(fields, "nextPageToken") {
		t.Errorf("Expected fields to request comments and nextPageToken, got %q", fields)
	}
	if got := query.Get("pageSize"); got != "50" {
		t.Errorf("Expected pageSize=50, got %q", got)
	}
	if got := query.Get("pageToken"); got != "page-token" {
		t.Errorf("Expected pageToken=page-token, got %q", got)
	}

	// Resolved comments are included by default
	if got := result["count"]; got != 2 {
		t.Errorf("Expected 2 comments, got %v", got)
	}
	if got := result["next_page_token"]; got != "next-token" {
		t.Errorf("Expected next_page_token=next-token, got %v", got)
	}

	comments := result["comments"].([]map[string]interface{})
	first := comments[0]
	if first["id"] != "c1" || first["content"] != "Please fix this" || first["resolved"] != false {
		t.Errorf("Unexpected first comment: %v", first)
	}
	if first["quotedFileContent"] != "the quoted text" {
		t.Errorf("Expected quoted text, got %v", first["quotedFileContent"])
	}
	if first["anchor"] != "kix.abc" {
		t.Errorf("Expected anchor kix.abc, got %v", first["anchor"])
	}
	author := first["author"].(map[string]interface{})
	if author["displayName"] != "Alice" || author["me"] != true {
		t.Errorf("Unexpected author: %v", author)
	}
	replies := first["replies"].([]map[string]interface{})
	if len(replies) != 1 || replies[0]["id"] != "r1" || replies[0]["action"] != "resolve" {
		t.Errorf("Unexpected replies: %v", replies)
	}
	if replies := comments[1]["replies"].([]map[string]interface{}); len(replies) != 0 {
		t.Errorf("Expected no replies on the second comment, got %v", replies)
	}
}

func TestCommentsList_ExcludeResolved(t *testing.T) {
	h, _ := newCommentsTestHandler(t, commentListResponse)

	result, err := callTool(t, h, "drive_comments_list", map[string]interface{}{
		"file_id":          "file123",
		"include_resolved": false,
	})
	if err != nil {
		t.Fatalf("drive_comments_list error = %v", err)
	}

	comments := result["comments"].([]map[string]interface{})
	if len(comments) != 1 || comments[0]["id"] != "c1" {
		t.Errorf("Expected only the open comment c1, got %v", comments)
	}
}

func TestCommentsList_PageSizeCapped(t *testing.T) {
	h, rt := newCommentsTestHandler(t, `{"comments": []}`)

	result, err := callTool(t, h, "drive_comments_list", map[string]interface{}{
		"file_id":   "file123",
		"page_size": 500,
	})
	if err != nil {
		t.Fatalf("drive_comments_list error = %v", err)
	}

	req, _ := rt.lastRequest(t)
	if got := req.URL.Query().Get("pageSize"); got != "100" {
		t.Errorf("Expected pageSize capped at 100, got %q", got)
	}
	if _, ok := result["next_page_token"]; ok {
		t.Error("Expected no next_page_token on the last page")
	}
}

func TestCommentsList_RequiresFileID(t *testing.T) {
	h, rt := newCommentsTestHandler(t, `{}`)

	if _, err := callTool(t, h, "drive_comments_list", map[string]interface{}{}); err == nil {
		t.Error("Expected an error without file_id")
	}
	if len(rt.requests) != 0 {
		t.Error("Expected no API request without file_id")
	}
}

func TestCommentCreate(t *testing.T) {
	h, rt := newCommentsTestHandler(t, `{"id": "c9", "content": "Looks good", "quotedFileContent": {"value": "Intro"}}`)

	result, err := callTool(t, h, "drive_comment_create", map[string]interface{}{
		"file_id":     "file123",
		"content":     "Looks good",
		"quoted_text": "Intro",
	})
	if err != nil {
		t.Fatalf("drive_comment_create error = %v", err)
	}

	req, body := rt.lastRequest(t)
	if req.Method != http.MethodPost || !strings.HasSuffix(req.URL.Path, "/files/file123/comments") {
		t.Errorf("Unexpected request: %s %s", req.Method, req.URL.Path)
	}
	if req.URL.Query().Get("fields") == "" {
		t.Error("Expected the fields parameter to be set")
	}

	var sent drive.Comment
	if err := json.Unmarshal([]byte(body), &sent); err != nil {
		t.Fatalf("Failed to decode request body %q: %v", body, err)
	}
	if sent.Content != "Looks good" {
		t.Errorf("Expected content to be sent, got %q", sent.Content)
	}
	if sent.QuotedFileContent == nil || sent.QuotedFileContent.Value != "Intro" || sent.QuotedFileContent.MimeType != "text/plain" {
		t.Errorf("Expected quoted text to be sent as text/plain, got %+v", sent.QuotedFileContent)
	}
	if sent.Anchor != "" {
		t.Errorf("Expected no anchor, got %q", sent.Anchor)
	}

	if result["id"] != "c9" || result["quotedFileContent"] != "Intro" {
		t.Errorf("Unexpected result: %v", result)
	}
}

func TestCommentCreate_Validation(t *testing.T) {
	h, rt := newCommentsTestHandler(t, `{}`)

	for _, args := range []map[string]interface{}{
		{"content": "missing file"},
		{"file_id": "file123"},
	} {
		if _, err := callTool(t, h, "drive_comment_create", args); err == nil {
			t.Errorf("Expected an error for arguments %v", args)
		}
	}
	if len(rt.requests) != 0 {
		t.Error("Expected no API request for invalid arguments")
	}
}

func TestCommentReplyCreate(t *testing.T) {
	h, rt := newCommentsTestHandler(t, `{"id": "r2", "content": "Fixed", "action": "resolve", "author": {"displayName": "Alice", "me": true}}`)

	result, err := callTool(t, h, "drive_comment_reply_create", map[string]interface{}{
		"file_id":    "file123",
		"comment_id": "c1",
		"content":    "Fixed",
		"action":     "resolve",
	})
	if err != nil {
		t.Fatalf("drive_comment_reply_create error = %v", err)
	}

	req, body := rt.lastRequest(t)
	if req.Method != http.MethodPost || !strings.HasSuffix(req.URL.Path, "/files/file123/comments/c1/replies") {
		t.Errorf("Unexpected request: %s %s", req.Method, req.URL.Path)
	}

	var sent drive.Reply
	if err := json.Unmarshal([]byte(body), &sent); err != nil {
		t.Fatalf("Failed to decode request body %q: %v", body, err)
	}
	if sent.Content != "Fixed" || sent.Action != "resolve" {
		t.Errorf("Unexpected reply sent: %+v", sent)
	}

	if result["id"] != "r2" || result["comment_id"] != "c1" || result["action"] != "resolve" {
		t.Errorf("Unexpected result: %v", result)
	}
}

func TestCommentReplyCreate_ActionWithoutContent(t *testing.T) {
	h, rt := newCommentsTestHandler(t, `{"id": "r3", "action": "reopen"}`)

	if _, err := callTool(t, h, "drive_comment_reply_create", map[string]interface{}{
		"file_id":    "file123",
		"comment_id": "c1",
		"action":     "reopen",
	}); err != nil {
		t.Fatalf("drive_comment_reply_create error = %v", err)
	}

	_, body := rt.lastRequest(t)
	var sent drive.Reply
	if err := json.Unmarshal([]byte(body), &sent); err != nil {
		t.Fatalf("Failed to decode request body %q: %v", body, err)
	}
	if sent.Action != "reopen" || sent.Content != "" {
		t.Errorf("Unexpected reply sent: %+v", sent)
	}
}

func TestCommentReplyCreate_Validation(t *testing.T) {
	h, rt := newCommentsTestHandler(t, `{}`)

	for _, args := range []map[string]interface{}{
		{"comment_id": "c1", "content": "missing file"},
		{"file_id": "file123", "content": "missing comment"},
		{"file_id": "file123", "comment_id": "c1"},
		{"file_id": "file123", "comment_id": "c1", "content": "bad action", "action": "delete"},
	} {
		if _, err := callTool(t, h, "drive_comment_reply_create", args); err == nil {
			t.Errorf("Expected an error for arguments %v", args)
		}
	}
	if len(rt.requests) != 0 {
		t.Error("Expected no API request for invalid arguments")
	}
}

func TestCommentResolve(t *testing.T) {
	h, rt := newCommentsTestHandler(t, `{"id": "r4", "action": "resolve"}`)

	result, err := callTool(t, h, "drive_comment_resolve", map[string]interface{}{
		"file_id":    "file123",
		"comment_id": "c1",
	})
	if err != nil {
		t.Fatalf("drive_comment_resolve error = %v", err)
	}

	req, body := rt.lastRequest(t)
	if !strings.HasSuffix(req.URL.Path, "/files/file123/comments/c1/replies") {
		t.Errorf("Unexpected request path: %s", req.URL.Path)
	}
	var sent drive.Reply
	if err := json.Unmarshal([]byte(body), &sent); err != nil {
		t.Fatalf("Failed to decode request body %q: %v", body, err)
	}
	if sent.Action != "resolve" {
		t.Errorf("Expected a resolve action, got %+v", sent)
	}
	if result["action"] != "resolve" {
		t.Errorf("Unexpected result: %v", result)
	}
}

func TestCommentToolsAcceptAccount(t *testing.T) {
	byName := toolNames(NewMultiAccountHandler(newTestAccountManager(t), nil).GetTools())

	for _, name := range []string{
		"drive_comments_list",
		"drive_comment_create",
		"drive_comment_reply_create",
		"drive_comment_resolve",
	} {
		tool, ok := byName[name]
		if !ok {
			t.Errorf("Expected tool %s to be listed", name)
			continue
		}
		if _, ok := tool.InputSchema.Properties["account"]; !ok {
			t.Errorf("Expected %s to accept an account parameter", name)
		}
	}
}

func TestCommentsList_InvalidPageSize(t *testing.T) {
	h, rt := newCommentsTestHandler(t, `{"comments": []}`)

	for _, pageSize := range []interface{}{-1, 0.5, 20.5} {
		if _, err := callTool(t, h, "drive_comments_list", map[string]interface{}{
			"file_id":   "file123",
			"page_size": pageSize,
		}); err == nil {
			t.Errorf("Expected an error for page_size %v", pageSize)
		}
	}
	if len(rt.requests) != 0 {
		t.Error("Expected no API request for an invalid page_size")
	}
}

func TestCommentsList_HugePageSizeCapped(t *testing.T) {
	h, rt := newCommentsTestHandler(t, `{"comments": []}`)

	if _, err := callTool(t, h, "drive_comments_list", map[string]interface{}{
		"file_id":   "file123",
		"page_size": 1e30,
	}); err != nil {
		t.Fatalf("drive_comments_list error = %v", err)
	}

	req, _ := rt.lastRequest(t)
	if got := req.URL.Query().Get("pageSize"); got != "100" {
		t.Errorf("Expected pageSize capped at 100, got %q", got)
	}
}
