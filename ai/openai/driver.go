package openai

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/nexxia-ai/aigentic/ai"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

const (
	OpenAIBaseURL     = "https://api.openai.com/v1"
	OpenRouterBaseURL = "https://openrouter.ai/api/v1"
	HeliconeBaseURL   = "https://ai-gateway.helicone.ai/v1"
)

func init() {
	registerStandardModels()
}

func registerStandardModels() {
	models := []struct {
		identifier string
		provider   string
		model      string
		family     string
		baseURL    string
		aPIKeyName string
	}{
		{"GPT 4o", "openai", "gpt-4o", "gpt", "", "OPENAI_API_KEY"},
		{"GPT 5", "openai", "gpt-5", "gpt", "", "OPENAI_API_KEY"},
		{"GPT 5 Mini", "openai", "gpt-5-mini", "gpt", "", "OPENAI_API_KEY"},
		{"GPT 5 Nano", "openai", "gpt-5-nano-2025-08-07", "gpt", "", "OPENAI_API_KEY"},
		{"GPT 5.2", "openai", "gpt-5.2-2025-12-11", "gpt", "", "OPENAI_API_KEY"},
		{"Qwen 30B (openrouter)", "openrouter", "qwen/qwen3-30b-a3b-instruct-2507", "qwen", OpenRouterBaseURL, "OPENROUTER_API_KEY"},
		{"Qwen 235B (openrouter)", "openrouter", "qwen/qwen3-235b-a22b-thinking-2507", "qwen", OpenRouterBaseURL, "OPENROUTER_API_KEY"},
		{"Qwen Max (openrouter)", "openrouter", "qwen/qwen3-max", "qwen", OpenRouterBaseURL, "OPENROUTER_API_KEY"},
		{"GLM 4.6 (openrouter)", "openrouter", "z-ai/glm-4.6", "glm", OpenRouterBaseURL, "OPENROUTER_API_KEY"},
		{"GLM 4.7 (openrouter)", "openrouter", "z-ai/glm-4.6", "glm", OpenRouterBaseURL, "OPENROUTER_API_KEY"},
		{"DeepSeek V3.1 Terminus (openrouter)", "openrouter", "deepseek/deepseek-v3.1-terminus", "deepseek", OpenRouterBaseURL, "OPENROUTER_API_KEY"},
		{"DeepSeek Chat V3.1 (openrouter)", "openrouter", "deepseek/deepseek-chat-v3.1", "deepseek", OpenRouterBaseURL, "OPENROUTER_API_KEY"},
		{"Grok Code Fast 1 (openrouter)", "openrouter", "x-ai/grok-code-fast-1", "grok", OpenRouterBaseURL, "OPENROUTER_API_KEY"},
	}

	for _, m := range models {
		baseURL := m.baseURL
		if baseURL == "" {
			baseURL = OpenAIBaseURL
		}
		ai.RegisterModel(ai.ModelInfo{
			Provider:   m.provider,
			Model:      m.model,
			Identifier: m.identifier,
			Family:     m.family,
			BaseURL:    baseURL,
			NewModel: func(modelName, apiKey string, baseURLs ...string) *ai.Model {
				return NewModel(modelName, apiKey, baseURLs...)
			},
			APIKeyName: m.aPIKeyName,
		})
	}
}

func NewModel(modelName string, apiKey string, baseURLs ...string) *ai.Model {
	url := OpenAIBaseURL
	if len(baseURLs) > 0 && baseURLs[0] != "" {
		url = baseURLs[0]
	}

	if apiKey == "" {
		switch url {
		case OpenRouterBaseURL:
			apiKey = os.Getenv("OPENROUTER_API_KEY")
			if apiKey == "" {
				slog.Error("OPENROUTER_API_KEY is not set")
			}
		default:
			apiKey = os.Getenv("OPENAI_API_KEY")
			if apiKey == "" {
				slog.Error("OPENAI_API_KEY is not set")
			}
		}
	}

	apiType := ai.APIResponses
	// OpenRouter and other OpenAI-compatible endpoints use Chat API
	if url == HeliconeBaseURL {
		apiType = ai.APIChat
	}

	model := &ai.Model{
		ModelName:  modelName,
		APIKey:     apiKey,
		BaseURL:    url,
		API:        apiType,
		Parameters: map[string]any{},
	}
	model.SetGenerateFunc(openaiGenerate)
	model.SetStreamingFunc(openaiStream)
	return model
}

func openaiGenerate(ctx context.Context, model *ai.Model, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
	client := createClient(model)

	if model.API == "" {
		model.API = ai.APIResponses
	}

	switch model.API {
	case ai.APIResponses:
		return callResponsesAPI(ctx, client, model, messages, tools)
	case ai.APIChat:
		return callChatAPI(ctx, client, model, messages, tools)
	default:
		return ai.AIMessage{}, fmt.Errorf("unsupported API: %s", model.API)
	}
}

func openaiStream(ctx context.Context, model *ai.Model, messages []ai.Message, tools []ai.Tool, chunkFunction func(ai.AIMessage) error) (ai.AIMessage, error) {
	client := createClient(model)

	if model.API == "" {
		model.API = ai.APIResponses
	}

	switch model.API {
	case ai.APIResponses:
		return streamResponsesAPI(ctx, client, model, messages, tools, chunkFunction)
	case ai.APIChat:
		return streamChatAPI(ctx, client, model, messages, tools, chunkFunction)
	default:
		return ai.AIMessage{}, fmt.Errorf("unsupported API: %s", model.API)
	}
}

func createClient(model *ai.Model) openai.Client {
	opts := []option.RequestOption{
		option.WithAPIKey(model.APIKey),
	}

	if model.BaseURL != "" && model.BaseURL != OpenAIBaseURL {
		opts = append(opts, option.WithBaseURL(model.BaseURL))
	}

	return openai.NewClient(opts...)
}

func isRetryableError(err error) error {
	if err == nil {
		return nil
	}

	errStr := err.Error()

	if strings.Contains(errStr, "status: 502") ||
		strings.Contains(errStr, "status: 503") ||
		strings.Contains(errStr, "status: 504") ||
		strings.Contains(errStr, "status: 429") {
		return fmt.Errorf("%w: %v", ai.ErrTemporary, err)
	}

	if strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "network") ||
		strings.Contains(errStr, "temporary") {
		return fmt.Errorf("%w: %v", ai.ErrTemporary, err)
	}

	var apiErr interface {
		StatusCode() int
	}
	if errors.As(err, &apiErr) {
		if apiErr.StatusCode() >= 500 || apiErr.StatusCode() == 429 {
			return fmt.Errorf("%w: %v", ai.ErrTemporary, err)
		}
	}

	return err
}
