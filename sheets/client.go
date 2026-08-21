package sheets

import (
	"context"
	"fmt"
	"strings"

	"go.ngs.io/google-mcp-server/auth"
	"google.golang.org/api/sheets/v4"
)

// Client wraps the Google Sheets API client
type Client struct {
	service *sheets.Service
}

// NewClient creates a new Sheets client
func NewClient(ctx context.Context, oauth *auth.OAuthClient) (*Client, error) {
	if oauth == nil {
		return nil, fmt.Errorf("no OAuth client available")
	}

	service, err := sheets.NewService(ctx, oauth.GetClientOption())
	if err != nil {
		return nil, fmt.Errorf("failed to create sheets service: %w", err)
	}

	return &Client{
		service: service,
	}, nil
}

// CreateSpreadsheet creates a new spreadsheet.
// sheetTitles is optional; when provided the spreadsheet starts with those sheets.
func (c *Client) CreateSpreadsheet(title string, sheetTitles []string) (*sheets.Spreadsheet, error) {
	spreadsheet := &sheets.Spreadsheet{
		Properties: &sheets.SpreadsheetProperties{
			Title: title,
		},
	}

	for i, sheetTitle := range sheetTitles {
		spreadsheet.Sheets = append(spreadsheet.Sheets, &sheets.Sheet{
			Properties: &sheets.SheetProperties{
				Title: sheetTitle,
				Index: int64(i),
			},
		})
	}

	created, err := c.service.Spreadsheets.Create(spreadsheet).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to create spreadsheet: %w", err)
	}
	return created, nil
}

// GetSpreadsheet gets spreadsheet metadata
func (c *Client) GetSpreadsheet(spreadsheetID string) (*sheets.Spreadsheet, error) {
	spreadsheet, err := c.service.Spreadsheets.Get(spreadsheetID).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get spreadsheet: %w", err)
	}
	return spreadsheet, nil
}

// formatSheetList formats sheet metadata for tool responses
func formatSheetList(list []*sheets.Sheet) []map[string]interface{} {
	formatted := make([]map[string]interface{}, 0, len(list))
	for _, sheet := range list {
		if sheet.Properties == nil {
			continue
		}
		formatted = append(formatted, map[string]interface{}{
			"sheetId": sheet.Properties.SheetId,
			"title":   sheet.Properties.Title,
			"index":   sheet.Properties.Index,
		})
	}
	return formatted
}

// batchUpdate sends a single request through the spreadsheets.batchUpdate endpoint
func (c *Client) batchUpdate(spreadsheetID string, request *sheets.Request) (*sheets.BatchUpdateSpreadsheetResponse, error) {
	batchRequest := &sheets.BatchUpdateSpreadsheetRequest{
		Requests: []*sheets.Request{request},
	}
	return c.service.Spreadsheets.BatchUpdate(spreadsheetID, batchRequest).Do()
}

// DuplicateSheet duplicates a sheet (tab) within a spreadsheet.
// newName is optional; when empty the API assigns a default title ("Copy of ...").
// insertIndex is optional; when nil the copy is inserted right after the source sheet.
func (c *Client) DuplicateSheet(spreadsheetID string, sheetID int64, newName string, insertIndex *int64) (*sheets.SheetProperties, error) {
	request := &sheets.DuplicateSheetRequest{
		SourceSheetId: sheetID,
		// SourceSheetId may legitimately be 0 (the default sheet)
		ForceSendFields: []string{"SourceSheetId"},
	}
	if newName != "" {
		request.NewSheetName = newName
	}

	if insertIndex != nil {
		request.InsertSheetIndex = *insertIndex
		request.ForceSendFields = append(request.ForceSendFields, "InsertSheetIndex")
	} else {
		// Default to the position right after the source sheet
		sourceIndex, err := c.getSheetIndex(spreadsheetID, sheetID)
		if err != nil {
			return nil, err
		}
		request.InsertSheetIndex = sourceIndex + 1
		request.ForceSendFields = append(request.ForceSendFields, "InsertSheetIndex")
	}

	response, err := c.batchUpdate(spreadsheetID, &sheets.Request{DuplicateSheet: request})
	if err != nil {
		return nil, fmt.Errorf("failed to duplicate sheet: %w", err)
	}

	if len(response.Replies) == 0 || response.Replies[0].DuplicateSheet == nil ||
		response.Replies[0].DuplicateSheet.Properties == nil {
		return nil, fmt.Errorf("failed to duplicate sheet: no sheet properties returned")
	}

	return response.Replies[0].DuplicateSheet.Properties, nil
}

// AddSheet adds a new sheet (tab) to a spreadsheet.
// index, rowCount and columnCount are optional; nil leaves the API defaults in place.
func (c *Client) AddSheet(spreadsheetID, title string, index *int64, rowCount, columnCount *int64) (*sheets.SheetProperties, error) {
	properties := &sheets.SheetProperties{
		Title: title,
	}

	if index != nil {
		properties.Index = *index
		properties.ForceSendFields = append(properties.ForceSendFields, "Index")
	}

	if rowCount != nil || columnCount != nil {
		properties.GridProperties = &sheets.GridProperties{}
		if rowCount != nil {
			properties.GridProperties.RowCount = *rowCount
			properties.GridProperties.ForceSendFields = append(properties.GridProperties.ForceSendFields, "RowCount")
		}
		if columnCount != nil {
			properties.GridProperties.ColumnCount = *columnCount
			properties.GridProperties.ForceSendFields = append(properties.GridProperties.ForceSendFields, "ColumnCount")
		}
	}

	response, err := c.batchUpdate(spreadsheetID, &sheets.Request{
		AddSheet: &sheets.AddSheetRequest{Properties: properties},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to add sheet: %w", err)
	}

	if len(response.Replies) == 0 || response.Replies[0].AddSheet == nil ||
		response.Replies[0].AddSheet.Properties == nil {
		return nil, fmt.Errorf("failed to add sheet: no sheet properties returned")
	}

	return response.Replies[0].AddSheet.Properties, nil
}

// DeleteSheet deletes a sheet (tab) from a spreadsheet. This is destructive.
func (c *Client) DeleteSheet(spreadsheetID string, sheetID int64) error {
	_, err := c.batchUpdate(spreadsheetID, &sheets.Request{
		DeleteSheet: &sheets.DeleteSheetRequest{
			SheetId: sheetID,
			// SheetId may legitimately be 0 (the default sheet)
			ForceSendFields: []string{"SheetId"},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to delete sheet: %w", err)
	}
	return nil
}

// UpdateSheetProperties updates the properties of a sheet (tab).
// Only the non-nil arguments are sent in the field mask.
func (c *Client) UpdateSheetProperties(spreadsheetID string, sheetID int64, newTitle *string, newIndex *int64, hidden *bool) (*sheets.SheetProperties, error) {
	properties := &sheets.SheetProperties{
		SheetId: sheetID,
		// SheetId may legitimately be 0 (the default sheet)
		ForceSendFields: []string{"SheetId"},
	}

	var fields []string
	if newTitle != nil {
		properties.Title = *newTitle
		properties.ForceSendFields = append(properties.ForceSendFields, "Title")
		fields = append(fields, "title")
	}
	if newIndex != nil {
		properties.Index = *newIndex
		properties.ForceSendFields = append(properties.ForceSendFields, "Index")
		fields = append(fields, "index")
	}
	if hidden != nil {
		properties.Hidden = *hidden
		properties.ForceSendFields = append(properties.ForceSendFields, "Hidden")
		fields = append(fields, "hidden")
	}

	if len(fields) == 0 {
		return nil, fmt.Errorf("failed to update sheet: no properties to update")
	}

	_, err := c.batchUpdate(spreadsheetID, &sheets.Request{
		UpdateSheetProperties: &sheets.UpdateSheetPropertiesRequest{
			Properties: properties,
			Fields:     strings.Join(fields, ","),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update sheet: %w", err)
	}

	// UpdateSheetProperties returns no reply, so read the sheet back
	spreadsheet, err := c.GetSpreadsheet(spreadsheetID)
	if err != nil {
		return nil, err
	}
	for _, sheet := range spreadsheet.Sheets {
		if sheet.Properties != nil && sheet.Properties.SheetId == sheetID {
			return sheet.Properties, nil
		}
	}

	return nil, fmt.Errorf("sheet %d not found in spreadsheet %s", sheetID, spreadsheetID)
}

// InsertDimension inserts rows or columns into a sheet.
// dimension is "ROWS" or "COLUMNS"; startIndex is zero-based and count must be positive.
func (c *Client) InsertDimension(spreadsheetID string, sheetID int64, dimension string, startIndex, count int64, inheritFromBefore bool) error {
	if err := validateDimension(dimension); err != nil {
		return err
	}
	if count <= 0 {
		return fmt.Errorf("invalid count: must be greater than 0")
	}
	if startIndex < 0 {
		return fmt.Errorf("invalid start_index: must be 0 or greater")
	}
	if inheritFromBefore && startIndex == 0 {
		return fmt.Errorf("invalid inherit_from_before: must be false when start_index is 0 because there is no preceding row or column")
	}

	_, err := c.batchUpdate(spreadsheetID, &sheets.Request{
		InsertDimension: &sheets.InsertDimensionRequest{
			Range: &sheets.DimensionRange{
				SheetId:    sheetID,
				Dimension:  dimension,
				StartIndex: startIndex,
				EndIndex:   startIndex + count,
				// SheetId may legitimately be 0 (the default sheet)
				ForceSendFields: []string{"SheetId", "StartIndex"},
			},
			InheritFromBefore: inheritFromBefore,
			ForceSendFields:   []string{"InheritFromBefore"},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to insert dimension: %w", err)
	}
	return nil
}

// DeleteDimension deletes rows or columns from a sheet. This is destructive.
func (c *Client) DeleteDimension(spreadsheetID string, sheetID int64, dimension string, startIndex, count int64) error {
	if err := validateDimension(dimension); err != nil {
		return err
	}
	if count <= 0 {
		return fmt.Errorf("invalid count: must be greater than 0")
	}
	if startIndex < 0 {
		return fmt.Errorf("invalid start_index: must be 0 or greater")
	}

	_, err := c.batchUpdate(spreadsheetID, &sheets.Request{
		DeleteDimension: &sheets.DeleteDimensionRequest{
			Range: &sheets.DimensionRange{
				SheetId:    sheetID,
				Dimension:  dimension,
				StartIndex: startIndex,
				EndIndex:   startIndex + count,
				// SheetId may legitimately be 0 (the default sheet)
				ForceSendFields: []string{"SheetId", "StartIndex"},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to delete dimension: %w", err)
	}
	return nil
}

// defaultInheritFromBefore picks the inherit_from_before default for an
// insertion: true in general, but false at index 0 where the API rejects
// inheriting because no preceding dimension exists.
func defaultInheritFromBefore(startIndex int64) bool {
	return startIndex > 0
}

// validateDimension checks that a dimension is one of the values accepted by the API
func validateDimension(dimension string) error {
	switch dimension {
	case "ROWS", "COLUMNS":
		return nil
	default:
		return fmt.Errorf("invalid dimension: %q (must be ROWS or COLUMNS)", dimension)
	}
}

// getSheetIndex resolves the current index of a sheet within a spreadsheet
func (c *Client) getSheetIndex(spreadsheetID string, sheetID int64) (int64, error) {
	spreadsheet, err := c.GetSpreadsheet(spreadsheetID)
	if err != nil {
		return 0, err
	}

	for _, sheet := range spreadsheet.Sheets {
		if sheet.Properties != nil && sheet.Properties.SheetId == sheetID {
			return sheet.Properties.Index, nil
		}
	}

	return 0, fmt.Errorf("sheet %d not found in spreadsheet %s", sheetID, spreadsheetID)
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

// AppendValues appends rows after the last row of data in a range.
// valueInputOption is optional and defaults to "USER_ENTERED".
func (c *Client) AppendValues(spreadsheetID, range_ string, values [][]interface{}, valueInputOption string) (*sheets.AppendValuesResponse, error) {
	if valueInputOption == "" {
		valueInputOption = "USER_ENTERED"
	}
	if valueInputOption != "USER_ENTERED" && valueInputOption != "RAW" {
		return nil, fmt.Errorf("invalid value_input_option: %q (must be USER_ENTERED or RAW)", valueInputOption)
	}

	valueRange := &sheets.ValueRange{
		Values: values,
	}
	response, err := c.service.Spreadsheets.Values.Append(spreadsheetID, range_, valueRange).
		ValueInputOption(valueInputOption).
		InsertDataOption("INSERT_ROWS").Do()
	if err != nil {
		return nil, fmt.Errorf("failed to append values: %w", err)
	}
	return response, nil
}

// ClearValues clears the values in a range. This is destructive.
func (c *Client) ClearValues(spreadsheetID, range_ string) (*sheets.ClearValuesResponse, error) {
	response, err := c.service.Spreadsheets.Values.Clear(spreadsheetID, range_, &sheets.ClearValuesRequest{}).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to clear values: %w", err)
	}
	return response, nil
}
