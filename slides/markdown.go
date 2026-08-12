package slides

import (
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"
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
	AltText string   // For images
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

	// Split by horizontal rules (---) but ignore those inside code blocks
	sections := mc.splitByPageBreaks(markdown)
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

// splitByPageBreaks splits markdown by --- but ignores those inside code blocks
func (mc *MarkdownConverter) splitByPageBreaks(markdown string) []string {
	var sections []string
	var currentSection strings.Builder
	lines := strings.Split(markdown, "\n")
	inCodeBlock := false

	for i, line := range lines {
		// Track code block state
		if strings.HasPrefix(line, "```") {
			inCodeBlock = !inCodeBlock
		}

		// Check for page break (--- on its own line, not in code block)
		if !inCodeBlock && strings.TrimSpace(line) == "---" {
			// End current section
			section := strings.TrimSpace(currentSection.String())
			if section != "" {
				sections = append(sections, section)
			}
			currentSection.Reset()
			continue
		}

		// Add line to current section
		if currentSection.Len() > 0 {
			currentSection.WriteString("\n")
		}
		currentSection.WriteString(line)

		// Handle last line
		if i == len(lines)-1 {
			section := strings.TrimSpace(currentSection.String())
			if section != "" {
				sections = append(sections, section)
			}
		}
	}

	return sections
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
					AltText: matches[1], // Alt text
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
			log.Printf("[WARNING] Failed to delete first slide: %v\n", err)
		}
	}

	if _, err := mc.appendSlidesFromMarkdown(markdown); err != nil {
		return nil, err
	}

	// Return updated presentation slides
	updatedPresentation, err := mc.client.GetPresentation(mc.presentationId)
	if err != nil {
		return nil, err
	}

	return updatedPresentation.Slides, nil
}

// maxRequestsPerBatch caps how many requests one batchUpdate carries. A batch
// is applied atomically, so a whole deck in a single call is the cheapest and
// safest shape; the cap only exists because a very large deck could exceed the
// API's per-request limits. It is a variable so tests can force chunking
// without building an enormous deck.
var maxRequestsPerBatch = 500

// slideBatch accumulates requests and sends them in as few batchUpdate calls as
// possible. Object IDs are assigned here rather than read back from the API,
// which is what lets a slide and everything on it be built in one round trip.
type slideBatch struct {
	client         *Client
	presentationId string
	prefix         string
	nextId         int

	pending []*slides.Request
	// Slides whose requests are still pending. A failed batch is rolled back
	// by the API, so these must not be reported as created until they flush.
	pendingSlideIds []string
	createdSlideIds []string
}

func newSlideBatch(client *Client, presentationId string) *slideBatch {
	return &slideBatch{
		client:         client,
		presentationId: presentationId,
		prefix:         newObjectIdPrefix(),
	}
}

// newObjectIdPrefix returns a prefix unique to this run, so generated IDs
// cannot collide with objects already in the presentation.
func newObjectIdPrefix() string {
	return fmt.Sprintf("md%d", time.Now().UnixNano())
}

// id returns a fresh object ID. The API accepts [a-zA-Z0-9_][a-zA-Z0-9_-:]{4,49},
// which the prefix, kind and counter stay well inside.
func (b *slideBatch) id(kind string) string {
	b.nextId++
	return fmt.Sprintf("%s-%s%d", b.prefix, kind, b.nextId)
}

func (b *slideBatch) add(requests ...*slides.Request) {
	b.pending = append(b.pending, requests...)
}

// addSlide records a slide the pending requests create.
func (b *slideBatch) addSlide(slideId string) {
	b.pendingSlideIds = append(b.pendingSlideIds, slideId)
}

func (b *slideBatch) flush() error {
	return b.flushChunk(len(b.pending))
}

// flushChunk sends the first n pending requests and keeps the rest.
//
// Every slide's createSlide request is appended before the rest of that slide,
// and a chunk is only cut once the buffer has reached the cap, so the cut is
// always past the createSlide of every slide represented in it. That makes it
// safe to treat all pending slides as created once the chunk lands.
func (b *slideBatch) flushChunk(n int) error {
	if n > len(b.pending) {
		n = len(b.pending)
	}
	if n == 0 {
		return nil
	}

	if _, err := b.client.BatchUpdate(b.presentationId, b.pending[:n]); err != nil {
		return err
	}

	b.pending = b.pending[n:]
	b.createdSlideIds = append(b.createdSlideIds, b.pendingSlideIds...)
	b.pendingSlideIds = nil

	return nil
}

// flushIfFull sends full chunks while the buffer is at or over the cap, so no
// single BatchUpdate ever carries more than maxRequestsPerBatch requests.
//
// Checking only at slide boundaries is not enough on its own: a slide appended
// onto an almost-full buffer, or a single slide with a large table, would
// otherwise push one batch past the cap. Chunking mid-slide is safe because the
// object IDs are ours, so a later chunk addresses objects an earlier one
// created.
func (b *slideBatch) flushIfFull() error {
	for len(b.pending) >= maxRequestsPerBatch {
		if err := b.flushChunk(maxRequestsPerBatch); err != nil {
			return err
		}
	}

	return nil
}

// layoutPlaceholders holds a layout's own placeholder element IDs.
// CreateSlideRequest.PlaceholderIdMappings rejects a mapping for a placeholder
// the layout does not define, so these are read from the layout itself rather
// than assumed.
type layoutPlaceholders struct {
	title    string
	body     string
	subtitle string
}

func placeholdersForLayout(presentation *slides.Presentation, layoutId string) layoutPlaceholders {
	var found layoutPlaceholders
	if layoutId == "" {
		return found
	}

	for _, layout := range presentation.Layouts {
		if layout.ObjectId != layoutId {
			continue
		}
		for _, element := range layout.PageElements {
			if element.Shape == nil || element.Shape.Placeholder == nil {
				continue
			}
			switch element.Shape.Placeholder.Type {
			case "TITLE", "CENTERED_TITLE":
				if found.title == "" {
					found.title = element.ObjectId
				}
			case "BODY":
				if found.body == "" {
					found.body = element.ObjectId
				}
			case "SUBTITLE":
				if found.subtitle == "" {
					found.subtitle = element.ObjectId
				}
			}
		}
	}

	return found
}

// slidePlaceholderIds names the placeholders this run will write to, and
// returns the mappings that bind those names at slide creation time.
type slidePlaceholderIds struct {
	title    string
	body     string
	subtitle string
}

func (b *slideBatch) mapPlaceholders(layout layoutPlaceholders) (slidePlaceholderIds, []*slides.LayoutPlaceholderIdMapping) {
	var ids slidePlaceholderIds
	var mappings []*slides.LayoutPlaceholderIdMapping

	bind := func(layoutObjectId, kind string) string {
		if layoutObjectId == "" {
			return ""
		}
		objectId := b.id(kind)
		mappings = append(mappings, &slides.LayoutPlaceholderIdMapping{
			LayoutPlaceholderObjectId: layoutObjectId,
			ObjectId:                  objectId,
		})
		return objectId
	}

	ids.title = bind(layout.title, "title")
	ids.body = bind(layout.body, "body")
	ids.subtitle = bind(layout.subtitle, "sub")

	return ids, mappings
}

// appendSlidesFromMarkdown appends the slides described by markdown to the end
// of the presentation and returns their IDs. It never deletes anything, so a
// caller replacing a deck can keep the old slides until the new ones are
// safely in place.
//
// On failure it returns the slides it did manage to create, so the caller can
// clean them up rather than leaving them stranded in the presentation.
func (mc *MarkdownConverter) appendSlidesFromMarkdown(markdown string) ([]string, error) {
	parsedSlides := mc.ParseMarkdown(markdown)

	// One fetch for the whole deck. The layouts carry the placeholder IDs every
	// slide needs, so nothing here has to be read back per slide.
	presentation, err := mc.client.GetPresentation(mc.presentationId)
	if err != nil {
		return nil, err
	}

	// Get the TITLE_AND_BODY layout ID
	layoutId, err := findLayoutId(presentation, "TITLE_AND_BODY")
	if err != nil {
		// Fallback to blank slides if layout not found
		log.Printf("[WARNING] Failed to get TITLE_AND_BODY layout: %v\n", err)
		layoutId = ""
	}

	// Get the TITLE layout ID for title slides (slides with only two headings)
	titleLayoutId, _ := findLayoutId(presentation, "TITLE")

	// Create all slides fresh
	// Get the TITLE_ONLY layout ID for slides with tables
	titleOnlyLayoutId, _ := findLayoutId(presentation, "TITLE_ONLY")

	bodyLayout := placeholdersForLayout(presentation, layoutId)
	titleLayout := placeholdersForLayout(presentation, titleLayoutId)
	titleOnlyLayout := placeholdersForLayout(presentation, titleOnlyLayoutId)

	// The TITLE layout's subtitle slot is a SUBTITLE placeholder in most themes
	// and a BODY one in the rest.
	if titleLayout.subtitle == "" {
		titleLayout.subtitle = titleLayout.body
		titleLayout.body = ""
	}

	batch := newSlideBatch(mc.client, mc.presentationId)

	for i, slide := range parsedSlides {
		// Check if slide contains tables or images (both need more space)
		hasTable := false
		hasImage := false
		for _, element := range slide.Content {
			if element.Type == "table" {
				hasTable = true
			}
			if element.Type == "image" {
				hasImage = true
			}
		}
		// Use TITLE_ONLY layout for slides with tables or images
		needsTitleOnlyLayout := hasTable || hasImage

		// Check if slide has only two headings (title slide pattern)
		// Title slides are detected when:
		// 1. A slide has exactly 2 headings with no other content (title + subtitle)
		// 2. The first slide (index 0) contains only headings (common for presentation title slides)
		// This provides better visual layout for title/section divider slides
		isTitleSlide := false
		if titleLayoutId != "" && !needsTitleOnlyLayout {
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

		// Choose layout based on content, naming the placeholders up front so the
		// same batch can fill them.
		slideId := batch.id("s")
		var placeholders slidePlaceholderIds
		var mappings []*slides.LayoutPlaceholderIdMapping
		var createLayoutId, layoutType string
		if isTitleSlide && titleLayoutId != "" {
			// Use TITLE layout for title slides
			createLayoutId = titleLayoutId
			placeholders, mappings = batch.mapPlaceholders(titleLayout)
			layoutType = "TITLE"
		} else if needsTitleOnlyLayout && titleOnlyLayoutId != "" {
			// Use TITLE_ONLY layout for slides with tables or images
			createLayoutId = titleOnlyLayoutId
			placeholders, mappings = batch.mapPlaceholders(titleOnlyLayout)
			layoutType = "TITLE_ONLY"
		} else if layoutId != "" {
			// Use TITLE_AND_BODY layout for regular slides
			createLayoutId = layoutId
			placeholders, mappings = batch.mapPlaceholders(bodyLayout)
			layoutType = "TITLE_AND_BODY"
		} else {
			// Fallback to blank slide
			layoutType = "BLANK"
		}

		batch.add(createSlideRequests(createLayoutId, -1, slideId, mappings)...)
		batch.addSlide(slideId)

		// Populate slide based on layout type and content
		switch layoutType {
		case "TITLE":
			// Special handling for title slides
			populateSlideWithTitleLayout(batch, placeholders, slide)
		case "TITLE_ONLY":
			// Special handling for slides with tables (TITLE_ONLY layout)
			populateSlideWithTableLayout(batch, slideId, placeholders, slide)
		case "TITLE_AND_BODY":
			// Regular TITLE_AND_BODY layout
			populateSlideWithLayout(batch, placeholders, slide)
		default:
			// Blank slide
			populateSlide(batch, slideId, slide)
		}

		// The rejected request can belong to any slide in the batch, not just
		// the one that filled it, so the error names the batch rather than
		// pointing at a slide that may be blameless.
		if err := batch.flushIfFull(); err != nil {
			return batch.createdSlideIds, fmt.Errorf(
				"a batch of requests covering slides up to %d was rejected: %w", i+1, err)
		}
	}

	if err := batch.flush(); err != nil {
		return batch.createdSlideIds, fmt.Errorf(
			"the final batch of requests for %d slides was rejected: %w", len(parsedSlides), err)
	}

	return batch.createdSlideIds, nil
}

// populateSlideWithLayout populates a slide that uses a predefined layout.
//
// The placeholders belong to a slide created in this same batch, so they are
// still empty; the delete-text calls the unbatched path made first would fail
// the whole batch rather than being ignorable.
func populateSlideWithLayout(b *slideBatch, placeholders slidePlaceholderIds, slide MarkdownSlide) {
	titlePlaceholderId, bodyPlaceholderId := placeholders.title, placeholders.body

	// Insert title if we have a title placeholder
	if titlePlaceholderId != "" && slide.Title != "" {
		b.add(insertTextInPlaceholderRequests(titlePlaceholderId, slide.Title)...)
	}

	// Insert body content if we have a body placeholder
	if bodyPlaceholderId != "" && len(slide.Content) > 0 {
		// Find the first heading (Level 2 or 3) to use as title if slide.Title is empty
		var slideTitle string
		var bodyText []string
		var codeRanges []codeRange

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
				codeRanges = append(codeRanges, codeRange{
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
			b.add(insertTextInPlaceholderRequests(titlePlaceholderId, slideTitle)...)
		}

		if len(bodyText) > 0 {
			combinedText := strings.Join(bodyText, "\n")
			b.add(insertTextInPlaceholderRequests(bodyPlaceholderId, combinedText)...)

			// Apply Courier New font to code blocks
			b.add(applyCodeFormattingRequests(bodyPlaceholderId, codeRanges)...)
		}
	}
}

// populateSlideWithTitleLayout populates a slide with TITLE layout (for title slides with only headings)
// This function is used for slides that contain only headings (typically 2: title and subtitle)
// It maps the headings to the appropriate title and subtitle placeholders in the TITLE layout
func populateSlideWithTitleLayout(b *slideBatch, placeholders slidePlaceholderIds, slide MarkdownSlide) {
	titlePlaceholderId, subtitlePlaceholderId := placeholders.title, placeholders.subtitle

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
		b.add(insertTextInPlaceholderRequests(titlePlaceholderId, titleText)...)
	}

	// Insert subtitle
	if subtitlePlaceholderId != "" && subtitleText != "" {
		b.add(insertTextInPlaceholderRequests(subtitlePlaceholderId, subtitleText)...)
	}
}

// populateSlideWithTableLayout populates a slide with TITLE_ONLY layout that contains tables or images
// This layout provides more space for content that needs it (tables, images)
func populateSlideWithTableLayout(b *slideBatch, slideId string, placeholders slidePlaceholderIds, slide MarkdownSlide) {
	titlePlaceholderId := placeholders.title

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
		b.add(insertTextInPlaceholderRequests(titlePlaceholderId, titleText)...)
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

			b.add(addTextBoxRequests(
				slideId,
				b.id("tb"),
				element.Content,
				MarginLeft,
				currentY,
				SlideWidth-MarginLeft-MarginRight,
				fontSize*LineHeight,
			)...)
			currentY += fontSize * LineHeight * 1.2

		case "bullet", "numbering":
			prefix := "• "
			if element.Type == "numbering" {
				prefix = "1. "
			}

			b.add(addTextBoxRequests(
				slideId,
				b.id("tb"),
				prefix+element.Content,
				MarginLeft,
				currentY,
				SlideWidth-MarginLeft-MarginRight,
				DefaultFontSize*LineHeight,
			)...)
			currentY += DefaultFontSize * LineHeight

		case "code":
			// Add code block with Courier New font
			b.add(addCodeTextBoxRequests(
				slideId,
				b.id("code"),
				element.Content,
				MarginLeft,
				currentY,
				SlideWidth-MarginLeft-MarginRight,
				100, // Fixed height for code blocks
			)...)
			currentY += 100 + 10

		case "image":
			currentY = addMarkdownImage(b, slideId, element, currentY)

		case "table":
			currentY = addMarkdownTable(b, slideId, element, currentY)
		}
	}
}

// addMarkdownImage places an image, and its alt text as a caption, and returns
// the vertical position the next element should start at.
func addMarkdownImage(b *slideBatch, slideId string, element MarkdownElement, currentY float64) float64 {
	// Add image at 50% of slide size
	imageWidth := SlideWidth * 0.5
	imageHeight := SlideHeight * 0.5
	imageX := (SlideWidth - imageWidth) / 2

	b.add(addImageRequests(slideId, b.id("img"), element.Content, imageX, currentY, imageWidth, imageHeight)...)
	currentY += imageHeight + 10

	// Add alt text as caption below image if present
	if element.AltText != "" {
		captionWidth := imageWidth
		b.add(addTextBoxRequests(
			slideId,
			b.id("tb"),
			element.AltText,
			imageX,
			currentY,
			captionWidth,
			DefaultFontSize*LineHeight,
		)...)
		currentY += DefaultFontSize*LineHeight + 10
	}

	return currentY
}

// addMarkdownTable places a table and fills every cell in the same batch, so a
// 6x4 table costs 25 requests rather than 25 round trips. It returns the
// vertical position the next element should start at.
func addMarkdownTable(b *slideBatch, slideId string, element MarkdownElement, currentY float64) float64 {
	if len(element.Items) == 0 {
		return currentY
	}

	rows := len(element.Items)
	cols := strings.Count(element.Items[0], "|") - 1
	if cols <= 0 {
		return currentY
	}

	tableWidth := SlideWidth - MarginLeft - MarginRight
	tableHeight := float64(rows) * 30.0
	tableId := b.id("tbl")

	b.add(addTableRequests(slideId, tableId, rows, cols, MarginLeft, currentY, tableWidth, tableHeight)...)

	// Populate table cells
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
				b.add(insertTextInTableCellRequests(tableId, rowIdx, colIdx, cellText)...)
			}
		}
	}

	return currentY + tableHeight + 10
}

func populateSlide(b *slideBatch, slideId string, slide MarkdownSlide) {
	// All slides are now blank, so we add text boxes for everything
	currentY := MarginTop

	// Add title if exists
	if slide.Title != "" {
		b.add(addTextBoxRequests(
			slideId,
			b.id("tb"),
			slide.Title,
			MarginLeft,
			currentY,
			SlideWidth-MarginLeft-MarginRight,
			TitleFontSize*LineHeight,
		)...)
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

			b.add(addTextBoxRequests(
				slideId,
				b.id("tb"),
				element.Content,
				MarginLeft,
				currentY,
				SlideWidth-MarginLeft-MarginRight,
				fontSize*LineHeight,
			)...)
			currentY += fontSize * LineHeight * 1.2

		case "bullet", "numbering":
			prefix := "• "
			if element.Type == "numbering" {
				prefix = "1. "
			}
			indent := float64(element.Level) * 20.0

			b.add(addTextBoxRequests(
				slideId,
				b.id("tb"),
				prefix+element.Content,
				MarginLeft+indent,
				currentY,
				SlideWidth-MarginLeft-MarginRight-indent,
				DefaultFontSize*LineHeight,
			)...)
			currentY += DefaultFontSize * LineHeight

		case "code":
			// Add code block with Courier New font
			b.add(addCodeTextBoxRequests(
				slideId,
				b.id("code"),
				element.Content,
				MarginLeft,
				currentY,
				SlideWidth-MarginLeft-MarginRight,
				100, // Fixed height for code blocks
			)...)
			currentY += 100 + 10

		case "image":
			currentY = addMarkdownImage(b, slideId, element, currentY)

		case "table":
			currentY = addMarkdownTable(b, slideId, element, currentY)
		}
	}
}

// UpdateSlidesFromMarkdown replaces the deck's contents with the slides
// described by markdown.
//
// The new slides are appended first and the previous ones are removed only once
// every new slide is in place. Deleting first, as this used to, meant that a
// failure partway through the rebuild — a rate limit on a large deck, most
// likely — left the user with an empty presentation and no way back.
func (mc *MarkdownConverter) UpdateSlidesFromMarkdown(markdown string) error {
	presentation, err := mc.client.GetPresentation(mc.presentationId)
	if err != nil {
		return err
	}

	obsoleteSlideIds := make([]string, 0, len(presentation.Slides))
	for _, slide := range presentation.Slides {
		obsoleteSlideIds = append(obsoleteSlideIds, slide.ObjectId)
	}

	createdSlideIds, err := mc.appendSlidesFromMarkdown(markdown)
	if err != nil {
		// Roll back the half-built replacement. Without this the user is left
		// with their original deck plus a run of partial slides, and each retry
		// strands another run.
		mc.removePartialSlides(createdSlideIds)

		return fmt.Errorf("failed to build the new slides, so the existing %d were left in place: %w",
			len(obsoleteSlideIds), err)
	}

	// Removing every slide would leave a presentation the API cannot represent,
	// and would discard the user's deck in exchange for nothing.
	if len(createdSlideIds) == 0 {
		return fmt.Errorf("markdown produced no slides, so the existing %d were left in place",
			len(obsoleteSlideIds))
	}

	if err := mc.deleteSlidesInBatches(obsoleteSlideIds); err != nil {
		return fmt.Errorf("created %d new slides but failed to remove the previous ones: %w",
			len(createdSlideIds), err)
	}

	return nil
}

// deleteSlidesInBatches removes slides with as few calls as the request cap
// allows. Deleting one slide per call would put the update straight back into
// rate limiting on exactly the decks this batching exists to serve: replacing a
// 45-slide deck would cost 45 calls to tear the old one down.
func (mc *MarkdownConverter) deleteSlidesInBatches(slideIds []string) error {
	for start := 0; start < len(slideIds); start += maxRequestsPerBatch {
		end := start + maxRequestsPerBatch
		if end > len(slideIds) {
			end = len(slideIds)
		}

		requests := make([]*slides.Request, 0, end-start)
		for _, slideId := range slideIds[start:end] {
			requests = append(requests, deleteSlideRequests(slideId)...)
		}

		if _, err := mc.client.BatchUpdate(mc.presentationId, requests); err != nil {
			return err
		}
	}

	return nil
}

// removePartialSlides deletes slides left behind by a failed rebuild. It is
// best effort: the caller is already returning the error that caused the
// rollback, and a cleanup failure should not replace it.
func (mc *MarkdownConverter) removePartialSlides(slideIds []string) {
	if len(slideIds) == 0 {
		return
	}

	if err := mc.deleteSlidesInBatches(slideIds); err != nil {
		log.Printf("[WARNING] Failed to remove %d partially built slide(s); they may need deleting by hand: %v\n",
			len(slideIds), err)
	}
}
