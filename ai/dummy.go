package ai

import (
	"context"
	"fmt"
	"sync"
)

// NewDummyModel is useful for testing purposes. It allows you to mock the model's response.
func NewDummyModel(responseFunc func(ctx context.Context, messages []Message, tools []Tool) (AIMessage, error)) *Model {
	return &Model{
		ModelName: "dummy",
		callFunc: func(ctx context.Context, model *Model, messages []Message, tools []Tool) (AIMessage, error) {
			return responseFunc(ctx, messages, tools)
		},
		callStreamingFunc: func(ctx context.Context, model *Model, messages []Message, tools []Tool, chunkFunction func(AIMessage) error) (AIMessage, error) {
			// Get the final response
			finalResponse, err := responseFunc(ctx, messages, tools)
			if err != nil {
				return finalResponse, err
			}

			// Simulate streaming by breaking the content into chunks
			content := finalResponse.Content
			if content == "" {
				// If no content, just return the final response
				return finalResponse, nil
			}

			// Split content into chunks (simulate streaming)
			chunkSize := len(content) / 3
			if chunkSize == 0 {
				chunkSize = 1
			}

			for i := 0; i < len(content); i += chunkSize {
				end := i + chunkSize
				if end > len(content) {
					end = len(content)
				}

				chunk := AIMessage{
					Role:    finalResponse.Role,
					Content: content[i:end],
				}

				// Call the chunk function
				if err := chunkFunction(chunk); err != nil {
					return AIMessage{}, err
				}
			}

			// Return the final complete response
			return finalResponse, nil
		},
	}
}

// Global state for replay function to ensure sequential access across multiple model instances
var (
	replayRecords     []RecordedResponse
	replayIndex       int
	replayMutex       sync.Mutex
	replayInitialized bool
)

// ReplayFunctionFromData creates a function that replays recorded responses from provided data sequentially
func ReplayFunctionFromData(responses []RecordedResponse) (func(ctx context.Context, messages []Message, tools []Tool) (AIMessage, error), error) {
	replayMutex.Lock()
	defer replayMutex.Unlock()

	// Reset the replay state
	replayRecords = responses
	replayIndex = 0
	replayInitialized = true

	// Return a function that replays the next recorded response
	return func(ctx context.Context, messages []Message, tools []Tool) (AIMessage, error) {
		// Check if context is done (timeout or cancellation)
		select {
		case <-ctx.Done():
			return AIMessage{}, ctx.Err()
		default:
			// Continue with normal processing
		}
		replayMutex.Lock()
		defer replayMutex.Unlock()

		// Check if we have more records to replay
		if replayIndex >= len(replayRecords) {
			// Return a default response if we've exhausted all records
			return AIMessage{
				Role:    AssistantRole,
				Content: fmt.Sprintf("No more recorded responses available (requested %d, have %d)", replayIndex+1, len(replayRecords)),
			}, nil
		}

		// Get the current record
		record := replayRecords[replayIndex]
		replayIndex++

		// Return the recorded response and error if it exists
		if record.Error != "" {
			return record.AIMessage, fmt.Errorf("%s: %s", record.AIMessage.Content, record.Error)
		}
		return record.AIMessage, nil
	}, nil
}

// ReplayFunction creates a function that loads recorded responses from a file and replays them sequentially
func ReplayFunction(filename string) (func(ctx context.Context, messages []Message, tools []Tool) (AIMessage, error), error) {
	// Load the recorded responses from file
	records, err := LoadDummyRecords(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to load recorded responses: %w", err)
	}

	// Use the ReplayFunctionFromData to create the replay function
	return ReplayFunctionFromData(records)
}
