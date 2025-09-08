package docs

import (
	"fmt"
	"strings"
	"testing"

	"google.golang.org/api/docs/v1"
)

func TestConvertMarkdownToHTML(t *testing.T) {
	tests := []struct {
		name     string
		markdown string
		contains []string // HTML fragments that should be present
	}{
		{
			name:     "Simple text",
			markdown: "Hello World",
			contains: []string{"<p>Hello World</p>"},
		},
		{
			name:     "Headers",
			markdown: "# H1\n## H2\n### H3\n#### H4\n##### H5\n###### H6",
			contains: []string{
				"<h1",
				"H1</h1>",
				"<h2",
				"H2</h2>",
				"<h3",
				"H3</h3>",
				"<h4",
				"H4</h4>",
				"<h5",
				"H5</h5>",
				"<h6",
				"H6</h6>",
			},
		},
		{
			name:     "Bold text",
			markdown: "This is **bold** text",
			contains: []string{"<strong>bold</strong>"},
		},
		{
			name:     "Italic text",
			markdown: "This is *italic* text",
			contains: []string{"<em>italic</em>"},
		},
		{
			name:     "Inline code",
			markdown: "Use `code` for inline code",
			contains: []string{"<code>code</code>"},
		},
		{
			name:     "Code block",
			markdown: "```\ncode block\n```",
			contains: []string{"<pre>", "<code>", "code block"},
		},
		{
			name:     "Unordered list",
			markdown: "- Item 1\n- Item 2\n- Item 3",
			contains: []string{"<ul>", "<li>Item 1</li>", "<li>Item 2</li>", "<li>Item 3</li>"},
		},
		{
			name:     "Ordered list",
			markdown: "1. First\n2. Second\n3. Third",
			contains: []string{"<ol>", "<li>First</li>", "<li>Second</li>", "<li>Third</li>"},
		},
		{
			name:     "Blockquote",
			markdown: "> This is a quote",
			contains: []string{"<blockquote>", "This is a quote"},
		},
		{
			name:     "Link",
			markdown: "[Google](https://google.com)",
			contains: []string{`<a href="https://google.com">Google</a>`},
		},
		{
			name:     "Mixed formatting",
			markdown: "# Title\n\nThis has **bold** and *italic* and `code`.\n\n- List item",
			contains: []string{
				"<h1",
				"Title</h1>",
				"<strong>bold</strong>",
				"<em>italic</em>",
				"<code>code</code>",
				"<li>List item</li>",
			},
		},
	}

	converter := &MarkdownConverter{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			html := converter.ConvertMarkdownToHTML(tt.markdown)

			for _, fragment := range tt.contains {
				if !strings.Contains(html, fragment) {
					t.Errorf("HTML output missing expected fragment: %s\nGot: %s", fragment, html)
				}
			}
		})
	}
}

func TestParseHTML(t *testing.T) {
	tests := []struct {
		name       string
		html       string
		validateFn func(*HTMLNode) error
	}{
		{
			name: "Simple paragraph",
			html: "<p>Hello World</p>",
			validateFn: func(node *HTMLNode) error {
				if node == nil {
					return fmt.Errorf("node is nil")
				}
				// Find the p tag
				var pNode *HTMLNode
				for _, child := range node.Children {
					if child.Tag == "p" {
						pNode = child
						break
					}
				}
				if pNode == nil {
					return fmt.Errorf("no p tag found")
				}
				if len(pNode.Children) != 1 {
					return fmt.Errorf("expected 1 child, got %d", len(pNode.Children))
				}
				if pNode.Children[0].Text != "Hello World" {
					return fmt.Errorf("expected text 'Hello World', got '%s'", pNode.Children[0].Text)
				}
				return nil
			},
		},
		{
			name: "Nested formatting",
			html: "<p>This has <strong>bold <em>and italic</em></strong> text</p>",
			validateFn: func(node *HTMLNode) error {
				// Basic structure validation
				if node == nil {
					return fmt.Errorf("node is nil")
				}
				return nil
			},
		},
		{
			name: "Headers",
			html: "<h1>Title</h1><h2>Subtitle</h2>",
			validateFn: func(node *HTMLNode) error {
				if node == nil {
					return fmt.Errorf("node is nil")
				}
				// Find headers
				var h1Found, h2Found bool
				for _, child := range node.Children {
					if child.Tag == "h1" {
						h1Found = true
					}
					if child.Tag == "h2" {
						h2Found = true
					}
				}
				if !h1Found {
					return fmt.Errorf("h1 tag not found")
				}
				if !h2Found {
					return fmt.Errorf("h2 tag not found")
				}
				return nil
			},
		},
	}

	converter := &MarkdownConverter{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Wrap HTML in body tag for parsing
			fullHTML := "<html><body>" + tt.html + "</body></html>"
			node, err := converter.ParseHTML(fullHTML)
			if err != nil {
				t.Fatalf("Failed to parse HTML: %v", err)
			}

			if err := tt.validateFn(node); err != nil {
				t.Errorf("Validation failed: %v", err)
			}
		})
	}
}

func TestGetStylesForTag(t *testing.T) {
	tests := []struct {
		name              string
		tag               string
		expectTextStyle   bool
		expectParaStyle   bool
		validateTextStyle func(*docs.TextStyle) error
		validateParaStyle func(*docs.ParagraphStyle) error
	}{
		{
			name:            "H1 tag",
			tag:             "h1",
			expectTextStyle: false,
			expectParaStyle: true,
			validateParaStyle: func(style *docs.ParagraphStyle) error {
				if style.NamedStyleType != "TITLE" {
					return fmt.Errorf("expected TITLE, got %s", style.NamedStyleType)
				}
				return nil
			},
		},
		{
			name:            "H2 tag",
			tag:             "h2",
			expectTextStyle: false,
			expectParaStyle: true,
			validateParaStyle: func(style *docs.ParagraphStyle) error {
				if style.NamedStyleType != "HEADING_1" {
					return fmt.Errorf("expected HEADING_1, got %s", style.NamedStyleType)
				}
				return nil
			},
		},
		{
			name:            "Bold tag",
			tag:             "strong",
			expectTextStyle: true,
			expectParaStyle: false,
			validateTextStyle: func(style *docs.TextStyle) error {
				if !style.Bold {
					return fmt.Errorf("expected bold to be true")
				}
				return nil
			},
		},
		{
			name:            "Italic tag",
			tag:             "em",
			expectTextStyle: true,
			expectParaStyle: false,
			validateTextStyle: func(style *docs.TextStyle) error {
				if !style.Italic {
					return fmt.Errorf("expected italic to be true")
				}
				return nil
			},
		},
		{
			name:            "Code tag",
			tag:             "code",
			expectTextStyle: true,
			expectParaStyle: false,
			validateTextStyle: func(style *docs.TextStyle) error {
				if style.WeightedFontFamily == nil {
					return fmt.Errorf("expected font family to be set")
				}
				if style.WeightedFontFamily.FontFamily != "Courier New" {
					return fmt.Errorf("expected Courier New font")
				}
				if style.BackgroundColor == nil {
					return fmt.Errorf("expected background color to be set")
				}
				return nil
			},
		},
		{
			name:            "Blockquote tag",
			tag:             "blockquote",
			expectTextStyle: false,
			expectParaStyle: true,
			validateParaStyle: func(style *docs.ParagraphStyle) error {
				if style.IndentStart == nil || style.IndentEnd == nil {
					return fmt.Errorf("expected indentation to be set")
				}
				return nil
			},
		},
	}

	converter := &MarkdownConverter{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			textStyle, paraStyle := converter.getStylesForTag(tt.tag)

			if tt.expectTextStyle && textStyle == nil {
				t.Error("Expected text style but got nil")
			}
			if !tt.expectTextStyle && textStyle != nil {
				t.Error("Expected no text style but got one")
			}
			if tt.expectParaStyle && paraStyle == nil {
				t.Error("Expected paragraph style but got nil")
			}
			if !tt.expectParaStyle && paraStyle != nil {
				t.Error("Expected no paragraph style but got one")
			}

			if tt.validateTextStyle != nil && textStyle != nil {
				if err := tt.validateTextStyle(textStyle); err != nil {
					t.Errorf("Text style validation failed: %v", err)
				}
			}
			if tt.validateParaStyle != nil && paraStyle != nil {
				if err := tt.validateParaStyle(paraStyle); err != nil {
					t.Errorf("Paragraph style validation failed: %v", err)
				}
			}
		})
	}
}

func TestIsBlockElement(t *testing.T) {
	tests := []struct {
		tag     string
		isBlock bool
	}{
		{"p", true},
		{"div", true},
		{"h1", true},
		{"h2", true},
		{"ul", true},
		{"ol", true},
		{"li", true},
		{"blockquote", true},
		{"pre", true},
		{"span", false},
		{"strong", false},
		{"em", false},
		{"code", false},
		{"a", false},
	}

	converter := &MarkdownConverter{}

	for _, tt := range tests {
		t.Run(tt.tag, func(t *testing.T) {
			result := converter.isBlockElement(tt.tag)
			if result != tt.isBlock {
				t.Errorf("Expected isBlockElement(%s) to be %v, got %v", tt.tag, tt.isBlock, result)
			}
		})
	}
}

func TestGetTextStyleFields(t *testing.T) {
	tests := []struct {
		name     string
		style    *docs.TextStyle
		expected []string
	}{
		{
			name: "Bold only",
			style: &docs.TextStyle{
				Bold: true,
			},
			expected: []string{"bold"},
		},
		{
			name: "Italic only",
			style: &docs.TextStyle{
				Italic: true,
			},
			expected: []string{"italic"},
		},
		{
			name: "Bold and italic",
			style: &docs.TextStyle{
				Bold:   true,
				Italic: true,
			},
			expected: []string{"bold", "italic"},
		},
		{
			name: "With font",
			style: &docs.TextStyle{
				Bold: true,
				WeightedFontFamily: &docs.WeightedFontFamily{
					FontFamily: "Courier New",
				},
			},
			expected: []string{"bold", "weightedFontFamily"},
		},
		{
			name: "With colors",
			style: &docs.TextStyle{
				BackgroundColor: &docs.OptionalColor{},
				ForegroundColor: &docs.OptionalColor{},
			},
			expected: []string{"backgroundColor", "foregroundColor"},
		},
	}

	converter := &MarkdownConverter{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fields := converter.getTextStyleFields(tt.style)
			fieldsList := strings.Split(fields, ",")

			if len(fieldsList) != len(tt.expected) {
				t.Errorf("Expected %d fields, got %d", len(tt.expected), len(fieldsList))
			}

			// Check each expected field is present
			for _, expectedField := range tt.expected {
				found := false
				for _, field := range fieldsList {
					if field == expectedField {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected field '%s' not found in result: %s", expectedField, fields)
				}
			}
		})
	}
}

func TestGetParagraphStyleFields(t *testing.T) {
	tests := []struct {
		name     string
		style    *docs.ParagraphStyle
		expected []string
	}{
		{
			name: "Named style only",
			style: &docs.ParagraphStyle{
				NamedStyleType: "HEADING_1",
			},
			expected: []string{"namedStyleType"},
		},
		{
			name: "With alignment",
			style: &docs.ParagraphStyle{
				NamedStyleType: "NORMAL",
				Alignment:      "CENTER",
			},
			expected: []string{"namedStyleType", "alignment"},
		},
		{
			name: "With indentation",
			style: &docs.ParagraphStyle{
				IndentStart: &docs.Dimension{Magnitude: 36},
				IndentEnd:   &docs.Dimension{Magnitude: 36},
			},
			expected: []string{"indentStart", "indentEnd"},
		},
		{
			name: "With spacing",
			style: &docs.ParagraphStyle{
				SpacingMode: "CUSTOM",
				SpaceAbove:  &docs.Dimension{Magnitude: 12},
				SpaceBelow:  &docs.Dimension{Magnitude: 12},
			},
			expected: []string{"spacingMode", "spaceAbove", "spaceBelow"},
		},
	}

	converter := &MarkdownConverter{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fields := converter.getParagraphStyleFields(tt.style)
			fieldsList := strings.Split(fields, ",")

			if len(fieldsList) != len(tt.expected) {
				t.Errorf("Expected %d fields, got %d", len(tt.expected), len(fieldsList))
			}

			// Check each expected field is present
			for _, expectedField := range tt.expected {
				found := false
				for _, field := range fieldsList {
					if field == expectedField {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected field '%s' not found in result: %s", expectedField, fields)
				}
			}
		})
	}
}

func TestConvertHTMLToDocsRequests(t *testing.T) {
	tests := []struct {
		name        string
		node        *HTMLNode
		startIndex  int64
		expectCount int // Expected number of requests
	}{
		{
			name: "Simple text",
			node: &HTMLNode{
				Type: "text",
				Text: "Hello World",
			},
			startIndex:  1,
			expectCount: 1, // 1 InsertText request
		},
		{
			name: "Paragraph with text",
			node: &HTMLNode{
				Type: "element",
				Tag:  "p",
				Children: []*HTMLNode{
					{
						Type: "text",
						Text: "Hello",
					},
				},
			},
			startIndex:  1,
			expectCount: 2, // 1 InsertText + 1 newline
		},
		{
			name: "Bold text",
			node: &HTMLNode{
				Type:  "element",
				Tag:   "strong",
				Style: &docs.TextStyle{Bold: true},
				Children: []*HTMLNode{
					{
						Type: "text",
						Text: "Bold",
					},
				},
			},
			startIndex:  1,
			expectCount: 2, // 1 InsertText + 1 UpdateTextStyle
		},
		{
			name: "Heading",
			node: &HTMLNode{
				Type: "element",
				Tag:  "h1",
				ParaStyle: &docs.ParagraphStyle{
					NamedStyleType: "TITLE",
				},
				Children: []*HTMLNode{
					{
						Type: "text",
						Text: "Title",
					},
				},
			},
			startIndex:  1,
			expectCount: 3, // 1 InsertText + 1 newline + 1 UpdateParagraphStyle
		},
	}

	converter := &MarkdownConverter{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requests, err := converter.ConvertHTMLToDocsRequests(tt.node, tt.startIndex)
			if err != nil {
				t.Fatalf("Failed to convert: %v", err)
			}

			if len(requests) != tt.expectCount {
				t.Errorf("Expected %d requests, got %d", tt.expectCount, len(requests))
				for i, req := range requests {
					t.Logf("Request %d: %T", i, req)
				}
			}
		})
	}
}

func TestEndToEndConversion(t *testing.T) {
	tests := []struct {
		name     string
		markdown string
		mode     string
	}{
		{
			name: "Complete document",
			markdown: `# Main Title

This is a paragraph with **bold** and *italic* text.

## Section 1

Here's some ` + "`code`" + ` and a [link](https://example.com).

### Subsection

- Item 1
- Item 2
- Item 3

1. First
2. Second
3. Third

> This is a blockquote

` + "```" + `
code block
with multiple lines
` + "```",
			mode: "replace",
		},
		{
			name: "Simple append",
			markdown: `## New Section

Additional content to append.`,
			mode: "append",
		},
	}

	// Create a mock client for testing
	client := &Client{}
	converter := NewMarkdownConverter("test-doc-id", client)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that the conversion doesn't panic and produces some requests
			// Note: This would need a mock client to fully test

			// Test HTML conversion
			html := converter.ConvertMarkdownToHTML(tt.markdown)
			if html == "" {
				t.Error("HTML conversion produced empty result")
			}

			// Test HTML parsing
			rootNode, err := converter.ParseHTML(html)
			if err != nil {
				t.Errorf("HTML parsing failed: %v", err)
			}
			if rootNode == nil {
				t.Error("HTML parsing produced nil root node")
			}
		})
	}
}
