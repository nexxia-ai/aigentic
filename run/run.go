package run

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/nexxia-ai/aigentic/ai"
	"github.com/nexxia-ai/aigentic/ctxt"
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

	eventQueue           chan event.Event
	actionQueue          chan action
	pendingApprovals     map[string]pendingApproval
	processedToolCallIDs map[string]bool
	currentStreamGroup   *ToolCallGroup
	trace                Trace
	enableTrace          bool
	parentRun            *AgentRun
	Logger               *slog.Logger
	logLevel             slog.LevelVar
	maxLLMCalls          int
	llmCallCount         int
	approvalTimeout      time.Duration
	includeHistory       bool

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

func (r *AgentRun) Turn() *ctxt.Turn {
	return r.agentContext.Turn()
}

func (r *AgentRun) Cancel() {
	if r.cancelFunc != nil {
		r.cancelFunc()
	}
}

func (r *AgentRun) SetRetrievers(retrievers []Retriever) {
	r.retrievers = retrievers
}

func (r *AgentRun) SetOutputInstructions(instructions string) {
	r.agentContext.SetOutputInstructions(instructions)
}

func NewAgentRun(name, description, instructions, baseDir string) (*AgentRun, error) {
	runID := uuid.New().String()
	sessionID := uuid.New().String()

	model := ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
		return ai.AIMessage{}, fmt.Errorf("agent model is not set")
	})

	ac, err := ctxt.New(runID, description, instructions, baseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create agent context: %w", err)
	}
	// ac.StartTurn("")
	run := &AgentRun{
		agentName:            name,
		id:                   runID,
		sessionID:            sessionID,
		agentContext:         ac,
		model:                model,
		maxLLMCalls:          20,
		eventQueue:           make(chan event.Event, 100),
		actionQueue:          make(chan action, 100),
		pendingApprovals:     make(map[string]pendingApproval),
		processedToolCallIDs: make(map[string]bool),
		approvalTimeout:      approvalTimeout,
		interceptors:         make([]Interceptor, 0),
		tools:                make([]AgentTool, 0),
		trace:                &TraceRun{},
		streaming:            false,
		includeHistory:       true,
	}
	run.logLevel.Set(slog.LevelError)
	run.Logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: &run.logLevel})).With("agent", name)
	return run, nil
}

func (r *AgentRun) AgentContext() *ctxt.AgentContext {
	return r.agentContext
}

func (r *AgentRun) SetModel(model *ai.Model) {
	r.model = model
}

func (r *AgentRun) SetLogLevel(level slog.Level) {
	r.logLevel.Set(level)
}

func (r *AgentRun) SetAgentName(agentName string) {
	r.agentName = agentName
}

func (r *AgentRun) SetEnableTrace(enable bool) {
	r.enableTrace = enable
}

func (r *AgentRun) SetInterceptors(interceptors []Interceptor) {
	r.interceptors = interceptors
}

func (r *AgentRun) SetMaxLLMCalls(maxLLMCalls int) {
	r.maxLLMCalls = maxLLMCalls
}

func (r *AgentRun) SetTools(tools []AgentTool) {
	r.tools = tools
}

func (r *AgentRun) IncludeHistory(enable bool) {
	r.includeHistory = enable
}

func (r *AgentRun) Run(ctx context.Context, message string) {

	turn := r.agentContext.StartTurn(message)

	if r.enableTrace {
		traceFile := filepath.Join(turn.Dir(), "trace.txt")
		turn.TraceFile = traceFile
		r.trace = &TraceRun{filepath: traceFile}
	}

	turn.AgentName = r.agentName

	r.ctx, r.cancelFunc = context.WithCancel(ctx)
	r.pendingApprovals = make(map[string]pendingApproval)
	r.processedToolCallIDs = make(map[string]bool)
	r.llmCallCount = 0

	// new channels for the run
	r.eventQueue = make(chan event.Event, 100)
	r.actionQueue = make(chan action, 100)

	// goroutine to read the action queue and process actions.
	// it will terminate when the action queue is closed and the agent is finished.
	go r.processLoop()
	r.queueAction(&llmCallAction{Message: r.agentContext.Turn().UserMessage})
}

func (r *AgentRun) stop() {
	if r.cancelFunc != nil {
		r.cancelFunc()
	}
	close(r.eventQueue)
	close(r.actionQueue)
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
			subRun, err := NewAgentRun(name, description, message, r.agentContext.ExecutionEnvironment().RootDir)
			if err != nil {
				return nil, fmt.Errorf("failed to create sub-agent run: %w", err)
			}
			subRun.SetModel(model)
			subRun.SetTools(tools)
			subRun.trace = r.trace
			subRun.Logger = r.Logger.With("sub-agent", name)
			subRun.parentRun = r
			// Inherit Stream setting from parent
			if r.streaming {
				subRun.SetStreaming(true)
			}

			subRun.Run(r.ctx, input)
			content, err := subRun.Wait(0)

			// Record sub-agent errors to trace
			if r.enableTrace && err != nil {
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
