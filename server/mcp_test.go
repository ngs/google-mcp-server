package server

import (
	"encoding/json"
	"strings"
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

// TestPropertyOmitsEmptyNestedFields makes sure the nested object support does
// not change the JSON emitted for the properties that do not use it.
func TestPropertyOmitsEmptyNestedFields(t *testing.T) {
	encoded, err := json.Marshal(Property{
		Type:        "string",
		Description: "Spreadsheet ID",
	})
	if err != nil {
		t.Fatalf("Failed to marshal property: %v", err)
	}

	got := string(encoded)
	for _, key := range []string{"properties", "required", "items", "enum"} {
		if strings.Contains(got, key) {
			t.Errorf("Property JSON should not contain %q, got %s", key, got)
		}
	}
}

// TestPropertyEncodesNestedObject covers a property that declares its own
// object schema, as the cell formatting tool does.
func TestPropertyEncodesNestedObject(t *testing.T) {
	encoded, err := json.Marshal(Property{
		Type:        "object",
		Description: "Text style",
		Properties: map[string]Property{
			"bold": {Type: "boolean", Description: "Bold text"},
		},
		Required: []string{"bold"},
	})
	if err != nil {
		t.Fatalf("Failed to marshal property: %v", err)
	}

	got := string(encoded)
	for _, want := range []string{`"properties"`, `"bold"`, `"required":["bold"]`} {
		if !strings.Contains(got, want) {
			t.Errorf("Property JSON should contain %s, got %s", want, got)
		}
	}
}

// TestPropertyEncodesUnionSchema covers a property that accepts more than one
// shape, as the color arguments do.
func TestPropertyEncodesUnionSchema(t *testing.T) {
	encoded, err := json.Marshal(Property{
		Description: "A color",
		AnyOf: []Property{
			{Type: "string", Description: "Hex string"},
			{Type: "object", Description: "Components"},
		},
	})
	if err != nil {
		t.Fatalf("Failed to marshal property: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal property: %v", err)
	}
	if _, ok := decoded["anyOf"]; !ok {
		t.Errorf("Property JSON should contain anyOf, got %s", encoded)
	}
	// A union must not also claim a single type, which would contradict it.
	// The branches inside anyOf keep their own types.
	if _, ok := decoded["type"]; ok {
		t.Errorf("Property JSON should omit an empty type, got %s", encoded)
	}
}
