package run

import (
	"github.com/nexxia-ai/aigentic/ai"
)

type ToolCallGroup struct {
	AIMessage     *ai.AIMessage
	Responses     map[string]ai.ToolMessage // LLM-facing content (includes file refs)
	UserResponses map[string]string         // User-facing content (original tool output only)
}
