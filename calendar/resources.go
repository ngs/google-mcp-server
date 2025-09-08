package calendar

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.ngs.io/google-mcp-server/server"
)

// GetResources returns the available Calendar resources
func (h *Handler) GetResources() []server.Resource {
	return []server.Resource{
		{
			URI:         "calendar://primary/events",
			Name:        "Primary Calendar Events",
			Description: "Events from the user's primary calendar",
			MimeType:    "application/json",
		},
		{
			URI:         "calendar://calendars",
			Name:        "Calendar List",
			Description: "List of all accessible calendars",
			MimeType:    "application/json",
		},
	}
}

// HandleResourceCall handles a resource call for Calendar service
func (h *Handler) HandleResourceCall(ctx context.Context, uri string) (interface{}, error) {
	// Parse the URI
	if !strings.HasPrefix(uri, "calendar://") {
		return nil, fmt.Errorf("invalid calendar URI: %s", uri)
	}
	
	path := strings.TrimPrefix(uri, "calendar://")
	parts := strings.Split(path, "/")
	
	switch parts[0] {
	case "primary":
		if len(parts) > 1 && parts[1] == "events" {
			return h.getPrimaryCalendarEvents(ctx)
		}
		return nil, fmt.Errorf("unknown primary calendar resource: %s", uri)
		
	case "calendars":
		return h.getCalendarsList(ctx)
		
	default:
		return nil, fmt.Errorf("unknown calendar resource: %s", uri)
	}
}

func (h *Handler) getPrimaryCalendarEvents(ctx context.Context) (interface{}, error) {
	// Get events from primary calendar (no date filter, get upcoming events)
	events, err := h.client.ListEvents("primary", time.Now(), time.Time{}, 100)
	if err != nil {
		return nil, fmt.Errorf("failed to get primary calendar events: %w", err)
	}
	
	// Format events
	result := make([]map[string]interface{}, len(events))
	for i, event := range events {
		result[i] = formatEvent(event)
	}
	
	return map[string]interface{}{
		"calendar": "primary",
		"events":   result,
		"count":    len(result),
	}, nil
}

func (h *Handler) getCalendarsList(ctx context.Context) (interface{}, error) {
	calendars, err := h.client.ListCalendars()
	if err != nil {
		return nil, fmt.Errorf("failed to list calendars: %w", err)
	}
	
	// Format calendars
	result := make([]map[string]interface{}, len(calendars))
	for i, cal := range calendars {
		result[i] = map[string]interface{}{
			"id":              cal.Id,
			"summary":         cal.Summary,
			"description":     cal.Description,
			"primary":         cal.Primary,
			"accessRole":      cal.AccessRole,
			"backgroundColor": cal.BackgroundColor,
			"foregroundColor": cal.ForegroundColor,
			"selected":        cal.Selected,
			"timeZone":        cal.TimeZone,
		}
	}
	
	return map[string]interface{}{
		"calendars": result,
		"count":     len(result),
	}, nil
}