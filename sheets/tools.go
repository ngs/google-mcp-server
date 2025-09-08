package sheets

import (
	"context"
	"encoding/json"
	"fmt"

	"go.ngs.io/google-mcp-server/server"
)

// Handler implements the ServiceHandler interface for Sheets
type Handler struct {
	client *Client
}

// NewHandler creates a new Sheets handler
func NewHandler(client *Client) *Handler {
	return &Handler{client: client}
}

// GetTools returns the available Sheets tools
func (h *Handler) GetTools() []server.Tool {
	return []server.Tool{
		{
			Name:        "sheets_spreadsheet_get",
			Description: "Get spreadsheet metadata",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"spreadsheet_id": {
						Type:        "string",
						Description: "Spreadsheet ID",
					},
				},
				Required: []string{"spreadsheet_id"},
			},
		},
		{
			Name:        "sheets_values_get",
			Description: "Get cell values from a range",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"spreadsheet_id": {
						Type:        "string",
						Description: "Spreadsheet ID",
					},
					"range": {
						Type:        "string",
						Description: "A1 notation range (e.g., 'Sheet1!A1:B10')",
					},
				},
				Required: []string{"spreadsheet_id", "range"},
			},
		},
		{
			Name:        "sheets_values_update",
			Description: "Update cell values in a range",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"spreadsheet_id": {
						Type:        "string",
						Description: "Spreadsheet ID",
					},
					"range": {
						Type:        "string",
						Description: "A1 notation range",
					},
					"values": {
						Type:        "array",
						Description: "2D array of values",
						Items: &server.Property{
							Type: "array",
						},
					},
				},
				Required: []string{"spreadsheet_id", "range", "values"},
			},
		},
	}
}

// HandleToolCall handles a tool call for Sheets service
func (h *Handler) HandleToolCall(ctx context.Context, name string, arguments json.RawMessage) (interface{}, error) {
	switch name {
	case "sheets_spreadsheet_get":
		var args struct {
			SpreadsheetID string `json:"spreadsheet_id"`
		}
		if err := json.Unmarshal(arguments, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
		spreadsheet, err := h.client.GetSpreadsheet(args.SpreadsheetID)
		if err != nil {
			return nil, err
		}

		// Format spreadsheet metadata for response
		result := map[string]interface{}{
			"spreadsheetId":  spreadsheet.SpreadsheetId,
			"spreadsheetUrl": spreadsheet.SpreadsheetUrl,
			"title":          spreadsheet.Properties.Title,
		}

		// Add sheets information
		if len(spreadsheet.Sheets) > 0 {
			sheets := make([]map[string]interface{}, len(spreadsheet.Sheets))
			for i, sheet := range spreadsheet.Sheets {
				sheets[i] = map[string]interface{}{
					"sheetId": sheet.Properties.SheetId,
					"title":   sheet.Properties.Title,
					"index":   sheet.Properties.Index,
				}
			}
			result["sheets"] = sheets
		}

		return result, nil

	case "sheets_values_get":
		var args struct {
			SpreadsheetID string `json:"spreadsheet_id"`
			Range         string `json:"range"`
		}
		if err := json.Unmarshal(arguments, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
		values, err := h.client.GetValues(args.SpreadsheetID, args.Range)
		if err != nil {
			return nil, err
		}

		// Format values response
		result := map[string]interface{}{
			"range":          values.Range,
			"majorDimension": values.MajorDimension,
			"values":         values.Values,
		}
		return result, nil

	case "sheets_values_update":
		var args struct {
			SpreadsheetID string          `json:"spreadsheet_id"`
			Range         string          `json:"range"`
			Values        [][]interface{} `json:"values"`
		}
		if err := json.Unmarshal(arguments, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
		response, err := h.client.UpdateValues(args.SpreadsheetID, args.Range, args.Values)
		if err != nil {
			return nil, err
		}

		// Format update response
		result := map[string]interface{}{
			"spreadsheetId":  response.SpreadsheetId,
			"updatedRange":   response.UpdatedRange,
			"updatedRows":    response.UpdatedRows,
			"updatedColumns": response.UpdatedColumns,
			"updatedCells":   response.UpdatedCells,
		}
		return result, nil

	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

// GetResources returns the available Sheets resources
func (h *Handler) GetResources() []server.Resource {
	return []server.Resource{}
}

// HandleResourceCall handles a resource call for Sheets service
func (h *Handler) HandleResourceCall(ctx context.Context, uri string) (interface{}, error) {
	return nil, fmt.Errorf("no resources available for sheets")
}
