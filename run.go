package aigentic

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/nexxia-ai/aigentic/ai"
)

type AgentRun struct {
	agent   *Agent
	session *Session
	model   *ai.Model

	tools      []ai.Tool
	msgHistory []ai.Message

	eventQueue       chan Event
	actionQueue      chan Action
	pendingApprovals map[string]*pendingApproval
	trace            *Trace
	userMessage      string
	parentRun        *AgentRun // pointer toparent if this is a sub-agent
	logger           *slog.Logger
	maxLLMCalls      int // maximum number of LLM calls
	llmCallCount     int // number of LLM calls made
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
		agent:            a,
		model:            model,
		session:          session,
		userMessage:      message,
		trace:            trace,
		maxLLMCalls:      maxLLMCalls,
		eventQueue:       make(chan Event, 100),
		actionQueue:      make(chan Action, 100),
		pendingApprovals: make(map[string]*pendingApproval),
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
				run := newAgentRun(aa, args["input"].(string))
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
			content += event.Content
		case *ErrorEvent:
			err = event.Err
		}
	}
	return content, err
}

func (r *AgentRun) RunAndWait() (string, error) {
	return r.Wait(0)
}

func (r *AgentRun) Approve(eventID string) {
	r.queueAction(&approvalAction{EventID: eventID, Approved: true})
}

func (r *AgentRun) Next() <-chan Event {
	return r.eventQueue
}

func (r *AgentRun) stop() {
	close(r.eventQueue)
	close(r.actionQueue)
}

func (r *AgentRun) processLoop() {
	for action := range r.actionQueue {
		switch act := action.(type) {
		case *stopAction:
			r.stop() // close the channels
			return

		case *llmCallAction:
			r.runLLMCallAction(act.Message, r.tools)

		case *approvalAction:
			if pending, ok := r.pendingApprovals[act.EventID]; ok {
				delete(r.pendingApprovals, act.EventID)
				r.queueAction(&toolCallAction{EventID: act.EventID, ToolName: pending.event.ToolName, ToolArgs: pending.event.ToolArgs, Group: pending.event.ToolGroup})
			}

		case *toolCallAction:
			r.runToolCallAction(act)

		default:
			panic(fmt.Sprintf("unknown action: %T", act))
		}
	}
}

func (r *AgentRun) runLLMCallAction(message string, tools []ai.Tool) {
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

	var respMsg ai.AIMessage
	var err error
	if r.parentRun == nil {
		r.logger.Debug("calling LLM", "model", r.model.ModelName, "messages", len(msgs), "tools", len(tools))
	} else {
		r.logger.Debug("calling sub-agent LLM", "model", r.model.ModelName, "messages", len(msgs), "tools", len(tools))

	}
	respMsg, err = r.model.Call(r.session.Context, msgs, tools)
	if err != nil {
		if r.trace != nil {
			r.trace.RecordError(err)
		}
		r.fireErrorAction(err)
		return
	}
	if r.trace != nil {
		r.trace.LLMAIResponse(r.agent.Name, respMsg.Content, respMsg.ToolCalls, respMsg.Think)
	}
	if respMsg.Think != "" {
		r.fireThinkingAction(respMsg.Think)
	}
	if len(respMsg.ToolCalls) == 0 {
		// No tool calls, safe to add AI message to history immediately
		r.msgHistory = append(r.msgHistory, respMsg)
		r.fireContentAction(respMsg.Content, true)
		return
	}

	// Create a tool call group to coordinate all tool calls
	// Process each tool call individually, passing the group
	group := &toolCallGroup{
		aiMessage: &respMsg,
		responses: make(map[string]ai.ToolMessage),
	}

	for _, tc := range respMsg.ToolCalls {
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(tc.Args), &args); err != nil {
			if r.trace != nil {
				r.trace.RecordError(err)
			}
			// respond with an error to the LLM
			r.fireToolResponseAction(&toolCallAction{
				EventID: "invalid-tool", ToolName: tc.Name, ToolArgs: args, Group: group},
				fmt.Sprintf("invalid tool parameters: %v", err))
			continue
		}
		r.fireToolCallAction(tc.Name, args, tc.ID, group)
	}
}

func (r *AgentRun) runToolCallAction(act *toolCallAction) {
	tool := r.findTool(act.ToolName)
	if tool == nil {
		r.fireToolResponseAction(act, fmt.Sprintf("tool not found: %s", act.ToolName))
		return
	}
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

// Add queueEvent and queueAction methods to AgentRun
func (r *AgentRun) queueEvent(event Event) {
	// if this is a sub-agent, queue the event to the parent agent
	if r.parentRun != nil {
		r.parentRun.queueEvent(event)
		return
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
