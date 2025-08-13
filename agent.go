package aigentic

import (
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/nexxia-ai/aigentic/ai"
)

type Agent struct {
	Model      *ai.Model
	Name       string
	Agents     []*Agent
	Session    *Session
	AgentTools []AgentTool

	// Description should containt a description of the agent's role and capabilities.
	// It will be added to the system prompt. If this is a sub-agent, the Description is passed to the parent agent.
	Description string

	// Instructions should contain specific instructions for the agent to carry out its task.
	// Instructions are useful to create specific bullet points for the agent.
	// For example,
	//       "Return dates in yyyy/mm/dd format"
	//       "call tool X, then tool Y, then tool Z, in this order".
	Instructions string

	// Retries is the number of times to retry the agent if it fails.
	Retries             int
	DelayBetweenRetries int
	ExponentialBackoff  bool
	Stream              bool
	Documents           []*Document
	DocumentReferences  []*Document
	parentAgent         *Agent
	Trace               *Trace
	LogLevel            slog.Level
	MaxLLMCalls         int // Maximum number of LLM calls (0 = unlimited)
}

func (a *Agent) Start(message string) (*AgentRun, error) {
	if a.Name == "" {
		a.Name = "noname_" + uuid.New().String()
	}
	run := newAgentRun(a, message)
	run.start()
	return run, nil
}

func (a *Agent) Execute(message string) (string, error) {
	run, err := a.Start(message)
	if err != nil {
		return "", err
	}
	return run.Wait(0)
}

// deriveTypeFromMime derives the resource type from MIME type
func deriveTypeFromMime(mimeType string) string {
	switch {
	case strings.HasPrefix(mimeType, "image/"):
		return "image"
	case strings.HasPrefix(mimeType, "audio/"):
		return "audio"
	case strings.HasPrefix(mimeType, "video/"):
		return "video"
	case strings.HasPrefix(mimeType, "text/"):
		return "text"
	case mimeType == "application/pdf":
		return "document"
	default:
		return "document"
	}
}
