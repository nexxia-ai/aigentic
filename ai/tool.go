package ai

import (
	"encoding/json"
	"fmt"
)

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

// CallWithJSON executes the tool with JSON input
func (t *Tool) CallWithJSON(jsonInput []byte) (*ToolResult, error) {
	var args map[string]interface{}
	if err := json.Unmarshal(jsonInput, &args); err != nil {
		return &ToolResult{
			Content: []ToolContent{{
				Type:    "text",
				Content: fmt.Sprintf("Invalid JSON input: %v", err),
			}},
			Error: true,
		}, nil
	}

	return t.Call(args)
}
