package run

import (
	"time"

	"github.com/nexxia-ai/aigentic/ai"
	"github.com/nexxia-ai/aigentic/event"
)

type pendingApproval struct {
	ApprovalID       string
	Tool             *AgentTool
	ToolCallID       string
	ValidationResult event.ValidationResult
	Group            *ToolCallGroup
	deadline         time.Time
}

type ToolCallGroup struct {
	AIMessage *ai.AIMessage
	Responses map[string]ai.ToolMessage
}
