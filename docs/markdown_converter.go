package docs

import (
	"fmt"
	"strings"

	"github.com/russross/blackfriday/v2"
	"golang.org/x/net/html"
	"google.golang.org/api/docs/v1"
)

// MarkdownConverter handles conversion from Markdown to Google Docs format
type MarkdownConverter struct {
	documentID string
	client     ClientInterface
}

// NewMarkdownConverter creates a new markdown converter instance
func NewMarkdownConverter(documentID string, client ClientInterface) *MarkdownConverter {
	return &MarkdownConverter{
		documentID: documentID,
		client:     client,
	}
}

// ConvertMarkdownToHTML converts markdown text to HTML
func (mc *MarkdownConverter) ConvertMarkdownToHTML(markdown string) string {
	// Use blackfriday for markdown to HTML conversion
	extensions := blackfriday.CommonExtensions | blackfriday.AutoHeadingIDs
	renderer := blackfriday.NewHTMLRenderer(blackfriday.HTMLRendererParameters{
		Flags: blackfriday.CommonHTMLFlags,
	})

	htmlBytes := blackfriday.Run([]byte(markdown),
		blackfriday.WithExtensions(extensions),
		blackfriday.WithRenderer(renderer))

	return string(htmlBytes)
}

// HTMLNode represents a parsed HTML node with its styling information
type HTMLNode struct {
	Type       string               // "text", "element"
	Tag        string               // HTML tag name
	Text       string               // Text content
	Attributes map[string]string    // HTML attributes
	Children   []*HTMLNode          // Child nodes
	Style      *docs.TextStyle      // Google Docs text style
	ParaStyle  *docs.ParagraphStyle // Google Docs paragraph style
}

// ParseHTML parses HTML string into a tree of HTMLNode
func (mc *MarkdownConverter) ParseHTML(htmlContent string) (*HTMLNode, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	// Find the body element
	var body *html.Node
	var findBody func(*html.Node)
	findBody = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "body" {
			body = n
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			findBody(c)
		}
	}
	findBody(doc)

	if body == nil {
		// If no body found, use the root
		body = doc
	}

	return mc.parseHTMLNode(body), nil
}

// parseHTMLNode recursively parses HTML nodes
func (mc *MarkdownConverter) parseHTMLNode(n *html.Node) *HTMLNode {
	node := &HTMLNode{
		Attributes: make(map[string]string),
		Children:   []*HTMLNode{},
	}

	switch n.Type {
	case html.TextNode:
		node.Type = "text"
		node.Text = n.Data

	case html.ElementNode:
		node.Type = "element"
		node.Tag = n.Data

		// Copy attributes
		for _, attr := range n.Attr {
			node.Attributes[attr.Key] = attr.Val
		}

		// Set styles based on tag
		node.Style, node.ParaStyle = mc.getStylesForTag(n.Data)

		// Parse children
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if child := mc.parseHTMLNode(c); child != nil {
				node.Children = append(node.Children, child)
			}
		}
	}

	return node
}

// getStylesForTag returns Google Docs styles for HTML tags
func (mc *MarkdownConverter) getStylesForTag(tag string) (*docs.TextStyle, *docs.ParagraphStyle) {
	var textStyle *docs.TextStyle
	var paraStyle *docs.ParagraphStyle

	switch tag {
	case "h1":
		paraStyle = &docs.ParagraphStyle{
			NamedStyleType: "TITLE",
		}
	case "h2":
		paraStyle = &docs.ParagraphStyle{
			NamedStyleType: "HEADING_1",
		}
	case "h3":
		paraStyle = &docs.ParagraphStyle{
			NamedStyleType: "HEADING_2",
		}
	case "h4":
		paraStyle = &docs.ParagraphStyle{
			NamedStyleType: "HEADING_3",
		}
	case "h5":
		paraStyle = &docs.ParagraphStyle{
			NamedStyleType: "HEADING_4",
		}
	case "h6":
		paraStyle = &docs.ParagraphStyle{
			NamedStyleType: "HEADING_5",
		}
	case "strong", "b":
		textStyle = &docs.TextStyle{
			Bold: true,
		}
	case "em", "i":
		textStyle = &docs.TextStyle{
			Italic: true,
		}
	case "u":
		textStyle = &docs.TextStyle{
			Underline: true,
		}
	case "code":
		textStyle = &docs.TextStyle{
			WeightedFontFamily: &docs.WeightedFontFamily{
				FontFamily: "Courier New",
				Weight:     400,
			},
			BackgroundColor: &docs.OptionalColor{
				Color: &docs.Color{
					RgbColor: &docs.RgbColor{
						Red:   0.95,
						Green: 0.95,
						Blue:  0.95,
					},
				},
			},
		}
	case "pre":
		textStyle = &docs.TextStyle{
			WeightedFontFamily: &docs.WeightedFontFamily{
				FontFamily: "Courier New",
				Weight:     400,
			},
			FontSize: &docs.Dimension{
				Magnitude: 10,
				Unit:      "PT",
			},
		}
		paraStyle = &docs.ParagraphStyle{
			IndentFirstLine: &docs.Dimension{
				Magnitude: 36,
				Unit:      "PT",
			},
		}
	case "blockquote":
		paraStyle = &docs.ParagraphStyle{
			IndentStart: &docs.Dimension{
				Magnitude: 36,
				Unit:      "PT",
			},
			IndentEnd: &docs.Dimension{
				Magnitude: 36,
				Unit:      "PT",
			},
		}
	}

	return textStyle, paraStyle
}

// ConvertHTMLToDocsRequests converts HTML nodes to Google Docs API requests
func (mc *MarkdownConverter) ConvertHTMLToDocsRequests(root *HTMLNode, startIndex int64) ([]*docs.Request, error) {
	var requests []*docs.Request
	var currentIndex int64 = startIndex

	// Traverse the HTML tree and generate requests
	mc.traverseAndConvert(root, &requests, &currentIndex)

	return requests, nil
}

// traverseAndConvert recursively traverses HTML nodes and generates requests
func (mc *MarkdownConverter) traverseAndConvert(node *HTMLNode, requests *[]*docs.Request, currentIndex *int64) {
	if node == nil {
		return
	}

	switch node.Type {
	case "text":
		if node.Text != "" {
			// Insert text
			*requests = append(*requests, &docs.Request{
				InsertText: &docs.InsertTextRequest{
					Text: node.Text,
					Location: &docs.Location{
						Index: *currentIndex,
					},
				},
			})
			*currentIndex += int64(len(node.Text))
		}

	case "element":
		startIdx := *currentIndex

		// Process children first
		for _, child := range node.Children {
			mc.traverseAndConvert(child, requests, currentIndex)
		}

		// Add newline for block elements
		if mc.isBlockElement(node.Tag) && *currentIndex > startIdx {
			*requests = append(*requests, &docs.Request{
				InsertText: &docs.InsertTextRequest{
					Text: "\n",
					Location: &docs.Location{
						Index: *currentIndex,
					},
				},
			})
			*currentIndex++
		}

		endIdx := *currentIndex

		// Apply text style if needed
		if node.Style != nil && endIdx > startIdx {
			fields := mc.getTextStyleFields(node.Style)
			if fields != "" {
				*requests = append(*requests, &docs.Request{
					UpdateTextStyle: &docs.UpdateTextStyleRequest{
						Range: &docs.Range{
							StartIndex: startIdx,
							EndIndex:   endIdx,
						},
						TextStyle: node.Style,
						Fields:    fields,
					},
				})
			}
		}

		// Apply paragraph style if needed
		if node.ParaStyle != nil && endIdx > startIdx {
			fields := mc.getParagraphStyleFields(node.ParaStyle)
			if fields != "" {
				*requests = append(*requests, &docs.Request{
					UpdateParagraphStyle: &docs.UpdateParagraphStyleRequest{
						Range: &docs.Range{
							StartIndex: startIdx,
							EndIndex:   endIdx,
						},
						ParagraphStyle: node.ParaStyle,
						Fields:         fields,
					},
				})
			}
		}
	}
}

// isBlockElement checks if an HTML tag is a block element
func (mc *MarkdownConverter) isBlockElement(tag string) bool {
	blockElements := map[string]bool{
		"p": true, "div": true, "h1": true, "h2": true, "h3": true,
		"h4": true, "h5": true, "h6": true, "ul": true, "ol": true,
		"li": true, "blockquote": true, "pre": true, "hr": true,
		"table": true, "tr": true, "td": true, "th": true,
	}
	return blockElements[tag]
}

// getTextStyleFields returns the fields string for text style
func (mc *MarkdownConverter) getTextStyleFields(style *docs.TextStyle) string {
	var fields []string

	if style.Bold {
		fields = append(fields, "bold")
	}
	if style.Italic {
		fields = append(fields, "italic")
	}
	if style.Underline {
		fields = append(fields, "underline")
	}
	if style.WeightedFontFamily != nil {
		fields = append(fields, "weightedFontFamily")
	}
	if style.FontSize != nil {
		fields = append(fields, "fontSize")
	}
	if style.BackgroundColor != nil {
		fields = append(fields, "backgroundColor")
	}
	if style.ForegroundColor != nil {
		fields = append(fields, "foregroundColor")
	}

	return strings.Join(fields, ",")
}

// getParagraphStyleFields returns the fields string for paragraph style
func (mc *MarkdownConverter) getParagraphStyleFields(style *docs.ParagraphStyle) string {
	var fields []string

	if style.NamedStyleType != "" {
		fields = append(fields, "namedStyleType")
	}
	if style.Alignment != "" {
		fields = append(fields, "alignment")
	}
	if style.IndentFirstLine != nil {
		fields = append(fields, "indentFirstLine")
	}
	if style.IndentStart != nil {
		fields = append(fields, "indentStart")
	}
	if style.IndentEnd != nil {
		fields = append(fields, "indentEnd")
	}
	if style.SpacingMode != "" {
		fields = append(fields, "spacingMode")
	}
	if style.SpaceAbove != nil {
		fields = append(fields, "spaceAbove")
	}
	if style.SpaceBelow != nil {
		fields = append(fields, "spaceBelow")
	}

	return strings.Join(fields, ",")
}

// ConvertMarkdownToDocsRequests is the main conversion function
func (mc *MarkdownConverter) ConvertMarkdownToDocsRequests(markdown string, mode string) ([]*docs.Request, error) {
	var requests []*docs.Request
	var startIndex int64 = 1

	// Handle replace mode
	if mode == "replace" {
		doc, err := mc.client.GetDocument(mc.documentID)
		if err != nil {
			return nil, fmt.Errorf("failed to get document: %w", err)
		}

		// Find the end index
		endIndex := int64(1)
		if doc.Body != nil && doc.Body.Content != nil && len(doc.Body.Content) > 0 {
			lastElement := doc.Body.Content[len(doc.Body.Content)-1]
			if lastElement.EndIndex > 0 {
				endIndex = lastElement.EndIndex - 1
			}
		}

		// Delete existing content
		if endIndex > 1 {
			requests = append(requests, &docs.Request{
				DeleteContentRange: &docs.DeleteContentRangeRequest{
					Range: &docs.Range{
						StartIndex: 1,
						EndIndex:   endIndex,
					},
				},
			})
		}
	} else if mode == "append" {
		// Find where to append
		doc, err := mc.client.GetDocument(mc.documentID)
		if err != nil {
			return nil, fmt.Errorf("failed to get document: %w", err)
		}

		if doc.Body != nil && doc.Body.Content != nil && len(doc.Body.Content) > 0 {
			lastElement := doc.Body.Content[len(doc.Body.Content)-1]
			if lastElement.EndIndex > 0 {
				startIndex = lastElement.EndIndex - 1
			}
		}
	}

	// Convert markdown to HTML
	htmlContent := mc.ConvertMarkdownToHTML(markdown)

	// Parse HTML to node tree
	rootNode, err := mc.ParseHTML(htmlContent)
	if err != nil {
		return nil, err
	}

	// Convert HTML nodes to Docs requests
	convertRequests, err := mc.ConvertHTMLToDocsRequests(rootNode, startIndex)
	if err != nil {
		return nil, err
	}

	requests = append(requests, convertRequests...)

	return requests, nil
}
