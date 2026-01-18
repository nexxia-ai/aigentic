package ai

// This file contains the test suite for the ai package.
// It is used by packages that implement the ai models to test the ai models and its implementations.
// If a model implementation passes all these tests, it is considered to be working correctly.
import (
	"context"
	"embed"
	"strings"
	"sync"
	"testing"
	"time"
)

//go:embed testdata/*
var testData embed.FS

// Common echo tool for testing
var echoTool = Tool{
	Name:        "echo",
	Description: "Echoes back the input text",
	InputSchema: map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"text": map[string]interface{}{
				"type":        "string",
				"description": "The text to echo back",
			},
		},
		"required": []string{"text"},
	},
	Execute: func(args map[string]interface{}) (*ToolResult, error) {
		text := args["text"].(string)
		return &ToolResult{
			Content: []ToolContent{{
				Type:    "text",
				Content: text,
			}},
			Error: false,
		}, nil
	},
}

type testArgs struct {
	ctx      context.Context
	messages []Message
	tools    []Tool
}

// ModelTestSuite defines a test suite for a model implementation
type ModelTestSuite struct {
	NewModel  func() *Model
	Name      string
	SkipTests []string // List of test names to skip
}

// RunModelTestSuite runs all standard tests against a model implementation
func RunModelTestSuite(t *testing.T, suite ModelTestSuite) {

	// Helper function to check if a test should be skipped
	shouldSkipTest := func(testName string) bool {
		for _, skipTest := range suite.SkipTests {
			if skipTest == testName {
				return true
			}
		}
		return false
	}

	t.Run(suite.Name, func(t *testing.T) {
		t.Run("GenerateSimple", func(t *testing.T) {
			if shouldSkipTest("GenerateSimple") {
				t.Skipf("Skipping GenerateSimple test for %s", suite.Name)
			}
			TestGenerateSimple(t, suite.NewModel())
		})

		t.Run("ProcessImage", func(t *testing.T) {
			if shouldSkipTest("ProcessImage") {
				t.Skipf("Skipping ProcessImage test for %s", suite.Name)
			}
			TestProcessImage(t, suite.NewModel())
		})

		t.Run("ProcessAttachments", func(t *testing.T) {
			if shouldSkipTest("ProcessAttachments") {
				t.Skipf("Skipping ProcessAttachments test for %s", suite.Name)
			}
			TestProcessAttachments(t, suite.NewModel())
		})

		t.Run("GenerateContentWithTools", func(t *testing.T) {
			if shouldSkipTest("GenerateContentWithTools") {
				t.Skipf("Skipping GenerateContentWithTools test for %s", suite.Name)
			}
			TestGenerateContentWithTools(t, suite.NewModel())
		})

		t.Run("SetContextSize", func(t *testing.T) {
			if shouldSkipTest("SetContextSize") {
				t.Skipf("Skipping SetContextSize test for %s", suite.Name)
			}
			TestSetContextSize(t, suite.NewModel())
		})

		t.Run("AllZeroValues", func(t *testing.T) {
			if shouldSkipTest("AllZeroValues") {
				t.Skipf("Skipping AllZeroValues test for %s", suite.Name)
			}
			TestAllZeroValues(t, suite.NewModel())
		})

		t.Run("ChainingAndOverwriting", func(t *testing.T) {
			if shouldSkipTest("ChainingAndOverwriting") {
				t.Skipf("Skipping ChainingAndOverwriting test for %s", suite.Name)
			}
			TestChainingAndOverwriting(t, suite.NewModel())
		})

		t.Run("ContextTimeout", func(t *testing.T) {
			if shouldSkipTest("ContextTimeout") {
				t.Skipf("Skipping ContextTimeout test for %s", suite.Name)
			}
			TestContextTimeout(t, suite.NewModel())
		})

		t.Run("ContextCancellation", func(t *testing.T) {
			if shouldSkipTest("ContextCancellation") {
				t.Skipf("Skipping ContextCancellation test for %s", suite.Name)
			}
			TestContextCancellation(t, suite.NewModel())
		})

		t.Run("StreamingBasic", func(t *testing.T) {
			if shouldSkipTest("StreamingBasic") {
				t.Skipf("Skipping StreamingBasic test for %s", suite.Name)
			}
			TestStreamingBasic(t, suite.NewModel())
		})

		t.Run("StreamingWithTools", func(t *testing.T) {
			if shouldSkipTest("StreamingWithTools") {
				t.Skipf("Skipping StreamingWithTools test for %s", suite.Name)
			}
			TestStreamingWithTools(t, suite.NewModel())
		})

		t.Run("TextMultipleMessages", func(t *testing.T) {
			if shouldSkipTest("TextMultipleMessages") {
				t.Skipf("Skipping TextMultipleMessages test for %s", suite.Name)
			}
			TestTextMultipleMessages(t, suite.NewModel())
		})
	})
}

// Individual test functions that can be reused
func TestGenerateSimple(t *testing.T, model *Model) {
	args := testArgs{
		ctx:      context.Background(),
		messages: []Message{UserMessage{Role: UserRole, Content: "What is the capital of Australia?"}},
		tools:    []Tool{},
	}

	got, err := model.Call(args.ctx, args.messages, args.tools)
	if err != nil {
		t.Errorf("Model.Call() error = %v", err)
		return
	}
	_, content := got.Value()
	if !strings.Contains(content, "Canberra") {
		t.Errorf("Model.Call() = %v, want content containing 'Canberra'", got)
	}
}

func TestProcessImage(t *testing.T, model *Model) {
	testArgs := testArgs{
		ctx: context.Background(),
		messages: []Message{
			UserMessage{Role: UserRole,
				Content: "Extract the text from this image and return the word SUCCESS if it worked followed by the extracted text verbatim" +
					"Return the word FAILED followed by your explanation if you could not extract the text" +
					"Do not make up information. If you cannot read the image, return FAILED followed by your explanation."},
			UserMessage{Role: UserRole, Parts: []ContentPart{
				{Type: ContentPartImage, Name: "test.png", MimeType: "image/png", Data: func() []byte {
					data, _ := testData.ReadFile("testdata/test.png")
					return data
				}()},
			}},
		},
		tools: []Tool{},
	}

	got, err := model.Call(testArgs.ctx, testArgs.messages, testArgs.tools)
	if err != nil {
		t.Errorf("Model.Generate() error = %v", err)
		return
	}
	_, content := got.Value()
	t.Logf("Model returned: %s", content)
	if !strings.Contains(strings.ToLower(content), strings.ToLower("SUCCESS")) {
		t.Errorf("Model.Generate() did not find required word 'SUCCESS' in response: %s", content)
	}
}

func TestProcessAttachments(t *testing.T, model *Model) {
	testArgs := testArgs{
		ctx: context.Background(),
		messages: []Message{
			UserMessage{Role: UserRole, Content: "Read the content of this text file and return the content of the file verbatim. do not add any other text"},
			UserMessage{Role: UserRole, Parts: []ContentPart{
				{Type: ContentPartText, Name: "sample.txt", MimeType: "text/plain", Text: func() string {
					data, _ := testData.ReadFile("testdata/sample.txt")
					return string(data)
				}()},
			}},
		},
		tools: []Tool{},
	}

	got, err := model.Call(testArgs.ctx, testArgs.messages, testArgs.tools)
	if err != nil {
		t.Errorf("Model.Generate() error = %v", err)
		return
	}
	_, content := got.Value()
	if !strings.Contains(strings.ToLower(content), strings.ToLower("ATTACHMENT_SUCCESS")) {
		t.Errorf("Model.Generate() did not find required word 'ATTACHMENT_SUCCESS' in response: %s", content)
	}
}

func TestGenerateContentWithTools(t *testing.T, model *Model) {
	args := testArgs{
		ctx: context.Background(),
		messages: []Message{
			UserMessage{Role: UserRole, Content: "Please use the echo tool to echo back the text 'Hello, World!'"},
		},
		tools: []Tool{
			echoTool,
		},
	}

	got, err := model.Call(args.ctx, args.messages, args.tools)
	if err != nil {
		t.Errorf("Model.Generate() error = %v", err)
		return
	}

	// Check that it's an AIMessage with tool calls
	if len(got.ToolCalls) == 0 {
		t.Errorf("Expected tool calls in response, got none")
	} else {
		// Check that the tool call is for the echo tool
		found := false
		for _, toolCall := range got.ToolCalls {
			if toolCall.Name == "echo" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected echo tool call, got: %v", got.ToolCalls)
		}
	}
}

func TestSetContextSize(t *testing.T, model *Model) {
	result := model.WithContextSize(8192).WithTemperature(0.7).WithMaxTokens(1000)

	if result != model {
		t.Error("WithContextSize should return the same model instance for chaining")
	}

	if model.ContextSize == nil || *model.ContextSize != 8192 {
		t.Errorf("Expected context size 8192, got %v", model.ContextSize)
	}

	if model.Temperature == nil || *model.Temperature != 0.7 {
		t.Errorf("Expected temperature 0.7, got %v", model.Temperature)
	}

	if model.MaxTokens == nil || *model.MaxTokens != 1000 {
		t.Errorf("Expected max tokens 1000, got %v", model.MaxTokens)
	}
}

func TestAllZeroValues(t *testing.T, model *Model) {
	// Test all available With methods with zero values
	result := model.
		WithTemperature(0.0).
		WithMaxTokens(0).
		WithContextSize(0).
		WithTopP(0.0).
		WithFrequencyPenalty(0.0).
		WithPresencePenalty(0.0).
		WithStopSequences([]string{})

	// Verify the model was returned for chaining
	if result != model {
		t.Error("With methods should return the same model instance for chaining")
	}

	// Verify all values were set to zero
	if model.Temperature == nil || *model.Temperature != 0.0 {
		t.Errorf("Expected temperature 0.0, got %v", model.Temperature)
	}
	if model.MaxTokens == nil || *model.MaxTokens != 0 {
		t.Errorf("Expected max tokens 0, got %v", model.MaxTokens)
	}
	if model.ContextSize == nil || *model.ContextSize != 0 {
		t.Errorf("Expected context size 0, got %v", model.ContextSize)
	}
	if model.TopP == nil || *model.TopP != 0.0 {
		t.Errorf("Expected top_p 0.0, got %v", model.TopP)
	}
	if model.FrequencyPenalty == nil || *model.FrequencyPenalty != 0.0 {
		t.Errorf("Expected frequency penalty 0.0, got %v", model.FrequencyPenalty)
	}
	if model.PresencePenalty == nil || *model.PresencePenalty != 0.0 {
		t.Errorf("Expected presence penalty 0.0, got %v", model.PresencePenalty)
	}
	if model.StopSequences == nil || len(*model.StopSequences) != 0 {
		t.Errorf("Expected empty stop sequences, got %v", model.StopSequences)
	}
}

func TestChainingAndOverwriting(t *testing.T, model *Model) {
	// Test chaining with multiple method calls and overwriting
	result := model.
		WithTemperature(0.5).
		WithMaxTokens(100).
		WithContextSize(4096).
		WithTemperature(0.8). // Overwrite previous value
		WithMaxTokens(200).   // Overwrite previous value
		WithContextSize(8192) // Overwrite previous value

	// Verify the model was returned for chaining
	if result != model {
		t.Error("With methods should return the same model instance for chaining")
	}

	// Verify the final values are the last ones set
	if model.Temperature == nil || *model.Temperature != 0.8 {
		t.Errorf("Expected temperature 0.8, got %v", model.Temperature)
	}
	if model.MaxTokens == nil || *model.MaxTokens != 200 {
		t.Errorf("Expected max tokens 200, got %v", model.MaxTokens)
	}
	if model.ContextSize == nil || *model.ContextSize != 8192 {
		t.Errorf("Expected context size 8192, got %v", model.ContextSize)
	}
}

func TestContextTimeout(t *testing.T, model *Model) {
	testCases := []struct {
		name        string
		timeout     time.Duration
		expectError bool
		errorType   string
	}{
		{
			name:        "short timeout",
			timeout:     100 * time.Millisecond, // Very short timeout
			expectError: true,
			errorType:   "context deadline exceeded",
		},
		{
			name:        "reasonable timeout",
			timeout:     30 * time.Second, // Reasonable timeout
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a context with timeout
			ctx, cancel := context.WithTimeout(context.Background(), tc.timeout)
			defer cancel()

			// Create a simple message
			messages := []Message{
				UserMessage{
					Role:    UserRole,
					Content: "What is the capital of France?",
				},
			}

			// Make the call
			start := time.Now()
			response, err := model.Call(ctx, messages, []Tool{})
			duration := time.Since(start)

			if tc.expectError {
				// Should have an error
				if err == nil {
					t.Error("Expected error due to timeout, but got none")
					return
				}

				// Check error type
				errStr := err.Error()
				if !strings.Contains(strings.ToLower(errStr), strings.ToLower(tc.errorType)) {
					t.Errorf("Expected error containing '%s', got: %s", tc.errorType, errStr)
				}

				// Verify the call was actually interrupted (duration should be close to timeout)
				if duration > tc.timeout*2 {
					t.Errorf("Call took too long (%v) for timeout test with %v timeout", duration, tc.timeout)
				}
			} else {
				// Should not have an error
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
					return
				}

				// Should have a response
				if response.Content == "" {
					t.Error("Expected response content, got empty string")
				}

				// Verify the call completed within a reasonable time
				if duration > tc.timeout {
					t.Errorf("Call took longer than timeout (%v > %v)", duration, tc.timeout)
				}
			}
		})
	}
}

func TestContextCancellation(t *testing.T, model *Model) {
	testCases := []struct {
		name        string
		cancelDelay time.Duration
		expectError bool
		errorType   string
	}{
		{
			name:        "immediate cancellation",
			cancelDelay: 100 * time.Millisecond,
			expectError: true,
			errorType:   "context canceled",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a context that will be cancelled
			ctx, cancel := context.WithCancel(context.Background())

			// Create a simple message
			messages := []Message{
				UserMessage{
					Role:    UserRole,
					Content: "What is the capital of France?",
				},
			}

			// Start the call in a goroutine
			var response AIMessage
			var err error
			var wg sync.WaitGroup
			wg.Add(1)

			go func() {
				defer wg.Done()
				response, err = model.Call(ctx, messages, []Tool{})
			}()

			// Cancel the context after the specified delay
			time.Sleep(tc.cancelDelay)
			cancel()

			// Wait for the call to complete
			wg.Wait()

			if tc.expectError {
				// Should have an error
				if err == nil {
					t.Error("Expected error due to context cancellation, but got none")
					return
				}

				// Check error type
				errStr := err.Error()
				if !strings.Contains(strings.ToLower(errStr), strings.ToLower(tc.errorType)) {
					t.Errorf("Expected error containing '%s', got: %s", tc.errorType, errStr)
				}

			} else {
				// Should not have an error
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
					return
				}

				// Should have a response
				if response.Content == "" {
					t.Error("Expected response content, got empty string")
				}

				// Log the response for debugging
				t.Logf("Got successful response: %s", response.Content)
			}
		})
	}
}

func TestStreamingBasic(t *testing.T, model *Model) {
	args := testArgs{
		ctx:      context.Background(),
		messages: []Message{UserMessage{Role: UserRole, Content: "Please explain the process of photosynthesis"}},
		tools:    []Tool{},
	}

	// Track streaming chunks
	var chunks []string
	var thinkChunks []string
	var finalMessage AIMessage

	// Test streaming
	finalMessage, err := model.Stream(args.ctx, args.messages, args.tools, func(chunk AIMessage) error {
		// Simply append each chunk's content
		chunks = append(chunks, chunk.Content)
		thinkChunks = append(thinkChunks, chunk.Think)
		// t.Logf("Chunk: %s", chunk.Content)
		return nil
	})

	if err != nil {
		t.Fatalf("Streaming failed: %v", err)
	}

	// Verify we got a final message
	if finalMessage.Content == "" {
		t.Error("Final message content is empty")
	}

	// Verify we received multiple chunks
	if len(chunks) < 2 {
		t.Errorf("Expected multiple chunks, got %d", len(chunks))
	}
	// t.Logf("Got %d chunks", len(chunks))

	// Verify the final content matches the accumulated chunks
	accumulatedChunks := strings.Join(chunks, "")
	if finalMessage.Content != accumulatedChunks {
		t.Error("Final message content doesn't match accumulated chunks")
	}

	// Verify the final think matches the accumulated think
	accumulatedThink := strings.Join(thinkChunks, "")
	if finalMessage.Think != accumulatedThink {
		t.Error("Final message think doesn't match accumulated think")
	}

	// t.Logf("Final message: %s", finalMessage.Content)
}

func TestStreamingWithTools(t *testing.T, model *Model) {
	args := testArgs{
		ctx: context.Background(),
		messages: []Message{
			UserMessage{Role: UserRole, Content: `
				Please create a detailed explanation of how greenhouse gases contribute to global warming. 
				Then send your explanation to the echo tool and append "Analysis Complete" to the end of your explanation. 
				Finally respond with the result from the echo tool.`,
			},
		},
		tools: []Tool{echoTool},
	}

	// Track streaming chunks
	var chunks []AIMessage

	// Test streaming
	finalMessage, err := model.Stream(args.ctx, args.messages, args.tools, func(chunk AIMessage) error {
		chunks = append(chunks, chunk)
		if len(chunk.ToolCalls) > 0 {
			t.Logf("Tool calls: %+v", chunk.ToolCalls)
		}
		return nil
	})

	if err != nil {
		t.Fatalf("Streaming failed: %v", err)
	}

	// Verify we got a final message
	if finalMessage.Content == "" && len(finalMessage.ToolCalls) == 0 {
		t.Error("Final message has no content and no tool calls")
	}

	// Check that we received tool calls in any chunk or final message
	foundEchoTool := false
	for _, toolCall := range finalMessage.ToolCalls {
		if toolCall.Name == "echo" {
			foundEchoTool = true
			if !strings.Contains(toolCall.Args, "Analysis Complete") {
				t.Errorf("Expected 'Analysis Complete' in tool call args, got %s", toolCall.Args)
			}
			break
		}
	}

	if !foundEchoTool {
		t.Errorf("Expected echo tool call in chunks or final message, but found none in %d chunks", len(chunks))
	}

	// t.Logf("Final message: %s", finalMessage.Content)
}

func TestTextMultipleMessages(t *testing.T, model *Model) {
	args := testArgs{
		ctx: context.Background(),
		messages: []Message{
			SystemMessage{Role: SystemRole, Content: "You are a helpful AI assistant that provides accurate information."},
			SystemMessage{Role: SystemRole, Content: "Always be polite and professional in your responses."},
			SystemMessage{Role: SystemRole, Content: "When using tools, explain what you're doing clearly."},
			UserMessage{Role: UserRole, Content: "Hello, I need help with some calculations."},
			UserMessage{Role: UserRole, Content: "Can you use the echo tool to repeat 'Calculation started'?"},
			AIMessage{
				Role:    AssistantRole,
				Content: "I'll help you with that calculation. Let me first use the echo tool as requested.",
				ToolCalls: []ToolCall{
					{
						ID:   "call_123",
						Type: "function",
						Name: "echo",
						Args: `{"text": "Calculation started"}`,
					},
				},
			},
			ToolMessage{
				Role:       ToolRole,
				Content:    "Calculation started",
				ToolCallID: "call_123",
				ToolName:   "echo",
			},
			UserMessage{Role: UserRole, Content: "Then tell me what 25 multiplied by 4 equals."},
		},
		tools: []Tool{echoTool},
	}

	got, err := model.Call(args.ctx, args.messages, args.tools)
	if err != nil {
		t.Errorf("Model.Call() error = %v", err)
		return
	}

	if got.Content == "" {
		t.Errorf("Expected non-empty response content")
		return
	}

	// Verify the response acknowledges the calculation request
	content := strings.ToLower(got.Content)
	if !strings.Contains(content, "100") && !strings.Contains(content, "25") && !strings.Contains(content, "4") {
		t.Errorf("Expected response to contain calculation result or reference to numbers, got: %s", got.Content)
	}

	// t.Logf("Response: %s", got.Content)
}
