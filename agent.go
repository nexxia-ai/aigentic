package aigentic

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/nexxia-ai/aigentic/ai"
)

type pendingApproval struct {
	event   *ToolEvent
	created time.Time
}

type toolCallGroup struct {
	aiMessage *ai.AIMessage
	responses map[string]ai.ToolMessage
}

type Agent struct {
	Model   *ai.Model
	Name    string
	ID      string
	Agents  []*Agent
	Session *Session
	Tools   []ai.Tool

	Role              string
	Description       string
	Goal              string
	Instructions      string
	ExpectedOutput    string
	AdditionalContext string
	SuccessCriteria   string

	userMessage string

	Retries             int
	DelayBetweenRetries int
	ExponentialBackoff  bool
	Stream              bool
	Attachments         []Attachment
	History             []*RunResponse
	run                 *RunResponse
	parentAgent         *Agent
	Trace               *Trace

	actionQueue      chan Action
	eventChan        chan Event
	started          bool
	pendingApprovals map[string]*pendingApproval
}

type RunResponse struct {
	Agent            string
	Content          string
	Session          *Session
	Thinking         string
	RedactedThinking string
	ReasoningContent string
	ContentType      string
	CreatedAt        time.Time

	LLMMsg          string
	msgHistory      []ai.Message
	generateCounter int
}

func (a *Agent) Run(message string) error {
	if a.started {
		return errors.New("agent already executed")
	}
	a.started = true
	a.init()
	a.userMessage = message

	go a.startProcessor()
	a.fireLLMCallEvent(message, a.Tools)
	return nil
}

func (a *Agent) Wait(d time.Duration) (string, error) {
	if !a.started {
		return "", errors.New("agent not started")
	}
	content := ""
	for event := range a.Next() {
		switch event := event.(type) {
		case *ContentEvent:
			content += event.Content
			if event.IsFinal {
				return content, nil
			}
		case *ErrorEvent:
			return "", event.Err
		}
	}
	return content, nil
}

func (a *Agent) RunAndWait(message string) (string, error) {
	if err := a.Run(message); err != nil {
		return "", err
	}
	return a.Wait(time.Duration(0))
}

func (a *Agent) stop() {
	// do not close the eventChan if this is a sub-agent
	slog.Debug("stopping agent", "agent", a.Name)
	if a.parentAgent == nil {
		close(a.eventChan)
	}
	close(a.actionQueue)
}

func (a *Agent) Next() <-chan Event {
	return a.eventChan
}

func (a *Agent) Approve(eventID string) {
	a.queueAction(&approvalAction{
		EventID:  eventID,
		Approved: true,
	})
}

func (a *Agent) queueAction(action Action) {
	slog.Debug("queueing action", "actionType", fmt.Sprintf("%T", action), "agent", a.Name, "len", len(a.actionQueue))
	select {
	case a.actionQueue <- action:
		// queued
	default:
		// queue full, drop or handle overflow
		slog.Error("action queue is full. dropping action", "action", action)
	}
}

func (a *Agent) queueEvent(event Event) {
	slog.Debug("queueing event", "eventType", fmt.Sprintf("%T", event), "agent", a.Name, "len", len(a.eventChan))
	select {
	case a.eventChan <- event:
		// queued
	default:
		// queue full, drop or handle overflow
		slog.Error("event queue is full. dropping event", "event", event)
	}
}

func (a *Agent) fireErrorEvent(err error) {
	event := &ErrorEvent{
		EventID:   uuid.New().String(),
		AgentName: a.Name,
		SessionID: a.Session.ID,
		Err:       err,
	}
	a.queueEvent(event)
}

func (a *Agent) fireContentEvent(content string, isFinal bool) {
	event := &ContentEvent{
		EventID:   uuid.New().String(),
		AgentName: a.Name,
		SessionID: a.Session.ID,
		Content:   content,
		IsFinal:   isFinal,
	}
	a.eventChan <- event
	if isFinal {
		a.queueAction(&stopAction{EventID: event.EventID})
	}
}

func (a *Agent) findTool(tcName string) *ai.Tool {
	for i := range a.Tools {
		if a.Tools[i].Name == tcName {
			return &a.Tools[i]
		}
	}
	return nil
}

func (a *Agent) fireToolCallEvent(tcName string, tcArgs map[string]interface{}, toolCallID string, group *toolCallGroup) string {
	tool := a.findTool(tcName)
	if tool == nil {
		slog.Error("invalid tool", "tool", tcName)
		return ""
	}
	eventID := uuid.New().String()
	toolEvent := &ToolEvent{
		EventID:         eventID,
		AgentName:       a.Name,
		SessionID:       a.Session.ID,
		ToolName:        tcName,
		ToolArgs:        tcArgs,
		RequireApproval: tool.RequireApproval,
		ToolGroup:       group,
	}
	if tool.RequireApproval {
		a.pendingApprovals[eventID] = &pendingApproval{event: toolEvent}
	}
	a.queueEvent(toolEvent) // send after adding to the map
	if !tool.RequireApproval {
		a.queueAction(&toolExecutionAction{EventID: eventID, ToolCallID: toolCallID, ToolName: tcName, ToolArgs: tcArgs, Group: group})
	}
	return eventID
}

func (a *Agent) fireThinkingEvent(thought string) {
	event := &ThinkingEvent{
		EventID:   uuid.New().String(),
		AgentName: a.Name,
		SessionID: a.Session.ID,
		Thought:   thought,
	}
	a.queueEvent(event)
}

func (a *Agent) fireToolResponseEvent(action *toolExecutionAction, content string) {
	event := &ToolResponseEvent{
		EventID:    uuid.New().String(),
		AgentName:  a.Name,
		SessionID:  a.Session.ID,
		ToolCallID: action.ToolCallID,
		ToolName:   action.ToolName,
		Content:    content,
	}

	a.queueEvent(event)

	// Add response to the group
	toolMsg := ai.ToolMessage{
		Role:       ai.ToolRole,
		Content:    content,
		ToolCallID: action.ToolCallID,
	}
	action.Group.responses[action.ToolCallID] = toolMsg

	// Check if all tool calls in this group are completed
	if len(action.Group.responses) == len(action.Group.aiMessage.ToolCalls) {
		// Then add the AI message to history
		a.run.msgHistory = append(a.run.msgHistory, *action.Group.aiMessage)

		// Add all tool responses last
		for _, tc := range action.Group.aiMessage.ToolCalls {
			if response, exists := action.Group.responses[tc.ID]; exists {
				a.run.msgHistory = append(a.run.msgHistory, response)
			}
		}

		// Send any content from the AI message
		if action.Group.aiMessage.Content != "" {
			a.fireContentEvent(action.Group.aiMessage.Content, true)
		}

		// Trigger the next LLM call
		a.fireLLMCallEvent(a.userMessage, a.Tools)
	}
}

func (a *Agent) fireLLMCallEvent(msg string, tools []ai.Tool) {
	event := &LLMCallEvent{
		EventID:   uuid.New().String(),
		AgentName: a.Name,
		SessionID: a.Session.ID,
		Message:   msg,
		Tools:     tools,
	}
	a.queueEvent(event)
	a.queueAction(&runAction{Message: msg})
}
func (a *Agent) startProcessor() {
	defer a.stop()

	if a.Session == nil {
		a.Session = NewSession()
	}
	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	if a.Model == nil {
		a.Model = ai.NewOpenAIModel("gpt-4o-mini", "")
	}
	if a.run == nil {
		a.run = &RunResponse{
			Agent:   a.Name,
			Session: a.Session,
		}
	}
	for action := range a.actionQueue {
		switch act := action.(type) {
		case *stopAction:
			slog.Debug("received stop action", "agent", a.Name, "len", len(a.actionQueue))
			return

		case *runAction:
			slog.Debug("running agent", "agent", a.Name)
			userMsgs := a.createUserMsg(act.Message)
			sysMsg := a.createSystemMessage("")
			msgs := []ai.Message{
				ai.SystemMessage{Role: ai.SystemRole, Content: sysMsg},
			}
			msgs = append(msgs, userMsgs...)
			msgs = append(msgs, a.run.msgHistory...)

			if a.Trace != nil {
				a.Trace.LLMCall(a.Model.ModelName, a.Name, msgs)
			}

			var respMsg ai.AIMessage
			var err error
			respMsg, err = a.Model.Call(a.Session.Context, msgs, a.Tools)
			if err != nil {
				if a.Trace != nil {
					a.Trace.RecordError(err)
				}
				a.fireErrorEvent(err)
				continue
			}
			if a.Trace != nil {
				a.Trace.LLMAIResponse(a.Name, respMsg.Content, respMsg.ToolCalls, respMsg.Think)
			}
			if respMsg.Think != "" {
				a.fireThinkingEvent(respMsg.Think)
			}
			if len(respMsg.ToolCalls) > 0 {
				// Create a tool call group to coordinate all tool calls
				group := &toolCallGroup{
					aiMessage: &respMsg,
					responses: make(map[string]ai.ToolMessage),
				}

				// Process each tool call individually, passing the group
				for _, tc := range respMsg.ToolCalls {
					var args map[string]interface{}
					if err := json.Unmarshal([]byte(tc.Args), &args); err != nil {
						if a.Trace != nil {
							a.Trace.RecordError(err)
						}
						a.fireErrorEvent(fmt.Errorf("invalid tool args: %v", err))
						continue
					}
					a.fireToolCallEvent(tc.Name, args, tc.ID, group)
				}
			} else {
				// No tool calls, safe to add AI message to history immediately
				a.run.msgHistory = append(a.run.msgHistory, respMsg)
				if respMsg.Content != "" {
					a.fireContentEvent(respMsg.Content, true)
				}
			}
		case *approvalAction:
			if pending, ok := a.pendingApprovals[act.EventID]; ok {
				delete(a.pendingApprovals, act.EventID)
				a.actionQueue <- &toolExecutionAction{EventID: act.EventID, ToolName: pending.event.ToolName, ToolArgs: pending.event.ToolArgs, Group: pending.event.ToolGroup}
			}
		case *toolExecutionAction:
			slog.Debug("running tool", "tool", act.ToolName)
			tool := a.findTool(act.ToolName)
			if tool == nil {
				a.fireErrorEvent(fmt.Errorf("tool not found: %s", act.ToolName))
				continue
			}
			result, err := tool.Call(act.ToolArgs)
			if err != nil {
				if a.Trace != nil {
					a.Trace.RecordError(err)
				}
				a.fireErrorEvent(fmt.Errorf("tool execution error: %v", err))
				continue
			}
			content := ""
			for _, c := range result.Content {
				if s, ok := c.Content.(string); ok {
					content += s
				}
			}

			if a.Trace != nil {
				a.Trace.LLMToolResponse(a.Name, &ai.ToolCall{
					ID:   act.ToolCallID,
					Type: "function",
					Name: act.ToolName,
					Args: "",
				}, content)
			}

			a.fireToolResponseEvent(act, content)

		case *cancelAction:
			// no-op for now
		}
	}
}

// Attachment represents a file attachment for LLM requests
type Attachment struct {
	Type     string // "image", "audio", "video", "document", etc.
	Content  []byte // Base64 encoded content
	MimeType string // MIME type of the file
	Name     string // Original filename
}

func (a *Agent) init() error {
	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	if a.Model == nil {
		a.Model = ai.NewOpenAIModel("gpt-4o-mini", "")
	}
	if a.Session == nil {
		a.Session = NewSession()
	}
	a.actionQueue = make(chan Action, 100)

	if a.parentAgent == nil {
		a.eventChan = make(chan Event, 100)
	} else {
		// sub-agent uses the parent agent's eventChan
		a.eventChan = a.parentAgent.eventChan
	}
	a.pendingApprovals = make(map[string]*pendingApproval)
	a.run = &RunResponse{
		Agent:   a.Name,
		Session: a.Session,
	}
	if a.Trace == nil && a.Session.Trace != nil {
		a.Trace = a.Session.Trace
	}
	for _, aa := range a.Agents {
		aa.Session = a.Session
		aa.parentAgent = a
		aa.Trace = a.Trace
		// Create SimpleTool adapter for sub-agent
		agentTool := ai.Tool{
			Name:            aa.Name,
			Description:     aa.Description,
			RequireApproval: false,
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
			Execute: func(args map[string]interface{}) (*ai.ToolResult, error) {
				input := args["input"].(string)
				content, err := aa.RunAndWait(input)
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
		a.Tools = append(a.Tools, agentTool)
	}
	return nil
}

func (a *Agent) createSystemMessage(think string) string {
	sysMsg := a.Description
	if a.Instructions != "" {
		sysMsg += "\n <instructions>\n" +
			a.Instructions +
			"\nAnalyse the entire history message history before you decide the next step to prevent executing the same calls." +
			"\n</instructions>\n"
	}

	// sysMsg += `
	// <scratchpad>
	// You have access to a scratch pad to plan your next step.
	// Use the scratch pad to store your plan of action. For Example:
	//   1. I will first perform a search for the information I need.
	//   2. If the information is not found, then I will call the next agent.
	//   3. I will analyse the response and respond to the user.

	// To update the scratch pad, include your notes in your response between <scratchpad> your notes </scratchpad>.
	// Anything you add to the scratch pad will be sent back to you on the next iteration.

	// Here is the current scratch pad:
	// </scratchpad>
	// `

	if len(a.Tools) > 0 {
		sysMsg += "\n<tools>\nYou have access to the following tools:\n"
		for _, tool := range a.Tools {
			sysMsg += fmt.Sprintf("<tool>\n%s\n%s\n</tool>\n", tool.Name, tool.Description)
		}
		sysMsg += "\n</tools>\n"
	}

	// if think != "" {
	// 	sysMsg += "\n<think>\n" + think + "\n</think>\n"
	// }

	return sysMsg
}

// createUserMsg returns a list of Messages, with each attachment as a separate Resource message
func (a *Agent) createUserMsg(message string) []ai.Message {
	var messages []ai.Message

	// Add the main user message if there's content
	if message != "" {
		userMsg := ai.UserMessage{Role: ai.UserRole, Content: message}
		messages = append(messages, userMsg)
	}

	// Add each attachment as a separate message with the file content
	for _, attachment := range a.Attachments {
		attachmentMsg := ai.ResourceMessage{
			Role: ai.UserRole,
			URI:  "",
			Name: attachment.Name,
			Body: attachment.Content,
			Type: attachment.Type,
		}
		messages = append(messages, attachmentMsg)
	}

	return messages
}
