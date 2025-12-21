package aigentic

import (
	"sync"
	"time"

	"github.com/nexxia-ai/aigentic/ai"
	"github.com/nexxia-ai/aigentic/event"
	"github.com/nexxia-ai/aigentic/run"
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

func (l *loggerInterceptor) BeforeCall(agentRun *run.AgentRun, messages []ai.Message, tools []ai.Tool) ([]ai.Message, []ai.Tool, error) {
	callID := agentRun.ID()
	l.mu.Lock()
	l.llmCallTimers[callID] = time.Now()
	l.mu.Unlock()

	agentRun.Logger.Debug("calling LLM", "model", agentRun.Model().ModelName, "messages", len(messages), "tools", len(tools))

	return messages, tools, nil
}

func (l *loggerInterceptor) AfterCall(agentRun *run.AgentRun, request []ai.Message, response ai.AIMessage) (ai.AIMessage, error) {
	callID := agentRun.ID()
	l.mu.Lock()
	startTime, exists := l.llmCallTimers[callID]
	var duration time.Duration
	if exists {
		delete(l.llmCallTimers, callID)
		duration = time.Since(startTime)
	}
	l.mu.Unlock()

	if exists {
		agentRun.Logger.Debug("LLM call completed", "model", agentRun.Model().ModelName, "duration", duration, "tool_calls", len(response.ToolCalls))
	} else {
		agentRun.Logger.Debug("LLM call completed", "model", agentRun.Model().ModelName, "tool_calls", len(response.ToolCalls))
	}

	return response, nil
}

func (l *loggerInterceptor) BeforeToolCall(agentRun *run.AgentRun, toolName string, toolCallID string, validationResult event.ValidationResult) (event.ValidationResult, error) {
	timerKey := toolCallID
	l.mu.Lock()
	l.toolTimers[timerKey] = time.Now()
	l.mu.Unlock()

	agentRun.Logger.Debug("calling tool", "tool", toolName, "tool_call_id", toolCallID, "args", validationResult)

	return validationResult, nil
}

func (l *loggerInterceptor) AfterToolCall(agentRun *run.AgentRun, toolName string, toolCallID string, validationResult event.ValidationResult, result *ai.ToolResult) (*ai.ToolResult, error) {
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
			agentRun.Logger.Debug("tool call completed with error", "tool", toolName, "tool_call_id", toolCallID, "duration", duration)
		} else {
			agentRun.Logger.Debug("tool call completed with error", "tool", toolName, "tool_call_id", toolCallID)
		}
	} else {
		if exists {
			agentRun.Logger.Debug("tool call completed", "tool", toolName, "tool_call_id", toolCallID, "duration", duration)
		} else {
			agentRun.Logger.Debug("tool call completed", "tool", toolName, "tool_call_id", toolCallID)
		}
	}

	return result, nil
}
