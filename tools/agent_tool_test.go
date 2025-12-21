package tools

import (
	"testing"

	"github.com/nexxia-ai/aigentic"
	"github.com/nexxia-ai/aigentic/run"
)

// Test that all tools implement AgentTool correctly
func TestToolsReturnAgentTool(t *testing.T) {
	tests := []struct {
		name     string
		toolFunc func() run.AgentTool
	}{
		{"ReadFileTool", NewReadFileTool},
		{"WriteFileTool", NewWriteFileTool},
		{"PythonSandboxTool", NewPythonSandboxTool},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool := tt.toolFunc()

			// Check that tool has required fields
			if tool.Name == "" {
				t.Error("tool name should not be empty")
			}

			if tool.Description == "" {
				t.Error("tool description should not be empty")
			}

			if tool.InputSchema == nil {
				t.Error("tool input schema should not be nil")
			}

			if tool.Execute == nil {
				t.Error("tool execute function should not be nil")
			}
		})
	}
}

// Test that ReadFileTool has correct schema
func TestReadFileToolSchema(t *testing.T) {
	tool := NewReadFileTool()

	if tool.Name != ReadFileToolName {
		t.Errorf("expected name %s, got %s", ReadFileToolName, tool.Name)
	}

	// Check schema has required properties
	schema := tool.InputSchema
	props, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("schema should have properties")
	}

	if props["file_name"] == nil {
		t.Error("schema should have file_name property")
	}

	if props["store_name"] == nil {
		t.Error("schema should have store_name property")
	}
}

// Test that WriteFileTool has correct schema
func TestWriteFileToolSchema(t *testing.T) {
	tool := NewWriteFileTool()

	if tool.Name != WriteFileToolName {
		t.Errorf("expected name %s, got %s", WriteFileToolName, tool.Name)
	}

	// Check schema has required properties
	schema := tool.InputSchema
	props, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("schema should have properties")
	}

	if props["file_name"] == nil {
		t.Error("schema should have file_name property")
	}

	if props["store_name"] == nil {
		t.Error("schema should have store_name property")
	}

	if props["content"] == nil {
		t.Error("schema should have content property")
	}
}

// Test that PythonSandboxTool has correct schema
func TestPythonSandboxToolSchema(t *testing.T) {
	tool := NewPythonSandboxTool()

	if tool.Name != PythonSandboxToolName {
		t.Errorf("expected name %s, got %s", PythonSandboxToolName, tool.Name)
	}

	// Check schema has required properties
	schema := tool.InputSchema
	props, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("schema should have properties")
	}

	if props["code"] == nil {
		t.Error("schema should have code property")
	}

	if props["timeout"] == nil {
		t.Error("schema should have timeout property")
	}
}

// Test that tools can be used with Agent
func TestToolsWithAgent(t *testing.T) {
	// This test just verifies the types are compatible
	var tools []run.AgentTool

	tools = append(tools, NewReadFileTool())
	tools = append(tools, NewWriteFileTool())
	tools = append(tools, NewPythonSandboxTool())

	if len(tools) != 3 {
		t.Errorf("expected 3 tools, got %d", len(tools))
	}

	// Verify we can assign to an Agent's AgentTools field
	_ = aigentic.Agent{
		Name:       "test-agent",
		AgentTools: tools,
	}
}
