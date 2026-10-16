package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"docupilot/backend/ai"
	"docupilot/backend/extractor"
	"docupilot/backend/jobs"
	"docupilot/backend/store"
	"github.com/gin-gonic/gin"
)

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

// corsMiddleware sets headers to allow requests from local frontend development servers.
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Gemini-Api-Key")

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
) *gin.Engine {
	r := gin.Default()
	r.Use(corsMiddleware())
	r.MaxMultipartMemory = maxUploadSize

	// Health check endpoint
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// List saved documents
	r.GET("/api/documents", func(c *gin.Context) {
		docs, err := st.ListDocuments(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusOK, docs)
	})

	// Get single document with QA history
	r.GET("/api/documents/:id", func(c *gin.Context) {
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

		qaList, _ := st.ListQAByDocument(c.Request.Context(), &id)
		c.JSON(http.StatusOK, gin.H{
			"document": doc,
			"history":  qaList,
		})
	})

	// Delete document
	r.DELETE("/api/documents/:id", func(c *gin.Context) {
		idStr := c.Param("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid document ID"})
			return
		}

		if err := st.DeleteDocument(c.Request.Context(), id); err != nil {
			if err == store.ErrNotFound {
				c.JSON(http.StatusNotFound, ErrorResponse{Error: "Document not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "deleted"})
	})

	// Question answering endpoint (Semantic Search + AI)
	r.POST("/api/ask", func(c *gin.Context) {
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

		var contextForAI string
		var sourceSnippets []string

		if req.DocumentID != nil {
			// Document mode: retrieve relevant semantic passages
			doc, err := st.GetDocument(c.Request.Context(), *req.DocumentID)
			if err != nil {
				c.JSON(http.StatusNotFound, ErrorResponse{Error: "Specified document not found"})
				return
			}

			// Perform semantic retrieval if embedder and chunks are available
			queryEmb, err := reqEmbedder.Embed(c.Request.Context(), trimmedQuestion)
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

			// Fallback to full document text if chunk search returned empty
			if strings.TrimSpace(contextForAI) == "" {
				contextForAI = doc.Content
			}
		} else {
			// Direct pasted text mode
			trimmedText := strings.TrimSpace(req.Text)
			if trimmedText == "" {
				c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Either document_id or text must be provided"})
				return
			}
			contextForAI = trimmedText
		}

		answer, err := reqAIProvider.GenerateAnswer(c.Request.Context(), contextForAI, trimmedQuestion)
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
			return
		}

		// Save QA entry into history
		_ = st.SaveQA(c.Request.Context(), &store.QAHistory{
			DocumentID: req.DocumentID,
			Question:   trimmedQuestion,
			Answer:     answer,
		})

		c.JSON(http.StatusOK, AskResponse{
			Answer:  answer,
			Sources: sourceSnippets,
		})
	})

	// PDF document upload and text extraction endpoint
	r.POST("/api/upload", func(c *gin.Context) {
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

		// Save document record
		doc := &store.Document{
			Filename:  fileHeader.Filename,
			Content:   text,
			CharCount: len(text),
			Status:    "processing",
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
	router := SetupRouter(provider, embedder, pdfExtractor, st, processor)

	log.Printf("Starting DocuPilot backend server on port %s...", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
