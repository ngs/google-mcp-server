package docs

import (
	"context"
	"fmt"
	"testing"

	"google.golang.org/api/docs/v1"
)

// MockClient is a mock implementation of the Client for testing
type MockClient struct {
	GetDocumentFunc    func(documentID string) (*docs.Document, error)
	CreateDocumentFunc func(title string) (*docs.Document, error)
	BatchUpdateFunc    func(documentID string, requests []*docs.Request) (*docs.BatchUpdateDocumentResponse, error)
}

func (m *MockClient) GetDocument(documentID string) (*docs.Document, error) {
	if m.GetDocumentFunc != nil {
		return m.GetDocumentFunc(documentID)
	}
	return &docs.Document{
		DocumentId: documentID,
		Title:      "Test Document",
		Body: &docs.Body{
			Content: []*docs.StructuralElement{
				{
					EndIndex: 2,
				},
			},
		},
	}, nil
}

func (m *MockClient) CreateDocument(title string) (*docs.Document, error) {
	if m.CreateDocumentFunc != nil {
		return m.CreateDocumentFunc(title)
	}
	return &docs.Document{
		DocumentId: "test-doc-id",
		Title:      title,
	}, nil
}

func (m *MockClient) BatchUpdate(documentID string, requests []*docs.Request) (*docs.BatchUpdateDocumentResponse, error) {
	if m.BatchUpdateFunc != nil {
		return m.BatchUpdateFunc(documentID, requests)
	}
	return &docs.BatchUpdateDocumentResponse{
		DocumentId: documentID,
		Replies:    make([]*docs.Response, len(requests)),
	}, nil
}

func TestPayloadBuilder(t *testing.T) {
	tests := []struct {
		name        string
		buildFunc   func(*PayloadBuilder)
		expectCount int
		validate    func([]*docs.Request) error
	}{
		{
			name: "Insert text",
			buildFunc: func(pb *PayloadBuilder) {
				pb.AddInsertText("Hello World", 1)
			},
			expectCount: 1,
			validate: func(requests []*docs.Request) error {
				if requests[0].InsertText == nil {
					return fmt.Errorf("expected InsertText request")
				}
				if requests[0].InsertText.Text != "Hello World" {
					return fmt.Errorf("expected text 'Hello World', got '%s'", requests[0].InsertText.Text)
				}
				return nil
			},
		},
		{
			name: "Delete range",
			buildFunc: func(pb *PayloadBuilder) {
				pb.AddDeleteRange(1, 10)
			},
			expectCount: 1,
			validate: func(requests []*docs.Request) error {
				if requests[0].DeleteContentRange == nil {
					return fmt.Errorf("expected DeleteContentRange request")
				}
				if requests[0].DeleteContentRange.Range.StartIndex != 1 {
					return fmt.Errorf("expected start index 1")
				}
				if requests[0].DeleteContentRange.Range.EndIndex != 10 {
					return fmt.Errorf("expected end index 10")
				}
				return nil
			},
		},
		{
			name: "Update text style",
			buildFunc: func(pb *PayloadBuilder) {
				pb.AddUpdateTextStyle(1, 10, &docs.TextStyle{Bold: true}, "bold")
			},
			expectCount: 1,
			validate: func(requests []*docs.Request) error {
				if requests[0].UpdateTextStyle == nil {
					return fmt.Errorf("expected UpdateTextStyle request")
				}
				if !requests[0].UpdateTextStyle.TextStyle.Bold {
					return fmt.Errorf("expected bold to be true")
				}
				if requests[0].UpdateTextStyle.Fields != "bold" {
					return fmt.Errorf("expected fields to be 'bold'")
				}
				return nil
			},
		},
		{
			name: "Update paragraph style",
			buildFunc: func(pb *PayloadBuilder) {
				pb.AddUpdateParagraphStyle(1, 10, &docs.ParagraphStyle{
					NamedStyleType: "HEADING_1",
				}, "namedStyleType")
			},
			expectCount: 1,
			validate: func(requests []*docs.Request) error {
				if requests[0].UpdateParagraphStyle == nil {
					return fmt.Errorf("expected UpdateParagraphStyle request")
				}
				if requests[0].UpdateParagraphStyle.ParagraphStyle.NamedStyleType != "HEADING_1" {
					return fmt.Errorf("expected HEADING_1 style")
				}
				return nil
			},
		},
		{
			name: "Multiple operations",
			buildFunc: func(pb *PayloadBuilder) {
				pb.AddInsertText("Title\n", 1).
					AddUpdateParagraphStyle(1, 7, &docs.ParagraphStyle{
						NamedStyleType: "TITLE",
					}, "namedStyleType").
					AddInsertText("Body text", 7).
					AddUpdateTextStyle(7, 16, &docs.TextStyle{Bold: true}, "bold")
			},
			expectCount: 4,
			validate: func(requests []*docs.Request) error {
				if len(requests) != 4 {
					return fmt.Errorf("expected 4 requests, got %d", len(requests))
				}
				return nil
			},
		},
		{
			name: "Insert image",
			buildFunc: func(pb *PayloadBuilder) {
				pb.AddInsertInlineImage("https://example.com/image.png", 1, 200, 150)
			},
			expectCount: 1,
			validate: func(requests []*docs.Request) error {
				if requests[0].InsertInlineImage == nil {
					return fmt.Errorf("expected InsertInlineImage request")
				}
				if requests[0].InsertInlineImage.Uri != "https://example.com/image.png" {
					return fmt.Errorf("unexpected image URI")
				}
				return nil
			},
		},
		{
			name: "Insert page break",
			buildFunc: func(pb *PayloadBuilder) {
				pb.AddInsertPageBreak(1)
			},
			expectCount: 1,
			validate: func(requests []*docs.Request) error {
				if requests[0].InsertPageBreak == nil {
					return fmt.Errorf("expected InsertPageBreak request")
				}
				return nil
			},
		},
		{
			name: "Insert table",
			buildFunc: func(pb *PayloadBuilder) {
				pb.AddInsertTable(3, 4, 1)
			},
			expectCount: 1,
			validate: func(requests []*docs.Request) error {
				if requests[0].InsertTable == nil {
					return fmt.Errorf("expected InsertTable request")
				}
				if requests[0].InsertTable.Rows != 3 {
					return fmt.Errorf("expected 3 rows")
				}
				if requests[0].InsertTable.Columns != 4 {
					return fmt.Errorf("expected 4 columns")
				}
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pb := NewPayloadBuilder()
			tt.buildFunc(pb)
			requests := pb.Build()

			if len(requests) != tt.expectCount {
				t.Errorf("Expected %d requests, got %d", tt.expectCount, len(requests))
			}

			if tt.validate != nil {
				if err := tt.validate(requests); err != nil {
					t.Errorf("Validation failed: %v", err)
				}
			}
		})
	}
}

func TestDocumentUpdater_UpdateWithMarkdown(t *testing.T) {
	tests := []struct {
		name           string
		documentID     string
		markdown       string
		mode           string
		mockSetup      func(*MockClient)
		validateResult func(*docs.BatchUpdateDocumentResponse, error) error
	}{
		{
			name:       "Simple markdown update",
			documentID: "test-doc",
			markdown:   "# Hello World\n\nThis is a test.",
			mode:       "replace",
			mockSetup: func(mc *MockClient) {
				mc.BatchUpdateFunc = func(docID string, requests []*docs.Request) (*docs.BatchUpdateDocumentResponse, error) {
					// Verify we have requests
					if len(requests) == 0 {
						t.Error("Expected requests but got none")
					}
					return &docs.BatchUpdateDocumentResponse{
						DocumentId: docID,
						Replies:    make([]*docs.Response, len(requests)),
					}, nil
				}
			},
			validateResult: func(response *docs.BatchUpdateDocumentResponse, err error) error {
				if err != nil {
					return fmt.Errorf("unexpected error: %v", err)
				}
				if response.DocumentId != "test-doc" {
					return fmt.Errorf("expected document ID 'test-doc', got '%s'", response.DocumentId)
				}
				return nil
			},
		},
		{
			name:       "Append mode",
			documentID: "test-doc",
			markdown:   "## New Section",
			mode:       "append",
			mockSetup: func(mc *MockClient) {
				mc.GetDocumentFunc = func(docID string) (*docs.Document, error) {
					return &docs.Document{
						DocumentId: docID,
						Body: &docs.Body{
							Content: []*docs.StructuralElement{
								{EndIndex: 100},
							},
						},
					}, nil
				}
			},
			validateResult: func(response *docs.BatchUpdateDocumentResponse, err error) error {
				if err != nil {
					return fmt.Errorf("unexpected error: %v", err)
				}
				return nil
			},
		},
		{
			name:       "Complex markdown",
			documentID: "test-doc",
			markdown: `# Main Title

This has **bold** and *italic* text.

## Lists

- Item 1
- Item 2

1. First
2. Second

> Quote block

` + "```" + `
code block
` + "```",
			mode: "replace",
			mockSetup: func(mc *MockClient) {
				mc.BatchUpdateFunc = func(docID string, requests []*docs.Request) (*docs.BatchUpdateDocumentResponse, error) {
					// Should have multiple requests for formatting
					if len(requests) < 5 {
						t.Errorf("Expected at least 5 requests for complex markdown, got %d", len(requests))
					}
					return &docs.BatchUpdateDocumentResponse{
						DocumentId: docID,
						Replies:    make([]*docs.Response, len(requests)),
					}, nil
				}
			},
			validateResult: func(response *docs.BatchUpdateDocumentResponse, err error) error {
				if err != nil {
					return fmt.Errorf("unexpected error: %v", err)
				}
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockClient{}
			if tt.mockSetup != nil {
				tt.mockSetup(mockClient)
			}

			// Create updater with mock client
			updater := &DocumentUpdater{
				client: mockClient,
			}

			response, err := updater.UpdateWithMarkdown(context.Background(), tt.documentID, tt.markdown, tt.mode)

			if tt.validateResult != nil {
				if err := tt.validateResult(response, err); err != nil {
					t.Errorf("Validation failed: %v", err)
				}
			}
		})
	}
}

func TestDocumentUpdater_UpdateWithHTML(t *testing.T) {
	tests := []struct {
		name           string
		documentID     string
		html           string
		mode           string
		mockSetup      func(*MockClient)
		validateResult func(*docs.BatchUpdateDocumentResponse, error) error
	}{
		{
			name:       "Simple HTML update",
			documentID: "test-doc",
			html:       "<h1>Hello World</h1><p>This is a test.</p>",
			mode:       "replace",
			mockSetup: func(mc *MockClient) {
				mc.BatchUpdateFunc = func(docID string, requests []*docs.Request) (*docs.BatchUpdateDocumentResponse, error) {
					if len(requests) == 0 {
						t.Error("Expected requests but got none")
					}
					return &docs.BatchUpdateDocumentResponse{
						DocumentId: docID,
						Replies:    make([]*docs.Response, len(requests)),
					}, nil
				}
			},
			validateResult: func(response *docs.BatchUpdateDocumentResponse, err error) error {
				if err != nil {
					return fmt.Errorf("unexpected error: %v", err)
				}
				return nil
			},
		},
		{
			name:       "HTML with formatting",
			documentID: "test-doc",
			html:       "<p>This has <strong>bold</strong> and <em>italic</em> text.</p>",
			mode:       "replace",
			validateResult: func(response *docs.BatchUpdateDocumentResponse, err error) error {
				if err != nil {
					return fmt.Errorf("unexpected error: %v", err)
				}
				return nil
			},
		},
		{
			name:       "HTML with lists",
			documentID: "test-doc",
			html:       "<ul><li>Item 1</li><li>Item 2</li></ul><ol><li>First</li><li>Second</li></ol>",
			mode:       "append",
			mockSetup: func(mc *MockClient) {
				mc.GetDocumentFunc = func(docID string) (*docs.Document, error) {
					return &docs.Document{
						DocumentId: docID,
						Body: &docs.Body{
							Content: []*docs.StructuralElement{
								{EndIndex: 50},
							},
						},
					}, nil
				}
			},
			validateResult: func(response *docs.BatchUpdateDocumentResponse, err error) error {
				if err != nil {
					return fmt.Errorf("unexpected error: %v", err)
				}
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockClient{}
			if tt.mockSetup != nil {
				tt.mockSetup(mockClient)
			}

			updater := &DocumentUpdater{
				client: mockClient,
			}

			response, err := updater.UpdateWithHTML(context.Background(), tt.documentID, tt.html, tt.mode)

			if tt.validateResult != nil {
				if err := tt.validateResult(response, err); err != nil {
					t.Errorf("Validation failed: %v", err)
				}
			}
		})
	}
}

func TestDocumentUpdater_CreateAndPopulateDocument(t *testing.T) {
	mockClient := &MockClient{
		CreateDocumentFunc: func(title string) (*docs.Document, error) {
			return &docs.Document{
				DocumentId: "new-doc-id",
				Title:      title,
			}, nil
		},
		BatchUpdateFunc: func(docID string, requests []*docs.Request) (*docs.BatchUpdateDocumentResponse, error) {
			if docID != "new-doc-id" {
				t.Errorf("Expected document ID 'new-doc-id', got '%s'", docID)
			}
			return &docs.BatchUpdateDocumentResponse{
				DocumentId: docID,
				Replies:    make([]*docs.Response, len(requests)),
			}, nil
		},
		GetDocumentFunc: func(docID string) (*docs.Document, error) {
			return &docs.Document{
				DocumentId: docID,
				Title:      "Test Document",
				Body: &docs.Body{
					Content: []*docs.StructuralElement{
						{
							Paragraph: &docs.Paragraph{
								Elements: []*docs.ParagraphElement{
									{
										TextRun: &docs.TextRun{
											Content: "# Test Title\n\nContent here",
										},
									},
								},
							},
						},
					},
				},
			}, nil
		},
	}

	updater := &DocumentUpdater{
		client: mockClient,
	}

	doc, err := updater.CreateAndPopulateDocument(
		context.Background(),
		"Test Document",
		"# Test Title\n\nContent here",
	)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if doc.DocumentId != "new-doc-id" {
		t.Errorf("Expected document ID 'new-doc-id', got '%s'", doc.DocumentId)
	}

	if doc.Title != "Test Document" {
		t.Errorf("Expected title 'Test Document', got '%s'", doc.Title)
	}
}

func TestBuildBatchUpdateRequest(t *testing.T) {
	pb := NewPayloadBuilder()
	pb.AddInsertText("Hello", 1).
		AddUpdateTextStyle(1, 6, &docs.TextStyle{Bold: true}, "bold")

	batchRequest := pb.BuildBatchUpdateRequest()

	if batchRequest == nil {
		t.Fatal("Expected BatchUpdateDocumentRequest but got nil")
	}

	if len(batchRequest.Requests) != 2 {
		t.Errorf("Expected 2 requests, got %d", len(batchRequest.Requests))
	}

	if batchRequest.Requests[0].InsertText == nil {
		t.Error("Expected first request to be InsertText")
	}

	if batchRequest.Requests[1].UpdateTextStyle == nil {
		t.Error("Expected second request to be UpdateTextStyle")
	}
}
