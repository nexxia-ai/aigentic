package aigentic

import (
	"errors"
	"fmt"
	"time"

	"encoding/json"

	"github.com/google/uuid"
	"github.com/nexxia-ai/aigentic/ai"
)

// Attachment represents a file attachment for LLM requests
type Attachment struct {
	Type     string // "image", "audio", "video", "document", etc.
	Content  []byte // Base64 encoded content
	MimeType string // MIME type of the file
	Name     string // Original filename
}

// Agent represents an autonomous agent that can perform tasks and interact with tools
type Agent struct {
	// Core attributes
	Model   *ai.Model
	Name    string
	ID      string
	Agents  []*Agent
	Session *Session
	el      *EventLoop
	Tools   []ai.Tool

	// Settings for building default system message
	Role              string
	Description       string
	Goal              string
	Instructions      string
	ExpectedOutput    string
	AdditionalContext string
	SuccessCriteria   string

	// Agent Response Settings
	Retries             int
	DelayBetweenRetries int
	ExponentialBackoff  bool

	// Agent Streaming
	Stream bool

	Attachments []Attachment // Slice of attachments to include in LLM requests

	// Run history
	History []*RunResponse
	run     *RunResponse

	parentAgent *Agent

	// Tracing
	Trace *Trace
}

// RunResponse represents the response from a run
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

func (a *Agent) init(message string) error {
	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	if a.Model == nil {
		a.Model = ai.NewOpenAIModel("gpt-4o-mini", "")
	}
	if a.Session == nil {
		a.Session = NewSession()
	}
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
			Name:        aa.Name,
			Description: aa.Description,
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
				response, err := aa.Run(input)
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
						Content: response.Content,
					}},
					Error: false,
				}, nil
			},
		}
		a.Tools = append(a.Tools, agentTool)
	}
	return nil
}

// Run executes the agent with the given message
func (a *Agent) Run(message string) (RunResponse, error) {
	var response RunResponse
	var err error
	var el *EventLoop
	if el = a.Start(message); el == nil {
		return RunResponse{}, errors.New("failed to start session")
	}
	for ev := range el.Next() {
		if response, err = ev.Execute(); err != nil {
			return RunResponse{}, err
		}
	}
	return response, nil
}

func (a *Agent) Start(message string) *EventLoop {
	if err := a.init(message); err != nil {
		return nil
	}
	a.el = &EventLoop{Session: a.Session, Agent: a, next: make(chan *Event, 10)}
	a.send(message)
	return a.el
}

func (a *Agent) send(message string) {
	a.el.next <- &Event{Agent: a, Message: message}
}

func (a *Agent) generate(message string) (string, error) {
	var content string
	var err error

	if a.run.generateCounter > 64 {
		return "", errors.New("too many repeats")
	}
	a.run.generateCounter++

	userMsgs := a.createUserMsg2(message)
	sysMsg := a.createSystemMessage("")

	msgs := []ai.Message{
		ai.SystemMessage{Role: ai.SystemRole, Content: sysMsg},
	}
	msgs = append(msgs, userMsgs...)
	msgs = append(msgs, a.run.msgHistory...)
	if a.Trace != nil {
		a.Trace.LLMCall(a.Model.ModelName, a.Name, msgs)
	}

	respMsg, err := a.Model.Call(a.Session.Context, msgs, a.Tools)
	if err != nil {
		if a.Trace != nil {
			a.Trace.RecordError(err)
		}
		return "", err
	}
	if a.Trace != nil {
		a.Trace.LLMAIResponse(a.Name, respMsg.Content, respMsg.ToolCalls, respMsg.Think)
	}

	// Extract content and tool calls from the returned ai.Message
	var toolCalls []ai.ToolCall
	content = respMsg.Content
	toolCalls = respMsg.ToolCalls

	a.run.msgHistory = append(a.run.msgHistory, respMsg)

	// Execute tool calls if any
	n := a.runTools(toolCalls)
	if n > 0 {
		a.send(message) // send the same user message again
		return content, nil
	}

	close(a.el.next)
	return content, nil
	// toolMessage := ai.ToolMessage{
	// 	Role:       ai.ToolRole,
	// 	Content:    content,
	// 	ToolCallID: a.parentToolID,
	// }
	// a.parentAgent.run.msgHistory = append(a.parentAgent.run.msgHistory, toolMessage)
	// a.Session.send(a.parentAgent, content)

}

func (a *Agent) runTools(toolCalls []ai.ToolCall) int {
	if len(toolCalls) == 0 {
		return 0
	}

	n := 0
	for _, toolCall := range toolCalls {
		for _, tool := range a.Tools {
			if tool.Name != toolCall.Name {
				continue
			}

			var content string
			var args map[string]interface{}
			if err := json.Unmarshal([]byte(toolCall.Args), &args); err != nil {
				content = fmt.Sprintf("error: invalid JSON args: %v", err)
			} else {
				result, err := tool.Call(args)
				if err != nil {
					if a.Session.Trace != nil {
						a.Session.Trace.RecordError(err)
					}
					content = fmt.Sprintf("error: %v", err)
				} else {
					for _, c := range result.Content {
						switch c.Type {
						case "text":
							content += c.Content.(string)
						case "image":
							content += c.Content.(string)
						default:
							content += fmt.Sprintf("[%s content]", c.Type)
						}
						if a.Session.Trace != nil {
							aiToolCall := &ai.ToolCall{
								ID:     toolCall.ID,
								Type:   toolCall.Type,
								Name:   tool.Name,
								Args:   toolCall.Args,
								Result: "",
							}
							a.Session.Trace.LLMToolResponse(a.Name, aiToolCall, content)
						}
					}
				}
			}

			// Add tool response to message history
			toolMessage := ai.ToolMessage{
				Role:       ai.ToolRole,
				Content:    content,
				ToolCallID: toolCall.ID,
			}
			a.run.msgHistory = append(a.run.msgHistory, toolMessage)
			n++
			break
		}
	}
	return n
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

func (a *Agent) createUserMsg(message string) string {
	// TODO: make this work with multiple parts in the same message
	// Including attachments in the user message for now
	if len(a.Attachments) > 0 {
		message += "\n <attachments>\n"
		for _, attachment := range a.Attachments {
			message += fmt.Sprintf("<file>- Name %s: MimeType:%s\n", attachment.Name, attachment.MimeType)
			message += fmt.Sprintf("- %s\n</file>\n", attachment.Content)
		}
		message += "\n</attachments>\n"
	}
	return message
}

// createUserMsg2 returns a list of Messages, with each attachment as a separate Resource message
func (a *Agent) createUserMsg2(message string) []ai.Message {
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
