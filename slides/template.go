package slides

import (
	"fmt"
	"strings"
	"time"

	"google.golang.org/api/slides/v1"
)

// defaultBulletPreset is what createParagraphBullets uses when the caller does
// not name one.
const defaultBulletPreset = "BULLET_DISC_CIRCLE_SQUARE"

// maxLayoutsInError caps how many layouts an error message lists, so a deck
// with a large theme does not produce an unreadable error.
const maxLayoutsInError = 20

// layoutPlaceholder is one placeholder a layout defines.
type layoutPlaceholder struct {
	kind     string // Placeholder.Type
	index    int64  // Placeholder.Index; 0 when the layout has only one of this type
	objectId string
}

// placeholderRequest is one placeholder fill as the caller asked for it.
type placeholderRequest struct {
	kind         string
	index        int64
	text         string
	bullets      bool
	bulletPreset string
}

// placeholderFill is a requested fill after its layout placeholder has been
// resolved and an object ID has been assigned for this run.
type placeholderFill struct {
	kind         string
	index        int64
	text         string
	bullets      bool
	bulletPreset string
	objectId     string
}

// slideFromLayoutInput is a whole request to create one slide from a layout.
type slideFromLayoutInput struct {
	layoutId       string
	layoutName     string
	slideObjectId  string
	insertionIndex *int64
	placeholders   []placeholderRequest
}

// slideFromLayoutResult describes the slide that was created.
type slideFromLayoutResult struct {
	slideId        string
	layoutId       string
	layoutName     string
	insertionIndex *int64
	fills          []placeholderFill
	requestCount   int
}

// textReplacement is one token substitution.
type textReplacement struct {
	find      string
	replace   string
	matchCase bool
}

// replaceAllTextResult reports how many occurrences each replacement changed.
type replaceAllTextResult struct {
	total          int64
	perReplacement []int64
}

// resolveLayout finds a layout by object ID or by name. Unlike findLayoutId it
// never falls back to another layout: filling a template with the wrong layout
// is worse than failing, because the caller cannot tell it happened.
func resolveLayout(presentation *slides.Presentation, layoutId, layoutName string) (*slides.Page, error) {
	if (layoutId == "") == (layoutName == "") {
		return nil, fmt.Errorf("specify exactly one of layout_id or layout_name")
	}

	if layoutId != "" {
		for _, layout := range presentation.Layouts {
			if layout.ObjectId == layoutId {
				return layout, nil
			}
		}
		return nil, fmt.Errorf("layout %q not found; available layouts: %s", layoutId, describeLayouts(presentation))
	}

	// An exact display name first, then the API name. Matching is exact: a
	// near miss would silently fill the wrong layout.
	for _, layout := range presentation.Layouts {
		if layout.LayoutProperties != nil && layout.LayoutProperties.DisplayName == layoutName {
			return layout, nil
		}
	}
	for _, layout := range presentation.Layouts {
		if layout.LayoutProperties != nil && layout.LayoutProperties.Name == layoutName {
			return layout, nil
		}
	}

	return nil, fmt.Errorf("layout %q not found; available layouts: %s", layoutName, describeLayouts(presentation))
}

// describeLayouts renders the layouts of a deck as "Display name" (NAME), so an
// error can tell the caller what they could have asked for.
func describeLayouts(presentation *slides.Presentation) string {
	if len(presentation.Layouts) == 0 {
		return "none"
	}

	described := make([]string, 0, len(presentation.Layouts))
	for _, layout := range presentation.Layouts {
		if len(described) == maxLayoutsInError {
			described = append(described, fmt.Sprintf("and %d more", len(presentation.Layouts)-maxLayoutsInError))
			break
		}
		if layout.LayoutProperties == nil {
			described = append(described, fmt.Sprintf("%q", layout.ObjectId))
			continue
		}
		described = append(described, fmt.Sprintf("%q (%s)",
			layout.LayoutProperties.DisplayName, layout.LayoutProperties.Name))
	}

	return strings.Join(described, ", ")
}

// layoutPlaceholdersOf lists every placeholder a layout defines, keyed by type
// and index.
func layoutPlaceholdersOf(layout *slides.Page) []layoutPlaceholder {
	if layout == nil {
		return nil
	}

	found := make([]layoutPlaceholder, 0, len(layout.PageElements))
	for _, element := range layout.PageElements {
		if element.Shape == nil || element.Shape.Placeholder == nil {
			continue
		}
		found = append(found, layoutPlaceholder{
			kind:     element.Shape.Placeholder.Type,
			index:    element.Shape.Placeholder.Index,
			objectId: element.ObjectId,
		})
	}

	return found
}

// findPlaceholder returns the layout placeholder matching a requested type and
// index, or an error naming what the layout does have.
func findPlaceholder(available []layoutPlaceholder, kind string, index int64) (layoutPlaceholder, error) {
	for _, candidate := range available {
		if candidate.kind == kind && candidate.index == index {
			return candidate, nil
		}
	}

	return layoutPlaceholder{}, fmt.Errorf("no placeholder %s; available placeholders: %s",
		placeholderLabel(kind, index), describePlaceholders(available))
}

// placeholderLabel renders a placeholder as TYPE[index], the form the errors
// and the tool description both use.
func placeholderLabel(kind string, index int64) string {
	return fmt.Sprintf("%s[%d]", kind, index)
}

func describePlaceholders(available []layoutPlaceholder) string {
	if len(available) == 0 {
		return "none"
	}

	described := make([]string, 0, len(available))
	for _, ph := range available {
		described = append(described, placeholderLabel(ph.kind, ph.index))
	}
	return strings.Join(described, ", ")
}

// validatePlaceholderRequests checks what can be checked without reading the
// presentation, so an obviously wrong call costs no API request.
func validatePlaceholderRequests(requests []placeholderRequest) error {
	if len(requests) == 0 {
		return fmt.Errorf("placeholders must not be empty")
	}

	seen := make(map[string]bool, len(requests))
	for _, request := range requests {
		if request.kind == "" {
			return fmt.Errorf("every placeholder needs a type")
		}
		if request.index < 0 {
			return fmt.Errorf("placeholder %s has a negative index", placeholderLabel(request.kind, request.index))
		}
		if request.text == "" {
			return fmt.Errorf("placeholder %s has no text", placeholderLabel(request.kind, request.index))
		}
		label := placeholderLabel(request.kind, request.index)
		if seen[label] {
			return fmt.Errorf("duplicate placeholder %s", label)
		}
		seen[label] = true
	}

	return nil
}

// validateReplacements checks the token substitutions before any are sent.
func validateReplacements(replacements []textReplacement) error {
	if len(replacements) == 0 {
		return fmt.Errorf("replacements must not be empty")
	}
	if len(replacements) > maxRequestsPerBatch {
		return fmt.Errorf("too many replacements (%d); at most %d per call",
			len(replacements), maxRequestsPerBatch)
	}

	for i, replacement := range replacements {
		if replacement.find == "" {
			return fmt.Errorf("replacement %d has an empty find string", i)
		}
	}

	return nil
}

// validateSlideObjectIds checks a reorder before it is sent.
func validateSlideObjectIds(slideObjectIds []string, insertionIndex int64) error {
	if len(slideObjectIds) == 0 {
		return fmt.Errorf("slide_object_ids must not be empty")
	}
	if insertionIndex < 0 {
		return fmt.Errorf("insertion_index must be 0 or greater")
	}

	seen := make(map[string]bool, len(slideObjectIds))
	for _, id := range slideObjectIds {
		if id == "" {
			return fmt.Errorf("slide_object_ids must not contain an empty id")
		}
		if seen[id] {
			return fmt.Errorf("duplicate slide id %q", id)
		}
		seen[id] = true
	}

	return nil
}

// createSlideFromLayoutRequests builds the whole batch: one createSlide with
// placeholder ID mappings, one insertText per placeholder, and one
// createParagraphBullets per placeholder that asked for bullets.
//
// Nothing here moves, resizes or restyles an element. The template owns the
// design, and this path only puts text into the holes it left.
func createSlideFromLayoutRequests(layoutId, slideObjectId string, insertionIndex *int64, fills []placeholderFill, mappings []*slides.LayoutPlaceholderIdMapping) []*slides.Request {
	create := &slides.CreateSlideRequest{
		ObjectId:              slideObjectId,
		PlaceholderIdMappings: mappings,
	}
	if layoutId != "" {
		create.SlideLayoutReference = &slides.LayoutReference{LayoutId: layoutId}
	}
	if insertionIndex != nil {
		create.InsertionIndex = *insertionIndex
		// Index 0 means the front of the deck, and would otherwise be dropped
		// as an empty value, silently appending instead
		create.ForceSendFields = append(create.ForceSendFields, "InsertionIndex")
	}

	requests := []*slides.Request{{CreateSlide: create}}

	for _, fill := range fills {
		requests = append(requests, &slides.Request{
			InsertText: &slides.InsertTextRequest{
				ObjectId: fill.objectId,
				Text:     fill.text,
				// The placeholder is new and therefore empty, so there is
				// nothing to delete first and no offset to skip
				InsertionIndex:  0,
				ForceSendFields: []string{"InsertionIndex"},
			},
		})
	}

	// Bullets apply to text that is already in place, so they come after every
	// insertion rather than interleaved with it
	for _, fill := range fills {
		if !fill.bullets {
			continue
		}
		preset := fill.bulletPreset
		if preset == "" {
			preset = defaultBulletPreset
		}
		requests = append(requests, &slides.Request{
			CreateParagraphBullets: &slides.CreateParagraphBulletsRequest{
				ObjectId:     fill.objectId,
				TextRange:    &slides.Range{Type: "ALL"},
				BulletPreset: preset,
			},
		})
	}

	return requests
}

// replaceAllTextRequests builds one replaceAllText per replacement.
func replaceAllTextRequests(replacements []textReplacement, pageObjectIds []string) []*slides.Request {
	requests := make([]*slides.Request, 0, len(replacements))
	for _, replacement := range replacements {
		request := &slides.ReplaceAllTextRequest{
			ContainsText: &slides.SubstringMatchCriteria{
				Text:      replacement.find,
				MatchCase: replacement.matchCase,
			},
			ReplaceText: replacement.replace,
			// An empty replacement deletes the token, so it has to be sent
			ForceSendFields: []string{"ReplaceText"},
		}
		if len(pageObjectIds) > 0 {
			request.PageObjectIds = pageObjectIds
		}
		requests = append(requests, &slides.Request{ReplaceAllText: request})
	}

	return requests
}

// updateSlidesPositionRequests builds the single reorder request. The API keeps
// the given order and places the slides at the insertion index.
func updateSlidesPositionRequests(slideObjectIds []string, insertionIndex int64) []*slides.Request {
	return []*slides.Request{{
		UpdateSlidesPosition: &slides.UpdateSlidesPositionRequest{
			SlideObjectIds: slideObjectIds,
			InsertionIndex: insertionIndex,
			// Index 0 means the front of the deck and must survive being zero
			ForceSendFields: []string{"InsertionIndex"},
		},
	}}
}

// newTemplateObjectIdPrefix mirrors newObjectIdPrefix: a per-run prefix so
// generated IDs cannot collide with objects already in the presentation.
func newTemplateObjectIdPrefix() string {
	return fmt.Sprintf("tpl%d", time.Now().UnixNano())
}

// formatLayouts renders the masters, layouts and placeholders of a deck for the
// layouts listing tool.
func formatLayouts(presentation *slides.Presentation) map[string]interface{} {
	masters := make([]map[string]interface{}, 0, len(presentation.Masters))
	for _, master := range presentation.Masters {
		entry := map[string]interface{}{"master_id": master.ObjectId}
		if master.MasterProperties != nil {
			entry["display_name"] = master.MasterProperties.DisplayName
		}
		masters = append(masters, entry)
	}

	layouts := make([]map[string]interface{}, 0, len(presentation.Layouts))
	for _, layout := range presentation.Layouts {
		placeholders := make([]map[string]interface{}, 0, len(layout.PageElements))
		for _, ph := range layoutPlaceholdersOf(layout) {
			placeholders = append(placeholders, map[string]interface{}{
				"type":      ph.kind,
				"index":     ph.index,
				"object_id": ph.objectId,
			})
		}

		entry := map[string]interface{}{
			"layout_id":    layout.ObjectId,
			"placeholders": placeholders,
		}
		if layout.LayoutProperties != nil {
			entry["display_name"] = layout.LayoutProperties.DisplayName
			entry["name"] = layout.LayoutProperties.Name
			entry["master_id"] = layout.LayoutProperties.MasterObjectId
		}
		layouts = append(layouts, entry)
	}

	result := map[string]interface{}{
		"presentation_id": presentation.PresentationId,
		"masters":         masters,
		"layouts":         layouts,
	}
	if presentation.Title != "" {
		result["title"] = presentation.Title
	}

	return result
}

// formatPlaceholderFills renders the filled placeholders for a tool response.
func formatPlaceholderFills(fills []placeholderFill) []map[string]interface{} {
	formatted := make([]map[string]interface{}, 0, len(fills))
	for _, fill := range fills {
		formatted = append(formatted, map[string]interface{}{
			"type":       fill.kind,
			"index":      fill.index,
			"object_id":  fill.objectId,
			"characters": len([]rune(fill.text)),
		})
	}
	return formatted
}
