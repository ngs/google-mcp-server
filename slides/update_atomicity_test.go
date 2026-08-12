package slides

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"google.golang.org/api/slides/v1"
)

// fakeLayout describes one of the layouts the fake presentation offers,
// including the placeholders that layout produces. Slides created from a layout
// may only map the placeholders it actually defines, which is what forces the
// converter to read them from the layout rather than guess.
type fakeLayout struct {
	objectId     string
	name         string
	placeholders []fakePlaceholder
}

type fakePlaceholder struct {
	objectId string
	kind     string
}

var fakeLayouts = []fakeLayout{
	{objectId: "layout-title-body", name: "TITLE_AND_BODY", placeholders: []fakePlaceholder{
		{objectId: "layout-title-body-title", kind: "TITLE"},
		{objectId: "layout-title-body-body", kind: "BODY"},
	}},
	{objectId: "layout-title", name: "TITLE", placeholders: []fakePlaceholder{
		// Themes differ here; CENTERED_TITLE + SUBTITLE is the common shape.
		{objectId: "layout-title-title", kind: "CENTERED_TITLE"},
		{objectId: "layout-title-sub", kind: "SUBTITLE"},
	}},
	{objectId: "layout-title-only", name: "TITLE_ONLY", placeholders: []fakePlaceholder{
		{objectId: "layout-title-only-title", kind: "TITLE"},
	}},
}

func layoutPages() []*slides.Page {
	pages := make([]*slides.Page, 0, len(fakeLayouts))
	for _, layout := range fakeLayouts {
		elements := make([]*slides.PageElement, 0, len(layout.placeholders))
		for _, ph := range layout.placeholders {
			elements = append(elements, &slides.PageElement{
				ObjectId: ph.objectId,
				Shape:    &slides.Shape{Placeholder: &slides.Placeholder{Type: ph.kind}},
			})
		}
		pages = append(pages, &slides.Page{
			ObjectId:         layout.objectId,
			LayoutProperties: &slides.LayoutProperties{Name: layout.name},
			PageElements:     elements,
		})
	}
	return pages
}

func findFakeLayout(objectId string) (fakeLayout, bool) {
	for _, layout := range fakeLayouts {
		if layout.objectId == objectId {
			return layout, true
		}
	}
	return fakeLayout{}, false
}

// textStyling records an UpdateTextStyle the fake accepted.
type textStyling struct {
	objectId   string
	fontFamily string
}

// fakeSlidesAPI is a minimal stand-in for the two Slides endpoints this package
// uses: presentations.get and presentations.batchUpdate. It keeps enough state
// that a deck can actually be built against it, and records the order of the
// requests so tests can assert on sequencing.
type fakeSlidesAPI struct {
	slides   []*slides.Page // current deck, in order
	requests []string       // request kinds in the order they were issued
	batches  int            // batchUpdate calls, successful or not
	// text holds what landed in each shape, keyed by object ID and, for table
	// cells, by "tableId[row,col]".
	text map[string]string
	// objectIds are the IDs the caller assigned, in the order they were used
	objectIds  []string
	tableIds   []string
	styled     []textStyling
	nextID     int
	failCreate bool // fail any batch containing a createSlide
	// failCreateAfter, when non-zero, lets that many slides be created before
	// failing, so tests can exercise a rebuild that dies partway through
	failCreateAfter int
}

func (f *fakeSlidesAPI) RoundTrip(req *http.Request) (*http.Response, error) {
	if strings.Contains(req.URL.Path, ":batchUpdate") {
		return f.handleBatchUpdate(req)
	}
	return f.handleGet()
}

func (f *fakeSlidesAPI) handleGet() (*http.Response, error) {
	f.requests = append(f.requests, "get")

	return jsonResponse(http.StatusOK, &slides.Presentation{
		PresentationId: "test-presentation",
		Slides:         f.slides,
		Layouts:        layoutPages(),
	})
}

func (f *fakeSlidesAPI) handleBatchUpdate(req *http.Request) (*http.Response, error) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}

	var parsed slides.BatchUpdatePresentationRequest
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}

	f.batches++

	// batchUpdate is atomic: a batch that fails partway leaves nothing behind,
	// so the fake has to undo whatever this batch had already applied.
	slidesBefore := append([]*slides.Page(nil), f.slides...)
	textBefore := maps.Clone(f.text)
	objectIdsBefore := append([]string(nil), f.objectIds...)
	tableIdsBefore := append([]string(nil), f.tableIds...)
	styledBefore := append([]textStyling(nil), f.styled...)
	fail := func(status int, message string) (*http.Response, error) {
		f.slides = slidesBefore
		f.text = textBefore
		f.objectIds = objectIdsBefore
		f.tableIds = tableIdsBefore
		f.styled = styledBefore
		return jsonResponse(status,
			map[string]any{"error": map[string]any{"code": status, "message": message}})
	}

	replies := make([]*slides.Response, 0, len(parsed.Requests))
	for _, r := range parsed.Requests {
		switch {
		case r.CreateSlide != nil:
			f.requests = append(f.requests, "createSlide")
			if f.failCreateAfter > 0 && f.count("createSlide") > f.failCreateAfter {
				return fail(http.StatusInternalServerError, "boom")
			}
			if f.failCreate {
				return fail(http.StatusInternalServerError, "boom")
			}
			id, err := f.addSlideFromRequest(r.CreateSlide)
			if err != nil {
				return fail(http.StatusBadRequest, err.Error())
			}
			replies = append(replies, &slides.Response{CreateSlide: &slides.CreateSlideResponse{ObjectId: id}})

		case r.DeleteObject != nil:
			f.requests = append(f.requests, "deleteObject")
			f.removeSlide(r.DeleteObject.ObjectId)
			replies = append(replies, &slides.Response{})

		case r.CreateTable != nil:
			f.requests = append(f.requests, "createTable")
			f.recordObjectId(r.CreateTable.ObjectId)
			f.tableIds = append(f.tableIds, r.CreateTable.ObjectId)
			replies = append(replies, &slides.Response{CreateTable: &slides.CreateTableResponse{
				ObjectId: r.CreateTable.ObjectId,
			}})

		case r.CreateImage != nil:
			f.requests = append(f.requests, "createImage")
			f.recordObjectId(r.CreateImage.ObjectId)
			replies = append(replies, &slides.Response{CreateImage: &slides.CreateImageResponse{
				ObjectId: r.CreateImage.ObjectId,
			}})

		case r.CreateShape != nil:
			f.requests = append(f.requests, "createShape")
			f.recordObjectId(r.CreateShape.ObjectId)
			replies = append(replies, &slides.Response{CreateShape: &slides.CreateShapeResponse{
				ObjectId: r.CreateShape.ObjectId,
			}})

		case r.InsertText != nil:
			f.requests = append(f.requests, "insertText")
			f.recordText(r.InsertText)
			replies = append(replies, &slides.Response{})

		case r.DeleteText != nil:
			f.requests = append(f.requests, "deleteText")
			replies = append(replies, &slides.Response{})

		case r.UpdateTextStyle != nil:
			f.requests = append(f.requests, "updateTextStyle")
			styling := textStyling{objectId: r.UpdateTextStyle.ObjectId}
			if r.UpdateTextStyle.Style != nil {
				styling.fontFamily = r.UpdateTextStyle.Style.FontFamily
			}
			f.styled = append(f.styled, styling)
			replies = append(replies, &slides.Response{})

		default:
			f.requests = append(f.requests, "other")
			replies = append(replies, &slides.Response{})
		}
	}

	return jsonResponse(http.StatusOK, &slides.BatchUpdatePresentationResponse{Replies: replies})
}

func (f *fakeSlidesAPI) recordObjectId(id string) {
	if id != "" {
		f.objectIds = append(f.objectIds, id)
	}
}

func (f *fakeSlidesAPI) recordText(req *slides.InsertTextRequest) {
	if f.text == nil {
		f.text = map[string]string{}
	}
	key := req.ObjectId
	if req.CellLocation != nil {
		key = fmt.Sprintf("%s[%d,%d]", req.ObjectId, req.CellLocation.RowIndex, req.CellLocation.ColumnIndex)
	}
	f.text[key] = req.Text
}

// addSlideFromRequest honours the caller-supplied object ID and placeholder
// mappings, rejecting a mapping the layout does not define exactly as the real
// API does.
func (f *fakeSlidesAPI) addSlideFromRequest(req *slides.CreateSlideRequest) (string, error) {
	id := req.ObjectId
	if id == "" {
		f.nextID++
		id = "slide-" + strconv.Itoa(f.nextID)
	}
	f.recordObjectId(req.ObjectId)

	var layout fakeLayout
	if req.SlideLayoutReference != nil {
		found, ok := findFakeLayout(req.SlideLayoutReference.LayoutId)
		if !ok {
			return "", fmt.Errorf("unknown layout %q", req.SlideLayoutReference.LayoutId)
		}
		layout = found
	}

	mapped := map[string]string{}
	for _, m := range req.PlaceholderIdMappings {
		kind := ""
		for _, ph := range layout.placeholders {
			if ph.objectId == m.LayoutPlaceholderObjectId {
				kind = ph.kind
				break
			}
		}
		if kind == "" {
			return "", fmt.Errorf("layout %q has no placeholder %q", layout.name, m.LayoutPlaceholderObjectId)
		}
		f.recordObjectId(m.ObjectId)
		mapped[m.LayoutPlaceholderObjectId] = m.ObjectId
	}

	elements := make([]*slides.PageElement, 0, len(layout.placeholders))
	for _, ph := range layout.placeholders {
		objectId, ok := mapped[ph.objectId]
		if !ok {
			objectId = id + "-" + strings.ToLower(ph.kind)
		}
		elements = append(elements, &slides.PageElement{
			ObjectId: objectId,
			Shape:    &slides.Shape{Placeholder: &slides.Placeholder{Type: ph.kind}},
		})
	}

	f.slides = append(f.slides, &slides.Page{ObjectId: id, PageElements: elements})

	return id, nil
}

// seedSlide adds a pre-existing slide to the deck, standing in for one the user
// already had.
func (f *fakeSlidesAPI) seedSlide() string {
	f.nextID++
	id := "slide-" + strconv.Itoa(f.nextID)

	f.slides = append(f.slides, &slides.Page{
		ObjectId: id,
		PageElements: []*slides.PageElement{
			{ObjectId: id + "-title", Shape: &slides.Shape{Placeholder: &slides.Placeholder{Type: "TITLE"}}},
			{ObjectId: id + "-body", Shape: &slides.Shape{Placeholder: &slides.Placeholder{Type: "BODY"}}},
		},
	})

	return id
}

// placeholderText returns the text inserted into the placeholder of the given
// kind on the nth slide (0-based) of the deck.
func (f *fakeSlidesAPI) placeholderText(slideIndex int, kind string) string {
	if slideIndex >= len(f.slides) {
		return ""
	}
	for _, element := range f.slides[slideIndex].PageElements {
		if element.Shape != nil && element.Shape.Placeholder != nil && element.Shape.Placeholder.Type == kind {
			return f.text[element.ObjectId]
		}
	}
	return ""
}

func (f *fakeSlidesAPI) removeSlide(id string) {
	kept := f.slides[:0]
	for _, s := range f.slides {
		if s.ObjectId != id {
			kept = append(kept, s)
		}
	}
	f.slides = kept
}

func (f *fakeSlidesAPI) slideIDs() []string {
	ids := make([]string, 0, len(f.slides))
	for _, s := range f.slides {
		ids = append(ids, s.ObjectId)
	}
	return ids
}

func (f *fakeSlidesAPI) count(kind string) int {
	n := 0
	for _, r := range f.requests {
		if r == kind {
			n++
		}
	}
	return n
}

func jsonResponse(status int, payload any) (*http.Response, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(string(encoded))),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}, nil
}

// newFakeConverter wires a MarkdownConverter to a fake API pre-loaded with the
// given number of existing slides.
func newFakeConverter(t *testing.T, existingSlides int) (*MarkdownConverter, *fakeSlidesAPI) {
	t.Helper()

	fake := &fakeSlidesAPI{}
	for i := 0; i < existingSlides; i++ {
		fake.seedSlide()
	}
	// Requests made while seeding are setup, not part of what we assert on
	fake.requests = nil
	fake.batches = 0

	client, err := NewClient(context.Background(), &http.Client{Transport: fake})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	return NewMarkdownConverter(client, "test-presentation"), fake
}

const twoSlideMarkdown = "# First\n\nBody one\n\n---\n\n# Second\n\nBody two\n"

// TestUpdateCreatesBeforeDeleting pins the ordering: every new slide must exist
// before any old slide is removed, so an interrupted rebuild never empties the
// deck.
func TestUpdateCreatesBeforeDeleting(t *testing.T) {
	mc, fake := newFakeConverter(t, 3)
	before := fake.slideIDs()

	if err := mc.UpdateSlidesFromMarkdown(twoSlideMarkdown); err != nil {
		t.Fatalf("UpdateSlidesFromMarkdown() error = %v", err)
	}

	lastCreate, firstDelete := -1, -1
	for i, r := range fake.requests {
		if r == "createSlide" {
			lastCreate = i
		}
		if r == "deleteObject" && firstDelete == -1 {
			firstDelete = i
		}
	}

	if lastCreate == -1 || firstDelete == -1 {
		t.Fatalf("expected both creates and deletes, got %v", fake.requests)
	}
	if firstDelete < lastCreate {
		t.Errorf("a slide was deleted at step %d before the last slide was created at step %d; "+
			"an interrupted rebuild would lose the deck", firstDelete, lastCreate)
	}

	// The old slides are gone and only the new ones remain
	for _, old := range before {
		for _, remaining := range fake.slideIDs() {
			if old == remaining {
				t.Errorf("old slide %s survived the update", old)
			}
		}
	}
	if got := len(fake.slideIDs()); got != 2 {
		t.Errorf("deck has %d slides after update, want 2", got)
	}
}

// TestUpdateKeepsExistingSlidesWhenCreationFails is the data-loss regression:
// the old implementation deleted first, so a failure here left nothing behind.
func TestUpdateKeepsExistingSlidesWhenCreationFails(t *testing.T) {
	mc, fake := newFakeConverter(t, 3)
	before := fake.slideIDs()
	fake.failCreate = true

	err := mc.UpdateSlidesFromMarkdown(twoSlideMarkdown)
	if err == nil {
		t.Fatal("expected an error when slide creation fails")
	}

	if n := fake.count("deleteObject"); n != 0 {
		t.Errorf("%d slides were deleted despite creation failing; the deck must be left intact", n)
	}
	if got := fake.slideIDs(); len(got) != len(before) {
		t.Errorf("deck has %d slides after a failed update, want the original %d", len(got), len(before))
	}
}

// TestUpdateRemovesPartiallyBuiltSlidesOnFailure covers a rebuild that dies
// after some slides already exist. A single batch is atomic, so the only way to
// strand slides now is to fail on a later chunk: the earlier chunks are already
// committed. Keeping the original deck is not enough on its own — the
// half-built replacement has to go too, or the user is left with both decks
// interleaved and every retry strands another run of orphans.
func TestUpdateRemovesPartiallyBuiltSlidesOnFailure(t *testing.T) {
	mc, fake := newFakeConverter(t, 3)
	before := fake.slideIDs()

	// Force one slide per chunk so the failure lands after a committed chunk
	withMaxRequestsPerBatch(t, 1)
	fake.failCreateAfter = 1 // the first chunk succeeds, the second does not

	if err := mc.UpdateSlidesFromMarkdown(twoSlideMarkdown); err == nil {
		t.Fatal("expected an error when the rebuild fails partway")
	}

	if fake.batches < 2 {
		t.Fatalf("the deck was sent in %d batches; the test only means anything "+
			"if the failure lands after an earlier chunk committed", fake.batches)
	}

	if got := fake.slideIDs(); len(got) != len(before) {
		t.Errorf("deck has %d slides after a partial failure, want the original %d; "+
			"orphaned slides from the failed rebuild were left behind", len(got), len(before))
	}
	for i, id := range fake.slideIDs() {
		if i < len(before) && id != before[i] {
			t.Errorf("slide %d is %s, want the original %s", i, id, before[i])
		}
	}

	// Retrying must not accumulate orphans either
	fake.failCreateAfter = 1
	fake.requests = nil
	if err := mc.UpdateSlidesFromMarkdown(twoSlideMarkdown); err == nil {
		t.Fatal("expected an error on the retry as well")
	}
	if got := fake.slideIDs(); len(got) != len(before) {
		t.Errorf("deck grew to %d slides after retrying a failing update, want %d", len(got), len(before))
	}
}

// TestUpdateRefusesToEmptyTheDeck covers markdown that yields no slides, which
// would otherwise delete everything and put nothing back.
func TestUpdateRefusesToEmptyTheDeck(t *testing.T) {
	mc, fake := newFakeConverter(t, 3)

	err := mc.UpdateSlidesFromMarkdown("   \n\n")
	if err == nil {
		t.Fatal("expected an error when the markdown produces no slides")
	}

	if n := fake.count("deleteObject"); n != 0 {
		t.Errorf("%d slides were deleted for markdown that produced none", n)
	}
	if got := len(fake.slideIDs()); got != 3 {
		t.Errorf("deck has %d slides, want the original 3", got)
	}
}

// TestBatchUpdateRetriesAreNotNested guards the retry budget. BatchUpdate used
// to wrap doWithRetry in doWithRetry, turning maxRetries into maxRetries^2 —
// up to 100 attempts against an API that was already rate limiting.
func TestBatchUpdateRetriesAreNotNested(t *testing.T) {
	restoreInitial, restoreRateLimit := initialBackoff, rateLimitBackoff
	initialBackoff, rateLimitBackoff = time.Millisecond, time.Millisecond
	t.Cleanup(func() { initialBackoff, rateLimitBackoff = restoreInitial, restoreRateLimit })

	attempts := 0
	transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
		attempts++
		return jsonResponse(http.StatusTooManyRequests,
			map[string]any{"error": map[string]any{"code": 429, "message": "rateLimitExceeded"}})
	})

	client, err := NewClient(context.Background(), &http.Client{Transport: transport})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	if _, err := client.BatchUpdate("test-presentation", []*slides.Request{{}}); err == nil {
		t.Fatal("expected an error when the API keeps returning 429")
	}

	if attempts != maxRetries {
		t.Errorf("BatchUpdate made %d attempts, want %d; nested retries multiply the budget "+
			"and hammer an API that is already rate limiting", attempts, maxRetries)
	}
}

// withMaxRequestsPerBatch lowers the flush threshold for the duration of a
// test, so chunking can be exercised without building an enormous deck.
func withMaxRequestsPerBatch(t *testing.T, n int) {
	t.Helper()

	restore := maxRequestsPerBatch
	maxRequestsPerBatch = n
	t.Cleanup(func() { maxRequestsPerBatch = restore })
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }
