package run

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/nexxia-ai/aigentic/ai"
	"github.com/nexxia-ai/aigentic/document"
	"github.com/nexxia-ai/aigentic/event"
)

var approvalTimeout = time.Minute * 60

type AgentRun struct {
	id        string
	sessionID string // a unique identifier for multiple runs
	model     *ai.Model
	agentName string

	ctx        context.Context
	cancelFunc context.CancelFunc

	tools []AgentTool

	agentContext *AgentContext
	interceptors []Interceptor

	eventQueue              chan event.Event
	actionQueue             chan action
	pendingApprovals        map[string]pendingApproval
	processedToolCallIDs    map[string]bool
	currentStreamGroup      *ToolCallGroup
	trace                   *TraceRun
	userMessage             string
	parentRun               *AgentRun
	Logger                  *slog.Logger
	maxLLMCalls             int
	llmCallCount            int
	approvalTimeout         time.Duration
	currentConversationTurn *ConversationTurn

	// ContextManager defines the context manager for the agent.
	// If set, this context manager will be used instead of the default BasicContextManager.
	// Set "ContextManager: aigentic.NewEnhancedSystemContextManager(agent, message)" to use a custom context manager.
	ContextManager ContextManager

	// ContextFunctions contains functions that provide dynamic context for the agent.
	// These functions are called before each LLM call and their output is included
	// as a separate user message wrapped in <Session context> tags.
	ContextFunctions []ContextFunction

	streaming bool

	retrievers []Retriever

	subAgents []AgentTool
}

func (r *AgentRun) ID() string {
	return r.id
}

func (r *AgentRun) AgentName() string {
	return r.agentName
}

func (r *AgentRun) SetStreaming(streaming bool) {
	r.streaming = streaming
}

func (r *AgentRun) Model() *ai.Model {
	return r.model
}

func (r *AgentRun) ConversationTurn() *ConversationTurn {
	return r.currentConversationTurn
}

func (r *AgentRun) Cancel() {
	if r.cancelFunc != nil {
		r.cancelFunc()
	}
}

// AddMemory adds a memory entry or updates an existing one
func (r *AgentRun) AddMemory(id, description, content, scope string) error {
	return r.agentContext.AddMemory(id, description, content, scope, r.id)
}

// DeleteMemory removes a memory entry by ID
func (r *AgentRun) DeleteMemory(id string) error {
	return r.agentContext.DeleteMemory(id)
}

// GetMemories returns all memories in insertion order
func (r *AgentRun) GetMemories() []MemoryEntry {
	return r.agentContext.GetMemories()
}

// AddDocument adds a document to the conversation turn and optionally to the session
func (r *AgentRun) AddDocument(toolID string, doc *document.Document, scope string) error {
	if doc == nil {
		return fmt.Errorf("document cannot be nil")
	}

	if scope != "local" && scope != "model" && scope != "session" {
		return fmt.Errorf("invalid scope: %s (must be 'local', 'model', or 'session')", scope)
	}

	entry := DocumentEntry{
		Document: doc,
		Scope:    scope,
		ToolID:   toolID,
	}

	r.currentConversationTurn.Documents = append(r.currentConversationTurn.Documents, entry)

	if scope == "model" || scope == "session" {
		r.agentContext.documents = append(r.agentContext.documents, doc)
	}

	return nil
}

func (r *AgentRun) SetRetrievers(retrievers []Retriever) {
	r.retrievers = retrievers
}

func NewAgentRun(name, description, instructions, message string) *AgentRun {
	runID := uuid.New().String()
	sessionID := uuid.New().String()
	runCtx, cancelFunc := context.WithCancel(context.Background())

	model := ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
		return ai.AIMessage{}, fmt.Errorf("agent model is not set")
	})

	ac := NewAgentContext(description, instructions, message)
	run := &AgentRun{
		agentName:               name,
		id:                      runID,
		sessionID:               sessionID,
		ctx:                     runCtx,
		cancelFunc:              cancelFunc,
		agentContext:            ac,
		model:                   model,
		maxLLMCalls:             20,
		eventQueue:              make(chan event.Event, 100),
		actionQueue:             make(chan action, 100),
		pendingApprovals:        make(map[string]pendingApproval),
		processedToolCallIDs:    make(map[string]bool),
		approvalTimeout:         approvalTimeout,
		interceptors:            make([]Interceptor, 0),
		tools:                   make([]AgentTool, 0),
		streaming:               false,
		currentConversationTurn: NewConversationTurn(message, runID, "", ""),
		Logger:                  slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})).With("agent", name),
	}

	return run
}

func (r *AgentRun) AgentContext() *AgentContext {
	return r.agentContext
}

func (r *AgentRun) SetModel(model *ai.Model) {
	r.model = model
}

func (r *AgentRun) SetAgentName(agentName string) {
	r.agentName = agentName
}

func (r *AgentRun) SetInterceptors(interceptors []Interceptor) {
	r.interceptors = interceptors
}

func (r *AgentRun) SetMaxLLMCalls(maxLLMCalls int) {
	r.maxLLMCalls = maxLLMCalls
}

func (r *AgentRun) SetTracer(tracer *Tracer) {
	if tracer != nil {
		r.trace = tracer.NewTraceRun()
	}
}

func (r *AgentRun) SetTools(tools []AgentTool) {
	r.tools = tools
}

func (r *AgentRun) EnableHistory() {
	r.interceptors = append(r.interceptors, newHistoryInterceptor(r.agentContext.conversationHistory))
}

func (r *AgentRun) SetConversationHistory(history *ConversationHistory) {
	r.agentContext.SetConversationHistory(history)
	if history != nil {
		r.interceptors = append(r.interceptors, newHistoryInterceptor(history))
	}
}

func (r *AgentRun) Start() {
	// Add trace interceptor if present - this must be the last interceptor to capture the full response
	if r.trace != nil {
		r.interceptors = append(r.interceptors, r.trace)
	}

	// goroutine to read the action queue and process actions.
	// it will terminate when the action queue is closed and the agent is finished.
	go r.processLoop()
	r.queueAction(&llmCallAction{Message: r.userMessage})
}

func (r *AgentRun) stop() {
	if r.cancelFunc != nil {
		r.cancelFunc()
	}
	close(r.eventQueue)
	close(r.actionQueue)
	// Only close trace if this is not a sub-agent (sub-agents share trace with parent)
	if r.trace != nil && r.parentRun == nil {
		r.trace.Close()
	}
}

func (r *AgentRun) AddSubAgent(name, description, message string, model *ai.Model, tools []AgentTool) {
	agentTool := AgentTool{
		Name:        name,
		Description: description,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"input": map[string]interface{}{
					"type":        "string",
					"description": "The input to send to the agent",
				},
			},
			"required": []string{"input"},
		},
		Validate: func(run *AgentRun, args map[string]interface{}) (event.ValidationResult, error) {
			return event.ValidationResult{Values: args}, nil
		},
		NewExecute: func(run *AgentRun, validationResult event.ValidationResult) (*ai.ToolResult, error) {
			input := ""
			if v, ok := validationResult.Values.(map[string]any)["input"].(string); ok {
				input = v
			}
			subRun := NewAgentRun(name, description, message, input)
			subRun.SetModel(model)
			subRun.SetTools(tools)
			subRun.trace = r.trace
			subRun.Logger = r.Logger.With("sub-agent", name)
			subRun.parentRun = r
			// Inherit Stream setting from parent
			if r.streaming {
				subRun.SetStreaming(true)
			}

			subRun.Start()
			content, err := subRun.Wait(0)

			// Record sub-agent errors to trace
			if r.trace != nil && err != nil {
				r.trace.RecordError(fmt.Errorf("sub-agent %s error: %v", name, err))
			}

			if err != nil {
				return &ai.ToolResult{
					Content: []ai.ToolContent{{
						Type:    "text",
						Content: fmt.Sprintf("Error: %v", err),
					}},
					Error: true,
				}, nil
			}
			return &ai.ToolResult{
				Content: []ai.ToolContent{{
					Type:    "text",
					Content: content,
				}},
				Error: false,
			}, nil
		},
	}
	r.subAgents = append(r.subAgents, agentTool)
}

func (r *AgentRun) addTools() []AgentTool {
	totalToolsCount := len(r.tools) + len(r.subAgents)
	tools := make([]AgentTool, 0, totalToolsCount)
	tools = append(tools, r.tools...)
	tools = append(tools, r.subAgents...)

	// Retriever tools
	for _, retriever := range r.retrievers {
		tools = append(tools, retriever.ToTool())
	}

	// make sure all tools have a validation and execute function
	for i := range tools {
		if tools[i].Validate == nil {
			tools[i].Validate = func(run *AgentRun, args map[string]interface{}) (event.ValidationResult, error) {
				return event.ValidationResult{Values: args, Message: ""}, nil
			}
		}
		if tools[i].NewExecute == nil {
			tools[i].NewExecute = func(run *AgentRun, validationResult event.ValidationResult) (*ai.ToolResult, error) {
				return nil, nil
			}
		}
	}
	return tools
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

func (r *AgentRun) Wait(d time.Duration) (string, error) {
	content := ""
	var err error
	for evt := range r.eventQueue {
		switch event := evt.(type) {
		case *event.ContentEvent:
			// only append content that is for the same run ID so you don't append sub-agent content to the parent agent
			if r.ID() == event.RunID {
				content += event.Content
			}
		case *event.ErrorEvent:
			err = event.Err
		}
	}
	return content, err
}

func (r *AgentRun) Approve(approvalID string, approved bool) {
	r.queueAction(&approvalAction{ApprovalID: approvalID, Approved: approved})
}

func (r *AgentRun) Next() <-chan event.Event {
	return r.eventQueue
}

func (r *AgentRun) TraceFilepath() string {
	if r.trace == nil {
		return ""
	}
	return r.trace.Filepath()
}

// keep it a variable to make it easier to test
var tickerInterval = time.Second * 30

func (r *AgentRun) processLoop() {
	ticker := time.NewTicker(tickerInterval)
	defer ticker.Stop()

	for {
		select {
		case action, ok := <-r.actionQueue:
			if !ok {
				r.runStopAction(&stopAction{Error: fmt.Errorf("action queue closed unexpectedly")})
				return
			}
			switch act := action.(type) {
			case *stopAction:
				r.runStopAction(act)
				return

			case *llmCallAction:
				allTools := make([]AgentTool, 0, len(r.tools)+len(r.subAgents))
				allTools = append(allTools, r.tools...)
				allTools = append(allTools, r.subAgents...)
				for _, retriever := range r.retrievers {
					allTools = append(allTools, retriever.ToTool())
				}
				r.runLLMCallAction(act.Message, allTools)

			case *toolResponseAction:
				r.runToolResponseAction(act.request, act.response)

			case *approvalAction:
				r.runApprovalAction(act)

			case *toolCallAction:
				r.runToolCallAction(act)

			default:
				panic(fmt.Sprintf("unknown action: %T", act))
			}
		case <-ticker.C:
			r.checkApprovalTimeouts()

		case <-r.ctx.Done():
			r.runStopAction(&stopAction{Error: fmt.Errorf("run context cancelled")})
			return
		}
	}
}

func (r *AgentRun) runStopAction(act *stopAction) {
	if act.Error != nil {
		r.Logger.Error("stopping agent", "error", act.Error)
		event := &event.ErrorEvent{
			RunID:     r.id,
			AgentName: r.AgentName(),
			SessionID: r.sessionID,
			Err:       act.Error,
		}
		r.queueEvent(event)
	}

	r.stop()
}

func (r *AgentRun) checkApprovalTimeouts() {
	now := time.Now()
	for approvalID, approval := range r.pendingApprovals {
		if now.After(approval.deadline) {
			r.Logger.Error("approval timed out", "approvalID", approvalID, "deadline", approval.deadline)
			delete(r.pendingApprovals, approvalID)
			r.queueAction(
				&toolResponseAction{
					request:  &toolCallAction{ToolCallID: approval.ToolCallID, ToolName: approval.Tool.Name, ValidationResult: approval.ValidationResult, Group: approval.Group},
					response: fmt.Sprintf("approval timed out for tool: %s", approval.Tool.Name),
				})
		}
	}
}

func (r *AgentRun) runApprovalAction(act *approvalAction) {
	approval, ok := r.pendingApprovals[act.ApprovalID]
	if !ok {
		r.Logger.Error("invalid approval ID", "approvalID", act.ApprovalID)
		return
	}
	delete(r.pendingApprovals, act.ApprovalID)

	if act.Approved {
		r.queueAction(&toolCallAction{ToolCallID: approval.ToolCallID, ToolName: approval.Tool.Name, ValidationResult: approval.ValidationResult, Group: approval.Group})
		return
	}
	r.queueAction(&toolResponseAction{
		request:  &toolCallAction{ToolCallID: approval.ToolCallID, ToolName: approval.Tool.Name, ValidationResult: approval.ValidationResult, Group: approval.Group},
		response: fmt.Sprintf("approval denied for tool: %s", approval.Tool.Name),
	})
}

func (r *AgentRun) runLLMCallAction(message string, agentTools []AgentTool) {
	// Check limit before making any LLM call
	if r.maxLLMCalls > 0 && r.llmCallCount >= r.maxLLMCalls {
		err := fmt.Errorf("LLM call limit exceeded: %d calls (configured limit: %d)",
			r.llmCallCount, r.maxLLMCalls)
		r.queueAction(&stopAction{Error: err})
		return
	}
	r.llmCallCount++ // Increment counter

	// Clear processed tool call IDs and stream group for this new LLM call
	r.processedToolCallIDs = make(map[string]bool)
	r.currentStreamGroup = nil

	tools := make([]ai.Tool, len(agentTools))
	for i, agentTool := range agentTools {
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
	if r.ContextManager != nil {
		msgs, err = r.ContextManager.BuildPrompt(r, r.currentConversationTurn.getCurrentMessages(), tools)
	} else {
		msgs, err = r.agentContext.BuildPrompt(r, r.currentConversationTurn.getCurrentMessages(), tools)
	}
	if err != nil {
		r.queueAction(&stopAction{Error: err})
		return
	}

	// Chain BeforeCall interceptors
	currentMsgs := msgs
	currentTools := tools
	for _, interceptor := range r.interceptors {
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
		if r.trace != nil {
			r.trace.RecordError(err)
		}
		r.queueAction(&stopAction{Error: err})
		return
	}

	// Chain AfterCall interceptors
	currentResp := respMsg
	for _, interceptor := range r.interceptors {
		currentResp, err = interceptor.AfterCall(r, currentMsgs, currentResp)
		if err != nil {
			r.queueAction(&stopAction{Error: fmt.Errorf("interceptor error: %w", err)})
			return
		}
	}

	r.handleAIMessage(currentResp, false)
}

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
		RunID:            r.id,
		EventID:          eventID,
		AgentName:        r.AgentName(),
		SessionID:        r.sessionID,
		ToolName:         act.ToolName,
		ValidationResult: act.ValidationResult,
		ToolGroup:        act.Group,
	}
	r.queueEvent(toolEvent) // send after adding to the map

	currentValidationResult := act.ValidationResult
	var err error
	for _, interceptor := range r.interceptors {
		currentValidationResult, err = interceptor.BeforeToolCall(r, act.ToolName, act.ToolCallID, currentValidationResult)
		if err != nil {
			errMsg := fmt.Sprintf("interceptor rejected tool call: %v", err)
			r.queueAction(&toolResponseAction{request: act, response: errMsg})
			return
		}
	}

	result, err := tool.call(r, currentValidationResult)
	if err != nil {
		if r.trace != nil {
			r.trace.RecordError(err)
		}
		errMsg := fmt.Sprintf("tool execution error: %v", err)
		r.queueAction(&toolResponseAction{request: act, response: errMsg})
		return
	}

	currentResult := result
	for _, interceptor := range r.interceptors {
		currentResult, err = interceptor.AfterToolCall(r, act.ToolName, act.ToolCallID, currentValidationResult, currentResult)
		if err != nil {
			errMsg := fmt.Sprintf("interceptor error after tool call: %v", err)
			if r.trace != nil {
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
		if r.trace != nil {
			r.trace.RecordError(toolErr)
		}
	}

	r.queueAction(&toolResponseAction{request: act, response: response})
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
				r.currentConversationTurn.addMessage(response)
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
					AIMessage: &chunkMsg,
					Responses: make(map[string]ai.ToolMessage),
				}
			}
			// Process tool calls using the shared group
			r.processToolCallsFromChunk(msg.ToolCalls)
		}
		return
	}

	// this not a chunk, which means the model Call/Stream is complete
	// add to history and fire tool calls
	if len(msg.ToolCalls) == 0 {
		r.currentConversationTurn.addMessage(msg)
		r.currentConversationTurn.Reply = msg
		r.currentConversationTurn.compact()
		r.queueAction(&stopAction{Error: nil})
		return
	}

	r.currentConversationTurn.addMessage(msg)

	// If we have a stream group from chunks, update it with the final message, otherwise create new group
	if r.currentStreamGroup != nil {
		r.currentStreamGroup.AIMessage = &msg
		r.groupToolCalls(msg.ToolCalls, msg, r.currentStreamGroup)
		// Check if all tool calls in the group are now completed (now that we have the final message)
		if len(r.currentStreamGroup.Responses) == len(r.currentStreamGroup.AIMessage.ToolCalls) {
			// add all tool responses and queue their events
			for _, tc := range r.currentStreamGroup.AIMessage.ToolCalls {
				if response, exists := r.currentStreamGroup.Responses[tc.ID]; exists {
					r.currentConversationTurn.addMessage(response)
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
			if r.currentStreamGroup.AIMessage.Content != "" {
				event := &event.ContentEvent{
					RunID:     r.id,
					AgentName: r.AgentName(),
					SessionID: r.sessionID,
					Content:   r.currentStreamGroup.AIMessage.Content,
				}
				r.queueEvent(event)
			}

			r.queueAction(&llmCallAction{Message: r.userMessage})
		}
		r.currentStreamGroup = nil // clear after processing
	} else {
		r.groupToolCalls(msg.ToolCalls, msg, nil)
	}
}

func (r *AgentRun) processToolCall(tc ai.ToolCall, group *ToolCallGroup) bool {
	if r.processedToolCallIDs[tc.ID] {
		return false
	}
	r.processedToolCallIDs[tc.ID] = true

	var args map[string]interface{}
	if err := json.Unmarshal([]byte(tc.Args), &args); err != nil {
		if r.trace != nil {
			r.trace.RecordError(err)
		}
		r.queueAction(&toolResponseAction{
			request: &toolCallAction{
				ToolCallID:       tc.ID,
				ToolName:         tc.Name,
				ValidationResult: event.ValidationResult{Values: args},
				Group:            group,
			},
			response: fmt.Sprintf("invalid tool parameters: %v", err),
		})
		return false
	}

	tool := r.findTool(tc.Name)
	if tool == nil {
		r.queueAction(&toolResponseAction{
			request:  &toolCallAction{ToolName: tc.Name, ValidationResult: event.ValidationResult{Values: args}, Group: group},
			response: fmt.Sprintf("tool not found: %s", tc.Name),
		})
		return false
	}

	values, err := tool.validateInput(r, args)
	if err != nil {
		r.queueAction(&toolResponseAction{
			request:  &toolCallAction{ToolCallID: tc.ID, ToolName: tc.Name, ValidationResult: event.ValidationResult{Values: args}, Group: group},
			response: fmt.Sprintf("invalid tool parameters: %v", err),
		})
		return false
	}

	if tool.RequireApproval {
		approvalID := uuid.New().String()
		r.pendingApprovals[approvalID] = pendingApproval{
			ApprovalID:       approvalID,
			Tool:             tool,
			ToolCallID:       tc.ID,
			ValidationResult: values,
			Group:            group,
			deadline:         time.Now().Add(r.approvalTimeout),
		}
		approvalEvent := &event.ApprovalEvent{
			RunID:            r.id,
			ApprovalID:       approvalID,
			ToolName:         tc.Name,
			ValidationResult: values,
		}
		r.queueEvent(approvalEvent)
		return true
	}

	r.queueAction(&toolCallAction{ToolCallID: tc.ID, ToolName: tc.Name, ValidationResult: values, Group: group})
	return false
}

// processToolCallsFromChunk processes tool calls from a streaming chunk using the shared stream group
func (r *AgentRun) processToolCallsFromChunk(toolCalls []ai.ToolCall) {
	for _, tc := range toolCalls {
		if r.processToolCall(tc, r.currentStreamGroup) {
			return
		}
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
		if r.processToolCall(tc, group) {
			return
		}
	}
}

func (r *AgentRun) queueEvent(event event.Event) {
	// if this is a sub-agent, queue the event to the parent agent
	if r.parentRun != nil {
		r.parentRun.queueEvent(event)
	}
	select {
	case r.eventQueue <- event:
		// queued
	default:
		// queue full, drop or handle overflow
		r.Logger.Error("event queue is full. dropping event", "event", event)
	}
}

func (r *AgentRun) queueAction(action action) {
	select {
	case r.actionQueue <- action:
		// queued
	default:
		// queue full, drop or handle overflow
		r.Logger.Error("action queue is full. dropping action", "action", action)
	}
}
