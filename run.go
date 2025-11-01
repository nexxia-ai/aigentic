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

	eventQueue           chan Event
	actionQueue          chan action
	pendingApprovals     map[string]pendingApproval
	processedToolCallIDs map[string]bool // track tool calls processed from streaming chunks to avoid duplicates
	currentStreamGroup   *toolCallGroup  // group for current streaming response (shared between chunks and final message)
	trace                *TraceRun
	userMessage          string
	parentRun            *AgentRun // pointer to parent if this is a sub-agent
	Logger               *slog.Logger
	maxLLMCalls          int // maximum number of LLM calls (defaults to 20 when unset)
	llmCallCount         int // number of LLM calls made
	approvalTimeout      time.Duration
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
	// Create TraceRun from factory if tracer is set
	var traceRun *TraceRun
	if a.Tracer != nil {
		traceRun = a.Tracer.NewTraceRun()
	}
	// Build interceptor chain: copy from agent, add trace if set
	interceptors := make([]Interceptor, len(a.Interceptors))
	copy(interceptors, a.Interceptors)
	if traceRun != nil {
		interceptors = append(interceptors, traceRun)
	}
	// Always add LoggerInterceptor as the last interceptor
	interceptors = append(interceptors, newLoggerInterceptor())
	// Apply a conservative default to prevent runaway tool/LLM loops.
	maxLLMCalls := 20
	if a.MaxLLMCalls != 0 {
		maxLLMCalls = a.MaxLLMCalls
	}

	if a.ContextManager == nil {
		a.ContextManager = NewBasicContextManager(a, message)
	}

	run := &AgentRun{
		id:                   runID,
		agent:                a,
		model:                model,
		session:              session,
		ctx:                  runCtx,
		cancelFunc:           cancelFunc,
		userMessage:          message,
		trace:                traceRun,
		interceptors:         interceptors,
		maxLLMCalls:          maxLLMCalls,
		eventQueue:           make(chan Event, 100),
		actionQueue:          make(chan action, 100),
		pendingApprovals:     make(map[string]pendingApproval),
		processedToolCallIDs: make(map[string]bool),
		approvalTimeout:      approvalTimeout,
		contextManager:       a.ContextManager,
		Logger:               slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: a.LogLevel})).With("agent", a.Name),
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
	totalToolsCount := len(r.agent.AgentTools) + len(r.agent.Agents)
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

func (r *AgentRun) TraceFilepath() string {
	if r.trace == nil {
		return ""
	}
	return r.trace.Filepath()
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

	// Clear processed tool call IDs and stream group for this new LLM call
	r.processedToolCallIDs = make(map[string]bool)
	r.currentStreamGroup = nil

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
	msgs, err = r.contextManager.BuildPrompt(r, r.msgHistory, tools)
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

	// Don't check completion if we're still streaming (will be checked when final message arrives)
	if r.currentStreamGroup != nil && r.currentStreamGroup == action.Group {
		return
	}

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

	// Process tool calls from chunks immediately for better UX, but track them to avoid duplicates
	if isChunk {
		if len(msg.ToolCalls) > 0 {
			// Initialize stream group if this is the first chunk with tool calls
			if r.currentStreamGroup == nil {
				chunkMsg := ai.AIMessage{
					Role:      msg.Role,
					ToolCalls: msg.ToolCalls,
				}
				r.currentStreamGroup = &toolCallGroup{
					aiMessage: &chunkMsg,
					responses: make(map[string]ai.ToolMessage),
				}
			}
			// Process tool calls using the shared group
			r.processToolCallsFromChunk(msg.ToolCalls)
		}
		return
	}

	// this not a chunk, which means the model Call/Stream is complete
	// add to history and fire tool calls
	if len(msg.ToolCalls) == 0 {
		r.msgHistory = append(r.msgHistory, msg)
		r.queueAction(&stopAction{Error: nil})
		return
	}

	// reset history slice each time so that we only keep the last assistant msg and tool responses (if any)
	r.msgHistory = []ai.Message{msg}

	// If we have a stream group from chunks, update it with the final message, otherwise create new group
	if r.currentStreamGroup != nil {
		r.currentStreamGroup.aiMessage = &msg
		r.groupToolCalls(msg.ToolCalls, msg, r.currentStreamGroup)
		// Check if all tool calls in the group are now completed (now that we have the final message)
		if len(r.currentStreamGroup.responses) == len(r.currentStreamGroup.aiMessage.ToolCalls) {
			// add all tool responses and queue their events
			for _, tc := range r.currentStreamGroup.aiMessage.ToolCalls {
				if response, exists := r.currentStreamGroup.responses[tc.ID]; exists {
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
			if r.currentStreamGroup.aiMessage.Content != "" {
				event := &ContentEvent{
					RunID:     r.id,
					AgentName: r.agent.Name,
					SessionID: r.session.ID,
					Content:   r.currentStreamGroup.aiMessage.Content,
				}
				r.queueEvent(event)
			}

			r.queueAction(&llmCallAction{Message: r.userMessage})
		}
		r.currentStreamGroup = nil // clear after processing
	} else {
		r.groupToolCalls(msg.ToolCalls, msg, nil)
	}
}

// processToolCallsFromChunk processes tool calls from a streaming chunk using the shared stream group
func (r *AgentRun) processToolCallsFromChunk(toolCalls []ai.ToolCall) {
	for _, tc := range toolCalls {
		// Skip tool calls that were already processed
		if r.processedToolCallIDs[tc.ID] {
			continue
		}
		// Mark this tool call as processed
		r.processedToolCallIDs[tc.ID] = true

		var args map[string]interface{}
		if err := json.Unmarshal([]byte(tc.Args), &args); err != nil {
			if r.trace != nil {
				r.trace.RecordError(err)
			}
			r.queueAction(&toolResponseAction{request: &toolCallAction{
				ToolCallID:       tc.ID,
				ToolName:         tc.Name,
				ValidationResult: ValidationResult{Values: args},
				Group:            r.currentStreamGroup},
				response: fmt.Sprintf("invalid tool parameters: %v", err)})
			continue
		}

		tool := r.findTool(tc.Name)
		if tool == nil {
			r.queueAction(&toolResponseAction{
				request:  &toolCallAction{ToolName: tc.Name, ValidationResult: ValidationResult{Values: args}, Group: r.currentStreamGroup},
				response: fmt.Sprintf("tool not found: %s", tc.Name),
			})
			continue
		}

		// run validation
		values, err := tool.validateInput(r, args)
		if err != nil {
			r.queueAction(&toolResponseAction{
				request:  &toolCallAction{ToolCallID: tc.ID, ToolName: tc.Name, ValidationResult: ValidationResult{Values: args}, Group: r.currentStreamGroup},
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
				Group:            r.currentStreamGroup,
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

		r.queueAction(&toolCallAction{ToolCallID: tc.ID, ToolName: tc.Name, ValidationResult: values, Group: r.currentStreamGroup})
	}
}

// groupToolCalls processes a slice of tool calls and queues the appropriate actions
func (r *AgentRun) groupToolCalls(toolCalls []ai.ToolCall, msg ai.AIMessage, existingGroup *toolCallGroup) {
	var group *toolCallGroup
	if existingGroup != nil {
		group = existingGroup
		group.aiMessage = &msg
	} else {
		group = &toolCallGroup{
			aiMessage: &msg,
			responses: make(map[string]ai.ToolMessage),
		}
	}

	for _, tc := range toolCalls {
		// Skip tool calls that were already processed from streaming chunks
		if r.processedToolCallIDs[tc.ID] {
			continue
		}
		// Mark this tool call as processed
		r.processedToolCallIDs[tc.ID] = true

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
