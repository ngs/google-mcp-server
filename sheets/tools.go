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
	return defaultSheetsTools()
}

// defaultSheetsTools returns the Sheets tool definitions. It is shared by
// Handler and MultiAccountHandler so the definitions are not maintained twice.
func defaultSheetsTools() []server.Tool {
	return []server.Tool{
		{
			Name:        "sheets_spreadsheet_create",
			Description: "Create a new spreadsheet",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"title": {
						Type:        "string",
						Description: "Spreadsheet title",
					},
					"sheet_titles": {
						Type:        "array",
						Description: "Titles of the initial sheets (optional)",
						Items: &server.Property{
							Type: "string",
						},
					},
				},
				Required: []string{"title"},
			},
		},
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
		{
			Name:        "sheets_sheet_duplicate",
			Description: "Duplicate a sheet (tab) within a spreadsheet",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"spreadsheet_id": {
						Type:        "string",
						Description: "Spreadsheet ID",
					},
					"sheet_id": {
						Type:        "number",
						Description: "Sheet ID of the sheet to duplicate",
					},
					"new_name": {
						Type:        "string",
						Description: "Title for the duplicated sheet (optional, defaults to 'Copy of ...')",
					},
					"insert_index": {
						Type:        "number",
						Description: "Zero-based index to insert the duplicated sheet at (optional, defaults to right after the source sheet)",
					},
				},
				Required: []string{"spreadsheet_id", "sheet_id"},
			},
		},
		{
			Name:        "sheets_sheet_add",
			Description: "Add a new sheet (tab) to a spreadsheet",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"spreadsheet_id": {
						Type:        "string",
						Description: "Spreadsheet ID",
					},
					"title": {
						Type:        "string",
						Description: "Title of the new sheet",
					},
					"index": {
						Type:        "number",
						Description: "Zero-based index to insert the sheet at (optional, defaults to last)",
					},
					"row_count": {
						Type:        "number",
						Description: "Number of rows in the new sheet (optional)",
					},
					"column_count": {
						Type:        "number",
						Description: "Number of columns in the new sheet (optional)",
					},
				},
				Required: []string{"spreadsheet_id", "title"},
			},
		},
		{
			Name:        "sheets_sheet_delete",
			Description: "Delete a sheet (tab) from a spreadsheet. Destructive: the sheet and all of its data are removed",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"spreadsheet_id": {
						Type:        "string",
						Description: "Spreadsheet ID",
					},
					"sheet_id": {
						Type:        "number",
						Description: "Sheet ID of the sheet to delete",
					},
				},
				Required: []string{"spreadsheet_id", "sheet_id"},
			},
		},
		{
			Name:        "sheets_sheet_update",
			Description: "Update properties of a sheet (tab), such as its title, position or visibility",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"spreadsheet_id": {
						Type:        "string",
						Description: "Spreadsheet ID",
					},
					"sheet_id": {
						Type:        "number",
						Description: "Sheet ID of the sheet to update",
					},
					"new_title": {
						Type:        "string",
						Description: "New sheet title (optional)",
					},
					"new_index": {
						Type:        "number",
						Description: "New zero-based position of the sheet (optional)",
					},
					"hidden": {
						Type:        "boolean",
						Description: "Whether the sheet is hidden (optional)",
					},
				},
				Required: []string{"spreadsheet_id", "sheet_id"},
			},
		},
		{
			Name:        "sheets_dimension_insert",
			Description: "Insert rows or columns into a sheet",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"spreadsheet_id": {
						Type:        "string",
						Description: "Spreadsheet ID",
					},
					"sheet_id": {
						Type:        "number",
						Description: "Sheet ID to insert into",
					},
					"dimension": {
						Type:        "string",
						Description: "Dimension to insert",
						Enum:        []string{"ROWS", "COLUMNS"},
					},
					"start_index": {
						Type:        "number",
						Description: "Zero-based index to insert at",
					},
					"count": {
						Type:        "number",
						Description: "Number of rows or columns to insert",
					},
					"inherit_from_before": {
						Type:        "boolean",
						Description: "Inherit formatting from the preceding row or column (optional; defaults to true, except at start_index 0 where it defaults to and must be false because no preceding row or column exists)",
					},
				},
				Required: []string{"spreadsheet_id", "sheet_id", "dimension", "start_index", "count"},
			},
		},
		{
			Name:        "sheets_dimension_delete",
			Description: "Delete rows or columns from a sheet. Destructive: the deleted cells and their data are removed",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"spreadsheet_id": {
						Type:        "string",
						Description: "Spreadsheet ID",
					},
					"sheet_id": {
						Type:        "number",
						Description: "Sheet ID to delete from",
					},
					"dimension": {
						Type:        "string",
						Description: "Dimension to delete",
						Enum:        []string{"ROWS", "COLUMNS"},
					},
					"start_index": {
						Type:        "number",
						Description: "Zero-based index to start deleting at",
					},
					"count": {
						Type:        "number",
						Description: "Number of rows or columns to delete",
					},
				},
				Required: []string{"spreadsheet_id", "sheet_id", "dimension", "start_index", "count"},
			},
		},
		{
			Name:        "sheets_values_append",
			Description: "Append rows after the last row of data in a range",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"spreadsheet_id": {
						Type:        "string",
						Description: "Spreadsheet ID",
					},
					"range": {
						Type:        "string",
						Description: "A1 notation range to search for a table (e.g., 'Sheet1!A1')",
					},
					"values": {
						Type:        "array",
						Description: "2D array of values",
						Items: &server.Property{
							Type: "array",
						},
					},
					"value_input_option": {
						Type:        "string",
						Description: "How input values are interpreted (optional, defaults to USER_ENTERED)",
						Enum:        []string{"USER_ENTERED", "RAW"},
					},
				},
				Required: []string{"spreadsheet_id", "range", "values"},
			},
		},
		{
			Name:        "sheets_values_clear",
			Description: "Clear the values in a range. Destructive: the cell values are removed",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"spreadsheet_id": {
						Type:        "string",
						Description: "Spreadsheet ID",
					},
					"range": {
						Type:        "string",
						Description: "A1 notation range to clear",
					},
				},
				Required: []string{"spreadsheet_id", "range"},
			},
		},
	}
}

// HandleToolCall handles a tool call for Sheets service
func (h *Handler) HandleToolCall(ctx context.Context, name string, arguments json.RawMessage) (interface{}, error) {
	switch name {
	case "sheets_spreadsheet_create":
		var args struct {
			Title       string   `json:"title"`
			SheetTitles []string `json:"sheet_titles"`
		}
		if err := json.Unmarshal(arguments, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
		spreadsheet, err := h.client.CreateSpreadsheet(args.Title, args.SheetTitles)
		if err != nil {
			return nil, err
		}

		result := map[string]interface{}{
			"spreadsheetId":  spreadsheet.SpreadsheetId,
			"spreadsheetUrl": spreadsheet.SpreadsheetUrl,
		}
		if spreadsheet.Properties != nil {
			result["title"] = spreadsheet.Properties.Title
		}
		result["sheets"] = formatSheetList(spreadsheet.Sheets)
		return result, nil

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
			result["sheets"] = formatSheetList(spreadsheet.Sheets)
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

	case "sheets_sheet_duplicate":
		var args struct {
			SpreadsheetID string `json:"spreadsheet_id"`
			SheetID       int64  `json:"sheet_id"`
			NewName       string `json:"new_name"`
			InsertIndex   *int64 `json:"insert_index"`
		}
		if err := json.Unmarshal(arguments, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
		properties, err := h.client.DuplicateSheet(args.SpreadsheetID, args.SheetID, args.NewName, args.InsertIndex)
		if err != nil {
			return nil, err
		}

		// Format the duplicated sheet properties for response
		result := map[string]interface{}{
			"spreadsheetId": args.SpreadsheetID,
			"sheetId":       properties.SheetId,
			"title":         properties.Title,
			"index":         properties.Index,
		}
		return result, nil

	case "sheets_sheet_add":
		var args struct {
			SpreadsheetID string `json:"spreadsheet_id"`
			Title         string `json:"title"`
			Index         *int64 `json:"index"`
			RowCount      *int64 `json:"row_count"`
			ColumnCount   *int64 `json:"column_count"`
		}
		if err := json.Unmarshal(arguments, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
		properties, err := h.client.AddSheet(args.SpreadsheetID, args.Title, args.Index, args.RowCount, args.ColumnCount)
		if err != nil {
			return nil, err
		}

		result := map[string]interface{}{
			"spreadsheetId": args.SpreadsheetID,
			"sheetId":       properties.SheetId,
			"title":         properties.Title,
			"index":         properties.Index,
		}
		return result, nil

	case "sheets_sheet_delete":
		var args struct {
			SpreadsheetID string `json:"spreadsheet_id"`
			SheetID       int64  `json:"sheet_id"`
		}
		if err := json.Unmarshal(arguments, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
		if err := h.client.DeleteSheet(args.SpreadsheetID, args.SheetID); err != nil {
			return nil, err
		}

		result := map[string]interface{}{
			"spreadsheetId": args.SpreadsheetID,
			"sheetId":       args.SheetID,
			"deleted":       true,
		}
		return result, nil

	case "sheets_sheet_update":
		var args struct {
			SpreadsheetID string  `json:"spreadsheet_id"`
			SheetID       int64   `json:"sheet_id"`
			NewTitle      *string `json:"new_title"`
			NewIndex      *int64  `json:"new_index"`
			Hidden        *bool   `json:"hidden"`
		}
		if err := json.Unmarshal(arguments, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
		properties, err := h.client.UpdateSheetProperties(args.SpreadsheetID, args.SheetID, args.NewTitle, args.NewIndex, args.Hidden)
		if err != nil {
			return nil, err
		}

		result := map[string]interface{}{
			"spreadsheetId": args.SpreadsheetID,
			"sheetId":       properties.SheetId,
			"title":         properties.Title,
			"index":         properties.Index,
			"hidden":        properties.Hidden,
		}
		return result, nil

	case "sheets_dimension_insert":
		var args struct {
			SpreadsheetID     string `json:"spreadsheet_id"`
			SheetID           int64  `json:"sheet_id"`
			Dimension         string `json:"dimension"`
			StartIndex        int64  `json:"start_index"`
			Count             int64  `json:"count"`
			InheritFromBefore *bool  `json:"inherit_from_before"`
		}
		if err := json.Unmarshal(arguments, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}

		inheritFromBefore := defaultInheritFromBefore(args.StartIndex)
		if args.InheritFromBefore != nil {
			inheritFromBefore = *args.InheritFromBefore
		}

		if err := h.client.InsertDimension(args.SpreadsheetID, args.SheetID, args.Dimension, args.StartIndex, args.Count, inheritFromBefore); err != nil {
			return nil, err
		}

		result := map[string]interface{}{
			"spreadsheetId": args.SpreadsheetID,
			"sheetId":       args.SheetID,
			"dimension":     args.Dimension,
			"startIndex":    args.StartIndex,
			"count":         args.Count,
			"inserted":      true,
		}
		return result, nil

	case "sheets_dimension_delete":
		var args struct {
			SpreadsheetID string `json:"spreadsheet_id"`
			SheetID       int64  `json:"sheet_id"`
			Dimension     string `json:"dimension"`
			StartIndex    int64  `json:"start_index"`
			Count         int64  `json:"count"`
		}
		if err := json.Unmarshal(arguments, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
		if err := h.client.DeleteDimension(args.SpreadsheetID, args.SheetID, args.Dimension, args.StartIndex, args.Count); err != nil {
			return nil, err
		}

		result := map[string]interface{}{
			"spreadsheetId": args.SpreadsheetID,
			"sheetId":       args.SheetID,
			"dimension":     args.Dimension,
			"startIndex":    args.StartIndex,
			"count":         args.Count,
			"deleted":       true,
		}
		return result, nil

	case "sheets_values_append":
		var args struct {
			SpreadsheetID    string          `json:"spreadsheet_id"`
			Range            string          `json:"range"`
			Values           [][]interface{} `json:"values"`
			ValueInputOption string          `json:"value_input_option"`
		}
		if err := json.Unmarshal(arguments, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
		response, err := h.client.AppendValues(args.SpreadsheetID, args.Range, args.Values, args.ValueInputOption)
		if err != nil {
			return nil, err
		}

		// Format append response
		result := map[string]interface{}{
			"spreadsheetId": response.SpreadsheetId,
			"tableRange":    response.TableRange,
		}
		if response.Updates != nil {
			result["updatedRange"] = response.Updates.UpdatedRange
			result["updatedRows"] = response.Updates.UpdatedRows
			result["updatedColumns"] = response.Updates.UpdatedColumns
			result["updatedCells"] = response.Updates.UpdatedCells
		}
		return result, nil

	case "sheets_values_clear":
		var args struct {
			SpreadsheetID string `json:"spreadsheet_id"`
			Range         string `json:"range"`
		}
		if err := json.Unmarshal(arguments, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
		response, err := h.client.ClearValues(args.SpreadsheetID, args.Range)
		if err != nil {
			return nil, err
		}

		// Format clear response
		result := map[string]interface{}{
			"spreadsheetId": response.SpreadsheetId,
			"clearedRange":  response.ClearedRange,
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
