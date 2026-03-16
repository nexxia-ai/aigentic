package run

import (
	"github.com/nexxia-ai/aigentic/ai"
	"github.com/nexxia-ai/aigentic/ctxt"
)

type ToolCallGroup struct {
	AIMessage     *ai.AIMessage
	Responses     map[string]ai.ToolMessage   // LLM-facing content (includes file refs)
	UserResponses map[string]string          // User-facing content (original tool output only)
	FileRefs      map[string][]ctxt.FileRef  // Per-tool-call file refs (includes ephemeral, for event emission)
	Terminal      bool                       // Set when any tool in the group returns Terminal: true
}
