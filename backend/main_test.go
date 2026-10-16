package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"docupilot/backend/ai"
	"docupilot/backend/extractor"
	"docupilot/backend/jobs"
	"docupilot/backend/store"
)

// mockPDFExtractor implements extractor.TextExtractor for tests.
type mockPDFExtractor struct {
	Text string
	Err  error
}

var _ extractor.TextExtractor = (*mockPDFExtractor)(nil)

func (m *mockPDFExtractor) ExtractText(_ io.ReaderAt, _ int64) (string, error) {
	if m.Err != nil {
		return "", m.Err
	}
	return m.Text, nil
}

func setupTestRouter(provider ai.AIProvider) (*ginTestEnv, func()) {
	if provider == nil {
		provider = &ai.MockProvider{Answer: "Mock answer"}
	}
	embedder := ai.NewMockEmbedder()
	pdfExtractor := &mockPDFExtractor{Text: "Test PDF document content for testing."}
	st := store.NewMemoryStore()
	processor := jobs.NewDocumentProcessor(st, embedder, 1)

	router := SetupRouter(provider, embedder, pdfExtractor, st, processor)
	cleanup := func() {
		processor.Close()
	}

	return &ginTestEnv{
		router:    router,
		store:     st,
		embedder:  embedder,
		processor: processor,
	}, cleanup
}

type ginTestEnv struct {
	router    http.Handler
	store     store.Store
	embedder  ai.Embedder
	processor *jobs.DocumentProcessor
}

func TestHealthEndpoint(t *testing.T) {
	env, cleanup := setupTestRouter(nil)
	defer cleanup()

	req, _ := http.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	env.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var res map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if res["status"] != "ok" {
		t.Fatalf("expected status 'ok', got %q", res["status"])
	}
}

func TestAskEndpointSuccess(t *testing.T) {
	mockAI := &ai.MockProvider{
		Answer: "Go is an open-source programming language created at Google.",
	}
	env, cleanup := setupTestRouter(mockAI)
	defer cleanup()

	payload := AskRequest{
		Text:     "Go is an open source programming language supported by Google.",
		Question: "What is Go?",
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest(http.MethodPost, "/api/ask", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	env.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d. Body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var res AskResponse
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if res.Answer != "Go is an open-source programming language created at Google." {
		t.Fatalf("expected answer %q, got %q", "Go is an open-source programming language created at Google.", res.Answer)
	}
}

func TestAskEndpointAIError(t *testing.T) {
	mockAI := &ai.MockProvider{
		Err: errors.New("upstream AI quota exceeded"),
	}
	env, cleanup := setupTestRouter(mockAI)
	defer cleanup()

	payload := AskRequest{
		Text:     "Some valid text",
		Question: "A valid question?",
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest(http.MethodPost, "/api/ask", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	env.router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

func TestAskWithDocumentAndSemanticSearch(t *testing.T) {
	mockAI := &ai.MockProvider{
		Answer: "Semantic search retrieved relevant chunks successfully.",
	}
	env, cleanup := setupTestRouter(mockAI)
	defer cleanup()

	ctx := context.Background()
	doc := &store.Document{
		Filename:  "architecture.pdf",
		Content:   "DocuPilot uses Gin for HTTP and pgvector for semantic vector similarity.",
		CharCount: 75,
	}
	_ = env.store.SaveDocument(ctx, doc)
	_ = env.processor.ProcessSync(ctx, doc.ID)

	payload := AskRequest{
		DocumentID: &doc.ID,
		Question:   "What does DocuPilot use for semantic vector similarity?",
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest(http.MethodPost, "/api/ask", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	env.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var res AskResponse
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	if len(res.Sources) == 0 {
		t.Fatalf("expected source snippets in response, got none")
	}
}

func createMultipartRequest(fieldName, filename string, content []byte) (*http.Request, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if fieldName != "" {
		part, err := writer.CreateFormFile(fieldName, filename)
		if err != nil {
			return nil, err
		}
		if _, err := part.Write(content); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, "/api/upload", body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req, nil
}

func TestUploadEndpointSuccess(t *testing.T) {
	env, cleanup := setupTestRouter(nil)
	defer cleanup()

	pdfContent := []byte("%PDF-1.4 mock content bytes")
	req, err := createMultipartRequest("file", "sample.pdf", pdfContent)
	if err != nil {
		t.Fatalf("failed to build request: %v", err)
	}

	w := httptest.NewRecorder()
	env.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d. Body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var res UploadResponse
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if res.Filename != "sample.pdf" {
		t.Fatalf("expected filename 'sample.pdf', got %q", res.Filename)
	}
	if res.ID == 0 {
		t.Fatalf("expected non-zero doc ID, got %d", res.ID)
	}
}

func TestDocumentsCRUD(t *testing.T) {
	env, cleanup := setupTestRouter(nil)
	defer cleanup()

	ctx := context.Background()
	doc := &store.Document{
		Filename:  "sample.pdf",
		Content:   "Sample text",
		CharCount: 11,
	}
	_ = env.store.SaveDocument(ctx, doc)

	// List documents
	reqList, _ := http.NewRequest(http.MethodGet, "/api/documents", nil)
	wList := httptest.NewRecorder()
	env.router.ServeHTTP(wList, reqList)
	if wList.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", wList.Code)
	}

	// Get single document
	reqGet, _ := http.NewRequest(http.MethodGet, "/api/documents/1", nil)
	wGet := httptest.NewRecorder()
	env.router.ServeHTTP(wGet, reqGet)
	if wGet.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", wGet.Code)
	}

	// Delete document
	reqDel, _ := http.NewRequest(http.MethodDelete, "/api/documents/1", nil)
	wDel := httptest.NewRecorder()
	env.router.ServeHTTP(wDel, reqDel)
	if wDel.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", wDel.Code)
	}
}
