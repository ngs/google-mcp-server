package server

import (
	"context"
	"encoding/json"
)

// CombinedHandler combines multiple service handlers
type CombinedHandler struct {
	tools      []Tool
	resources  []Resource
	handleFunc func(ctx context.Context, name string, args json.RawMessage) (interface{}, error)
}

// NewCombinedHandler creates a new combined handler
func NewCombinedHandler(tools []Tool, handleFunc func(ctx context.Context, name string, args json.RawMessage) (interface{}, error)) *CombinedHandler {
	return &CombinedHandler{
		tools:      tools,
		resources:  []Resource{},
		handleFunc: handleFunc,
	}
}

// GetTools returns all tools from combined services
func (h *CombinedHandler) GetTools() []Tool {
	return h.tools
}

// GetResources returns all resources from combined services
func (h *CombinedHandler) GetResources() []Resource {
	return h.resources
}

// HandleToolCall delegates to the appropriate service handler
func (h *CombinedHandler) HandleToolCall(ctx context.Context, name string, arguments json.RawMessage) (interface{}, error) {
	return h.handleFunc(ctx, name, arguments)
}

// HandleResourceCall delegates to the appropriate service handler
func (h *CombinedHandler) HandleResourceCall(ctx context.Context, uri string) (interface{}, error) {
	// Not implemented for slides
	return nil, nil
}
