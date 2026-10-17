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

func TestMemoryStoreUserOperations(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	user := &User{
		Email:        "pilot@example.com",
		PasswordHash: "hashedsecret",
	}

	if err := s.CreateUser(ctx, user); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	if user.ID == 0 {
		t.Fatalf("expected non-zero user ID, got %d", user.ID)
	}

	// Reject duplicate email
	dupUser := &User{
		Email:        "pilot@example.com",
		PasswordHash: "anothersecret",
	}
	if err := s.CreateUser(ctx, dupUser); err != ErrUserAlreadyExists {
		t.Fatalf("expected ErrUserAlreadyExists, got %v", err)
	}

	// Lookup by email
	byEmail, err := s.GetUserByEmail(ctx, "pilot@example.com")
	if err != nil {
		t.Fatalf("failed to find user by email: %v", err)
	}
	if byEmail.ID != user.ID {
		t.Fatalf("expected user ID %d, got %d", user.ID, byEmail.ID)
	}

	// Lookup by ID
	byID, err := s.GetUserByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("failed to find user by ID: %v", err)
	}
	if byID.Email != "pilot@example.com" {
		t.Fatalf("expected email pilot@example.com, got %q", byID.Email)
	}

	// Non-existent user
	if _, err := s.GetUserByID(ctx, 9999); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound for missing user, got %v", err)
	}
}

func TestMemoryStoreUserDocumentIsolation(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	u1ID := int64(10)
	u2ID := int64(20)

	doc1 := &Document{Filename: "u1_notes.pdf", UserID: &u1ID, Content: "Notes 1"}
	doc2 := &Document{Filename: "u2_report.pdf", UserID: &u2ID, Content: "Report 2"}
	_ = s.SaveDocument(ctx, doc1)
	_ = s.SaveDocument(ctx, doc2)

	u1Docs, err := s.ListDocumentsByUser(ctx, u1ID)
	if err != nil {
		t.Fatalf("failed to list user 1 docs: %v", err)
	}
	if len(u1Docs) != 1 || u1Docs[0].Filename != "u1_notes.pdf" {
		t.Fatalf("expected 1 doc for user 1, got %v", u1Docs)
	}

	u2Docs, err := s.ListDocumentsByUser(ctx, u2ID)
	if err != nil {
		t.Fatalf("failed to list user 2 docs: %v", err)
	}
	if len(u2Docs) != 1 || u2Docs[0].Filename != "u2_report.pdf" {
		t.Fatalf("expected 1 doc for user 2, got %v", u2Docs)
	}
}
