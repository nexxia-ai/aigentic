package run

import (
	"github.com/nexxia-ai/aigentic/ai"
	"github.com/nexxia-ai/aigentic/ctxt"
	"github.com/nexxia-ai/aigentic/document"
	"github.com/nexxia-ai/aigentic/event"
)

func (r *AgentRun) runToolResponseAction(action *toolCallAction, content string, fileRefs []ctxt.FileRefEntry) {
	// Store original content for user-facing display
	action.Group.UserResponses[action.ToolCallID] = content
	
	// For LLM: include file content in the tool message
	llmContent := content
	if len(fileRefs) > 0 {
		llmContent = appendFileRefsToToolResponse(r, content, fileRefs)
	}

	toolMsg := ai.ToolMessage{
		Role:       ai.ToolRole,
		Content:    llmContent,
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
		turn := r.agentContext.Turn()
		for _, tc := range action.Group.AIMessage.ToolCalls {
			if response, exists := action.Group.Responses[tc.ID]; exists {
				turn.AddMessage(response)
				var docs []*document.Document
				for _, entry := range turn.Documents {
					if entry.ToolID == tc.ID || entry.ToolID == "" {
						docs = append(docs, entry.Document)
					}
				}
				userContent := action.Group.UserResponses[tc.ID]
				event := &event.ToolResponseEvent{
					RunID:      r.id,
					AgentName:  r.AgentName(),
					SessionID:  r.sessionID,
					ToolCallID: response.ToolCallID,
					ToolName:   response.ToolName,
					Content:    userContent,
					Documents:  docs,
				}
				r.queueEvent(event)
			}
		}

		if action.Group.AIMessage.Content != "" {
			event := &event.ContentEvent{
				RunID:     r.id,
				AgentName: r.AgentName(),
				SessionID: r.sessionID,
				Content:   action.Group.AIMessage.Content,
			}
			r.queueEvent(event)
		}

		if action.Group.Terminal {
			r.queueAction(&stopAction{})
		} else {
			r.queueAction(&llmCallAction{Message: r.agentContext.Turn().UserMessage})
		}
	}
}
