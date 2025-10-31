package aigentic

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/nexxia-ai/aigentic/ai"
)

var approvalTimeout = time.Minute * 60

type AgentRun struct {
	id      string
	agent   Agent
	session *Session
	model   *ai.Model

	ctx        context.Context
	cancelFunc context.CancelFunc

	tools      []AgentTool
	msgHistory []ai.Message

	contextManager ContextManager
	interceptors   []Interceptor

	eventQueue       chan Event
	actionQueue      chan action
	pendingApprovals map[string]pendingApproval
	trace            *Trace
	userMessage      string
	parentRun        *AgentRun // pointer to parent if this is a sub-agent
	Logger           *slog.Logger
	maxLLMCalls      int // maximum number of LLM calls (defaults to 20 when unset)
	llmCallCount     int // number of LLM calls made
	approvalTimeout  time.Duration
}

func (r *AgentRun) ID() string {
	return r.id
}

func (r *AgentRun) Session() *Session {
	return r.session
}

func (r *AgentRun) Cancel() {
	if r.cancelFunc != nil {
		r.cancelFunc()
	}
}

func newAgentRun(a Agent, message string) *AgentRun {
	runID := uuid.New().String()
	session := a.Session
	if session == nil {
		session = NewSession(context.Background())
	}
	runCtx, cancelFunc := context.WithCancel(session.Context)
	model := a.Model
	if model == nil {
		model = ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
			return ai.AIMessage{}, fmt.Errorf("agent model is not set")
		})
	}
	trace := session.Trace
	if a.Trace != nil {
		trace = a.Trace
	}
	// Build interceptor chain: copy from agent, add trace if set
	interceptors := make([]Interceptor, len(a.Interceptors))
	copy(interceptors, a.Interceptors)
	if trace != nil {
		interceptors = append(interceptors, trace)
	}
	// Apply a conservative default to prevent runaway tool/LLM loops.
	maxLLMCalls := 20
	if a.MaxLLMCalls != 0 {
		maxLLMCalls = a.MaxLLMCalls
	}

	if a.ContextManager == nil {
		a.ContextManager = NewBasicContextManager(a, message)
	}

	run := &AgentRun{
		id:               runID,
		agent:            a,
		model:            model,
		session:          session,
		ctx:              runCtx,
		cancelFunc:       cancelFunc,
		userMessage:      message,
		trace:            trace,
		interceptors:     interceptors,
		maxLLMCalls:      maxLLMCalls,
		eventQueue:       make(chan Event, 100),
		actionQueue:      make(chan action, 100),
		pendingApprovals: make(map[string]pendingApproval),
		approvalTimeout:  approvalTimeout,
		contextManager:   a.ContextManager,
		Logger:           slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: a.LogLevel})).With("agent", a.Name),
	}
	run.tools = run.addTools()
	return run
}

func (r *AgentRun) start() {

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

func (r *AgentRun) addTools() []AgentTool {
	totalToolsCount := len(r.agent.AgentTools) + len(r.agent.Agents) + 1 // +1 for memory tool
	tools := make([]AgentTool, 0, totalToolsCount)
	tools = append(tools, r.agent.AgentTools...)

	for _, aa := range r.agent.Agents {
		// tool adapter for sub-agent
		agentTool := AgentTool{
			Name:        aa.Name,
			Description: aa.Description,
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
			Validate: func(run *AgentRun, args map[string]interface{}) (ValidationResult, error) {
				return ValidationResult{Values: args}, nil
			},
			NewExecute: func(run *AgentRun, validationResult ValidationResult) (*ai.ToolResult, error) {
				input := ""
				if v, ok := validationResult.Values.(map[string]any)["input"].(string); ok {
					input = v
				}
				aa.Session = r.session
				// Inherit Stream setting from parent if child agent doesn't have it explicitly set
				if !aa.Stream && r.agent.Stream {
					aa.Stream = true
				}
				subRun := newAgentRun(aa, input)
				subRun.trace = r.trace
				subRun.Logger = r.Logger.With("sub-agent", aa.Name)
				subRun.parentRun = r

				subRun.start()
				content, err := subRun.Wait(0)

				// Record sub-agent errors to trace
				if r.trace != nil && err != nil {
					r.trace.RecordError(fmt.Errorf("sub-agent %s error: %v", aa.Name, err))
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
		tools = append(tools, agentTool)
	}

	// Retriever tools
	for _, retriever := range r.agent.Retrievers {
		tools = append(tools, retriever.ToTool())
	}

	// Add memory tools
	if r.agent.Memory != nil {
		memoryTools := r.agent.Memory.GetTools()
		for _, tool := range memoryTools {
			tools = append(tools, WrapTool(tool))
		}
	}

	// make sure all tools have a validation and execute function
	for i := range tools {
		if tools[i].Validate == nil {
			tools[i].Validate = func(run *AgentRun, args map[string]interface{}) (ValidationResult, error) {
				return ValidationResult{Values: args, Message: ""}, nil
			}
		}
		if tools[i].NewExecute == nil {
			tools[i].NewExecute = func(run *AgentRun, validationResult ValidationResult) (*ai.ToolResult, error) {
				return nil, nil
			}
		}
	}
	return tools
}

func (r *AgentRun) findTool(tcName string) *AgentTool {
	for i := range r.tools {
		if r.tools[i].Name == tcName {
			return &r.tools[i]
		}
	}
	return nil
}

func (r *AgentRun) Wait(d time.Duration) (string, error) {
	content := ""
	var err error
	for evt := range r.eventQueue {
		switch event := evt.(type) {
		case *ContentEvent:
			// only append content that is for the same run ID so you don't append sub-agent content to the parent agent
			if r.ID() == event.RunID {
				content += event.Content
			}
		case *ErrorEvent:
			err = event.Err
		}
	}
	return content, err
}

func (r *AgentRun) Approve(approvalID string, approved bool) {
	r.queueAction(&approvalAction{ApprovalID: approvalID, Approved: approved})
}

func (r *AgentRun) Next() <-chan Event {
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
				r.runLLMCallAction(act.Message, r.tools)

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
		event := &ErrorEvent{
			RunID:     r.id,
			AgentName: r.agent.Name,
			SessionID: r.session.ID,
			Err:       act.Error,
		}
		r.queueEvent(event)
	}

	// Clear run memory at the end of each agent run
	if r.agent.Memory != nil {
		if err := r.agent.Memory.ClearRunMemory(); err != nil {
			r.Logger.Error("failed to clear run memory", "error", err)
		}
	}

	r.stop()
}

func (r *AgentRun) checkApprovalTimeouts() {
	now := time.Now()
	for approvalID, approval := range r.pendingApprovals {
		if now.After(approval.deadline) {
			r.Logger.Error("approval timed out", "approvalID", approvalID, "deadline", approval.deadline)
			delete(r.pendingApprovals, approvalID)
			r.queueAction(
				&toolResponseAction{
					request:  &toolCallAction{ToolCallID: approval.ToolCallID, ToolName: approval.Tool.Name, ValidationResult: approval.ValidationResult, Group: approval.Group},
					response: fmt.Sprintf("approval timed out for tool: %s", approval.Tool.Name),
				})
		}
	}
}

func (r *AgentRun) runApprovalAction(act *approvalAction) {
	approval, ok := r.pendingApprovals[act.ApprovalID]
	if !ok {
		r.Logger.Error("invalid approval ID", "approvalID", act.ApprovalID)
		return
	}
	delete(r.pendingApprovals, act.ApprovalID)

	if act.Approved {
		r.queueAction(&toolCallAction{ToolCallID: approval.ToolCallID, ToolName: approval.Tool.Name, ValidationResult: approval.ValidationResult, Group: approval.Group})
		return
	}
	r.queueAction(&toolResponseAction{
		request:  &toolCallAction{ToolCallID: approval.ToolCallID, ToolName: approval.Tool.Name, ValidationResult: approval.ValidationResult, Group: approval.Group},
		response: fmt.Sprintf("approval denied for tool: %s", approval.Tool.Name),
	})
}

func (r *AgentRun) runLLMCallAction(message string, agentTools []AgentTool) {

	// Check limit before making any LLM call
	if r.maxLLMCalls > 0 && r.llmCallCount >= r.maxLLMCalls {
		err := fmt.Errorf("LLM call limit exceeded: %d calls (configured limit: %d)",
			r.llmCallCount, r.maxLLMCalls)
		r.queueAction(&stopAction{Error: err})
		return
	}
	r.llmCallCount++ // Increment counter

	tools := make([]ai.Tool, len(agentTools))
	for i, agentTool := range agentTools {
		tools[i] = agentTool.toTool(r)
	}

	event := &LLMCallEvent{
		RunID:     r.id,
		AgentName: r.agent.Name,
		SessionID: r.session.ID,
		Message:   message,
		Tools:     tools,
	}
	r.queueEvent(event)

	var err error
	var msgs []ai.Message
	msgs, err = r.contextManager.BuildPrompt(r.ctx, r.msgHistory, tools)
	if err != nil {
		r.queueAction(&stopAction{Error: err})
		return
	}

	// Chain BeforeCall interceptors
	currentMsgs := msgs
	currentTools := tools
	for _, interceptor := range r.interceptors {
		currentMsgs, currentTools, err = interceptor.BeforeCall(r, currentMsgs, currentTools)
		if err != nil {
			r.queueAction(&stopAction{Error: fmt.Errorf("interceptor rejected: %w", err)})
			return
		}
	}

	if r.parentRun == nil {
		r.Logger.Debug("calling LLM", "model", r.model.ModelName, "messages", len(currentMsgs), "tools", len(currentTools))
	} else {
		r.Logger.Debug("calling sub-agent LLM", "model", r.model.ModelName, "messages", len(currentMsgs), "tools", len(currentTools))
	}

	// Capture timing for evaluation
	callStart := time.Now()

	var respMsg ai.AIMessage

	switch r.agent.Stream {
	case true:
		respMsg, err = r.model.Stream(r.ctx, currentMsgs, currentTools, func(chunk ai.AIMessage) error {
			// Handle each chunk as a non-final message
			r.handleAIMessage(chunk, true) // isChunk is true
			return nil
		})

		if r.agent.EnableEvaluation {
			evalEvent := &EvalEvent{
				RunID:     r.id,
				AgentName: r.agent.Name,
				SessionID: r.session.ID,
				Sequence:  r.llmCallCount,
				Timestamp: callStart,
				Duration:  time.Since(callStart),
				Messages:  currentMsgs,
				Tools:     currentTools,
				Response:  respMsg,
				Error:     err,
				ModelName: r.model.ModelName,
			}
			r.queueEvent(evalEvent)
		}

	default:
		respMsg, err = r.model.Call(r.ctx, currentMsgs, currentTools)

		// Emit evaluation event if enabled
		if r.agent.EnableEvaluation {
			evalEvent := &EvalEvent{
				RunID:     r.id,
				AgentName: r.agent.Name,
				SessionID: r.session.ID,
				Sequence:  r.llmCallCount,
				Timestamp: callStart,
				Duration:  time.Since(callStart),
				Messages:  currentMsgs,
				Tools:     currentTools,
				Response:  respMsg,
				Error:     err,
				ModelName: r.model.ModelName,
			}

			r.queueEvent(evalEvent)
		}
	}

	if err != nil {
		if r.trace != nil {
			r.trace.RecordError(err)
		}
		r.queueAction(&stopAction{Error: err})
		return
	}

	// Chain AfterCall interceptors
	currentResp := respMsg
	for _, interceptor := range r.interceptors {
		currentResp, err = interceptor.AfterCall(r, currentMsgs, currentResp)
		if err != nil {
			r.queueAction(&stopAction{Error: fmt.Errorf("interceptor error: %w", err)})
			return
		}
	}

	r.handleAIMessage(currentResp, false)
}

func (r *AgentRun) runToolCallAction(act *toolCallAction) {
	tool := r.findTool(act.ToolName)
	if tool == nil {
		// r.fireToolResponseAction(act, fmt.Sprintf("tool not found: %s", act.ToolName))
		r.queueAction(&toolResponseAction{
			request:  act,
			response: fmt.Sprintf("tool not found: %s", act.ToolName),
		})
		return
	}

	eventID := uuid.New().String()
	toolEvent := &ToolEvent{
		RunID:            r.id,
		EventID:          eventID,
		AgentName:        r.agent.Name,
		SessionID:        r.session.ID,
		ToolName:         act.ToolName,
		ValidationResult: act.ValidationResult,
		ToolGroup:        act.Group,
	}
	r.queueEvent(toolEvent) // send after adding to the map

	r.Logger.Debug("calling tool", "tool", act.ToolName, "args", act.ValidationResult)

	currentValidationResult := act.ValidationResult
	var err error
	for _, interceptor := range r.interceptors {
		currentValidationResult, err = interceptor.BeforeToolCall(r, act.ToolName, act.ToolCallID, currentValidationResult)
		if err != nil {
			errMsg := fmt.Sprintf("interceptor rejected tool call: %v", err)
			r.queueAction(&toolResponseAction{request: act, response: errMsg})
			return
		}
	}

	result, err := tool.call(r, currentValidationResult)
	if err != nil {
		if r.trace != nil {
			r.trace.RecordError(err)
		}
		errMsg := fmt.Sprintf("tool execution error: %v", err)
		r.queueAction(&toolResponseAction{request: act, response: errMsg})
		return
	}

	currentResult := result
	for _, interceptor := range r.interceptors {
		currentResult, err = interceptor.AfterToolCall(r, act.ToolName, act.ToolCallID, currentValidationResult, currentResult)
		if err != nil {
			errMsg := fmt.Sprintf("interceptor error after tool call: %v", err)
			if r.trace != nil {
				r.trace.RecordError(err)
			}
			r.queueAction(&toolResponseAction{request: act, response: errMsg})
			return
		}
	}

	response := formatToolResponse(currentResult)

	if currentResult != nil && currentResult.Error {
		toolErr := fmt.Errorf("tool %s reported error", act.ToolName)
		if response != "" {
			toolErr = fmt.Errorf("tool %s reported error: %s", act.ToolName, response)
		}
		if r.trace != nil {
			r.trace.RecordError(toolErr)
		}
	}

	r.queueAction(&toolResponseAction{request: act, response: response})
}

func formatToolResponse(result *ai.ToolResult) string {
	if result == nil || len(result.Content) == 0 {
		return ""
	}

	parts := make([]string, 0, len(result.Content))
	for _, item := range result.Content {
		segment := stringifyToolContent(item.Content)
		if segment == "" {
			continue
		}
		if item.Type != "" && item.Type != "text" {
			segment = fmt.Sprintf("[%s] %s", item.Type, segment)
		}
		parts = append(parts, segment)
	}

	return strings.Join(parts, "\n")
}

func stringifyToolContent(content any) string {
	switch v := content.(type) {
	case nil:
		return ""
	case string:
		return v
	case []byte:
		if utf8.Valid(v) {
			return string(v)
		}
		return fmt.Sprintf("0x%x", v)
	case fmt.Stringer:
		return v.String()
	default:
		encoded, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(encoded)
	}
}

func (r *AgentRun) runToolResponseAction(action *toolCallAction, content string) {
	toolMsg := ai.ToolMessage{
		Role:       ai.ToolRole,
		Content:    content,
		ToolCallID: action.ToolCallID,
		ToolName:   action.ToolName,
	}
	action.Group.responses[action.ToolCallID] = toolMsg

	// Check if all tool calls in this group are completed
	if len(action.Group.responses) == len(action.Group.aiMessage.ToolCalls) {

		// add all tool responses and queue their events
		for _, tc := range action.Group.aiMessage.ToolCalls {
			if response, exists := action.Group.responses[tc.ID]; exists {
				r.msgHistory = append(r.msgHistory, response)
				event := &ToolResponseEvent{
					RunID:      r.id,
					AgentName:  r.agent.Name,
					SessionID:  r.session.ID,
					ToolCallID: response.ToolCallID,
					ToolName:   response.ToolName,
					Content:    response.Content,
				}
				r.queueEvent(event)
			}
		}

		// Notify any content from the AI message
		if action.Group.aiMessage.Content != "" {
			event := &ContentEvent{
				RunID:     r.id,
				AgentName: r.agent.Name,
				SessionID: r.session.ID,
				Content:   action.Group.aiMessage.Content,
			}
			r.queueEvent(event)
		}

		r.queueAction(&llmCallAction{Message: r.userMessage})
	}
}

// handleAIMessage handles the response from the LLM, whether it's a complete message or a chunk
func (r *AgentRun) handleAIMessage(msg ai.AIMessage, isChunk bool) {

	// only fire events if not streaming or if this is a chunk in streaming.
	// do not fire event if this is the last chunk (streaming) to prevent duplicate content
	if !r.agent.Stream || isChunk {
		if msg.Think != "" {
			event := &ThinkingEvent{
				RunID:     r.id,
				AgentName: r.agent.Name,
				SessionID: r.session.ID,
				Thought:   msg.Think,
			}
			r.queueEvent(event)
		}

		if msg.Content != "" {
			event := &ContentEvent{
				RunID:     r.id,
				AgentName: r.agent.Name,
				SessionID: r.session.ID,
				Content:   msg.Content,
			}
			r.queueEvent(event)
		}
	}

	// return if this is a chunk (streaming) and there are no tool calls
	if isChunk && len(msg.ToolCalls) == 0 {
		return
	}

	// this not a chunk, which means the model Call/Stream is complete
	// add to history and fire tool calls
	if len(msg.ToolCalls) == 0 {
		r.msgHistory = append(r.msgHistory, msg)
		r.queueAction(&stopAction{Error: nil})
		return
	}

	// Optimisation for single save_memory tool call. There is no need to respond to the tool call.
	// Simply save to memory and rerun the current message.
	// if len(msg.ToolCalls) == 1 && msg.ToolCalls[0].Name == "save_memory" {
	// 	var args map[string]interface{}
	// 	if err := json.Unmarshal([]byte(msg.ToolCalls[0].Args), &args); err == nil {
	// 		r.agent.Memory.Tool.Execute(r, args)
	// 		r.queueAction(&llmCallAction{Message: r.userMessage})
	// 		return
	// 	}
	// }

	// reset history slice each time so that we only keep the last assistant msg and tool responses (if any)
	r.msgHistory = []ai.Message{msg}

	r.groupToolCalls(msg.ToolCalls, msg)
}

// groupToolCalls processes a slice of tool calls and queues the appropriate actions
func (r *AgentRun) groupToolCalls(toolCalls []ai.ToolCall, msg ai.AIMessage) {
	group := &toolCallGroup{
		aiMessage: &msg,
		responses: make(map[string]ai.ToolMessage),
	}

	for _, tc := range toolCalls {
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(tc.Args), &args); err != nil {
			if r.trace != nil {
				r.trace.RecordError(err)
			}
			r.queueAction(&toolResponseAction{request: &toolCallAction{
				ToolCallID: tc.ID,
				ToolName:   tc.Name, ValidationResult: ValidationResult{Values: args}, Group: group},
				response: fmt.Sprintf("invalid tool parameters: %v", err)})
			continue
		}

		tool := r.findTool(tc.Name)
		if tool == nil {
			r.queueAction(&toolResponseAction{
				request:  &toolCallAction{ToolName: tc.Name, ValidationResult: ValidationResult{Values: args}, Group: group},
				response: fmt.Sprintf("tool not found: %s", tc.Name),
			})
			continue
		}

		// run validation
		values, err := tool.validateInput(r, args)
		if err != nil {
			r.queueAction(&toolResponseAction{
				request:  &toolCallAction{ToolCallID: tc.ID, ToolName: tc.Name, ValidationResult: ValidationResult{Values: args}, Group: group},
				response: fmt.Sprintf("invalid tool parameters: %v", err)})
			continue
		}

		if tool.RequireApproval {
			approvalID := uuid.New().String()
			r.pendingApprovals[approvalID] = pendingApproval{
				ApprovalID:       approvalID,
				Tool:             tool,
				ToolCallID:       tc.ID,
				ValidationResult: values,
				Group:            group,
				deadline:         time.Now().Add(r.approvalTimeout),
			}
			approvalEvent := &ApprovalEvent{
				RunID:            r.id,
				ApprovalID:       approvalID,
				ToolName:         tc.Name,
				ValidationResult: values,
			}
			r.queueEvent(approvalEvent)
			return
		}

		r.queueAction(&toolCallAction{ToolCallID: tc.ID, ToolName: tc.Name, ValidationResult: values, Group: group})
	}
}

// Add queueEvent and queueAction methods to AgentRun
func (r *AgentRun) queueEvent(event Event) {
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
