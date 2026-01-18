package openai

import (
	"context"
	"fmt"
	"strings"

	"github.com/nexxia-ai/aigentic/ai"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/shared"
)

func callChatAPI(ctx context.Context, client openai.Client, model *ai.Model, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
	chatMsgs, err := toChatMessages(messages)
	if err != nil {
		return ai.AIMessage{}, fmt.Errorf("failed to convert messages: %w", err)
	}

	params := openai.ChatCompletionNewParams{
		Model:    shared.ChatModel(model.ModelName),
		Messages: chatMsgs,
	}

	if len(tools) > 0 {
		chatTools := toChatTools(tools)
		params.Tools = chatTools
	}

	if model.Temperature != nil {
		params.Temperature = openai.Opt(*model.Temperature)
	}
	if model.MaxTokens != nil {
		params.MaxTokens = openai.Opt(int64(*model.MaxTokens))
	}
	if model.TopP != nil {
		params.TopP = openai.Opt(*model.TopP)
	}
	if model.FrequencyPenalty != nil {
		params.FrequencyPenalty = openai.Opt(*model.FrequencyPenalty)
	}
	if model.PresencePenalty != nil {
		params.PresencePenalty = openai.Opt(*model.PresencePenalty)
	}
	if model.StopSequences != nil && len(*model.StopSequences) > 0 {
		stopSeqs := *model.StopSequences
		if len(stopSeqs) == 1 {
			params.Stop = openai.ChatCompletionNewParamsStopUnion{
				OfString: openai.Opt(stopSeqs[0]),
			}
		} else {
			params.Stop = openai.ChatCompletionNewParamsStopUnion{
				OfStringArray: stopSeqs,
			}
		}
	}

	resp, err := client.Chat.Completions.New(ctx, params)
	if err != nil {
		return ai.AIMessage{}, isRetryableError(err)
	}

	aiMsg := fromChatResponse(resp, 0)
	content, thinkPart := ai.ExtractThinkTags(aiMsg.Content)
	aiMsg.Content = content
	aiMsg.Think = thinkPart

	return aiMsg, nil
}

func streamChatAPI(ctx context.Context, client openai.Client, model *ai.Model, messages []ai.Message, tools []ai.Tool, chunkFunction func(ai.AIMessage) error) (ai.AIMessage, error) {
	chatMsgs, err := toChatMessages(messages)
	if err != nil {
		return ai.AIMessage{}, fmt.Errorf("failed to convert messages: %w", err)
	}

	params := openai.ChatCompletionNewParams{
		Model:    shared.ChatModel(model.ModelName),
		Messages: chatMsgs,
	}

	if len(tools) > 0 {
		chatTools := toChatTools(tools)
		params.Tools = chatTools
	}

	if model.Temperature != nil {
		params.Temperature = openai.Opt(*model.Temperature)
	}
	if model.MaxTokens != nil {
		params.MaxTokens = openai.Opt(int64(*model.MaxTokens))
	}
	if model.TopP != nil {
		params.TopP = openai.Opt(*model.TopP)
	}
	if model.FrequencyPenalty != nil {
		params.FrequencyPenalty = openai.Opt(*model.FrequencyPenalty)
	}
	if model.PresencePenalty != nil {
		params.PresencePenalty = openai.Opt(*model.PresencePenalty)
	}
	if model.StopSequences != nil && len(*model.StopSequences) > 0 {
		stopSeqs := *model.StopSequences
		if len(stopSeqs) == 1 {
			params.Stop = openai.ChatCompletionNewParamsStopUnion{
				OfString: openai.Opt(stopSeqs[0]),
			}
		} else {
			params.Stop = openai.ChatCompletionNewParamsStopUnion{
				OfStringArray: stopSeqs,
			}
		}
	}

	stream := client.Chat.Completions.NewStreaming(ctx, params)
	defer stream.Close()

	var finalMessage ai.AIMessage
	var accumulatedContent strings.Builder
	var accumulatedThink strings.Builder
	var toolCallsMap = make(map[int]*ai.ToolCall)
	var responseID string
	var responseCreated int64
	var responseModel string
	parser := &streamingThinkParser{}

	for stream.Next() {
		chunk := stream.Current()
		if chunk.ID != "" && responseID == "" {
			responseID = chunk.ID
		}
		if chunk.Created != 0 && responseCreated == 0 {
			responseCreated = chunk.Created
		}
		if chunk.Model != "" && responseModel == "" {
			responseModel = string(chunk.Model)
		}

		if len(chunk.Choices) > 0 {
			choice := chunk.Choices[0]
			delta := choice.Delta
			if delta.Content != "" {
				contentForChunk, thinkForChunk := parser.addChunk(delta.Content)
				accumulatedContent.WriteString(contentForChunk)
				accumulatedThink.WriteString(thinkForChunk)

				if contentForChunk != "" || thinkForChunk != "" {
					partialMessage := ai.AIMessage{
						Role:    ai.AssistantRole,
						Content: contentForChunk,
						Think:   thinkForChunk,
					}
					if err := chunkFunction(partialMessage); err != nil {
						return ai.AIMessage{}, err
					}
				}
			}

			if len(delta.ToolCalls) > 0 {
				for _, deltaToolCall := range delta.ToolCalls {
					index := int(deltaToolCall.Index)
					if toolCallsMap[index] == nil {
						toolCallsMap[index] = &ai.ToolCall{
							ID:   deltaToolCall.ID,
							Type: string(deltaToolCall.Type),
							Name: deltaToolCall.Function.Name,
						}
					}
					if deltaToolCall.Function.Arguments != "" {
						toolCallsMap[index].Args += deltaToolCall.Function.Arguments
					}
				}
			}

			if delta.Role != "" && finalMessage.Role == "" {
				finalMessage.Role = ai.MessageRole(delta.Role)
			}

			if choice.FinishReason != "" {
				break
			}
		}
	}

	if err := stream.Err(); err != nil {
		return ai.AIMessage{}, isRetryableError(err)
	}

	flushContent, flushThink := parser.flush()
	if flushContent != "" {
		accumulatedContent.WriteString(flushContent)
	}
	if flushThink != "" {
		accumulatedThink.WriteString(flushThink)
	}

	finalMessage.Content = accumulatedContent.String()
	finalMessage.Think = accumulatedThink.String()

	var finalToolCalls []ai.ToolCall
	for i := 0; i < len(toolCallsMap); i++ {
		if toolCall, exists := toolCallsMap[i]; exists {
			finalToolCalls = append(finalToolCalls, *toolCall)
		}
	}
	finalMessage.ToolCalls = finalToolCalls

	if responseID != "" {
		finalMessage.Response = ai.Response{
			ID:      responseID,
			Object:  "chat.completion",
			Created: responseCreated,
			Model:   responseModel,
		}
	}

	return finalMessage, nil
}
