package ai

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// TestRetryMechanism tests the retry functionality of the model
func TestRetryMechanism(t *testing.T) {
	t.Run("TemporaryErrorRetries", func(t *testing.T) {
		attempts := 0
		maxRetries := 3

		model := NewDummyModel(func(ctx context.Context, messages []Message, tools []Tool) (AIMessage, error) {
			attempts++
			// Fail first 2 attempts with temporary error, succeed on 3rd
			if attempts < 3 {
				return AIMessage{}, ErrTemporary
			}
			return AIMessage{
				Role:    AssistantRole,
				Content: "Success after retries",
			}, nil
		})
		model.MaxRetries = &maxRetries

		response, err := model.Call(context.Background(), []Message{
			UserMessage{Role: UserRole, Content: "Test message"},
		}, []Tool{})

		if err != nil {
			t.Errorf("Expected success after retries, got error: %v", err)
		}
		if response.Content != "Success after retries" {
			t.Errorf("Expected 'Success after retries', got: %s", response.Content)
		}
		if attempts != 3 {
			t.Errorf("Expected 3 attempts, got %d", attempts)
		}
	})

	t.Run("NonTemporaryErrorNoRetry", func(t *testing.T) {
		attempts := 0
		maxRetries := 3

		model := NewDummyModel(func(ctx context.Context, messages []Message, tools []Tool) (AIMessage, error) {
			attempts++
			// Return a non-temporary error
			return AIMessage{}, fmt.Errorf("permanent error")
		})
		model.MaxRetries = &maxRetries

		_, err := model.Call(context.Background(), []Message{
			UserMessage{Role: UserRole, Content: "Test message"},
		}, []Tool{})

		if err == nil {
			t.Error("Expected error, got nil")
		}
		if err.Error() != "permanent error" {
			t.Errorf("Expected 'permanent error', got: %v", err)
		}
		if attempts != 1 {
			t.Errorf("Expected 1 attempt (no retry), got %d", attempts)
		}
	})

	t.Run("MaxRetriesExhausted", func(t *testing.T) {
		attempts := 0
		maxRetries := 2

		model := NewDummyModel(func(ctx context.Context, messages []Message, tools []Tool) (AIMessage, error) {
			attempts++
			// Always return temporary error
			return AIMessage{}, ErrTemporary
		})
		model.MaxRetries = &maxRetries

		_, err := model.Call(context.Background(), []Message{
			UserMessage{Role: UserRole, Content: "Test message"},
		}, []Tool{})

		if err != ErrTemporary {
			t.Errorf("Expected ErrTemporary, got: %v", err)
		}
		if attempts != maxRetries {
			t.Errorf("Expected %d attempts, got %d", maxRetries, attempts)
		}
	})

	t.Run("StatusError503Retry", func(t *testing.T) {
		// This test simulates the specific error you encountered
		attempts := 0
		maxRetries := 3

		model := NewDummyModel(func(ctx context.Context, messages []Message, tools []Tool) (AIMessage, error) {
			attempts++
			// Simulate 503 error for first 2 attempts, then succeed
			if attempts < 3 {
				return AIMessage{}, ErrTemporary
			}
			return AIMessage{
				Role:    AssistantRole,
				Content: "Success after 503 errors",
			}, nil
		})
		model.MaxRetries = &maxRetries

		response, err := model.Call(context.Background(), []Message{
			UserMessage{Role: UserRole, Content: "Test message"},
		}, []Tool{})

		if err != nil {
			t.Errorf("Expected success after retries, got error: %v", err)
		}
		if response.Content != "Success after 503 errors" {
			t.Errorf("Expected 'Success after 503 errors', got: %s", response.Content)
		}
		if attempts != 3 {
			t.Errorf("Expected 3 attempts, got %d", attempts)
		}
	})

	t.Run("ZeroMaxRetriesStillMakesOneAttempt", func(t *testing.T) {
		attempts := 0
		maxRetries := 0

		model := NewDummyModel(func(ctx context.Context, messages []Message, tools []Tool) (AIMessage, error) {
			attempts++
			return AIMessage{
				Role:    AssistantRole,
				Content: "Single attempt",
			}, nil
		})
		model.MaxRetries = &maxRetries

		response, err := model.Call(context.Background(), []Message{
			UserMessage{Role: UserRole, Content: "Test message"},
		}, []Tool{})

		if err != nil {
			t.Errorf("Expected success, got error: %v", err)
		}
		if response.Content != "Single attempt" {
			t.Errorf("Expected 'Single attempt', got: %s", response.Content)
		}
		if attempts != 1 {
			t.Errorf("Expected 1 attempt, got %d", attempts)
		}
	})

	t.Run("DefaultMaxRetries", func(t *testing.T) {
		attempts := 0

		model := NewDummyModel(func(ctx context.Context, messages []Message, tools []Tool) (AIMessage, error) {
			attempts++
			// Always return temporary error to test default max retries
			return AIMessage{}, ErrTemporary
		})
		// Don't set MaxRetries, should use default value of 3

		_, err := model.Call(context.Background(), []Message{
			UserMessage{Role: UserRole, Content: "Test message"},
		}, []Tool{})

		if err != ErrTemporary {
			t.Errorf("Expected ErrTemporary, got: %v", err)
		}
		if attempts != defaultMaxRetries {
			t.Errorf("Expected %d attempts (default), got %d", defaultMaxRetries, attempts)
		}
	})
}

// TestBackoffDelayCalculation tests the exponential backoff delay calculation
func TestBackoffDelayCalculation(t *testing.T) {
	model := &Model{}

	testCases := []struct {
		attempt     int
		minExpected time.Duration
		maxExpected time.Duration
	}{
		{0, 900 * time.Millisecond, 1100 * time.Millisecond},  // 1s ± 10% jitter
		{1, 1800 * time.Millisecond, 2200 * time.Millisecond}, // 2s ± 10% jitter
		{2, 3600 * time.Millisecond, 4400 * time.Millisecond}, // 4s ± 10% jitter
		{10, 29 * time.Second, 33 * time.Second},              // Should cap at 30s ± 10% jitter
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("Attempt%d", tc.attempt), func(t *testing.T) {
			delay := model.calculateBackoffDelay(tc.attempt)

			if delay < tc.minExpected || delay > tc.maxExpected {
				t.Errorf("Attempt %d: expected delay between %v and %v, got %v",
					tc.attempt, tc.minExpected, tc.maxExpected, delay)
			}
		})
	}
}

// TestStreamingRetry tests retry functionality for streaming calls
func TestStreamingRetry(t *testing.T) {
	t.Run("StreamingTemporaryErrorRetries", func(t *testing.T) {
		attempts := 0
		maxRetries := 3

		model := NewDummyModel(func(ctx context.Context, messages []Message, tools []Tool) (AIMessage, error) {
			attempts++
			// Fail first 2 attempts with temporary error, succeed on 3rd
			if attempts < 3 {
				return AIMessage{}, ErrTemporary
			}
			return AIMessage{
				Role:    AssistantRole,
				Content: "Streaming success after retries",
			}, nil
		})
		model.MaxRetries = &maxRetries

		var chunks []string
		response, err := model.Stream(context.Background(), []Message{
			UserMessage{Role: UserRole, Content: "Test message"},
		}, []Tool{}, func(chunk AIMessage) error {
			chunks = append(chunks, chunk.Content)
			return nil
		})

		if err != nil {
			t.Errorf("Expected success after retries, got error: %v", err)
		}
		if response.Content != "Streaming success after retries" {
			t.Errorf("Expected 'Streaming success after retries', got: %s", response.Content)
		}
		if attempts != 3 {
			t.Errorf("Expected 3 attempts, got %d", attempts)
		}
		if len(chunks) == 0 {
			t.Error("Expected streaming chunks, got none")
		}
	})
}

// TestContextCancellationDuringRetry tests that context cancellation works during retry delays
func TestContextCancellationDuringRetry(t *testing.T) {
	attempts := 0
	maxRetries := 5

	model := NewDummyModel(func(ctx context.Context, messages []Message, tools []Tool) (AIMessage, error) {
		attempts++
		// Always return temporary error to trigger retries
		return AIMessage{}, ErrTemporary
	})
	model.MaxRetries = &maxRetries

	// Create a context that will be cancelled after a short delay
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := model.Call(ctx, []Message{
		UserMessage{Role: UserRole, Content: "Test message"},
	}, []Tool{})

	duration := time.Since(start)

	// Should get a context error
	if err == nil {
		t.Error("Expected context error, got nil")
	}

	// Should not take too long (should be cancelled before all retries complete)
	if duration > 200*time.Millisecond {
		t.Errorf("Call took too long (%v), context cancellation may not be working during retries", duration)
	}

	// Should have made at least one attempt
	if attempts == 0 {
		t.Error("Expected at least one attempt")
	}
}
