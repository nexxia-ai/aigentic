package openai

import (
	"github.com/nexxia-ai/aigentic/ai"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/shared"
)

func toChatTools(tools []ai.Tool) []openai.ChatCompletionToolUnionParam {
	if len(tools) == 0 {
		return nil
	}
	result := make([]openai.ChatCompletionToolUnionParam, len(tools))
	for i, tool := range tools {
		result[i] = openai.ChatCompletionToolUnionParam{
			OfFunction: &openai.ChatCompletionFunctionToolParam{
				Function: shared.FunctionDefinitionParam{
					Name:        tool.Name,
					Description: openai.Opt(tool.Description),
					Parameters:  tool.InputSchema,
				},
			},
		}
	}
	return result
}

func toResponsesTools(tools []ai.Tool) []responses.ToolUnionParam {
	if len(tools) == 0 {
		return nil
	}
	result := make([]responses.ToolUnionParam, len(tools))
	for i, tool := range tools {
		result[i] = responses.ToolUnionParam{
			OfFunction: &responses.FunctionToolParam{
				Name:        tool.Name,
				Description: openai.Opt(tool.Description),
				Parameters:  tool.InputSchema,
			},
		}
	}
	return result
}
