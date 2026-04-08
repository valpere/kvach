// Package mcp provides a client for the Model Context Protocol (MCP).
//
// MCP allows external processes (servers) to expose tools, resources, and
// prompts to the agent over a simple JSON-RPC protocol. Servers can be run
// as local subprocesses (stdio transport) or as remote HTTP services
// (Streamable HTTP or legacy SSE transport).
//
// Built-in tools and MCP tools are indistinguishable from the agent's
// perspective — MCP tools are wrapped as [tool.Tool] implementations by the
// tool adapter in this package.
package mcp

import "context"

// Transport identifies how the agent communicates with an MCP server.
type Transport string

const (
	// TransportStdio communicates via the server process's stdin/stdout.
	TransportStdio Transport = "stdio"
	// TransportSSE uses HTTP Server-Sent Events (legacy, widely supported).
	TransportSSE Transport = "sse"
	// TransportHTTP uses the Streamable HTTP transport (MCP 2025 standard).
	TransportHTTP Transport = "http"
)

// ServerConfig is the user-supplied configuration for one MCP server entry.
type ServerConfig struct {
	// Name is the logical name used to reference this server in config and CLI.
	Name string
	// Transport selects the communication mechanism.
	Transport Transport

	// Stdio fields (TransportStdio only)
	Command string
	Args    []string
	Env     map[string]string

	// HTTP/SSE fields
	URL     string
	Headers map[string]string

	// Disabled prevents the server from being started.
	Disabled bool
}

// Status describes the current connection state of an MCP server.
type Status string

const (
	StatusDisconnected Status = "disconnected"
	StatusConnecting   Status = "connecting"
	StatusConnected    Status = "connected"
	StatusFailed       Status = "failed"
	StatusNeedsAuth    Status = "needs_auth"
)

// ToolDef describes a tool exposed by an MCP server.
type ToolDef struct {
	Name        string
	Description string
	InputSchema map[string]any
}

// ToolResult is the outcome of calling an MCP tool.
type ToolResult struct {
	Content []ContentBlock
	IsError bool
}

// ContentBlock is one item in an MCP tool result.
type ContentBlock struct {
	Type string // "text" | "image" | "resource"
	Text string
	// Image and Resource fields omitted until needed.
}

// Resource is a readable data source exposed by an MCP server.
type Resource struct {
	URI         string
	Name        string
	Description string
	MimeType    string
}

// ResourceContent is the content of a read resource.
type ResourceContent struct {
	URI      string
	MimeType string
	Text     string
	// BlobData holds binary content when MimeType is not text.
	BlobData []byte
}

// PromptDef describes a prompt template exposed by an MCP server.
type PromptDef struct {
	Name        string
	Description string
	Arguments   []PromptArgument
}

// PromptArgument is a parameter of a [PromptDef].
type PromptArgument struct {
	Name        string
	Description string
	Required    bool
}

// PromptContent is the rendered content of a prompt template.
type PromptContent struct {
	Description string
	Messages    []PromptMessage
}

// PromptMessage is one message in a rendered prompt.
type PromptMessage struct {
	Role    string
	Content string
}

// Client is the interface every MCP transport must implement.
type Client interface {
	// Connect establishes the connection to the MCP server.
	Connect(ctx context.Context) error
	// Disconnect closes the connection and releases resources.
	Disconnect() error
	// Status returns the current connection state.
	Status() Status

	// ListTools returns all tools advertised by this server.
	ListTools(ctx context.Context) ([]ToolDef, error)
	// CallTool invokes name with args and returns the result.
	CallTool(ctx context.Context, name string, args map[string]any) (*ToolResult, error)

	// ListResources returns all resources available on this server.
	ListResources(ctx context.Context) ([]Resource, error)
	// ReadResource fetches the content at uri.
	ReadResource(ctx context.Context, uri string) (*ResourceContent, error)

	// ListPrompts returns all prompt templates available on this server.
	ListPrompts(ctx context.Context) ([]PromptDef, error)
	// GetPrompt renders the prompt named name with the given arguments.
	GetPrompt(ctx context.Context, name string, args map[string]any) (*PromptContent, error)
}
