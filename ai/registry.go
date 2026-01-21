package ai

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"
)

var (
	ErrModelNotFound      = errors.New("model not found")
	ErrInvalidIdentifier  = errors.New("invalid model identifier format, expected 'provider/modelName'")
	ErrModelAlreadyExists = errors.New("model already registered")
	ErrEmptyIdentifier    = errors.New("identifier cannot be empty")
	ErrEmptyProvider      = errors.New("provider cannot be empty")
	ErrEmptyModel         = errors.New("model name cannot be empty")
)

type ModelFactoryFunc func(modelName, apiKey string, baseURL ...string) *Model

type ModelInfo struct {
	Identifier string
	Provider   string
	Model      string
	BaseURL    string
	Family     string
	APIKeyName string
	NewModel   ModelFactoryFunc
}

type modelRegistry struct {
	mu     sync.RWMutex
	models map[string]ModelInfo
}

var defaultRegistry *modelRegistry

func init() {
	defaultRegistry = &modelRegistry{
		models: make(map[string]ModelInfo),
	}
}

func RegisterModel(info ModelInfo) error {
	if info.Identifier == "" {
		return ErrEmptyIdentifier
	}
	if info.Provider == "" {
		return ErrEmptyProvider
	}
	if info.Model == "" {
		return ErrEmptyModel
	}
	if info.NewModel == nil {
		return fmt.Errorf("NewModel function cannot be nil")
	}
	if info.APIKeyName == "" {
		return fmt.Errorf("API key name cannot be empty")
	}

	defaultRegistry.mu.Lock()
	defer defaultRegistry.mu.Unlock()

	if _, exists := defaultRegistry.models[info.Identifier]; exists {
		slog.Warn("Overwriting model registration", "identifier", info.Identifier)
	}

	defaultRegistry.models[info.Identifier] = info
	return nil
}

func New(identifier, apiKey string) (*Model, error) {
	if identifier == "" {
		return nil, ErrInvalidIdentifier
	}

	defaultRegistry.mu.RLock()
	info, exists := defaultRegistry.models[identifier]
	defaultRegistry.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrModelNotFound, identifier)
	}

	return info.NewModel(info.Model, apiKey, info.BaseURL), nil
}

func Models() []ModelInfo {
	defaultRegistry.mu.RLock()
	defer defaultRegistry.mu.RUnlock()

	result := make([]ModelInfo, 0, len(defaultRegistry.models))
	for identifier := range defaultRegistry.models {
		info := defaultRegistry.models[identifier]
		result = append(result, info)
	}

	return result
}

func GetModelInfo(identifier string) (ModelInfo, error) {
	defaultRegistry.mu.RLock()
	defer defaultRegistry.mu.RUnlock()
	info, exists := defaultRegistry.models[identifier]
	if !exists {
		return ModelInfo{}, fmt.Errorf("%w: %s", ErrModelNotFound, identifier)
	}
	return info, nil
}
