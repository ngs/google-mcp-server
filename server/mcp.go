package server

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/sourcegraph/jsonrpc2"
	"go.ngs.io/google-mcp-server/config"
)

// MCPServer represents the MCP server
type MCPServer struct {
	config    *config.Config
	services  map[string]ServiceHandler
	conn      *jsonrpc2.Conn
	mu        sync.RWMutex
	tools     []Tool
	resources []Resource
}

// ServiceHandler represents a service that provides tools and resources
type ServiceHandler interface {
	GetTools() []Tool
	GetResources() []Resource
	HandleToolCall(ctx context.Context, name string, arguments json.RawMessage) (interface{}, error)
	HandleResourceCall(ctx context.Context, uri string) (interface{}, error)
}

// Tool represents an MCP tool
type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema InputSchema `json:"inputSchema"`
}

// InputSchema represents the JSON schema for tool input
type InputSchema struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties"`
	Required   []string            `json:"required,omitempty"`
}

// Property represents a property in the input schema
type Property struct {
	Type        string    `json:"type"`
	Description string    `json:"description"`
	Items       *Property `json:"items,omitempty"`
	Enum        []string  `json:"enum,omitempty"`
}

// Resource represents an MCP resource
type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// NewMCPServer creates a new MCP server
func NewMCPServer(cfg *config.Config) *MCPServer {
	return &MCPServer{
		config:    cfg,
		services:  make(map[string]ServiceHandler),
		tools:     []Tool{},
		resources: []Resource{},
	}
}

// RegisterService registers a service handler
func (s *MCPServer) RegisterService(name string, handler ServiceHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.services[name] = handler

	// Add tools from the service
	tools := handler.GetTools()
	s.tools = append(s.tools, tools...)

	// Add resources from the service
	resources := handler.GetResources()
	s.resources = append(s.resources, resources...)
}

// Start starts the MCP server
func (s *MCPServer) Start() error {
	// Create JSON-RPC connection using stdio
	handler := &Handler{server: s}

	// Create a pipe-based stream for stdio
	stream := &StdioStream{
		input:  os.Stdin,
		output: os.Stdout,
	}

	conn := jsonrpc2.NewConn(
		context.Background(),
		jsonrpc2.NewBufferedStream(stream, &jsonrpc2.VarintObjectCodec{}),
		handler,
	)

	s.conn = conn

	// Wait for connection to close
	<-conn.DisconnectNotify()
	return nil
}

// StdioStream implements io.ReadWriteCloser for stdio
type StdioStream struct {
	input  *os.File
	output *os.File
}

func (s *StdioStream) Read(p []byte) (n int, err error) {
	return s.input.Read(p)
}

func (s *StdioStream) Write(p []byte) (n int, err error) {
	return s.output.Write(p)
}

func (s *StdioStream) Close() error {
	// Don't close stdin/stdout
	return nil
}

// Handler handles JSON-RPC requests
type Handler struct {
	server *MCPServer
}

func (h *Handler) Handle(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) {
	switch req.Method {
	case "initialize":
		h.handleInitialize(ctx, conn, req)
	case "initialized":
		// Client confirms initialization
	case "tools/list":
		h.handleToolsList(ctx, conn, req)
	case "tools/call":
		h.handleToolCall(ctx, conn, req)
	case "resources/list":
		h.handleResourcesList(ctx, conn, req)
	case "resources/read":
		h.handleResourceRead(ctx, conn, req)
	case "completion/complete":
		h.handleCompletion(ctx, conn, req)
	default:
		_ = conn.ReplyWithError(ctx, req.ID, &jsonrpc2.Error{
			Code:    jsonrpc2.CodeMethodNotFound,
			Message: fmt.Sprintf("method not found: %s", req.Method),
		})
	}
}

func (h *Handler) handleInitialize(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) {
	var params struct {
		ProtocolVersion string `json:"protocolVersion"`
		Capabilities    struct {
			Roots    interface{} `json:"roots,omitempty"`
			Sampling interface{} `json:"sampling,omitempty"`
		} `json:"capabilities"`
		ClientInfo struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"clientInfo"`
	}

	if err := json.Unmarshal(*req.Params, &params); err != nil {
		_ = conn.ReplyWithError(ctx, req.ID, &jsonrpc2.Error{
			Code:    jsonrpc2.CodeInvalidParams,
			Message: "invalid parameters",
		})
		return
	}

	response := struct {
		ProtocolVersion string `json:"protocolVersion"`
		Capabilities    struct {
			Tools     interface{} `json:"tools,omitempty"`
			Resources interface{} `json:"resources,omitempty"`
			Prompts   interface{} `json:"prompts,omitempty"`
			Logging   interface{} `json:"logging,omitempty"`
		} `json:"capabilities"`
		ServerInfo struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"serverInfo"`
	}{
		ProtocolVersion: "2024-11-05",
		ServerInfo: struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		}{
			Name:    "google-mcp-server",
			Version: "0.1.0",
		},
	}

	// Set capabilities
	response.Capabilities.Tools = struct{}{}
	response.Capabilities.Resources = struct{}{}

	_ = conn.Reply(ctx, req.ID, response)
}

func (h *Handler) handleToolsList(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) {
	h.server.mu.RLock()
	tools := h.server.tools
	h.server.mu.RUnlock()

	response := struct {
		Tools []Tool `json:"tools"`
	}{
		Tools: tools,
	}

	_ = conn.Reply(ctx, req.ID, response)
}

func (h *Handler) handleToolCall(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}

	if err := json.Unmarshal(*req.Params, &params); err != nil {
		_ = conn.ReplyWithError(ctx, req.ID, &jsonrpc2.Error{
			Code:    jsonrpc2.CodeInvalidParams,
			Message: "invalid parameters",
		})
		return
	}

	// Find the appropriate service handler
	h.server.mu.RLock()
	var handler ServiceHandler
	for _, service := range h.server.services {
		tools := service.GetTools()
		for _, tool := range tools {
			if tool.Name == params.Name {
				handler = service
				break
			}
		}
		if handler != nil {
			break
		}
	}
	h.server.mu.RUnlock()

	if handler == nil {
		_ = conn.ReplyWithError(ctx, req.ID, &jsonrpc2.Error{
			Code:    jsonrpc2.CodeMethodNotFound,
			Message: fmt.Sprintf("tool not found: %s", params.Name),
		})
		return
	}

	// Call the tool
	result, err := handler.HandleToolCall(ctx, params.Name, params.Arguments)
	if err != nil {
		_ = conn.ReplyWithError(ctx, req.ID, &jsonrpc2.Error{
			Code:    jsonrpc2.CodeInternalError,
			Message: err.Error(),
		})
		return
	}

	response := struct {
		Content []struct {
			Type string      `json:"type"`
			Text string      `json:"text,omitempty"`
			Data interface{} `json:"data,omitempty"`
		} `json:"content"`
		IsError bool `json:"isError,omitempty"`
	}{
		Content: []struct {
			Type string      `json:"type"`
			Text string      `json:"text,omitempty"`
			Data interface{} `json:"data,omitempty"`
		}{
			{
				Type: "text",
				Text: fmt.Sprintf("%v", result),
			},
		},
		IsError: false,
	}

	_ = conn.Reply(ctx, req.ID, response)
}

func (h *Handler) handleResourcesList(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) {
	h.server.mu.RLock()
	resources := h.server.resources
	h.server.mu.RUnlock()

	response := struct {
		Resources []Resource `json:"resources"`
	}{
		Resources: resources,
	}

	_ = conn.Reply(ctx, req.ID, response)
}

func (h *Handler) handleResourceRead(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) {
	var params struct {
		URI string `json:"uri"`
	}

	if err := json.Unmarshal(*req.Params, &params); err != nil {
		_ = conn.ReplyWithError(ctx, req.ID, &jsonrpc2.Error{
			Code:    jsonrpc2.CodeInvalidParams,
			Message: "invalid parameters",
		})
		return
	}

	// Find the appropriate service handler
	h.server.mu.RLock()
	var handler ServiceHandler
	for _, service := range h.server.services {
		resources := service.GetResources()
		for _, resource := range resources {
			if resource.URI == params.URI {
				handler = service
				break
			}
		}
		if handler != nil {
			break
		}
	}
	h.server.mu.RUnlock()

	if handler == nil {
		_ = conn.ReplyWithError(ctx, req.ID, &jsonrpc2.Error{
			Code:    jsonrpc2.CodeMethodNotFound,
			Message: fmt.Sprintf("resource not found: %s", params.URI),
		})
		return
	}

	// Read the resource
	result, err := handler.HandleResourceCall(ctx, params.URI)
	if err != nil {
		_ = conn.ReplyWithError(ctx, req.ID, &jsonrpc2.Error{
			Code:    jsonrpc2.CodeInternalError,
			Message: err.Error(),
		})
		return
	}

	response := struct {
		Contents []struct {
			URI      string `json:"uri"`
			MimeType string `json:"mimeType,omitempty"`
			Text     string `json:"text,omitempty"`
		} `json:"contents"`
	}{
		Contents: []struct {
			URI      string `json:"uri"`
			MimeType string `json:"mimeType,omitempty"`
			Text     string `json:"text,omitempty"`
		}{
			{
				URI:      params.URI,
				MimeType: "text/plain",
				Text:     fmt.Sprintf("%v", result),
			},
		},
	}

	_ = conn.Reply(ctx, req.ID, response)
}

func (h *Handler) handleCompletion(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) {
	var params struct {
		Ref struct {
			Type string `json:"type"`
			Name string `json:"name,omitempty"`
			URI  string `json:"uri,omitempty"`
		} `json:"ref"`
		Argument struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"argument"`
	}

	if err := json.Unmarshal(*req.Params, &params); err != nil {
		_ = conn.ReplyWithError(ctx, req.ID, &jsonrpc2.Error{
			Code:    jsonrpc2.CodeInvalidParams,
			Message: "invalid parameters",
		})
		return
	}

	// For now, return empty completions
	response := struct {
		Completion struct {
			Values []string `json:"values"`
		} `json:"completion"`
	}{
		Completion: struct {
			Values []string `json:"values"`
		}{
			Values: []string{},
		},
	}

	_ = conn.Reply(ctx, req.ID, response)
}
