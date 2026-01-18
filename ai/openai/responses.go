package openai

import (
	"context"
	"fmt"
	"strings"

	"github.com/nexxia-ai/aigentic/ai"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/shared"
)

func callResponsesAPI(ctx context.Context, client openai.Client, model *ai.Model, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
	inputItems, err := toResponsesInput(messages)
	if err != nil {
		return ai.AIMessage{}, fmt.Errorf("failed to convert messages: %w", err)
	}

	params := responses.ResponseNewParams{
		Model: shared.ResponsesModel(model.ModelName),
		Input: responses.ResponseNewParamsInputUnion{
			OfInputItemList: inputItems,
		},
	}

	if len(tools) > 0 {
		respTools := toResponsesTools(tools)
		params.Tools = respTools
	}

	if model.Temperature != nil {
		params.Temperature = openai.Opt(*model.Temperature)
	}
	if model.MaxTokens != nil {
		params.MaxOutputTokens = openai.Opt(int64(*model.MaxTokens))
	}
	if model.TopP != nil {
		params.TopP = openai.Opt(*model.TopP)
	}
	// Responses API does not support stop sequences in the same way as Chat API
	// Stop sequences would need to be handled differently if needed

	resp, err := client.Responses.New(ctx, params)
	if err != nil {
		return ai.AIMessage{}, isRetryableError(err)
	}

	aiMsg := fromResponsesOutput(resp)
	content, thinkPart := ai.ExtractThinkTags(aiMsg.Content)
	aiMsg.Content = content
	aiMsg.Think = thinkPart

	return aiMsg, nil
}

func streamResponsesAPI(ctx context.Context, client openai.Client, model *ai.Model, messages []ai.Message, tools []ai.Tool, chunkFunction func(ai.AIMessage) error) (ai.AIMessage, error) {
	inputItems, err := toResponsesInput(messages)
	if err != nil {
		return ai.AIMessage{}, fmt.Errorf("failed to convert messages: %w", err)
	}

	params := responses.ResponseNewParams{
		Model: shared.ResponsesModel(model.ModelName),
		Input: responses.ResponseNewParamsInputUnion{
			OfInputItemList: inputItems,
		},
	}

	if len(tools) > 0 {
		respTools := toResponsesTools(tools)
		params.Tools = respTools
	}

	if model.Temperature != nil {
		params.Temperature = openai.Opt(*model.Temperature)
	}
	if model.MaxTokens != nil {
		params.MaxOutputTokens = openai.Opt(int64(*model.MaxTokens))
	}
	if model.TopP != nil {
		params.TopP = openai.Opt(*model.TopP)
	}
	// Responses API does not support stop sequences in the same way as Chat API
	// Stop sequences would need to be handled differently if needed

	stream := client.Responses.NewStreaming(ctx, params)
	defer stream.Close()

	var finalMessage ai.AIMessage
	var accumulatedContent strings.Builder
	var accumulatedThink strings.Builder
	var toolCallsMap = make(map[string]*ai.ToolCall)
	var responseID string
	var responseCreated int64
	var responseModel string
	parser := &streamingThinkParser{}

	for stream.Next() {
		event := stream.Current()

		switch event.Type {
		case "response.created":
			evt := event.AsResponseCreated()
			responseID = evt.Response.ID
			responseCreated = int64(evt.Response.CreatedAt)
			responseModel = string(evt.Response.Model)

		case "response.output_text.delta":
			evt := event.AsResponseOutputTextDelta()
			contentForChunk, thinkForChunk := parser.addChunk(evt.Delta)
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

		case "response.function_call_arguments.delta":
			// Delta events use ItemID which is the item ID, not CallID
			// We'll capture tool calls from output_item.added and response.completed instead
			break

		case "response.function_call_arguments.done":
			// Done events use ItemID which is the item ID, not CallID
			// We'll capture tool calls from output_item.added and response.completed instead
			break

		case "response.output_item.added":
			evt := event.AsResponseOutputItemAdded()
			if evt.Item.Type == "function_call" {
				// Use CallID as the tool call ID - this is what we need for function_call_output
				toolCallsMap[evt.Item.CallID] = &ai.ToolCall{
					ID:   evt.Item.CallID,
					Type: "function",
					Name: evt.Item.Name,
					Args: evt.Item.Arguments,
				}
			}

		case "response.completed":
			evt := event.AsResponseCompleted()
			responseID = evt.Response.ID
			responseCreated = int64(evt.Response.CreatedAt)
			responseModel = string(evt.Response.Model)
			// Extract tool calls from completed response
			if len(evt.Response.Output) > 0 {
				for _, outputItem := range evt.Response.Output {
					if outputItem.Type == "function_call" {
						toolCallsMap[outputItem.CallID] = &ai.ToolCall{
							ID:   outputItem.CallID,
							Type: "function",
							Name: outputItem.Name,
							Args: outputItem.Arguments,
						}
					}
				}
			}
			break
		case "response.output_item.done":
			// Handle individual output items being done
			break
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
	for _, toolCall := range toolCallsMap {
		finalToolCalls = append(finalToolCalls, *toolCall)
	}
	finalMessage.ToolCalls = finalToolCalls

	if responseID != "" {
		finalMessage.Response = ai.Response{
			ID:      responseID,
			Object:  "response",
			Created: responseCreated,
			Model:   responseModel,
		}
	}

	return finalMessage, nil
}

type streamingThinkParser struct {
	buffer     string
	inThinkTag bool
}

func (p *streamingThinkParser) addChunk(rawChunk string) (contentChunk string, thinkChunk string) {
	p.buffer += rawChunk

	for {
		if !p.inThinkTag {
			startIdx := strings.Index(p.buffer, "<think>")
			if startIdx == -1 {
				contentChunk = p.buffer
				p.buffer = ""
				return contentChunk, ""
			}

			if startIdx > 0 {
				contentChunk = p.buffer[:startIdx]
				p.buffer = p.buffer[startIdx:]
				return contentChunk, ""
			}

			if len(p.buffer) >= len("<think>") {
				p.inThinkTag = true
				p.buffer = p.buffer[len("<think>"):]
				continue
			}

			return "", ""
		} else {
			endIdx := strings.Index(p.buffer, "</think>")
			if endIdx == -1 {
				if len(p.buffer) > 0 {
					thinkChunk = p.buffer
					p.buffer = ""
				}
				return "", thinkChunk
			}

			thinkChunk = p.buffer[:endIdx]
			p.buffer = p.buffer[endIdx+len("</think>"):]
			p.inThinkTag = false
			return "", thinkChunk
		}
	}
}

func (p *streamingThinkParser) flush() (contentChunk string, thinkChunk string) {
	if p.inThinkTag {
		return "", p.buffer
	}
	return p.buffer, ""
}
