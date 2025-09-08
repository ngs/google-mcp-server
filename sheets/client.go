package sheets

import (
	"context"
	"fmt"

	"go.ngs.io/google-mcp-server/auth"
	"google.golang.org/api/sheets/v4"
)

// Client wraps the Google Sheets API client
type Client struct {
	service *sheets.Service
	ctx     context.Context
}

// NewClient creates a new Sheets client
func NewClient(ctx context.Context, oauth *auth.OAuthClient) (*Client, error) {
	service, err := sheets.NewService(ctx, oauth.GetClientOption())
	if err != nil {
		return nil, fmt.Errorf("failed to create sheets service: %w", err)
	}

	return &Client{
		service: service,
		ctx:     ctx,
	}, nil
}

// GetSpreadsheet gets spreadsheet metadata
func (c *Client) GetSpreadsheet(spreadsheetID string) (*sheets.Spreadsheet, error) {
	spreadsheet, err := c.service.Spreadsheets.Get(spreadsheetID).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get spreadsheet: %w", err)
	}
	return spreadsheet, nil
}

// GetValues gets cell values from a range
func (c *Client) GetValues(spreadsheetID, range_ string) (*sheets.ValueRange, error) {
	values, err := c.service.Spreadsheets.Values.Get(spreadsheetID, range_).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get values: %w", err)
	}
	return values, nil
}

// UpdateValues updates cell values in a range
func (c *Client) UpdateValues(spreadsheetID, range_ string, values [][]interface{}) (*sheets.UpdateValuesResponse, error) {
	valueRange := &sheets.ValueRange{
		Values: values,
	}
	response, err := c.service.Spreadsheets.Values.Update(spreadsheetID, range_, valueRange).
		ValueInputOption("USER_ENTERED").Do()
	if err != nil {
		return nil, fmt.Errorf("failed to update values: %w", err)
	}
	return response, nil
}