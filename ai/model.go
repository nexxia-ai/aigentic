package ai

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

var (
	ErrToolExceeded = errors.New("tool loop limit exceeded")
)

// RecordedResponse represents a recorded AI response with error information
type RecordedResponse struct {
	AIMessage AIMessage `json:"ai_message"`
	Error     string    `json:"error,omitempty"` // Empty string if no error
	Timestamp string    `json:"timestamp"`
}

type StatusError struct {
	StatusCode   int
	Status       string
	ErrorMessage string
}

func (e StatusError) Error() string {
	return fmt.Sprintf("status: %s, code: %d, error: %s", e.Status, e.StatusCode, e.ErrorMessage)
}

// Model represents a generic model container that uses function variables for provider-specific logic
type Model struct {
	ModelName string
	APIKey    string
	BaseURL   string
	client    *http.Client

	// callFunc is the implementation for each provider
	callFunc func(ctx context.Context, model *Model, messages []Message, tools []Tool) (AIMessage, error)

	// Options pointer variables - use nil to represent option not set
	Temperature      *float64
	MaxTokens        *int
	TopP             *float64
	FrequencyPenalty *float64
	PresencePenalty  *float64
	StopSequences    *[]string
	Stream           *bool
	ContextSize      *int
	Parameters       map[string]interface{} // additional non-standard parameters for the model

	// Recording functionality
	RecordFilename string // If set, record responses to this file
}

// Call makes a single call to the model. It does not execute any tool calls, but return the requested ToolCalls.
// This is useful to implemnent your own tool execution loop.
func (m *Model) Call(ctx context.Context, messages []Message, tools []Tool) (AIMessage, error) {
	// Make the actual provider call
	response, err := m.callFunc(ctx, m, messages, tools)

	// If recording is enabled, record the response
	if m.RecordFilename != "" {
		m.recordAIMessage(response, err)
	}

	return response, err
}

// Generate executes a complete conversation with tool execution loop
func (model *Model) Generate(ctx context.Context, messages []Message, tools []Tool) (AIMessage, error) {
	maxIterations := 32 // Prevent infinite loops
	iteration := 0

	for iteration < maxIterations {
		iteration++

		// Generate response from model
		respMsg, err := model.Call(ctx, messages, tools)
		if err != nil {
			return AIMessage{}, err
		}

		messages = append(messages, respMsg)

		// If no tool calls, return the final response
		if len(respMsg.ToolCalls) == 0 {
			return respMsg, nil
		}

		// Execute tool calls
		for _, toolCall := range respMsg.ToolCalls {
			for _, tool := range tools {
				if tool.Name != toolCall.Name {
					continue
				}

				var args map[string]interface{}
				if err := json.Unmarshal([]byte(toolCall.Args), &args); err != nil {
					// Add error message to conversation
					toolMessage := ToolMessage{
						Role:       ToolRole,
						Content:    fmt.Sprintf("error: invalid JSON args: %v", err),
						ToolCallID: toolCall.ID,
					}
					messages = append(messages, toolMessage)
					continue
				}

				// Execute the tool
				result, err := tool.Call(args)
				var content string
				if err != nil {
					content = fmt.Sprintf("error: %v", err)
				} else {
					for _, c := range result.Content {
						switch c.Type {
						case "text":
							content += c.Content.(string)
						case "image":
							content += "[image content]"
						default:
							content += fmt.Sprintf("[%s content]", c.Type)
						}
					}
				}

				// Add tool response to conversation
				toolMessage := ToolMessage{
					Role:       ToolRole,
					Content:    content,
					ToolCallID: toolCall.ID,
				}
				messages = append(messages, toolMessage)
				break
			}
		}
	}

	return AIMessage{}, ErrToolExceeded
}

// WithContextSize sets the context size for the model and returns the model for chaining
func (m *Model) WithContextSize(contextSize int) *Model {
	m.ContextSize = &contextSize
	return m
}

// WithTemperature sets the temperature for the model and returns the model for chaining
func (m *Model) WithTemperature(temperature float64) *Model {
	m.Temperature = &temperature
	return m
}

// WithMaxTokens sets the maximum tokens for the model and returns the model for chaining
func (m *Model) WithMaxTokens(maxTokens int) *Model {
	m.MaxTokens = &maxTokens
	return m
}

// WithTopP sets the top_p parameter for the model and returns the model for chaining
func (m *Model) WithTopP(topP float64) *Model {
	m.TopP = &topP
	return m
}

// WithFrequencyPenalty sets the frequency penalty for the model and returns the model for chaining
func (m *Model) WithFrequencyPenalty(penalty float64) *Model {
	m.FrequencyPenalty = &penalty
	return m
}

// WithPresencePenalty sets the presence penalty for the model and returns the model for chaining
func (m *Model) WithPresencePenalty(penalty float64) *Model {
	m.PresencePenalty = &penalty
	return m
}

// WithStopSequences sets the stop sequences for the model and returns the model for chaining
func (m *Model) WithStopSequences(sequences []string) *Model {
	m.StopSequences = &sequences
	return m
}

// WithStream sets the stream option for the model and returns the model for chaining
func (m *Model) WithStream(stream bool) *Model {
	m.Stream = &stream
	return m
}

func (m *Model) WithParameter(name string, value interface{}) *Model {
	m.Parameters[name] = value
	return m
}

// SetGenerateFunc sets the generate function for the model. This is used to override the default generate function to use a non standard provider.
// Not required most of the time unless you are using a non standard provider.
func (m *Model) SetGenerateFunc(generateFunc func(ctx context.Context, model *Model, messages []Message, tools []Tool) (AIMessage, error)) error {
	m.callFunc = generateFunc
	return nil
}

// ExtractThinkTags extracts <think>...</think> tags from the content and returns both the cleaned content and the think part
func ExtractThinkTags(content string) (cleanedContent string, thinkPart string) {
	// Find the start and end positions of think tags
	startTag := "<think>"
	endTag := "</think>"

	start := strings.Index(content, startTag)
	if start == -1 {
		return content, "" // No think tags found
	}

	end := strings.Index(content[start:], endTag)
	if end == -1 {
		return content, "" // No closing tag found
	}
	end += start + len(endTag)

	// Extract the think part (without the tags)
	thinkPart = content[start+len(startTag) : end-len(endTag)]

	// Remove the think tags and their content from the main content
	cleanedContent = content[:start] + content[end:]

	return strings.TrimSpace(cleanedContent), strings.TrimSpace(thinkPart)
}

// recordAIMessage records an AI response to the specified file
func (m *Model) recordAIMessage(response AIMessage, err error) {
	// Create the recorded response structure
	recorded := RecordedResponse{
		AIMessage: response,
		Timestamp: time.Now().Format(time.RFC3339),
	}

	// Set error message if there was an error
	if err != nil {
		recorded.Error = err.Error()
	}

	// Marshal to JSON (compact format for JSONL)
	jsonData, marshalErr := json.Marshal(recorded)
	if marshalErr != nil {
		return // Silently fail if we can't marshal
	}

	// Open file for appending (create if doesn't exist)
	file, openErr := os.OpenFile(m.RecordFilename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if openErr != nil {
		return // Silently fail if we can't open file
	}
	defer file.Close()

	// Write the JSON record followed by a newline
	file.Write(jsonData)
	file.WriteString("\n")
}

// LoadDummyRecords loads recorded responses from a file for use in dummy models
func LoadDummyRecords(filename string) ([]RecordedResponse, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open recorded responses file: %w", err)
	}
	defer file.Close()

	var records []RecordedResponse
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var record RecordedResponse
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			return nil, fmt.Errorf("failed to unmarshal recorded response: %w", err)
		}

		records = append(records, record)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading recorded responses file: %w", err)
	}

	return records, nil
}
