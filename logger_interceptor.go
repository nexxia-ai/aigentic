package aigentic

import (
	"sync"
	"time"

	"github.com/nexxia-ai/aigentic/ai"
)

type loggerInterceptor struct {
	mu            sync.Mutex
	llmCallTimers map[string]time.Time
	toolTimers    map[string]time.Time
}

func newLoggerInterceptor() *loggerInterceptor {
	return &loggerInterceptor{
		llmCallTimers: make(map[string]time.Time),
		toolTimers:    make(map[string]time.Time),
	}
}

func (l *loggerInterceptor) BeforeCall(run *AgentRun, messages []ai.Message, tools []ai.Tool) ([]ai.Message, []ai.Tool, error) {
	callID := run.ID()
	l.mu.Lock()
	l.llmCallTimers[callID] = time.Now()
	l.mu.Unlock()

	if run.parentRun == nil {
		run.Logger.Debug("calling LLM", "model", run.model.ModelName, "messages", len(messages), "tools", len(tools))
	} else {
		run.Logger.Debug("calling sub-agent LLM", "model", run.model.ModelName, "messages", len(messages), "tools", len(tools))
	}

	return messages, tools, nil
}

func (l *loggerInterceptor) AfterCall(run *AgentRun, request []ai.Message, response ai.AIMessage) (ai.AIMessage, error) {
	callID := run.ID()
	l.mu.Lock()
	startTime, exists := l.llmCallTimers[callID]
	var duration time.Duration
	if exists {
		delete(l.llmCallTimers, callID)
		duration = time.Since(startTime)
	}
	l.mu.Unlock()

	if run.parentRun == nil {
		if exists {
			run.Logger.Debug("LLM call completed", "model", run.model.ModelName, "duration", duration, "tool_calls", len(response.ToolCalls))
		} else {
			run.Logger.Debug("LLM call completed", "model", run.model.ModelName, "tool_calls", len(response.ToolCalls))
		}
	} else {
		if exists {
			run.Logger.Debug("sub-agent LLM call completed", "model", run.model.ModelName, "duration", duration, "tool_calls", len(response.ToolCalls))
		} else {
			run.Logger.Debug("sub-agent LLM call completed", "model", run.model.ModelName, "tool_calls", len(response.ToolCalls))
		}
	}

	return response, nil
}

func (l *loggerInterceptor) BeforeToolCall(run *AgentRun, toolName string, toolCallID string, validationResult ValidationResult) (ValidationResult, error) {
	timerKey := toolCallID
	l.mu.Lock()
	l.toolTimers[timerKey] = time.Now()
	l.mu.Unlock()

	run.Logger.Debug("calling tool", "tool", toolName, "tool_call_id", toolCallID, "args", validationResult)

	return validationResult, nil
}

func (l *loggerInterceptor) AfterToolCall(run *AgentRun, toolName string, toolCallID string, validationResult ValidationResult, result *ai.ToolResult) (*ai.ToolResult, error) {
	timerKey := toolCallID
	l.mu.Lock()
	startTime, exists := l.toolTimers[timerKey]
	var duration time.Duration
	if exists {
		delete(l.toolTimers, timerKey)
		duration = time.Since(startTime)
	}
	l.mu.Unlock()

	if result != nil && result.Error {
		if exists {
			run.Logger.Debug("tool call completed with error", "tool", toolName, "tool_call_id", toolCallID, "duration", duration)
		} else {
			run.Logger.Debug("tool call completed with error", "tool", toolName, "tool_call_id", toolCallID)
		}
	} else {
		if exists {
			run.Logger.Debug("tool call completed", "tool", toolName, "tool_call_id", toolCallID, "duration", duration)
		} else {
			run.Logger.Debug("tool call completed", "tool", toolName, "tool_call_id", toolCallID)
		}
	}

	return result, nil
}
