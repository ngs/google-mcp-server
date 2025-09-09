package calendar

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"go.ngs.io/google-mcp-server/auth"
	"go.ngs.io/google-mcp-server/server"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

// MultiAccountHandler implements the ServiceHandler interface with multi-account support
type MultiAccountHandler struct {
	accountManager *auth.AccountManager
	defaultClient  *Client // For backward compatibility
}

// NewMultiAccountHandler creates a new multi-account aware Calendar handler
func NewMultiAccountHandler(accountManager *auth.AccountManager, defaultClient *Client) *MultiAccountHandler {
	return &MultiAccountHandler{
		accountManager: accountManager,
		defaultClient:  defaultClient,
	}
}

// GetTools returns the available Calendar tools
func (h *MultiAccountHandler) GetTools() []server.Tool {
	// Return the same tools as the original handler
	return []server.Tool{
		{
			Name:        "calendar_list",
			Description: "List all accessible calendars",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"account": {
						Type:        "string",
						Description: "Email address of the account to use (optional)",
					},
				},
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
					"account": {
						Type:        "string",
						Description: "Email address of the account to use (optional)",
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
					"account": {
						Type:        "string",
						Description: "Email address of the account to use (optional)",
					},
				},
				Required: []string{"calendar_id", "summary", "start_time", "end_time"},
			},
		},
		{
			Name:        "calendar_events_list_all_accounts",
			Description: "List events from all authenticated accounts for today or specified date range",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"time_min": {
						Type:        "string",
						Description: "Start time (RFC3339 format, defaults to today)",
					},
					"time_max": {
						Type:        "string",
						Description: "End time (RFC3339 format, defaults to end of today)",
					},
					"max_results": {
						Type:        "number",
						Description: "Maximum number of events per account",
					},
				},
			},
		},
	}
}

// HandleToolCall handles a tool call
func (h *MultiAccountHandler) HandleToolCall(ctx context.Context, name string, arguments json.RawMessage) (interface{}, error) {
	switch name {
	case "calendar_list":
		return h.handleCalendarList(ctx, arguments)
	case "calendar_events_list":
		return h.handleEventsList(ctx, arguments)
	case "calendar_event_create":
		return h.handleEventCreate(ctx, arguments)
	case "calendar_events_list_all_accounts":
		return h.handleEventsListAllAccounts(ctx, arguments)
	default:
		// Fall back to default client for other operations
		if h.defaultClient != nil {
			origHandler := NewHandler(h.defaultClient)
			return origHandler.HandleToolCall(ctx, name, arguments)
		}
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

// getClientForAccount gets or creates a calendar client for the specified account
func (h *MultiAccountHandler) getClientForAccount(ctx context.Context, email string) (*Client, error) {
	// If no email specified, use default client
	if email == "" && h.defaultClient != nil {
		return h.defaultClient, nil
	}

	// Get account from manager
	account, err := h.accountManager.GetAccount(email)
	if err != nil {
		// If specific account not found, try to find one
		if email == "" {
			accounts := h.accountManager.ListAccounts()
			if len(accounts) > 0 {
				account = accounts[0]
			} else {
				return nil, fmt.Errorf("no authenticated accounts available")
			}
		} else {
			return nil, err
		}
	}

	// Create calendar service for this account
	service, err := calendar.NewService(ctx, option.WithHTTPClient(account.OAuthClient.GetHTTPClient()))
	if err != nil {
		return nil, fmt.Errorf("failed to create calendar service: %w", err)
	}

	return &Client{service: service}, nil
}

// handleCalendarList lists calendars for the specified account
func (h *MultiAccountHandler) handleCalendarList(ctx context.Context, arguments json.RawMessage) (interface{}, error) {
	var args struct {
		Account string `json:"account"`
	}
	if err := json.Unmarshal(arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	client, err := h.getClientForAccount(ctx, args.Account)
	if err != nil {
		return nil, err
	}

	calendars, err := client.ListCalendars()
	if err != nil {
		return nil, fmt.Errorf("failed to list calendars: %w", err)
	}

	return map[string]interface{}{
		"calendars": calendars,
	}, nil
}

// handleEventsList lists events for the specified account
func (h *MultiAccountHandler) handleEventsList(ctx context.Context, arguments json.RawMessage) (interface{}, error) {
	var args struct {
		CalendarID string `json:"calendar_id"`
		TimeMin    string `json:"time_min"`
		TimeMax    string `json:"time_max"`
		MaxResults int64  `json:"max_results"`
		Account    string `json:"account"`
	}
	if err := json.Unmarshal(arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	// Determine which account to use based on calendar_id
	accountEmail := args.Account
	if accountEmail == "" && strings.Contains(args.CalendarID, "@") {
		// Try to match account based on calendar ID
		accounts := h.accountManager.ListAccounts()
		for _, acc := range accounts {
			if strings.Contains(args.CalendarID, acc.Email) || args.CalendarID == acc.Email {
				accountEmail = acc.Email
				break
			}
		}
	}

	client, err := h.getClientForAccount(ctx, accountEmail)
	if err != nil {
		return nil, err
	}

	// If calendar_id looks like an email and matches an account, use "primary" instead
	calendarID := args.CalendarID
	if accountEmail != "" && args.CalendarID == accountEmail {
		calendarID = "primary"
	}

	// Parse time strings
	var timeMin, timeMax time.Time
	if args.TimeMin != "" {
		timeMin, _ = time.Parse(time.RFC3339, args.TimeMin)
	}
	if args.TimeMax != "" {
		timeMax, _ = time.Parse(time.RFC3339, args.TimeMax)
	}

	events, err := client.ListEvents(calendarID, timeMin, timeMax, args.MaxResults)
	if err != nil {
		return nil, fmt.Errorf("failed to list events: %w", err)
	}

	return map[string]interface{}{
		"events":  events,
		"account": accountEmail,
	}, nil
}

// handleEventCreate creates an event
func (h *MultiAccountHandler) handleEventCreate(ctx context.Context, arguments json.RawMessage) (interface{}, error) {
	var args struct {
		CalendarID  string   `json:"calendar_id"`
		Summary     string   `json:"summary"`
		Description string   `json:"description"`
		Location    string   `json:"location"`
		StartTime   string   `json:"start_time"`
		EndTime     string   `json:"end_time"`
		Attendees   []string `json:"attendees"`
		Reminders   []int    `json:"reminders"`
		Account     string   `json:"account"`
	}
	if err := json.Unmarshal(arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	client, err := h.getClientForAccount(ctx, args.Account)
	if err != nil {
		return nil, err
	}

	// Create calendar event object
	event := &calendar.Event{
		Summary:     args.Summary,
		Description: args.Description,
		Location:    args.Location,
		Start: &calendar.EventDateTime{
			DateTime: args.StartTime,
		},
		End: &calendar.EventDateTime{
			DateTime: args.EndTime,
		},
	}

	// Add attendees if provided
	if len(args.Attendees) > 0 {
		var attendees []*calendar.EventAttendee
		for _, email := range args.Attendees {
			attendees = append(attendees, &calendar.EventAttendee{
				Email: email,
			})
		}
		event.Attendees = attendees
	}

	// Add reminders if provided
	if len(args.Reminders) > 0 {
		var overrides []*calendar.EventReminder
		for _, minutes := range args.Reminders {
			overrides = append(overrides, &calendar.EventReminder{
				Method:  "popup",
				Minutes: int64(minutes),
			})
		}
		event.Reminders = &calendar.EventReminders{
			UseDefault: false,
			Overrides:  overrides,
		}
	}

	createdEvent, err := client.CreateEvent(args.CalendarID, event)
	if err != nil {
		return nil, fmt.Errorf("failed to create event: %w", err)
	}

	return map[string]interface{}{
		"event":   createdEvent,
		"message": fmt.Sprintf("Event '%s' created successfully", args.Summary),
	}, nil
}

// handleEventsListAllAccounts lists events from all accounts
func (h *MultiAccountHandler) handleEventsListAllAccounts(ctx context.Context, arguments json.RawMessage) (interface{}, error) {
	var args struct {
		TimeMin    string `json:"time_min"`
		TimeMax    string `json:"time_max"`
		MaxResults int64  `json:"max_results"`
	}
	if err := json.Unmarshal(arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	// Default to today if no time range specified
	if args.TimeMin == "" {
		now := time.Now()
		loc, _ := time.LoadLocation("Asia/Tokyo")
		todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
		args.TimeMin = todayStart.Format(time.RFC3339)
	}
	if args.TimeMax == "" {
		minTime, _ := time.Parse(time.RFC3339, args.TimeMin)
		args.TimeMax = minTime.Add(24 * time.Hour).Format(time.RFC3339)
	}
	if args.MaxResults == 0 {
		args.MaxResults = 50
	}

	// Get all accounts
	accounts := h.accountManager.ListAccounts()
	if len(accounts) == 0 {
		return nil, fmt.Errorf("no authenticated accounts available")
	}

	// Collect events from all accounts
	allEvents := make(map[string]interface{})

	for _, account := range accounts {
		client, err := h.getClientForAccount(ctx, account.Email)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to get client for %s: %v\n", account.Email, err)
			continue
		}

		// Parse time strings for this call
		timeMin, _ := time.Parse(time.RFC3339, args.TimeMin)
		timeMax, _ := time.Parse(time.RFC3339, args.TimeMax)

		// Get events from primary calendar
		events, err := client.ListEvents("primary", timeMin, timeMax, args.MaxResults)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to list events for %s: %v\n", account.Email, err)
			continue
		}

		if len(events) > 0 {
			allEvents[account.Email] = map[string]interface{}{
				"account_name": account.Name,
				"events":       events,
			}
		}
	}

	return map[string]interface{}{
		"accounts": allEvents,
		"time_range": map[string]string{
			"start": args.TimeMin,
			"end":   args.TimeMax,
		},
		"total_accounts": len(accounts),
	}, nil
}

// GetResources returns available resources
func (h *MultiAccountHandler) GetResources() []server.Resource {
	if h.defaultClient != nil {
		origHandler := NewHandler(h.defaultClient)
		return origHandler.GetResources()
	}
	return []server.Resource{}
}

// HandleResourceCall handles resource calls
func (h *MultiAccountHandler) HandleResourceCall(ctx context.Context, uri string) (interface{}, error) {
	if h.defaultClient != nil {
		origHandler := NewHandler(h.defaultClient)
		return origHandler.HandleResourceCall(ctx, uri)
	}
	return nil, fmt.Errorf("no default client available")
}
