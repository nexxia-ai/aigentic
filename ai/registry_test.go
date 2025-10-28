package ai

import (
	"context"
	"strings"
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
		BaseURL:  "https://api.test.com",
		NewModel: mockFactory,
	}

	err := RegisterModel(testProvider, testModel, info)
	if err != nil {
		t.Fatalf("Failed to register model: %v", err)
	}

	modelsList := Models()
	if len(modelsList) == 0 {
		t.Fatal("No models registered")
	}

	var retrievedInfo ModelInfo
	expectedIdentifier := testProvider + "/" + testModel
	for _, m := range modelsList {
		if m.Model == expectedIdentifier {
			retrievedInfo = m
			break
		}
	}

	if retrievedInfo.Model == "" {
		t.Fatal("Could not find registered model")
	}

	if retrievedInfo.Model == "" {
		t.Fatal("ModelInfo.Model field is empty")
	}

	expectedFullIdentifier := testProvider + "/" + testModel
	if retrievedInfo.Model != expectedFullIdentifier {
		t.Errorf("Expected ModelInfo.Model to be '%s', got: %s", expectedFullIdentifier, retrievedInfo.Model)
	}

	model, err := New(retrievedInfo.Model, "test-api-key")
	if err != nil {
		t.Errorf("Expected no error when using ModelInfo.Model with New(), got: %v", err)
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
		BaseURL:  "https://api.bug.com",
		NewModel: mockFactory,
	}

	err := RegisterModel(testProvider, testModel, info)
	if err != nil {
		t.Fatalf("Failed to register model: %v", err)
	}

	modelsList := Models()
	var retrievedInfo ModelInfo
	expectedIdentifier := testProvider + "/" + testModel
	for _, m := range modelsList {
		if m.Model == expectedIdentifier {
			retrievedInfo = m
			break
		}
	}

	if retrievedInfo.Model == "" {
		t.Fatal("Could not find registered model")
	}

	parts := strings.SplitN(retrievedInfo.Model, "/", 2)
	modelNameOnly := parts[1]
	_, err = New(modelNameOnly, "test-api-key")
	if err == nil {
		t.Error("Expected error when using ModelInfo.ModelName (without provider prefix) with New(), but got none")
	}
	if err != ErrInvalidIdentifier {
		t.Errorf("Expected ErrInvalidIdentifier, got: %v", err)
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
		BaseURL:  "https://api.another.com",
		NewModel: mockFactory,
	}

	err := RegisterModel(testProvider, testModel, info)
	if err != nil {
		t.Fatalf("Failed to register model: %v", err)
	}

	fullIdentifier := testProvider + "/" + testModel
	model, err := New(fullIdentifier, "test-api-key")
	if err != nil {
		t.Errorf("Expected no error with full identifier, got: %v", err)
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
		BaseURL:     "https://api.workflow.com",
		DisplayName: "Workflow Model",
		Family:      "test",
		NewModel:    mockFactory,
	}

	err := RegisterModel(testProvider, testModel, info)
	if err != nil {
		t.Fatalf("Failed to register model: %v", err)
	}

	allModels := Models()
	var targetModel ModelInfo
	expectedIdentifier := testProvider + "/" + testModel
	for _, m := range allModels {
		if m.Model == expectedIdentifier && m.DisplayName == "Workflow Model" {
			targetModel = m
			break
		}
	}

	if targetModel.Model == "" {
		t.Fatal("Could not find registered model")
	}

	if targetModel.Model != "workflow/workflow-model" {
		t.Errorf("Expected Model field to be 'workflow/workflow-model', got: %s", targetModel.Model)
	}

	model, err := New(targetModel.Model, "workflow-key")
	if err != nil {
		t.Errorf("Failed to create model using ModelInfo.Model: %v", err)
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
