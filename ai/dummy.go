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
