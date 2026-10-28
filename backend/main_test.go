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
	"strconv"
	"strings"
	"testing"
	"time"

	"docupilot/backend/ai"
	"docupilot/backend/auth"
	"docupilot/backend/converter"
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

func TestRootEndpoint(t *testing.T) {
	env, cleanup := setupTestRouter(nil)
	defer cleanup()

	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	env.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var res map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if res["status"] != "running" {
		t.Fatalf("expected status 'running', got %q", res["status"])
	}
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

func TestAskStreamSuccess(t *testing.T) {
	mockAI := &ai.MockProvider{
		Answer: "DocuPilot streams answers chunk by chunk.",
	}
	env, cleanup := setupTestRouter(mockAI)
	defer cleanup()

	payload := AskRequest{
		Text:     "DocuPilot is designed with Go and React.",
		Question: "How does DocuPilot stream answers?",
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest(http.MethodPost, "/api/ask/stream", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	env.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d. Body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/event-stream") {
		t.Fatalf("expected Content-Type text/event-stream, got %q", contentType)
	}

	bodyStr := w.Body.String()
	if !strings.Contains(bodyStr, "event: sources") {
		t.Fatalf("expected event: sources in stream, got %s", bodyStr)
	}
	if !strings.Contains(bodyStr, "event: chunk") {
		t.Fatalf("expected event: chunk in stream, got %s", bodyStr)
	}
	if !strings.Contains(bodyStr, "event: done") {
		t.Fatalf("expected event: done in stream, got %s", bodyStr)
	}
	if !strings.Contains(bodyStr, "DocuPilot streams answers chunk by chunk.") {
		t.Fatalf("expected answer text in done event, got %s", bodyStr)
	}
}

func TestAskStreamWithDocumentAndSemanticSearch(t *testing.T) {
	mockAI := &ai.MockProvider{
		Answer: "DocuPilot stream with document citations.",
	}
	env, cleanup := setupTestRouter(mockAI)
	defer cleanup()

	ctx := context.Background()
	doc := &store.Document{
		Filename:  "streaming_guide.pdf",
		Content:   "Server-Sent Events allow continuous streaming of response tokens directly to the browser.",
		CharCount: 90,
	}
	_ = env.store.SaveDocument(ctx, doc)
	_ = env.processor.ProcessSync(ctx, doc.ID)

	payload := AskRequest{
		DocumentID: &doc.ID,
		Question:   "What do Server-Sent Events allow?",
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest(http.MethodPost, "/api/ask/stream", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	env.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	bodyStr := w.Body.String()
	if !strings.Contains(bodyStr, "Passage 1") {
		t.Fatalf("expected Passage citation in stream sources, got %s", bodyStr)
	}
}

func TestAskStreamMissingQuestion(t *testing.T) {
	env, cleanup := setupTestRouter(nil)
	defer cleanup()

	payload := AskRequest{
		Text:     "Some valid text",
		Question: "   ",
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest(http.MethodPost, "/api/ask/stream", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	env.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 for empty question, got %d", w.Code)
	}
}

func TestAskStreamAIError(t *testing.T) {
	mockAI := &ai.MockProvider{
		Err: errors.New("upstream streaming error occurred"),
	}
	env, cleanup := setupTestRouter(mockAI)
	defer cleanup()

	payload := AskRequest{
		Text:     "Some valid text",
		Question: "A question?",
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest(http.MethodPost, "/api/ask/stream", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	env.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200 (SSE connection established), got %d", w.Code)
	}

	bodyStr := w.Body.String()
	if !strings.Contains(bodyStr, "event: error") {
		t.Fatalf("expected event: error, got %s", bodyStr)
	}
	if !strings.Contains(bodyStr, "upstream streaming error occurred") {
		t.Fatalf("expected error message in stream, got %s", bodyStr)
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

func TestAuthEndpointsAndUserIsolation(t *testing.T) {
	env, cleanup := setupTestRouter(nil)
	defer cleanup()

	// 1. Weak password registration should fail (400)
	weakPayload, _ := json.Marshal(RegisterRequest{Email: "user@example.com", Password: "123"})
	reqWeak, _ := http.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBuffer(weakPayload))
	reqWeak.Header.Set("Content-Type", "application/json")
	wWeak := httptest.NewRecorder()
	env.router.ServeHTTP(wWeak, reqWeak)
	if wWeak.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for weak password, got %d", wWeak.Code)
	}

	// 2. Successful registration for User 1
	u1Payload, _ := json.Marshal(RegisterRequest{Email: "user1@example.com", Password: "SecurePassword1!"})
	reqU1Reg, _ := http.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBuffer(u1Payload))
	reqU1Reg.Header.Set("Content-Type", "application/json")
	wU1Reg := httptest.NewRecorder()
	env.router.ServeHTTP(wU1Reg, reqU1Reg)
	if wU1Reg.Code != http.StatusCreated {
		t.Fatalf("expected 201 for user 1 registration, got %d. Body: %s", wU1Reg.Code, wU1Reg.Body.String())
	}

	var u1Auth AuthResponse
	_ = json.Unmarshal(wU1Reg.Body.Bytes(), &u1Auth)
	if u1Auth.Token == "" || u1Auth.User == nil || u1Auth.User.Email != "user1@example.com" {
		t.Fatalf("invalid auth response for user 1: %+v", u1Auth)
	}

	// 3. Duplicate email registration should fail (409 Conflict)
	reqDup, _ := http.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBuffer(u1Payload))
	reqDup.Header.Set("Content-Type", "application/json")
	wDup := httptest.NewRecorder()
	env.router.ServeHTTP(wDup, reqDup)
	if wDup.Code != http.StatusConflict {
		t.Fatalf("expected 409 for duplicate registration, got %d", wDup.Code)
	}

	// 4. Successful login
	loginPayload, _ := json.Marshal(LoginRequest{Email: "user1@example.com", Password: "SecurePassword1!"})
	reqLogin, _ := http.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBuffer(loginPayload))
	reqLogin.Header.Set("Content-Type", "application/json")
	wLogin := httptest.NewRecorder()
	env.router.ServeHTTP(wLogin, reqLogin)
	if wLogin.Code != http.StatusOK {
		t.Fatalf("expected 200 for login, got %d", wLogin.Code)
	}

	// 5. Failed login with wrong password (401)
	badLoginPayload, _ := json.Marshal(LoginRequest{Email: "user1@example.com", Password: "WrongPassword1!"})
	reqBadLogin, _ := http.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBuffer(badLoginPayload))
	reqBadLogin.Header.Set("Content-Type", "application/json")
	wBadLogin := httptest.NewRecorder()
	env.router.ServeHTTP(wBadLogin, reqBadLogin)
	if wBadLogin.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for bad login, got %d", wBadLogin.Code)
	}

	// 6. Access /api/auth/me without token (401)
	reqMeNoAuth, _ := http.NewRequest(http.MethodGet, "/api/auth/me", nil)
	wMeNoAuth := httptest.NewRecorder()
	env.router.ServeHTTP(wMeNoAuth, reqMeNoAuth)
	if wMeNoAuth.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthenticated /me, got %d", wMeNoAuth.Code)
	}

	// 7. Access /api/auth/me with User 1 token (200)
	reqMeAuth, _ := http.NewRequest(http.MethodGet, "/api/auth/me", nil)
	reqMeAuth.Header.Set("Authorization", "Bearer "+u1Auth.Token)
	wMeAuth := httptest.NewRecorder()
	env.router.ServeHTTP(wMeAuth, reqMeAuth)
	if wMeAuth.Code != http.StatusOK {
		t.Fatalf("expected 200 for authenticated /me, got %d", wMeAuth.Code)
	}

	// 8. Register User 2
	u2Payload, _ := json.Marshal(RegisterRequest{Email: "user2@example.com", Password: "SecurePassword2!"})
	reqU2Reg, _ := http.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBuffer(u2Payload))
	reqU2Reg.Header.Set("Content-Type", "application/json")
	wU2Reg := httptest.NewRecorder()
	env.router.ServeHTTP(wU2Reg, reqU2Reg)
	var u2Auth AuthResponse
	_ = json.Unmarshal(wU2Reg.Body.Bytes(), &u2Auth)

	// 9. User 1 uploads a document
	pdfContent := []byte("%PDF-1.4 user1 document content")
	reqUp1, _ := createMultipartRequest("file", "user1_doc.pdf", pdfContent)
	reqUp1.Header.Set("Authorization", "Bearer "+u1Auth.Token)
	wUp1 := httptest.NewRecorder()
	env.router.ServeHTTP(wUp1, reqUp1)
	if wUp1.Code != http.StatusOK {
		t.Fatalf("expected 200 for user 1 upload, got %d", wUp1.Code)
	}
	var upRes UploadResponse
	_ = json.Unmarshal(wUp1.Body.Bytes(), &upRes)
	u1DocID := upRes.ID

	// 10. User 2 lists documents: must NOT see User 1's document!
	reqListU2, _ := http.NewRequest(http.MethodGet, "/api/documents", nil)
	reqListU2.Header.Set("Authorization", "Bearer "+u2Auth.Token)
	wListU2 := httptest.NewRecorder()
	env.router.ServeHTTP(wListU2, reqListU2)
	var u2Docs []*store.Document
	_ = json.Unmarshal(wListU2.Body.Bytes(), &u2Docs)
	if len(u2Docs) != 0 {
		t.Fatalf("user 2 should see 0 documents, but saw %d", len(u2Docs))
	}

	// 11. User 2 attempts to GET User 1's document: must be rejected with 403 Forbidden!
	reqGetOther, _ := http.NewRequest(http.MethodGet, "/api/documents/"+strconv.FormatInt(u1DocID, 10), nil)
	reqGetOther.Header.Set("Authorization", "Bearer "+u2Auth.Token)
	wGetOther := httptest.NewRecorder()
	env.router.ServeHTTP(wGetOther, reqGetOther)
	if wGetOther.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden when accessing another user's document, got %d", wGetOther.Code)
	}

	// 12. User 2 attempts to DELETE User 1's document: must be rejected with 403 Forbidden!
	reqDelOther, _ := http.NewRequest(http.MethodDelete, "/api/documents/"+strconv.FormatInt(u1DocID, 10), nil)
	reqDelOther.Header.Set("Authorization", "Bearer "+u2Auth.Token)
	wDelOther := httptest.NewRecorder()
	env.router.ServeHTTP(wDelOther, reqDelOther)
	if wDelOther.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden when deleting another user's document, got %d", wDelOther.Code)
	}

	// 13. User 1 CAN delete their own document (200)
	reqDelOwn, _ := http.NewRequest(http.MethodDelete, "/api/documents/"+strconv.FormatInt(u1DocID, 10), nil)
	reqDelOwn.Header.Set("Authorization", "Bearer "+u1Auth.Token)
	wDelOwn := httptest.NewRecorder()
	env.router.ServeHTTP(wDelOwn, reqDelOwn)
	if wDelOwn.Code != http.StatusOK {
		t.Fatalf("expected 200 when deleting own document, got %d", wDelOwn.Code)
	}
}

type mockConverter struct {
	pairs []converter.ConversionPair
	res   *converter.ConvertResult
	err   error
}

func (m *mockConverter) Convert(_ context.Context, req converter.ConvertRequest) (*converter.ConvertResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.res != nil {
		return m.res, nil
	}
	return &converter.ConvertResult{
		Filename: "converted-" + req.Filename,
		MimeType: converter.GetMimeType(req.TargetFormat),
		Data:     []byte("mock-converted-data"),
	}, nil
}

func (m *mockConverter) SupportedPairs() []converter.ConversionPair {
	if m.pairs != nil {
		return m.pairs
	}
	return converter.NewLibreOfficeConverter("", nil).SupportedPairs()
}

func (m *mockConverter) IsSupported(source, target string) bool {
	return converter.NewLibreOfficeConverter("", nil).IsSupported(source, target)
}

func TestConvertEndpoints(t *testing.T) {
	env, cleanup := setupTestRouter(nil)
	defer cleanup()

	// 1. GET /api/convert/formats
	reqFormats, _ := http.NewRequest(http.MethodGet, "/api/convert/formats", nil)
	wFormats := httptest.NewRecorder()
	env.router.ServeHTTP(wFormats, reqFormats)

	if wFormats.Code != http.StatusOK {
		t.Fatalf("expected 200 from /api/convert/formats, got %d", wFormats.Code)
	}

	var formatsResp struct {
		Pairs []converter.ConversionPair `json:"pairs"`
	}
	if err := json.Unmarshal(wFormats.Body.Bytes(), &formatsResp); err != nil {
		t.Fatalf("failed to decode formats response: %v", err)
	}
	if len(formatsResp.Pairs) != 8 {
		t.Fatalf("expected 8 supported pairs, got %d", len(formatsResp.Pairs))
	}

	// 2. POST /api/convert with missing file
	reqEmpty, _ := http.NewRequest(http.MethodPost, "/api/convert", nil)
	wEmpty := httptest.NewRecorder()
	env.router.ServeHTTP(wEmpty, reqEmpty)
	if wEmpty.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty convert request, got %d", wEmpty.Code)
	}

	// 3. POST /api/convert with missing target_format
	bodyNoTarget := new(bytes.Buffer)
	writerNoTarget := multipart.NewWriter(bodyNoTarget)
	partNoTarget, _ := writerNoTarget.CreateFormFile("file", "test.pdf")
	_, _ = partNoTarget.Write([]byte("%PDF-1.4 test"))
	_ = writerNoTarget.Close()

	reqNoTarget, _ := http.NewRequest(http.MethodPost, "/api/convert", bodyNoTarget)
	reqNoTarget.Header.Set("Content-Type", writerNoTarget.FormDataContentType())
	wNoTarget := httptest.NewRecorder()
	env.router.ServeHTTP(wNoTarget, reqNoTarget)
	if wNoTarget.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing target_format, got %d", wNoTarget.Code)
	}

	// 4. POST /api/convert successful conversion with mock converter
	mockConv := &mockConverter{
		res: &converter.ConvertResult{
			Filename: "sample.docx",
			MimeType: converter.GetMimeType("docx"),
			Data:     []byte("word-docx-bytes"),
		},
	}

	customRouter := SetupRouter(
		&ai.MockProvider{Answer: "test"},
		ai.NewMockEmbedder(),
		&mockPDFExtractor{Text: "test"},
		store.NewMemoryStore(),
		nil,
		mockConv,
	)

	bodyValid := new(bytes.Buffer)
	writerValid := multipart.NewWriter(bodyValid)
	_ = writerValid.WriteField("target_format", "docx")
	_ = writerValid.WriteField("source_format", "pdf")
	partValid, _ := writerValid.CreateFormFile("file", "sample.pdf")
	_, _ = partValid.Write([]byte("%PDF-1.5 test document"))
	_ = writerValid.Close()

	reqValid, _ := http.NewRequest(http.MethodPost, "/api/convert", bodyValid)
	reqValid.Header.Set("Content-Type", writerValid.FormDataContentType())
	wValid := httptest.NewRecorder()
	customRouter.ServeHTTP(wValid, reqValid)

	if wValid.Code != http.StatusOK {
		t.Fatalf("expected 200 from /api/convert, got %d: %s", wValid.Code, wValid.Body.String())
	}

	if wValid.Header().Get("Content-Type") != converter.GetMimeType("docx") {
		t.Fatalf("unexpected Content-Type: %s", wValid.Header().Get("Content-Type"))
	}

	if !strings.Contains(wValid.Header().Get("Content-Disposition"), "sample.docx") {
		t.Fatalf("unexpected Content-Disposition header: %s", wValid.Header().Get("Content-Disposition"))
	}

	if wValid.Body.String() != "word-docx-bytes" {
		t.Fatalf("unexpected converted file body: %s", wValid.Body.String())
	}
}

func TestOAuthProvidersEndpoint(t *testing.T) {
	env, cleanup := setupTestRouter(nil)
	defer cleanup()

	req, _ := http.NewRequest(http.MethodGet, "/api/auth/providers", nil)
	w := httptest.NewRecorder()
	env.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var res map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to parse json: %v", err)
	}

	if _, ok := res["google"]; !ok {
		t.Errorf("expected 'google' in response")
	}
}

func TestGoogleOAuthFlow(t *testing.T) {
	mockOAuth := &auth.MockOAuthVerifier{}
	jwtManager := auth.NewJWTManager("test-secret-key-32-chars-long!", time.Hour)
	st := store.NewMemoryStore()

	router := SetupRouter(
		&ai.MockProvider{Answer: "mock"},
		ai.NewMockEmbedder(),
		&mockPDFExtractor{Text: "test"},
		st,
		nil,
		jwtManager,
		mockOAuth,
	)

	// Test 1: New user registration via Google OAuth
	reqBody := bytes.NewBufferString(`{"credential":"valid-google-id-token"}`)
	req, _ := http.NewRequest(http.MethodPost, "/api/auth/oauth/google", reqBody)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 from Google OAuth, got %d: %s", w.Code, w.Body.String())
	}

	var authRes AuthResponse
	if err := json.Unmarshal(w.Body.Bytes(), &authRes); err != nil {
		t.Fatalf("failed to decode auth response: %v", err)
	}
	if authRes.Token == "" {
		t.Errorf("expected non-empty JWT token")
	}
	if authRes.User.Email != "test.google@example.com" {
		t.Errorf("expected email test.google@example.com, got %s", authRes.User.Email)
	}

	// Test 2: Existing user sign in with Google OAuth
	reqBody2 := bytes.NewBufferString(`{"credential":"valid-google-id-token"}`)
	req2, _ := http.NewRequest(http.MethodPost, "/api/auth/oauth/google", reqBody2)
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200 from existing user Google OAuth, got %d", w2.Code)
	}

	// Test 3: Empty credential returns 400
	reqBody3 := bytes.NewBufferString(`{"credential":""}`)
	req3, _ := http.NewRequest(http.MethodPost, "/api/auth/oauth/google", reqBody3)
	req3.Header.Set("Content-Type", "application/json")
	w3 := httptest.NewRecorder()
	router.ServeHTTP(w3, req3)

	if w3.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty credential, got %d", w3.Code)
	}
}

func TestGitHubOAuthFlow(t *testing.T) {
	mockOAuth := &auth.MockOAuthVerifier{}
	jwtManager := auth.NewJWTManager("test-secret-key-32-chars-long!", time.Hour)
	st := store.NewMemoryStore()

	router := SetupRouter(
		&ai.MockProvider{Answer: "mock"},
		ai.NewMockEmbedder(),
		&mockPDFExtractor{Text: "test"},
		st,
		nil,
		jwtManager,
		mockOAuth,
	)

	// Test 1: Successful GitHub OAuth
	reqBody := bytes.NewBufferString(`{"token":"valid-github-token"}`)
	req, _ := http.NewRequest(http.MethodPost, "/api/auth/oauth/github", reqBody)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 from GitHub OAuth, got %d: %s", w.Code, w.Body.String())
	}

	var authRes AuthResponse
	if err := json.Unmarshal(w.Body.Bytes(), &authRes); err != nil {
		t.Fatalf("failed to decode auth response: %v", err)
	}
	if authRes.User.Email != "test.github@example.com" {
		t.Errorf("expected email test.github@example.com, got %s", authRes.User.Email)
	}

	// Test 2: Missing token/code returns 400
	reqBody2 := bytes.NewBufferString(`{}`)
	req2, _ := http.NewRequest(http.MethodPost, "/api/auth/oauth/github", reqBody2)
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing token, got %d", w2.Code)
	}
}
