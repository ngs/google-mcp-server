package auth

import (
	"fmt"
	"strings"
)

// HandleServiceError provides consistent error handling across all services
func HandleServiceError(err error, service string, account string) error {
	if err == nil {
		return nil
	}

	// Check if it's a scope error
	if scopeErr, ok := err.(*ScopeError); ok {
		return fmt.Errorf(
			"%v\n\nTo fix this:\n"+
				"1. Run: accounts_refresh\n"+
				"2. Select account: %s\n"+
				"3. Authorize the required scopes",
			scopeErr, account,
		)
	}

	// Check if API is disabled
	if IsAPIDisabledError(err) {
		apiURL := getAPIEnableURL(service)
		return fmt.Errorf(
			"Google %s API is not enabled for this project.\n\n"+
				"Account: %s\n\n"+
				"To fix this:\n"+
				"1. Enable the API at: %s\n"+
				"2. Wait a few minutes for the change to propagate\n"+
				"3. Re-authenticate using: accounts_refresh\n"+
				"4. Select account: %s",
			strings.Title(service), account, apiURL, account,
		)
	}

	// Check for insufficient permissions
	if isInsufficientPermissionsError(err) {
		return fmt.Errorf(
			"Insufficient permissions for %s API.\n\n"+
				"Account: %s\n\n"+
				"This error usually means:\n"+
				"- Missing required OAuth scopes\n"+
				"- API not enabled in your project\n"+
				"- Account doesn't have access to the resource\n\n"+
				"To fix this:\n"+
				"1. Run: accounts_refresh\n"+
				"2. Select account: %s\n"+
				"3. Authorize all requested permissions",
			strings.Title(service), account, account,
		)
	}

	// Return original error if not a known type
	return err
}

func getAPIEnableURL(service string) string {
	urls := map[string]string{
		"calendar": "https://console.cloud.google.com/apis/library/calendar-json.googleapis.com",
		"drive":    "https://console.cloud.google.com/apis/library/drive.googleapis.com",
		"gmail":    "https://console.cloud.google.com/apis/library/gmail.googleapis.com",
		"sheets":   "https://console.cloud.google.com/apis/library/sheets.googleapis.com",
		"docs":     "https://console.cloud.google.com/apis/library/docs.googleapis.com",
		"slides":   "https://console.cloud.google.com/apis/library/slides.googleapis.com",
	}

	if url, ok := urls[service]; ok {
		return url
	}
	return "https://console.cloud.google.com/apis/library"
}

func isInsufficientPermissionsError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "403") &&
		(strings.Contains(errStr, "insufficientPermissions") ||
			strings.Contains(errStr, "Insufficient Permission") ||
			strings.Contains(errStr, "PERMISSION_DENIED"))
}
