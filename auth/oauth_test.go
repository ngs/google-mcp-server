package auth

import (
	"testing"
)

func TestDefaultScopes(t *testing.T) {
	scopes := DefaultScopes()
	
	if len(scopes) == 0 {
		t.Error("DefaultScopes returned empty slice")
	}
	
	expectedScopes := []string{
		"https://www.googleapis.com/auth/calendar",
		"https://www.googleapis.com/auth/drive",
		"https://www.googleapis.com/auth/gmail.modify",
		"https://www.googleapis.com/auth/photoslibrary",
		"https://www.googleapis.com/auth/spreadsheets",
		"https://www.googleapis.com/auth/documents",
		"https://www.googleapis.com/auth/userinfo.email",
		"https://www.googleapis.com/auth/userinfo.profile",
	}
	
	if len(scopes) != len(expectedScopes) {
		t.Errorf("Expected %d scopes, got %d", len(expectedScopes), len(scopes))
	}
	
	// Check each scope
	scopeMap := make(map[string]bool)
	for _, scope := range scopes {
		scopeMap[scope] = true
	}
	
	for _, expected := range expectedScopes {
		if !scopeMap[expected] {
			t.Errorf("Missing expected scope: %s", expected)
		}
	}
}

func TestOAuthConfig(t *testing.T) {
	config := OAuthConfig{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		RedirectURI:  "http://localhost:8080/callback",
		TokenFile:    "/tmp/test-token.json",
		Scopes:       DefaultScopes(),
	}
	
	if config.ClientID != "test-client-id" {
		t.Errorf("Expected ClientID to be 'test-client-id', got %s", config.ClientID)
	}
	
	if config.ClientSecret != "test-client-secret" {
		t.Errorf("Expected ClientSecret to be 'test-client-secret', got %s", config.ClientSecret)
	}
}