package slides

import (
	"regexp"
	"strings"
	"testing"

	"google.golang.org/api/slides/v1"
)

// fakePresentation is a presentation carrying only the fake layouts, which is
// all the layout resolution code looks at.
func fakePresentation() *slides.Presentation {
	return &slides.Presentation{
		PresentationId: "test-presentation",
		Title:          "Test deck",
		Layouts:        layoutPages(),
		Masters: []*slides.Page{
			{ObjectId: "master-1", MasterProperties: &slides.MasterProperties{DisplayName: "Simple light"}},
		},
	}
}

func TestResolveLayoutByObjectId(t *testing.T) {
	layout, err := resolveLayout(fakePresentation(), "layout-title-body", "")
	if err != nil {
		t.Fatalf("resolveLayout returned an error: %v", err)
	}
	if layout.ObjectId != "layout-title-body" {
		t.Errorf("resolved %q, want layout-title-body", layout.ObjectId)
	}
}

func TestResolveLayoutByName(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"api name", "TITLE_AND_BODY"},
		{"display name", "Title and body"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			layout, err := resolveLayout(fakePresentation(), "", tt.input)
			if err != nil {
				t.Fatalf("resolveLayout(%q) returned an error: %v", tt.input, err)
			}
			if layout.ObjectId != "layout-title-body" {
				t.Errorf("resolved %q, want layout-title-body", layout.ObjectId)
			}
		})
	}
}

func TestResolveLayoutErrors(t *testing.T) {
	tests := []struct {
		name       string
		layoutId   string
		layoutName string
	}{
		{"unknown object id", "nope", ""},
		{"unknown name", "", "Nonexistent"},
		{"case does not match", "", "title and body"},
		{"both given", "layout-title-body", "TITLE_AND_BODY"},
		{"neither given", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := resolveLayout(fakePresentation(), tt.layoutId, tt.layoutName); err == nil {
				t.Error("resolveLayout should have returned an error")
			}
		})
	}
}

// TestResolveLayoutNeverFallsBack is the regression test for the behaviour of
// findLayoutId, which quietly returns the first layout when a name does not
// match. Filling a template with the wrong layout is worse than failing.
func TestResolveLayoutNeverFallsBack(t *testing.T) {
	presentation := fakePresentation()

	layout, err := resolveLayout(presentation, "", "Nonexistent")
	if err == nil {
		t.Fatalf("resolveLayout should have failed, got layout %q", layout.ObjectId)
	}
	if layout != nil {
		t.Errorf("no layout should be returned on failure, got %q", layout.ObjectId)
	}
	// The error has to help: it names what the deck does offer
	for _, want := range []string{"TITLE_AND_BODY", "Title and body"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error should list available layouts, got %q", err.Error())
		}
	}
}

func TestLayoutPlaceholdersOf(t *testing.T) {
	presentation := fakePresentation()
	layout, err := resolveLayout(presentation, "layout-two-columns", "")
	if err != nil {
		t.Fatalf("resolveLayout returned an error: %v", err)
	}

	available := layoutPlaceholdersOf(layout)
	if len(available) != 3 {
		t.Fatalf("got %d placeholders, want 3: %+v", len(available), available)
	}

	var bodies []int64
	for _, ph := range available {
		if ph.kind == "BODY" {
			bodies = append(bodies, ph.index)
		}
	}
	if len(bodies) != 2 || bodies[0] == bodies[1] {
		t.Errorf("the two BODY placeholders should have distinct indices, got %v", bodies)
	}
}

func TestFindPlaceholder(t *testing.T) {
	tests := []struct {
		name     string
		layout   string
		kind     string
		index    int64
		wantId   string
		wantFail bool
	}{
		{name: "title by default index", layout: "layout-title-body", kind: "TITLE", index: 0, wantId: "layout-title-body-title"},
		{name: "body index zero", layout: "layout-title-body", kind: "BODY", index: 0, wantId: "layout-title-body-body"},
		{name: "body index one is absent", layout: "layout-title-body", kind: "BODY", index: 1, wantFail: true},
		{name: "second body of two columns", layout: "layout-two-columns", kind: "BODY", index: 1, wantId: "layout-two-columns-right"},
		{name: "title only has no body", layout: "layout-title-only", kind: "BODY", index: 0, wantFail: true},
		{name: "centered title", layout: "layout-title", kind: "CENTERED_TITLE", index: 0, wantId: "layout-title-title"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			layout, err := resolveLayout(fakePresentation(), tt.layout, "")
			if err != nil {
				t.Fatalf("resolveLayout returned an error: %v", err)
			}

			found, err := findPlaceholder(layoutPlaceholdersOf(layout), tt.kind, tt.index)
			if tt.wantFail {
				if err == nil {
					t.Fatalf("findPlaceholder should have failed, got %q", found.objectId)
				}
				// The error names what the layout actually has
				if !strings.Contains(err.Error(), "TITLE") {
					t.Errorf("error should list available placeholders, got %q", err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("findPlaceholder returned an error: %v", err)
			}
			if found.objectId != tt.wantId {
				t.Errorf("resolved %q, want %q", found.objectId, tt.wantId)
			}
		})
	}
}

// requestKinds names the request in each entry, so ordering can be asserted.
func requestKinds(requests []*slides.Request) []string {
	kinds := make([]string, 0, len(requests))
	for _, r := range requests {
		switch {
		case r.CreateSlide != nil:
			kinds = append(kinds, "createSlide")
		case r.InsertText != nil:
			kinds = append(kinds, "insertText")
		case r.CreateParagraphBullets != nil:
			kinds = append(kinds, "createParagraphBullets")
		case r.ReplaceAllText != nil:
			kinds = append(kinds, "replaceAllText")
		case r.UpdateSlidesPosition != nil:
			kinds = append(kinds, "updateSlidesPosition")
		default:
			kinds = append(kinds, "other")
		}
	}
	return kinds
}

func titleAndBodyFills() []placeholderFill {
	return []placeholderFill{
		{kind: "TITLE", index: 0, text: "Quarterly review", objectId: "run-title"},
		{kind: "BODY", index: 0, text: "First\nSecond", objectId: "run-body"},
	}
}

func titleAndBodyMappings() []*slides.LayoutPlaceholderIdMapping {
	return []*slides.LayoutPlaceholderIdMapping{
		{LayoutPlaceholderObjectId: "layout-title-body-title", ObjectId: "run-title"},
		{LayoutPlaceholderObjectId: "layout-title-body-body", ObjectId: "run-body"},
	}
}

func TestCreateSlideFromLayoutRequestsOrder(t *testing.T) {
	requests := createSlideFromLayoutRequests("layout-title-body", "run-slide", nil,
		titleAndBodyFills(), titleAndBodyMappings())

	got := requestKinds(requests)
	want := []string{"createSlide", "insertText", "insertText"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("request order = %v, want %v", got, want)
	}
}

// TestCreateSlideFromLayoutRequestsBulletsFollowText pins the ordering the API
// needs: bullets apply to text that is already there.
func TestCreateSlideFromLayoutRequestsBulletsFollowText(t *testing.T) {
	fills := titleAndBodyFills()
	fills[1].bullets = true
	fills[1].bulletPreset = "BULLET_DISC_CIRCLE_SQUARE"

	requests := createSlideFromLayoutRequests("layout-title-body", "run-slide", nil, fills, titleAndBodyMappings())

	got := requestKinds(requests)
	want := []string{"createSlide", "insertText", "insertText", "createParagraphBullets"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("request order = %v, want %v", got, want)
	}

	bullets := requests[len(requests)-1].CreateParagraphBullets
	if bullets.ObjectId != "run-body" {
		t.Errorf("bullets applied to %q, want run-body", bullets.ObjectId)
	}
	if bullets.TextRange == nil || bullets.TextRange.Type != "ALL" {
		t.Errorf("bullets should cover the whole text range, got %+v", bullets.TextRange)
	}
	if bullets.BulletPreset != "BULLET_DISC_CIRCLE_SQUARE" {
		t.Errorf("bullet preset = %q", bullets.BulletPreset)
	}
}

// TestCreateSlideFromLayoutRequestsInsertionIndexZero covers the bug in
// createSlideRequests: index 0 is a zero value and is dropped from the request
// unless it is forced, so inserting at the front silently appends instead.
func TestCreateSlideFromLayoutRequestsInsertionIndexZero(t *testing.T) {
	var zero int64
	requests := createSlideFromLayoutRequests("layout-title-body", "run-slide", &zero,
		titleAndBodyFills(), titleAndBodyMappings())

	create := requests[0].CreateSlide
	if create.InsertionIndex != 0 {
		t.Errorf("insertion index = %d, want 0", create.InsertionIndex)
	}
	forced := false
	for _, field := range create.ForceSendFields {
		if field == "InsertionIndex" {
			forced = true
		}
	}
	if !forced {
		t.Errorf("InsertionIndex must be forced so that 0 is sent, got %v", create.ForceSendFields)
	}
}

func TestCreateSlideFromLayoutRequestsWithoutInsertionIndex(t *testing.T) {
	requests := createSlideFromLayoutRequests("layout-title-body", "run-slide", nil,
		titleAndBodyFills(), titleAndBodyMappings())

	create := requests[0].CreateSlide
	if create.InsertionIndex != 0 {
		t.Errorf("insertion index = %d, want the unset zero value", create.InsertionIndex)
	}
	for _, field := range create.ForceSendFields {
		if field == "InsertionIndex" {
			t.Errorf("InsertionIndex should not be forced when appending, got %v", create.ForceSendFields)
		}
	}
}

func TestCreateSlideFromLayoutRequestsMapsLayoutObjectIds(t *testing.T) {
	requests := createSlideFromLayoutRequests("layout-title-body", "run-slide", nil,
		titleAndBodyFills(), titleAndBodyMappings())

	create := requests[0].CreateSlide
	if create.SlideLayoutReference == nil || create.SlideLayoutReference.LayoutId != "layout-title-body" {
		t.Fatalf("layout reference = %+v", create.SlideLayoutReference)
	}
	if len(create.PlaceholderIdMappings) != 2 {
		t.Fatalf("got %d mappings, want 2", len(create.PlaceholderIdMappings))
	}

	// The mapping has to point at a placeholder the layout really defines,
	// otherwise the API rejects the whole batch
	layoutIds := map[string]bool{}
	for _, ph := range layoutPlaceholdersOf(mustLayout(t, "layout-title-body")) {
		layoutIds[ph.objectId] = true
	}
	for _, mapping := range create.PlaceholderIdMappings {
		if !layoutIds[mapping.LayoutPlaceholderObjectId] {
			t.Errorf("mapping references %q, which the layout does not define", mapping.LayoutPlaceholderObjectId)
		}
	}
}

func mustLayout(t *testing.T, objectId string) *slides.Page {
	t.Helper()
	layout, err := resolveLayout(fakePresentation(), objectId, "")
	if err != nil {
		t.Fatalf("resolveLayout returned an error: %v", err)
	}
	return layout
}

var objectIdPattern = regexp.MustCompile(`^[a-zA-Z0-9_][a-zA-Z0-9_:-]{4,49}$`)

func TestGeneratedObjectIdsAreAcceptable(t *testing.T) {
	prefix := newTemplateObjectIdPrefix()
	for _, id := range []string{prefix + "-slide", prefix + "-ph1", prefix + "-ph12"} {
		if !objectIdPattern.MatchString(id) {
			t.Errorf("generated id %q does not match the API's accepted pattern", id)
		}
	}
}

// TestCreateSlideFromLayoutRequestsOnlyAllowedKinds is the guarantee of the
// whole feature: the template's design is left alone, so nothing that moves or
// restyles an element may ever be emitted.
func TestCreateSlideFromLayoutRequestsOnlyAllowedKinds(t *testing.T) {
	fills := titleAndBodyFills()
	fills[1].bullets = true

	cases := map[string][]*slides.Request{
		"append":            createSlideFromLayoutRequests("layout-title-body", "run-slide", nil, titleAndBodyFills(), titleAndBodyMappings()),
		"insert at front":   createSlideFromLayoutRequests("layout-title-body", "run-slide", int64Ptr(0), titleAndBodyFills(), titleAndBodyMappings()),
		"with bullets":      createSlideFromLayoutRequests("layout-title-body", "run-slide", nil, fills, titleAndBodyMappings()),
		"replace all text":  replaceAllTextRequests([]textReplacement{{find: "{{title}}", replace: "Hello"}}, nil),
		"reorder":           updateSlidesPositionRequests([]string{"slide-1"}, 0),
		"no placeholders":   createSlideFromLayoutRequests("layout-title-only", "run-slide", nil, nil, nil),
		"scoped replace":    replaceAllTextRequests([]textReplacement{{find: "x", replace: "y", matchCase: true}}, []string{"slide-1"}),
		"reorder to middle": updateSlidesPositionRequests([]string{"slide-1", "slide-2"}, 3),
	}

	for name, requests := range cases {
		t.Run(name, func(t *testing.T) {
			for i, r := range requests {
				if r.UpdateTextStyle != nil {
					t.Errorf("request %d carries UpdateTextStyle", i)
				}
				if r.UpdateShapeProperties != nil {
					t.Errorf("request %d carries UpdateShapeProperties", i)
				}
				if r.UpdatePageElementTransform != nil {
					t.Errorf("request %d carries UpdatePageElementTransform", i)
				}
				if r.CreateShape != nil {
					t.Errorf("request %d carries CreateShape", i)
				}
				if r.CreateImage != nil {
					t.Errorf("request %d carries CreateImage", i)
				}
				if r.CreateTable != nil {
					t.Errorf("request %d carries CreateTable", i)
				}
				if r.DeleteObject != nil {
					t.Errorf("request %d carries DeleteObject", i)
				}
				if r.DeleteText != nil {
					t.Errorf("request %d carries DeleteText", i)
				}
				if r.UpdateParagraphStyle != nil {
					t.Errorf("request %d carries UpdateParagraphStyle", i)
				}
			}
		})
	}
}

func int64Ptr(v int64) *int64 { return &v }

// TestCreateSlideFromLayoutRequestsTextIsVerbatim separates this path from the
// Markdown converter: the text is inserted exactly as given, markers and all,
// because interpreting them would mean restyling the template.
func TestCreateSlideFromLayoutRequestsTextIsVerbatim(t *testing.T) {
	raw := "**bold** and _italic_ and `code`"
	fills := []placeholderFill{{kind: "BODY", index: 0, text: raw, objectId: "run-body"}}

	requests := createSlideFromLayoutRequests("layout-title-body", "run-slide", nil, fills,
		[]*slides.LayoutPlaceholderIdMapping{{LayoutPlaceholderObjectId: "layout-title-body-body", ObjectId: "run-body"}})

	var inserted string
	for _, r := range requests {
		if r.InsertText != nil {
			inserted = r.InsertText.Text
		}
	}
	if inserted != raw {
		t.Errorf("inserted %q, want the text unchanged at %q", inserted, raw)
	}
}

func TestReplaceAllTextRequests(t *testing.T) {
	requests := replaceAllTextRequests([]textReplacement{
		{find: "{{title}}", replace: "Hello", matchCase: true},
		{find: "{{date}}", replace: ""},
	}, []string{"slide-1"})

	if len(requests) != 2 {
		t.Fatalf("got %d requests, want 2", len(requests))
	}

	first := requests[0].ReplaceAllText
	if first.ContainsText == nil || first.ContainsText.Text != "{{title}}" {
		t.Errorf("first request searches for %+v", first.ContainsText)
	}
	if !first.ContainsText.MatchCase {
		t.Error("match_case should be honoured")
	}
	if len(first.PageObjectIds) != 1 || first.PageObjectIds[0] != "slide-1" {
		t.Errorf("page scope = %v", first.PageObjectIds)
	}

	// An empty replacement is a deletion, and must survive being a zero value
	second := requests[1].ReplaceAllText
	if second.ReplaceText != "" {
		t.Errorf("replace text = %q, want empty", second.ReplaceText)
	}
}

func TestUpdateSlidesPositionRequests(t *testing.T) {
	requests := updateSlidesPositionRequests([]string{"slide-2", "slide-3"}, 0)
	if len(requests) != 1 {
		t.Fatalf("got %d requests, want 1", len(requests))
	}

	move := requests[0].UpdateSlidesPosition
	if strings.Join(move.SlideObjectIds, ",") != "slide-2,slide-3" {
		t.Errorf("slide ids = %v", move.SlideObjectIds)
	}
	if move.InsertionIndex != 0 {
		t.Errorf("insertion index = %d, want 0", move.InsertionIndex)
	}
	forced := false
	for _, field := range move.ForceSendFields {
		if field == "InsertionIndex" {
			forced = true
		}
	}
	if !forced {
		t.Errorf("InsertionIndex must be forced so that 0 is sent, got %v", move.ForceSendFields)
	}
}

func TestFormatLayouts(t *testing.T) {
	formatted := formatLayouts(fakePresentation())

	if formatted["presentation_id"] != "test-presentation" {
		t.Errorf("presentation_id = %v", formatted["presentation_id"])
	}

	layouts, ok := formatted["layouts"].([]map[string]interface{})
	if !ok {
		t.Fatalf("layouts = %T, want a slice of maps", formatted["layouts"])
	}
	if len(layouts) != len(fakeLayouts) {
		t.Fatalf("got %d layouts, want %d", len(layouts), len(fakeLayouts))
	}

	var titleAndBody map[string]interface{}
	for _, layout := range layouts {
		if layout["layout_id"] == "layout-title-body" {
			titleAndBody = layout
		}
	}
	if titleAndBody == nil {
		t.Fatal("the title and body layout is missing from the listing")
	}
	if titleAndBody["name"] != "TITLE_AND_BODY" || titleAndBody["display_name"] != "Title and body" {
		t.Errorf("layout names = %v / %v", titleAndBody["name"], titleAndBody["display_name"])
	}

	placeholders, ok := titleAndBody["placeholders"].([]map[string]interface{})
	if !ok || len(placeholders) != 2 {
		t.Fatalf("placeholders = %v", titleAndBody["placeholders"])
	}
	if placeholders[0]["type"] != "TITLE" || placeholders[0]["object_id"] != "layout-title-body-title" {
		t.Errorf("first placeholder = %v", placeholders[0])
	}

	masters, ok := formatted["masters"].([]map[string]interface{})
	if !ok || len(masters) != 1 {
		t.Fatalf("masters = %v", formatted["masters"])
	}
}

// TestCreateSlideFromLayoutUsesOneBatch drives the client against the fake: a
// slide costs one presentation read and one batchUpdate, not several.
func TestCreateSlideFromLayoutUsesOneBatch(t *testing.T) {
	client, fake := newFakeClient(t, 2)

	result, err := client.CreateSlideFromLayout("test-presentation", slideFromLayoutInput{
		layoutName: "Title and body",
		placeholders: []placeholderRequest{
			{kind: "TITLE", text: "Quarterly review"},
			{kind: "BODY", text: "First\nSecond\nThird"},
		},
	})
	if err != nil {
		t.Fatalf("CreateSlideFromLayout returned an error: %v", err)
	}

	if fake.count("get") != 1 {
		t.Errorf("presentation was read %d times, want 1", fake.count("get"))
	}
	if fake.batches != 1 {
		t.Errorf("batchUpdate was called %d times, want 1", fake.batches)
	}
	if result.slideId == "" {
		t.Error("the result should name the slide it created")
	}
	if result.layoutId != "layout-title-body" {
		t.Errorf("layout id = %q", result.layoutId)
	}

	// The slide holds the layout's placeholders and nothing else: no free
	// floating text boxes were added
	created := fake.slides[len(fake.slides)-1]
	if len(created.PageElements) != 2 {
		t.Errorf("the new slide has %d elements, want the layout's 2", len(created.PageElements))
	}
	for _, element := range created.PageElements {
		if element.Shape == nil || element.Shape.Placeholder == nil {
			t.Errorf("element %q is not a placeholder", element.ObjectId)
		}
	}
}

// TestCreateSlideFromLayoutRejectsMissingPlaceholder checks the request never
// reaches the API when the layout cannot hold it, which would otherwise fail
// the whole batch.
func TestCreateSlideFromLayoutRejectsMissingPlaceholder(t *testing.T) {
	client, fake := newFakeClient(t, 1)

	_, err := client.CreateSlideFromLayout("test-presentation", slideFromLayoutInput{
		layoutId: "layout-title-only",
		placeholders: []placeholderRequest{
			{kind: "TITLE", text: "Title"},
			{kind: "BODY", text: "This layout has no body"},
		},
	})
	if err == nil {
		t.Fatal("CreateSlideFromLayout should have refused the missing placeholder")
	}
	if fake.batches != 0 {
		t.Errorf("nothing should have been sent, but batchUpdate ran %d times", fake.batches)
	}
}

func TestCreateSlideFromLayoutRejectsUnknownLayout(t *testing.T) {
	client, fake := newFakeClient(t, 1)

	if _, err := client.CreateSlideFromLayout("test-presentation", slideFromLayoutInput{
		layoutName:   "Nonexistent",
		placeholders: []placeholderRequest{{kind: "TITLE", text: "Title"}},
	}); err == nil {
		t.Fatal("CreateSlideFromLayout should have refused an unknown layout")
	}
	if fake.batches != 0 {
		t.Errorf("nothing should have been sent, but batchUpdate ran %d times", fake.batches)
	}
}

func TestReplaceAllTextThroughClient(t *testing.T) {
	client, fake := newFakeClient(t, 0)

	if _, err := client.CreateSlideFromLayout("test-presentation", slideFromLayoutInput{
		layoutId: "layout-title-body",
		placeholders: []placeholderRequest{
			{kind: "TITLE", text: "{{title}}"},
			{kind: "BODY", text: "{{title}} and {{date}}"},
		},
	}); err != nil {
		t.Fatalf("CreateSlideFromLayout returned an error: %v", err)
	}

	batchesBefore := fake.batches
	result, err := client.ReplaceAllText("test-presentation", []textReplacement{
		{find: "{{title}}", replace: "Quarterly review", matchCase: true},
		{find: "{{absent}}", replace: "nothing"},
	}, nil)
	if err != nil {
		t.Fatalf("ReplaceAllText returned an error: %v", err)
	}

	if fake.batches-batchesBefore != 1 {
		t.Errorf("replacements took %d batches, want 1", fake.batches-batchesBefore)
	}
	if fake.count("replaceAllText") != 2 {
		t.Errorf("sent %d replaceAllText requests, want 2", fake.count("replaceAllText"))
	}
	if result.total != 2 {
		t.Errorf("total replacements = %d, want 2", result.total)
	}
	if len(result.perReplacement) != 2 || result.perReplacement[1] != 0 {
		t.Errorf("per replacement counts = %v, want the second to be 0", result.perReplacement)
	}
}

func TestUpdateSlidesPositionThroughClient(t *testing.T) {
	client, fake := newFakeClient(t, 3)
	before := fake.slideIDs()
	if len(before) != 3 {
		t.Fatalf("expected 3 seeded slides, got %v", before)
	}

	if _, err := client.UpdateSlidesPosition("test-presentation", []string{before[2]}, 0); err != nil {
		t.Fatalf("UpdateSlidesPosition returned an error: %v", err)
	}

	after := fake.slideIDs()
	want := []string{before[2], before[0], before[1]}
	if strings.Join(after, ",") != strings.Join(want, ",") {
		t.Errorf("slide order = %v, want %v", after, want)
	}
}

func TestValidatePlaceholderRequests(t *testing.T) {
	tests := []struct {
		name  string
		input []placeholderRequest
	}{
		{"empty", nil},
		{"duplicate type and index", []placeholderRequest{
			{kind: "TITLE", text: "One"},
			{kind: "TITLE", text: "Two"},
		}},
		{"missing text", []placeholderRequest{{kind: "BODY", text: ""}}},
		{"missing type", []placeholderRequest{{kind: "", text: "Something"}}},
		{"negative index", []placeholderRequest{{kind: "BODY", index: -1, text: "Something"}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validatePlaceholderRequests(tt.input); err == nil {
				t.Error("validatePlaceholderRequests should have returned an error")
			}
		})
	}

	valid := []placeholderRequest{
		{kind: "BODY", index: 0, text: "Left"},
		{kind: "BODY", index: 1, text: "Right"},
	}
	if err := validatePlaceholderRequests(valid); err != nil {
		t.Errorf("two indices of the same type are valid, got %v", err)
	}
}

func TestValidateReplacements(t *testing.T) {
	if err := validateReplacements(nil); err == nil {
		t.Error("an empty replacement list should be rejected")
	}
	if err := validateReplacements([]textReplacement{{find: "", replace: "x"}}); err == nil {
		t.Error("an empty find string should be rejected")
	}
	if err := validateReplacements([]textReplacement{{find: "x", replace: ""}}); err != nil {
		t.Errorf("an empty replacement is a deletion and is allowed, got %v", err)
	}

	tooMany := make([]textReplacement, maxRequestsPerBatch+1)
	for i := range tooMany {
		tooMany[i] = textReplacement{find: "x", replace: "y"}
	}
	if err := validateReplacements(tooMany); err == nil {
		t.Error("more replacements than fit in one batch should be rejected")
	}
}

func TestValidateSlideObjectIds(t *testing.T) {
	if err := validateSlideObjectIds(nil, 0); err == nil {
		t.Error("an empty slide list should be rejected")
	}
	if err := validateSlideObjectIds([]string{"a", "a"}, 0); err == nil {
		t.Error("a duplicate slide id should be rejected")
	}
	if err := validateSlideObjectIds([]string{"a"}, -1); err == nil {
		t.Error("a negative insertion index should be rejected")
	}
	if err := validateSlideObjectIds([]string{"a", "b"}, 2); err != nil {
		t.Errorf("a valid reorder should pass, got %v", err)
	}
}

// TestCreateSlideFromLayoutChunksOversizedBatches covers a layout with more
// placeholders than fit in one batchUpdate. The cap is lowered rather than
// building an absurd layout.
func TestCreateSlideFromLayoutChunksOversizedBatches(t *testing.T) {
	withMaxRequestsPerBatch(t, 3)
	client, fake := newFakeClient(t, 0)

	// One createSlide plus three insertText is four requests, over the cap
	result, err := client.CreateSlideFromLayout("test-presentation", slideFromLayoutInput{
		layoutId: "layout-two-columns",
		placeholders: []placeholderRequest{
			{kind: "TITLE", text: "Two columns"},
			{kind: "BODY", index: 0, text: "Left"},
			{kind: "BODY", index: 1, text: "Right"},
		},
	})
	if err != nil {
		t.Fatalf("CreateSlideFromLayout returned an error: %v", err)
	}

	if fake.batches < 2 {
		t.Errorf("four requests under a cap of three should span %d batches, want at least 2", fake.batches)
	}
	for i, size := range fake.batchSizes {
		if size > 3 {
			t.Errorf("batch %d carried %d requests, over the cap of 3", i, size)
		}
	}

	if len(fake.slides) != 1 {
		t.Fatalf("expected one slide, got %d", len(fake.slides))
	}
	// Every placeholder still received its text despite the split
	for _, want := range []string{"Two columns", "Left", "Right"} {
		found := false
		for _, got := range fake.text {
			if got == want {
				found = true
			}
		}
		if !found {
			t.Errorf("text %q never reached the deck", want)
		}
	}
	if result.slideId == "" {
		t.Error("the result should name the slide it created")
	}
}

// TestCreateSlideFromLayoutCleansUpAfterChunkFailure checks the deck is not
// left holding a half-filled slide when a later chunk fails.
func TestCreateSlideFromLayoutCleansUpAfterChunkFailure(t *testing.T) {
	withMaxRequestsPerBatch(t, 2)
	client, fake := newFakeClient(t, 1)
	// The first batch carries the createSlide and succeeds, the second fails,
	// and the cleanup that follows is allowed through
	fake.failBatchNumber = 2

	_, err := client.CreateSlideFromLayout("test-presentation", slideFromLayoutInput{
		layoutId: "layout-two-columns",
		placeholders: []placeholderRequest{
			{kind: "TITLE", text: "Two columns"},
			{kind: "BODY", index: 0, text: "Left"},
			{kind: "BODY", index: 1, text: "Right"},
		},
	})
	if err == nil {
		t.Fatal("CreateSlideFromLayout should have reported the failure")
	}

	// Whatever happened, the caller's deck is back to the one slide it had
	if len(fake.slides) != 1 {
		t.Errorf("deck holds %d slides, want the original 1", len(fake.slides))
	}
}

// TestReplaceAllTextRespectsPageScope is the end-to-end check that a scoped
// replacement does not leak into slides the caller did not name.
func TestReplaceAllTextRespectsPageScope(t *testing.T) {
	client, fake := newFakeClient(t, 0)

	first, err := client.CreateSlideFromLayout("test-presentation", slideFromLayoutInput{
		layoutId:     "layout-title-only",
		placeholders: []placeholderRequest{{kind: "TITLE", text: "{{token}} on the first slide"}},
	})
	if err != nil {
		t.Fatalf("CreateSlideFromLayout returned an error: %v", err)
	}
	second, err := client.CreateSlideFromLayout("test-presentation", slideFromLayoutInput{
		layoutId:     "layout-title-only",
		placeholders: []placeholderRequest{{kind: "TITLE", text: "{{token}} on the second slide"}},
	})
	if err != nil {
		t.Fatalf("CreateSlideFromLayout returned an error: %v", err)
	}

	result, err := client.ReplaceAllText("test-presentation",
		[]textReplacement{{find: "{{token}}", replace: "REPLACED", matchCase: true}},
		[]string{first.slideId})
	if err != nil {
		t.Fatalf("ReplaceAllText returned an error: %v", err)
	}

	if result.total != 1 {
		t.Errorf("replaced %d occurrences, want only the one on the named slide", result.total)
	}

	firstText := textOnSlide(t, fake, first.slideId)
	if !strings.Contains(firstText, "REPLACED") {
		t.Errorf("the named slide should have been changed, got %q", firstText)
	}
	secondText := textOnSlide(t, fake, second.slideId)
	if !strings.Contains(secondText, "{{token}}") {
		t.Errorf("the other slide should be untouched, got %q", secondText)
	}
}

// textOnSlide joins whatever text the fake recorded for the elements of a slide.
func textOnSlide(t *testing.T, fake *fakeSlidesAPI, slideId string) string {
	t.Helper()

	for _, page := range fake.slides {
		if page.ObjectId != slideId {
			continue
		}
		var parts []string
		for _, element := range page.PageElements {
			if got, ok := fake.text[element.ObjectId]; ok {
				parts = append(parts, got)
			}
		}
		return strings.Join(parts, " ")
	}

	t.Fatalf("slide %q is not in the deck", slideId)
	return ""
}
