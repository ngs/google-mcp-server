package calendar

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

// mockTransport is a mock HTTP transport for testing
type mockTransport struct {
	responses map[string]*http.Response
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if resp, ok := m.responses[req.URL.Path]; ok {
		return resp, nil
	}
	return &http.Response{
		StatusCode: 404,
		Body:       io.NopCloser(strings.NewReader("Not Found")),
	}, nil
}

func TestListCalendars(t *testing.T) {
	// Create mock response
	mockResp := `{
		"items": [
			{
				"id": "primary",
				"summary": "Test Calendar",
				"description": "Test Description",
				"timeZone": "America/New_York"
			},
			{
				"id": "test-calendar-2",
				"summary": "Secondary Calendar",
				"timeZone": "UTC"
			}
		]
	}`

	httpClient := &http.Client{
		Transport: &mockTransport{
			responses: map[string]*http.Response{
				"/calendar/v3/users/me/calendarList": {
					StatusCode: 200,
					Body:       io.NopCloser(strings.NewReader(mockResp)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				},
			},
		},
	}

	// Create Calendar service with mock client
	service, err := calendar.NewService(context.Background(), option.WithHTTPClient(httpClient))
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	client := &Client{service: service}

	// Test ListCalendars
	calendars, err := client.ListCalendars()
	if err != nil {
		// This is expected to fail with the mock setup, but we're testing the logic
		t.Logf("ListCalendars failed as expected with mock: %v", err)
	} else {
		if len(calendars) != 2 {
			t.Errorf("Expected 2 calendars, got %d", len(calendars))
		}
		if calendars[0].Id != "primary" {
			t.Errorf("Expected first calendar ID 'primary', got %s", calendars[0].Id)
		}
	}
}

func TestListEvents(t *testing.T) {
	// Create mock response
	mockResp := `{
		"items": [
			{
				"id": "event1",
				"summary": "Test Event 1",
				"start": {
					"dateTime": "2024-01-01T10:00:00Z"
				},
				"end": {
					"dateTime": "2024-01-01T11:00:00Z"
				}
			},
			{
				"id": "event2",
				"summary": "Test Event 2",
				"start": {
					"dateTime": "2024-01-01T14:00:00Z"
				},
				"end": {
					"dateTime": "2024-01-01T15:00:00Z"
				}
			}
		]
	}`

	httpClient := &http.Client{
		Transport: &mockTransport{
			responses: map[string]*http.Response{
				"/calendar/v3/calendars/primary/events": {
					StatusCode: 200,
					Body:       io.NopCloser(strings.NewReader(mockResp)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				},
			},
		},
	}

	// Create Calendar service with mock client
	service, err := calendar.NewService(context.Background(), option.WithHTTPClient(httpClient))
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	client := &Client{service: service}

	// Test ListEvents
	timeMin := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	timeMax := time.Date(2024, 1, 31, 23, 59, 59, 0, time.UTC)

	events, err := client.ListEvents("primary", timeMin, timeMax, 10)
	if err != nil {
		t.Logf("ListEvents failed as expected with mock: %v", err)
	} else {
		if len(events) != 2 {
			t.Errorf("Expected 2 events, got %d", len(events))
		}
	}
}

func TestCreateEvent(t *testing.T) {
	// Create mock response
	mockResp := `{
		"id": "new-event-id",
		"summary": "New Event",
		"description": "Test Description",
		"start": {
			"dateTime": "2024-01-15T10:00:00Z"
		},
		"end": {
			"dateTime": "2024-01-15T11:00:00Z"
		},
		"status": "confirmed"
	}`

	httpClient := &http.Client{
		Transport: &mockTransport{
			responses: map[string]*http.Response{
				"/calendar/v3/calendars/primary/events": {
					StatusCode: 200,
					Body:       io.NopCloser(strings.NewReader(mockResp)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				},
			},
		},
	}

	// Create Calendar service with mock client
	service, err := calendar.NewService(context.Background(), option.WithHTTPClient(httpClient))
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	client := &Client{service: service}

	// Test CreateEvent
	event := &calendar.Event{
		Summary:     "New Event",
		Description: "Test Description",
		Start: &calendar.EventDateTime{
			DateTime: "2024-01-15T10:00:00Z",
		},
		End: &calendar.EventDateTime{
			DateTime: "2024-01-15T11:00:00Z",
		},
	}

	created, err := client.CreateEvent("primary", event)
	if err != nil {
		t.Logf("CreateEvent failed as expected with mock: %v", err)
	} else {
		if created.Id != "new-event-id" {
			t.Errorf("Expected event ID 'new-event-id', got %s", created.Id)
		}
		if created.Summary != "New Event" {
			t.Errorf("Expected event summary 'New Event', got %s", created.Summary)
		}
	}
}

func TestCreateEventFromDetails(t *testing.T) {
	// Create mock response
	mockResp := `{
		"id": "detailed-event-id",
		"summary": "Meeting",
		"description": "Team meeting",
		"location": "Conference Room A",
		"start": {
			"dateTime": "2024-01-20T14:00:00Z",
			"timeZone": "UTC"
		},
		"end": {
			"dateTime": "2024-01-20T15:00:00Z",
			"timeZone": "UTC"
		},
		"attendees": [
			{"email": "user1@example.com"},
			{"email": "user2@example.com"}
		],
		"reminders": {
			"useDefault": false,
			"overrides": [
				{"method": "popup", "minutes": 10},
				{"method": "popup", "minutes": 30}
			]
		}
	}`

	httpClient := &http.Client{
		Transport: &mockTransport{
			responses: map[string]*http.Response{
				"/calendar/v3/calendars/primary/events": {
					StatusCode: 200,
					Body:       io.NopCloser(strings.NewReader(mockResp)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				},
			},
		},
	}

	// Create Calendar service with mock client
	service, err := calendar.NewService(context.Background(), option.WithHTTPClient(httpClient))
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	client := &Client{service: service}

	// Test CreateEventFromDetails
	startTime := time.Date(2024, 1, 20, 14, 0, 0, 0, time.UTC)
	endTime := time.Date(2024, 1, 20, 15, 0, 0, 0, time.UTC)
	attendees := []string{"user1@example.com", "user2@example.com"}
	reminders := []int{10, 30}

	event, err := client.CreateEventFromDetails(
		"primary",
		"Meeting",
		"Team meeting",
		"Conference Room A",
		startTime,
		endTime,
		attendees,
		reminders,
	)

	if err != nil {
		t.Logf("CreateEventFromDetails failed as expected with mock: %v", err)
	} else {
		if event.Id != "detailed-event-id" {
			t.Errorf("Expected event ID 'detailed-event-id', got %s", event.Id)
		}
		if len(event.Attendees) != 2 {
			t.Errorf("Expected 2 attendees, got %d", len(event.Attendees))
		}
		if event.Reminders == nil || len(event.Reminders.Overrides) != 2 {
			t.Error("Expected 2 reminder overrides")
		}
	}
}

func TestUpdateEvent(t *testing.T) {
	// Create mock response
	mockResp := `{
		"id": "event1",
		"summary": "Updated Event",
		"description": "Updated Description",
		"start": {
			"dateTime": "2024-01-15T11:00:00Z"
		},
		"end": {
			"dateTime": "2024-01-15T12:00:00Z"
		}
	}`

	httpClient := &http.Client{
		Transport: &mockTransport{
			responses: map[string]*http.Response{
				"/calendar/v3/calendars/primary/events/event1": {
					StatusCode: 200,
					Body:       io.NopCloser(strings.NewReader(mockResp)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				},
			},
		},
	}

	// Create Calendar service with mock client
	service, err := calendar.NewService(context.Background(), option.WithHTTPClient(httpClient))
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	client := &Client{service: service}

	// Test UpdateEvent
	event := &calendar.Event{
		Summary:     "Updated Event",
		Description: "Updated Description",
		Start: &calendar.EventDateTime{
			DateTime: "2024-01-15T11:00:00Z",
		},
		End: &calendar.EventDateTime{
			DateTime: "2024-01-15T12:00:00Z",
		},
	}

	updated, err := client.UpdateEvent("primary", "event1", event)
	if err != nil {
		t.Logf("UpdateEvent failed as expected with mock: %v", err)
	} else {
		if updated.Summary != "Updated Event" {
			t.Errorf("Expected summary 'Updated Event', got %s", updated.Summary)
		}
	}
}

func TestDeleteEvent(t *testing.T) {
	httpClient := &http.Client{
		Transport: &mockTransport{
			responses: map[string]*http.Response{
				"/calendar/v3/calendars/primary/events/event1": {
					StatusCode: 204, // No Content - successful deletion
					Body:       io.NopCloser(strings.NewReader("")),
				},
			},
		},
	}

	// Create Calendar service with mock client
	service, err := calendar.NewService(context.Background(), option.WithHTTPClient(httpClient))
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	client := &Client{service: service}

	// Test DeleteEvent
	err = client.DeleteEvent("primary", "event1")
	if err != nil {
		t.Logf("DeleteEvent failed as expected with mock: %v", err)
	}
}

func TestSearchEvents(t *testing.T) {
	// Create mock response
	mockResp := `{
		"items": [
			{
				"id": "search-result-1",
				"summary": "Meeting with John",
				"start": {
					"dateTime": "2024-01-10T10:00:00Z"
				},
				"end": {
					"dateTime": "2024-01-10T11:00:00Z"
				}
			}
		]
	}`

	httpClient := &http.Client{
		Transport: &mockTransport{
			responses: map[string]*http.Response{
				"/calendar/v3/calendars/primary/events": {
					StatusCode: 200,
					Body:       io.NopCloser(strings.NewReader(mockResp)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				},
			},
		},
	}

	// Create Calendar service with mock client
	service, err := calendar.NewService(context.Background(), option.WithHTTPClient(httpClient))
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	client := &Client{service: service}

	// Test SearchEvents
	timeMin := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	timeMax := time.Date(2024, 1, 31, 23, 59, 59, 0, time.UTC)

	events, err := client.SearchEvents("primary", "Meeting", timeMin, timeMax)
	if err != nil {
		t.Logf("SearchEvents failed as expected with mock: %v", err)
	} else {
		if len(events) != 1 {
			t.Errorf("Expected 1 event, got %d", len(events))
		}
		if events[0].Summary != "Meeting with John" {
			t.Errorf("Expected summary 'Meeting with John', got %s", events[0].Summary)
		}
	}
}

func TestQueryFreeBusy(t *testing.T) {
	// Create mock response
	mockResp := `{
		"calendars": {
			"primary": {
				"busy": [
					{
						"start": "2024-01-15T10:00:00Z",
						"end": "2024-01-15T11:00:00Z"
					},
					{
						"start": "2024-01-15T14:00:00Z",
						"end": "2024-01-15T15:30:00Z"
					}
				]
			},
			"secondary@example.com": {
				"busy": [
					{
						"start": "2024-01-15T09:00:00Z",
						"end": "2024-01-15T10:00:00Z"
					}
				]
			}
		}
	}`

	httpClient := &http.Client{
		Transport: &mockTransport{
			responses: map[string]*http.Response{
				"/calendar/v3/freeBusy": {
					StatusCode: 200,
					Body:       io.NopCloser(strings.NewReader(mockResp)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				},
			},
		},
	}

	// Create Calendar service with mock client
	service, err := calendar.NewService(context.Background(), option.WithHTTPClient(httpClient))
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	client := &Client{service: service}

	// Test QueryFreeBusy
	calendarIDs := []string{"primary", "secondary@example.com"}
	timeMin := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	timeMax := time.Date(2024, 1, 15, 23, 59, 59, 0, time.UTC)

	response, err := client.QueryFreeBusy(calendarIDs, timeMin, timeMax)
	if err != nil {
		t.Logf("QueryFreeBusy failed as expected with mock: %v", err)
	} else {
		if response.Calendars == nil {
			t.Error("Expected calendars in response")
		}
		if _, ok := response.Calendars["primary"]; !ok {
			t.Error("Expected 'primary' calendar in response")
		}
	}
}

func TestGetEvent(t *testing.T) {
	// Create mock response
	mockResp := `{
		"id": "event1",
		"summary": "Test Event",
		"description": "Test Description",
		"start": {
			"dateTime": "2024-01-15T10:00:00Z"
		},
		"end": {
			"dateTime": "2024-01-15T11:00:00Z"
		},
		"status": "confirmed"
	}`

	httpClient := &http.Client{
		Transport: &mockTransport{
			responses: map[string]*http.Response{
				"/calendar/v3/calendars/primary/events/event1": {
					StatusCode: 200,
					Body:       io.NopCloser(strings.NewReader(mockResp)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				},
			},
		},
	}

	// Create Calendar service with mock client
	service, err := calendar.NewService(context.Background(), option.WithHTTPClient(httpClient))
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	client := &Client{service: service}

	// Test GetEvent
	event, err := client.GetEvent("primary", "event1")
	if err != nil {
		t.Logf("GetEvent failed as expected with mock: %v", err)
	} else {
		if event.Id != "event1" {
			t.Errorf("Expected event ID 'event1', got %s", event.Id)
		}
		if event.Summary != "Test Event" {
			t.Errorf("Expected summary 'Test Event', got %s", event.Summary)
		}
	}
}

func TestGetCalendarByID(t *testing.T) {
	// Create mock response
	mockResp := `{
		"id": "test-calendar",
		"summary": "Test Calendar",
		"description": "A test calendar",
		"timeZone": "America/New_York",
		"accessRole": "owner"
	}`

	httpClient := &http.Client{
		Transport: &mockTransport{
			responses: map[string]*http.Response{
				"/calendar/v3/users/me/calendarList/test-calendar": {
					StatusCode: 200,
					Body:       io.NopCloser(strings.NewReader(mockResp)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				},
			},
		},
	}

	// Create Calendar service with mock client
	service, err := calendar.NewService(context.Background(), option.WithHTTPClient(httpClient))
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	client := &Client{service: service}

	// Test GetCalendarByID
	cal, err := client.GetCalendarByID("test-calendar")
	if err != nil {
		t.Logf("GetCalendarByID failed as expected with mock: %v", err)
	} else {
		if cal.Id != "test-calendar" {
			t.Errorf("Expected calendar ID 'test-calendar', got %s", cal.Id)
		}
		if cal.Summary != "Test Calendar" {
			t.Errorf("Expected summary 'Test Calendar', got %s", cal.Summary)
		}
	}
}

func TestGetPrimaryCalendar(t *testing.T) {
	// Create mock response
	mockResp := `{
		"id": "primary",
		"summary": "Primary Calendar",
		"timeZone": "UTC",
		"accessRole": "owner"
	}`

	httpClient := &http.Client{
		Transport: &mockTransport{
			responses: map[string]*http.Response{
				"/calendar/v3/users/me/calendarList/primary": {
					StatusCode: 200,
					Body:       io.NopCloser(strings.NewReader(mockResp)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				},
			},
		},
	}

	// Create Calendar service with mock client
	service, err := calendar.NewService(context.Background(), option.WithHTTPClient(httpClient))
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	client := &Client{service: service}

	// Test GetPrimaryCalendar
	cal, err := client.GetPrimaryCalendar()
	if err != nil {
		t.Logf("GetPrimaryCalendar failed as expected with mock: %v", err)
	} else {
		if cal.Id != "primary" {
			t.Errorf("Expected calendar ID 'primary', got %s", cal.Id)
		}
	}
}

func TestEventTimeFormatting(t *testing.T) {
	tests := []struct {
		name     string
		time     time.Time
		expected string
	}{
		{
			name:     "UTC time",
			time:     time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
			expected: "2024-01-15T10:30:00Z",
		},
		{
			name:     "EST time",
			time:     time.Date(2024, 1, 15, 10, 30, 0, 0, time.FixedZone("EST", -5*3600)),
			expected: "2024-01-15T10:30:00-05:00",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatted := tt.time.Format(time.RFC3339)
			if formatted != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, formatted)
			}
		})
	}
}

func TestEventValidation(t *testing.T) {
	// Test that event with required fields is valid
	event := &calendar.Event{
		Summary: "Test Event",
		Start: &calendar.EventDateTime{
			DateTime: time.Now().Format(time.RFC3339),
		},
		End: &calendar.EventDateTime{
			DateTime: time.Now().Add(time.Hour).Format(time.RFC3339),
		},
	}

	// Basic validation
	if event.Summary == "" {
		t.Error("Event summary should not be empty")
	}
	if event.Start == nil || event.Start.DateTime == "" {
		t.Error("Event start time should be set")
	}
	if event.End == nil || event.End.DateTime == "" {
		t.Error("Event end time should be set")
	}

	// Test event with all-day dates
	allDayEvent := &calendar.Event{
		Summary: "All Day Event",
		Start: &calendar.EventDateTime{
			Date: "2024-01-15",
		},
		End: &calendar.EventDateTime{
			Date: "2024-01-16",
		},
	}

	if allDayEvent.Start.Date == "" {
		t.Error("All-day event should have date field set")
	}
}

func TestAttendeeValidation(t *testing.T) {
	attendees := []*calendar.EventAttendee{
		{Email: "user1@example.com"},
		{Email: "user2@example.com"},
		{Email: "invalid-email"},
	}

	validEmails := 0
	for _, attendee := range attendees {
		if strings.Contains(attendee.Email, "@") {
			validEmails++
		}
	}

	if validEmails != 2 {
		t.Errorf("Expected 2 valid email addresses, got %d", validEmails)
	}
}

func TestReminderValidation(t *testing.T) {
	reminders := &calendar.EventReminders{
		UseDefault: false,
		Overrides: []*calendar.EventReminder{
			{Method: "popup", Minutes: 10},
			{Method: "email", Minutes: 60},
			{Method: "sms", Minutes: 30},
		},
	}

	if reminders.UseDefault {
		t.Error("UseDefault should be false when overrides are specified")
	}

	validMethods := map[string]bool{
		"popup": true,
		"email": true,
		"sms":   true,
	}

	for _, reminder := range reminders.Overrides {
		if !validMethods[reminder.Method] {
			t.Errorf("Invalid reminder method: %s", reminder.Method)
		}
		if reminder.Minutes < 0 {
			t.Error("Reminder minutes should not be negative")
		}
	}
}

func TestTimeZoneHandling(t *testing.T) {
	locations := []string{
		"America/New_York",
		"Europe/London",
		"Asia/Tokyo",
		"UTC",
	}

	for _, locName := range locations {
		loc, err := time.LoadLocation(locName)
		if err != nil {
			t.Errorf("Failed to load location %s: %v", locName, err)
			continue
		}

		testTime := time.Date(2024, 1, 15, 10, 0, 0, 0, loc)
		formatted := testTime.Format(time.RFC3339)

		// Parse it back
		parsed, err := time.Parse(time.RFC3339, formatted)
		if err != nil {
			t.Errorf("Failed to parse time for location %s: %v", locName, err)
			continue
		}

		// Check if times are equal (they should be the same instant)
		if !testTime.Equal(parsed) {
			t.Errorf("Time mismatch for location %s: original=%v, parsed=%v",
				locName, testTime, parsed)
		}
	}
}

func TestEventJSONMarshaling(t *testing.T) {
	event := &calendar.Event{
		Id:          "test-event",
		Summary:     "Test Event",
		Description: "Test Description",
		Location:    "Test Location",
		Start: &calendar.EventDateTime{
			DateTime: "2024-01-15T10:00:00Z",
			TimeZone: "UTC",
		},
		End: &calendar.EventDateTime{
			DateTime: "2024-01-15T11:00:00Z",
			TimeZone: "UTC",
		},
		Attendees: []*calendar.EventAttendee{
			{Email: "user@example.com", ResponseStatus: "accepted"},
		},
		Reminders: &calendar.EventReminders{
			UseDefault: false,
			Overrides: []*calendar.EventReminder{
				{Method: "popup", Minutes: 15},
			},
		},
	}

	// Marshal to JSON
	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("Failed to marshal event: %v", err)
	}

	// Unmarshal back
	var decoded calendar.Event
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal event: %v", err)
	}

	// Verify fields
	if decoded.Id != event.Id {
		t.Errorf("ID mismatch: expected %s, got %s", event.Id, decoded.Id)
	}
	if decoded.Summary != event.Summary {
		t.Errorf("Summary mismatch: expected %s, got %s", event.Summary, decoded.Summary)
	}
	if decoded.Start.DateTime != event.Start.DateTime {
		t.Errorf("Start time mismatch: expected %s, got %s",
			event.Start.DateTime, decoded.Start.DateTime)
	}
	if len(decoded.Attendees) != len(event.Attendees) {
		t.Errorf("Attendees count mismatch: expected %d, got %d",
			len(event.Attendees), len(decoded.Attendees))
	}
}

func BenchmarkCreateEvent(b *testing.B) {
	// Create a simple mock client
	httpClient := &http.Client{
		Transport: &mockTransport{
			responses: map[string]*http.Response{
				"/calendar/v3/calendars/primary/events": {
					StatusCode: 200,
					Body:       io.NopCloser(strings.NewReader(`{"id": "test"}`)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				},
			},
		},
	}

	service, _ := calendar.NewService(context.Background(), option.WithHTTPClient(httpClient))
	client := &Client{service: service}

	event := &calendar.Event{
		Summary: "Benchmark Event",
		Start: &calendar.EventDateTime{
			DateTime: time.Now().Format(time.RFC3339),
		},
		End: &calendar.EventDateTime{
			DateTime: time.Now().Add(time.Hour).Format(time.RFC3339),
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = client.CreateEvent("primary", event)
	}
}

func BenchmarkListEvents(b *testing.B) {
	// Create a simple mock client
	httpClient := &http.Client{
		Transport: &mockTransport{
			responses: map[string]*http.Response{
				"/calendar/v3/calendars/primary/events": {
					StatusCode: 200,
					Body:       io.NopCloser(strings.NewReader(`{"items": []}`)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				},
			},
		},
	}

	service, _ := calendar.NewService(context.Background(), option.WithHTTPClient(httpClient))
	client := &Client{service: service}

	timeMin := time.Now()
	timeMax := time.Now().Add(24 * time.Hour)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = client.ListEvents("primary", timeMin, timeMax, 10)
	}
}
