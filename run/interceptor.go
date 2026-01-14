package run

import (
	"github.com/nexxia-ai/aigentic/ai"
)

type Interceptor interface {
	BeforeCall(run *AgentRun, messages []ai.Message, tools []ai.Tool) ([]ai.Message, []ai.Tool, error)
	AfterCall(run *AgentRun, request []ai.Message, response ai.AIMessage) (ai.AIMessage, error)
	BeforeToolCall(run *AgentRun, toolName string, toolCallID string, args map[string]any) (map[string]any, error)
	AfterToolCall(run *AgentRun, toolName string, toolCallID string, args map[string]any, result *ai.ToolResult) (*ai.ToolResult, error)
}
