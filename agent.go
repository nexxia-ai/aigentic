package aigentic

import (
	"log/slog"

	"github.com/google/uuid"
	"github.com/nexxia-ai/aigentic/ai"
	"github.com/nexxia-ai/aigentic/document"
)

// ContextFunction is a function that provides dynamic context for the agent.
// It receives the AgentRun and returns a context string to be included in every LLM call.
// If an error occurs, the error message will be included in the context.
type ContextFunction func(*AgentRun) (string, error)

// Agent is the main declarative type for an agent.
type Agent struct {
	Model      *ai.Model
	Name       string
	Agents     []Agent
	Session    *Session
	AgentTools []AgentTool

	// Description should contain a description of the agent's role and capabilities.
	// It will be added to the system prompt. If this is a sub-agent, the Description is passed to the parent agent.
	Description string

	// Instructions should contain specific instructions for the agent to carry out its task.
	// Instructions are useful to create specific bullet points for the agent.
	// For example,
	//       "Return dates in yyyy/mm/dd format"
	//       "call tool X, then tool Y, then tool Z, in this order".
	Instructions string

	// ConversationHistory enables automatic conversation history tracking across multiple Start() calls.
	// Messages are captured with metadata (trace file, run ID, timestamp) for correlation and debugging.
	// Set to a ConversationHistory object to share history across multiple agents or conversation sessions.
	ConversationHistory *ConversationHistory

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

	// Tracer defines the trace factory for the agent.
	// Set "Tracer: aigentic.NewTracer()" to create trace files for each run.
	// Each run gets its own independent trace file.
	Tracer *Tracer

	// Interceptors chain allows inspection and modification of model calls
	Interceptors []Interceptor

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

	// ContextFunctions contains functions that provide dynamic context for the agent.
	// These functions are called before each LLM call and their output is included
	// as a separate user message wrapped in <Session context> tags.
	ContextFunctions []ContextFunction
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
