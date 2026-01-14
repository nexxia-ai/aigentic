package tools

import (
	"fmt"

	"github.com/nexxia-ai/aigentic/ai"
	"github.com/nexxia-ai/aigentic/run"
)

func NewMemoryTool() run.AgentTool {
	return run.AgentTool{
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
		Execute: func(agentRun *run.AgentRun, args map[string]interface{}) (*ai.ToolResult, error) {
			id := args["memory_id"].(string)
			description := args["memory_description"].(string)
			content := args["memory_content"].(string)

			if description == "" && content == "" {
				if err := agentRun.AgentContext().RemoveMemory(id); err != nil {
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

			agentRun.AgentContext().AddMemory(id, description, content)

			msg := fmt.Sprintf("Memory '%s' stored", id)
			return &ai.ToolResult{
				Content: []ai.ToolContent{{Type: "text", Content: msg}},
				Error:   false,
			}, nil
		},
	}
}
