package slides

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

// buildMarkdownDeck returns markdown for n slides, each with a title and a
// couple of bullets.
func buildMarkdownDeck(n int) string {
	var sections []string
	for i := 1; i <= n; i++ {
		sections = append(sections, fmt.Sprintf("# Slide %d\n\n- first point\n- second point\n", i))
	}
	return strings.Join(sections, "\n---\n\n")
}

// TestDeckIsBuiltInOneBatch is the regression that motivated batching: a deck
// used to cost six to nine calls per slide, so a 45-slide deck ran head-first
// into the 60-requests-per-minute quota.
func TestDeckIsBuiltInOneBatch(t *testing.T) {
	const slideCount = 45

	mc, fake := newFakeConverter(t, 0)

	created, err := mc.appendSlidesFromMarkdown(buildMarkdownDeck(slideCount))
	if err != nil {
		t.Fatalf("appendSlidesFromMarkdown() error = %v", err)
	}

	if len(created) != slideCount {
		t.Fatalf("built %d slides, want %d", len(created), slideCount)
	}
	if fake.count("createSlide") != slideCount {
		t.Errorf("issued %d createSlide requests, want %d", fake.count("createSlide"), slideCount)
	}

	if fake.batches > 2 {
		t.Errorf("a %d-slide deck took %d batchUpdate calls, want at most 2; "+
			"the whole point of batching is that the deck is one round trip",
			slideCount, fake.batches)
	}

	// The layouts carry every placeholder ID the build needs, so one fetch is
	// enough. A fetch per slide is also O(n^2) in bytes as the deck grows.
	if got := fake.count("get"); got != 1 {
		t.Errorf("fetched the presentation %d times while building %d slides, want 1",
			got, slideCount)
	}
}

// TestBuildEmitsNoDeleteTextForFreshSlides guards a batching-specific hazard:
// placeholders on a slide created in the same batch are empty, and deleting
// text from an empty shape fails the entire batch rather than being ignorable
// the way it was when each call stood alone.
// TestUpdateOfExistingDeckStaysWithinAFewCalls measures the whole user-facing
// operation rather than construction alone. Building the new slides in one
// batch is only half of an update: tearing the old deck down one slide per call
// put a 45-slide replacement back at 46 calls against a 60/min quota, which is
// the situation reported in #5.
func TestUpdateOfExistingDeckStaysWithinAFewCalls(t *testing.T) {
	const slideCount = 45

	for _, existing := range []int{1, slideCount} {
		t.Run(fmt.Sprintf("replacing%dslides", existing), func(t *testing.T) {
			mc, fake := newFakeConverter(t, existing)

			if err := mc.UpdateSlidesFromMarkdown(buildMarkdownDeck(slideCount)); err != nil {
				t.Fatalf("UpdateSlidesFromMarkdown() error = %v", err)
			}

			if fake.batches > 2 {
				t.Errorf("replacing a %d-slide deck took %d batchUpdate calls, want at most 2; "+
					"deleting the old slides one call at a time undoes the batching",
					existing, fake.batches)
			}
			if got := fake.count("get"); got > 2 {
				t.Errorf("fetched the presentation %d times, want at most 2", got)
			}
			if got := len(fake.slideIDs()); got != slideCount {
				t.Errorf("deck ended with %d slides, want %d", got, slideCount)
			}
		})
	}
}

func TestBuildEmitsNoDeleteTextForFreshSlides(t *testing.T) {
	mc, fake := newFakeConverter(t, 0)

	if _, err := mc.appendSlidesFromMarkdown(buildMarkdownDeck(3)); err != nil {
		t.Fatalf("appendSlidesFromMarkdown() error = %v", err)
	}

	for _, kind := range fake.requests {
		if kind == "deleteText" {
			t.Fatal("the build deleted text from a placeholder it had just created; " +
				"in a batch that fails the whole deck")
		}
	}
}

// TestLayoutPathsProduceTheExpectedContent checks each layout path still puts
// the same text in the same kind of placeholder as the per-call version did.
func TestLayoutPathsProduceTheExpectedContent(t *testing.T) {
	tests := []struct {
		name     string
		markdown string
		check    func(t *testing.T, fake *fakeSlidesAPI)
	}{
		{
			name:     "TITLE",
			markdown: "# Deck title\n\n## A subtitle\n",
			check: func(t *testing.T, fake *fakeSlidesAPI) {
				if got := fake.placeholderText(0, "CENTERED_TITLE"); got != "Deck title" {
					t.Errorf("title placeholder holds %q, want %q", got, "Deck title")
				}
				if got := fake.placeholderText(0, "SUBTITLE"); got != "A subtitle" {
					t.Errorf("subtitle placeholder holds %q, want %q", got, "A subtitle")
				}
			},
		},
		{
			name:     "TITLE_AND_BODY",
			markdown: "# Heading\n\n- one\n- two\n",
			check: func(t *testing.T, fake *fakeSlidesAPI) {
				if got := fake.placeholderText(0, "TITLE"); got != "Heading" {
					t.Errorf("title placeholder holds %q, want %q", got, "Heading")
				}
				if got := fake.placeholderText(0, "BODY"); got != "• one\n• two" {
					t.Errorf("body placeholder holds %q, want %q", got, "• one\n• two")
				}
			},
		},
		{
			name:     "TITLE_ONLY with a table",
			markdown: "# Numbers\n\n| A | B |\n| --- | --- |\n| 1 | 2 |\n",
			check: func(t *testing.T, fake *fakeSlidesAPI) {
				if got := fake.placeholderText(0, "TITLE"); got != "Numbers" {
					t.Errorf("title placeholder holds %q, want %q", got, "Numbers")
				}
				if len(fake.tableIds) != 1 {
					t.Fatalf("created %d tables, want 1", len(fake.tableIds))
				}

				tableId := fake.tableIds[0]
				want := map[string]string{
					fmt.Sprintf("%s[0,0]", tableId): "A",
					fmt.Sprintf("%s[0,1]", tableId): "B",
					fmt.Sprintf("%s[1,0]", tableId): "1",
					fmt.Sprintf("%s[1,1]", tableId): "2",
				}
				for key, wantText := range want {
					if got := fake.text[key]; got != wantText {
						t.Errorf("cell %s holds %q, want %q", key, got, wantText)
					}
				}
			},
		},
		{
			name:     "TITLE_ONLY with an image",
			markdown: "# Picture\n\n![a caption](https://example.com/image.png)\n",
			check: func(t *testing.T, fake *fakeSlidesAPI) {
				if got := fake.placeholderText(0, "TITLE"); got != "Picture" {
					t.Errorf("title placeholder holds %q, want %q", got, "Picture")
				}
				if got := fake.count("createImage"); got != 1 {
					t.Errorf("issued %d createImage requests, want 1", got)
				}
				// The alt text becomes a caption text box under the image
				if got := fake.count("createShape"); got != 1 {
					t.Errorf("issued %d createShape requests, want 1 for the caption", got)
				}
			},
		},
		{
			name:     "code block in the body",
			markdown: "# Example\n\nSome prose\n\n```\nfmt.Println()\n```\n",
			check: func(t *testing.T, fake *fakeSlidesAPI) {
				if got := fake.placeholderText(0, "TITLE"); got != "Example" {
					t.Errorf("title placeholder holds %q, want %q", got, "Example")
				}
				body := fake.placeholderText(0, "BODY")
				if !strings.Contains(body, "fmt.Println()") {
					t.Errorf("body placeholder holds %q, want it to contain the code", body)
				}
				// The code range is restyled to Courier New in the same batch
				styledBody := false
				for _, styling := range fake.styled {
					if styling.fontFamily == "Courier New" {
						styledBody = true
					}
				}
				if !styledBody {
					t.Error("the code block was never restyled to Courier New")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mc, fake := newFakeConverter(t, 0)

			if _, err := mc.appendSlidesFromMarkdown(tt.markdown); err != nil {
				t.Fatalf("appendSlidesFromMarkdown() error = %v", err)
			}
			if len(fake.slides) != 1 {
				t.Fatalf("built %d slides, want 1", len(fake.slides))
			}

			tt.check(t, fake)
		})
	}
}

// TestBuildChunksLargeDecks covers the safety valve: past the flush threshold
// the deck is split across batches rather than sent as one enormous request.
func TestBuildChunksLargeDecks(t *testing.T) {
	const slideCount = 10

	// Each slide costs more than one request, so a threshold of 1 flushes at
	// every slide boundary
	withMaxRequestsPerBatch(t, 1)

	mc, fake := newFakeConverter(t, 0)

	created, err := mc.appendSlidesFromMarkdown(buildMarkdownDeck(slideCount))
	if err != nil {
		t.Fatalf("appendSlidesFromMarkdown() error = %v", err)
	}

	if len(created) != slideCount {
		t.Fatalf("built %d slides, want %d", len(created), slideCount)
	}
	if fake.batches != slideCount {
		t.Errorf("sent %d batches for %d slides at a threshold of one request per batch, want %d",
			fake.batches, slideCount, slideCount)
	}
}

// TestFailedBatchReportsNoCreatedSlides covers the flip side of atomic
// batches. Nothing a failed batch did survives, so reporting its slides as
// created would send the rollback chasing IDs that never existed while the
// caller believes work was done.
func TestFailedBatchReportsNoCreatedSlides(t *testing.T) {
	mc, fake := newFakeConverter(t, 0)
	fake.failCreateAfter = 2 // the batch dies on the third slide

	created, err := mc.appendSlidesFromMarkdown(buildMarkdownDeck(3))
	if err == nil {
		t.Fatal("expected an error when the batch fails")
	}

	if len(created) != 0 {
		t.Errorf("reported %d created slides from a batch that failed, want 0", len(created))
	}
	if len(fake.slides) != 0 {
		t.Errorf("the deck has %d slides after a failed batch, want 0; "+
			"a batch is applied atomically", len(fake.slides))
	}
}

// TestGeneratedObjectIdsAreValid pins the IDs to what the API accepts. A
// rejected ID fails the whole batch, and with everything in one batch that
// means the whole deck.
func TestGeneratedObjectIdsAreValid(t *testing.T) {
	// [a-zA-Z0-9_][a-zA-Z0-9_-:]{4,49}, as documented for object IDs
	valid := regexp.MustCompile(`^[a-zA-Z0-9_][a-zA-Z0-9_:-]{4,49}$`)

	markdown := strings.Join([]string{
		"# Deck title\n\n## A subtitle\n",
		"# Heading\n\n- one\n- two\n",
		"# Numbers\n\n| A | B |\n| --- | --- |\n| 1 | 2 |\n",
		"# Picture\n\n![a caption](https://example.com/image.png)\n",
		"# Example\n\nSome prose\n\n```\nfmt.Println()\n```\n",
	}, "\n---\n\n")

	mc, fake := newFakeConverter(t, 0)

	if _, err := mc.appendSlidesFromMarkdown(markdown); err != nil {
		t.Fatalf("appendSlidesFromMarkdown() error = %v", err)
	}

	if len(fake.objectIds) == 0 {
		t.Fatal("no caller-assigned object IDs were seen; the test is not checking anything")
	}

	seen := map[string]bool{}
	for _, id := range fake.objectIds {
		if !valid.MatchString(id) {
			t.Errorf("object ID %q is not accepted by the API", id)
		}
		if seen[id] {
			t.Errorf("object ID %q was assigned twice; the batch would be rejected", id)
		}
		seen[id] = true
	}
}
