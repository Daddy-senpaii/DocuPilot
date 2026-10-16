package ai

import (
	"context"
	"errors"
	"math"
	"testing"
)

func TestMockEmbedderDeterministic(t *testing.T) {
	mock := NewMockEmbedder()
	ctx := context.Background()

	vec1, err := mock.Embed(ctx, "hello world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	vec2, err := mock.Embed(ctx, "hello world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(vec1) != 768 {
		t.Fatalf("expected 768 dimensions, got %d", len(vec1))
	}

	for i := range vec1 {
		if vec1[i] != vec2[i] {
			t.Fatalf("expected identical embeddings for same text at index %d", i)
		}
	}

	// Verify unit norm
	var normSq float64
	for _, v := range vec1 {
		normSq += float64(v * v)
	}
	norm := math.Sqrt(normSq)
	if math.Abs(norm-1.0) > 1e-4 {
		t.Fatalf("expected vector to be unit-normalized (~1.0), got %f", norm)
	}
}

func TestMockEmbedderError(t *testing.T) {
	mock := &MockEmbedder{Err: errors.New("embedding quota exceeded")}
	_, err := mock.Embed(context.Background(), "test text")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestGeminiEmbedderMissingKey(t *testing.T) {
	gemini := NewGeminiEmbedder("", "")
	_, err := gemini.Embed(context.Background(), "test text")
	if err == nil {
		t.Fatalf("expected error for missing key, got nil")
	}
}
