package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"time"

	"docupilot/backend/ai"
	"docupilot/backend/auth"
	"docupilot/backend/converter"
	"docupilot/backend/extractor"
	"docupilot/backend/jobs"
	"docupilot/backend/store"
	"github.com/gin-gonic/gin"
)

// RegisterRequest represents the user signup payload.
type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginRequest represents the user signin payload.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// AuthResponse represents the authentication response containing a JWT token and user profile.
type AuthResponse struct {
	Token string      `json:"token"`
	User  *store.User `json:"user"`
}

// AskRequest represents the user request payload containing text context or a document ID and a question.
type AskRequest struct {
	DocumentID *int64 `json:"document_id,omitempty"`
	Text       string `json:"text,omitempty"`
	Question   string `json:"question"`
}

// AskResponse represents the response containing the generated answer and source passages.
type AskResponse struct {
	Answer  string   `json:"answer"`
	Sources []string `json:"sources,omitempty"`
}

// UploadResponse represents the response from extracting text from an uploaded document.
type UploadResponse struct {
	ID        int64  `json:"id"`
	Filename  string `json:"filename"`
	Text      string `json:"text"`
	CharCount int    `json:"char_count"`
	Status    string `json:"status"`
}

// ErrorResponse represents a structured error message.
type ErrorResponse struct {
	Error string `json:"error"`
}

const maxUploadSize = 10 * 1024 * 1024 // 10 MB

// loadEnv reads key=value pairs from .env files without external dependencies.
func loadEnv(filepaths ...string) {
	for _, path := range filepaths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				val := strings.TrimSpace(parts[1])
				val = strings.Trim(val, `"'`)
				if os.Getenv(key) == "" && val != "" {
					_ = os.Setenv(key, val)
				}
			}
		}
	}
}

// corsMiddleware sets headers to allow requests from local frontend development servers and sets security headers.
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Gemini-Api-Key")
		c.Writer.Header().Set("Access-Control-Expose-Headers", "Content-Disposition")
		c.Writer.Header().Set("X-Content-Type-Options", "nosniff")
		c.Writer.Header().Set("X-Frame-Options", "DENY")
		c.Writer.Header().Set("X-XSS-Protection", "1; mode=block")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// SetupRouter creates and configures the Gin engine with routes and dependencies.
func SetupRouter(
	provider ai.AIProvider,
	embedder ai.Embedder,
	pdfExtractor extractor.TextExtractor,
	st store.Store,
	processor *jobs.DocumentProcessor,
	authParams ...any,
) *gin.Engine {
	var jwtManager *auth.JWTManager
	var limiter *auth.IPRateLimiter
	var docConverter converter.DocumentConverter

	for _, param := range authParams {
		switch p := param.(type) {
		case *auth.JWTManager:
			jwtManager = p
		case *auth.IPRateLimiter:
			limiter = p
		case converter.DocumentConverter:
			docConverter = p
		}
	}

	if docConverter == nil {
		docConverter = converter.NewLibreOfficeConverter("", nil)
	}

	if jwtManager == nil {
		jwtManager = auth.NewJWTManager("docupilot-default-development-secret-key-32b!", 24*time.Hour)
	}
	if limiter == nil {
		limiter = auth.NewIPRateLimiter(5 * time.Minute)
	}

	r := gin.Default()
	r.Use(corsMiddleware())
	r.MaxMultipartMemory = maxUploadSize

	// Health check endpoint
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// --- Authentication Endpoints ---

	// User Registration (Rate limited: max 10 per minute per IP)
	r.POST("/api/auth/register", func(c *gin.Context) {
		if !limiter.Allow(c.ClientIP(), 10, 1*time.Minute) {
			c.JSON(http.StatusTooManyRequests, ErrorResponse{Error: "Too many registration attempts. Please wait a minute."})
			return
		}

		var req RegisterRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid JSON request payload"})
			return
		}

		email, err := auth.NormalizeAndValidateEmail(req.Email)
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
			return
		}

		hashedPassword, err := auth.HashPassword(req.Password)
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
			return
		}

		user := &store.User{
			Email:        email,
			PasswordHash: hashedPassword,
		}

		if err := st.CreateUser(c.Request.Context(), user); err != nil {
			if err == store.ErrUserAlreadyExists {
				c.JSON(http.StatusConflict, ErrorResponse{Error: "An account with this email address already exists"})
				return
			}
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to create user account"})
			return
		}

		token, err := jwtManager.GenerateToken(user)
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to generate authentication token"})
			return
		}

		c.JSON(http.StatusCreated, AuthResponse{
			Token: token,
			User:  user,
		})
	})

	// User Login (Rate limited: max 5 per minute per IP to mitigate brute force)
	r.POST("/api/auth/login", func(c *gin.Context) {
		if !limiter.Allow(c.ClientIP(), 5, 1*time.Minute) {
			c.JSON(http.StatusTooManyRequests, ErrorResponse{Error: "Too many login attempts. Please wait a minute."})
			return
		}

		var req LoginRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid JSON request payload"})
			return
		}

		email, err := auth.NormalizeAndValidateEmail(req.Email)
		if err != nil {
			auth.DummyPasswordCheck(req.Password)
			c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "Invalid email or password"})
			return
		}

		user, err := st.GetUserByEmail(c.Request.Context(), email)
		if err != nil {
			// Anti-timing attack check
			auth.DummyPasswordCheck(req.Password)
			c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "Invalid email or password"})
			return
		}

		if !auth.CheckPassword(user.PasswordHash, req.Password) {
			c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "Invalid email or password"})
			return
		}

		token, err := jwtManager.GenerateToken(user)
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to generate authentication token"})
			return
		}

		c.JSON(http.StatusOK, AuthResponse{
			Token: token,
			User:  user,
		})
	})

	// Current authenticated user profile
	r.GET("/api/auth/me", auth.AuthMiddleware(jwtManager, true), func(c *gin.Context) {
		userID, ok := auth.GetAuthUserID(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "Unauthorized"})
			return
		}

		user, err := st.GetUserByID(c.Request.Context(), userID)
		if err != nil {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: "User not found"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"user": user})
	})

	// --- Scoped Document & Question Answering Endpoints ---
	api := r.Group("/api")
	api.Use(auth.AuthMiddleware(jwtManager, false))

	// List saved documents (scoped to authenticated user if signed in)
	api.GET("/documents", func(c *gin.Context) {
		var docs []*store.Document
		var err error

		if userID, ok := auth.GetAuthUserID(c); ok {
			docs, err = st.ListDocumentsByUser(c.Request.Context(), userID)
		} else {
			docs, err = st.ListDocuments(c.Request.Context())
		}

		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
			return
		}
		if docs == nil {
			docs = []*store.Document{}
		}
		c.JSON(http.StatusOK, docs)
	})

	// Get single document with QA history
	api.GET("/documents/:id", func(c *gin.Context) {
		idStr := c.Param("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid document ID"})
			return
		}

		doc, err := st.GetDocument(c.Request.Context(), id)
		if err == store.ErrNotFound {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: "Document not found"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
			return
		}

		// Security: prevent accessing documents owned by other users
		if userID, ok := auth.GetAuthUserID(c); ok && doc.UserID != nil && *doc.UserID != userID {
			c.JSON(http.StatusForbidden, ErrorResponse{Error: "Access denied to this document"})
			return
		}

		qaList, _ := st.ListQAByDocument(c.Request.Context(), &id)
		c.JSON(http.StatusOK, gin.H{
			"document": doc,
			"history":  qaList,
		})
	})

	// Delete document
	api.DELETE("/documents/:id", func(c *gin.Context) {
		idStr := c.Param("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid document ID"})
			return
		}

		doc, err := st.GetDocument(c.Request.Context(), id)
		if err == store.ErrNotFound {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: "Document not found"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
			return
		}

		// Security: prevent deleting documents owned by other users
		if userID, ok := auth.GetAuthUserID(c); ok && doc.UserID != nil && *doc.UserID != userID {
			c.JSON(http.StatusForbidden, ErrorResponse{Error: "Access denied to this document"})
			return
		}

		if err := st.DeleteDocument(c.Request.Context(), id); err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "deleted"})
	})

	// Helper function to resolve document/text context and perform semantic snippet retrieval
	resolveQAContext := func(
		c *gin.Context,
		req AskRequest,
		reqEmbedder ai.Embedder,
	) (contextForAI string, sourceSnippets []string, errResp *ErrorResponse, statusCode int) {
		if req.DocumentID != nil {
			doc, err := st.GetDocument(c.Request.Context(), *req.DocumentID)
			if err != nil {
				return "", nil, &ErrorResponse{Error: "Specified document not found"}, http.StatusNotFound
			}

			if userID, ok := auth.GetAuthUserID(c); ok && doc.UserID != nil && *doc.UserID != userID {
				return "", nil, &ErrorResponse{Error: "Access denied to this document"}, http.StatusForbidden
			}

			queryEmb, err := reqEmbedder.Embed(c.Request.Context(), strings.TrimSpace(req.Question))
			if err == nil && len(queryEmb) > 0 {
				scoredChunks, _ := st.SearchSimilarChunks(c.Request.Context(), *req.DocumentID, queryEmb, 3)
				if len(scoredChunks) > 0 {
					var passageBuilder strings.Builder
					for i, sc := range scoredChunks {
						snippet := fmt.Sprintf("[Passage %d | similarity: %.2f]:\n%s", i+1, sc.Similarity, sc.Content)
						sourceSnippets = append(sourceSnippets, snippet)
						passageBuilder.WriteString(snippet + "\n\n")
					}
					contextForAI = passageBuilder.String()
				}
			}

			if strings.TrimSpace(contextForAI) == "" {
				contextForAI = doc.Content
			}
		} else {
			trimmedText := strings.TrimSpace(req.Text)
			if trimmedText == "" {
				return "", nil, &ErrorResponse{Error: "Either document_id or text must be provided"}, http.StatusBadRequest
			}
			contextForAI = trimmedText
		}

		return contextForAI, sourceSnippets, nil, http.StatusOK
	}

	// Question answering endpoint (Semantic Search + AI)
	api.POST("/ask", func(c *gin.Context) {
		var req AskRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid JSON request payload"})
			return
		}

		trimmedQuestion := strings.TrimSpace(req.Question)
		if trimmedQuestion == "" {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Question cannot be empty"})
			return
		}

		// Support API key override via X-Gemini-Api-Key header from frontend
		reqAIProvider := provider
		reqEmbedder := embedder
		customKey := strings.TrimSpace(c.GetHeader("X-Gemini-Api-Key"))
		if customKey != "" {
			model := os.Getenv("GEMINI_MODEL")
			reqAIProvider = ai.NewGeminiProvider(customKey, model)
			reqEmbedder = ai.NewGeminiEmbedder(customKey, "")
		}

		contextForAI, sourceSnippets, errResp, status := resolveQAContext(c, req, reqEmbedder)
		if errResp != nil {
			c.JSON(status, errResp)
			return
		}

		answer, err := reqAIProvider.GenerateAnswer(c.Request.Context(), contextForAI, trimmedQuestion)
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
			return
		}

		// Save QA entry into history with user ownership if authenticated
		qa := &store.QAHistory{
			DocumentID: req.DocumentID,
			Question:   trimmedQuestion,
			Answer:     answer,
		}
		if userID, ok := auth.GetAuthUserID(c); ok {
			qa.UserID = &userID
		}
		_ = st.SaveQA(c.Request.Context(), qa)

		c.JSON(http.StatusOK, AskResponse{
			Answer:  answer,
			Sources: sourceSnippets,
		})
	})

	// Question answering streaming endpoint (Server-Sent Events)
	api.POST("/ask/stream", func(c *gin.Context) {
		var req AskRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid JSON request payload"})
			return
		}

		trimmedQuestion := strings.TrimSpace(req.Question)
		if trimmedQuestion == "" {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Question cannot be empty"})
			return
		}

		reqAIProvider := provider
		reqEmbedder := embedder
		customKey := strings.TrimSpace(c.GetHeader("X-Gemini-Api-Key"))
		if customKey != "" {
			model := os.Getenv("GEMINI_MODEL")
			reqAIProvider = ai.NewGeminiProvider(customKey, model)
			reqEmbedder = ai.NewGeminiEmbedder(customKey, "")
		}

		contextForAI, sourceSnippets, errResp, status := resolveQAContext(c, req, reqEmbedder)
		if errResp != nil {
			c.JSON(status, errResp)
			return
		}

		// Configure Server-Sent Events headers
		c.Writer.Header().Set("Content-Type", "text/event-stream")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.Header().Set("Connection", "keep-alive")
		c.Writer.Header().Set("Transfer-Encoding", "chunked")
		c.Writer.WriteHeader(http.StatusOK)

		flusher, ok := c.Writer.(http.Flusher)
		if !ok {
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Streaming unsupported by response writer"})
			return
		}
		flusher.Flush()

		// Stream retrieved source citations first
		sourcesBytes, _ := json.Marshal(gin.H{"sources": sourceSnippets})
		fmt.Fprintf(c.Writer, "event: sources\ndata: %s\n\n", string(sourcesBytes))
		flusher.Flush()

		// Stream answer chunks token by token
		var fullAnswer strings.Builder
		streamErr := reqAIProvider.GenerateAnswerStream(c.Request.Context(), contextForAI, trimmedQuestion, func(chunk string) error {
			fullAnswer.WriteString(chunk)
			chunkData, _ := json.Marshal(gin.H{"chunk": chunk})
			_, writeErr := fmt.Fprintf(c.Writer, "event: chunk\ndata: %s\n\n", string(chunkData))
			flusher.Flush()
			return writeErr
		})

		if streamErr != nil {
			errData, _ := json.Marshal(gin.H{"error": streamErr.Error()})
			fmt.Fprintf(c.Writer, "event: error\ndata: %s\n\n", string(errData))
			flusher.Flush()
			return
		}

		finalAnswer := strings.TrimSpace(fullAnswer.String())

		// Persist QA into history
		qa := &store.QAHistory{
			DocumentID: req.DocumentID,
			Question:   trimmedQuestion,
			Answer:     finalAnswer,
		}
		if userID, ok := auth.GetAuthUserID(c); ok {
			qa.UserID = &userID
		}
		_ = st.SaveQA(c.Request.Context(), qa)

		doneData, _ := json.Marshal(gin.H{
			"answer": finalAnswer,
			"qa_id":  qa.ID,
		})
		fmt.Fprintf(c.Writer, "event: done\ndata: %s\n\n", string(doneData))
		flusher.Flush()
	})

	// PDF document upload and text extraction endpoint
	api.POST("/upload", func(c *gin.Context) {
		fileHeader, err := c.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Please choose a PDF file to upload"})
			return
		}

		if fileHeader.Size > maxUploadSize {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "File size exceeds the 10MB limit"})
			return
		}

		ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
		if ext != ".pdf" {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Only PDF files (.pdf) are supported"})
			return
		}

		file, err := fileHeader.Open()
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to read uploaded file"})
			return
		}
		defer file.Close()

		fileBytes, err := io.ReadAll(file)
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to read file contents"})
			return
		}

		if err := extractor.ValidatePDFHeader(fileBytes); err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
			return
		}

		text, err := pdfExtractor.ExtractText(bytes.NewReader(fileBytes), int64(len(fileBytes)))
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
			return
		}

		// Save document record with user ownership if authenticated
		doc := &store.Document{
			Filename:  fileHeader.Filename,
			Content:   text,
			CharCount: len(text),
			Status:    "processing",
		}
		if userID, ok := auth.GetAuthUserID(c); ok {
			doc.UserID = &userID
		}

		if err := st.SaveDocument(c.Request.Context(), doc); err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to save document metadata"})
			return
		}

		// Enqueue for background semantic chunking and embedding
		if processor != nil {
			processor.Enqueue(doc.ID)
		}

		c.JSON(http.StatusOK, UploadResponse{
			ID:        doc.ID,
			Filename:  doc.Filename,
			Text:      text,
			CharCount: doc.CharCount,
			Status:    doc.Status,
		})
	})

	// --- Document File Conversion Endpoints ---

	// List supported conversion pairs
	api.GET("/convert/formats", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"pairs": docConverter.SupportedPairs(),
		})
	})

	// Convert document endpoint
	api.POST("/convert", func(c *gin.Context) {
		targetFormat := strings.TrimSpace(c.PostForm("target_format"))
		sourceFormat := strings.TrimSpace(c.PostForm("source_format"))

		fileHeader, err := c.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Please choose a document file to convert"})
			return
		}

		if fileHeader.Size > maxUploadSize {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "File size exceeds the 10MB limit"})
			return
		}

		if targetFormat == "" {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "target_format is required (e.g. pdf, docx, xlsx, pptx, txt)"})
			return
		}

		file, err := fileHeader.Open()
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to read uploaded file"})
			return
		}
		defer file.Close()

		fileBytes, err := io.ReadAll(file)
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to read file contents"})
			return
		}

		res, err := docConverter.Convert(c.Request.Context(), converter.ConvertRequest{
			Filename:     fileHeader.Filename,
			SourceFormat: sourceFormat,
			TargetFormat: targetFormat,
			Data:         fileBytes,
		})
		if err != nil {
			log.Printf("[Converter] Conversion error for %s (%s -> %s): %v", fileHeader.Filename, sourceFormat, targetFormat, err)
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
			return
		}

		c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, res.Filename))
		c.Header("Content-Type", res.MimeType)
		c.Data(http.StatusOK, res.MimeType, res.Data)
	})

	return r
}

func main() {
	// Auto-load .env files from current or parent directories if present
	loadEnv(".env", "../.env")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	apiKey := os.Getenv("GEMINI_API_KEY")
	var provider ai.AIProvider
	var embedder ai.Embedder

	if apiKey != "" {
		model := os.Getenv("GEMINI_MODEL")
		log.Println("Gemini API key detected in environment. Initializing Gemini provider and embedder...")
		provider = ai.NewGeminiProvider(apiKey, model)
		embedder = ai.NewGeminiEmbedder(apiKey, "")
	} else {
		log.Println("Notice: GEMINI_API_KEY is not set in environment or .env. Using local MockProvider. (Keys can also be passed via X-Gemini-Api-Key header).")
		provider = &ai.MockProvider{
			Answer: "[Notice] GEMINI_API_KEY is not set. Set GEMINI_API_KEY in your environment, or configure it in the UI / .env for live Gemini answers.",
		}
		embedder = ai.NewMockEmbedder()
	}

	var st store.Store
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL != "" {
		log.Println("Connecting to PostgreSQL at DATABASE_URL...")
		pgStore, err := store.NewPostgresStore(databaseURL)
		if err != nil {
			log.Fatalf("Failed to initialize PostgreSQL store: %v", err)
		}
		st = pgStore
		log.Println("Connected to PostgreSQL successfully.")
	} else {
		log.Println("Notice: DATABASE_URL is not set. Using in-memory Store with vector similarity.")
		st = store.NewMemoryStore()
	}
	defer st.Close()

	processor := jobs.NewDocumentProcessor(st, embedder, 2)
	defer processor.Close()

	pdfExtractor := extractor.NewPDFExtractor()
	docConverter := converter.NewLibreOfficeConverter("", nil)

	// Initialize JWT manager and rate limiter
	jwtSecret := os.Getenv("JWT_SECRET")
	jwtManager := auth.NewJWTManager(jwtSecret, 24*time.Hour)
	rateLimiter := auth.NewIPRateLimiter(5 * time.Minute)
	defer rateLimiter.Close()

	router := SetupRouter(provider, embedder, pdfExtractor, st, processor, jwtManager, rateLimiter, docConverter)

	log.Printf("Starting DocuPilot backend server on port %s...", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
