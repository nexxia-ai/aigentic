package aigentic

import (
	"log/slog"

	"github.com/google/uuid"
	"github.com/nexxia-ai/aigentic/ai"
	"github.com/nexxia-ai/aigentic/document"
	"github.com/nexxia-ai/aigentic/memory"
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

	Memory *memory.Memory

	// Retries is the number of times to retry the agent if it fails.
	Retries int
	Stream  bool

	// Documents contains a list of documents to be embedded in the agent's context.
	// You must manage the document sizes so they don't exceed the model's context window.
	Documents []*document.Document

	// DocumentReferences contains a list of document references to be embedded in the agent's context.
	// These are used to reference documents that are not embedded in the agent's context.
	// For example, if you have a document that is too large to embed in the agent's context, you can reference it here.
	// The document will be fetched from the document store when the agent needs it.
	DocumentReferences []*document.Document

	// Trace defines the trace for the agent.
	// Set "Trace: aigentic.NewTrace()" to create trace files in the default temporary directory under $TMP/traces
	Trace *Trace

	// ContextManager defines the context manager for the agent.
	// If set, this context manager will be used instead of the default BasicContextManager.
	// Set "ContextManager: aigentic.NewEnhancedSystemContextManager(agent, message)" to use a custom context manager.
	ContextManager ContextManager

	LogLevel    slog.Level
	MaxLLMCalls int // Maximum number of LLM calls (0 = unlimited)

	// EnableEvaluation is a flag to enable evaluation events.
	// If true, the agent will generate evaluation events for each llm call and response.
	// These can be used to evaluate the agent's prompt performance using the eval package.
	EnableEvaluation bool

	Retrievers []Retriever
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
