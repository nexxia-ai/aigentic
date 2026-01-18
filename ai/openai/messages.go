package openai

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/nexxia-ai/aigentic/ai"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/responses"
)

func toChatMessages(msgs []ai.Message) ([]openai.ChatCompletionMessageParamUnion, error) {
	result := make([]openai.ChatCompletionMessageParamUnion, 0, len(msgs))
	for _, msg := range msgs {
		switch m := msg.(type) {
		case ai.UserMessage:
			chatMsg, err := toChatUserMessage(m)
			if err != nil {
				return nil, err
			}
			result = append(result, chatMsg)
		case ai.SystemMessage:
			chatMsg, err := toChatSystemMessage(m)
			if err != nil {
				return nil, err
			}
			result = append(result, chatMsg)
		case ai.AIMessage:
			chatMsg, err := toChatAssistantMessage(m)
			if err != nil {
				return nil, err
			}
			result = append(result, chatMsg)
		case ai.ToolMessage:
			chatMsg := toChatToolMessage(m)
			result = append(result, chatMsg)
		default:
			return nil, fmt.Errorf("unsupported message type: %T", msg)
		}
	}
	return result, nil
}

func toChatUserMessage(msg ai.UserMessage) (openai.ChatCompletionMessageParamUnion, error) {
	if len(msg.Parts) > 0 {
		parts := make([]openai.ChatCompletionContentPartUnionParam, 0, len(msg.Parts))
		for _, part := range msg.Parts {
			switch part.Type {
			case ai.ContentPartText:
				parts = append(parts, openai.ChatCompletionContentPartUnionParam{
					OfText: &openai.ChatCompletionContentPartTextParam{
						Text: part.Text,
					},
				})
			case ai.ContentPartImage, ai.ContentPartImageURL:
				var imageURL string
				if part.URI != "" {
					imageURL = part.URI
				} else if len(part.Data) > 0 {
					imageURL = fmt.Sprintf("data:%s;base64,%s", part.MimeType, base64.StdEncoding.EncodeToString(part.Data))
				} else {
					return openai.ChatCompletionMessageParamUnion{}, fmt.Errorf("image part missing URI or data")
				}
				parts = append(parts, openai.ImageContentPart(openai.ChatCompletionContentPartImageImageURLParam{
					URL: imageURL,
				}))
			case ai.ContentPartFile, ai.ContentPartInputFile:
				if part.FileID == "" {
					return openai.ChatCompletionMessageParamUnion{}, fmt.Errorf("file part missing file_id")
				}
				parts = append(parts, openai.FileContentPart(openai.ChatCompletionContentPartFileFileParam{
					FileID: openai.Opt(part.FileID),
				}))
			}
		}
		return openai.ChatCompletionMessageParamUnion{
			OfUser: &openai.ChatCompletionUserMessageParam{
				Content: openai.ChatCompletionUserMessageParamContentUnion{
					OfArrayOfContentParts: parts,
				},
			},
		}, nil
	}
	return openai.ChatCompletionMessageParamUnion{
		OfUser: &openai.ChatCompletionUserMessageParam{
			Content: openai.ChatCompletionUserMessageParamContentUnion{
				OfString: openai.String(msg.Content),
			},
		},
	}, nil
}

func toChatSystemMessage(msg ai.SystemMessage) (openai.ChatCompletionMessageParamUnion, error) {
	if len(msg.Parts) > 0 {
		textParts := make([]string, 0)
		for _, part := range msg.Parts {
			if part.Type == ai.ContentPartText {
				textParts = append(textParts, part.Text)
			}
		}
		content := strings.Join(textParts, "\n")
		return openai.ChatCompletionMessageParamUnion{
			OfSystem: &openai.ChatCompletionSystemMessageParam{
				Content: openai.ChatCompletionSystemMessageParamContentUnion{
					OfString: openai.Opt(content),
				},
			},
		}, nil
	}
	return openai.ChatCompletionMessageParamUnion{
		OfSystem: &openai.ChatCompletionSystemMessageParam{
			Content: openai.ChatCompletionSystemMessageParamContentUnion{
				OfString: openai.Opt(msg.Content),
			},
		},
	}, nil
}

func toChatAssistantMessage(msg ai.AIMessage) (openai.ChatCompletionMessageParamUnion, error) {
	assistantMsg := &openai.ChatCompletionAssistantMessageParam{
		Content: openai.ChatCompletionAssistantMessageParamContentUnion{
			OfString: openai.Opt(msg.Content),
		},
	}
	if len(msg.ToolCalls) > 0 {
		toolCalls := make([]openai.ChatCompletionMessageToolCallUnionParam, len(msg.ToolCalls))
		for i, tc := range msg.ToolCalls {
			toolCalls[i] = openai.ChatCompletionMessageToolCallUnionParam{
				OfFunction: &openai.ChatCompletionMessageFunctionToolCallParam{
					ID: tc.ID,
					Function: openai.ChatCompletionMessageFunctionToolCallFunctionParam{
						Name:      tc.Name,
						Arguments: tc.Args,
					},
				},
			}
		}
		assistantMsg.ToolCalls = toolCalls
	}
	return openai.ChatCompletionMessageParamUnion{
		OfAssistant: assistantMsg,
	}, nil
}

func toChatToolMessage(msg ai.ToolMessage) openai.ChatCompletionMessageParamUnion {
	return openai.ChatCompletionMessageParamUnion{
		OfTool: &openai.ChatCompletionToolMessageParam{
			Content: openai.ChatCompletionToolMessageParamContentUnion{
				OfString: openai.Opt(msg.Content),
			},
			ToolCallID: msg.ToolCallID,
		},
	}
}

func fromChatResponse(resp *openai.ChatCompletion, choiceIndex int) ai.AIMessage {
	if len(resp.Choices) <= choiceIndex {
		return ai.AIMessage{}
	}
	choice := resp.Choices[choiceIndex]
	msg := choice.Message

	aiMsg := ai.AIMessage{
		Role:    ai.AssistantRole,
		Content: msg.Content,
	}

	if len(msg.ToolCalls) > 0 {
		aiMsg.ToolCalls = make([]ai.ToolCall, len(msg.ToolCalls))
		for i, tc := range msg.ToolCalls {
			aiMsg.ToolCalls[i] = ai.ToolCall{
				ID:   tc.ID,
				Type: string(tc.Type),
				Name: tc.Function.Name,
				Args: tc.Function.Arguments,
			}
		}
	}

	aiMsg.Response = ai.Response{
		ID:      resp.ID,
		Object:  string(resp.Object),
		Created: resp.Created,
		Model:   string(resp.Model),
		Usage: ai.Usage{
			PromptTokens:     int(resp.Usage.PromptTokens),
			CompletionTokens: int(resp.Usage.CompletionTokens),
			TotalTokens:      int(resp.Usage.TotalTokens),
		},
	}

	return aiMsg
}

func toResponsesInput(msgs []ai.Message) (responses.ResponseInputParam, error) {
	result := make(responses.ResponseInputParam, 0, len(msgs))
	for _, msg := range msgs {
		switch m := msg.(type) {
		case ai.UserMessage:
			item, err := toResponsesUserItem(m)
			if err != nil {
				return nil, err
			}
			result = append(result, item)
		case ai.SystemMessage:
			item, err := toResponsesSystemItem(m)
			if err != nil {
				return nil, err
			}
			result = append(result, item)
		case ai.AIMessage:
			item, err := toResponsesAssistantItem(m)
			if err != nil {
				return nil, err
			}
			result = append(result, item)
			// Responses API requires function calls to be separate items
			if len(m.ToolCalls) > 0 {
				for _, tc := range m.ToolCalls {
					result = append(result, responses.ResponseInputItemUnionParam{
						OfFunctionCall: &responses.ResponseFunctionToolCallParam{
							CallID:    tc.ID,
							Name:      tc.Name,
							Arguments: tc.Args,
						},
					})
				}
			}
		case ai.ToolMessage:
			item := toResponsesToolItem(m)
			result = append(result, item)
		default:
			return nil, fmt.Errorf("unsupported message type: %T", msg)
		}
	}
	return result, nil
}

func toResponsesUserItem(msg ai.UserMessage) (responses.ResponseInputItemUnionParam, error) {
	if len(msg.Parts) > 0 {
		parts := make([]responses.ResponseInputContentUnionParam, 0, len(msg.Parts))
		for _, part := range msg.Parts {
			switch part.Type {
			case ai.ContentPartText:
				parts = append(parts, responses.ResponseInputContentParamOfInputText(part.Text))
			case ai.ContentPartImage, ai.ContentPartImageURL:
				var imageURL string
				if part.URI != "" {
					imageURL = part.URI
				} else if len(part.Data) > 0 {
					imageURL = fmt.Sprintf("data:%s;base64,%s", part.MimeType, base64.StdEncoding.EncodeToString(part.Data))
				} else {
					return responses.ResponseInputItemUnionParam{}, fmt.Errorf("image part missing URI or data")
				}
				parts = append(parts, responses.ResponseInputContentUnionParam{
					OfInputImage: &responses.ResponseInputImageParam{
						ImageURL: openai.Opt(imageURL),
						Detail:   "auto",
					},
				})
			case ai.ContentPartAudio:
				return responses.ResponseInputItemUnionParam{}, fmt.Errorf("audio content parts not yet supported in Responses API input")
			case ai.ContentPartVideo:
				return responses.ResponseInputItemUnionParam{}, fmt.Errorf("video content parts not yet supported in Responses API input")
			case ai.ContentPartFile, ai.ContentPartInputFile:
				if part.FileID == "" {
					return responses.ResponseInputItemUnionParam{}, fmt.Errorf("file part missing file_id")
				}
				parts = append(parts, responses.ResponseInputContentUnionParam{
					OfInputFile: &responses.ResponseInputFileParam{
						FileID: openai.Opt(part.FileID),
					},
				})
			}
		}
		return responses.ResponseInputItemUnionParam{
			OfInputMessage: &responses.ResponseInputItemMessageParam{
				Role:    "user",
				Content: responses.ResponseInputMessageContentListParam(parts),
			},
		}, nil
	}
	contentParts := []responses.ResponseInputContentUnionParam{
		responses.ResponseInputContentParamOfInputText(msg.Content),
	}
	return responses.ResponseInputItemUnionParam{
		OfInputMessage: &responses.ResponseInputItemMessageParam{
			Role:    "user",
			Content: responses.ResponseInputMessageContentListParam(contentParts),
		},
	}, nil
}

func toResponsesSystemItem(msg ai.SystemMessage) (responses.ResponseInputItemUnionParam, error) {
	if len(msg.Parts) > 0 {
		textParts := make([]string, 0)
		for _, part := range msg.Parts {
			if part.Type == ai.ContentPartText {
				textParts = append(textParts, part.Text)
			}
		}
		content := strings.Join(textParts, "\n")
		textContent := []responses.ResponseInputContentUnionParam{
			responses.ResponseInputContentParamOfInputText(content),
		}
		return responses.ResponseInputItemUnionParam{
			OfInputMessage: &responses.ResponseInputItemMessageParam{
				Role:    "system",
				Content: responses.ResponseInputMessageContentListParam(textContent),
			},
		}, nil
	}
	textContent := []responses.ResponseInputContentUnionParam{
		responses.ResponseInputContentParamOfInputText(msg.Content),
	}
	return responses.ResponseInputItemUnionParam{
		OfInputMessage: &responses.ResponseInputItemMessageParam{
			Role:    "system",
			Content: responses.ResponseInputMessageContentListParam(textContent),
		},
	}, nil
}

func toResponsesAssistantItem(msg ai.AIMessage) (responses.ResponseInputItemUnionParam, error) {
	return responses.ResponseInputItemUnionParam{
		OfOutputMessage: &responses.ResponseOutputMessageParam{
			Content: []responses.ResponseOutputMessageContentUnionParam{
				{
					OfOutputText: &responses.ResponseOutputTextParam{
						Text:        msg.Content,
						Annotations: []responses.ResponseOutputTextAnnotationUnionParam{},
					},
				},
			},
		},
	}, nil
}

func toResponsesToolItem(msg ai.ToolMessage) responses.ResponseInputItemUnionParam {
	return responses.ResponseInputItemUnionParam{
		OfFunctionCallOutput: &responses.ResponseInputItemFunctionCallOutputParam{
			CallID: msg.ToolCallID,
			Output: responses.ResponseInputItemFunctionCallOutputOutputUnionParam{
				OfString: openai.Opt(msg.Content),
			},
		},
	}
}

func fromResponsesOutput(resp *responses.Response) ai.AIMessage {
	aiMsg := ai.AIMessage{
		Role: ai.AssistantRole,
	}

	textOutput := resp.OutputText()
	if textOutput != "" {
		aiMsg.Content = textOutput
	}

	var toolCalls []ai.ToolCall
	if len(resp.Output) > 0 {
		for _, outputItem := range resp.Output {
			if outputItem.Type == "message" && outputItem.Role == "assistant" {
				if outputItem.Content != nil {
					for _, contentItem := range outputItem.Content {
						if contentItem.Type == "output_text" && contentItem.Text != "" {
							if aiMsg.Content != "" {
								aiMsg.Content += "\n"
							}
							aiMsg.Content += contentItem.Text
						}
					}
				}
			} else if outputItem.Type == "function_call" {
				toolCalls = append(toolCalls, ai.ToolCall{
					ID:   outputItem.CallID,
					Type: "function",
					Name: outputItem.Name,
					Args: outputItem.Arguments,
				})
			}
		}
	}
	aiMsg.ToolCalls = toolCalls

	aiMsg.Response = ai.Response{
		ID:      resp.ID,
		Object:  "response",
		Created: int64(resp.CreatedAt),
		Model:   string(resp.Model),
		Usage: ai.Usage{
			PromptTokens:     int(resp.Usage.InputTokens),
			CompletionTokens: int(resp.Usage.OutputTokens),
			TotalTokens:      int(resp.Usage.TotalTokens),
		},
	}

	return aiMsg
}
