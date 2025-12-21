package run

import (
	"github.com/nexxia-ai/aigentic/ai"
	"github.com/nexxia-ai/aigentic/document"
	"github.com/nexxia-ai/aigentic/event"
)

func (r *AgentRun) runToolResponseAction(action *toolCallAction, content string) {
	toolMsg := ai.ToolMessage{
		Role:       ai.ToolRole,
		Content:    content,
		ToolCallID: action.ToolCallID,
		ToolName:   action.ToolName,
	}
	action.Group.Responses[action.ToolCallID] = toolMsg

	// Don't check completion if we're still streaming (will be checked when final message arrives)
	if r.currentStreamGroup != nil && r.currentStreamGroup == action.Group {
		return
	}

	// Check if all tool calls in this group are completed
	if len(action.Group.Responses) == len(action.Group.AIMessage.ToolCalls) {

		// add all tool responses and queue their events
		for _, tc := range action.Group.AIMessage.ToolCalls {
			if response, exists := action.Group.Responses[tc.ID]; exists {
				r.currentConversationTurn.AddMessage(response)
				var docs []*document.Document
				for _, entry := range r.currentConversationTurn.Documents {
					if entry.ToolID == tc.ID || entry.ToolID == "" {
						docs = append(docs, entry.Document)
					}
				}
				event := &event.ToolResponseEvent{
					RunID:      r.id,
					AgentName:  r.AgentName(),
					SessionID:  r.sessionID,
					ToolCallID: response.ToolCallID,
					ToolName:   response.ToolName,
					Content:    response.Content,
					Documents:  docs,
				}
				r.queueEvent(event)
			}
		}

		// Notify any content from the AI message
		if action.Group.AIMessage.Content != "" {
			event := &event.ContentEvent{
				RunID:     r.id,
				AgentName: r.AgentName(),
				SessionID: r.sessionID,
				Content:   action.Group.AIMessage.Content,
			}
			r.queueEvent(event)
		}

		r.queueAction(&llmCallAction{Message: r.userMessage})
	}
}
