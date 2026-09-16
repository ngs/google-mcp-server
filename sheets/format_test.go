package sheets

import (
	"encoding/json"
	"math"
	"reflect"
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
		// A bare sheet title is A1 notation for the whole sheet, so it must not
		// be parsed as a cell reference such as column SHEET row 1
		{"bare sheet title", "Sheet1", "Sheet1", unset(), unset(), unset(), unset()},
		{"bare quoted title", "'Sheet 1'", "Sheet 1", unset(), unset(), unset(), unset()},
		{"bare japanese title", "日本語シート", "日本語シート", unset(), unset(), unset(), unset()},
		{"bare title with escaped quote", "'It''s a sheet'", "It's a sheet", unset(), unset(), unset(), unset()},
		// Spaces inside the quotes are part of the name and must survive
		{"bare quoted title with edge spaces", "' Budget '", " Budget ", unset(), unset(), unset(), unset()},
		{"qualified quoted title with edge spaces", "' Budget '!A1", " Budget ", idx(0), idx(1), idx(0), idx(1)},
		{"three letter column still parses", "Sheet1!ZZZ1", "Sheet1", idx(0), idx(1), idx(18277), idx(18278)},
		// A "!" inside a quoted title is part of the name, not the separator
		{"bare quoted title containing a bang", "'Data!Sheet'", "Data!Sheet", unset(), unset(), unset(), unset()},
		{"quoted title containing a bang", "'Data!Sheet'!A1", "Data!Sheet", idx(0), idx(1), idx(0), idx(1)},
		{"quoted title with bang and escaped quote", "'It''s!Here'!B2", "It's!Here", idx(1), idx(2), idx(1), idx(2)},
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
		// An empty sheet qualifier is a typo, not a request for the first sheet
		"!A1",
		"''!A1",
		"  !A1",
		// An empty quoted title on its own would otherwise select the whole
		// first sheet, which is a destructive thing to do for a typo
		"''",
		"'  '",
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

	// Sheets stops at column ZZZ, so a longer run of letters is a sheet title
	for _, label := range []string{"", "A1", "AAAA", "Sheet"} {
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
		// Sheets does not generally honour alpha in a colour style, so the
		// input does not pretend to accept it
		`{"red":1,"alpha":0.5}`,
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
	if !reflect.DeepEqual(format, &sheets.CellFormat{}) {
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

	// A prefix check alone would let a second path ride along in the mask and
	// clear the values that repeatCell also applies it to
	for _, fields := range []string{
		"",
		"values",
		"*",
		"userEnteredValue",
		"userEnteredFormat,userEnteredValue",
		"userEnteredFormat.backgroundColorStyle,userEnteredValue",
		"userEnteredFormatButNotReally",
		"userEnteredFormat,",
	} {
		if err := client.FormatCells("sheet", grid, safeCell, fields); err == nil {
			t.Errorf("FormatCells should reject the field mask %q", fields)
		}
	}
}

func TestColorStyleToHex(t *testing.T) {
	tests := []struct {
		name  string
		input json.RawMessage
		want  string
	}{
		{"light yellow", json.RawMessage(`"#FFF299"`), "#FFF299"},
		{"black", json.RawMessage(`"#000000"`), "#000000"},
		{"white", json.RawMessage(`"#FFFFFF"`), "#FFFFFF"},
		{"shorthand expands", json.RawMessage(`"#FC9"`), "#FFCC99"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			color, err := parseColor(tt.input)
			if err != nil {
				t.Fatalf("parseColor returned an error: %v", err)
			}
			if got := colorStyleToHex(&sheets.ColorStyle{RgbColor: color}); got != tt.want {
				t.Errorf("colorStyleToHex = %q, want %q", got, tt.want)
			}
		})
	}

	if got := colorStyleToHex(nil); got != "" {
		t.Errorf("colorStyleToHex(nil) = %q, want an empty string", got)
	}
	if got := colorStyleToHex(&sheets.ColorStyle{ThemeColor: "ACCENT1"}); got != "" {
		t.Errorf("a theme color has no rgb value, got %q", got)
	}
}

// TestSummarizeCellFormatOmitsUnsetFields keeps the read response small: a cell
// with no styling of its own reports nothing rather than a row of defaults.
func TestSummarizeCellFormatOmitsUnsetFields(t *testing.T) {
	if got := summarizeCellFormat(nil); len(got) != 0 {
		t.Errorf("a nil format should summarize to nothing, got %v", got)
	}
	if got := summarizeCellFormat(&sheets.CellFormat{}); len(got) != 0 {
		t.Errorf("an empty format should summarize to nothing, got %v", got)
	}

	color, err := parseColor(json.RawMessage(`"#FFF299"`))
	if err != nil {
		t.Fatalf("parseColor returned an error: %v", err)
	}
	got := summarizeCellFormat(&sheets.CellFormat{
		BackgroundColorStyle: &sheets.ColorStyle{RgbColor: color},
		HorizontalAlignment:  "CENTER",
		TextFormat:           &sheets.TextFormat{Bold: true, FontSize: 12},
		NumberFormat:         &sheets.NumberFormat{Type: "CURRENCY", Pattern: "#,##0"},
	})

	if got["backgroundColor"] != "#FFF299" {
		t.Errorf("backgroundColor = %v, want #FFF299", got["backgroundColor"])
	}
	if got["horizontalAlignment"] != "CENTER" {
		t.Errorf("horizontalAlignment = %v, want CENTER", got["horizontalAlignment"])
	}
	if got["bold"] != true {
		t.Errorf("bold = %v, want true", got["bold"])
	}
	if got["fontSize"] != int64(12) {
		t.Errorf("fontSize = %v, want 12", got["fontSize"])
	}
	numberFormat, ok := got["numberFormat"].(map[string]interface{})
	if !ok {
		t.Fatalf("numberFormat = %v, want a map", got["numberFormat"])
	}
	if numberFormat["type"] != "CURRENCY" || numberFormat["pattern"] != "#,##0" {
		t.Errorf("unexpected number format: %v", numberFormat)
	}
	// italic was never set, so it must not appear at all
	if _, ok := got["italic"]; ok {
		t.Errorf("italic should be absent, got %v", got)
	}
}

// TestReadFormatFieldMaskStaysOnFormatting is the read-side counterpart of the
// write guarantee: the request must not ask for cell values.
func TestReadFormatFieldMaskStaysOnFormatting(t *testing.T) {
	for _, forbidden := range []string{"userEnteredValue", "formattedValue", "effectiveValue", "values.userEntered"} {
		if strings.Contains(readFormatFieldMask, forbidden) {
			t.Errorf("the read field mask should not request %s, got %q", forbidden, readFormatFieldMask)
		}
	}
	if !strings.Contains(readFormatFieldMask, "effectiveFormat") {
		t.Errorf("the read field mask should request effectiveFormat, got %q", readFormatFieldMask)
	}
}

// TestBuildFormatRequestRunsBeforeSheetResolution documents that argument
// validation does not depend on any API call, so a bad argument is reported as
// such instead of surfacing as a network or OAuth error from resolving a sheet.
func TestBuildFormatRequestRunsBeforeSheetResolution(t *testing.T) {
	// A grid range whose sheet has not been resolved yet must still validate
	grid := &sheets.GridRange{}

	if _, _, err := buildFormatRequest(grid, formatArgs{BackgroundColor: json.RawMessage(`"nope"`)}); err == nil {
		t.Error("a bad color should be rejected without resolving the sheet")
	}
	if _, _, err := buildFormatRequest(grid, formatArgs{}); err == nil {
		t.Error("missing options should be rejected without resolving the sheet")
	}
	if grid.SheetId != 0 || len(grid.ForceSendFields) != 0 {
		t.Errorf("building the request should not touch the grid range, got %+v", grid)
	}

	// The handler sets the sheet id on this same pointer afterwards, so a
	// request built first still ends up carrying it
	request, _, err := buildFormatRequest(grid, formatArgs{BackgroundColor: json.RawMessage(`"#FFF299"`)})
	if err != nil {
		t.Fatalf("buildFormatRequest returned an error: %v", err)
	}
	grid.SheetId = 42
	if request.RepeatCell.Range.SheetId != 42 {
		t.Errorf("the request should share the grid range pointer, got %d", request.RepeatCell.Range.SheetId)
	}
}

func TestCountGridCells(t *testing.T) {
	tests := []struct {
		name  string
		input string
		cells int64
		known bool
	}{
		{"block", "Sheet1!B46:H51", 42, true},
		{"single cell", "Sheet1!G52", 1, true},
		{"single row", "Sheet1!B61:H61", 7, true},
		{"whole columns", "Sheet1!A:C", 0, false},
		{"whole rows", "Sheet1!2:5", 0, false},
		{"whole sheet", "Sheet1", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, grid, err := parseA1Range(tt.input)
			if err != nil {
				t.Fatalf("parseA1Range returned an error: %v", err)
			}
			cells, known := countGridCells(grid)
			if known != tt.known {
				t.Fatalf("known = %v, want %v", known, tt.known)
			}
			if known && cells != tt.cells {
				t.Errorf("cells = %d, want %d", cells, tt.cells)
			}
		})
	}
}

// TestReadFormatFieldMaskUsesNestedSelectors checks the partial response
// selector is written in the nested form the API accepts.
func TestReadFormatFieldMaskUsesNestedSelectors(t *testing.T) {
	if strings.Contains(readFormatFieldMask, ".") {
		t.Errorf("the selector should use nested parentheses, not dotted paths: %q", readFormatFieldMask)
	}
	for _, want := range []string{"sheets(", "properties(", "data(", "rowData(", "values(", "effectiveFormat"} {
		if !strings.Contains(readFormatFieldMask, want) {
			t.Errorf("the selector should contain %q, got %q", want, readFormatFieldMask)
		}
	}
}

// TestSummarizeCellFormatOmitsEmptyPattern keeps the promise that unset
// properties are absent: a number format with only a type must not report an
// empty pattern.
func TestSummarizeCellFormatOmitsEmptyPattern(t *testing.T) {
	got := summarizeCellFormat(&sheets.CellFormat{
		NumberFormat: &sheets.NumberFormat{Type: "PERCENT"},
	})
	numberFormat, ok := got["numberFormat"].(map[string]interface{})
	if !ok {
		t.Fatalf("numberFormat = %v, want a map", got["numberFormat"])
	}
	if numberFormat["type"] != "PERCENT" {
		t.Errorf("type = %v, want PERCENT", numberFormat["type"])
	}
	if _, ok := numberFormat["pattern"]; ok {
		t.Errorf("an empty pattern should be omitted, got %v", numberFormat)
	}

	// A pattern with no type is still reported
	got = summarizeCellFormat(&sheets.CellFormat{
		NumberFormat: &sheets.NumberFormat{Pattern: "#,##0"},
	})
	numberFormat, ok = got["numberFormat"].(map[string]interface{})
	if !ok {
		t.Fatalf("numberFormat = %v, want a map", got["numberFormat"])
	}
	if _, ok := numberFormat["type"]; ok {
		t.Errorf("an empty type should be omitted, got %v", numberFormat)
	}
	if numberFormat["pattern"] != "#,##0" {
		t.Errorf("pattern = %v, want #,##0", numberFormat["pattern"])
	}
}

// TestCountGridCellsSaturates guards the size check against an overflow that
// would wrap a huge range around to a small number.
func TestCountGridCellsSaturates(t *testing.T) {
	grid := &sheets.GridRange{
		StartRowIndex:    0,
		EndRowIndex:      1 << 50,
		StartColumnIndex: 0,
		EndColumnIndex:   1 << 14,
		ForceSendFields:  []string{"StartRowIndex", "StartColumnIndex"},
	}

	cells, known := countGridCells(grid)
	if !known {
		t.Fatal("a bounded range should report a known size")
	}
	if cells <= maxReadFormatCells {
		t.Errorf("an enormous range reported %d cells, which would pass the limit", cells)
	}
}

func TestIndexToColumnLabel(t *testing.T) {
	tests := map[int64]string{0: "A", 25: "Z", 26: "AA", 51: "AZ", 52: "BA", 701: "ZZ", 18277: "ZZZ"}

	for index, want := range tests {
		if got := indexToColumnLabel(index); got != want {
			t.Errorf("indexToColumnLabel(%d) = %q, want %q", index, got, want)
		}
	}

	// Every label must survive the round trip back to its index
	for index := range tests {
		got, err := columnLabelToIndex(indexToColumnLabel(index))
		if err != nil {
			t.Errorf("columnLabelToIndex returned an error for index %d: %v", index, err)
			continue
		}
		if got != index {
			t.Errorf("round trip of %d gave %d", index, got)
		}
	}
}

// TestGridRangeToA1 covers the canonical range the read path sends to the API,
// so it reads exactly the range the parser accepted.
func TestGridRangeToA1(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"block", "Sheet1!B46:H51", "'Sheet1'!B46:H51"},
		{"no title", "B46:H51", "B46:H51"},
		{"single cell", "Sheet1!G52", "'Sheet1'!G52:G52"},
		{"quote is doubled", "'It''s a sheet'!A1", "'It''s a sheet'!A1:A1"},
		{"full width folded to ascii", "Sheet1!Ｂ４６:Ｈ５１", "'Sheet1'!B46:H51"},
		{"title containing a bang", "'Data!Sheet'!A1", "'Data!Sheet'!A1:A1"},
		{"japanese title", "日本語シート!B2", "'日本語シート'!B2:B2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			title, grid, err := parseA1Range(tt.input)
			if err != nil {
				t.Fatalf("parseA1Range returned an error: %v", err)
			}
			if got := gridRangeToA1(title, grid); got != tt.want {
				t.Errorf("gridRangeToA1 = %q, want %q", got, tt.want)
			}
		})
	}
}
