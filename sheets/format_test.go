package sheets

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"google.golang.org/api/sheets/v4"
)

// ptr returns a pointer to v, for the optional formatting arguments.
func ptr[T any](v T) *T {
	return &v
}

// nullableIndex describes an expected GridRange bound. set=false means the
// bound must be left unset so the range covers whole rows or columns.
type nullableIndex struct {
	set   bool
	value int64
}

func idx(v int64) nullableIndex { return nullableIndex{set: true, value: v} }

func unset() nullableIndex { return nullableIndex{} }

func TestParseA1Range(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		title    string
		startRow nullableIndex
		endRow   nullableIndex
		startCol nullableIndex
		endCol   nullableIndex
	}{
		{"basic", "Sheet1!B46:H51", "Sheet1", idx(45), idx(51), idx(1), idx(8)},
		{"no sheet title", "B46:H51", "", idx(45), idx(51), idx(1), idx(8)},
		{"quoted japanese title", "'2026-09-16-見積-運用込み v2'!B46:H51", "2026-09-16-見積-運用込み v2", idx(45), idx(51), idx(1), idx(8)},
		{"escaped quote in title", "'It''s a sheet'!A1", "It's a sheet", idx(0), idx(1), idx(0), idx(1)},
		{"single cell", "Sheet1!G52", "Sheet1", idx(51), idx(52), idx(6), idx(7)},
		{"single row", "Sheet1!B61:H61", "Sheet1", idx(60), idx(61), idx(1), idx(8)},
		{"single column", "Sheet1!G62:G65", "Sheet1", idx(61), idx(65), idx(6), idx(7)},
		{"whole columns", "Sheet1!A:C", "Sheet1", unset(), unset(), idx(0), idx(3)},
		{"whole rows", "Sheet1!2:5", "Sheet1", idx(1), idx(5), unset(), unset()},
		{"two letter column", "Sheet1!AA1", "Sheet1", idx(0), idx(1), idx(26), idx(27)},
		{"column carry", "Sheet1!ZZ1", "Sheet1", idx(0), idx(1), idx(701), idx(702)},
		{"lowercase column labels", "sheet1!b46:h51", "sheet1", idx(45), idx(51), idx(1), idx(8)},
		{"full width reference", "Sheet1!Ｂ４６:Ｈ５１", "Sheet1", idx(45), idx(51), idx(1), idx(8)},
		{"japanese title is not folded", "日本語シート!B2", "日本語シート", idx(1), idx(2), idx(1), idx(2)},
		{"reversed range is normalized", "Sheet1!H51:B46", "Sheet1", idx(45), idx(51), idx(1), idx(8)},
		{"title containing a bang", "Data!Sheet1!A1", "Data!Sheet1", idx(0), idx(1), idx(0), idx(1)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			title, gr, err := parseA1Range(tt.input)
			if err != nil {
				t.Fatalf("parseA1Range(%q) returned an error: %v", tt.input, err)
			}
			if title != tt.title {
				t.Errorf("sheet title = %q, want %q", title, tt.title)
			}
			checkBound(t, "StartRowIndex", gr.StartRowIndex, gr.ForceSendFields, tt.startRow)
			checkBound(t, "StartColumnIndex", gr.StartColumnIndex, gr.ForceSendFields, tt.startCol)
			checkEndBound(t, "EndRowIndex", gr.EndRowIndex, tt.endRow)
			checkEndBound(t, "EndColumnIndex", gr.EndColumnIndex, tt.endCol)
		})
	}
}

// checkBound verifies a start index, which must appear in ForceSendFields when
// set so that a zero index is not dropped from the request.
func checkBound(t *testing.T, field string, got int64, forceSend []string, want nullableIndex) {
	t.Helper()

	forced := false
	for _, name := range forceSend {
		if name == field {
			forced = true
		}
	}

	if !want.set {
		if forced {
			t.Errorf("%s should be left unset, but it is in ForceSendFields", field)
		}
		if got != 0 {
			t.Errorf("%s = %d, want the zero value while unset", field, got)
		}
		return
	}

	if !forced {
		t.Errorf("%s should be listed in ForceSendFields", field)
	}
	if got != want.value {
		t.Errorf("%s = %d, want %d", field, got, want.value)
	}
}

// checkEndBound verifies an end index. End indices are always positive when
// set, so they do not need ForceSendFields.
func checkEndBound(t *testing.T, field string, got int64, want nullableIndex) {
	t.Helper()

	if !want.set {
		if got != 0 {
			t.Errorf("%s = %d, want the zero value while unset", field, got)
		}
		return
	}
	if got != want.value {
		t.Errorf("%s = %d, want %d", field, got, want.value)
	}
}

func TestParseA1RangeErrors(t *testing.T) {
	for _, input := range []string{
		"Sheet1!",
		"",
		"Sheet1!B0",
		"Sheet1!B46:H",
		"Sheet1!1A",
		"Sheet1!A1:B2:C3",
	} {
		if _, _, err := parseA1Range(input); err == nil {
			t.Errorf("parseA1Range(%q) should have returned an error", input)
		}
	}
}

func TestColumnLabelToIndex(t *testing.T) {
	tests := map[string]int64{
		"A":  0,
		"Z":  25,
		"AA": 26,
		"AZ": 51,
		"BA": 52,
		"ZZ": 701,
		"a":  0,
	}

	for label, want := range tests {
		got, err := columnLabelToIndex(label)
		if err != nil {
			t.Errorf("columnLabelToIndex(%q) returned an error: %v", label, err)
			continue
		}
		if got != want {
			t.Errorf("columnLabelToIndex(%q) = %d, want %d", label, got, want)
		}
	}

	for _, label := range []string{"", "A1"} {
		if _, err := columnLabelToIndex(label); err == nil {
			t.Errorf("columnLabelToIndex(%q) should have returned an error", label)
		}
	}
}

func TestParseColorHex(t *testing.T) {
	tests := []struct {
		input            string
		red, green, blue float64
	}{
		{`"#FFF299"`, 1.0, 242.0 / 255.0, 153.0 / 255.0},
		{`"FFF299"`, 1.0, 242.0 / 255.0, 153.0 / 255.0},
		{`"#FC9"`, 1.0, 204.0 / 255.0, 153.0 / 255.0},
	}

	for _, tt := range tests {
		color, err := parseColor(json.RawMessage(tt.input))
		if err != nil {
			t.Errorf("parseColor(%s) returned an error: %v", tt.input, err)
			continue
		}
		assertNear(t, tt.input+" red", color.Red, tt.red)
		assertNear(t, tt.input+" green", color.Green, tt.green)
		assertNear(t, tt.input+" blue", color.Blue, tt.blue)
	}
}

func TestParseColorObject(t *testing.T) {
	color, err := parseColor(json.RawMessage(`{"red":1,"green":0.95,"blue":0.6}`))
	if err != nil {
		t.Fatalf("parseColor returned an error: %v", err)
	}
	assertNear(t, "red", color.Red, 1)
	assertNear(t, "green", color.Green, 0.95)
	assertNear(t, "blue", color.Blue, 0.6)
}

func TestParseColorErrors(t *testing.T) {
	for _, input := range []string{
		`"#GGGGGG"`,
		`"#FFFF"`,
		`{"red":2}`,
		`"blue"`,
	} {
		if _, err := parseColor(json.RawMessage(input)); err == nil {
			t.Errorf("parseColor(%s) should have returned an error", input)
		}
	}
}

// TestParseColorForcesZeroComponents makes sure a black fill is not dropped
// from the request because every component is zero.
func TestParseColorForcesZeroComponents(t *testing.T) {
	color, err := parseColor(json.RawMessage(`"#000000"`))
	if err != nil {
		t.Fatalf("parseColor returned an error: %v", err)
	}
	for _, field := range []string{"Red", "Green", "Blue"} {
		if !contains(color.ForceSendFields, field) {
			t.Errorf("ForceSendFields should contain %s, got %v", field, color.ForceSendFields)
		}
	}
}

func assertNear(t *testing.T, label string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-6 {
		t.Errorf("%s = %v, want %v", label, got, want)
	}
}

func contains(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}

func TestBuildFormatRequestBackgroundOnly(t *testing.T) {
	request, fields, err := buildFormatRequest(&sheets.GridRange{}, formatArgs{
		BackgroundColor: json.RawMessage(`"#FFF299"`),
	})
	if err != nil {
		t.Fatalf("buildFormatRequest returned an error: %v", err)
	}
	if fields != "userEnteredFormat.backgroundColorStyle" {
		t.Errorf("fields = %q, want userEnteredFormat.backgroundColorStyle", fields)
	}
	cell := request.RepeatCell.Cell
	if cell.UserEnteredFormat.BackgroundColorStyle == nil ||
		cell.UserEnteredFormat.BackgroundColorStyle.RgbColor == nil {
		t.Fatal("the background color style should be set")
	}
}

func TestBuildFormatRequestFieldOrder(t *testing.T) {
	_, fields, err := buildFormatRequest(&sheets.GridRange{}, formatArgs{
		TextFormat:          &cellTextFormatArgs{Bold: ptr(true)},
		HorizontalAlignment: "CENTER",
	})
	if err != nil {
		t.Fatalf("buildFormatRequest returned an error: %v", err)
	}
	want := "userEnteredFormat.textFormat.bold,userEnteredFormat.horizontalAlignment"
	if fields != want {
		t.Errorf("fields = %q, want %q", fields, want)
	}
}

// TestBuildFormatRequestNeverTouchesValues is the core guarantee of the tool:
// no combination of arguments may widen the field mask to cell values.
func TestBuildFormatRequestNeverTouchesValues(t *testing.T) {
	cases := []formatArgs{
		{BackgroundColor: json.RawMessage(`"#FFF299"`)},
		{TextFormat: &cellTextFormatArgs{Bold: ptr(true), Italic: ptr(false), FontSize: ptr(12.0), ForegroundColor: json.RawMessage(`"#000000"`)}},
		{HorizontalAlignment: "RIGHT"},
		{NumberFormat: &cellNumberFormatArgs{Type: "CURRENCY", Pattern: `"¥"#,##0`}},
		{Clear: true},
	}

	for i, args := range cases {
		request, fields, err := buildFormatRequest(&sheets.GridRange{}, args)
		if err != nil {
			t.Errorf("case %d returned an error: %v", i, err)
			continue
		}
		if strings.Contains(fields, "userEnteredValue") {
			t.Errorf("case %d produced a field mask reaching cell values: %q", i, fields)
		}
		if !strings.HasPrefix(fields, "userEnteredFormat") {
			t.Errorf("case %d produced a field mask outside userEnteredFormat: %q", i, fields)
		}
		if request.RepeatCell.Cell.UserEnteredValue != nil {
			t.Errorf("case %d carried a cell value", i)
		}
	}
}

func TestBuildFormatRequestClear(t *testing.T) {
	request, fields, err := buildFormatRequest(&sheets.GridRange{}, formatArgs{Clear: true})
	if err != nil {
		t.Fatalf("buildFormatRequest returned an error: %v", err)
	}
	if fields != "userEnteredFormat" {
		t.Errorf("fields = %q, want userEnteredFormat", fields)
	}
	format := request.RepeatCell.Cell.UserEnteredFormat
	if format == nil {
		t.Fatal("the cell format should be an empty struct, not nil")
	}
	if *format != (sheets.CellFormat{}) {
		t.Errorf("the cell format should be empty, got %+v", format)
	}
}

// TestBuildFormatRequestBoldFalseIsSent verifies that turning bold off is not
// dropped as a zero value.
func TestBuildFormatRequestBoldFalseIsSent(t *testing.T) {
	request, fields, err := buildFormatRequest(&sheets.GridRange{}, formatArgs{
		TextFormat: &cellTextFormatArgs{Bold: ptr(false)},
	})
	if err != nil {
		t.Fatalf("buildFormatRequest returned an error: %v", err)
	}
	if fields != "userEnteredFormat.textFormat.bold" {
		t.Errorf("fields = %q, want userEnteredFormat.textFormat.bold", fields)
	}
	textFormat := request.RepeatCell.Cell.UserEnteredFormat.TextFormat
	if !contains(textFormat.ForceSendFields, "Bold") {
		t.Errorf("ForceSendFields should contain Bold, got %v", textFormat.ForceSendFields)
	}
}

func TestBuildFormatRequestValidation(t *testing.T) {
	tests := []struct {
		name string
		args formatArgs
	}{
		{"clear with another option", formatArgs{Clear: true, BackgroundColor: json.RawMessage(`"#FFF299"`)}},
		{"no options", formatArgs{}},
		{"empty text format", formatArgs{TextFormat: &cellTextFormatArgs{}}},
		{"unknown alignment", formatArgs{HorizontalAlignment: "JUSTIFY"}},
		{"unknown number format type", formatArgs{NumberFormat: &cellNumberFormatArgs{Type: "MONEY"}}},
		{"zero font size", formatArgs{TextFormat: &cellTextFormatArgs{FontSize: ptr(0.0)}}},
		{"negative font size", formatArgs{TextFormat: &cellTextFormatArgs{FontSize: ptr(-4.0)}}},
		{"fractional font size", formatArgs{TextFormat: &cellTextFormatArgs{FontSize: ptr(10.5)}}},
		{"bad color", formatArgs{BackgroundColor: json.RawMessage(`"not-a-color"`)}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, err := buildFormatRequest(&sheets.GridRange{}, tt.args); err == nil {
				t.Error("buildFormatRequest should have returned an error")
			}
		})
	}
}

// TestFormatCellsRejectsUnsafeRequests covers the last line of defence in the
// client, which runs before any API call is made.
func TestFormatCellsRejectsUnsafeRequests(t *testing.T) {
	client := &Client{}
	grid := &sheets.GridRange{}
	safeCell := &sheets.CellData{UserEnteredFormat: &sheets.CellFormat{}}

	valueCell := &sheets.CellData{
		UserEnteredFormat: &sheets.CellFormat{},
		UserEnteredValue:  &sheets.ExtendedValue{StringValue: ptr("overwritten")},
	}
	if err := client.FormatCells("sheet", grid, valueCell, "userEnteredFormat"); err == nil {
		t.Error("FormatCells should reject a cell carrying a value")
	}

	for _, fields := range []string{"", "values", "*", "userEnteredValue"} {
		if err := client.FormatCells("sheet", grid, safeCell, fields); err == nil {
			t.Errorf("FormatCells should reject the field mask %q", fields)
		}
	}
}
