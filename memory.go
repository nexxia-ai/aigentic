package aigentic

import (
	"github.com/nexxia-ai/aigentic/ai"
)

type Memory struct {
	SystemPrompt   func() string
	UserPrompt     func() string
	content        string
	Tool           AgentTool
	IncludeHistory bool
}

const systemPrompt = `
The agent maintains a file called ContextMemory.md that will be automatically added to your context. 

The ContextMemory.md file serves as a persistent memory store any information relevant to completing the current task and future related tasks.
The ContextMemory.md file starts empty and is updated by you using the save_memory tool.
It is most useful for complex, multi-step tasks that require coordination between multiple tools. Or when the request explicitly asks you to save the memory.

You can use the save_memory tool to save the ContextMemory.md file at any time during your task execution. 

Reasons why you want to save the ContextMemory.md file:
- Save a multi-step plan of action for the task so you can followw and track progress
- Retain state between multiple tool invocations

For complex processing with several steps, you should use the save_memory tool to store information relevant for completing the current task or other sub tasks.
For simple tasks, you do not need to use the save_memory tool for simple requests that can be answered in one step.

`

func NewMemory() *Memory {
	return &Memory{
		SystemPrompt: func() string {
			return systemPrompt
		},
		content:        "memory is empty",
		UserPrompt:     nil,
		Tool:           NewSaveMemoryTool(),
		IncludeHistory: false,
	}
}

func (m *Memory) Content() string {
	return m.content
}

type SaveMemoryParams struct {
	Content string `json:"content"`
}

const (
	SaveMemoryToolName    = "save_memory"
	saveMemoryDescription = `
This tool saves the content to ContextMemory.md for persistent memory across multi step tasks.
The ContextMemory.md file is automatically included in future inference calls.

When to use this tool:
- Use the tool for complex multi step processing or when you have explicitly told to do so.
- Use it to record the plan of action and completion status.
- Use when you want to save important context information that should be remembered for future invocations
- Use it to save important context such as account IDs, user IDs, validation data, and business rules required for future calls

Before using the save_memory tool, consider the current state of the memory. What is the current memory state? What new information do you need to save to memory? Why? Is it already there? If not, use the save_memory tool to replace the file.

Instructions:
- Provide the full content you want to save to memory. The tool will overwrite ContextMemory.md with the new content
- Keep memory entries concise and well-organized with clear labels
- You must maintain the memory by adding or removing memory entries before saving to ContextMemory.md
- Use markdown formatting for better readability and structure
- Always maintain a progress log to track the status of the task. For example:
  agent task log:
  - step 1: perform task 1, status: completed
  - step 2: perform task 2, status: error, reason: task 2 failed
  - step 3: perform task 3, status: completed
  - step 4: perform task 4, status: todo
  - step 5: perform task 5, status: todo
  - step 6: perform task 6, status: todo
`
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

	run.memory.content = content

	return &ai.ToolResult{
		Content: []ai.ToolContent{{
			Type:    "text",
			Content: "memory saved successfully",
		}},
		Error: false,
	}, nil

}
