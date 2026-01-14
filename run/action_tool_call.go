package run

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/nexxia-ai/aigentic/ai"
	"github.com/nexxia-ai/aigentic/event"
)

func (r *AgentRun) runToolCallAction(act *toolCallAction) {
	tool := r.findTool(act.ToolName)
	if tool == nil {
		r.queueAction(&toolResponseAction{
			request:  act,
			response: fmt.Sprintf("tool not found: %s", act.ToolName),
		})
		return
	}

	eventID := uuid.New().String()
	toolEvent := &event.ToolEvent{
		RunID:     r.id,
		EventID:   eventID,
		AgentName: r.AgentName(),
		SessionID: r.sessionID,
		ToolName:  act.ToolName,
		Args:      act.Args,
		ToolGroup: act.Group,
	}
	r.queueEvent(toolEvent)

	currentArgs := act.Args
	var err error
	interceptors := r.interceptors

	if r.enableTrace {
		interceptors = append(interceptors, r.trace)
	}
	for _, interceptor := range interceptors {
		currentArgs, err = interceptor.BeforeToolCall(r, act.ToolName, act.ToolCallID, currentArgs)
		if err != nil {
			errMsg := fmt.Sprintf("interceptor rejected tool call: %v", err)
			r.queueAction(&toolResponseAction{request: act, response: errMsg})
			return
		}
	}

	result, err := tool.call(r, currentArgs)
	if err != nil {
		if r.enableTrace {
			r.trace.RecordError(err)
		}
		errMsg := fmt.Sprintf("tool execution error: %v", err)
		r.queueAction(&toolResponseAction{request: act, response: errMsg})
		return
	}

	currentResult := result
	for _, interceptor := range interceptors {
		currentResult, err = interceptor.AfterToolCall(r, act.ToolName, act.ToolCallID, currentArgs, currentResult)
		if err != nil {
			errMsg := fmt.Sprintf("interceptor error after tool call: %v", err)
			if r.enableTrace {
				r.trace.RecordError(err)
			}
			r.queueAction(&toolResponseAction{request: act, response: errMsg})
			return
		}
	}

	response := formatToolResponse(currentResult)

	if currentResult != nil && currentResult.Error {
		toolErr := fmt.Errorf("tool %s reported error", act.ToolName)
		if response != "" {
			toolErr = fmt.Errorf("tool %s reported error: %s", act.ToolName, response)
		}
		if r.enableTrace {
			r.trace.RecordError(toolErr)
		}
	}

	r.queueAction(&toolResponseAction{request: act, response: response})
}

func (r *AgentRun) findTool(tcName string) *AgentTool {
	for i := range r.tools {
		if r.tools[i].Name == tcName {
			return &r.tools[i]
		}
	}
	for i := range r.subAgents {
		if r.subAgents[i].Name == tcName {
			return &r.subAgents[i]
		}
	}
	return nil
}

func formatToolResponse(result *ai.ToolResult) string {
	if result == nil || len(result.Content) == 0 {
		return ""
	}

	parts := make([]string, 0, len(result.Content))
	for _, item := range result.Content {
		segment := stringifyToolContent(item.Content)
		if segment == "" {
			continue
		}
		if item.Type != "" && item.Type != "text" {
			segment = fmt.Sprintf("[%s] %s", item.Type, segment)
		}
		parts = append(parts, segment)
	}

	return strings.Join(parts, "\n")
}

func stringifyToolContent(content any) string {
	switch v := content.(type) {
	case nil:
		return ""
	case string:
		return v
	case []byte:
		if utf8.Valid(v) {
			return string(v)
		}
		return fmt.Sprintf("0x%x", v)
	case fmt.Stringer:
		return v.String()
	default:
		encoded, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(encoded)
	}
}

func (r *AgentRun) processToolCall(tc ai.ToolCall, group *ToolCallGroup) {
	if r.processedToolCallIDs[tc.ID] {
		return
	}
	r.processedToolCallIDs[tc.ID] = true

	var args map[string]interface{}
	if err := json.Unmarshal([]byte(tc.Args), &args); err != nil {
		if r.enableTrace {
			r.trace.RecordError(err)
		}
		r.queueAction(&toolResponseAction{
			request: &toolCallAction{
				ToolCallID: tc.ID,
				ToolName:   tc.Name,
				Args:       args,
				Group:      group,
			},
			response: fmt.Sprintf("invalid tool parameters: %v", err),
		})
		return
	}

	tool := r.findTool(tc.Name)
	if tool == nil {
		r.queueAction(&toolResponseAction{
			request:  &toolCallAction{ToolName: tc.Name, Args: args, Group: group},
			response: fmt.Sprintf("tool not found: %s", tc.Name),
		})
		return
	}

	r.queueAction(&toolCallAction{ToolCallID: tc.ID, ToolName: tc.Name, Args: args, Group: group})
}

// processToolCallsFromChunk processes tool calls from a streaming chunk using the shared stream group
func (r *AgentRun) processToolCallsFromChunk(toolCalls []ai.ToolCall) {
	for _, tc := range toolCalls {
		r.processToolCall(tc, r.currentStreamGroup)
	}
}

// groupToolCalls processes a slice of tool calls and queues the appropriate actions
func (r *AgentRun) groupToolCalls(toolCalls []ai.ToolCall, msg ai.AIMessage, existingGroup *ToolCallGroup) {
	var group *ToolCallGroup
	if existingGroup != nil {
		group = existingGroup
		group.AIMessage = &msg
	} else {
		group = &ToolCallGroup{
			AIMessage: &msg,
			Responses: make(map[string]ai.ToolMessage),
		}
	}

	for _, tc := range toolCalls {
		r.processToolCall(tc, group)
	}
}
