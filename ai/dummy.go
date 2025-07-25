package ai

import (
	"context"
)

// NewDummyModel is useful for testing purposes. It allows you to mock the model's response.
func NewDummyModel(responseFunc func(messages []Message, tools []Tool) AIMessage) *Model {
	return &Model{
		ModelName: "dummy",
		callFunc: func(ctx context.Context, model *Model, messages []Message, tools []Tool) (AIMessage, error) {
			return responseFunc(messages, tools), nil
		},
	}
}
