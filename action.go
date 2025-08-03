package aigentic

import (
	"fmt"
	"time"

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
	ApprovalID string
	Approved   bool
}

func (a *approvalAction) Target() string { return a.ApprovalID }

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

	if tool.RequireApproval {
		approvalID := uuid.New().String()
		r.pendingApprovals[approvalID] = pendingApproval{
			ApprovalID: approvalID,
			Tool:       tool,
			ToolCallID: toolCallID,
			ToolArgs:   tcArgs,
			Group:      group,
			deadline:   time.Now().Add(r.approvalTimeout),
		}
		approvalEvent := &ApprovalEvent{
			RunID:      r.id,
			ApprovalID: approvalID,
			Content:    fmt.Sprintf("Approval required for tool: %s", tcName),
		}
		r.queueEvent(approvalEvent)
		return
	}

	r.queueAction(&toolCallAction{ToolCallID: toolCallID, ToolName: tcName, ToolArgs: tcArgs, Group: group})
}

func (r *AgentRun) fireToolResponseAction(action *toolCallAction, content string) {
	// Add response to the group
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

		// Send any content from the AI message
		if action.Group.aiMessage.Content != "" {
			r.fireContentAction(action.Group.aiMessage.Content, true)
		}

		// Trigger the next LLM call
		r.fireLLMCallAction(r.userMessage, r.agent.Tools)
	}
}

func (r *AgentRun) fireErrorAction(err error) {
	event := &ErrorEvent{
		RunID:     r.id,
		AgentName: r.agent.Name,
		SessionID: r.session.ID,
		Err:       err,
	}
	r.queueEvent(event)
	r.queueAction(&stopAction{EventID: event.RunID})
}

func (r *AgentRun) fireContentAction(content string, isChunk bool) {
	event := &ContentEvent{
		RunID:     r.id,
		AgentName: r.agent.Name,
		SessionID: r.session.ID,
		Content:   content,
		IsChunk:   isChunk,
	}
	r.queueEvent(event)

	if !isChunk {
		r.queueAction(&stopAction{EventID: uuid.New().String()})
	}
}

func (r *AgentRun) fireThinkingAction(thought string) {
	event := &ThinkingEvent{
		RunID:     r.id,
		AgentName: r.agent.Name,
		SessionID: r.session.ID,
		Thought:   thought,
	}
	r.queueEvent(event)
	// no action needed
}
