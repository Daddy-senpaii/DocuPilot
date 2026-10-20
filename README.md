# DocuPilot ✈️

DocuPilot is a beginner-friendly, learning-first application for asking questions about documents and PDFs using **Go (Gin)**, **React (TypeScript)**, **PostgreSQL with pgvector**, and **Google Gemini AI**.

---

## 🚀 Features & Architecture

* **Backend (Go + Gin):**
  * Pure Go PDF parsing and text extraction (via `github.com/dslipak/pdf`).
  * AI Provider abstraction for Google Gemini (`gemini-1.5-flash`) with grounded document prompt engineering.
  * Vector embedding generator using Gemini's `text-embedding-004`.
  * Background worker queue for asynchronous chunking and vector indexing without blocking the UI.
  * Persistence layer supporting both PostgreSQL with `pgvector` and an in-memory fallback store with cosine vector similarity.
* **Frontend (React + TypeScript + Vite):**
  * Modern, responsive interface with document upload dropzone, raw text pasting, and document library.
  * Real-time status tracking (`processing`, `ready`, `failed`).
  * Semantic passage citations and source inspection.
  * Interactive Q&A history per document.
* **Database (PostgreSQL + pgvector):**
  * Automatically creates extensions and tables for documents, vector passage chunks, and Q&A history.
* **Document Conversion Studio:**
  * High-fidelity bidirectional conversion powered by headless LibreOffice:
    * **PDF ⇄ Word:** `PDF → DOCX` and `DOCX → PDF`
    * **PDF ⇄ Excel:** `PDF → XLSX` and `XLSX → PDF`
    * **PDF ⇄ PowerPoint:** `PDF → PPTX` and `PPTX → PDF`
    * **Word ⇄ Text:** `DOCX → TXT` and `TXT → PDF`
  * Process isolation, timeout protection, and instant integration bridge with DocuPilot Q&A.

---

## 🛠️ Quick Start with Docker Compose

The fastest way to launch the complete system (Postgres + pgvector, Backend, Frontend):

1. **Configure Gemini API Key (Optional but recommended):**
   ```bash
   cp .env.example .env
   # Edit .env and set your GEMINI_API_KEY
   ```
   *(If not provided, DocuPilot will run smoothly in local demo mode with deterministic mock embeddings and answers).*

2. **Start all services:**
   ```bash
   docker compose up --build
   ```

3. **Open DocuPilot:**
   * **Frontend:** `http://localhost:3000`
   * **Backend API & Health:** `http://localhost:8080/health`

---

## 💻 Running Locally (Without Docker)

### 1. Run the Backend

```bash
cd backend
# Optional: export GEMINI_API_KEY="your_api_key"
# Optional: export DATABASE_URL="postgres://user:pass@localhost:5432/docupilot?sslmode=disable"
go run main.go
```
*Runs on `http://localhost:8080`.*

### 2. Run the Frontend

In a separate terminal:
```bash
cd frontend
npm install
npm run dev
```
*Runs on `http://localhost:3000`.*

### 3. Local Conversion Prerequisites (For Document Converter Studio)

To run document conversions locally, install headless LibreOffice:
```bash
sudo apt update && sudo apt install -y libreoffice
```
*(When using Docker Compose, LibreOffice is pre-installed inside the backend container).*

---

## 🧪 Automated Tests & Quality Checks

Run all backend unit and integration tests:
```bash
cd backend
go test -v ./...
```
*(Tests cover API endpoints, AI providers, PDF extraction and header validation, worker queues, semantic chunking, and vector similarity search).*

Verify frontend TypeScript compilation and build:
```bash
cd frontend
npm run build
```

---

## 📋 Milestone Verification Summary

| Milestone | Description | Status |
| :--- | :--- | :--- |
| **0. Foundations** | Scope, stack agreements, and architecture |  Complete |
| **1. Minimal Version** | Go + Gin service, React frontend, pasted text flow |  Complete |
| **2. Connect Gemini** | Grounded Gemini AI integration with provider abstraction |  Complete |
| **3. PDF Support** | Pure Go PDF text extraction with type and size checks |  Complete |
| **4. Persistence** | Document metadata, sessions, and Q&A history |  Complete |
| **5. Semantic Search** | Overlapping passage chunking, embeddings, and vector retrieval |  Complete |
| **6. Background Jobs** | Asynchronous worker pool for extraction and indexing |  Complete |
| **7. Quality Checks** | 100% passing automated test suite for backend and frontend |  Complete |
| **8. Packaging & Deploy** | Multi-stage Dockerfiles and Docker Compose orchestration |  Complete |
| **9. Document Converter** | Bidirectional 8-format conversion studio with Q&A integration |  Complete |
