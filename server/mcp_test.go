package server

import (
	"testing"

	"go.ngs.io/google-mcp-server/config"
)

func TestNewMCPServer(t *testing.T) {
	cfg := &config.Config{
		Services: config.ServicesConfig{
			Calendar: config.CalendarConfig{Enabled: true},
		},
		Global: config.GlobalConfig{
			LogLevel: "info",
		},
	}

	server := NewMCPServer(cfg)

	if server == nil {
		t.Fatal("NewMCPServer returned nil")
	}

	if server.config != cfg {
		t.Error("Server config does not match provided config")
	}

	if server.services == nil {
		t.Error("Server services map is nil")
	}

	if server.tools == nil {
		t.Error("Server tools slice is nil")
	}

	if server.resources == nil {
		t.Error("Server resources slice is nil")
	}
}

func TestTool(t *testing.T) {
	tool := Tool{
		Name:        "test_tool",
		Description: "A test tool",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"param1": {
					Type:        "string",
					Description: "First parameter",
				},
			},
			Required: []string{"param1"},
		},
	}

	if tool.Name != "test_tool" {
		t.Errorf("Expected tool name to be 'test_tool', got %s", tool.Name)
	}

	if tool.InputSchema.Type != "object" {
		t.Errorf("Expected input schema type to be 'object', got %s", tool.InputSchema.Type)
	}

	if len(tool.InputSchema.Required) != 1 {
		t.Errorf("Expected 1 required parameter, got %d", len(tool.InputSchema.Required))
	}
}

func TestResource(t *testing.T) {
	resource := Resource{
		URI:         "test://resource",
		Name:        "Test Resource",
		Description: "A test resource",
		MimeType:    "application/json",
	}

	if resource.URI != "test://resource" {
		t.Errorf("Expected resource URI to be 'test://resource', got %s", resource.URI)
	}

	if resource.MimeType != "application/json" {
		t.Errorf("Expected MIME type to be 'application/json', got %s", resource.MimeType)
	}
}
