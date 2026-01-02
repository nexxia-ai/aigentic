package aigentic

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/nexxia-ai/aigentic/ai"
	"github.com/nexxia-ai/aigentic/document"
	"github.com/nexxia-ai/aigentic/run"
	"github.com/nexxia-ai/aigentic/trace"
)

// ContextFunction is a function that provides dynamic context for the agent.
// It receives the AgentRun and returns a context string to be included in every LLM call.
// If an error occurs, the error message will be included in the context.
type ContextFunction func(*run.AgentRun) (string, error)

// Agent is the main declarative type for an agent.
type Agent struct {
	Model      *ai.Model
	Name       string
	Agents     []Agent
	AgentTools []run.AgentTool

	// Description should contain a description of the agent's role and capabilities.
	// It will be added to the system prompt. If this is a sub-agent, the Description is passed to the parent agent.
	Description string

	// Instructions should contain specific instructions for the agent to carry out its task.
	// Instructions are useful to create specific bullet points for the agent.
	// For example,
	//       "Return dates in yyyy/mm/dd format"
	//       "call tool X, then tool Y, then tool Z, in this order".
	Instructions string

	// OutputInstructions contains full text instructions for how the LLM should format its output.
	// These instructions are passed directly to the LLM in the system prompt.
	// Examples:
	//   "Format your response as valid JSON only."
	//   "Use Markdown formatting with headings, lists, and tables."
	//   "Respond in HTML format with proper semantic tags."
	OutputInstructions string

	// IncludeHistory enables automatic conversation history tracking across multiple Start() calls.
	// Messages are captured with metadata (trace file, run ID, timestamp) for correlation and debugging.
	IncludeHistory bool

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

	EnableTrace bool

	// Interceptors chain allows inspection and modification of model calls
	Interceptors []run.Interceptor

	LogLevel    slog.Level
	MaxLLMCalls int // Maximum number of LLM calls (0 = unlimited)

	// EnableEvaluation is a flag to enable evaluation events.
	// If true, the agent will generate evaluation events for each llm call and response.
	// These can be used to evaluate the agent's prompt performance using the eval package.
	EnableEvaluation bool

	Retrievers []run.Retriever

	// BaseDir is the base directory for the agent execution environment.
	// If not set, the agent will use the default temporary directory.
	BaseDir string
}

// Start starts a new agent run.
// The agent is passed by value and is not modified during the run.
func (a Agent) Start(message string) (*run.AgentRun, error) {
	ar, err := a.New()
	if err != nil {
		return nil, err
	}
	ar.Run(context.Background(), message)
	return ar, nil
}

func (a Agent) New() (*run.AgentRun, error) {
	if a.Name == "" {
		a.Name = "noname_" + uuid.New().String()
	}
	ar, err := run.NewAgentRun(a.Name, a.Description, a.Instructions, a.BaseDir)
	if err != nil {
		return nil, err
	}
	ar.SetModel(a.Model)
	ar.SetInterceptors(a.Interceptors)
	ar.SetMaxLLMCalls(a.MaxLLMCalls)

	if a.EnableTrace {
		dir := filepath.Join(ar.AgentContext().ExecutionEnvironment().RootDir, "traces")
		os.MkdirAll(dir, 0755)
		ar.SetTracer(trace.NewTracer(trace.TraceConfig{Directory: dir}))
	}
	ar.SetTools(a.AgentTools)
	ar.SetRetrievers(a.Retrievers)
	ar.SetStreaming(a.Stream)
	ar.SetOutputInstructions(a.OutputInstructions)
	ar.SetLogLevel(a.LogLevel)
	for _, agent := range a.Agents {
		ar.AddSubAgent(agent.Name, agent.Description, agent.Instructions, agent.Model, agent.AgentTools)
	}

	ar.AgentContext().SetDocuments(a.Documents)
	ar.AgentContext().SetDocumentReferences(a.DocumentReferences)
	ar.IncludeHistory(a.IncludeHistory)
	return ar, nil
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
