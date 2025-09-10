package slides

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf16"

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

	// Get the TITLE_AND_BODY layout ID
	layoutId, err := mc.client.GetLayoutId(mc.presentationId, "TITLE_AND_BODY")
	if err != nil {
		// Fallback to blank slides if layout not found
		fmt.Printf("Warning: failed to get TITLE_AND_BODY layout: %v\n", err)
		layoutId = ""
	}

	// Get the TITLE layout ID for title slides (slides with only two headings)
	titleLayoutId, _ := mc.client.GetLayoutId(mc.presentationId, "TITLE")

	// Create all slides fresh
	// Get the TITLE_ONLY layout ID for slides with tables
	titleOnlyLayoutId, _ := mc.client.GetLayoutId(mc.presentationId, "TITLE_ONLY")

	for i, slide := range parsedSlides {
		// Check if slide contains tables
		hasTable := false
		for _, element := range slide.Content {
			if element.Type == "table" {
				hasTable = true
				break
			}
		}

		// Check if slide has only two headings (title slide pattern)
		// Title slides are detected when:
		// 1. A slide has exactly 2 headings with no other content (title + subtitle)
		// 2. The first slide (index 0) contains only headings (common for presentation title slides)
		// This provides better visual layout for title/section divider slides
		isTitleSlide := false
		if titleLayoutId != "" && !hasTable {
			headingCount := 0
			nonHeadingCount := 0
			for _, element := range slide.Content {
				if element.Type == "text" && element.Level > 0 {
					headingCount++
				} else if element.Type != "text" || element.Level == 0 {
					nonHeadingCount++
				}
			}
			// Consider it a title slide if it has exactly 2 headings and no other content
			// OR if it's the first slide (i == 0) with only headings
			isTitleSlide = (headingCount == 2 && nonHeadingCount == 0) || (i == 0 && headingCount > 0 && nonHeadingCount == 0)
		}

		// Choose layout based on content
		var resp *slides.BatchUpdatePresentationResponse
		var useLayoutBased bool
		var layoutType string
		if isTitleSlide && titleLayoutId != "" {
			// Use TITLE layout for title slides
			resp, err = mc.client.CreateSlideWithLayout(mc.presentationId, titleLayoutId, -1)
			useLayoutBased = true
			layoutType = "TITLE"
		} else if hasTable && titleOnlyLayoutId != "" {
			// Use TITLE_ONLY layout for slides with tables
			resp, err = mc.client.CreateSlideWithLayout(mc.presentationId, titleOnlyLayoutId, -1)
			useLayoutBased = true
			layoutType = "TITLE_ONLY"
		} else if layoutId != "" {
			// Use TITLE_AND_BODY layout for regular slides
			resp, err = mc.client.CreateSlideWithLayout(mc.presentationId, layoutId, -1)
			useLayoutBased = true
			layoutType = "TITLE_AND_BODY"
		} else {
			// Fallback to blank slide
			resp, err = mc.client.CreateSlide(mc.presentationId, -1)
			useLayoutBased = false
			layoutType = "BLANK"
		}

		if err != nil {
			return nil, fmt.Errorf("failed to create slide %d: %w", i+1, err)
		}

		var slideId string
		if len(resp.Replies) > 0 && resp.Replies[0].CreateSlide != nil {
			slideId = resp.Replies[0].CreateSlide.ObjectId
		} else {
			return nil, fmt.Errorf("failed to get slide ID for slide %d", i+1)
		}

		// Populate slide based on layout type and content
		if useLayoutBased {
			if layoutType == "TITLE" {
				// Special handling for title slides
				err = mc.populateSlideWithTitleLayout(slideId, slide)
			} else if layoutType == "TITLE_ONLY" {
				// Special handling for slides with tables (TITLE_ONLY layout)
				err = mc.populateSlideWithTableLayout(slideId, slide)
			} else {
				// Regular TITLE_AND_BODY layout
				err = mc.populateSlideWithLayout(slideId, slide)
			}
		} else {
			// Blank slide
			err = mc.populateSlide(slideId, slide)
		}

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

// populateSlideWithLayout populates a slide that uses a predefined layout
func (mc *MarkdownConverter) populateSlideWithLayout(slideId string, slide MarkdownSlide) error {
	// Get the slide to find placeholder shapes
	presentation, err := mc.client.GetPresentation(mc.presentationId)
	if err != nil {
		return fmt.Errorf("failed to get presentation: %w", err)
	}

	// Find the slide we just created
	var currentSlide *slides.Page
	for _, s := range presentation.Slides {
		if s.ObjectId == slideId {
			currentSlide = s
			break
		}
	}

	if currentSlide == nil {
		return fmt.Errorf("slide not found: %s", slideId)
	}

	// Find title and body placeholders
	var titlePlaceholderId, bodyPlaceholderId string
	for _, element := range currentSlide.PageElements {
		if element.Shape != nil && element.Shape.Placeholder != nil {
			switch element.Shape.Placeholder.Type {
			case "TITLE", "CENTERED_TITLE":
				titlePlaceholderId = element.ObjectId
			case "BODY":
				bodyPlaceholderId = element.ObjectId
			}
		}
	}

	// Insert title if we have a title placeholder
	if titlePlaceholderId != "" && slide.Title != "" {
		// Delete existing placeholder text
		// Ignore error as placeholder might be empty
		_, _ = mc.client.DeleteTextInPlaceholder(mc.presentationId, titlePlaceholderId)

		// Insert new title text
		_, err = mc.client.InsertTextInPlaceholder(mc.presentationId, titlePlaceholderId, slide.Title)
		if err != nil {
			return fmt.Errorf("failed to insert title: %w", err)
		}
	}

	// Insert body content if we have a body placeholder
	if bodyPlaceholderId != "" && len(slide.Content) > 0 {
		// Delete existing placeholder text
		// Ignore error as placeholder might be empty
		_, _ = mc.client.DeleteTextInPlaceholder(mc.presentationId, bodyPlaceholderId)

		// Find the first heading (Level 2 or 3) to use as title if slide.Title is empty
		var slideTitle string
		var bodyText []string
		var codeRanges []struct {
			start int
			end   int
		}

		// Build the text and track code positions using UTF-16 code units
		currentPos := 0
		for i, element := range slide.Content {
			switch element.Type {
			case "text":
				// If this is a heading (Level 2 or 3) and we don't have a slide title yet, use it as title
				if (element.Level == 2 || element.Level == 3) && slideTitle == "" && slide.Title == "" {
					slideTitle = element.Content
				} else if element.Level > 0 {
					// Other headings go to body with appropriate formatting
					bodyText = append(bodyText, element.Content)
					// Calculate UTF-16 length
					currentPos += len(utf16.Encode([]rune(element.Content)))
					if i < len(slide.Content)-1 {
						currentPos += 1 // +1 for newline in UTF-16
					}
				} else {
					// Regular text
					bodyText = append(bodyText, element.Content)
					currentPos += len(utf16.Encode([]rune(element.Content)))
					if i < len(slide.Content)-1 {
						currentPos += 1 // +1 for newline in UTF-16
					}
				}
			case "bullet":
				text := "• " + element.Content
				bodyText = append(bodyText, text)
				currentPos += len(utf16.Encode([]rune(text)))
				if i < len(slide.Content)-1 {
					currentPos += 1 // +1 for newline in UTF-16
				}
			case "numbering":
				text := "1. " + element.Content
				bodyText = append(bodyText, text)
				currentPos += len(utf16.Encode([]rune(text)))
				if i < len(slide.Content)-1 {
					currentPos += 1 // +1 for newline in UTF-16
				}
			case "code":
				// Track the position of code blocks for formatting using UTF-16 code units
				codeStart := currentPos
				codeEnd := currentPos + len(utf16.Encode([]rune(element.Content)))
				codeRanges = append(codeRanges, struct {
					start int
					end   int
				}{
					start: codeStart,
					end:   codeEnd,
				})
				bodyText = append(bodyText, element.Content)
				currentPos = codeEnd
				if i < len(slide.Content)-1 {
					currentPos += 1 // +1 for newline in UTF-16
				}
			}
		}

		// If we found a heading and no slide title was set, use it as title
		if slideTitle != "" && slide.Title == "" && titlePlaceholderId != "" {
			// Ignore error as placeholder might be empty
			_, _ = mc.client.DeleteTextInPlaceholder(mc.presentationId, titlePlaceholderId)

			_, err = mc.client.InsertTextInPlaceholder(mc.presentationId, titlePlaceholderId, slideTitle)
			if err != nil {
				return fmt.Errorf("failed to insert title: %w", err)
			}
		}

		if len(bodyText) > 0 {
			combinedText := strings.Join(bodyText, "\n")
			_, err = mc.client.InsertTextInPlaceholder(mc.presentationId, bodyPlaceholderId, combinedText)
			if err != nil {
				return fmt.Errorf("failed to insert body text: %w", err)
			}

			// Apply Courier New font to code blocks
			if len(codeRanges) > 0 {
				err = mc.client.ApplyCodeFormattingToPlaceholder(mc.presentationId, bodyPlaceholderId, codeRanges)
				if err != nil {
					// Return the error so we can see what's happening
					return fmt.Errorf("failed to apply code formatting: %w", err)
				}
			}
		}
	}

	return nil
}

// populateSlideWithTitleLayout populates a slide with TITLE layout (for title slides with only headings)
// This function is used for slides that contain only headings (typically 2: title and subtitle)
// It maps the headings to the appropriate title and subtitle placeholders in the TITLE layout
func (mc *MarkdownConverter) populateSlideWithTitleLayout(slideId string, slide MarkdownSlide) error {
	// Get the slide to find placeholder shapes
	presentation, err := mc.client.GetPresentation(mc.presentationId)
	if err != nil {
		return fmt.Errorf("failed to get presentation: %w", err)
	}

	// Find the slide we just created
	var currentSlide *slides.Page
	for _, s := range presentation.Slides {
		if s.ObjectId == slideId {
			currentSlide = s
			break
		}
	}

	if currentSlide == nil {
		return fmt.Errorf("slide not found: %s", slideId)
	}

	// Find title and subtitle placeholders
	var titlePlaceholderId, subtitlePlaceholderId string
	for _, element := range currentSlide.PageElements {
		if element.Shape != nil && element.Shape.Placeholder != nil {
			switch element.Shape.Placeholder.Type {
			case "TITLE", "CENTERED_TITLE":
				titlePlaceholderId = element.ObjectId
			case "SUBTITLE", "BODY":
				subtitlePlaceholderId = element.ObjectId
			}
		}
	}

	// Extract headings from content
	var headings []string
	for _, element := range slide.Content {
		if element.Type == "text" && element.Level > 0 {
			headings = append(headings, element.Content)
		}
	}

	// Use slide title if provided, otherwise use first heading
	titleText := slide.Title
	subtitleText := ""

	if titleText != "" {
		// If we have a slide title, use headings for subtitle
		if len(headings) > 0 {
			subtitleText = strings.Join(headings, "\n")
		}
	} else {
		// No slide title, use headings as title and subtitle
		if len(headings) > 0 {
			titleText = headings[0]
		}
		if len(headings) > 1 {
			subtitleText = strings.Join(headings[1:], "\n")
		}
	}

	// Insert title
	if titlePlaceholderId != "" && titleText != "" {
		// Ignore error as placeholder might be empty
		_, _ = mc.client.DeleteTextInPlaceholder(mc.presentationId, titlePlaceholderId)

		_, err = mc.client.InsertTextInPlaceholder(mc.presentationId, titlePlaceholderId, titleText)
		if err != nil {
			return fmt.Errorf("failed to insert title: %w", err)
		}
	}

	// Insert subtitle
	if subtitlePlaceholderId != "" && subtitleText != "" {
		// Ignore error as placeholder might be empty
		_, _ = mc.client.DeleteTextInPlaceholder(mc.presentationId, subtitlePlaceholderId)

		_, err = mc.client.InsertTextInPlaceholder(mc.presentationId, subtitlePlaceholderId, subtitleText)
		if err != nil {
			return fmt.Errorf("failed to insert subtitle: %w", err)
		}
	}

	return nil
}

// populateSlideWithTableLayout populates a slide with TITLE_ONLY layout that contains tables
func (mc *MarkdownConverter) populateSlideWithTableLayout(slideId string, slide MarkdownSlide) error {
	// Get the slide to find placeholder shapes
	presentation, err := mc.client.GetPresentation(mc.presentationId)
	if err != nil {
		return fmt.Errorf("failed to get presentation: %w", err)
	}

	// Find the slide we just created
	var currentSlide *slides.Page
	for _, s := range presentation.Slides {
		if s.ObjectId == slideId {
			currentSlide = s
			break
		}
	}

	if currentSlide == nil {
		return fmt.Errorf("slide not found: %s", slideId)
	}

	// Find title placeholder
	var titlePlaceholderId string
	for _, element := range currentSlide.PageElements {
		if element.Shape != nil && element.Shape.Placeholder != nil {
			switch element.Shape.Placeholder.Type {
			case "TITLE", "CENTERED_TITLE":
				titlePlaceholderId = element.ObjectId
			}
		}
	}

	// Insert title - use slide title or first heading from content
	titleText := slide.Title
	if titleText == "" {
		// Look for the first heading in content
		for _, element := range slide.Content {
			if element.Type == "text" && (element.Level == 2 || element.Level == 3) {
				titleText = element.Content
				break
			}
		}
	}

	if titlePlaceholderId != "" && titleText != "" {
		// Delete existing placeholder text
		// Ignore error as placeholder might be empty
		_, _ = mc.client.DeleteTextInPlaceholder(mc.presentationId, titlePlaceholderId)

		// Insert title text
		_, err = mc.client.InsertTextInPlaceholder(mc.presentationId, titlePlaceholderId, titleText)
		if err != nil {
			return fmt.Errorf("failed to insert title: %w", err)
		}
	}

	// Add content manually below the title
	currentY := MarginTop + TitleFontSize*LineHeight*2 // Space below title

	for _, element := range slide.Content {
		switch element.Type {
		case "text":
			// Skip headings that were used as titles
			if element.Level == 2 || element.Level == 3 {
				continue
			}

			fontSize := DefaultFontSize
			if element.Level == 1 {
				fontSize = H1FontSize
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

			_, err := mc.client.AddTextBox(
				mc.presentationId,
				slideId,
				prefix+element.Content,
				MarginLeft,
				currentY,
				SlideWidth-MarginLeft-MarginRight,
				DefaultFontSize*LineHeight,
			)
			if err != nil {
				return err
			}
			currentY += DefaultFontSize * LineHeight

		case "code":
			// Add code block with Courier New font
			_, err := mc.client.AddCodeTextBox(
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
					if resp != nil && len(resp.Replies) > 0 && resp.Replies[0].CreateTable != nil {
						tableId := resp.Replies[0].CreateTable.ObjectId

						for rowIdx, row := range element.Items {
							// Split by | and remove empty entries
							cells := strings.Split(row, "|")
							cellTexts := []string{}
							for _, cell := range cells {
								trimmed := strings.TrimSpace(cell)
								if trimmed != "" {
									cellTexts = append(cellTexts, trimmed)
								}
							}

							// Insert text into each cell
							for colIdx, cellText := range cellTexts {
								if colIdx < cols {
									_, err := mc.client.InsertTextInTableCell(
										mc.presentationId,
										tableId,
										rowIdx,
										colIdx,
										cellText,
									)
									if err != nil {
										// Log error but continue with other cells
										fmt.Printf("Warning: failed to insert text in cell [%d,%d]: %v\n", rowIdx, colIdx, err)
									}
								}
							}
						}
					}

					currentY += tableHeight + 10
				}
			}
		}
	}

	return nil
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
			// Add code block with Courier New font
			_, err := mc.client.AddCodeTextBox(
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

					// Get the table ID from the response
					if resp != nil && len(resp.Replies) > 0 && resp.Replies[0].CreateTable != nil {
						tableId := resp.Replies[0].CreateTable.ObjectId

						// Populate table cells with text
						for rowIdx, row := range element.Items {
							// Split by | and remove empty entries
							cells := strings.Split(row, "|")
							cellTexts := []string{}
							for _, cell := range cells {
								trimmed := strings.TrimSpace(cell)
								if trimmed != "" {
									cellTexts = append(cellTexts, trimmed)
								}
							}

							// Insert text into each cell
							for colIdx, cellText := range cellTexts {
								if colIdx < cols {
									_, err := mc.client.InsertTextInTableCell(
										mc.presentationId,
										tableId,
										rowIdx,
										colIdx,
										cellText,
									)
									if err != nil {
										// Log error but continue with other cells
										fmt.Printf("Warning: failed to insert text in cell [%d,%d]: %v\n", rowIdx, colIdx, err)
									}
								}
							}
						}
					}

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
