package ai

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"math"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// Embedder defines the contract for computing vector embeddings for text passages.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
}

// GeminiEmbedder implements Embedder using Google's text-embedding-004 model.
type GeminiEmbedder struct {
	apiKey    string
	modelName string
}

// NewGeminiEmbedder creates a new GeminiEmbedder.
func NewGeminiEmbedder(apiKey, modelName string) *GeminiEmbedder {
	if modelName == "" {
		modelName = "gemini-embedding-001"
	}
	return &GeminiEmbedder{
		apiKey:    apiKey,
		modelName: modelName,
	}
}

func (g *GeminiEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	if g.apiKey == "" {
		return nil, errors.New("GEMINI_API_KEY is not set for embeddings")
	}

	client, err := genai.NewClient(ctx, option.WithAPIKey(g.apiKey))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize GenAI client: %w", err)
	}
	defer client.Close()

	em := client.EmbeddingModel(g.modelName)
	res, err := em.EmbedContent(ctx, genai.Text(text))
	if err != nil {
		return nil, fmt.Errorf("gemini embedding error: %w", err)
	}

	if res.Embedding == nil || len(res.Embedding.Values) == 0 {
		return nil, errors.New("empty embedding returned from Gemini")
	}

	return res.Embedding.Values, nil
}

func (g *GeminiEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	results := make([][]float32, len(texts))
	for i, text := range texts {
		emb, err := g.Embed(ctx, text)
		if err != nil {
			return nil, err
		}
		results[i] = emb
	}
	return results, nil
}

// MockEmbedder produces deterministic 768-dimensional normalized embeddings for testing and local use.
type MockEmbedder struct {
	Dimension int
	Err       error
}

func NewMockEmbedder() *MockEmbedder {
	return &MockEmbedder{Dimension: 768}
}

func (m *MockEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	if m.Err != nil {
		return nil, m.Err
	}

	dim := m.Dimension
	if dim <= 0 {
		dim = 768
	}

	h := sha256.Sum256([]byte(text))
	vec := make([]float32, dim)

	var normSq float64
	for i := 0; i < dim; i++ {
		byteVal := h[i%len(h)]
		v := float32(float64(byteVal)/255.0 - 0.5)
		vec[i] = v
		normSq += float64(v * v)
	}

	// Normalize to unit length
	norm := math.Sqrt(normSq)
	if norm > 0 {
		for i := range vec {
			vec[i] = float32(float64(vec[i]) / norm)
		}
	}

	return vec, nil
}

func (m *MockEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	results := make([][]float32, len(texts))
	for i, text := range texts {
		emb, err := m.Embed(ctx, text)
		if err != nil {
			return nil, err
		}
		results[i] = emb
	}
	return results, nil
}
