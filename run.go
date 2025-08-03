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

	tools      []ai.Tool
	msgHistory []ai.Message

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
	run := &AgentRun{
		id:               uuid.New().String(),
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
		logger:           slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: a.LogLevel})).With("agent", a.Name),
	}
	run.tools = run.addSystemTools()
	return run
}

func (r *AgentRun) start() {
	go r.processLoop()
	r.fireLLMCallAction(r.userMessage, r.tools)
}

func (r *AgentRun) addSystemTools() []ai.Tool {
	tools := make([]ai.Tool, 0, len(r.agent.Tools)+len(r.agent.Agents))
	tools = append(tools, r.agent.Tools...)
	for _, aa := range r.agent.Agents {
		// Create SimpleTool adapter for sub-agent
		agentTool := ai.Tool{
			Name:            aa.Name,
			Description:     aa.Description,
			RequireApproval: false,
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
			Execute: func(args map[string]interface{}) (*ai.ToolResult, error) {
				input := ""
				if v, ok := args["input"].(string); ok {
					input = v
				}
				run := newAgentRun(aa, input)
				run.session = r.session
				run.trace = r.trace
				run.logger = r.logger.With("sub-agent", aa.Name)
				run.parentRun = r
				run.start()
				content, err := run.Wait(0)
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
	return tools
}

func (r *AgentRun) findTool(tcName string) *ai.Tool {
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

func (r *AgentRun) RunAndWait() (string, error) {
	return r.Wait(0)
}

func (r *AgentRun) Approve(approvalID string, approved bool) {
	r.queueAction(&approvalAction{ApprovalID: approvalID, Approved: approved})
}

func (r *AgentRun) Next() <-chan Event {
	return r.eventQueue
}

func (r *AgentRun) stop() {
	close(r.eventQueue)
	close(r.actionQueue)
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
				r.stop() // close the channels
				return

			case *llmCallAction:
				r.runLLMCallAction(act.Message, r.tools)

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

func (r *AgentRun) checkApprovalTimeouts() {
	now := time.Now()
	for approvalID, approval := range r.pendingApprovals {
		if now.After(approval.deadline) {
			r.logger.Error("approval timed out", "approvalID", approvalID, "deadline", approval.deadline)
			delete(r.pendingApprovals, approvalID)
			r.fireToolResponseAction(&toolCallAction{ToolCallID: approval.ToolCallID, ToolName: approval.Tool.Name, ToolArgs: approval.ToolArgs, Group: approval.Group},
				fmt.Sprintf("approval timed out for tool: %s", approval.Tool.Name))
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
	r.fireToolResponseAction(
		&toolCallAction{ToolCallID: approval.ToolCallID, ToolName: approval.Tool.Name, ToolArgs: approval.ToolArgs, Group: approval.Group},
		fmt.Sprintf("approval denied for tool: %s", approval.Tool.Name))
}

func (r *AgentRun) runLLMCallAction(message string, tools []ai.Tool) {

	// Check limit before making any LLM call
	if r.maxLLMCalls > 0 && r.llmCallCount >= r.maxLLMCalls {
		err := fmt.Errorf("LLM call limit exceeded: %d calls (configured limit: %d)",
			r.llmCallCount, r.maxLLMCalls)
		r.fireErrorAction(err)
		return
	}
	r.llmCallCount++ // Increment counter

	event := &LLMCallEvent{
		RunID:     r.id,
		AgentName: r.agent.Name,
		SessionID: r.session.ID,
		Message:   message,
		Tools:     tools,
	}
	r.queueEvent(event)

	userMsgs := r.agent.createUserMsg(message)
	sysMsg := r.agent.createSystemMessage("")
	msgs := []ai.Message{
		ai.SystemMessage{Role: ai.SystemRole, Content: sysMsg},
	}
	msgs = append(msgs, userMsgs...)
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

	if r.agent.Stream {
		// Streaming mode - the final chunk is returned as respMsg
		respMsg, err = r.model.Stream(r.session.Context, msgs, tools, func(chunk ai.AIMessage) error {
			// Handle each chunk as a non-final message
			r.handleAIMessage(chunk, true)
			return nil
		})
	} else {
		// Non-streaming mode
		respMsg, err = r.model.Call(r.session.Context, msgs, tools)
	}

	if err != nil {
		if r.trace != nil {
			r.trace.RecordError(err)
		}
		r.fireErrorAction(err)
		return
	}

	// Handle the final message
	r.handleAIMessage(respMsg, false)
}

func (r *AgentRun) runToolCallAction(act *toolCallAction) {
	tool := r.findTool(act.ToolName)
	if tool == nil {
		r.fireToolResponseAction(act, fmt.Sprintf("tool not found: %s", act.ToolName))
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
	result, err := tool.Call(act.ToolArgs)
	if err != nil {
		if r.trace != nil {
			r.trace.RecordError(err)
		}
		r.fireToolResponseAction(act, fmt.Sprintf("tool execution error: %v", err))
		return
	}
	content := ""
	for _, c := range result.Content {
		if s, ok := c.Content.(string); ok {
			content += s
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

	r.fireToolResponseAction(act, content)
}

// handleAIMessage handles the response from the LLM, whether it's a complete message or a chunk
func (r *AgentRun) handleAIMessage(msg ai.AIMessage, isChunk bool) {

	// fire thinking content notification
	if msg.Think != "" {
		r.fireThinkingAction(msg.Think)
	}

	// fire content notification if chunk
	if msg.Content != "" && (isChunk || len(msg.ToolCalls) > 0) {
		r.fireContentAction(msg.Content, true)
	}

	// return if this is a chunk (streaming)
	if isChunk {
		return
	}

	// this not a chunk
	// add to history and fire tool calls

	if r.trace != nil {
		r.trace.LLMAIResponse(r.agent.Name, msg)
	}
	r.msgHistory = append(r.msgHistory, msg)

	if len(msg.ToolCalls) == 0 {
		r.fireContentAction(msg.Content, false)
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
			r.fireToolResponseAction(&toolCallAction{
				EventID: "invalid-tool", ToolName: tc.Name, ToolArgs: args, Group: group},
				fmt.Sprintf("invalid tool parameters: %v", err))
			continue
		}
		r.fireToolCallAction(tc.Name, args, tc.ID, group)
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
