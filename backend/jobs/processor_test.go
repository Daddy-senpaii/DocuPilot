package jobs

import (
	"context"
	"testing"
	"time"

	"docupilot/backend/ai"
	"docupilot/backend/store"
)

func TestDocumentProcessorSync(t *testing.T) {
	s := store.NewMemoryStore()
	embedder := ai.NewMockEmbedder()
	proc := NewDocumentProcessor(s, embedder, 1)
	defer proc.Close()

	ctx := context.Background()
	doc := &store.Document{
		Filename:  "test.txt",
		Content:   "Antigravity is building DocuPilot with Go, Gin, and React. It supports PDF question answering and semantic search with pgvector.",
		CharCount: 120,
	}

	if err := s.SaveDocument(ctx, doc); err != nil {
		t.Fatalf("failed to save doc: %v", err)
	}

	if err := proc.ProcessSync(ctx, doc.ID); err != nil {
		t.Fatalf("failed to process doc: %v", err)
	}

	updated, err := s.GetDocument(ctx, doc.ID)
	if err != nil {
		t.Fatalf("failed to retrieve doc: %v", err)
	}
	if updated.Status != "ready" {
		t.Fatalf("expected status 'ready', got %q", updated.Status)
	}

	chunks, err := s.GetChunksByDocument(ctx, doc.ID)
	if err != nil {
		t.Fatalf("failed to get chunks: %v", err)
	}
	if len(chunks) == 0 {
		t.Fatalf("expected at least 1 chunk created, got 0")
	}
	if len(chunks[0].Embedding) != 768 {
		t.Fatalf("expected 768 dim embedding, got %d", len(chunks[0].Embedding))
	}
}

func TestDocumentProcessorAsync(t *testing.T) {
	s := store.NewMemoryStore()
	embedder := ai.NewMockEmbedder()
	proc := NewDocumentProcessor(s, embedder, 1)
	defer proc.Close()

	ctx := context.Background()
	doc := &store.Document{
		Filename:  "async.txt",
		Content:   "Quick test document for async worker queue processing.",
		CharCount: 54,
	}

	if err := s.SaveDocument(ctx, doc); err != nil {
		t.Fatalf("failed to save doc: %v", err)
	}

	proc.Enqueue(doc.ID)

	// Wait up to 2 seconds for worker to process
	var finalStatus string
	for i := 0; i < 20; i++ {
		time.Sleep(100 * time.Millisecond)
		updated, _ := s.GetDocument(ctx, doc.ID)
		if updated != nil && updated.Status == "ready" {
			finalStatus = "ready"
			break
		}
	}

	if finalStatus != "ready" {
		t.Fatalf("expected doc to reach 'ready' status via async queue, got %s", finalStatus)
	}
}
