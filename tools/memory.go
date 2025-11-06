package tools

import (
	"fmt"

	"github.com/nexxia-ai/aigentic"
	"github.com/nexxia-ai/aigentic/ai"
)

func NewMemoryTool() aigentic.AgentTool {
	return aigentic.AgentTool{
		Name:        "update_memory",
		Description: "This is your transient memory. Use it to store or update memory entries. Set description and content to empty strings to delete.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"memory_id": map[string]interface{}{
					"type":        "string",
					"description": "Unique identifier for this memory entry",
				},
				"memory_description": map[string]interface{}{
					"type":        "string",
					"description": "Description of this memory entry",
				},
				"memory_content": map[string]interface{}{
					"type":        "string",
					"description": "Content of this memory entry",
				},
			},
			"required": []string{"memory_id", "memory_description", "memory_content"},
		},
		NewExecute: func(run *aigentic.AgentRun, result aigentic.ValidationResult) (*ai.ToolResult, error) {
			args := result.Values.(map[string]interface{})
			id := args["memory_id"].(string)
			description := args["memory_description"].(string)
			content := args["memory_content"].(string)

			if description == "" && content == "" {
				if err := run.DeleteMemory(id); err != nil {
					return &ai.ToolResult{
						Content: []ai.ToolContent{{Type: "text", Content: fmt.Sprintf("Error deleting memory: %v", err)}},
						Error:   true,
					}, nil
				}
				return &ai.ToolResult{
					Content: []ai.ToolContent{{Type: "text", Content: fmt.Sprintf("Memory '%s' deleted", id)}},
					Error:   false,
				}, nil
			}

			if err := run.AddMemory(id, description, content, "session"); err != nil {
				return &ai.ToolResult{
					Content: []ai.ToolContent{{Type: "text", Content: fmt.Sprintf("Error updating memory: %v", err)}},
					Error:   true,
				}, nil
			}

			msg := fmt.Sprintf("Memory '%s' stored", id)
			return &ai.ToolResult{
				Content: []ai.ToolContent{{Type: "text", Content: msg}},
				Error:   false,
			}, nil
		},
	}
}
