package run

import (
	"github.com/nexxia-ai/aigentic/ai"
	"github.com/nexxia-ai/aigentic/ctxt"
	"github.com/nexxia-ai/aigentic/event"
)

func filesForToolEvent(group *ToolCallGroup, toolCallID string, turn *ctxt.Turn) []ctxt.FileRef {
	if group.FileRefs != nil {
		if refs := group.FileRefs[toolCallID]; len(refs) > 0 {
			return refs
		}
	}
	return turn.FilesForTool(toolCallID)
}

func endTerminalToolGroup(r *AgentRun, group *ToolCallGroup) {
	if r == nil || group == nil || group.AIMessage == nil {
		return
	}
	finalMsg := *group.AIMessage
	finalMsg.ToolCalls = nil
	r.agentContext.EndTurn(finalMsg)
}

func (r *AgentRun) runToolResponseAction(action *toolCallAction, content string, fileRefs []ctxt.FileRef) {
	// Store original content for user-facing display
	action.Group.UserResponses[action.ToolCallID] = content
	if action.Group.FileRefs == nil {
		action.Group.FileRefs = make(map[string][]ctxt.FileRef)
	}
	action.Group.FileRefs[action.ToolCallID] = fileRefs

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
				files := filesForToolEvent(action.Group, tc.ID, turn)
				userContent := action.Group.UserResponses[tc.ID]
				ev := &event.ToolResponseEvent{
					RunID:      r.id,
					AgentName:  r.AgentName(),
					SessionID:  r.sessionID,
					ToolCallID: response.ToolCallID,
					ToolName:   response.ToolName,
					Content:    userContent,
					Files:      files,
				}
				r.queueEvent(ev)
			}
		}

		if action.Group.AIMessage.Content != "" && !r.streaming {
			event := &event.ContentEvent{
				RunID:     r.id,
				AgentName: r.AgentName(),
				SessionID: r.sessionID,
				Content:   action.Group.AIMessage.Content,
			}
			r.queueEvent(event)
		}

		if action.Group.Terminal {
			endTerminalToolGroup(r, action.Group)
			r.queueAction(&stopAction{})
		} else {
			r.queueAction(&llmCallAction{Message: r.agentContext.Turn().UserMessage})
		}
	}
}
