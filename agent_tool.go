package aigentic

import (
	"time"

	"github.com/nexxia-ai/aigentic/ai"
)

type pendingApproval struct {
	ApprovalID       string
	Tool             *AgentTool
	ToolCallID       string
	ValidationResult ValidationResult
	Group            *toolCallGroup
	deadline         time.Time
}

type toolCallGroup struct {
	aiMessage *ai.AIMessage
	responses map[string]ai.ToolMessage
}

type ValidationResult struct {
	Values           any
	Message          string
	ValidationErrors []error
}

type AgentTool struct {
	RequireApproval bool
	Name            string                                                                   `json:"name"`
	Description     string                                                                   `json:"description"`
	InputSchema     map[string]interface{}                                                   `json:"inputSchema,omitempty"`
	Execute         func(run *AgentRun, args map[string]interface{}) (*ai.ToolResult, error) `json:"-"`
	Validate        func(run *AgentRun, args map[string]interface{}) (ValidationResult, error)
	NewExecute      func(run *AgentRun, validationResult ValidationResult) (*ai.ToolResult, error)
}

// validateInput is always called before calling the tool
// the result is used in the approaval request (if required) and in the tool call
func (t *AgentTool) validateInput(run *AgentRun, args map[string]interface{}) (ValidationResult, error) {
	if t.Validate == nil {
		return ValidationResult{Values: args}, nil
	}
	return t.Validate(run, args)
}

// call is invoked with the result of the validation step
func (t *AgentTool) call(run *AgentRun, validationResult ValidationResult) (*ai.ToolResult, error) {
	// TODO: legacy - to be deprecated at enf of Aug 2025
	if t.Execute != nil {
		args := validationResult.Values.(map[string]any)
		return t.Execute(run, args)
	}

	if t.NewExecute != nil {
		return t.NewExecute(run, validationResult)
	}
	return nil, nil
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
