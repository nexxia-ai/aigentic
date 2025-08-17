package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

var (
	ErrCallingTool = errors.New("error calling tool")
)

type MCPConfig struct {
	MCPServers map[string]ServerConfig `json:"mcpServers"`
}

type ServerConfig struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env,omitempty"`
}

// ToolContent represents a single piece of content returned by a tool
type ToolContent struct {
	Type    string // "text", "image", "audio", etc.
	Content any    // The actual content (string, []byte, etc.)
}

// ToolResult represents the complete result from a tool invocation
type ToolResult struct {
	Content []ToolContent
	Error   bool
}

type MCPClient struct {
	Name   string
	client mcpclient.MCPClient
	Tools  []Tool
}

type MCPHost struct {
	Clients map[string]MCPClient
}

func createMCPClients(config *MCPConfig) (map[string]MCPClient, error) {
	clients := make(map[string]MCPClient)

	for name, server := range config.MCPServers {
		var env []string
		for k, v := range server.Env {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
		var client *mcpclient.Client
		var err error

		if server.Command == "sse_server" {
			if len(server.Args) == 0 {
				return nil, fmt.Errorf(
					"no arguments provided for sse command",
				)
			}

			client, err = mcpclient.NewSSEMCPClient(server.Args[0])
			if err == nil {
				err = client.Start(context.Background())
			}
		} else {
			client, err = mcpclient.NewStdioMCPClient(server.Command, env, server.Args...)
		}
		if err != nil {
			slog.Error("failed to create MCP client - skipping mcp server", "name", name, "error", err)
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		slog.Info("Initializing server...", "name", name)
		initRequest := mcp.InitializeRequest{}
		initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
		initRequest.Params.ClientInfo = mcp.Implementation{Name: "mcphost", Version: "0.1.0"}
		initRequest.Params.Capabilities = mcp.ClientCapabilities{}

		_, err = client.Initialize(ctx, initRequest)
		if err != nil {
			client.Close()
			slog.Error("failed to initialize MCP client - skipping mcp server", "name", name, "error", err)
			continue
		}

		mcpClient := MCPClient{Name: name, client: client}
		mcpClient.Tools, err = mcpClient.fetchTools()
		if err != nil {
			slog.Error("failed to fetch tools", "name", name, "error", err)
			continue
		}
		clients[name] = mcpClient
	}

	return clients, nil
}

func (h *MCPClient) fetchTools() ([]Tool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	toolsResult, err := h.client.ListTools(ctx, mcp.ListToolsRequest{})
	cancel()

	if err != nil {
		slog.Error("Error fetching tools", "server", h.Name, "error", err)
		return nil, err
	}

	var tools []Tool
	for _, tool := range toolsResult.Tools {
		simpleTool := Tool{Name: tool.Name, Description: tool.Description}
		if len(tool.InputSchema.Properties) > 0 {
			// Convert mcp.ToolInputSchema to map[string]interface{}
			simpleTool.InputSchema = map[string]interface{}{
				"type":       "object",
				"properties": tool.InputSchema.Properties,
				"required":   tool.InputSchema.Required,
			}
		}

		// Set up the execute function to call the MCP tool
		simpleTool.Execute = func(args map[string]interface{}) (*ToolResult, error) {
			request := mcp.CallToolRequest{
				Request: mcp.Request{
					Method: "tools/call",
				},
			}
			request.Params.Name = tool.Name
			if len(tool.InputSchema.Properties) > 0 {
				request.Params.Arguments = args
			}

			slog.Debug("tool call", "tool", tool.Name, "args", args)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			result, err := h.client.CallTool(ctx, request)
			if err != nil {
				slog.Error("error calling tool", "tool", tool.Name, "error", err)
				err = errors.Join(ErrCallingTool, err)
				return nil, err
			}

			toolResult := &ToolResult{}
			if result.IsError {
				msg := "failed to call tool"
				if c, ok := result.Content[0].(mcp.TextContent); ok {
					msg = c.Text
				}
				slog.Error("error calling tool", "tool", tool.Name, "error", msg)
				err = errors.Join(ErrCallingTool, errors.New(msg))
				return nil, err
			}

			for _, content := range result.Content {
				switch c := content.(type) {
				case mcp.TextContent:
					toolResult.Content = append(toolResult.Content, ToolContent{
						Type:    "text",
						Content: string(c.Text),
					})
					slog.Info("tool call text result", "tool", tool.Name, "result", string(c.Text))
				case mcp.ImageContent:
					toolResult.Content = append(toolResult.Content, ToolContent{
						Type:    "image",
						Content: c.Data,
					})
					slog.Info("tool call image result", "tool", tool.Name, "result_len", len(c.Data))
				case mcp.EmbeddedResource:
					s, ok := c.Resource.(mcp.TextResourceContents)
					if !ok {
						slog.Error("tool call embedded result", "tool", tool.Name, "result", fmt.Sprintf("%+v", c.Resource))
						continue
					}
					toolResult.Content = append(toolResult.Content, ToolContent{
						Type:    "resource",
						Content: s.MIMEType + ":" + s.URI + ":" + s.Text,
					})
					slog.Info("tool call embedded result", "tool", tool.Name, "result", s.MIMEType+":"+s.URI+":"+s.Text)
				default:
					slog.Error("tool call unsupported content type", "tool", tool.Name, "type", fmt.Sprintf("%T", content))
				}
			}

			return toolResult, nil
		}

		tools = append(tools, simpleTool)
	}
	return tools, nil
}

func LoadMCPConfig(filename string) (*MCPConfig, error) {
	if filename == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("error getting home directory: %w", err)
		}
		filename = filepath.Join(homeDir, ".mcp.json")
	}

	// Check if config file exists
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		// Create default config
		defaultConfig := MCPConfig{
			MCPServers: make(map[string]ServerConfig),
		}

		// Create the file with default config
		configData, err := json.MarshalIndent(defaultConfig, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("error creating default config: %w", err)
		}

		if err := os.WriteFile(filename, configData, 0644); err != nil {
			return nil, fmt.Errorf("error writing default config file: %w", err)
		}

		slog.Info("Created default config file", "path", filename)
		return &defaultConfig, nil
	}

	// Read existing config
	configData, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("error reading config file %s: %w", filename, err)
	}

	var config MCPConfig
	if err := json.Unmarshal(configData, &config); err != nil {
		return nil, fmt.Errorf("error parsing config file: %w", err)
	}

	return &config, nil
}

func NewMCPHost(config *MCPConfig) (*MCPHost, error) {
	var err error

	h := &MCPHost{}
	h.Clients, err = createMCPClients(config)
	if err != nil {
		return nil, fmt.Errorf("error creating MCP clients: %v", err)
	}

	for name := range h.Clients {
		slog.Info("Server connected", "name", name)
	}

	return h, nil
}

func (h *MCPHost) Close() {
	slog.Info("Shutting down MCP servers...")
	for name, client := range h.Clients {
		if err := client.client.Close(); err != nil {
			slog.Error("Failed to close server", "name", name, "error", err)
		}
	}
}
