package aigentic

type Embedder interface {
	Embed(text string) ([]float64, error)
}
