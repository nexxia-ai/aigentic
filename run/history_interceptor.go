package run

import (
	"github.com/nexxia-ai/aigentic/ai"
	"github.com/nexxia-ai/aigentic/conversation"
	"github.com/nexxia-ai/aigentic/event"
)

type historyInterceptor struct {
	history *conversation.ConversationHistory
}

func newHistoryInterceptor(history *conversation.ConversationHistory) *historyInterceptor {
	return &historyInterceptor{
		history: history,
	}
}

func (h *historyInterceptor) BeforeCall(run *AgentRun, messages []ai.Message, tools []ai.Tool) ([]ai.Message, []ai.Tool, error) {
	if len(messages) == 0 {
		return messages, tools, nil
	}

	historyMessages := h.history.GetMessages()
	if len(historyMessages) == 0 {
		return messages, tools, nil
	}

	userMessageIndex := -1
	for i, msg := range messages {
		role, _ := msg.Value()
		if role == ai.UserRole {
			if _, ok := msg.(ai.UserMessage); ok {
				userMessageIndex = i
				break
			}
		}
	}

	if userMessageIndex == -1 {
		result := make([]ai.Message, 0, len(messages)+len(historyMessages))
		result = append(result, messages...)
		result = append(result, historyMessages...)
		return result, tools, nil
	}

	result := make([]ai.Message, 0, len(messages)+len(historyMessages))
	result = append(result, messages[:userMessageIndex]...)
	result = append(result, historyMessages...)
	result = append(result, messages[userMessageIndex:]...)
	return result, tools, nil
}

func (h *historyInterceptor) AfterCall(run *AgentRun, request []ai.Message, response ai.AIMessage) (ai.AIMessage, error) {
	if len(response.ToolCalls) == 0 {
		run.currentConversationTurn.Reply = response
		h.history.AppendTurn(*run.currentConversationTurn)
	}

	return response, nil
}

func (h *historyInterceptor) BeforeToolCall(run *AgentRun, toolName string, toolCallID string, validationResult event.ValidationResult) (event.ValidationResult, error) {
	return validationResult, nil
}

func (h *historyInterceptor) AfterToolCall(run *AgentRun, toolName string, toolCallID string, validationResult event.ValidationResult, result *ai.ToolResult) (*ai.ToolResult, error) {
	return result, nil
}
