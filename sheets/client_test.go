package sheets

import (
	"context"
	"testing"
)

func TestNewClientWithNilOAuth(t *testing.T) {
	client, err := NewClient(context.Background(), nil)
	if err == nil {
		t.Fatal("expected an error when the OAuth client is nil")
	}
	if client != nil {
		t.Errorf("expected no client, got %v", client)
	}
}
