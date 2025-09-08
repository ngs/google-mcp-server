package calendar

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.ngs.io/google-mcp-server/server"
)

// Handler implements the ServiceHandler interface for Calendar
type Handler struct {
	client *Client
}

// NewHandler creates a new Calendar handler
func NewHandler(client *Client) *Handler {
	return &Handler{client: client}
}

// GetTools returns the available Calendar tools
func (h *Handler) GetTools() []server.Tool {
	return []server.Tool{
		{
			Name:        "calendar_list",
			Description: "List all accessible calendars",
			InputSchema: server.InputSchema{
				Type:       "object",
				Properties: map[string]server.Property{},
			},
		},
		{
			Name:        "calendar_events_list",
			Description: "List events from a calendar with optional date range",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"calendar_id": {
						Type:        "string",
						Description: "Calendar ID (use 'primary' for main calendar)",
					},
					"time_min": {
						Type:        "string",
						Description: "Start time (RFC3339 format)",
					},
					"time_max": {
						Type:        "string",
						Description: "End time (RFC3339 format)",
					},
					"max_results": {
						Type:        "number",
						Description: "Maximum number of events to return",
					},
				},
				Required: []string{"calendar_id"},
			},
		},
		{
			Name:        "calendar_event_create",
			Description: "Create a new calendar event",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"calendar_id": {
						Type:        "string",
						Description: "Calendar ID (use 'primary' for main calendar)",
					},
					"summary": {
						Type:        "string",
						Description: "Event title",
					},
					"description": {
						Type:        "string",
						Description: "Event description",
					},
					"location": {
						Type:        "string",
						Description: "Event location",
					},
					"start_time": {
						Type:        "string",
						Description: "Start time (RFC3339 format)",
					},
					"end_time": {
						Type:        "string",
						Description: "End time (RFC3339 format)",
					},
					"attendees": {
						Type:        "array",
						Description: "List of attendee email addresses",
						Items: &server.Property{
							Type: "string",
						},
					},
					"reminders": {
						Type:        "array",
						Description: "List of reminder times in minutes",
						Items: &server.Property{
							Type: "number",
						},
					},
				},
				Required: []string{"calendar_id", "summary", "start_time", "end_time"},
			},
		},
		{
			Name:        "calendar_event_update",
			Description: "Update an existing calendar event",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"calendar_id": {
						Type:        "string",
						Description: "Calendar ID",
					},
					"event_id": {
						Type:        "string",
						Description: "Event ID",
					},
					"summary": {
						Type:        "string",
						Description: "Event title",
					},
					"description": {
						Type:        "string",
						Description: "Event description",
					},
					"location": {
						Type:        "string",
						Description: "Event location",
					},
					"start_time": {
						Type:        "string",
						Description: "Start time (RFC3339 format)",
					},
					"end_time": {
						Type:        "string",
						Description: "End time (RFC3339 format)",
					},
				},
				Required: []string{"calendar_id", "event_id"},
			},
		},
		{
			Name:        "calendar_event_delete",
			Description: "Delete a calendar event",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"calendar_id": {
						Type:        "string",
						Description: "Calendar ID",
					},
					"event_id": {
						Type:        "string",
						Description: "Event ID",
					},
				},
				Required: []string{"calendar_id", "event_id"},
			},
		},
		{
			Name:        "calendar_event_get",
			Description: "Get details of a specific event",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"calendar_id": {
						Type:        "string",
						Description: "Calendar ID",
					},
					"event_id": {
						Type:        "string",
						Description: "Event ID",
					},
				},
				Required: []string{"calendar_id", "event_id"},
			},
		},
		{
			Name:        "calendar_freebusy_query",
			Description: "Query free/busy information for calendars",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"calendar_ids": {
						Type:        "array",
						Description: "List of calendar IDs to check",
						Items: &server.Property{
							Type: "string",
						},
					},
					"time_min": {
						Type:        "string",
						Description: "Start time (RFC3339 format)",
					},
					"time_max": {
						Type:        "string",
						Description: "End time (RFC3339 format)",
					},
				},
				Required: []string{"calendar_ids", "time_min", "time_max"},
			},
		},
		{
			Name:        "calendar_event_search",
			Description: "Search for events in a calendar",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"calendar_id": {
						Type:        "string",
						Description: "Calendar ID",
					},
					"query": {
						Type:        "string",
						Description: "Search query",
					},
					"time_min": {
						Type:        "string",
						Description: "Start time (RFC3339 format)",
					},
					"time_max": {
						Type:        "string",
						Description: "End time (RFC3339 format)",
					},
				},
				Required: []string{"calendar_id", "query"},
			},
		},
	}
}

// HandleToolCall handles a tool call for Calendar service
func (h *Handler) HandleToolCall(ctx context.Context, name string, arguments json.RawMessage) (interface{}, error) {
	switch name {
	case "calendar_list":
		return h.handleCalendarList(ctx)
	
	case "calendar_events_list":
		var args struct {
			CalendarID string  `json:"calendar_id"`
			TimeMin    string  `json:"time_min"`
			TimeMax    string  `json:"time_max"`
			MaxResults float64 `json:"max_results"`
		}
		if err := json.Unmarshal(arguments, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
		return h.handleEventsList(ctx, args.CalendarID, args.TimeMin, args.TimeMax, int64(args.MaxResults))
	
	case "calendar_event_create":
		var args struct {
			CalendarID  string   `json:"calendar_id"`
			Summary     string   `json:"summary"`
			Description string   `json:"description"`
			Location    string   `json:"location"`
			StartTime   string   `json:"start_time"`
			EndTime     string   `json:"end_time"`
			Attendees   []string `json:"attendees"`
			Reminders   []int    `json:"reminders"`
		}
		if err := json.Unmarshal(arguments, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
		return h.handleEventCreate(ctx, args.CalendarID, args.Summary, args.Description, 
			args.Location, args.StartTime, args.EndTime, args.Attendees, args.Reminders)
	
	case "calendar_event_update":
		var args struct {
			CalendarID  string `json:"calendar_id"`
			EventID     string `json:"event_id"`
			Summary     string `json:"summary"`
			Description string `json:"description"`
			Location    string `json:"location"`
			StartTime   string `json:"start_time"`
			EndTime     string `json:"end_time"`
		}
		if err := json.Unmarshal(arguments, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
		return h.handleEventUpdate(ctx, args.CalendarID, args.EventID, args.Summary,
			args.Description, args.Location, args.StartTime, args.EndTime)
	
	case "calendar_event_delete":
		var args struct {
			CalendarID string `json:"calendar_id"`
			EventID    string `json:"event_id"`
		}
		if err := json.Unmarshal(arguments, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
		return h.handleEventDelete(ctx, args.CalendarID, args.EventID)
	
	case "calendar_event_get":
		var args struct {
			CalendarID string `json:"calendar_id"`
			EventID    string `json:"event_id"`
		}
		if err := json.Unmarshal(arguments, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
		return h.handleEventGet(ctx, args.CalendarID, args.EventID)
	
	case "calendar_freebusy_query":
		var args struct {
			CalendarIDs []string `json:"calendar_ids"`
			TimeMin     string   `json:"time_min"`
			TimeMax     string   `json:"time_max"`
		}
		if err := json.Unmarshal(arguments, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
		return h.handleFreeBusyQuery(ctx, args.CalendarIDs, args.TimeMin, args.TimeMax)
	
	case "calendar_event_search":
		var args struct {
			CalendarID string `json:"calendar_id"`
			Query      string `json:"query"`
			TimeMin    string `json:"time_min"`
			TimeMax    string `json:"time_max"`
		}
		if err := json.Unmarshal(arguments, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
		return h.handleEventSearch(ctx, args.CalendarID, args.Query, args.TimeMin, args.TimeMax)
	
	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

// Tool handlers
func (h *Handler) handleCalendarList(ctx context.Context) (interface{}, error) {
	calendars, err := h.client.ListCalendars()
	if err != nil {
		return nil, err
	}
	
	// Format the response
	result := make([]map[string]interface{}, len(calendars))
	for i, cal := range calendars {
		result[i] = map[string]interface{}{
			"id":            cal.Id,
			"summary":       cal.Summary,
			"description":   cal.Description,
			"primary":       cal.Primary,
			"accessRole":    cal.AccessRole,
			"backgroundColor": cal.BackgroundColor,
		}
	}
	
	return result, nil
}

func (h *Handler) handleEventsList(ctx context.Context, calendarID, timeMinStr, timeMaxStr string, maxResults int64) (interface{}, error) {
	var timeMin, timeMax time.Time
	var err error
	
	if timeMinStr != "" {
		timeMin, err = time.Parse(time.RFC3339, timeMinStr)
		if err != nil {
			return nil, fmt.Errorf("invalid time_min format: %w", err)
		}
	}
	
	if timeMaxStr != "" {
		timeMax, err = time.Parse(time.RFC3339, timeMaxStr)
		if err != nil {
			return nil, fmt.Errorf("invalid time_max format: %w", err)
		}
	}
	
	events, err := h.client.ListEvents(calendarID, timeMin, timeMax, maxResults)
	if err != nil {
		return nil, err
	}
	
	// Format the response
	result := make([]map[string]interface{}, len(events))
	for i, event := range events {
		result[i] = formatEvent(event)
	}
	
	return result, nil
}

func (h *Handler) handleEventCreate(ctx context.Context, calendarID, summary, description, location,
	startTimeStr, endTimeStr string, attendees []string, reminders []int) (interface{}, error) {
	
	startTime, err := time.Parse(time.RFC3339, startTimeStr)
	if err != nil {
		return nil, fmt.Errorf("invalid start_time format: %w", err)
	}
	
	endTime, err := time.Parse(time.RFC3339, endTimeStr)
	if err != nil {
		return nil, fmt.Errorf("invalid end_time format: %w", err)
	}
	
	event, err := h.client.CreateEventFromDetails(calendarID, summary, description, location,
		startTime, endTime, attendees, reminders)
	if err != nil {
		return nil, err
	}
	
	return formatEvent(event), nil
}

func (h *Handler) handleEventUpdate(ctx context.Context, calendarID, eventID, summary,
	description, location, startTimeStr, endTimeStr string) (interface{}, error) {
	
	// Get existing event
	event, err := h.client.GetEvent(calendarID, eventID)
	if err != nil {
		return nil, err
	}
	
	// Update fields if provided
	if summary != "" {
		event.Summary = summary
	}
	if description != "" {
		event.Description = description
	}
	if location != "" {
		event.Location = location
	}
	if startTimeStr != "" {
		startTime, err := time.Parse(time.RFC3339, startTimeStr)
		if err != nil {
			return nil, fmt.Errorf("invalid start_time format: %w", err)
		}
		event.Start.DateTime = startTime.Format(time.RFC3339)
	}
	if endTimeStr != "" {
		endTime, err := time.Parse(time.RFC3339, endTimeStr)
		if err != nil {
			return nil, fmt.Errorf("invalid end_time format: %w", err)
		}
		event.End.DateTime = endTime.Format(time.RFC3339)
	}
	
	updated, err := h.client.UpdateEvent(calendarID, eventID, event)
	if err != nil {
		return nil, err
	}
	
	return formatEvent(updated), nil
}

func (h *Handler) handleEventDelete(ctx context.Context, calendarID, eventID string) (interface{}, error) {
	if err := h.client.DeleteEvent(calendarID, eventID); err != nil {
		return nil, err
	}
	return map[string]string{"status": "deleted", "event_id": eventID}, nil
}

func (h *Handler) handleEventGet(ctx context.Context, calendarID, eventID string) (interface{}, error) {
	event, err := h.client.GetEvent(calendarID, eventID)
	if err != nil {
		return nil, err
	}
	return formatEvent(event), nil
}

func (h *Handler) handleFreeBusyQuery(ctx context.Context, calendarIDs []string, timeMinStr, timeMaxStr string) (interface{}, error) {
	timeMin, err := time.Parse(time.RFC3339, timeMinStr)
	if err != nil {
		return nil, fmt.Errorf("invalid time_min format: %w", err)
	}
	
	timeMax, err := time.Parse(time.RFC3339, timeMaxStr)
	if err != nil {
		return nil, fmt.Errorf("invalid time_max format: %w", err)
	}
	
	response, err := h.client.QueryFreeBusy(calendarIDs, timeMin, timeMax)
	if err != nil {
		return nil, err
	}
	
	// Format the response
	result := make(map[string]interface{})
	result["timeMin"] = response.TimeMin
	result["timeMax"] = response.TimeMax
	
	calendars := make(map[string]interface{})
	for id, cal := range response.Calendars {
		calData := make(map[string]interface{})
		if cal.Errors != nil {
			calData["errors"] = cal.Errors
		}
		if cal.Busy != nil {
			busy := make([]map[string]string, len(cal.Busy))
			for i, period := range cal.Busy {
				busy[i] = map[string]string{
					"start": period.Start,
					"end":   period.End,
				}
			}
			calData["busy"] = busy
		}
		calendars[id] = calData
	}
	result["calendars"] = calendars
	
	return result, nil
}

func (h *Handler) handleEventSearch(ctx context.Context, calendarID, query, timeMinStr, timeMaxStr string) (interface{}, error) {
	var timeMin, timeMax time.Time
	var err error
	
	if timeMinStr != "" {
		timeMin, err = time.Parse(time.RFC3339, timeMinStr)
		if err != nil {
			return nil, fmt.Errorf("invalid time_min format: %w", err)
		}
	}
	
	if timeMaxStr != "" {
		timeMax, err = time.Parse(time.RFC3339, timeMaxStr)
		if err != nil {
			return nil, fmt.Errorf("invalid time_max format: %w", err)
		}
	}
	
	events, err := h.client.SearchEvents(calendarID, query, timeMin, timeMax)
	if err != nil {
		return nil, err
	}
	
	// Format the response
	result := make([]map[string]interface{}, len(events))
	for i, event := range events {
		result[i] = formatEvent(event)
	}
	
	return result, nil
}

// formatEvent formats a calendar event for response
func formatEvent(event interface{}) map[string]interface{} {
	// This is a simplified version - the actual Google Calendar Event struct has many more fields
	// In a real implementation, we'd use type assertion and properly format all fields
	
	data := make(map[string]interface{})
	
	// Use JSON marshaling/unmarshaling as a simple way to convert
	jsonData, _ := json.Marshal(event)
	json.Unmarshal(jsonData, &data)
	
	// Clean up some fields for better readability
	if start, ok := data["start"].(map[string]interface{}); ok {
		if dt, exists := start["dateTime"]; exists {
			data["startTime"] = dt
		} else if d, exists := start["date"]; exists {
			data["startDate"] = d
		}
	}
	
	if end, ok := data["end"].(map[string]interface{}); ok {
		if dt, exists := end["dateTime"]; exists {
			data["endTime"] = dt
		} else if d, exists := end["date"]; exists {
			data["endDate"] = d
		}
	}
	
	return data
}