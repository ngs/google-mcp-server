package sheets

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"google.golang.org/api/sheets/v4"
)

// cellTextFormatArgs holds the optional text style of sheets_cells_format.
// Every field is optional; a nil field is left untouched on the cells.
type cellTextFormatArgs struct {
	Bold            *bool           `json:"bold"`
	Italic          *bool           `json:"italic"`
	FontSize        *float64        `json:"font_size"`
	ForegroundColor json.RawMessage `json:"foreground_color"`
}

// cellNumberFormatArgs holds the optional number format of sheets_cells_format.
type cellNumberFormatArgs struct {
	Type    string `json:"type"`
	Pattern string `json:"pattern"`
}

// formatArgs holds the arguments of the sheets_cells_format tool.
type formatArgs struct {
	SpreadsheetID       string                `json:"spreadsheet_id"`
	Range               string                `json:"range"`
	BackgroundColor     json.RawMessage       `json:"background_color"`
	TextFormat          *cellTextFormatArgs   `json:"text_format"`
	HorizontalAlignment string                `json:"horizontal_alignment"`
	NumberFormat        *cellNumberFormatArgs `json:"number_format"`
	Clear               bool                  `json:"clear"`
}

// horizontalAlignments lists the alignment values the API accepts.
var horizontalAlignments = []string{"LEFT", "CENTER", "RIGHT"}

// numberFormatTypes lists the number format types the API accepts.
var numberFormatTypes = []string{"TEXT", "NUMBER", "PERCENT", "CURRENCY", "DATE", "TIME", "DATE_TIME", "SCIENTIFIC"}

// cellReferencePattern splits a single A1 cell reference into its column label
// and row number. Either part may be empty for whole-column or whole-row
// ranges, but never both.
var cellReferencePattern = regexp.MustCompile(`^([A-Za-z]*)([0-9]*)$`)

// hasRawValue reports whether an optional raw JSON argument was supplied with a
// value. An absent argument and an explicit null are both treated as unset.
func hasRawValue(raw json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(raw))
	return trimmed != "" && trimmed != "null"
}

// parseColor accepts "#FFF299", "FFF299", "#FC9", or an object such as
// {"red":1,"green":0.95,"blue":0.6}. Components are forced into the request so
// that a zero component, as in black, is not dropped as an empty value.
func parseColor(raw json.RawMessage) (*sheets.Color, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return nil, fmt.Errorf("invalid color: no value given")
	}

	switch trimmed[0] {
	case '"':
		var hex string
		if err := json.Unmarshal(raw, &hex); err != nil {
			return nil, fmt.Errorf("invalid color: %w", err)
		}
		return parseHexColor(hex)
	case '{':
		return parseColorObject(raw)
	default:
		return nil, fmt.Errorf("invalid color: %s (expected #RRGGBB or {red, green, blue})", trimmed)
	}
}

// parseHexColor converts "#RRGGBB", "RRGGBB" or the three digit "#RGB"
// shorthand into an API color.
func parseHexColor(hex string) (*sheets.Color, error) {
	digits := strings.TrimPrefix(strings.TrimSpace(hex), "#")

	switch len(digits) {
	case 3:
		// Expand the shorthand, "FC9" means "FFCC99"
		expanded := make([]byte, 0, 6)
		for i := 0; i < 3; i++ {
			expanded = append(expanded, digits[i], digits[i])
		}
		digits = string(expanded)
	case 6:
	default:
		return nil, fmt.Errorf("invalid color: %q (expected #RRGGBB or {red, green, blue})", hex)
	}

	components := make([]float64, 3)
	for i := range components {
		value, err := strconv.ParseUint(digits[i*2:i*2+2], 16, 8)
		if err != nil {
			return nil, fmt.Errorf("invalid color: %q (expected #RRGGBB or {red, green, blue})", hex)
		}
		components[i] = float64(value) / 255.0
	}

	return &sheets.Color{
		Red:   components[0],
		Green: components[1],
		Blue:  components[2],
		// A component of 0 is meaningful, so keep it in the request
		ForceSendFields: []string{"Red", "Green", "Blue"},
	}, nil
}

// parseColorObject converts {red, green, blue, alpha} into an API color. Every
// component is optional and defaults to 0.
func parseColorObject(raw json.RawMessage) (*sheets.Color, error) {
	var object struct {
		Red   *float64 `json:"red"`
		Green *float64 `json:"green"`
		Blue  *float64 `json:"blue"`
		Alpha *float64 `json:"alpha"`
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&object); err != nil {
		return nil, fmt.Errorf("invalid color: %w", err)
	}

	color := &sheets.Color{
		ForceSendFields: []string{"Red", "Green", "Blue"},
	}
	for _, component := range []struct {
		name  string
		value *float64
		into  *float64
	}{
		{"red", object.Red, &color.Red},
		{"green", object.Green, &color.Green},
		{"blue", object.Blue, &color.Blue},
		{"alpha", object.Alpha, &color.Alpha},
	} {
		if component.value == nil {
			continue
		}
		if *component.value < 0 || *component.value > 1 {
			return nil, fmt.Errorf("invalid color: %s must be between 0 and 1", component.name)
		}
		*component.into = *component.value
	}
	if object.Alpha != nil {
		color.ForceSendFields = append(color.ForceSendFields, "Alpha")
	}

	return color, nil
}

// columnLabelToIndex converts a column label into a zero-based index:
// "A" is 0, "Z" is 25 and "AA" is 26.
func columnLabelToIndex(label string) (int64, error) {
	if label == "" {
		return 0, fmt.Errorf("invalid column label: no value given")
	}

	var index int64
	for _, char := range label {
		switch {
		case char >= 'A' && char <= 'Z':
			index = index*26 + int64(char-'A') + 1
		case char >= 'a' && char <= 'z':
			index = index*26 + int64(char-'a') + 1
		default:
			return 0, fmt.Errorf("invalid column label: %q", label)
		}
	}

	return index - 1, nil
}

// foldFullWidthASCII maps full-width ASCII letters and digits to their
// half-width equivalents. It is applied only to the cell reference part of an
// A1 string, so sheet titles written in Japanese are left untouched.
func foldFullWidthASCII(s string) string {
	return strings.Map(func(char rune) rune {
		switch {
		case char >= 'Ａ' && char <= 'Ｚ':
			return 'A' + (char - 'Ａ')
		case char >= 'ａ' && char <= 'ｚ':
			return 'a' + (char - 'ａ')
		case char >= '０' && char <= '９':
			return '0' + (char - '０')
		default:
			return char
		}
	}, s)
}

// cellReference is one side of an A1 range, with the parts that were present.
type cellReference struct {
	column    int64
	hasColumn bool
	row       int64
	hasRow    bool
}

// parseCellReference splits a single A1 cell reference such as "B46", "B" or
// "46" into its zero-based column and row indices.
func parseCellReference(reference string) (cellReference, error) {
	matches := cellReferencePattern.FindStringSubmatch(reference)
	if matches == nil {
		return cellReference{}, fmt.Errorf("invalid cell reference: %q", reference)
	}

	label, digits := matches[1], matches[2]
	if label == "" && digits == "" {
		return cellReference{}, fmt.Errorf("invalid cell reference: %q", reference)
	}

	parsed := cellReference{}
	if label != "" {
		column, err := columnLabelToIndex(label)
		if err != nil {
			return cellReference{}, err
		}
		parsed.column = column
		parsed.hasColumn = true
	}
	if digits != "" {
		row, err := strconv.ParseInt(digits, 10, 64)
		if err != nil {
			return cellReference{}, fmt.Errorf("invalid row number: %q", digits)
		}
		if row < 1 {
			return cellReference{}, fmt.Errorf("invalid row number: %d (rows start at 1)", row)
		}
		parsed.row = row - 1
		parsed.hasRow = true
	}

	return parsed, nil
}

// unquoteSheetTitle removes the single quotes around a sheet title and unescapes
// the doubled quotes inside it.
func unquoteSheetTitle(title string) string {
	if len(title) >= 2 && strings.HasPrefix(title, "'") && strings.HasSuffix(title, "'") {
		return strings.ReplaceAll(title[1:len(title)-1], "''", "'")
	}
	return title
}

// parseA1Range splits an A1 notation string into a sheet title, which may be
// empty, and a GridRange whose sheet id is left unset. Bounds that the range
// does not constrain, as in a whole-column range, are left out of
// ForceSendFields so the API treats them as unbounded.
func parseA1Range(a1 string) (string, *sheets.GridRange, error) {
	title := ""
	reference := a1
	// A sheet title may itself contain "!", so split on the last one
	if at := strings.LastIndex(a1, "!"); at >= 0 {
		title = unquoteSheetTitle(a1[:at])
		reference = a1[at+1:]
	}

	// Only the reference is folded; folding the title would corrupt it
	reference = strings.TrimSpace(foldFullWidthASCII(reference))
	if reference == "" {
		return "", nil, fmt.Errorf("invalid range: %q (no cell reference)", a1)
	}

	sides := strings.Split(reference, ":")
	if len(sides) > 2 {
		return "", nil, fmt.Errorf("invalid range: %q (expected at most one ':')", a1)
	}
	if len(sides) == 1 {
		sides = append(sides, sides[0])
	}

	start, err := parseCellReference(sides[0])
	if err != nil {
		return "", nil, fmt.Errorf("invalid range: %q: %w", a1, err)
	}
	end, err := parseCellReference(sides[1])
	if err != nil {
		return "", nil, fmt.Errorf("invalid range: %q: %w", a1, err)
	}
	if start.hasColumn != end.hasColumn || start.hasRow != end.hasRow {
		return "", nil, fmt.Errorf("invalid range: %q (both ends must have the same shape)", a1)
	}

	grid := &sheets.GridRange{}
	if start.hasRow {
		first, last := orderIndices(start.row, end.row)
		grid.StartRowIndex = first
		grid.EndRowIndex = last + 1
		// Row 1 is index 0, which would otherwise be dropped
		grid.ForceSendFields = append(grid.ForceSendFields, "StartRowIndex")
	}
	if start.hasColumn {
		first, last := orderIndices(start.column, end.column)
		grid.StartColumnIndex = first
		grid.EndColumnIndex = last + 1
		// Column A is index 0, which would otherwise be dropped
		grid.ForceSendFields = append(grid.ForceSendFields, "StartColumnIndex")
	}

	return title, grid, nil
}

// orderIndices normalizes a range written backwards, such as "H51:B46".
func orderIndices(a, b int64) (int64, int64) {
	if a > b {
		return b, a
	}
	return a, b
}

// buildFormatRequest turns validated tool arguments into a repeatCell request
// and the field mask that limits what it touches. The mask never leaves
// userEnteredFormat, so cell values and formulas are not modified.
func buildFormatRequest(grid *sheets.GridRange, args formatArgs) (*sheets.Request, string, error) {
	hasFormatting := hasRawValue(args.BackgroundColor) ||
		args.TextFormat != nil ||
		args.HorizontalAlignment != "" ||
		args.NumberFormat != nil

	if args.Clear {
		if hasFormatting {
			return nil, "", fmt.Errorf("clear cannot be combined with other formatting options")
		}
		return repeatCellRequest(grid, &sheets.CellData{UserEnteredFormat: &sheets.CellFormat{}}, "userEnteredFormat"),
			"userEnteredFormat", nil
	}
	if !hasFormatting {
		return nil, "", fmt.Errorf("no formatting options specified")
	}

	format := &sheets.CellFormat{}
	var fields []string

	if hasRawValue(args.BackgroundColor) {
		color, err := parseColor(args.BackgroundColor)
		if err != nil {
			return nil, "", err
		}
		// backgroundColor is deprecated in favour of backgroundColorStyle
		format.BackgroundColorStyle = &sheets.ColorStyle{RgbColor: color}
		fields = append(fields, "userEnteredFormat.backgroundColorStyle")
	}

	if args.TextFormat != nil {
		textFields, err := applyTextFormat(format, args.TextFormat)
		if err != nil {
			return nil, "", err
		}
		fields = append(fields, textFields...)
	}

	if args.HorizontalAlignment != "" {
		if !isOneOf(args.HorizontalAlignment, horizontalAlignments) {
			return nil, "", fmt.Errorf("invalid horizontal_alignment: %q (must be one of %s)",
				args.HorizontalAlignment, strings.Join(horizontalAlignments, ", "))
		}
		format.HorizontalAlignment = args.HorizontalAlignment
		fields = append(fields, "userEnteredFormat.horizontalAlignment")
	}

	if args.NumberFormat != nil {
		if !isOneOf(args.NumberFormat.Type, numberFormatTypes) {
			return nil, "", fmt.Errorf("invalid number_format.type: %q (must be one of %s)",
				args.NumberFormat.Type, strings.Join(numberFormatTypes, ", "))
		}
		format.NumberFormat = &sheets.NumberFormat{
			Type:    args.NumberFormat.Type,
			Pattern: args.NumberFormat.Pattern,
		}
		fields = append(fields, "userEnteredFormat.numberFormat")
	}

	if len(fields) == 0 {
		return nil, "", fmt.Errorf("no formatting options specified")
	}

	mask := strings.Join(fields, ",")
	return repeatCellRequest(grid, &sheets.CellData{UserEnteredFormat: format}, mask), mask, nil
}

// applyTextFormat fills in the text style and returns the fields it set.
func applyTextFormat(format *sheets.CellFormat, args *cellTextFormatArgs) ([]string, error) {
	textFormat := &sheets.TextFormat{}
	var fields []string

	if args.Bold != nil {
		textFormat.Bold = *args.Bold
		// bold: false must reach the API to clear an existing bold style
		textFormat.ForceSendFields = append(textFormat.ForceSendFields, "Bold")
		fields = append(fields, "userEnteredFormat.textFormat.bold")
	}
	if args.Italic != nil {
		textFormat.Italic = *args.Italic
		textFormat.ForceSendFields = append(textFormat.ForceSendFields, "Italic")
		fields = append(fields, "userEnteredFormat.textFormat.italic")
	}
	if args.FontSize != nil {
		size := *args.FontSize
		if size <= 0 || size != float64(int64(size)) {
			return nil, fmt.Errorf("invalid text_format.font_size: %v (must be a positive whole number)", size)
		}
		textFormat.FontSize = int64(size)
		fields = append(fields, "userEnteredFormat.textFormat.fontSize")
	}
	if hasRawValue(args.ForegroundColor) {
		color, err := parseColor(args.ForegroundColor)
		if err != nil {
			return nil, err
		}
		// foregroundColor is deprecated in favour of foregroundColorStyle
		textFormat.ForegroundColorStyle = &sheets.ColorStyle{RgbColor: color}
		fields = append(fields, "userEnteredFormat.textFormat.foregroundColorStyle")
	}

	if len(fields) == 0 {
		return nil, fmt.Errorf("no formatting options specified")
	}

	format.TextFormat = textFormat
	return fields, nil
}

// repeatCellRequest wraps a cell and its field mask into a repeatCell request.
func repeatCellRequest(grid *sheets.GridRange, cell *sheets.CellData, fields string) *sheets.Request {
	return &sheets.Request{
		RepeatCell: &sheets.RepeatCellRequest{
			Range:  grid,
			Cell:   cell,
			Fields: fields,
		},
	}
}

// isOneOf reports whether value is present in allowed.
func isOneOf(value string, allowed []string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

// formatGridRange renders a grid range for tool responses. Bounds the range
// leaves open are reported as null rather than as a zero index.
func formatGridRange(grid *sheets.GridRange) map[string]interface{} {
	forced := make(map[string]bool, len(grid.ForceSendFields))
	for _, name := range grid.ForceSendFields {
		forced[name] = true
	}

	rendered := map[string]interface{}{
		"startRowIndex":    nil,
		"endRowIndex":      nil,
		"startColumnIndex": nil,
		"endColumnIndex":   nil,
	}
	if forced["StartRowIndex"] {
		rendered["startRowIndex"] = grid.StartRowIndex
		rendered["endRowIndex"] = grid.EndRowIndex
	}
	if forced["StartColumnIndex"] {
		rendered["startColumnIndex"] = grid.StartColumnIndex
		rendered["endColumnIndex"] = grid.EndColumnIndex
	}
	return rendered
}
