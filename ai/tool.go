package ai

import (
	"fmt"
)

// Tool mimics a "standard" mcp tool definition so you can easily use it with any mcp client
type Tool struct {
	Name            string                                                 `json:"name"`
	Description     string                                                 `json:"description"`
	InputSchema     map[string]interface{}                                 `json:"inputSchema,omitempty"`
	Execute         func(args map[string]interface{}) (*ToolResult, error) `json:"-"`
	RequireApproval bool
}

// Call executes the tool with the given arguments
func (t *Tool) Call(args map[string]interface{}) (*ToolResult, error) {
	if t.Execute == nil {
		return nil, fmt.Errorf("tool %s has no execute function", t.Name)
	}

	return t.Execute(args)
}
