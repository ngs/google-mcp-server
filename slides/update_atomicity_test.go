package slides

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"google.golang.org/api/slides/v1"
)

// fakeSlidesAPI is a minimal stand-in for the two Slides endpoints this package
// uses: presentations.get and presentations.batchUpdate. It keeps enough state
// that a deck can actually be built against it, and records the order of the
// requests so tests can assert on sequencing.
type fakeSlidesAPI struct {
	slides     []*slides.Page // current deck, in order
	requests   []string       // request kinds in the order they were issued
	nextID     int
	failCreate bool // fail any batch containing a createSlide
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
		Layouts: []*slides.Page{
			{ObjectId: "layout-title-body", LayoutProperties: &slides.LayoutProperties{Name: "TITLE_AND_BODY"}},
			{ObjectId: "layout-title", LayoutProperties: &slides.LayoutProperties{Name: "TITLE"}},
			{ObjectId: "layout-title-only", LayoutProperties: &slides.LayoutProperties{Name: "TITLE_ONLY"}},
		},
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

	replies := make([]*slides.Response, 0, len(parsed.Requests))
	for _, r := range parsed.Requests {
		switch {
		case r.CreateSlide != nil:
			f.requests = append(f.requests, "createSlide")
			if f.failCreate {
				return jsonResponse(http.StatusInternalServerError,
					map[string]any{"error": map[string]any{"code": 500, "message": "boom"}})
			}
			replies = append(replies, &slides.Response{CreateSlide: &slides.CreateSlideResponse{
				ObjectId: f.addSlide(),
			}})

		case r.DeleteObject != nil:
			f.requests = append(f.requests, "deleteObject")
			f.removeSlide(r.DeleteObject.ObjectId)
			replies = append(replies, &slides.Response{})

		default:
			// insertText, deleteText, formatting and so on: accepted as-is
			f.requests = append(f.requests, "other")
			replies = append(replies, &slides.Response{})
		}
	}

	return jsonResponse(http.StatusOK, &slides.BatchUpdatePresentationResponse{Replies: replies})
}

// addSlide appends a slide carrying the title and body placeholders the
// populate step looks for, and returns its ID.
func (f *fakeSlidesAPI) addSlide() string {
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
		fake.addSlide()
	}
	// Requests made while seeding are setup, not part of what we assert on
	fake.requests = nil

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

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }
