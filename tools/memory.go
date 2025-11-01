package tools

import (
	"fmt"
	"strings"
	"sync"

	"github.com/nexxia-ai/aigentic"
	"github.com/nexxia-ai/aigentic/ai"
)

func NewMemoryTool() aigentic.AgentTool {
	data := make(map[string]string)
	var mutex sync.RWMutex

	formatAll := func() string {
		mutex.RLock()
		defer mutex.RUnlock()

		if len(data) == 0 {
			return ""
		}

		var parts []string
		for name, content := range data {
			parts = append(parts, fmt.Sprintf("## Memory: %s\n%s", name, content))
		}
		return strings.Join(parts, "\n\n")
	}

	update := func(name, content string) error {
		mutex.Lock()
		defer mutex.Unlock()

		if content == "" {
			delete(data, name)
		} else {
			data[name] = content
		}
		return nil
	}

	contextFn := func(run *aigentic.AgentRun) (string, error) {
		return formatAll(), nil
	}

	return aigentic.AgentTool{
		Name:        "update_memory",
		Description: "Update or delete memory entries. Set memory_content to empty string to delete.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"memory_name": map[string]interface{}{
					"type":        "string",
					"description": "Name/identifier for this memory entry",
				},
				"memory_content": map[string]interface{}{
					"type":        "string",
					"description": "Markdown content (empty string to delete)",
				},
			},
			"required": []string{"memory_name", "memory_content"},
		},
		ContextFunctions: []aigentic.ContextFunction{contextFn},
		NewExecute: func(run *aigentic.AgentRun, result aigentic.ValidationResult) (*ai.ToolResult, error) {
			args := result.Values.(map[string]interface{})
			name := args["memory_name"].(string)
			content := args["memory_content"].(string)

			if err := update(name, content); err != nil {
				return &ai.ToolResult{
					Content: []ai.ToolContent{{Type: "text", Content: fmt.Sprintf("Error: %v", err)}},
					Error:   true,
				}, nil
			}

			msg := fmt.Sprintf("Memory '%s' updated", name)
			if content == "" {
				msg = fmt.Sprintf("Memory '%s' deleted", name)
			}

			return &ai.ToolResult{
				Content: []ai.ToolContent{{Type: "text", Content: msg}},
				Error:   false,
			}, nil
		},
	}
}
