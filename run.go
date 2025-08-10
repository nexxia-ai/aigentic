package aigentic

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/nexxia-ai/aigentic/ai"
)

var approvalTimeout = time.Minute * 60

type AgentRun struct {
	id      string
	agent   *Agent
	session *Session
	model   *ai.Model

	tools      []AgentTool
	msgHistory []ai.Message

	memory *Memory

	eventQueue       chan Event
	actionQueue      chan Action
	pendingApprovals map[string]pendingApproval
	trace            *Trace
	userMessage      string
	parentRun        *AgentRun // pointer toparent if this is a sub-agent
	logger           *slog.Logger
	maxLLMCalls      int // maximum number of LLM calls
	llmCallCount     int // number of LLM calls made
	approvalTimeout  time.Duration
}

func (r *AgentRun) ID() string {
	return r.id
}

func newAgentRun(a *Agent, message string) *AgentRun {
	runID := uuid.New().String()
	session := a.Session
	if session == nil {
		session = NewSession()
	}
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
	maxLLMCalls := 20
	if a.MaxLLMCalls != 0 {
		maxLLMCalls = a.MaxLLMCalls
	}

	memory := NewMemory()
	run := &AgentRun{
		id:               runID,
		agent:            a,
		model:            model,
		session:          session,
		userMessage:      message,
		trace:            trace,
		maxLLMCalls:      maxLLMCalls,
		eventQueue:       make(chan Event, 100),
		actionQueue:      make(chan Action, 100),
		pendingApprovals: make(map[string]pendingApproval),
		approvalTimeout:  approvalTimeout,
		memory:           memory,
		logger:           slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: a.LogLevel})).With("agent", a.Name),
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
	close(r.eventQueue)
	close(r.actionQueue)
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
			Execute: func(run *AgentRun, args map[string]interface{}) (*ai.ToolResult, error) {
				input := ""
				if v, ok := args["input"].(string); ok {
					input = v
				}
				subRun := newAgentRun(aa, input)
				subRun.session = r.session
				subRun.trace = r.trace
				subRun.logger = r.logger.With("sub-agent", aa.Name)
				subRun.parentRun = r
				subRun.start()
				content, err := subRun.Wait(0)
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

	// Add memory tool
	tools = append(tools, r.memory.Tool)
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

		case <-r.session.Context.Done():
			return
		}
	}
}

func (r *AgentRun) runStopAction(act *stopAction) {
	if act.Error != nil {
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
			r.logger.Error("approval timed out", "approvalID", approvalID, "deadline", approval.deadline)
			delete(r.pendingApprovals, approvalID)
			r.queueAction(
				&toolResponseAction{
					request:  &toolCallAction{ToolCallID: approval.ToolCallID, ToolName: approval.Tool.Name, ToolArgs: approval.ToolArgs, Group: approval.Group},
					response: fmt.Sprintf("approval timed out for tool: %s", approval.Tool.Name),
				})
		}
	}
}

func (r *AgentRun) runApprovalAction(act *approvalAction) {
	approval, ok := r.pendingApprovals[act.ApprovalID]
	if !ok {
		r.logger.Error("invalid approval ID", "approvalID", act.ApprovalID)
		return
	}
	delete(r.pendingApprovals, act.ApprovalID)

	if act.Approved {
		r.queueAction(&toolCallAction{ToolCallID: approval.ToolCallID, ToolName: approval.Tool.Name, ToolArgs: approval.ToolArgs, Group: approval.Group})
		return
	}
	r.queueAction(&toolResponseAction{
		request:  &toolCallAction{ToolCallID: approval.ToolCallID, ToolName: approval.Tool.Name, ToolArgs: approval.ToolArgs, Group: approval.Group},
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

	msgs := []ai.Message{
		ai.SystemMessage{Role: ai.SystemRole, Content: r.createSystemPrompt()},
	}

	userMsgs := r.createUserMsg(message)
	msgs = append(msgs, userMsgs...)

	// always send msgHistory
	// even when memory is not including history, the msgHistory only contains
	// the last assistant and tool responses if this is a tool response action.
	// the agent will only keep the last assistant and tool responses if the memory is not including history.
	msgs = append(msgs, r.msgHistory...)

	if r.trace != nil {
		r.trace.LLMCall(r.model.ModelName, r.agent.Name, msgs)
	}

	if r.parentRun == nil {
		r.logger.Debug("calling LLM", "model", r.model.ModelName, "messages", len(msgs), "tools", len(tools))
	} else {
		r.logger.Debug("calling sub-agent LLM", "model", r.model.ModelName, "messages", len(msgs), "tools", len(tools))
	}

	var respMsg ai.AIMessage
	var err error

	switch r.agent.Stream {
	case true:
		respMsg, err = r.model.Stream(r.session.Context, msgs, tools, func(chunk ai.AIMessage) error {
			// Handle each chunk as a non-final message
			r.handleAIMessage(chunk, true) // isChunk is true
			return nil
		})
	default:
		respMsg, err = r.model.Call(r.session.Context, msgs, tools)
	}

	if err != nil {
		if r.trace != nil {
			r.trace.RecordError(err)
		}
		r.queueAction(&stopAction{Error: err})
		return
	}

	r.handleAIMessage(respMsg, false)
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
		RunID:     r.id,
		EventID:   eventID,
		AgentName: r.agent.Name,
		SessionID: r.session.ID,
		ToolName:  act.ToolName,
		ToolArgs:  act.ToolArgs,
		ToolGroup: act.Group,
	}
	r.queueEvent(toolEvent) // send after adding to the map

	r.logger.Debug("calling tool", "tool", act.ToolName, "args", act.ToolArgs)
	result, err := tool.Call(r, act.ToolArgs)
	if err != nil {
		if r.trace != nil {
			r.trace.RecordError(err)
		}
		r.queueAction(&toolResponseAction{
			request:  act,
			response: fmt.Sprintf("tool execution error: %v", err),
		})
		return
	}
	content := ""
	if result != nil {
		for _, c := range result.Content {
			if s, ok := c.Content.(string); ok {
				content += s
			}
		}
	}

	if r.trace != nil {
		// Convert tool args to JSON string for tracing
		argsJSON, _ := json.Marshal(act.ToolArgs)
		r.trace.LLMToolResponse(r.agent.Name, &ai.ToolCall{
			ID:   act.ToolCallID,
			Type: "function",
			Name: act.ToolName,
			Args: string(argsJSON),
		}, content)
	}

	r.queueAction(&toolResponseAction{request: act, response: content})
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
					ToolName:   action.ToolName,
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
				IsChunk:   true,
			}
			r.queueEvent(event)
		}

		r.queueAction(&llmCallAction{Message: r.userMessage})
	}
}

// handleAIMessage handles the response from the LLM, whether it's a complete message or a chunk
func (r *AgentRun) handleAIMessage(msg ai.AIMessage, isChunk bool) {

	if msg.Think != "" {
		event := &ThinkingEvent{
			RunID:     r.id,
			AgentName: r.agent.Name,
			SessionID: r.session.ID,
			Thought:   msg.Think,
		}
		r.queueEvent(event)
	}

	// fire content notification
	if msg.Content != "" {
		chunk := isChunk

		// Note: a chunk could be included in a tool call
		//       in this case isChunk is false but we want to notify
		if len(msg.ToolCalls) > 0 {
			chunk = true
		}
		event := &ContentEvent{
			RunID:     r.id,
			AgentName: r.agent.Name,
			SessionID: r.session.ID,
			Content:   msg.Content,
			IsChunk:   chunk,
		}
		r.queueEvent(event)
	}

	// return if this is a chunk (streaming)
	if isChunk {
		return
	}

	// this not a chunk, which means the model Call/Stream is complete
	// add to history and fire tool calls

	if r.trace != nil {
		r.trace.LLMAIResponse(r.agent.Name, msg)
	}

	// reset history slice each time
	if !r.memory.IncludeHistory {
		r.msgHistory = []ai.Message{}
	}
	r.msgHistory = append(r.msgHistory, msg)

	if len(msg.ToolCalls) == 0 {
		r.queueAction(&stopAction{Error: nil})
		return
	}

	// Handle tool calls
	group := &toolCallGroup{
		aiMessage: &msg,
		responses: make(map[string]ai.ToolMessage),
	}

	for _, tc := range msg.ToolCalls {
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(tc.Args), &args); err != nil {
			if r.trace != nil {
				r.trace.RecordError(err)
			}
			r.queueAction(&toolResponseAction{request: &toolCallAction{
				ToolName: tc.Name, ToolArgs: args, Group: group},
				response: fmt.Sprintf("invalid tool parameters: %v", err)})
			continue
		}
		r.queueToolCallAction(tc.Name, args, tc.ID, group)
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
		r.logger.Error("event queue is full. dropping event", "event", event)
	}
}

func (r *AgentRun) queueAction(action Action) {
	select {
	case r.actionQueue <- action:
		// queued
	default:
		// queue full, drop or handle overflow
		r.logger.Error("action queue is full. dropping action", "action", action)
	}
}
