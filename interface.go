package aigentic

import "github.com/nexxia-ai/aigentic/ai"

// Retriever interface defines the contract for all retrievers
type Retriever interface {
	ToTool() ai.Tool
}

// Embedder interface defines the contract for text embedding
type Embedder interface {
	Embed(text string) ([]float64, error)
}
