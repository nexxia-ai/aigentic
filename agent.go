package aigentic

import (
	"log/slog"

	"github.com/google/uuid"
	"github.com/nexxia-ai/aigentic/ai"
)

// Agent is the main declarative type for an agent.
type Agent struct {
	Model      *ai.Model
	Name       string
	Agents     []Agent
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

	// IncludeHistory is a flag to include the message history in the prompt.
	IncludeHistory bool

	Memory *Memory

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

// Start starts a new agent run.
// The agent is passed by value and is not modified during the run.
func (a Agent) Start(message string) (*AgentRun, error) {
	if a.Name == "" {
		a.Name = "noname_" + uuid.New().String()
	}
	run := newAgentRun(a, message)
	run.start()
	return run, nil
}

// Execute is a convenience method that starts a new agent run and waits for the result.
// The agent is passed by value and is not modified during the run.
func (a Agent) Execute(message string) (string, error) {
	run, err := a.Start(message)
	if err != nil {
		return "", err
	}
	return run.Wait(0)
}
