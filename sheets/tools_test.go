package sheets

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"google.golang.org/api/sheets/v4"
)

// TestDefaultSheetsToolsSchemas verifies that every tool declares the required
// parameters the handlers rely on.
func TestDefaultSheetsToolsSchemas(t *testing.T) {
	required := map[string][]string{
		"sheets_spreadsheet_create": {"title"},
		"sheets_spreadsheet_get":    {"spreadsheet_id"},
		"sheets_values_get":         {"spreadsheet_id", "range"},
		"sheets_values_update":      {"spreadsheet_id", "range", "values"},
		"sheets_values_append":      {"spreadsheet_id", "range", "values"},
		"sheets_values_clear":       {"spreadsheet_id", "range"},
		"sheets_sheet_duplicate":    {"spreadsheet_id", "sheet_id"},
		"sheets_sheet_add":          {"spreadsheet_id", "title"},
		"sheets_sheet_delete":       {"spreadsheet_id", "sheet_id"},
		"sheets_sheet_update":       {"spreadsheet_id", "sheet_id"},
		"sheets_dimension_insert":   {"spreadsheet_id", "sheet_id", "dimension", "start_index", "count"},
		"sheets_dimension_delete":   {"spreadsheet_id", "sheet_id", "dimension", "start_index", "count"},
	}

	tools := defaultSheetsTools()
	if len(tools) != len(required) {
		t.Errorf("Expected %d tools, got %d", len(required), len(tools))
	}

	seen := make(map[string]bool, len(tools))
	for _, tool := range tools {
		want, ok := required[tool.Name]
		if !ok {
			t.Errorf("Unexpected tool %s", tool.Name)
			continue
		}
		if seen[tool.Name] {
			t.Errorf("Tool %s is listed more than once", tool.Name)
		}
		seen[tool.Name] = true

		got := make(map[string]bool, len(tool.InputSchema.Required))
		for _, name := range tool.InputSchema.Required {
			got[name] = true
		}
		for _, name := range want {
			if !got[name] {
				t.Errorf("Tool %s should require %s", tool.Name, name)
			}
			if _, ok := tool.InputSchema.Properties[name]; !ok {
				t.Errorf("Tool %s should declare property %s", tool.Name, name)
			}
		}
	}

	for name := range required {
		if !seen[name] {
			t.Errorf("Tool %s is missing from the registry", name)
		}
	}
}

// TestDestructiveToolsAreLabeled ensures destructive tools warn about data loss
// in their description.
func TestDestructiveToolsAreLabeled(t *testing.T) {
	destructive := map[string]bool{
		"sheets_sheet_delete":     true,
		"sheets_dimension_delete": true,
		"sheets_values_clear":     true,
	}

	seen := make(map[string]bool, len(destructive))
	for _, tool := range defaultSheetsTools() {
		if !destructive[tool.Name] {
			continue
		}
		seen[tool.Name] = true
		if !strings.Contains(tool.Description, "Destructive") {
			t.Errorf("Tool %s should be labeled as destructive, got %q", tool.Name, tool.Description)
		}
	}

	for name := range destructive {
		if !seen[name] {
			t.Errorf("Tool %s is missing from the registry", name)
		}
	}
}

// TestDimensionToolsUseEnum checks that the dimension parameter is constrained
// to the values the API accepts.
func TestDimensionToolsUseEnum(t *testing.T) {
	seen := make(map[string]bool, 2)
	for _, tool := range defaultSheetsTools() {
		if tool.Name != "sheets_dimension_insert" && tool.Name != "sheets_dimension_delete" {
			continue
		}
		seen[tool.Name] = true
		enum := tool.InputSchema.Properties["dimension"].Enum
		if len(enum) != 2 || enum[0] != "ROWS" || enum[1] != "COLUMNS" {
			t.Errorf("Tool %s should constrain dimension to ROWS/COLUMNS, got %v", tool.Name, enum)
		}
	}

	for _, name := range []string{"sheets_dimension_insert", "sheets_dimension_delete"} {
		if !seen[name] {
			t.Errorf("Tool %s is missing from the registry", name)
		}
	}
}

func TestHandleToolCallUnknownTool(t *testing.T) {
	handler := NewHandler(&Client{})

	if _, err := handler.HandleToolCall(context.Background(), "sheets_unknown", json.RawMessage(`{}`)); err == nil {
		t.Fatal("expected an error for an unknown tool")
	}
}

func TestHandleToolCallInvalidArguments(t *testing.T) {
	handler := NewHandler(&Client{})

	for _, tool := range defaultSheetsTools() {
		_, err := handler.HandleToolCall(context.Background(), tool.Name, json.RawMessage(`"not-an-object"`))
		if err == nil {
			t.Errorf("Tool %s should reject malformed arguments", tool.Name)
		}
	}
}

func TestValidateDimension(t *testing.T) {
	for _, dimension := range []string{"ROWS", "COLUMNS"} {
		if err := validateDimension(dimension); err != nil {
			t.Errorf("validateDimension(%q) returned an error: %v", dimension, err)
		}
	}

	for _, dimension := range []string{"", "rows", "SHEETS"} {
		if err := validateDimension(dimension); err == nil {
			t.Errorf("validateDimension(%q) should have returned an error", dimension)
		}
	}
}

// TestDimensionArgumentValidation covers the checks that run before any API
// call is made.
func TestDimensionArgumentValidation(t *testing.T) {
	client := &Client{}

	if err := client.InsertDimension("sheet", 0, "ROWS", 0, 0, true); err == nil {
		t.Error("InsertDimension should reject a count of 0")
	}
	if err := client.InsertDimension("sheet", 0, "ROWS", -1, 1, true); err == nil {
		t.Error("InsertDimension should reject a negative start index")
	}
	if err := client.DeleteDimension("sheet", 0, "PAGES", 0, 1); err == nil {
		t.Error("DeleteDimension should reject an unknown dimension")
	}
	if err := client.DeleteDimension("sheet", 0, "COLUMNS", 0, -1); err == nil {
		t.Error("DeleteDimension should reject a negative count")
	}
	if err := client.InsertDimension("sheet", 0, "ROWS", 0, 1, true); err == nil {
		t.Error("InsertDimension should reject inherit_from_before at start index 0")
	}
}

// TestDefaultInheritFromBefore covers the inherit_from_before default: false at
// index 0 (nothing precedes the insertion), true everywhere else.
func TestDefaultInheritFromBefore(t *testing.T) {
	if defaultInheritFromBefore(0) {
		t.Error("defaultInheritFromBefore(0) should be false")
	}
	if !defaultInheritFromBefore(1) {
		t.Error("defaultInheritFromBefore(1) should be true")
	}
}

// TestUpdateSheetPropertiesWithoutFields verifies that an update with nothing to
// change fails before reaching the API.
func TestUpdateSheetPropertiesWithoutFields(t *testing.T) {
	client := &Client{}

	if _, err := client.UpdateSheetProperties("sheet", 0, nil, nil, nil); err == nil {
		t.Error("UpdateSheetProperties should reject an update with no properties")
	}
}

// TestAppendValuesInvalidValueInputOption verifies the value input option is
// validated before reaching the API.
func TestAppendValuesInvalidValueInputOption(t *testing.T) {
	client := &Client{}

	if _, err := client.AppendValues("sheet", "A1", nil, "MAGIC"); err == nil {
		t.Error("AppendValues should reject an unknown value input option")
	}
}

// TestFormatSheetListSkipsMissingProperties guards the response formatting
// against sheets without properties.
func TestFormatSheetListSkipsMissingProperties(t *testing.T) {
	if got := formatSheetList(nil); len(got) != 0 {
		t.Errorf("Expected an empty list, got %v", got)
	}

	list := []*sheets.Sheet{
		{Properties: &sheets.SheetProperties{SheetId: 42, Title: "Sheet1", Index: 1}},
		{},
	}

	got := formatSheetList(list)
	if len(got) != 1 {
		t.Fatalf("Expected 1 formatted sheet, got %d", len(got))
	}
	if got[0]["sheetId"] != int64(42) || got[0]["title"] != "Sheet1" || got[0]["index"] != int64(1) {
		t.Errorf("Unexpected formatted sheet: %v", got[0])
	}
}
