package ai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"os"
	"time"
)

// Gemini-specific request/response types
type GeminiGenerateContentRequest struct {
	Contents         []GeminiContent `json:"contents"`
	GenerationConfig *GeminiConfig   `json:"generationConfig,omitempty"`
	Tools            []GeminiTool    `json:"tools,omitempty"`
}

type GeminiContent struct {
	Role  string              `json:"role"`
	Parts []GeminiContentPart `json:"parts"`
}

type GeminiContentPart struct {
	Text         string              `json:"text,omitempty"`
	InlineData   *GeminiInlineData   `json:"inlineData,omitempty"`
	FunctionCall *GeminiFunctionCall `json:"functionCall,omitempty"`
}

type GeminiInlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"` // base64 encoded
}

type GeminiFunctionCall struct {
	Name string                 `json:"name"`
	Args map[string]interface{} `json:"args"`
}

type GeminiConfig struct {
	Temperature     float64  `json:"temperature,omitempty"`
	TopP            float64  `json:"topP,omitempty"`
	TopK            int      `json:"topK,omitempty"`
	MaxOutputTokens int      `json:"maxOutputTokens,omitempty"`
	StopSequences   []string `json:"stopSequences,omitempty"`
}

type GeminiTool struct {
	FunctionDeclarations []GeminiFunctionDeclaration `json:"functionDeclarations,omitempty"`
}

type GeminiFunctionDeclaration struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

type GeminiGenerateContentResponse struct {
	Candidates     []GeminiCandidate     `json:"candidates"`
	PromptFeedback *GeminiPromptFeedback `json:"promptFeedback,omitempty"`
	UsageMetadata  *GeminiUsageMetadata  `json:"usageMetadata,omitempty"`
}

type GeminiCandidate struct {
	Content       GeminiContent        `json:"content"`
	FinishReason  string               `json:"finishReason"`
	Index         int                  `json:"index"`
	SafetyRatings []GeminiSafetyRating `json:"safetyRatings,omitempty"`
}

type GeminiSafetyRating struct {
	Category    string `json:"category"`
	Probability string `json:"probability"`
}

type GeminiPromptFeedback struct {
	SafetyRatings []GeminiSafetyRating `json:"safetyRatings,omitempty"`
}

type GeminiUsageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

// NewGeminiModel creates a new Gemini model using the Model struct
func NewGeminiModel(modelName string, apiKey string) *Model {
	if apiKey == "" {
		apiKey = os.Getenv("GOOGLE_API_KEY")
	}

	// Initialize random seed for jitter calculation
	rand.Seed(time.Now().UnixNano())

	return &Model{
		ModelName: modelName,
		APIKey:    apiKey,
		BaseURL:   "https://generativelanguage.googleapis.com/v1beta",
		client: &http.Client{
			Timeout: 5 * time.Minute, // Add timeout to prevent hanging requests
		},
		callFunc: geminiGenerate,
	}
}

// geminiGenerateWithRetry is the generate function for Gemini models with retry logic
func geminiGenerateWithRetry(ctx context.Context, model *Model, messages []Message, tools []Tool) (AIMessage, error) {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Convert our message format to Gemini's format
		geminiContents := geminiConvertMessages(messages)
		geminiTools := geminiConvertTools(tools)

		// Make a single LLM call
		response, err := geminiREST(ctx, model, geminiContents, geminiTools)
		if err == nil {
			// Success - return the response
			return response, nil
		}

		lastErr = err

		// Check if this is a retryable error
		if !isRetryableError(err) {
			// Non-retryable error - return immediately
			return AIMessage{}, fmt.Errorf("failed to generate (non-retryable): %w", err)
		}

		// If this is the last attempt, don't retry
		if attempt == maxRetries {
			break
		}

		// Calculate delay for next retry
		delay := calculateBackoffDelay(attempt)

		slog.Warn("Gemini API request failed, retrying",
			"attempt", attempt+1,
			"max_attempts", maxRetries+1,
			"delay", delay,
			"error", err.Error())

		// Wait before retrying
		select {
		case <-ctx.Done():
			return AIMessage{}, fmt.Errorf("context cancelled during retry: %w", ctx.Err())
		case <-time.After(delay):
			// Continue to next attempt
		}
	}

	// All retries exhausted
	return AIMessage{}, fmt.Errorf("failed to generate after %d attempts. Last error: %w", maxRetries+1, lastErr)
}

// geminiGenerate is the generate function for Gemini models
func geminiGenerate(ctx context.Context, model *Model, messages []Message, tools []Tool) (AIMessage, error) {
	return geminiGenerateWithRetry(ctx, model, messages, tools)
}

// geminiConvertMessages converts our message format to Gemini's format
func geminiConvertMessages(messages []Message) []GeminiContent {
	geminiContents := make([]GeminiContent, 0, len(messages))

	// Track if we have a pending user message to combine with resources
	var pendingUserContent *GeminiContent

	for i := 0; i < len(messages); i++ {
		msg := messages[i]
		role, _ := msg.Value()

		if role == SystemRole {
			continue
		}

		switch r := msg.(type) {
		case UserMessage:
			// Look ahead for an image ResourceMessage
			if i+1 < len(messages) {
				if imgRes, ok := messages[i+1].(ResourceMessage); ok && imgRes.Type == "image" {
					// Create user content with image part first, then text
					parts := []GeminiContentPart{}
					if imageData, ok := imgRes.Body.([]byte); ok {
						parts = append(parts, GeminiContentPart{
							InlineData: &GeminiInlineData{
								MimeType: imgRes.MIMEType,
								Data:     base64.StdEncoding.EncodeToString(imageData),
							},
						})
					}
					if imgRes.Description != "" {
						parts = append(parts, GeminiContentPart{Text: imgRes.Description})
					}
					parts = append(parts, GeminiContentPart{Text: r.Content})
					geminiContents = append(geminiContents, GeminiContent{
						Role:  "user",
						Parts: parts,
					})
					i++ // Skip the image ResourceMessage
					continue
				}
			}
			// No image follows, handle as before
			if pendingUserContent != nil {
				geminiContents = append(geminiContents, *pendingUserContent)
			}
			pendingUserContent = &GeminiContent{
				Role:  string(role),
				Parts: []GeminiContentPart{{Text: r.Content}},
			}

		case AIMessage:
			if pendingUserContent != nil {
				geminiContents = append(geminiContents, *pendingUserContent)
				pendingUserContent = nil
			}
			content := GeminiContent{
				Role:  string(role),
				Parts: []GeminiContentPart{{Text: r.Content}},
			}
			geminiContents = append(geminiContents, content)

		case ToolMessage:
			if pendingUserContent != nil {
				geminiContents = append(geminiContents, *pendingUserContent)
				pendingUserContent = nil
			}
			content := GeminiContent{
				Role:  string(role),
				Parts: []GeminiContentPart{{Text: r.Content}},
			}
			geminiContents = append(geminiContents, content)

		case ResourceMessage:
			switch r.Type {
			case "image":
				if pendingUserContent != nil {
					if imageData, ok := r.Body.([]byte); ok {
						pendingUserContent.Parts = append([]GeminiContentPart{{
							InlineData: &GeminiInlineData{
								MimeType: r.MIMEType,
								Data:     base64.StdEncoding.EncodeToString(imageData),
							},
						}}, pendingUserContent.Parts...)
					}
					if r.Description != "" {
						pendingUserContent.Parts = append([]GeminiContentPart{{Text: r.Description}}, pendingUserContent.Parts...)
					}
				} else {
					content := GeminiContent{
						Role:  "user",
						Parts: []GeminiContentPart{},
					}
					if imageData, ok := r.Body.([]byte); ok {
						content.Parts = append(content.Parts, GeminiContentPart{
							InlineData: &GeminiInlineData{
								MimeType: r.MIMEType,
								Data:     base64.StdEncoding.EncodeToString(imageData),
							},
						})
					}
					if r.Description != "" {
						content.Parts = append(content.Parts, GeminiContentPart{Text: r.Description})
					}
					geminiContents = append(geminiContents, content)
				}

			case "text", "file":
				// Handle text files by converting to text content
				if pendingUserContent != nil {
					// Add file content to existing user message
					if textContent, ok := r.Body.([]byte); ok {
						fileText := string(textContent)
						if r.Description != "" {
							fileText = r.Description + "\n\n" + fileText
						}
						pendingUserContent.Parts = append(pendingUserContent.Parts, GeminiContentPart{Text: fileText})
					} else if textStr, ok := r.Body.(string); ok {
						fileText := textStr
						if r.Description != "" {
							fileText = r.Description + "\n\n" + fileText
						}
						pendingUserContent.Parts = append(pendingUserContent.Parts, GeminiContentPart{Text: fileText})
					}
				} else {
					// Create new user content for the file
					content := GeminiContent{
						Role:  "user",
						Parts: []GeminiContentPart{},
					}

					if textContent, ok := r.Body.([]byte); ok {
						fileText := string(textContent)
						if r.Description != "" {
							fileText = r.Description + "\n\n" + fileText
						}
						content.Parts = append(content.Parts, GeminiContentPart{Text: fileText})
					} else if textStr, ok := r.Body.(string); ok {
						fileText := textStr
						if r.Description != "" {
							fileText = r.Description + "\n\n" + fileText
						}
						content.Parts = append(content.Parts, GeminiContentPart{Text: fileText})
					}

					geminiContents = append(geminiContents, content)
				}

			default:
				// For other resource types, convert to text
				if pendingUserContent != nil {
					if textContent, ok := r.Body.(string); ok {
						pendingUserContent.Parts = append(pendingUserContent.Parts, GeminiContentPart{Text: textContent})
					}
				} else {
					content := GeminiContent{
						Role:  "user",
						Parts: []GeminiContentPart{},
					}

					if textContent, ok := r.Body.(string); ok {
						content.Parts = append(content.Parts, GeminiContentPart{Text: textContent})
					}

					geminiContents = append(geminiContents, content)
				}
			}

		default:
			panic(fmt.Sprintf("unsupported message type: %T - check that message is not a pointer", r))
		}
	}

	if pendingUserContent != nil {
		geminiContents = append(geminiContents, *pendingUserContent)
	}

	for _, msg := range messages {
		if systemMsg, ok := msg.(SystemMessage); ok {
			for j := range geminiContents {
				if geminiContents[j].Role == "user" {
					systemPart := GeminiContentPart{Text: systemMsg.Content}
					geminiContents[j].Parts = append([]GeminiContentPart{systemPart}, geminiContents[j].Parts...)
					break
				}
			}
			break
		}
	}

	return geminiContents
}

// geminiConvertTools converts our tool format to Gemini's format
func geminiConvertTools(tools []Tool) []GeminiTool {
	if len(tools) == 0 {
		return nil
	}

	functionDeclarations := make([]GeminiFunctionDeclaration, len(tools))
	for i, tool := range tools {
		functionDeclarations[i] = GeminiFunctionDeclaration{
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  tool.InputSchema,
		}
	}

	return []GeminiTool{
		{
			FunctionDeclarations: functionDeclarations,
		},
	}
}

// geminiREST makes a single call to the Gemini API
func geminiREST(ctx context.Context, model *Model, contents []GeminiContent, tools []GeminiTool) (AIMessage, error) {
	req := &GeminiGenerateContentRequest{
		Contents: contents,
		Tools:    tools,
	}

	// Apply configuration values from model pointer fields
	config := &GeminiConfig{}
	hasConfig := false

	if model.Temperature != nil {
		config.Temperature = *model.Temperature
		hasConfig = true
	}
	if model.MaxTokens != nil {
		config.MaxOutputTokens = *model.MaxTokens
		hasConfig = true
	}
	if model.TopP != nil {
		config.TopP = *model.TopP
		hasConfig = true
	}
	if model.StopSequences != nil {
		config.StopSequences = *model.StopSequences
		hasConfig = true
	}

	if hasConfig {
		req.GenerationConfig = config
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return AIMessage{}, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Debug: Log the request structure
	slog.Debug("Gemini API request",
		"contents_count", len(contents),
		"request_size", len(reqBody))

	for i, content := range contents {
		slog.Debug("Content",
			"index", i,
			"role", content.Role,
			"parts_count", len(content.Parts))

		for j, part := range content.Parts {
			if part.Text != "" {
				slog.Debug("Part", "index", j, "type", "text", "length", len(part.Text))
			}
			if part.InlineData != nil {
				slog.Debug("Part", "index", j, "type", "inline_data", "mime_type", part.InlineData.MimeType, "data_length", len(part.InlineData.Data))
			}
		}
	}

	// Construct the API URL
	apiURL := fmt.Sprintf("%s/models/%s:generateContent", model.BaseURL, model.ModelName)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(reqBody))
	if err != nil {
		return AIMessage{}, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-goog-api-key", model.APIKey)

	resp, err := model.client.Do(httpReq)
	if err != nil {
		return AIMessage{}, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return AIMessage{}, &StatusError{
			StatusCode:   resp.StatusCode,
			Status:       resp.Status,
			ErrorMessage: string(respBody),
		}
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return AIMessage{}, fmt.Errorf("failed to read response body: %w", err)
	}

	var geminiResp GeminiGenerateContentResponse
	if err := json.Unmarshal(respBody, &geminiResp); err != nil {
		return AIMessage{}, fmt.Errorf("failed to decode response body: %w", err)
	}

	if len(geminiResp.Candidates) == 0 {
		return AIMessage{}, fmt.Errorf("no candidates in response")
	}

	candidate := geminiResp.Candidates[0]

	// Extract text content from parts
	var content string
	for _, part := range candidate.Content.Parts {
		if part.Text != "" {
			content += part.Text
		}
	}

	content, thinkPart := ExtractThinkTags(content)

	msg := AIMessage{
		Role:    MessageRole(candidate.Content.Role),
		Content: content,
		Think:   thinkPart,
	}

	// Handle function calls from Gemini
	for _, part := range candidate.Content.Parts {
		if part.FunctionCall != nil {
			// Convert function call args to JSON string
			argsJSON, err := json.Marshal(part.FunctionCall.Args)
			if err != nil {
				return AIMessage{}, fmt.Errorf("failed to marshal function call args: %w", err)
			}

			msg.ToolCalls = append(msg.ToolCalls, ToolCall{
				ID:     fmt.Sprintf("call_%d", len(msg.ToolCalls)+1), // Generate a unique ID
				Type:   "function",
				Name:   part.FunctionCall.Name,
				Args:   string(argsJSON),
				Result: "",
			})
		}
	}

	// Set response metadata
	if geminiResp.UsageMetadata != nil {
		msg.Response = Response{
			Model: model.ModelName,
			Usage: Usage{
				PromptTokens:     geminiResp.UsageMetadata.PromptTokenCount,
				CompletionTokens: geminiResp.UsageMetadata.CandidatesTokenCount,
				TotalTokens:      geminiResp.UsageMetadata.TotalTokenCount,
			},
		}
	}

	return msg, nil
}
