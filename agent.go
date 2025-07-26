package aigentic

import (
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

	Role            string
	Description     string
	Goal            string
	Instructions    string
	ExpectedOutput  string
	SuccessCriteria string

	Retries             int
	DelayBetweenRetries int
	ExponentialBackoff  bool
	Stream              bool
	Attachments         []Attachment
	parentAgent         *Agent
	Trace               *Trace
	LogLevel            slog.Level
}

// Attachment represents a file attachment for LLM requests
type Attachment struct {
	Type     string // "image", "audio", "video", "document", etc.
	Content  []byte // Base64 encoded content
	MimeType string // MIME type of the file
	Name     string // Original filename
}

func (a *Agent) Run(message string) (*AgentRun, error) {
	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	if a.Name == "" {
		a.Name = a.ID
	}
	run := newAgentRun(a, message)
	run.start()
	return run, nil
}

func (a *Agent) RunAndWait(message string) (string, error) {
	run, err := a.Run(message)
	if err != nil {
		return "", err
	}
	return run.Wait(0)
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
