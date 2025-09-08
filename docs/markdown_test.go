package docs

import (
	"testing"

	"google.golang.org/api/docs/v1"
)

func TestMarkdownConverter_ConvertToRequests(t *testing.T) {
	tests := []struct {
		name      string
		markdown  string
		startIdx  int64
		wantTypes []string // Types of requests we expect to see
	}{
		{
			name:      "Simple heading",
			markdown:  "# Hello World",
			startIdx:  1,
			wantTypes: []string{"InsertText", "UpdateParagraphStyle", "InsertText"},
		},
		{
			name:      "Multiple headings",
			markdown:  "# Title\n## Subtitle",
			startIdx:  1,
			wantTypes: []string{"InsertText", "UpdateParagraphStyle", "InsertText", "InsertText", "UpdateParagraphStyle", "InsertText"},
		},
		{
			name:      "Bold text",
			markdown:  "This is **bold** text",
			startIdx:  1,
			wantTypes: []string{"InsertText", "UpdateTextStyle"},
		},
		{
			name:      "Italic text",
			markdown:  "This is *italic* text",
			startIdx:  1,
			wantTypes: []string{"InsertText", "UpdateTextStyle"},
		},
		{
			name:      "Code text",
			markdown:  "This is `code` text",
			startIdx:  1,
			wantTypes: []string{"InsertText", "UpdateTextStyle"},
		},
		{
			name:      "Bullet list",
			markdown:  "- Item 1\n- Item 2",
			startIdx:  1,
			wantTypes: []string{"InsertText", "CreateParagraphBullets", "InsertText", "InsertText", "CreateParagraphBullets"},
		},
		{
			name:      "Numbered list",
			markdown:  "1. First item\n2. Second item",
			startIdx:  1,
			wantTypes: []string{"InsertText", "CreateParagraphBullets", "InsertText", "InsertText", "CreateParagraphBullets"},
		},
		{
			name:      "Blockquote",
			markdown:  "> This is a quote",
			startIdx:  1,
			wantTypes: []string{"InsertText", "UpdateParagraphStyle", "UpdateTextStyle", "InsertText"},
		},
		{
			name:      "Link",
			markdown:  "[Google](https://google.com)",
			startIdx:  1,
			wantTypes: []string{"InsertText", "UpdateTextStyle"},
		},
		{
			name:      "Mixed formatting",
			markdown:  "# Title\nThis is **bold** and *italic* text.\n- Bullet point\n> Quote",
			startIdx:  1,
			wantTypes: []string{"InsertText", "UpdateParagraphStyle", "InsertText", "InsertText", "DeleteParagraphBullets", "UpdateParagraphStyle", "UpdateTextStyle", "UpdateTextStyle", "InsertText", "InsertText", "CreateParagraphBullets", "InsertText", "UpdateParagraphStyle", "UpdateTextStyle", "InsertText"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converter := NewMarkdownConverter(tt.startIdx)
			requests := converter.ConvertToRequests(tt.markdown)

			// Check that we got the expected number of requests
			if len(requests) != len(tt.wantTypes) {
				t.Errorf("ConvertToRequests() got %d requests, want %d", len(requests), len(tt.wantTypes))
				return
			}

			// Check that each request is of the expected type
			for i, request := range requests {
				requestType := getRequestType(request)
				if requestType != tt.wantTypes[i] {
					t.Errorf("Request %d: got type %s, want %s", i, requestType, tt.wantTypes[i])
				}
			}
		})
	}
}

func TestMarkdownConverter_createHeadingRequests(t *testing.T) {
	tests := []struct {
		name      string
		line      string
		level     int
		startIdx  int64
		wantStyle string
	}{
		{
			name:      "H1 heading",
			line:      "# Main Title",
			level:     1,
			startIdx:  1,
			wantStyle: "HEADING_1",
		},
		{
			name:      "H2 heading",
			line:      "## Subtitle",
			level:     2,
			startIdx:  10,
			wantStyle: "HEADING_2",
		},
		{
			name:      "H6 heading",
			line:      "###### Small heading",
			level:     6,
			startIdx:  1,
			wantStyle: "HEADING_6",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converter := NewMarkdownConverter(tt.startIdx)
			requests := converter.createHeadingRequests(tt.line, tt.level)

			if len(requests) != 3 {
				t.Fatalf("Expected 3 requests, got %d", len(requests))
			}

			// First request should be InsertText
			if requests[0].InsertText == nil {
				t.Error("First request should be InsertText")
			}

			// Second request should be UpdateParagraphStyle with correct heading style
			if requests[1].UpdateParagraphStyle == nil {
				t.Error("Second request should be UpdateParagraphStyle")
			} else if requests[1].UpdateParagraphStyle.ParagraphStyle.NamedStyleType != tt.wantStyle {
				t.Errorf("Got style %s, want %s", requests[1].UpdateParagraphStyle.ParagraphStyle.NamedStyleType, tt.wantStyle)
			}
			
			// Third request should be InsertText for newline
			if requests[2].InsertText == nil {
				t.Error("Third request should be InsertText for newline")
			}
		})
	}
}

func TestMarkdownConverter_createBulletListRequests(t *testing.T) {
	tests := []struct {
		name         string
		line         string
		startIdx     int64
		expectNewline bool
	}{
		{
			name:         "Dash bullet",
			line:         "- Bullet item",
			startIdx:     1,
			expectNewline: false, // First item in document
		},
		{
			name:         "Asterisk bullet",
			line:         "* Bullet item",
			startIdx:     5,
			expectNewline: true, // Not first item
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converter := NewMarkdownConverter(tt.startIdx)
			requests := converter.createBulletListRequests(tt.line)

			expectedRequests := 2
			if tt.expectNewline {
				expectedRequests = 3
			}

			if len(requests) != expectedRequests {
				t.Fatalf("Expected %d requests, got %d", expectedRequests, len(requests))
			}

			reqIdx := 0
			// If we expect a newline, first request should be InsertText with "\n"
			if tt.expectNewline {
				if requests[0].InsertText == nil || requests[0].InsertText.Text != "\n" {
					t.Error("First request should be InsertText with newline")
				}
				reqIdx = 1
			}

			// Next request should be InsertText with the list content
			if requests[reqIdx].InsertText == nil {
				t.Errorf("Request %d should be InsertText", reqIdx)
			}

			// Last request should be CreateParagraphBullets
			if requests[reqIdx+1].CreateParagraphBullets == nil {
				t.Errorf("Request %d should be CreateParagraphBullets", reqIdx+1)
			} else if requests[reqIdx+1].CreateParagraphBullets.BulletPreset != "BULLET_DISC_CIRCLE_SQUARE" {
				t.Errorf("Got bullet preset %s, want BULLET_DISC_CIRCLE_SQUARE", requests[reqIdx+1].CreateParagraphBullets.BulletPreset)
			}
		})
	}
}

func TestMarkdownConverter_createNumberedListRequests(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		startIdx int64
	}{
		{
			name:     "Single digit",
			line:     "1. First item",
			startIdx: 1,
		},
		{
			name:     "Double digit",
			line:     "12. Twelfth item",
			startIdx: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converter := NewMarkdownConverter(tt.startIdx)
			requests := converter.createNumberedListRequests(tt.line)

			if len(requests) != 2 {
				t.Fatalf("Expected 2 requests, got %d", len(requests))
			}

			// First request should be InsertText
			if requests[0].InsertText == nil {
				t.Error("First request should be InsertText")
			}

			// Second request should be CreateParagraphBullets
			if requests[1].CreateParagraphBullets == nil {
				t.Error("Second request should be CreateParagraphBullets")
			} else if requests[1].CreateParagraphBullets.BulletPreset != "NUMBERED_DECIMAL_ALPHA_ROMAN" {
				t.Errorf("Got bullet preset %s, want NUMBERED_DECIMAL_ALPHA_ROMAN", requests[1].CreateParagraphBullets.BulletPreset)
			}
		})
	}
}

func TestMarkdownConverter_processInlineFormatting(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		startIdx     int64
		wantText     string
		wantRequests int
	}{
		{
			name:         "Bold text",
			input:        "This is **bold** text",
			startIdx:     1,
			wantText:     "This is bold text",
			wantRequests: 1,
		},
		{
			name:         "Italic text",
			input:        "This is *italic* text",
			startIdx:     1,
			wantText:     "This is italic text",
			wantRequests: 1,
		},
		{
			name:         "Code text",
			input:        "This is `code` text",
			startIdx:     1,
			wantText:     "This is code text",
			wantRequests: 1,
		},
		{
			name:         "Link text",
			input:        "Visit [Google](https://google.com) please",
			startIdx:     1,
			wantText:     "Visit Google please",
			wantRequests: 1,
		},
		{
			name:         "Multiple formatting",
			input:        "**Bold** and *italic* and `code`",
			startIdx:     1,
			wantText:     "Bold and italic and code",
			wantRequests: 3,
		},
		{
			name:         "No formatting",
			input:        "Plain text with no formatting",
			startIdx:     1,
			wantText:     "Plain text with no formatting",
			wantRequests: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converter := NewMarkdownConverter(tt.startIdx)
			cleanText, requests := converter.processInlineFormatting(tt.input)

			if cleanText != tt.wantText {
				t.Errorf("processInlineFormatting() cleanText = %q, want %q", cleanText, tt.wantText)
			}

			if len(requests) != tt.wantRequests {
				t.Errorf("processInlineFormatting() got %d requests, want %d", len(requests), tt.wantRequests)
			}
		})
	}
}

func TestMarkdownConverter_createBlockquoteRequests(t *testing.T) {
	converter := NewMarkdownConverter(1)
	requests := converter.createBlockquoteRequests("> This is a quote")

	if len(requests) != 4 {
		t.Fatalf("Expected 4 requests, got %d", len(requests))
	}

	// First request should be InsertText
	if requests[0].InsertText == nil {
		t.Error("First request should be InsertText")
	}

	// Second request should be UpdateParagraphStyle for indentation
	if requests[1].UpdateParagraphStyle == nil {
		t.Error("Second request should be UpdateParagraphStyle")
	}

	// Third request should be UpdateTextStyle for italic
	if requests[2].UpdateTextStyle == nil {
		t.Error("Third request should be UpdateTextStyle")
	} else if !requests[2].UpdateTextStyle.TextStyle.Italic {
		t.Error("Text should be italic in blockquote")
	}

	// Fourth request should be InsertText for newline
	if requests[3].InsertText == nil {
		t.Error("Fourth request should be InsertText for newline")
	}
}

func TestMarkdownConverter_createCodeBlockRequests(t *testing.T) {
	converter := NewMarkdownConverter(1)
	requests := converter.createCodeBlockRequests("```python")

	if len(requests) != 2 {
		t.Fatalf("Expected 2 requests, got %d", len(requests))
	}

	// First request should be InsertText
	if requests[0].InsertText == nil {
		t.Error("First request should be InsertText")
	}

	// Second request should be UpdateTextStyle for monospace font
	if requests[1].UpdateTextStyle == nil {
		t.Error("Second request should be UpdateTextStyle")
	} else if requests[1].UpdateTextStyle.TextStyle.WeightedFontFamily.FontFamily != "Consolas" {
		t.Errorf("Font family should be Consolas, got %s", requests[1].UpdateTextStyle.TextStyle.WeightedFontFamily.FontFamily)
	}
}

// Helper function to determine the type of a docs.Request
func getRequestType(request *docs.Request) string {
	switch {
	case request.InsertText != nil:
		return "InsertText"
	case request.UpdateTextStyle != nil:
		return "UpdateTextStyle"
	case request.UpdateParagraphStyle != nil:
		return "UpdateParagraphStyle"
	case request.CreateParagraphBullets != nil:
		return "CreateParagraphBullets"
	case request.DeleteParagraphBullets != nil:
		return "DeleteParagraphBullets"
	case request.DeleteContentRange != nil:
		return "DeleteContentRange"
	default:
		return "Unknown"
	}
}

// Benchmark tests
func BenchmarkMarkdownConverter_ConvertToRequests(b *testing.B) {
	markdown := `# Large Document

## Introduction
This is a **large** document with *various* formatting.

### Features
- Bullet points
- More bullets
- Even more bullets

1. Numbered lists
2. Are also supported
3. With multiple items

> Blockquotes work too
> Multiple lines supported

Some ` + "`code`" + ` and [links](https://example.com) as well.

#### Code Blocks
` + "```go" + `
func main() {
    fmt.Println("Hello, World!")
}
` + "```"

	converter := NewMarkdownConverter(1)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = converter.ConvertToRequests(markdown)
		converter.currentIndex = 1 // Reset for next iteration
	}
}

func BenchmarkMarkdownConverter_processInlineFormatting(b *testing.B) {
	text := "This has **bold**, *italic*, `code`, and [links](https://example.com) formatting."
	converter := NewMarkdownConverter(1)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = converter.processInlineFormatting(text)
	}
}

// Test edge cases
func TestMarkdownConverter_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		markdown string
		startIdx int64
	}{
		{
			name:     "Empty string",
			markdown: "",
			startIdx: 1,
		},
		{
			name:     "Only whitespace",
			markdown: "   \n  \n  ",
			startIdx: 1,
		},
		{
			name:     "Malformed markdown",
			markdown: "**unclosed bold",
			startIdx: 1,
		},
		{
			name:     "Nested formatting",
			markdown: "**bold *and italic* text**",
			startIdx: 1,
		},
		{
			name:     "Multiple consecutive formatting",
			markdown: "**bold****more bold**",
			startIdx: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converter := NewMarkdownConverter(tt.startIdx)
			// Should not panic
			requests := converter.ConvertToRequests(tt.markdown)

			// Should return valid requests (even if empty)
			if requests == nil {
				t.Error("ConvertToRequests should not return nil")
			}

			// All requests should be valid (non-nil)
			for i, req := range requests {
				if req == nil {
					t.Errorf("Request %d should not be nil", i)
				}
			}
		})
	}
}

// Test that index tracking works correctly
func TestMarkdownConverter_IndexTracking(t *testing.T) {
	converter := NewMarkdownConverter(10)

	// Process some text
	requests := converter.ConvertToRequests("Hello **world**")

	// Check that requests use the correct starting index
	if len(requests) > 0 && requests[0].InsertText != nil {
		if requests[0].InsertText.Location.Index != 10 {
			t.Errorf("Expected index 10, got %d", requests[0].InsertText.Location.Index)
		}
	}

	// Check that currentIndex was updated
	if converter.currentIndex <= 10 {
		t.Error("currentIndex should have been updated after processing text")
	}
}
