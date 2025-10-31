package aigentic

import (
	"github.com/nexxia-ai/aigentic/ai"
)

// Interceptor allows inspection and modification of model calls
type Interceptor interface {
	// BeforeCall is invoked before the model is called
	// Returns modified messages, tools, or error to abort
	BeforeCall(run *AgentRun, messages []ai.Message, tools []ai.Tool) ([]ai.Message, []ai.Tool, error)

	// AfterCall is invoked after the model responds
	// Returns modified response or error
	AfterCall(run *AgentRun, request []ai.Message, response ai.AIMessage) (ai.AIMessage, error)

	// BeforeToolCall is invoked before a tool is executed
	// Returns modified validation result (args) or error to abort
	BeforeToolCall(run *AgentRun, toolName string, toolCallID string, validationResult ValidationResult) (ValidationResult, error)

	// AfterToolCall is invoked after a tool execution completes
	// Returns modified result or error
	AfterToolCall(run *AgentRun, toolName string, toolCallID string, validationResult ValidationResult, result *ai.ToolResult) (*ai.ToolResult, error)
}
