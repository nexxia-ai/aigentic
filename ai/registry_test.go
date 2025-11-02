package ai

import (
	"context"
	"errors"
	"testing"
)

func TestNew_WithModelInfoModel_ShouldWork(t *testing.T) {
	mockFactory := func(modelName, apiKey string, baseURL ...string) *Model {
		url := ""
		if len(baseURL) > 0 {
			url = baseURL[0]
		}
		return &Model{
			ModelName: modelName,
			APIKey:    apiKey,
			BaseURL:   url,
			callFunc: func(ctx context.Context, model *Model, messages []Message, tools []Tool) (AIMessage, error) {
				return AIMessage{Content: "test"}, nil
			},
		}
	}

	testProvider := "testprovider"
	testModel := "test-model"

	info := ModelInfo{
		Provider:   testProvider,
		Model:      testModel,
		Identifier: "Test Model",
		BaseURL:    "https://api.test.com",
		NewModel:   mockFactory,
	}

	err := RegisterModel(info)
	if err != nil {
		t.Fatalf("Failed to register model: %v", err)
	}

	modelsList := Models()
	if len(modelsList) == 0 {
		t.Fatal("No models registered")
	}

	var retrievedInfo ModelInfo
	for _, m := range modelsList {
		if m.Identifier == info.Identifier {
			retrievedInfo = m
			break
		}
	}

	if retrievedInfo.Identifier == "" {
		t.Fatal("Could not find registered model")
	}

	if retrievedInfo.Model != testModel {
		t.Errorf("Expected ModelInfo.Model to be '%s', got: %s", testModel, retrievedInfo.Model)
	}

	if retrievedInfo.Provider != testProvider {
		t.Errorf("Expected ModelInfo.Provider to be '%s', got: %s", testProvider, retrievedInfo.Provider)
	}

	model, err := New(info.Identifier, "test-api-key")
	if err != nil {
		t.Errorf("Expected no error when using Identifier with New(), got: %v", err)
	}
	if model == nil {
		t.Error("Expected model to be created, got nil")
	}
	if model != nil && model.APIKey != "test-api-key" {
		t.Errorf("Expected APIKey to be 'test-api-key', got: %s", model.APIKey)
	}
}

func TestNew_WithModelInfoModelName_ShouldFail(t *testing.T) {
	mockFactory := func(modelName, apiKey string, baseURL ...string) *Model {
		return &Model{
			ModelName: modelName,
			APIKey:    apiKey,
		}
	}

	testProvider := "bugprovider"
	testModel := "bug-model"

	info := ModelInfo{
		Provider:   testProvider,
		Model:      testModel,
		Identifier: "Bug Model",
		BaseURL:    "https://api.bug.com",
		NewModel:   mockFactory,
	}

	err := RegisterModel(info)
	if err != nil {
		t.Fatalf("Failed to register model: %v", err)
	}

	modelsList := Models()
	var retrievedInfo ModelInfo
	for _, m := range modelsList {
		if m.Identifier == info.Identifier {
			retrievedInfo = m
			break
		}
	}

	if retrievedInfo.Identifier == "" {
		t.Fatal("Could not find registered model")
	}

	_, err = New(info.Model, "test-api-key")
	if err == nil {
		t.Error("Expected error when using model name (not identifier) with New(), but got none")
	}
	if !errors.Is(err, ErrModelNotFound) {
		t.Errorf("Expected ErrModelNotFound, got: %v", err)
	}
}

func TestNew_WithFullIdentifier_ShouldWork(t *testing.T) {
	mockFactory := func(modelName, apiKey string, baseURL ...string) *Model {
		url := ""
		if len(baseURL) > 0 {
			url = baseURL[0]
		}
		return &Model{
			ModelName: modelName,
			APIKey:    apiKey,
			BaseURL:   url,
			callFunc: func(ctx context.Context, model *Model, messages []Message, tools []Tool) (AIMessage, error) {
				return AIMessage{Content: "test"}, nil
			},
		}
	}

	testProvider := "anotherprovider"
	testModel := "another-model"

	info := ModelInfo{
		Provider:   testProvider,
		Model:      testModel,
		Identifier: "Another Model",
		BaseURL:    "https://api.another.com",
		NewModel:   mockFactory,
	}

	err := RegisterModel(info)
	if err != nil {
		t.Fatalf("Failed to register model: %v", err)
	}

	model, err := New(info.Identifier, "test-api-key")
	if err != nil {
		t.Errorf("Expected no error with identifier, got: %v", err)
	}
	if model == nil {
		t.Error("Expected model to be created, got nil")
	}
	if model != nil && model.APIKey != "test-api-key" {
		t.Errorf("Expected APIKey to be 'test-api-key', got: %s", model.APIKey)
	}
}

func TestModelInfo_CompleteWorkflow(t *testing.T) {
	mockFactory := func(modelName, apiKey string, baseURL ...string) *Model {
		url := ""
		if len(baseURL) > 0 {
			url = baseURL[0]
		}
		return &Model{
			ModelName: modelName,
			APIKey:    apiKey,
			BaseURL:   url,
			callFunc: func(ctx context.Context, model *Model, messages []Message, tools []Tool) (AIMessage, error) {
				return AIMessage{Content: "response"}, nil
			},
		}
	}

	testProvider := "workflow"
	testModel := "workflow-model"

	info := ModelInfo{
		Provider:   testProvider,
		Model:      testModel,
		Identifier: "Workflow Model",
		BaseURL:    "https://api.workflow.com",
		Family:     "test",
		NewModel:   mockFactory,
	}

	err := RegisterModel(info)
	if err != nil {
		t.Fatalf("Failed to register model: %v", err)
	}

	allModels := Models()
	var targetModel ModelInfo
	for _, m := range allModels {
		if m.Identifier == "Workflow Model" {
			targetModel = m
			break
		}
	}

	if targetModel.Identifier == "" {
		t.Fatal("Could not find registered model")
	}

	if targetModel.Model != "workflow-model" {
		t.Errorf("Expected Model field to be 'workflow-model', got: %s", targetModel.Model)
	}

	if targetModel.Provider != "workflow" {
		t.Errorf("Expected Provider field to be 'workflow', got: %s", targetModel.Provider)
	}

	model, err := New(info.Identifier, "workflow-key")
	if err != nil {
		t.Errorf("Failed to create model using Identifier: %v", err)
	}

	if model == nil {
		t.Fatal("Model is nil")
	}

	if model.APIKey != "workflow-key" {
		t.Errorf("Expected APIKey to be 'workflow-key', got: %s", model.APIKey)
	}

	if model.BaseURL != "https://api.workflow.com" {
		t.Errorf("Expected BaseURL to be 'https://api.workflow.com', got: %s", model.BaseURL)
	}

	response, err := model.Call(context.Background(), []Message{UserMessage{Role: UserRole, Content: "test"}}, nil)
	if err != nil {
		t.Errorf("Model.Call() failed: %v", err)
	}

	if response.Content != "response" {
		t.Errorf("Expected response 'response', got: %s", response.Content)
	}
}
