package aigentic

import (
	"fmt"
	"time"

	"github.com/nexxia-ai/aigentic/ai"
)

type pendingApproval struct {
	ApprovalID string
	Tool       *AgentTool
	ToolCallID string
	ToolArgs   map[string]interface{}
	Group      *toolCallGroup
	deadline   time.Time
}

type toolCallGroup struct {
	aiMessage *ai.AIMessage
	responses map[string]ai.ToolMessage
}

type AgentTool struct {
	UserValidation  string
	RequireApproval bool
	Name            string                                                                   `json:"name"`
	Description     string                                                                   `json:"description"`
	InputSchema     map[string]interface{}                                                   `json:"inputSchema,omitempty"`
	Execute         func(run *AgentRun, args map[string]interface{}) (*ai.ToolResult, error) `json:"-"`
}

func (t *AgentTool) Call(run *AgentRun, args map[string]interface{}) (*ai.ToolResult, error) {
	if t.Execute == nil {
		return nil, fmt.Errorf("tool %s has no execute function", t.Name)
	}

	return t.Execute(run, args)
}

func (t *AgentTool) toTool(run *AgentRun) ai.Tool {
	return ai.Tool{
		Name:        t.Name,
		Description: t.Description,
		InputSchema: t.InputSchema,
		Execute: func(args map[string]interface{}) (*ai.ToolResult, error) {
			return t.Execute(run, args)
		},
	}
}

// WrapTool creates an AgentTool from an ai.Tool
func WrapTool(tool ai.Tool) AgentTool {
	return AgentTool{
		Name:        tool.Name,
		Description: tool.Description,
		InputSchema: tool.InputSchema,
		Execute: func(run *AgentRun, args map[string]interface{}) (*ai.ToolResult, error) {
			return tool.Execute(args)
		},
	}
}
