package run

import (
	"github.com/nexxia-ai/aigentic/ai"
	"github.com/nexxia-ai/aigentic/event"
)

type Interceptor interface {
	BeforeCall(run *AgentRun, messages []ai.Message, tools []ai.Tool) ([]ai.Message, []ai.Tool, error)
	AfterCall(run *AgentRun, request []ai.Message, response ai.AIMessage) (ai.AIMessage, error)
	BeforeToolCall(run *AgentRun, toolName string, toolCallID string, validationResult event.ValidationResult) (event.ValidationResult, error)
	AfterToolCall(run *AgentRun, toolName string, toolCallID string, validationResult event.ValidationResult, result *ai.ToolResult) (*ai.ToolResult, error)
}
