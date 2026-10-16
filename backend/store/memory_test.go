package store

import (
	"context"
	"testing"
)

func TestMemoryStoreDocumentOperations(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	doc := &Document{
		Filename:  "test.pdf",
		Content:   "This is a test document.",
		CharCount: 24,
	}

	if err := s.SaveDocument(ctx, doc); err != nil {
		t.Fatalf("failed to save document: %v", err)
	}

	if doc.ID == 0 {
		t.Fatalf("expected non-zero document ID, got %d", doc.ID)
	}

	retrieved, err := s.GetDocument(ctx, doc.ID)
	if err != nil {
		t.Fatalf("failed to get document: %v", err)
	}
	if retrieved.Filename != "test.pdf" {
		t.Fatalf("expected filename 'test.pdf', got %q", retrieved.Filename)
	}

	docs, err := s.ListDocuments(ctx)
	if err != nil {
		t.Fatalf("failed to list documents: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("expected 1 document, got %d", len(docs))
	}

	if err := s.UpdateDocumentStatus(ctx, doc.ID, "completed"); err != nil {
		t.Fatalf("failed to update status: %v", err)
	}
	updated, _ := s.GetDocument(ctx, doc.ID)
	if updated.Status != "completed" {
		t.Fatalf("expected status 'completed', got %q", updated.Status)
	}

	if err := s.DeleteDocument(ctx, doc.ID); err != nil {
		t.Fatalf("failed to delete document: %v", err)
	}

	_, err = s.GetDocument(ctx, doc.ID)
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound after deletion, got %v", err)
	}
}

func TestMemoryStoreVectorSearch(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	docID := int64(1)
	chunks := []Chunk{
		{
			DocumentID: docID,
			ChunkIndex: 0,
			Content:    "Go is an open source language.",
			Embedding:  []float32{1.0, 0.0, 0.0},
		},
		{
			DocumentID: docID,
			ChunkIndex: 1,
			Content:    "React is a frontend library.",
			Embedding:  []float32{0.0, 1.0, 0.0},
		},
	}

	if err := s.SaveChunks(ctx, chunks); err != nil {
		t.Fatalf("failed to save chunks: %v", err)
	}

	queryEmb := []float32{0.9, 0.1, 0.0}
	results, err := s.SearchSimilarChunks(ctx, docID, queryEmb, 1)
	if err != nil {
		t.Fatalf("failed to search similar chunks: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0].ChunkIndex != 0 {
		t.Fatalf("expected top chunk index 0, got %d", results[0].ChunkIndex)
	}
	if results[0].Similarity < 0.8 {
		t.Fatalf("expected high similarity, got %f", results[0].Similarity)
	}
}

func TestMemoryStoreQAHistory(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	docID := int64(42)
	qa := &QAHistory{
		DocumentID: &docID,
		Question:   "What is DocuPilot?",
		Answer:     "A learning-first PDF QA app.",
	}

	if err := s.SaveQA(ctx, qa); err != nil {
		t.Fatalf("failed to save QA: %v", err)
	}

	history, err := s.ListQAByDocument(ctx, &docID)
	if err != nil {
		t.Fatalf("failed to list QA: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("expected 1 QA history item, got %d", len(history))
	}
	if history[0].Question != "What is DocuPilot?" {
		t.Fatalf("expected question 'What is DocuPilot?', got %q", history[0].Question)
	}
}
