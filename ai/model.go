package ai

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"os"
	"strings"
	"time"
)

var (
	ErrToolExceeded = errors.New("tool loop limit exceeded")
	ErrTemporary    = errors.New("temporary error - retry recommended")
)

// Retry configuration variables - can be modified for testing
var (
	defaultMaxRetries   = 10
	defaultBaseDelay    = 3 * time.Second
	defaultMaxDelay     = 30 * time.Second
	defaultJitterFactor = 0.1
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

// Model represents a generic model container that uses closures for provider-specific logic
type Model struct {
	ModelName string
	APIKey    string
	BaseURL   string

	// callFunc is the implementation for each provider
	callFunc          func(ctx context.Context, model *Model, messages []Message, tools []Tool) (AIMessage, error)
	callStreamingFunc func(ctx context.Context, model *Model, messages []Message, tools []Tool, chunkFunction func(AIMessage) error) (AIMessage, error)

	// Options pointer variables - use nil to represent option not set
	Temperature      *float64
	MaxTokens        *int
	TopP             *float64
	FrequencyPenalty *float64
	PresencePenalty  *float64
	StopSequences    *[]string
	ContextSize      *int
	Parameters       map[string]interface{} // additional non-standard parameters for the model

	// Recording functionality
	RecordFilename string // If set, record responses to this file

	// Retry configuration
	MaxRetries *int // Maximum number of retry attempts (nil = use default, 1 = no retry, default = 10)
}

// calculateBackoffDelay calculates the delay for the next retry with exponential backoff and jitter
func (m *Model) calculateBackoffDelay(attempt int) time.Duration {
	// Exponential backoff: defaultBaseDelay * 2^attempt
	delay := float64(defaultBaseDelay) * math.Pow(2, float64(attempt))

	// Cap the delay at defaultMaxDelay
	if delay > float64(defaultMaxDelay) {
		delay = float64(defaultMaxDelay)
	}

	// Add jitter to prevent thundering herd
	jitter := delay * defaultJitterFactor * rand.Float64()
	delay += jitter

	return time.Duration(delay)
}

// callWithRetry handles retry logic for non-streaming calls
func (m *Model) callWithRetry(ctx context.Context, messages []Message, tools []Tool) (AIMessage, error) {
	var lastErr error

	maxAttempts := defaultMaxRetries
	if m.MaxRetries != nil {
		maxAttempts = *m.MaxRetries
	}

	if maxAttempts == 0 {
		maxAttempts = 1
	}

	for attempt := 0; attempt < maxAttempts; attempt++ {
		response, err := m.callFunc(ctx, m, messages, tools)
		if err == nil {
			// Success
			if m.RecordFilename != "" {
				m.recordAIMessage(response, err)
			}
			return response, nil
		}

		lastErr = err

		// Check if this is a temporary error
		if err != ErrTemporary {
			// Non-retryable error - return immediately
			if m.RecordFilename != "" {
				m.recordAIMessage(response, err)
			}
			return response, err
		}

		if attempt == maxAttempts-1 {
			break
		}

		// Calculate delay and wait
		delay := m.calculateBackoffDelay(attempt)
		select {
		case <-ctx.Done():
			return AIMessage{}, ctx.Err()
		case <-time.After(delay):
			// Continue to next attempt
		}
	}

	slog.Error("all retries exhausted", "error", lastErr)
	return AIMessage{}, lastErr
}

// Call makes a single call to the model. It does not execute any tool calls, but return the requested ToolCalls.
// This is useful to implemnent your own tool execution loop.
func (m *Model) Call(ctx context.Context, messages []Message, tools []Tool) (AIMessage, error) {
	return m.callWithRetry(ctx, messages, tools)
}

// streamWithRetry handles retry logic for streaming calls
func (m *Model) streamWithRetry(ctx context.Context, messages []Message, tools []Tool, chunkFunction func(AIMessage) error) (AIMessage, error) {
	var lastErr error

	maxAttempts := defaultMaxRetries
	if m.MaxRetries != nil {
		maxAttempts = *m.MaxRetries
	}

	if maxAttempts == 0 {
		maxAttempts = 1
	}

	for attempt := 0; attempt < maxAttempts; attempt++ {
		response, err := m.callStreamingFunc(ctx, m, messages, tools, chunkFunction)
		if err == nil {
			// Success
			return response, nil
		}

		lastErr = err

		// Check if this is a temporary error
		if err != ErrTemporary {
			// Non-retryable error - return immediately
			return response, err
		}

		if attempt == maxAttempts-1 {
			break
		}

		// Calculate delay and wait
		delay := m.calculateBackoffDelay(attempt)
		select {
		case <-ctx.Done():
			return AIMessage{}, ctx.Err()
		case <-time.After(delay):
			// Continue to next attempt
		}
	}
	slog.Error("all retries exhausted", "error", lastErr)

	// All retries exhausted
	return AIMessage{}, lastErr
}

// Stream makes a streaming call to the model. It calls the chunkFunction for each chunk received.
// This is useful for real-time processing of model responses.
func (m *Model) Stream(ctx context.Context, messages []Message, tools []Tool, chunkFunction func(AIMessage) error) (AIMessage, error) {
	if m.callStreamingFunc == nil {
		return AIMessage{}, fmt.Errorf("streaming not supported for this model")
	}

	return m.streamWithRetry(ctx, messages, tools, chunkFunction)
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

func (m *Model) SetStreamingFunc(streamingFunc func(ctx context.Context, model *Model, messages []Message, tools []Tool, chunkFunction func(AIMessage) error) (AIMessage, error)) error {
	m.callStreamingFunc = streamingFunc
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
	recorded := RecordedResponse{
		AIMessage: response,
		Timestamp: time.Now().Format(time.RFC3339),
	}

	if err != nil {
		recorded.Error = err.Error()
	}

	jsonData, marshalErr := json.Marshal(recorded)
	if marshalErr != nil {
		return // Silently fail if we can't marshal
	}

	file, openErr := os.OpenFile(m.RecordFilename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if openErr != nil {
		return // Silently fail if we can't open file
	}
	defer file.Close()

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
