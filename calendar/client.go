package calendar

import (
	"context"
	"fmt"
	"time"

	"go.ngs.io/google-mcp-server/auth"
	"google.golang.org/api/calendar/v3"
)

// Client wraps the Google Calendar API client
type Client struct {
	service *calendar.Service
}

// NewClient creates a new Calendar client
func NewClient(ctx context.Context, oauth *auth.OAuthClient) (*Client, error) {
	if oauth == nil {
		return nil, fmt.Errorf("oauth client is nil")
	}

	httpClient := oauth.GetHTTPClient()
	if httpClient == nil {
		return nil, fmt.Errorf("http client is nil")
	}

	service, err := calendar.NewService(ctx, oauth.GetClientOption())
	if err != nil {
		return nil, fmt.Errorf("failed to create calendar service: %w", err)
	}

	return &Client{
		service: service,
	}, nil
}

// ListCalendars lists all calendars
func (c *Client) ListCalendars() ([]*calendar.CalendarListEntry, error) {
	var calendars []*calendar.CalendarListEntry

	ctx := context.Background()
	call := c.service.CalendarList.List()
	err := call.Pages(ctx, func(page *calendar.CalendarList) error {
		calendars = append(calendars, page.Items...)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list calendars: %w", err)
	}

	return calendars, nil
}

// ListEvents lists events from a calendar
func (c *Client) ListEvents(calendarID string, timeMin, timeMax time.Time, maxResults int64) ([]*calendar.Event, error) {
	call := c.service.Events.List(calendarID).
		ShowDeleted(false).
		SingleEvents(true).
		OrderBy("startTime")

	if !timeMin.IsZero() {
		call = call.TimeMin(timeMin.Format(time.RFC3339))
	}
	if !timeMax.IsZero() {
		call = call.TimeMax(timeMax.Format(time.RFC3339))
	}
	if maxResults > 0 {
		call = call.MaxResults(maxResults)
	}

	events, err := call.Do()
	if err != nil {
		return nil, fmt.Errorf("failed to list events: %w", err)
	}

	return events.Items, nil
}

// GetEvent gets a specific event
func (c *Client) GetEvent(calendarID, eventID string) (*calendar.Event, error) {
	event, err := c.service.Events.Get(calendarID, eventID).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get event: %w", err)
	}
	return event, nil
}

// CreateEvent creates a new event
func (c *Client) CreateEvent(calendarID string, event *calendar.Event) (*calendar.Event, error) {
	created, err := c.service.Events.Insert(calendarID, event).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to create event: %w", err)
	}
	return created, nil
}

// UpdateEvent updates an existing event
func (c *Client) UpdateEvent(calendarID, eventID string, event *calendar.Event) (*calendar.Event, error) {
	updated, err := c.service.Events.Update(calendarID, eventID, event).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to update event: %w", err)
	}
	return updated, nil
}

// DeleteEvent deletes an event
func (c *Client) DeleteEvent(calendarID, eventID string) error {
	err := c.service.Events.Delete(calendarID, eventID).Do()
	if err != nil {
		return fmt.Errorf("failed to delete event: %w", err)
	}
	return nil
}

// SearchEvents searches for events
func (c *Client) SearchEvents(calendarID, query string, timeMin, timeMax time.Time) ([]*calendar.Event, error) {
	call := c.service.Events.List(calendarID).
		Q(query).
		ShowDeleted(false).
		SingleEvents(true).
		OrderBy("startTime")

	if !timeMin.IsZero() {
		call = call.TimeMin(timeMin.Format(time.RFC3339))
	}
	if !timeMax.IsZero() {
		call = call.TimeMax(timeMax.Format(time.RFC3339))
	}

	events, err := call.Do()
	if err != nil {
		return nil, fmt.Errorf("failed to search events: %w", err)
	}

	return events.Items, nil
}

// QueryFreeBusy queries free/busy information
func (c *Client) QueryFreeBusy(calendarIDs []string, timeMin, timeMax time.Time) (*calendar.FreeBusyResponse, error) {
	items := make([]*calendar.FreeBusyRequestItem, len(calendarIDs))
	for i, id := range calendarIDs {
		items[i] = &calendar.FreeBusyRequestItem{Id: id}
	}

	request := &calendar.FreeBusyRequest{
		TimeMin: timeMin.Format(time.RFC3339),
		TimeMax: timeMax.Format(time.RFC3339),
		Items:   items,
	}

	response, err := c.service.Freebusy.Query(request).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to query free/busy: %w", err)
	}

	return response, nil
}

// CreateEventFromDetails creates an event from basic details
func (c *Client) CreateEventFromDetails(calendarID, summary, description, location string,
	startTime, endTime time.Time, attendees []string, reminders []int) (*calendar.Event, error) {

	event := &calendar.Event{
		Summary:     summary,
		Description: description,
		Location:    location,
		Start: &calendar.EventDateTime{
			DateTime: startTime.Format(time.RFC3339),
			TimeZone: startTime.Location().String(),
		},
		End: &calendar.EventDateTime{
			DateTime: endTime.Format(time.RFC3339),
			TimeZone: endTime.Location().String(),
		},
	}

	// Add attendees
	if len(attendees) > 0 {
		eventAttendees := make([]*calendar.EventAttendee, len(attendees))
		for i, email := range attendees {
			eventAttendees[i] = &calendar.EventAttendee{
				Email: email,
			}
		}
		event.Attendees = eventAttendees
	}

	// Add reminders
	if len(reminders) > 0 {
		overrides := make([]*calendar.EventReminder, len(reminders))
		for i, minutes := range reminders {
			overrides[i] = &calendar.EventReminder{
				Method:  "popup",
				Minutes: int64(minutes),
			}
		}
		event.Reminders = &calendar.EventReminders{
			UseDefault: false,
			Overrides:  overrides,
		}
	}

	return c.CreateEvent(calendarID, event)
}

// GetCalendarByID gets a calendar by ID
func (c *Client) GetCalendarByID(calendarID string) (*calendar.CalendarListEntry, error) {
	cal, err := c.service.CalendarList.Get(calendarID).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get calendar: %w", err)
	}
	return cal, nil
}

// GetPrimaryCalendar gets the primary calendar
func (c *Client) GetPrimaryCalendar() (*calendar.CalendarListEntry, error) {
	return c.GetCalendarByID("primary")
}
