package aigentic

import (
	"fmt"
	"os"

	"github.com/nexxia-ai/aigentic/ai"
)

type Memory struct {
	SystemPrompt   func() string
	UserPrompt     func() string
	Content        func() string
	Tool           AgentTool
	IncludeHistory bool
	filepath       string
}

const systemPrompt = `
The current working directory contains a file called ContextMemory.md that will be automatically added to your context. 
This file serves as a persistent memory store for any information relevant to completing the current task and future related tasks.
The ContextMemory.md file starts empty and is updated by you using the save_memory tool.

The ContextMemory.md file can store any information that might be useful across multiple tool invocations or when you need to complete several related tasks to fulfill user instructions. 
This includes but is not limited to:
- Storing essential data such as account IDs, user IDs, customer references, and validation rules
- Recording step-by-step planning and process workflows for complex tasks
- Maintaining business logic, decision criteria, and operational parameters
- Preserving contextual information about ongoing processes, user preferences, and system states
- Keeping track of intermediate results, partial solutions, or state information between tool calls
- Storing any relevant information that might be needed in subsequent interactions

You can use the save_memory tool to save this file at any time during your task execution. 
The save_memory tool is particularly useful when:
- You need to maintain state between multiple tool invocations
- You're working on a complex task that requires several steps
- You want to preserve information that might be needed in future requests
- You've gathered important context or data that should be remembered

When you encounter any information that might be relevant for completing the current task or future related tasks, 
consider using the save_memory tool to store it in ContextMemory.md so it's available in subsequent interactions.
`

func NewFileMemory(path string) *Memory {
	return &Memory{
		SystemPrompt: func() string {
			return systemPrompt
		},
		UserPrompt: nil,
		Content: func() string {
			s, err := os.ReadFile(path)
			if err == nil {
				return string(s)
			}
			return "memory is empty"
		},
		Tool:           NewSaveMemoryTool(),
		IncludeHistory: false,
		filepath:       path,
	}
}

type SaveMemoryParams struct {
	Content string `json:"content"`
}

const (
	SaveMemoryToolName    = "save_memory"
	saveMemoryDescription = `Memory saving tool that stores important business context and information to ContextMemory.md for persistent memory across inference calls.

WHEN TO USE THIS TOOL:
- Use when you want to save important context information that should be remembered for future invocation
- Helpful for storing account IDs, user IDs, validation data, and business rules required for future calls
- Perfect for maintaining step-by-step instructions created by the LLM for complex processes
- Essential for preserving workflow state and process knowledge across different llm invocations

HOW TO USE:
- Provide the content you want to save to memory
- The tool will overwrite the content to ContextMemory.md with the new content
- This file will be automatically included in future LLM requests for context

LIMITATIONS:
- Content is stored as plain text in markdown format
- Cannot store binary data 
- Memory is transient and will be deletedafter a full agent run
- Each call to save_memory overwrites the content of ContextMemory.md

TIPS:
- Record step-by-step instructions created by the LLM for complex workflows
- Save important context about ongoing business processes as required to complete the task
- Keep memory entries concise and well-organized with clear labels
- Add or remove memory entries before writing to the file
- Use markdown formatting for better readability and structure`
)

func NewSaveMemoryTool() AgentTool {
	return AgentTool{
		Name:        SaveMemoryToolName,
		Description: saveMemoryDescription,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"content": map[string]interface{}{
					"type":        "string",
					"description": "The content to save to memory (will overwrite ContextMemory.md)",
				},
			},
			"required": []string{"content"},
		},
		Execute: saveMemoryExecute,
	}
}

func saveMemoryExecute(run *AgentRun, args map[string]interface{}) (*ai.ToolResult, error) {
	// Extract content directly from args
	rawContent, ok := args["content"]
	if !ok {
		return &ai.ToolResult{
			Content: []ai.ToolContent{{
				Type:    "text",
				Content: "content is required",
			}},
			Error: true,
		}, nil
	}
	content, ok := rawContent.(string)
	if !ok || content == "" {
		return &ai.ToolResult{
			Content: []ai.ToolContent{{
				Type:    "text",
				Content: "content is required",
			}},
			Error: true,
		}, nil
	}

	// timestamp := time.Now().Format("2006-01-02 15:04:05")
	// memoryEntry := fmt.Sprintf("\n\n## Memory Entry - %s\n\n%s\n", timestamp, content)
	memoryEntry := content

	// file, err := os.OpenFile(run.memory.filepath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	file, err := os.OpenFile(run.memory.filepath, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return &ai.ToolResult{
			Content: []ai.ToolContent{{
				Type:    "text",
				Content: fmt.Sprintf("Error opening memory file: %v", err),
			}},
			Error: true,
		}, nil
	}
	defer file.Close()

	if _, err := file.WriteString(memoryEntry); err != nil {
		return &ai.ToolResult{
			Content: []ai.ToolContent{{
				Type:    "text",
				Content: fmt.Sprintf("Error writing memory entry: %v", err),
			}},
			Error: true,
		}, nil
	}

	return &ai.ToolResult{
		Content: []ai.ToolContent{{
			Type:    "text",
			Content: "memory saved successfully",
		}},
		Error: false,
	}, nil

}
