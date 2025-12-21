package run

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/nexxia-ai/aigentic/ai"
	"github.com/nexxia-ai/aigentic/ctxt"
	"github.com/nexxia-ai/aigentic/document"
	"github.com/nexxia-ai/aigentic/event"
)

var approvalTimeout = time.Minute * 60

type AgentRun struct {
	id        string
	sessionID string // a unique identifier for multiple runs
	model     *ai.Model
	agentName string

	ctx        context.Context
	cancelFunc context.CancelFunc

	tools []AgentTool

	agentContext *ctxt.AgentContext
	interceptors []Interceptor

	eventQueue              chan event.Event
	actionQueue             chan action
	pendingApprovals        map[string]pendingApproval
	processedToolCallIDs    map[string]bool
	currentStreamGroup      *ToolCallGroup
	trace                   Trace
	userMessage             string
	parentRun               *AgentRun
	Logger                  *slog.Logger
	maxLLMCalls             int
	llmCallCount            int
	approvalTimeout         time.Duration
	currentConversationTurn *ctxt.ConversationTurn

	streaming bool

	retrievers []Retriever

	subAgents []AgentTool
}

func (r *AgentRun) ID() string {
	return r.id
}

func (r *AgentRun) AgentName() string {
	return r.agentName
}

func (r *AgentRun) SetStreaming(streaming bool) {
	r.streaming = streaming
}

func (r *AgentRun) Model() *ai.Model {
	return r.model
}

func (r *AgentRun) ConversationTurn() *ctxt.ConversationTurn {
	return r.currentConversationTurn
}

func (r *AgentRun) Cancel() {
	if r.cancelFunc != nil {
		r.cancelFunc()
	}
}

func (r *AgentRun) SetRetrievers(retrievers []Retriever) {
	r.retrievers = retrievers
}

func NewAgentRun(name, description, instructions string) *AgentRun {
	runID := uuid.New().String()
	sessionID := uuid.New().String()
	runCtx, cancelFunc := context.WithCancel(context.Background())

	model := ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
		return ai.AIMessage{}, fmt.Errorf("agent model is not set")
	})

	ac := ctxt.NewAgentContext(runID, description, instructions, "")
	run := &AgentRun{
		agentName:               name,
		id:                      runID,
		sessionID:               sessionID,
		ctx:                     runCtx,
		cancelFunc:              cancelFunc,
		agentContext:            ac,
		model:                   model,
		maxLLMCalls:             20,
		eventQueue:              make(chan event.Event, 100),
		actionQueue:             make(chan action, 100),
		pendingApprovals:        make(map[string]pendingApproval),
		processedToolCallIDs:    make(map[string]bool),
		approvalTimeout:         approvalTimeout,
		interceptors:            make([]Interceptor, 0),
		tools:                   make([]AgentTool, 0),
		streaming:               false,
		Logger:                  slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})).With("agent", name),
		currentConversationTurn: ctxt.NewConversationTurn("", runID, "", ""),
	}

	return run
}

func (r *AgentRun) AgentContext() *ctxt.AgentContext {
	return r.agentContext
}

func (r *AgentRun) SetModel(model *ai.Model) {
	r.model = model
}

func (r *AgentRun) SetAgentName(agentName string) {
	r.agentName = agentName
}

func (r *AgentRun) SetInterceptors(interceptors []Interceptor) {
	r.interceptors = interceptors
}

func (r *AgentRun) SetMaxLLMCalls(maxLLMCalls int) {
	r.maxLLMCalls = maxLLMCalls
}

func (r *AgentRun) SetTracer(tracer Trace) {
	r.trace = tracer
}

func (r *AgentRun) SetTools(tools []AgentTool) {
	r.tools = tools
}

func (r *AgentRun) EnableHistory() {
	r.interceptors = append(r.interceptors, newHistoryInterceptor(r.agentContext.ConversationHistory()))
}

func (r *AgentRun) SetConversationHistory(history *ctxt.ConversationHistory) {
	r.agentContext.SetConversationHistory(history)
	if history != nil {
		r.interceptors = append(r.interceptors, newHistoryInterceptor(history))
	}
}

// AddDocument adds a document to the conversation turn and optionally to the session
func (r *AgentRun) AddDocument(toolID string, doc *document.Document, scope string) error {
	if doc == nil {
		return fmt.Errorf("document cannot be nil")
	}

	if scope != "local" && scope != "model" && scope != "session" {
		return fmt.Errorf("invalid scope: %s (must be 'local', 'model', or 'session')", scope)
	}

	entry := ctxt.DocumentEntry{
		Document: doc,
		Scope:    scope,
		ToolID:   toolID,
	}

	r.currentConversationTurn.Documents = append(r.currentConversationTurn.Documents, entry)

	if scope == "model" || scope == "session" {
		r.agentContext.AddDocument(doc)
	}

	return nil
}

func (r *AgentRun) Start(context context.Context, message string) {
	r.currentConversationTurn = ctxt.NewConversationTurn(message, r.id, "", "")
	r.agentContext.SetUserMessage(message)

	// Add trace interceptor if present - this must be the last interceptor to capture the full response
	if r.trace != nil {
		r.interceptors = append(r.interceptors, r.trace)
	}

	// goroutine to read the action queue and process actions.
	// it will terminate when the action queue is closed and the agent is finished.
	go r.processLoop()
	r.queueAction(&llmCallAction{Message: r.userMessage})
}

func (r *AgentRun) stop() {
	if r.cancelFunc != nil {
		r.cancelFunc()
	}
	close(r.eventQueue)
	close(r.actionQueue)
	// Only close trace if this is not a sub-agent (sub-agents share trace with parent)
	if r.trace != nil && r.parentRun == nil {
		r.trace.Close()
	}
}

func (r *AgentRun) AddSubAgent(name, description, message string, model *ai.Model, tools []AgentTool) {
	agentTool := AgentTool{
		Name:        name,
		Description: description,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"input": map[string]interface{}{
					"type":        "string",
					"description": "The input to send to the agent",
				},
			},
			"required": []string{"input"},
		},
		Validate: func(run *AgentRun, args map[string]interface{}) (event.ValidationResult, error) {
			return event.ValidationResult{Values: args}, nil
		},
		NewExecute: func(run *AgentRun, validationResult event.ValidationResult) (*ai.ToolResult, error) {
			input := ""
			if v, ok := validationResult.Values.(map[string]any)["input"].(string); ok {
				input = v
			}
			subRun := NewAgentRun(name, description, message)
			subRun.SetModel(model)
			subRun.SetTools(tools)
			subRun.trace = r.trace
			subRun.Logger = r.Logger.With("sub-agent", name)
			subRun.parentRun = r
			// Inherit Stream setting from parent
			if r.streaming {
				subRun.SetStreaming(true)
			}

			subRun.Start(r.ctx, input)
			content, err := subRun.Wait(0)

			// Record sub-agent errors to trace
			if r.trace != nil && err != nil {
				r.trace.RecordError(fmt.Errorf("sub-agent %s error: %v", name, err))
			}

			if err != nil {
				return &ai.ToolResult{
					Content: []ai.ToolContent{{
						Type:    "text",
						Content: fmt.Sprintf("Error: %v", err),
					}},
					Error: true,
				}, nil
			}
			return &ai.ToolResult{
				Content: []ai.ToolContent{{
					Type:    "text",
					Content: content,
				}},
				Error: false,
			}, nil
		},
	}
	r.subAgents = append(r.subAgents, agentTool)
}

func (r *AgentRun) addTools() []AgentTool {
	totalToolsCount := len(r.tools) + len(r.subAgents)
	tools := make([]AgentTool, 0, totalToolsCount)
	tools = append(tools, r.tools...)
	tools = append(tools, r.subAgents...)

	// Retriever tools
	for _, retriever := range r.retrievers {
		tools = append(tools, retriever.ToTool())
	}

	// make sure all tools have a validation and execute function
	for i := range tools {
		if tools[i].Validate == nil {
			tools[i].Validate = func(run *AgentRun, args map[string]interface{}) (event.ValidationResult, error) {
				return event.ValidationResult{Values: args, Message: ""}, nil
			}
		}
		if tools[i].NewExecute == nil {
			tools[i].NewExecute = func(run *AgentRun, validationResult event.ValidationResult) (*ai.ToolResult, error) {
				return nil, nil
			}
		}
	}
	return tools
}

func (r *AgentRun) Wait(d time.Duration) (string, error) {
	content := ""
	var err error
	for evt := range r.eventQueue {
		switch event := evt.(type) {
		case *event.ContentEvent:
			// only append content that is for the same run ID so you don't append sub-agent content to the parent agent
			if r.ID() == event.RunID {
				content += event.Content
			}
		case *event.ErrorEvent:
			err = event.Err
		}
	}
	return content, err
}

func (r *AgentRun) Approve(approvalID string, approved bool) {
	r.queueAction(&approvalAction{ApprovalID: approvalID, Approved: approved})
}

func (r *AgentRun) Next() <-chan event.Event {
	return r.eventQueue
}

// keep it a variable to make it easier to test
var tickerInterval = time.Second * 30

func (r *AgentRun) processLoop() {
	ticker := time.NewTicker(tickerInterval)
	defer ticker.Stop()

	for {
		select {
		case action, ok := <-r.actionQueue:
			if !ok {
				r.runStopAction(&stopAction{Error: fmt.Errorf("action queue closed unexpectedly")})
				return
			}
			switch act := action.(type) {
			case *stopAction:
				r.runStopAction(act)
				return

			case *llmCallAction:
				r.runLLMCallAction(act.Message)

			case *toolResponseAction:
				r.runToolResponseAction(act.request, act.response)

			case *approvalAction:
				r.runApprovalAction(act)

			case *toolCallAction:
				r.runToolCallAction(act)

			default:
				panic(fmt.Sprintf("unknown action: %T", act))
			}
		case <-ticker.C:
			r.checkApprovalTimeouts()

		case <-r.ctx.Done():
			r.runStopAction(&stopAction{Error: fmt.Errorf("run context cancelled")})
			return
		}
	}
}

func (r *AgentRun) runStopAction(act *stopAction) {
	if act.Error != nil {
		r.Logger.Error("stopping agent", "error", act.Error)
		event := &event.ErrorEvent{
			RunID:     r.id,
			AgentName: r.AgentName(),
			SessionID: r.sessionID,
			Err:       act.Error,
		}
		r.queueEvent(event)
	}

	r.stop()
}

func (r *AgentRun) queueEvent(event event.Event) {
	// if this is a sub-agent, queue the event to the parent agent
	if r.parentRun != nil {
		r.parentRun.queueEvent(event)
	}
	select {
	case r.eventQueue <- event:
		// queued
	default:
		// queue full, drop or handle overflow
		r.Logger.Error("event queue is full. dropping event", "event", event)
	}
}

func (r *AgentRun) queueAction(action action) {
	select {
	case r.actionQueue <- action:
		// queued
	default:
		// queue full, drop or handle overflow
		r.Logger.Error("action queue is full. dropping action", "action", action)
	}
}
