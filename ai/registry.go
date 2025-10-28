package ai

import (
	"errors"
	"fmt"
	"strings"
	"sync"
)

var (
	ErrModelNotFound      = errors.New("model not found")
	ErrInvalidIdentifier  = errors.New("invalid model identifier format, expected 'provider/modelName'")
	ErrModelAlreadyExists = errors.New("model already registered")
	ErrEmptyProviderName  = errors.New("provider name cannot be empty")
	ErrEmptyModelName     = errors.New("model name cannot be empty")
)

type ModelFactoryFunc func(modelName, apiKey string, baseURL ...string) *Model

type ModelInfo struct {
	Model       string
	BaseURL     string
	DisplayName string
	Family      string
	NewModel    ModelFactoryFunc
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

func RegisterModel(provider, modelName string, info ModelInfo) error {
	if provider == "" {
		return ErrEmptyProviderName
	}
	if modelName == "" {
		return ErrEmptyModelName
	}

	identifier := fmt.Sprintf("%s/%s", provider, modelName)

	defaultRegistry.mu.Lock()
	defer defaultRegistry.mu.Unlock()

	if _, exists := defaultRegistry.models[identifier]; exists {
		return fmt.Errorf("%w: %s", ErrModelAlreadyExists, identifier)
	}

	info.Model = identifier
	defaultRegistry.models[identifier] = info
	return nil
}

func New(identifier, apiKey string) (*Model, error) {
	if identifier == "" {
		return nil, ErrInvalidIdentifier
	}

	parts := strings.SplitN(identifier, "/", 2)
	if len(parts) < 2 {
		return nil, ErrInvalidIdentifier
	}

	provider := parts[0]
	modelName := parts[1]

	if provider == "" || modelName == "" {
		return nil, ErrInvalidIdentifier
	}

	defaultRegistry.mu.RLock()
	info, exists := defaultRegistry.models[identifier]
	defaultRegistry.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrModelNotFound, identifier)
	}

	return info.NewModel(modelName, apiKey, info.BaseURL), nil
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
