package run

import (
	"fmt"

	"github.com/nexxia-ai/aigentic/ai"
	"github.com/nexxia-ai/aigentic/document"
	"github.com/nexxia-ai/aigentic/event"
)

func (r *AgentRun) runLLMCallAction(message string) {

	// Check LLM call limit before making any LLM call
	if r.maxLLMCalls > 0 && r.llmCallCount >= r.maxLLMCalls {
		err := fmt.Errorf("LLM call limit exceeded: %d calls (configured limit: %d)",
			r.llmCallCount, r.maxLLMCalls)
		r.queueAction(&stopAction{Error: err})
		return
	}
	r.llmCallCount++ // Increment counter

	// Get all tools from agent, system, sub-agents, and retrievers
	allTools := make([]AgentTool, 0, len(r.tools)+len(r.sysTools)+len(r.subAgents))
	allTools = append(allTools, r.tools...)
	allTools = append(allTools, r.sysTools...)
	allTools = append(allTools, r.subAgents...)
	for _, retriever := range r.retrievers {
		allTools = append(allTools, retriever.ToTool())
	}

	// Clear processed tool call IDs and stream group for this new LLM call
	r.processedToolCallIDs = make(map[string]bool)
	r.currentStreamGroup = nil

	tools := make([]ai.Tool, len(allTools))
	for i, agentTool := range allTools {
		tools[i] = agentTool.toTool(r)
	}

	event := &event.LLMCallEvent{
		RunID:     r.id,
		AgentName: r.AgentName(),
		SessionID: r.sessionID,
		Message:   message,
		Tools:     tools,
	}
	r.queueEvent(event)

	var err error
	var msgs []ai.Message
	msgs, err = r.agentContext.BuildPrompt(tools, r.includeHistory)
	if err != nil {
		r.queueAction(&stopAction{Error: err})
		return
	}

	// Chain BeforeCall interceptors
	currentMsgs := msgs
	currentTools := tools
	interceptors := r.interceptors

	// Trace must be the last interceptor to capture the full exchange
	if r.enableTrace {
		interceptors = append(interceptors, r.trace)
	}
	for _, interceptor := range interceptors {
		currentMsgs, currentTools, err = interceptor.BeforeCall(r, currentMsgs, currentTools)
		if err != nil {
			r.queueAction(&stopAction{Error: fmt.Errorf("interceptor rejected: %w", err)})
			return
		}
	}

	var respMsg ai.AIMessage

	switch r.streaming {
	case true:
		respMsg, err = r.model.Stream(r.ctx, currentMsgs, currentTools, func(chunk ai.AIMessage) error {
			// Handle each chunk as a non-final message
			r.handleAIMessage(chunk, true) // isChunk is true
			return nil
		})

	default:
		respMsg, err = r.model.Call(r.ctx, currentMsgs, currentTools)

	}

	if err != nil {
		if r.enableTrace {
			r.trace.RecordError(err)
		}
		r.queueAction(&stopAction{Error: err})
		return
	}

	// Chain AfterCall interceptors
	currentResp := respMsg
	for _, interceptor := range interceptors {
		currentResp, err = interceptor.AfterCall(r, currentMsgs, currentResp)
		if err != nil {
			r.queueAction(&stopAction{Error: fmt.Errorf("interceptor error: %w", err)})
			return
		}
	}

	r.turnMetrics.add(currentResp.Response.Usage)
	r.handleAIMessage(currentResp, false)
}

// handleAIMessage handles the response from the LLM, whether it's a complete message or a chunk
func (r *AgentRun) handleAIMessage(msg ai.AIMessage, isChunk bool) {
	// only fire events if not streaming or if this is a chunk in streaming.
	// do not fire event if this is the last chunk (streaming) to prevent duplicate content
	if !r.streaming || isChunk {
		if msg.Think != "" {
			event := &event.ThinkingEvent{
				RunID:     r.id,
				AgentName: r.AgentName(),
				SessionID: r.sessionID,
				Thought:   msg.Think,
			}
			r.queueEvent(event)
		}

		if msg.Content != "" {
			event := &event.ContentEvent{
				RunID:     r.id,
				AgentName: r.AgentName(),
				SessionID: r.sessionID,
				Content:   msg.Content,
			}
			r.queueEvent(event)
		}
	}

	// Process tool calls from chunks immediately for better UX, but track them to avoid duplicates
	if isChunk {
		if len(msg.ToolCalls) > 0 {
			// Initialize stream group if this is the first chunk with tool calls
			if r.currentStreamGroup == nil {
				chunkMsg := ai.AIMessage{
					Role:      msg.Role,
					ToolCalls: msg.ToolCalls,
				}
				r.currentStreamGroup = &ToolCallGroup{
					AIMessage:     &chunkMsg,
					Responses:     make(map[string]ai.ToolMessage),
					UserResponses: make(map[string]string),
				}
			}
			// Process tool calls using the shared group
			r.processToolCallsFromChunk(msg.ToolCalls)
		}
		return
	}

	// this not a chunk, which means the model Call/Stream is complete
	// end the turn and fire tool calls
	if len(msg.ToolCalls) == 0 {
		msg.Response.Usage = r.turnMetrics.usage
		r.agentContext.EndTurn(msg)
		r.queueAction(&stopAction{Error: nil})
		return
	}

	turn := r.agentContext.Turn()
	turn.AddMessage(msg)

	// If we have a stream group from chunks, update it with the final message, otherwise create new group
	if r.currentStreamGroup != nil {
		r.currentStreamGroup.AIMessage = &msg
		r.groupToolCalls(msg.ToolCalls, msg, r.currentStreamGroup)
		// Check if all tool calls in the group are now completed (now that we have the final message)
		if len(r.currentStreamGroup.Responses) == len(r.currentStreamGroup.AIMessage.ToolCalls) {
			// add all tool responses and queue their events
			turn := r.agentContext.Turn()
			for _, tc := range r.currentStreamGroup.AIMessage.ToolCalls {
				if response, exists := r.currentStreamGroup.Responses[tc.ID]; exists {
					turn.AddMessage(response)
					var docs []*document.Document
					for _, entry := range turn.Documents {
						if entry.ToolID == tc.ID || entry.ToolID == "" {
							docs = append(docs, entry.Document)
						}
					}
					userContent := r.currentStreamGroup.UserResponses[tc.ID]
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

			// Notify any content from the AI message (skip when streaming; already sent in chunks)
			if r.currentStreamGroup.AIMessage.Content != "" && !r.streaming {
				event := &event.ContentEvent{
					RunID:     r.id,
					AgentName: r.AgentName(),
					SessionID: r.sessionID,
					Content:   r.currentStreamGroup.AIMessage.Content,
				}
				r.queueEvent(event)
			}

			if r.currentStreamGroup.Terminal {
				r.queueAction(&stopAction{})
			} else {
				r.queueAction(&llmCallAction{Message: r.agentContext.Turn().UserMessage})
			}
		}
		r.currentStreamGroup = nil // clear after processing
	} else {
		r.groupToolCalls(msg.ToolCalls, msg, nil)
	}
}
