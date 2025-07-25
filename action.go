package aigentic

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/nexxia-ai/aigentic/ai"
)

type Action interface {
	Target() string
}

type llmCallAction struct {
	Message string
}

func (a *llmCallAction) Target() string { return "" }

type approvalAction struct {
	EventID  string
	Approved bool
}

func (a *approvalAction) Target() string { return a.EventID }

type toolCallAction struct {
	EventID    string
	ToolCallID string
	ToolName   string
	ToolArgs   map[string]interface{}
	Group      *toolCallGroup
}

func (a *toolCallAction) Target() string { return a.EventID }

type stopAction struct {
	EventID string
}

func (a *stopAction) Target() string { return a.EventID }

type cancelAction struct {
	EventID string
}

func (a *cancelAction) Target() string { return a.EventID }

func (r *AgentRun) fireLLMCallAction(msg string, tools []ai.Tool) {
	event := &LLMCallEvent{
		EventID:   uuid.New().String(),
		AgentName: r.agent.Name,
		SessionID: r.session.ID,
		Message:   msg,
		Tools:     tools,
	}
	r.queueEvent(event)
	r.queueAction(&llmCallAction{Message: msg})
}

func (r *AgentRun) fireToolCallAction(tcName string, tcArgs map[string]interface{}, toolCallID string, group *toolCallGroup) {
	tool := r.findTool(tcName)
	if tool == nil {
		r.fireToolResponseAction(&toolCallAction{
			EventID: "invalid-tool", ToolName: tcName, ToolArgs: tcArgs, Group: group},
			fmt.Sprintf("tool not found: %s", tcName))
		return
	}
	eventID := uuid.New().String()
	toolEvent := &ToolEvent{
		EventID:         eventID,
		AgentName:       r.agent.Name,
		SessionID:       r.session.ID,
		ToolName:        tcName,
		ToolArgs:        tcArgs,
		RequireApproval: tool.RequireApproval,
		ToolGroup:       group,
	}
	if tool.RequireApproval {
		r.pendingApprovals[eventID] = &pendingApproval{event: toolEvent}
	}
	r.queueEvent(toolEvent) // send after adding to the map
	if !tool.RequireApproval {
		r.queueAction(&toolCallAction{EventID: eventID, ToolCallID: toolCallID, ToolName: tcName, ToolArgs: tcArgs, Group: group})
	}
}

func (r *AgentRun) fireToolResponseAction(action *toolCallAction, content string) {
	event := &ToolResponseEvent{
		EventID:    uuid.New().String(),
		AgentName:  r.agent.Name,
		SessionID:  r.session.ID,
		ToolCallID: action.ToolCallID,
		ToolName:   action.ToolName,
		Content:    content,
	}

	r.queueEvent(event)

	// Add response to the group
	toolMsg := ai.ToolMessage{
		Role:       ai.ToolRole,
		Content:    content,
		ToolCallID: action.ToolCallID,
	}
	action.Group.responses[action.ToolCallID] = toolMsg

	// Check if all tool calls in this group are completed
	if len(action.Group.responses) == len(action.Group.aiMessage.ToolCalls) {
		// Then add the AI message to history
		r.msgHistory = append(r.msgHistory, *action.Group.aiMessage)

		// Add all tool responses last
		for _, tc := range action.Group.aiMessage.ToolCalls {
			if response, exists := action.Group.responses[tc.ID]; exists {
				r.msgHistory = append(r.msgHistory, response)
			}
		}

		// Send any content from the AI message
		if action.Group.aiMessage.Content != "" {
			r.fireContentAction(action.Group.aiMessage.Content, false)
		}

		// Trigger the next LLM call
		r.fireLLMCallAction(r.userMessage, r.agent.Tools)
	}
}

func (r *AgentRun) fireErrorAction(err error) {
	event := &ErrorEvent{
		EventID:   uuid.New().String(),
		AgentName: r.agent.Name,
		SessionID: r.session.ID,
		Err:       err,
	}
	r.queueEvent(event)
	r.queueAction(&stopAction{EventID: event.EventID})
}

func (r *AgentRun) fireContentAction(content string, isFinal bool) {
	event := &ContentEvent{
		EventID:   uuid.New().String(),
		AgentName: r.agent.Name,
		SessionID: r.session.ID,
		Content:   content,
		IsFinal:   isFinal,
	}
	r.queueEvent(event)
	if isFinal {
		r.queueAction(&stopAction{EventID: event.EventID})
	}
}

func (r *AgentRun) fireThinkingAction(thought string) {
	event := &ThinkingEvent{
		EventID:   uuid.New().String(),
		AgentName: r.agent.Name,
		SessionID: r.session.ID,
		Thought:   thought,
	}
	r.queueEvent(event)
	// no action needed
}
