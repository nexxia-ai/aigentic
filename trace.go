package aigentic

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/nexxia-ai/aigentic/ai"
)

// Trace stores the execution trace of an LLM.
type Trace struct {
	sync.Mutex
	SessionID string    // Unique session ID for the entire interaction
	StartTime time.Time // Start time of the trace
	EndTime   time.Time // End time of the trace
	filename  string    // Path to the trace file
	file      *os.File  // File to write traces to
}

// NewTrace creates a new Trace instance.
func NewTrace() *Trace {
	directory := "traces"

	// Create the trace directory if it doesn't exist
	if _, err := os.Stat(directory); os.IsNotExist(err) {
		if err := os.MkdirAll(directory, 0755); err != nil {
			return nil
		}
	}

	sessionID := time.Now().Format("20060102150405") // Unique session ID
	filename := filepath.Join(directory, fmt.Sprintf("trace-%s.txt", sessionID))

	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil
	}

	t := &Trace{
		SessionID: sessionID,
		StartTime: time.Now(),
		filename:  filename,
		file:      file,
	}

	return t
}

// LLMCall records the initial interaction with the LLM (model and messages).
func (t *Trace) LLMCall(modelName, agentName string, messages []ai.Message) error {
	if t == nil {
		return nil
	}

	t.Lock()
	defer t.Unlock()

	fmt.Fprintf(t.file, "\n====> [%s] Start %s (%s)\n", time.Now().Format("15:04:05"), agentName, modelName)

	for _, message := range messages {
		role, _ := message.Value()
		fmt.Fprintf(t.file, "⬆️  %s:\n", role)

		// Handle each message type specifically
		switch msg := message.(type) {
		case ai.UserMessage:
			t.logMessageContent("content", msg.Content)
		case ai.SystemMessage:
			t.logMessageContent("content", msg.Content)
		case ai.AIMessage:
			t.logMessageContent("content", msg.Content)
			if msg.Think != "" {
				t.logMessageContent("thinking", msg.Think)
			}
		case ai.ToolMessage:
			t.logMessageContent("content", msg.Content)
			fmt.Fprintf(t.file, " tool_call_id: %s\n", msg.ToolCallID)
		case ai.ResourceMessage:
			// Determine if this is a file ID reference or has content
			var isFileID bool
			var contentLen int
			var contentPreview string

			if body, ok := msg.Body.([]byte); ok && body != nil {
				// Has actual content
				isFileID = false
				contentLen = len(body)
				if contentLen > 0 {
					// Show first 64 characters of content
					previewLen := 64
					if contentLen < previewLen {
						previewLen = contentLen
					}
					contentPreview = string(body[:previewLen])
				}
			} else {
				// Likely a file ID reference
				isFileID = true
				contentLen = len(msg.Name)
			}

			// Log the resource type and basic info
			if isFileID {
				fmt.Fprintf(t.file, " resource: %s (file ID reference)\n", msg.Name)
			} else {
				fmt.Fprintf(t.file, " resource: %s (content length: %d)\n", msg.Name, contentLen)
			}

			// Log additional metadata
			if msg.URI != "" {
				fmt.Fprintf(t.file, " uri: %s\n", msg.URI)
			}
			if msg.MIMEType != "" {
				fmt.Fprintf(t.file, " mime_type: %s\n", msg.MIMEType)
			}
			if msg.Description != "" {
				fmt.Fprintf(t.file, " description: %s\n", msg.Description)
			}

			// Log content preview if available
			if contentPreview != "" {
				t.logMessageContent("content_preview", contentPreview)
			}
		default:
			// Fallback for unknown message types
			_, content := message.Value()
			t.logMessageContent("content", content)
		}
	}
	return nil
}

// logMessageContent is a helper method to format and log message content
func (t *Trace) logMessageContent(contentType, content string) {
	if content == "" {
		return
	}

	fmt.Fprintf(t.file, " %s:\n", contentType)
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if line != "" {
			fmt.Fprintf(t.file, "   %s\n", line)
		}
	}
}

// FinishLLMInteraction adds a closing line to mark the end of an LLM interaction
func (t *Trace) FinishLLMInteraction(modelName, agentName string) {
	if t == nil {
		return
	}

	t.Lock()
	defer t.Unlock()

	fmt.Fprintf(t.file, "==== [%s] End %s\n\n", time.Now().Format("15:04:05"), agentName)
}

// LLMAIResponse records the LLM's response, any tool calls made during the response, and any thinking process.
func (t *Trace) LLMAIResponse(agentName, response string, toolCalls []ai.ToolCall, thinkPart string) error {
	if t == nil {
		return nil
	}

	t.Lock()
	defer t.Unlock()

	if thinkPart != "" {
		fmt.Fprintf(t.file, "⬇️  %s thinking:\n%s\n\n", agentName, thinkPart)
	}

	if response != "" {
		fmt.Fprintf(t.file, "⬇️  %s response:\n%s\n", agentName, response)
	}

	if len(toolCalls) > 0 {
		fmt.Fprintf(t.file, "⬇️ ️  %s tool request:\n", agentName)
		for _, toolCall := range toolCalls {
			fmt.Fprintf(t.file, "   • %s(%s)\n",
				toolCall.Name,
				toolCall.Args)
		}
	}

	return nil
}

// LLMToolResponse records a single tool call response.
func (t *Trace) LLMToolResponse(agentName string, toolCall *ai.ToolCall, content string) error {
	if t == nil {
		return nil
	}

	t.Lock()
	defer t.Unlock()

	fmt.Fprintf(t.file, "🛠️️  %s tool response:\n", agentName)
	fmt.Fprintf(t.file, "   • %s(%s)\n",
		toolCall.Name,
		toolCall.Args)

	// Format the response content
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if line != "" {
			fmt.Fprintf(t.file, "     %s\n", line)
		}
	}
	return nil
}

// RecordError records an error that occurred during the interaction.
func (t *Trace) RecordError(err error) error {
	if t == nil {
		return nil
	}

	t.Lock()
	defer t.Unlock()

	fmt.Fprintf(t.file, "❌ Error: %v\n", err)
	return nil
}

// End ends the trace and saves the trace information to a file.
func (t *Trace) End() error {
	if t == nil {
		return nil
	}
	t.Lock()
	defer t.Unlock()
	t.EndTime = time.Now()

	_, err := fmt.Fprintf(t.file, "End Time: %s\n", t.EndTime.Format(time.RFC3339))
	if err != nil {
		return err
	}

	err = t.file.Close()
	if err != nil {
		return err
	}

	log.Printf("Trace saved to %s\n", t.filename)
	return nil
}
