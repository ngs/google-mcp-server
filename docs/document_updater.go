package docs

import (
	"context"
	"fmt"

	"google.golang.org/api/docs/v1"
)

// DocumentUpdater handles updating Google Docs with formatted content
type DocumentUpdater struct {
	client    ClientInterface
	converter *MarkdownConverter
}

// ClientInterface defines the interface for the Docs client
type ClientInterface interface {
	GetDocument(documentID string) (*docs.Document, error)
	CreateDocument(title string) (*docs.Document, error)
	BatchUpdate(documentID string, requests []*docs.Request) (*docs.BatchUpdateDocumentResponse, error)
}

// NewDocumentUpdater creates a new document updater
func NewDocumentUpdater(client ClientInterface) *DocumentUpdater {
	return &DocumentUpdater{
		client: client,
	}
}

// UpdateWithMarkdown updates a Google Doc with markdown content
func (du *DocumentUpdater) UpdateWithMarkdown(ctx context.Context, documentID string, markdown string, mode string) (*docs.BatchUpdateDocumentResponse, error) {
	// Create a converter for this document
	converter := NewMarkdownConverter(documentID, du.client)

	// Convert markdown to Google Docs requests
	requests, err := converter.ConvertMarkdownToDocsRequests(markdown, mode)
	if err != nil {
		return nil, fmt.Errorf("failed to convert markdown: %w", err)
	}

	// Send the batch update to Google Docs API
	response, err := du.client.BatchUpdate(documentID, requests)
	if err != nil {
		return nil, fmt.Errorf("failed to update document: %w", err)
	}

	return response, nil
}

// UpdateWithHTML updates a Google Doc with HTML content
func (du *DocumentUpdater) UpdateWithHTML(ctx context.Context, documentID string, htmlContent string, mode string) (*docs.BatchUpdateDocumentResponse, error) {
	// Create a converter for this document
	converter := NewMarkdownConverter(documentID, du.client)

	// Parse HTML to node tree
	rootNode, err := converter.ParseHTML(htmlContent)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	// Determine start index based on mode
	var startIndex int64 = 1
	var requests []*docs.Request

	if mode == "replace" {
		// Get document to find content range
		doc, err := du.client.GetDocument(documentID)
		if err != nil {
			return nil, fmt.Errorf("failed to get document: %w", err)
		}

		// Find the end index
		endIndex := int64(1)
		if doc.Body != nil && doc.Body.Content != nil && len(doc.Body.Content) > 0 {
			lastElement := doc.Body.Content[len(doc.Body.Content)-1]
			if lastElement.EndIndex > 0 {
				endIndex = lastElement.EndIndex - 1
			}
		}

		// Delete existing content if any
		if endIndex > 1 {
			requests = append(requests, &docs.Request{
				DeleteContentRange: &docs.DeleteContentRangeRequest{
					Range: &docs.Range{
						StartIndex: 1,
						EndIndex:   endIndex,
					},
				},
			})
		}
	} else if mode == "append" {
		// Find where to append
		doc, err := du.client.GetDocument(documentID)
		if err != nil {
			return nil, fmt.Errorf("failed to get document: %w", err)
		}

		if doc.Body != nil && doc.Body.Content != nil && len(doc.Body.Content) > 0 {
			lastElement := doc.Body.Content[len(doc.Body.Content)-1]
			if lastElement.EndIndex > 0 {
				startIndex = lastElement.EndIndex - 1
			}
		}
	}

	// Convert HTML nodes to Docs requests
	convertRequests, err := converter.ConvertHTMLToDocsRequests(rootNode, startIndex)
	if err != nil {
		return nil, fmt.Errorf("failed to convert HTML to requests: %w", err)
	}

	requests = append(requests, convertRequests...)

	// Send the batch update to Google Docs API
	response, err := du.client.BatchUpdate(documentID, requests)
	if err != nil {
		return nil, fmt.Errorf("failed to update document: %w", err)
	}

	return response, nil
}

// CreateAndPopulateDocument creates a new document and populates it with markdown content
func (du *DocumentUpdater) CreateAndPopulateDocument(ctx context.Context, title string, markdown string) (*docs.Document, error) {
	// Create a new document
	doc, err := du.client.CreateDocument(title)
	if err != nil {
		return nil, fmt.Errorf("failed to create document: %w", err)
	}

	// Update the document with markdown content
	_, err = du.UpdateWithMarkdown(ctx, doc.DocumentId, markdown, "replace")
	if err != nil {
		return nil, fmt.Errorf("failed to populate document: %w", err)
	}

	// Return the updated document
	return du.client.GetDocument(doc.DocumentId)
}

// PayloadBuilder helps build Google Docs API payloads
type PayloadBuilder struct {
	requests []*docs.Request
}

// NewPayloadBuilder creates a new payload builder
func NewPayloadBuilder() *PayloadBuilder {
	return &PayloadBuilder{
		requests: []*docs.Request{},
	}
}

// AddInsertText adds an insert text request
func (pb *PayloadBuilder) AddInsertText(text string, index int64) *PayloadBuilder {
	pb.requests = append(pb.requests, &docs.Request{
		InsertText: &docs.InsertTextRequest{
			Text: text,
			Location: &docs.Location{
				Index: index,
			},
		},
	})
	return pb
}

// AddDeleteRange adds a delete content range request
func (pb *PayloadBuilder) AddDeleteRange(startIndex, endIndex int64) *PayloadBuilder {
	pb.requests = append(pb.requests, &docs.Request{
		DeleteContentRange: &docs.DeleteContentRangeRequest{
			Range: &docs.Range{
				StartIndex: startIndex,
				EndIndex:   endIndex,
			},
		},
	})
	return pb
}

// AddUpdateTextStyle adds an update text style request
func (pb *PayloadBuilder) AddUpdateTextStyle(startIndex, endIndex int64, style *docs.TextStyle, fields string) *PayloadBuilder {
	pb.requests = append(pb.requests, &docs.Request{
		UpdateTextStyle: &docs.UpdateTextStyleRequest{
			Range: &docs.Range{
				StartIndex: startIndex,
				EndIndex:   endIndex,
			},
			TextStyle: style,
			Fields:    fields,
		},
	})
	return pb
}

// AddUpdateParagraphStyle adds an update paragraph style request
func (pb *PayloadBuilder) AddUpdateParagraphStyle(startIndex, endIndex int64, style *docs.ParagraphStyle, fields string) *PayloadBuilder {
	pb.requests = append(pb.requests, &docs.Request{
		UpdateParagraphStyle: &docs.UpdateParagraphStyleRequest{
			Range: &docs.Range{
				StartIndex: startIndex,
				EndIndex:   endIndex,
			},
			ParagraphStyle: style,
			Fields:         fields,
		},
	})
	return pb
}

// AddCreateNamedRange adds a create named range request
func (pb *PayloadBuilder) AddCreateNamedRange(name string, startIndex, endIndex int64) *PayloadBuilder {
	pb.requests = append(pb.requests, &docs.Request{
		CreateNamedRange: &docs.CreateNamedRangeRequest{
			Name: name,
			Range: &docs.Range{
				StartIndex: startIndex,
				EndIndex:   endIndex,
			},
		},
	})
	return pb
}

// AddInsertInlineImage adds an insert inline image request
func (pb *PayloadBuilder) AddInsertInlineImage(uri string, index int64, width, height float64) *PayloadBuilder {
	pb.requests = append(pb.requests, &docs.Request{
		InsertInlineImage: &docs.InsertInlineImageRequest{
			Uri: uri,
			Location: &docs.Location{
				Index: index,
			},
			ObjectSize: &docs.Size{
				Width: &docs.Dimension{
					Magnitude: width,
					Unit:      "PT",
				},
				Height: &docs.Dimension{
					Magnitude: height,
					Unit:      "PT",
				},
			},
		},
	})
	return pb
}

// AddInsertPageBreak adds an insert page break request
func (pb *PayloadBuilder) AddInsertPageBreak(index int64) *PayloadBuilder {
	pb.requests = append(pb.requests, &docs.Request{
		InsertPageBreak: &docs.InsertPageBreakRequest{
			Location: &docs.Location{
				Index: index,
			},
		},
	})
	return pb
}

// AddInsertTable adds an insert table request
func (pb *PayloadBuilder) AddInsertTable(rows, columns int64, index int64) *PayloadBuilder {
	pb.requests = append(pb.requests, &docs.Request{
		InsertTable: &docs.InsertTableRequest{
			Rows:    rows,
			Columns: columns,
			Location: &docs.Location{
				Index: index,
			},
		},
	})
	return pb
}

// Build returns the built requests
func (pb *PayloadBuilder) Build() []*docs.Request {
	return pb.requests
}

// BuildBatchUpdateRequest creates a BatchUpdateDocumentRequest
func (pb *PayloadBuilder) BuildBatchUpdateRequest() *docs.BatchUpdateDocumentRequest {
	return &docs.BatchUpdateDocumentRequest{
		Requests: pb.requests,
	}
}
