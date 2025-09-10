package slides

import (
	"fmt"
	"regexp"
	"strings"

	"google.golang.org/api/slides/v1"
)

const (
	// Slide dimensions in points (standard 16:9)
	SlideWidth  = 720.0
	SlideHeight = 405.0

	// Default margins
	MarginTop    = 50.0
	MarginBottom = 50.0
	MarginLeft   = 50.0
	MarginRight  = 50.0

	// Text properties
	DefaultFontSize = 14.0
	TitleFontSize   = 32.0
	H1FontSize      = 28.0
	H2FontSize      = 24.0
	H3FontSize      = 20.0
	LineHeight      = 1.5

	// Estimated character width for pagination
	CharWidth = 7.0 // Approximate for 14pt font
)

type MarkdownSlide struct {
	Title   string
	Content []MarkdownElement
	Layout  string
}

type MarkdownElement struct {
	Type    string // "text", "bullet", "numbering", "image", "table", "code"
	Content string
	Level   int      // For headers and lists
	Items   []string // For tables
}

type MarkdownConverter struct {
	presentationId string
	client         *Client
}

func NewMarkdownConverter(client *Client, presentationId string) *MarkdownConverter {
	return &MarkdownConverter{
		client:         client,
		presentationId: presentationId,
	}
}

func (mc *MarkdownConverter) ParseMarkdown(markdown string) []MarkdownSlide {
	slides := []MarkdownSlide{}

	// Split by horizontal rules (---)
	sections := strings.Split(markdown, "\n---\n")
	if len(sections) == 1 {
		// No explicit page breaks, try to auto-paginate
		sections = mc.autoPaginate(markdown)
	}

	for _, section := range sections {
		slide := mc.parseSection(section)
		if slide.Title != "" || len(slide.Content) > 0 {
			slides = append(slides, slide)
		}
	}

	return slides
}

var numberedListRegex = regexp.MustCompile(`^\d+\.\s+(.*)`)

func (mc *MarkdownConverter) parseSection(section string) MarkdownSlide {
	slide := MarkdownSlide{
		Content: []MarkdownElement{},
	}

	lines := strings.Split(strings.TrimSpace(section), "\n")
	inCodeBlock := false
	codeContent := []string{}
	inTable := false
	tableRows := []string{}

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		// Code block handling
		if strings.HasPrefix(line, "```") {
			if inCodeBlock {
				// End of code block
				slide.Content = append(slide.Content, MarkdownElement{
					Type:    "code",
					Content: strings.Join(codeContent, "\n"),
				})
				codeContent = []string{}
				inCodeBlock = false
			} else {
				// Start of code block
				inCodeBlock = true
			}
			continue
		}

		if inCodeBlock {
			codeContent = append(codeContent, line)
			continue
		}

		// Table handling
		if strings.Contains(line, "|") && strings.TrimSpace(line) != "" {
			if !inTable {
				inTable = true
				tableRows = []string{line}
			} else {
				tableRows = append(tableRows, line)
			}

			// Check if next line is not a table row
			if i+1 >= len(lines) || !strings.Contains(lines[i+1], "|") {
				slide.Content = append(slide.Content, mc.parseTable(tableRows))
				inTable = false
				tableRows = []string{}
			}
			continue
		}

		// Headers
		if strings.HasPrefix(line, "# ") {
			if slide.Title == "" {
				slide.Title = strings.TrimPrefix(line, "# ")
			} else {
				slide.Content = append(slide.Content, MarkdownElement{
					Type:    "text",
					Content: strings.TrimPrefix(line, "# "),
					Level:   1,
				})
			}
		} else if strings.HasPrefix(line, "## ") {
			slide.Content = append(slide.Content, MarkdownElement{
				Type:    "text",
				Content: strings.TrimPrefix(line, "## "),
				Level:   2,
			})
		} else if strings.HasPrefix(line, "### ") {
			slide.Content = append(slide.Content, MarkdownElement{
				Type:    "text",
				Content: strings.TrimPrefix(line, "### "),
				Level:   3,
			})
		} else if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
			// Bullet points
			content := strings.TrimPrefix(strings.TrimPrefix(line, "- "), "* ")
			level := 0
			// Check for indentation
			if strings.HasPrefix(line, "  ") {
				level = len(line) - len(strings.TrimLeft(line, " "))/2
			}
			slide.Content = append(slide.Content, MarkdownElement{
				Type:    "bullet",
				Content: content,
				Level:   level,
			})
		} else if numberedListRegex.MatchString(line) {
			// Numbered list
			matches := numberedListRegex.FindStringSubmatch(line)
			if len(matches) > 1 {
				slide.Content = append(slide.Content, MarkdownElement{
					Type:    "numbering",
					Content: matches[1],
				})
			}
		} else if strings.HasPrefix(line, "![") {
			// Image
			re := regexp.MustCompile(`!\[([^\]]*)\]\(([^)]+)\)`)
			matches := re.FindStringSubmatch(line)
			if len(matches) > 2 {
				slide.Content = append(slide.Content, MarkdownElement{
					Type:    "image",
					Content: matches[2], // URL
				})
			}
		} else if strings.TrimSpace(line) != "" {
			// Regular text
			slide.Content = append(slide.Content, MarkdownElement{
				Type:    "text",
				Content: line,
			})
		}
	}

	return slide
}

func (mc *MarkdownConverter) parseTable(rows []string) MarkdownElement {
	items := []string{}
	for _, row := range rows {
		// Skip separator rows (---|---)
		if strings.Contains(row, "---") || strings.Contains(row, "===") {
			continue
		}
		items = append(items, row)
	}
	return MarkdownElement{
		Type:  "table",
		Items: items,
	}
}

func (mc *MarkdownConverter) autoPaginate(markdown string) []string {
	sections := []string{}
	currentSection := []string{}
	currentHeight := 0.0

	lines := strings.Split(markdown, "\n")
	availableHeight := SlideHeight - MarginTop - MarginBottom

	for _, line := range lines {
		lineHeight := mc.estimateLineHeight(line)

		if currentHeight+lineHeight > availableHeight && len(currentSection) > 0 {
			// Start new slide
			sections = append(sections, strings.Join(currentSection, "\n"))
			currentSection = []string{line}
			currentHeight = lineHeight
		} else {
			currentSection = append(currentSection, line)
			currentHeight += lineHeight
		}
	}

	if len(currentSection) > 0 {
		sections = append(sections, strings.Join(currentSection, "\n"))
	}

	return sections
}

func (mc *MarkdownConverter) estimateLineHeight(line string) float64 {
	if strings.HasPrefix(line, "# ") {
		return TitleFontSize * LineHeight
	} else if strings.HasPrefix(line, "## ") {
		return H1FontSize * LineHeight
	} else if strings.HasPrefix(line, "### ") {
		return H2FontSize * LineHeight
	} else if strings.HasPrefix(line, "```") {
		return DefaultFontSize * LineHeight
	} else if strings.TrimSpace(line) == "" {
		return DefaultFontSize * 0.5
	} else {
		// Estimate wrapped lines
		availableWidth := SlideWidth - MarginLeft - MarginRight
		charsPerLine := availableWidth / CharWidth
		numLines := float64(len(line)) / charsPerLine
		if numLines < 1 {
			numLines = 1
		}
		return DefaultFontSize * LineHeight * numLines
	}
}

func (mc *MarkdownConverter) CreateSlidesFromMarkdown(markdown string) ([]*slides.Page, error) {
	parsedSlides := mc.ParseMarkdown(markdown)
	

	// Get current presentation to check existing slides
	presentation, err := mc.client.GetPresentation(mc.presentationId)
	if err != nil {
		return nil, err
	}

	// Delete the first slide if it exists (the default title slide)
	if len(presentation.Slides) > 0 {
		firstSlideId := presentation.Slides[0].ObjectId
		_, err := mc.client.DeleteSlide(mc.presentationId, firstSlideId)
		if err != nil {
			// Log error but continue
			fmt.Printf("Warning: failed to delete first slide: %v\n", err)
		}
	}

	// Create all slides fresh
	for i, slide := range parsedSlides {
		// Create a new slide at the end of the presentation
		resp, err := mc.client.CreateSlide(mc.presentationId, -1)
		if err != nil {
			return nil, fmt.Errorf("failed to create slide %d: %w", i+1, err)
		}

		var slideId string
		if len(resp.Replies) > 0 && resp.Replies[0].CreateSlide != nil {
			slideId = resp.Replies[0].CreateSlide.ObjectId
		} else {
			return nil, fmt.Errorf("failed to get slide ID for slide %d", i+1)
		}

		// Add content to slide
		err = mc.populateSlide(slideId, slide)
		if err != nil {
			return nil, fmt.Errorf("failed to populate slide %d: %w", i+1, err)
		}
	}

	// Return updated presentation slides
	updatedPresentation, err := mc.client.GetPresentation(mc.presentationId)
	if err != nil {
		return nil, err
	}

	return updatedPresentation.Slides, nil
}

func (mc *MarkdownConverter) populateSlide(slideId string, slide MarkdownSlide) error {
	// All slides are now blank, so we add text boxes for everything
	currentY := MarginTop
	

	// Add title if exists
	if slide.Title != "" {
		resp, err := mc.client.AddTextBox(
			mc.presentationId,
			slideId,
			slide.Title,
			MarginLeft,
			currentY,
			SlideWidth-MarginLeft-MarginRight,
			TitleFontSize*LineHeight,
		)
		if err != nil {
			return fmt.Errorf("failed to add title '%s': %w", slide.Title, err)
		}
		// Log response for debugging
		if resp == nil || len(resp.Replies) == 0 {
			return fmt.Errorf("no response for title '%s'", slide.Title)
		}
		currentY += TitleFontSize * LineHeight * 1.5
	}

	// Add content elements
	for _, element := range slide.Content {
		switch element.Type {
		case "text":
			fontSize := DefaultFontSize
			if element.Level == 1 {
				fontSize = H1FontSize
			} else if element.Level == 2 {
				fontSize = H2FontSize
			} else if element.Level == 3 {
				fontSize = H3FontSize
			}

			_, err := mc.client.AddTextBox(
				mc.presentationId,
				slideId,
				element.Content,
				MarginLeft,
				currentY,
				SlideWidth-MarginLeft-MarginRight,
				fontSize*LineHeight,
			)
			if err != nil {
				return err
			}
			currentY += fontSize * LineHeight * 1.2

		case "bullet", "numbering":
			prefix := "• "
			if element.Type == "numbering" {
				prefix = "1. "
			}
			indent := float64(element.Level) * 20.0

			_, err := mc.client.AddTextBox(
				mc.presentationId,
				slideId,
				prefix+element.Content,
				MarginLeft+indent,
				currentY,
				SlideWidth-MarginLeft-MarginRight-indent,
				DefaultFontSize*LineHeight,
			)
			if err != nil {
				return err
			}
			currentY += DefaultFontSize * LineHeight

		case "code":
			// Add code block with monospace font
			_, err := mc.client.AddTextBox(
				mc.presentationId,
				slideId,
				element.Content,
				MarginLeft,
				currentY,
				SlideWidth-MarginLeft-MarginRight,
				100, // Fixed height for code blocks
			)
			if err != nil {
				return err
			}
			currentY += 100 + 10

		case "image":
			// Add image centered
			imageWidth := 400.0
			imageHeight := 300.0
			_, err := mc.client.AddImage(
				mc.presentationId,
				slideId,
				element.Content,
				(SlideWidth-imageWidth)/2,
				currentY,
				imageWidth,
				imageHeight,
			)
			if err != nil {
				return err
			}
			currentY += imageHeight + 10

		case "table":
			if len(element.Items) > 0 {
				rows := len(element.Items)
				cols := strings.Count(element.Items[0], "|") - 1
				if cols > 0 {
					tableWidth := SlideWidth - MarginLeft - MarginRight
					tableHeight := float64(rows) * 30.0

					resp, err := mc.client.AddTable(
						mc.presentationId,
						slideId,
						rows,
						cols,
						MarginLeft,
						currentY,
						tableWidth,
						tableHeight,
					)
					if err != nil {
						return err
					}

					// Populate table cells
					// Note: Table population would require additional API calls
					// to insert text into each cell - not implemented yet
					_ = resp

					currentY += tableHeight + 10
				}
			}
		}
	}

	return nil
}

func (mc *MarkdownConverter) UpdateSlidesFromMarkdown(markdown string) error {
	// Get current presentation
	presentation, err := mc.client.GetPresentation(mc.presentationId)
	if err != nil {
		return err
	}

	// Delete all existing slides except the first one
	if len(presentation.Slides) > 1 {
		for i := 1; i < len(presentation.Slides); i++ {
			_, err := mc.client.DeleteSlide(mc.presentationId, presentation.Slides[i].ObjectId)
			if err != nil {
				return err
			}
		}
	}

	// Create new slides from markdown
	_, err = mc.CreateSlidesFromMarkdown(markdown)
	return err
}
