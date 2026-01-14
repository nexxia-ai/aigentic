package run

import (
	"github.com/nexxia-ai/aigentic/ai"
)

type ToolCallGroup struct {
	AIMessage *ai.AIMessage
	Responses map[string]ai.ToolMessage
}
