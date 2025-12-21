package run

import (
	"github.com/nexxia-ai/aigentic/ai"
)

type ContextManager interface {
	BuildPrompt(run *AgentRun, messages []ai.Message, tools []ai.Tool) ([]ai.Message, error)
}
