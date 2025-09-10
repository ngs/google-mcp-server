package slides

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"google.golang.org/api/option"
	"google.golang.org/api/slides/v1"
)

type Client struct {
	service *slides.Service
}

func NewClient(ctx context.Context, client *http.Client) (*Client, error) {
	service, err := slides.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("failed to create slides service: %w", err)
	}
	return &Client{service: service}, nil
}

func (c *Client) CreatePresentation(title string) (*slides.Presentation, error) {
	presentation := &slides.Presentation{
		Title: title,
	}
	return c.service.Presentations.Create(presentation).Do()
}

func (c *Client) GetPresentation(presentationId string) (*slides.Presentation, error) {
	return c.service.Presentations.Get(presentationId).Do()
}

func (c *Client) ListPresentations() ([]*slides.Presentation, error) {
	// Note: Slides API doesn't have a direct list method like Drive
	// This would typically be done through Drive API
	return nil, fmt.Errorf("use Drive API to list presentations")
}

func (c *Client) CreateSlide(presentationId string, insertionIndex int) (*slides.BatchUpdatePresentationResponse, error) {
	requests := []*slides.Request{
		{
			CreateSlide: &slides.CreateSlideRequest{
				InsertionIndex: int64(insertionIndex),
			},
		},
	}

	req := &slides.BatchUpdatePresentationRequest{
		Requests: requests,
	}

	return c.service.Presentations.BatchUpdate(presentationId, req).Do()
}

func (c *Client) DeleteSlide(presentationId string, slideId string) (*slides.BatchUpdatePresentationResponse, error) {
	requests := []*slides.Request{
		{
			DeleteObject: &slides.DeleteObjectRequest{
				ObjectId: slideId,
			},
		},
	}

	req := &slides.BatchUpdatePresentationRequest{
		Requests: requests,
	}

	return c.service.Presentations.BatchUpdate(presentationId, req).Do()
}

func (c *Client) DuplicateSlide(presentationId string, slideId string) (*slides.BatchUpdatePresentationResponse, error) {
	requests := []*slides.Request{
		{
			DuplicateObject: &slides.DuplicateObjectRequest{
				ObjectId: slideId,
			},
		},
	}

	req := &slides.BatchUpdatePresentationRequest{
		Requests: requests,
	}

	return c.service.Presentations.BatchUpdate(presentationId, req).Do()
}

func (c *Client) AddTextBox(presentationId string, slideId string, text string, x, y, width, height float64) (*slides.BatchUpdatePresentationResponse, error) {
	elementId := fmt.Sprintf("textbox_%s", generateId())

	requests := []*slides.Request{
		{
			CreateShape: &slides.CreateShapeRequest{
				ObjectId:  elementId,
				ShapeType: "TEXT_BOX",
				ElementProperties: &slides.PageElementProperties{
					PageObjectId: slideId,
					Size: &slides.Size{
						Width: &slides.Dimension{
							Magnitude: width,
							Unit:      "PT",
						},
						Height: &slides.Dimension{
							Magnitude: height,
							Unit:      "PT",
						},
					},
					Transform: &slides.AffineTransform{
						TranslateX: x,
						TranslateY: y,
						Unit:       "PT",
					},
				},
			},
		},
		{
			InsertText: &slides.InsertTextRequest{
				ObjectId:       elementId,
				InsertionIndex: 0,
				Text:           text,
			},
		},
	}

	req := &slides.BatchUpdatePresentationRequest{
		Requests: requests,
	}

	return c.service.Presentations.BatchUpdate(presentationId, req).Do()
}

func (c *Client) AddImage(presentationId string, slideId string, imageUrl string, x, y, width, height float64) (*slides.BatchUpdatePresentationResponse, error) {
	elementId := fmt.Sprintf("image_%s", generateId())

	requests := []*slides.Request{
		{
			CreateImage: &slides.CreateImageRequest{
				ObjectId: elementId,
				Url:      imageUrl,
				ElementProperties: &slides.PageElementProperties{
					PageObjectId: slideId,
					Size: &slides.Size{
						Width: &slides.Dimension{
							Magnitude: width,
							Unit:      "PT",
						},
						Height: &slides.Dimension{
							Magnitude: height,
							Unit:      "PT",
						},
					},
					Transform: &slides.AffineTransform{
						TranslateX: x,
						TranslateY: y,
						Unit:       "PT",
					},
				},
			},
		},
	}

	req := &slides.BatchUpdatePresentationRequest{
		Requests: requests,
	}

	return c.service.Presentations.BatchUpdate(presentationId, req).Do()
}

func (c *Client) AddTable(presentationId string, slideId string, rows, columns int, x, y, width, height float64) (*slides.BatchUpdatePresentationResponse, error) {
	elementId := fmt.Sprintf("table_%s", generateId())

	requests := []*slides.Request{
		{
			CreateTable: &slides.CreateTableRequest{
				ObjectId: elementId,
				Rows:     int64(rows),
				Columns:  int64(columns),
				ElementProperties: &slides.PageElementProperties{
					PageObjectId: slideId,
					Size: &slides.Size{
						Width: &slides.Dimension{
							Magnitude: width,
							Unit:      "PT",
						},
						Height: &slides.Dimension{
							Magnitude: height,
							Unit:      "PT",
						},
					},
					Transform: &slides.AffineTransform{
						TranslateX: x,
						TranslateY: y,
						Unit:       "PT",
					},
				},
			},
		},
	}

	req := &slides.BatchUpdatePresentationRequest{
		Requests: requests,
	}

	return c.service.Presentations.BatchUpdate(presentationId, req).Do()
}

func (c *Client) AddShape(presentationId string, slideId string, shapeType string, x, y, width, height float64) (*slides.BatchUpdatePresentationResponse, error) {
	elementId := fmt.Sprintf("shape_%s", generateId())

	requests := []*slides.Request{
		{
			CreateShape: &slides.CreateShapeRequest{
				ObjectId:  elementId,
				ShapeType: shapeType,
				ElementProperties: &slides.PageElementProperties{
					PageObjectId: slideId,
					Size: &slides.Size{
						Width: &slides.Dimension{
							Magnitude: width,
							Unit:      "PT",
						},
						Height: &slides.Dimension{
							Magnitude: height,
							Unit:      "PT",
						},
					},
					Transform: &slides.AffineTransform{
						TranslateX: x,
						TranslateY: y,
						Unit:       "PT",
					},
				},
			},
		},
	}

	req := &slides.BatchUpdatePresentationRequest{
		Requests: requests,
	}

	return c.service.Presentations.BatchUpdate(presentationId, req).Do()
}

func (c *Client) ApplyTemplate(presentationId string, templateId string) (*slides.BatchUpdatePresentationResponse, error) {
	// This would require fetching the template and applying its layouts
	// Complex operation that might need multiple API calls
	return nil, fmt.Errorf("template application not yet implemented")
}

func (c *Client) SetSlideLayout(presentationId string, slideId string, layoutId string) (*slides.BatchUpdatePresentationResponse, error) {
	requests := []*slides.Request{
		{
			UpdatePageProperties: &slides.UpdatePagePropertiesRequest{
				ObjectId: slideId,
				PageProperties: &slides.PageProperties{
					PageBackgroundFill: &slides.PageBackgroundFill{},
				},
				Fields: "pageBackgroundFill",
			},
		},
	}

	req := &slides.BatchUpdatePresentationRequest{
		Requests: requests,
	}

	return c.service.Presentations.BatchUpdate(presentationId, req).Do()
}

func (c *Client) BatchUpdate(presentationId string, requests []*slides.Request) (*slides.BatchUpdatePresentationResponse, error) {
	req := &slides.BatchUpdatePresentationRequest{
		Requests: requests,
	}

	return c.service.Presentations.BatchUpdate(presentationId, req).Do()
}

func generateId() string {
	// Simple ID generator - in production, use UUID or similar
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
