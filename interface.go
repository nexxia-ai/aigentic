package aigentic

// Retriever interface defines the contract for all retrievers
type Retriever interface {
	ToTool() AgentTool
}

// Embedder interface defines the contract for text embedding
type Embedder interface {
	Embed(text string) ([]float64, error)
}
