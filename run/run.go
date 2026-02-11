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
	processedToolCallIDs map[string]bool
	currentStreamGroup   *ToolCallGroup
	currentToolCallID    string // Set during tool execution for tools that need their own ID
	trace                Trace
	enableTrace          bool
	parentRun            *AgentRun
	Logger               *slog.Logger
	logLevel             slog.LevelVar
	maxLLMCalls          int
	llmCallCount         int
	includeHistory       bool

	streaming bool

	retrievers []Retriever

	subAgents    []AgentTool
	subAgentDefs map[string]subAgentDef

	dynamicPlanning bool

	turnMetrics turnMetrics
}

type subAgentDef struct {
	name         string
	description  string
	instructions string
	model        *ai.Model
	tools        []AgentTool
}

type turnMetrics struct {
	usage ai.Usage
}

func (tm *turnMetrics) reset() {
	tm.usage = ai.Usage{}
}

func (tm *turnMetrics) add(u ai.Usage) {
	tm.usage.PromptTokens += u.PromptTokens
	tm.usage.CompletionTokens += u.CompletionTokens
	tm.usage.TotalTokens += u.TotalTokens
	tm.usage.PromptTokensDetails.CachedTokens += u.PromptTokensDetails.CachedTokens
	tm.usage.PromptTokensDetails.AudioTokens += u.PromptTokensDetails.AudioTokens
	tm.usage.CompletionTokensDetails.ReasoningTokens += u.CompletionTokensDetails.ReasoningTokens
	tm.usage.CompletionTokensDetails.AudioTokens += u.CompletionTokensDetails.AudioTokens
	tm.usage.CompletionTokensDetails.AcceptedPredictionTokens += u.CompletionTokensDetails.AcceptedPredictionTokens
	tm.usage.CompletionTokensDetails.RejectedPredictionTokens += u.CompletionTokensDetails.RejectedPredictionTokens
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

func (r *AgentRun) CurrentToolCallID() string {
	return r.currentToolCallID
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
	run := &AgentRun{
		agentName:            name,
		id:                   runID,
		sessionID:            sessionID,
		agentContext:         ac,
		model:                model,
		maxLLMCalls:          20,
		eventQueue:           make(chan event.Event, 100),
		actionQueue:          make(chan action, 100),
		processedToolCallIDs: make(map[string]bool),
		interceptors:         make([]Interceptor, 0),
		tools:                make([]AgentTool, 0),
		subAgentDefs:         make(map[string]subAgentDef),
		trace:                &TraceRun{},
		streaming:            false,
		includeHistory:       true,
	}
	run.logLevel.Set(slog.LevelError)
	run.Logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: &run.logLevel})).With("agent", name)
	return run, nil
}

func Continue(ctx *ctxt.AgentContext, model *ai.Model, tools []AgentTool) (*AgentRun, error) {
	runID := uuid.New().String()
	sessionID := ctx.ID()

	run := &AgentRun{
		agentName:            "",
		id:                   runID,
		sessionID:            sessionID,
		agentContext:         ctx,
		model:                model,
		maxLLMCalls:          20,
		eventQueue:           make(chan event.Event, 100),
		actionQueue:          make(chan action, 100),
		processedToolCallIDs: make(map[string]bool),
		interceptors:         make([]Interceptor, 0),
		tools:                tools,
		subAgentDefs:         make(map[string]subAgentDef),
		trace:                &TraceRun{},
		streaming:            false,
		includeHistory:       true,
	}
	run.logLevel.Set(slog.LevelError)
	run.Logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: &run.logLevel}))
	run.SetEnableTrace(ctx.EnableTrace())
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
	rest, cmd, ok := parseAigenticCommand(message)
	if ok {
		// Command-only path: do not create or consume a turn (no StartTurn, no pendingRefs/history/usage side effects).
		r.ctx, r.cancelFunc = context.WithCancel(ctx)
		r.eventQueue = make(chan event.Event, 100)
		r.actionQueue = make(chan action, 100)

		go r.processLoop()
		r.handleAigenticCommand(ctx, cmd)
		return
	}
	message = rest

	turn := r.agentContext.StartTurn(message)
	r.turnMetrics.reset()

	if r.enableTrace {
		traceFile := filepath.Join(turn.Dir(), "trace.txt")
		turn.TraceFile = traceFile
		r.trace = &TraceRun{filepath: traceFile}
	}

	turn.AgentName = r.agentName

	r.ctx, r.cancelFunc = context.WithCancel(ctx)
	r.processedToolCallIDs = make(map[string]bool)
	r.llmCallCount = 0

	r.eventQueue = make(chan event.Event, 100)
	r.actionQueue = make(chan action, 100)

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

func (r *AgentRun) SetDynamicPlanning(enabled bool) {
	r.dynamicPlanning = enabled
}

func (r *AgentRun) DynamicPlanning() bool {
	return r.dynamicPlanning
}

func (r *AgentRun) SubAgentDefs() map[string]subAgentDef {
	return r.subAgentDefs
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
		Execute: func(run *AgentRun, args map[string]interface{}) (*ToolCallResult, error) {
			input := ""
			if v, ok := args["input"].(string); ok {
				input = v
			}
			subRun, err := NewAgentRun(name, description, message, r.agentContext.Workspace().RootDir)
			if err != nil {
				return nil, fmt.Errorf("failed to create sub-agent run: %w", err)
			}
			subRun.SetModel(model)
			subRun.SetTools(tools)
			subRun.trace = r.trace
			subRun.SetEnableTrace(r.enableTrace)
			subRun.Logger = r.Logger.With("sub-agent", name)
			subRun.parentRun = r
			if r.streaming {
				subRun.SetStreaming(true)
			}

			subRun.Run(r.ctx, input)
			content, err := subRun.Wait(0)

			if r.enableTrace && err != nil {
				r.trace.RecordError(fmt.Errorf("sub-agent %s error: %v", name, err))
			}

			if err != nil {
				return &ToolCallResult{
					Result: &ai.ToolResult{
						Content: []ai.ToolContent{{
							Type:    "text",
							Content: fmt.Sprintf("Error: %v", err),
						}},
						Error: true,
					},
					FileRefs: nil,
				}, nil
			}
			return &ToolCallResult{
				Result: &ai.ToolResult{
					Content: []ai.ToolContent{{
						Type:    "text",
						Content: content,
					}},
					Error: false,
				},
				FileRefs: nil,
			}, nil
		},
	}
	r.subAgents = append(r.subAgents, agentTool)

	r.subAgentDefs[name] = subAgentDef{
		name:         name,
		description:  description,
		instructions: message,
		model:        model,
		tools:        tools,
	}
}

func (r *AgentRun) Wait(d time.Duration) (string, error) {
	content := ""
	var err error
	for evt := range r.eventQueue {
		switch event := evt.(type) {
		case *event.ContentEvent:
			if r.ID() == event.RunID {
				content += event.Content
			}
		case *event.ErrorEvent:
			err = event.Err
		}
	}
	return content, err
}

func (r *AgentRun) Next() <-chan event.Event {
	return r.eventQueue
}

func (r *AgentRun) processLoop() {
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
				r.runToolResponseAction(act.request, act.response, act.fileRefs)

			case *toolCallAction:
				r.runToolCallAction(act)

			default:
				panic(fmt.Sprintf("unknown action: %T", act))
			}

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
	if r.parentRun != nil {
		r.parentRun.queueEvent(event)
	}
	select {
	case r.eventQueue <- event:
	default:
		r.Logger.Error("event queue is full. dropping event", "event", event)
	}
}

// EmitToolContent emits tool-scoped content during tool execution.
// This allows tools to stream progress updates and structured data (like cards)
// in real-time during their execution, before the final tool result is returned.
func (r *AgentRun) EmitToolContent(toolCallID, content string) {
	r.queueEvent(&event.ToolContentEvent{
		RunID:      r.id,
		AgentName:  r.agentName,
		SessionID:  r.sessionID,
		ToolCallID: toolCallID,
		Content:    content,
	})
}

// EmitToolActivity emits a live progress label for a tool execution.
// Each call replaces the previous label for this tool on the frontend.
func (r *AgentRun) EmitToolActivity(toolCallID, label string) {
	r.queueEvent(&event.ToolActivityEvent{
		RunID:      r.id,
		AgentName:  r.agentName,
		SessionID:  r.sessionID,
		ToolCallID: toolCallID,
		Label:      label,
	})
}

// EmitToolCard emits a structured card during tool execution.
// The card is rendered inline in the chat stream as a typed component.
func (r *AgentRun) EmitToolCard(toolCallID string, card map[string]any) {
	r.queueEvent(&event.ToolCardEvent{
		RunID:      r.id,
		AgentName:  r.agentName,
		SessionID:  r.sessionID,
		ToolCallID: toolCallID,
		Card:       card,
	})
}

func (r *AgentRun) queueAction(action action) {
	select {
	case r.actionQueue <- action:
	default:
		r.Logger.Error("action queue is full. dropping action", "action", action)
	}
}
