import React, { useState, useEffect, useRef, useCallback } from 'react';

const API_BASE_URL = 'http://localhost:8080';

interface DocumentItem {
  id: number;
  filename: string;
  char_count: number;
  status: string;
  created_at: string;
}

interface QAItem {
  id: number;
  question: string;
  answer: string;
  created_at: string;
}

export default function App() {
  const [activeTab, setActiveTab] = useState<'upload' | 'paste' | 'library'>('upload');
  const [text, setText] = useState('');
  const [activeDoc, setActiveDoc] = useState<DocumentItem | null>(null);
  const [documents, setDocuments] = useState<DocumentItem[]>([]);
  const [history, setHistory] = useState<QAItem[]>([]);

  const [question, setQuestion] = useState('');
  const [answer, setAnswer] = useState<string | null>(null);
  const [sources, setSources] = useState<string[]>([]);

  const [loading, setLoading] = useState(false);
  const [uploading, setUploading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [statusMessage, setStatusMessage] = useState<string | null>(null);

  // Gemini API key state (stored in localStorage)
  const [apiKey, setApiKey] = useState<string>(() => localStorage.getItem('docupilot_gemini_key') || '');
  const [showKeyModal, setShowKeyModal] = useState<boolean>(false);
  const [tempKey, setTempKey] = useState<string>(apiKey);

  const fileInputRef = useRef<HTMLInputElement | null>(null);

  const fetchDocuments = useCallback(async () => {
    try {
      const res = await fetch(`${API_BASE_URL}/api/documents`);
      if (res.ok) {
        const data = await res.json();
        setDocuments(data || []);
      }
    } catch {
      // Non-fatal if backend is starting up
    }
  }, []);

  const fetchDocumentDetails = useCallback(async (docId: number) => {
    try {
      const res = await fetch(`${API_BASE_URL}/api/documents/${docId}`);
      if (res.ok) {
        const data = await res.json();
        if (data.document) {
          setActiveDoc(data.document);
          setText(data.document.content);
        }
        if (data.history) {
          setHistory(data.history);
        }
      }
    } catch {
      // Ignore
    }
  }, []);

  useEffect(() => {
    fetchDocuments();
  }, [fetchDocuments]);

  // Poll processing status if active document is processing
  useEffect(() => {
    if (!activeDoc || activeDoc.status !== 'processing') return;

    const interval = setInterval(async () => {
      try {
        const res = await fetch(`${API_BASE_URL}/api/documents/${activeDoc.id}`);
        if (res.ok) {
          const data = await res.json();
          if (data.document && data.document.status !== 'processing') {
            setActiveDoc(data.document);
            fetchDocuments();
            clearInterval(interval);
          }
        }
      } catch {
        // Ignore
      }
    }, 2000);

    return () => clearInterval(interval);
  }, [activeDoc, fetchDocuments]);

  const handleSaveApiKey = () => {
    const trimmed = tempKey.trim();
    setApiKey(trimmed);
    if (trimmed) {
      localStorage.setItem('docupilot_gemini_key', trimmed);
    } else {
      localStorage.removeItem('docupilot_gemini_key');
    }
    setShowKeyModal(false);
  };

  const handleFileUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

    if (!file.name.toLowerCase().endsWith('.pdf')) {
      setError('Please select a valid PDF file (.pdf).');
      return;
    }

    if (file.size > 10 * 1024 * 1024) {
      setError('File size exceeds the 10MB limit.');
      return;
    }

    setUploading(true);
    setError(null);
    setStatusMessage('Uploading PDF and extracting text...');

    const formData = new FormData();
    formData.append('file', file);

    try {
      const response = await fetch(`${API_BASE_URL}/api/upload`, {
        method: 'POST',
        body: formData,
      });

      const data = await response.json();

      if (!response.ok) {
        throw new Error(data.error || 'Failed to extract text from PDF');
      }

      setText(data.text);
      const newDoc: DocumentItem = {
        id: data.id,
        filename: data.filename,
        char_count: data.char_count,
        status: data.status,
        created_at: new Date().toISOString(),
      };
      setActiveDoc(newDoc);
      setStatusMessage(`Extracted ${data.char_count.toLocaleString()} characters. Semantic indexing running in background.`);
      setAnswer(null);
      setSources([]);
      fetchDocuments();
    } catch (err: unknown) {
      if (err instanceof Error) {
        setError(err.message);
      } else {
        setError('An unexpected error occurred during PDF upload.');
      }
    } finally {
      setUploading(false);
      if (fileInputRef.current) {
        fileInputRef.current.value = '';
      }
    }
  };

  const handleDeleteDocument = async (id: number) => {
    try {
      const res = await fetch(`${API_BASE_URL}/api/documents/${id}`, {
        method: 'DELETE',
      });
      if (res.ok) {
        if (activeDoc?.id === id) {
          setActiveDoc(null);
          setText('');
          setHistory([]);
          setAnswer(null);
          setSources([]);
        }
        fetchDocuments();
      }
    } catch (err: unknown) {
      if (err instanceof Error) setError(err.message);
    }
  };

  const handleSelectDocument = (doc: DocumentItem) => {
    setActiveDoc(doc);
    fetchDocumentDetails(doc.id);
    setActiveTab('upload');
    setAnswer(null);
    setSources([]);
  };

  const handleAsk = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!text.trim() && !activeDoc) {
      setError('Please upload a document or paste text before asking.');
      return;
    }
    if (!question.trim()) {
      setError('Please enter a question to ask.');
      return;
    }

    setLoading(true);
    setError(null);
    setAnswer(null);
    setSources([]);

    try {
      const payload: { question: string; document_id?: number; text?: string } = {
        question: question.trim(),
      };

      if (activeDoc) {
        payload.document_id = activeDoc.id;
      } else {
        payload.text = text.trim();
      }

      const headers: Record<string, string> = {
        'Content-Type': 'application/json',
      };
      if (apiKey.trim()) {
        headers['X-Gemini-Api-Key'] = apiKey.trim();
      }

      const response = await fetch(`${API_BASE_URL}/api/ask`, {
        method: 'POST',
        headers,
        body: JSON.stringify(payload),
      });

      const data = await response.json();

      if (!response.ok) {
        throw new Error(data.error || `Server responded with status ${response.status}`);
      }

      setAnswer(data.answer);
      if (data.sources) {
        setSources(data.sources);
      }

      if (activeDoc) {
        fetchDocumentDetails(activeDoc.id);
      }
    } catch (err: unknown) {
      if (err instanceof Error) {
        setError(err.message);
      } else {
        setError('An unexpected error occurred while communicating with the server.');
      }
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="container">
      <header className="header">
        <div className="header-top">
          <h1>✈️ DocuPilot</h1>
          <button
            type="button"
            className={`btn-api-key ${apiKey ? 'configured' : ''}`}
            onClick={() => {
              setTempKey(apiKey);
              setShowKeyModal(true);
            }}
          >
            {apiKey ? '🔑 API Key: Configured' : '🔑 Set Gemini API Key'}
          </button>
        </div>
        <p className="badge">AI PDF Assistant • Semantic Search & Vector Retrieval</p>
      </header>

      {showKeyModal && (
        <div className="modal-backdrop">
          <div className="modal-content">
            <h3>🔑 Configure Gemini API Key</h3>
            <p className="modal-desc">
              Your API key will be safely stored in your browser session and sent directly with requests to your local backend.
            </p>
            <div className="form-group">
              <label htmlFor="api-key-input">Gemini API Key:</label>
              <input
                id="api-key-input"
                type="password"
                placeholder="AIzaSy..."
                value={tempKey}
                onChange={(e) => setTempKey(e.target.value)}
              />
            </div>
            <div className="modal-actions">
              <button type="button" className="btn-secondary" onClick={() => setShowKeyModal(false)}>
                Cancel
              </button>
              <button type="button" className="btn-primary-inline" onClick={handleSaveApiKey}>
                Save Key
              </button>
            </div>
          </div>
        </div>
      )}

      <div className="card">
        <nav className="tab-nav">
          <button
            type="button"
            className={`tab-btn ${activeTab === 'upload' ? 'active' : ''}`}
            onClick={() => setActiveTab('upload')}
          >
            📄 Document
          </button>
          <button
            type="button"
            className={`tab-btn ${activeTab === 'paste' ? 'active' : ''}`}
            onClick={() => {
              setActiveTab('paste');
              setActiveDoc(null);
            }}
          >
            ✏️ Paste Text
          </button>
          <button
            type="button"
            className={`tab-btn ${activeTab === 'library' ? 'active' : ''}`}
            onClick={() => {
              setActiveTab('library');
              fetchDocuments();
            }}
          >
            📚 Library ({documents.length})
          </button>
        </nav>

        {error && (
          <div className="alert-error" role="alert">
            {error}
          </div>
        )}

        {statusMessage && (
          <div className="alert-success" role="status">
            {statusMessage}
          </div>
        )}

        {activeTab === 'upload' && (
          <div className="upload-section">
            <label className="upload-dropzone">
              <input
                ref={fileInputRef}
                type="file"
                accept=".pdf,application/pdf"
                onChange={handleFileUpload}
                disabled={uploading}
                style={{ display: 'none' }}
              />
              <div className="upload-icon">📁</div>
              <p className="upload-prompt">
                {uploading ? 'Extracting text and indexing...' : 'Click or drop a PDF here (up to 10MB)'}
              </p>
              {activeDoc && (
                <div className="active-doc-banner">
                  Active Document: <strong>{activeDoc.filename}</strong>
                  <span className={`status-pill ${activeDoc.status}`}>
                    {activeDoc.status}
                  </span>
                </div>
              )}
            </label>

            {text && (
              <details className="text-preview">
                <summary>View Extracted Text Preview ({text.length} chars)</summary>
                <pre>{text.slice(0, 1000)}{text.length > 1000 ? '...\n[truncated in preview]' : ''}</pre>
              </details>
            )}
          </div>
        )}

        {activeTab === 'paste' && (
          <div className="form-group">
            <label htmlFor="document-text">Raw Document Text / Notes:</label>
            <textarea
              id="document-text"
              rows={8}
              value={text}
              onChange={(e) => {
                setText(e.target.value);
                setActiveDoc(null);
              }}
              placeholder="Paste raw document text or notes here..."
              disabled={loading}
            />
          </div>
        )}

        {activeTab === 'library' && (
          <div className="library-section">
            {documents.length === 0 ? (
              <p className="empty-msg">No documents saved yet. Upload a PDF to get started!</p>
            ) : (
              <ul className="doc-list">
                {documents.map((doc) => (
                  <li key={doc.id} className={`doc-item ${activeDoc?.id === doc.id ? 'selected' : ''}`}>
                    <div className="doc-info" onClick={() => handleSelectDocument(doc)}>
                      <strong>{doc.filename}</strong>
                      <span className="doc-meta">
                        {doc.char_count.toLocaleString()} chars • {new Date(doc.created_at).toLocaleDateString()}
                      </span>
                      <span className={`status-pill ${doc.status}`}>{doc.status}</span>
                    </div>
                    <button
                      type="button"
                      className="btn-delete"
                      onClick={() => handleDeleteDocument(doc.id)}
                      title="Delete document"
                    >
                      🗑️
                    </button>
                  </li>
                ))}
              </ul>
            )}
          </div>
        )}

        <hr className="divider" />

        <form onSubmit={handleAsk}>
          <div className="form-group">
            <label htmlFor="question">Ask a question about this document:</label>
            <input
              type="text"
              id="question"
              value={question}
              onChange={(e) => setQuestion(e.target.value)}
              placeholder="e.g., What are the main recommendations?"
              disabled={loading || uploading}
              required
            />
          </div>

          <button
            type="submit"
            className="btn-primary"
            disabled={loading || uploading || (!text.trim() && !activeDoc) || !question.trim()}
          >
            {loading ? 'Searching & Generating Answer...' : 'Ask DocuPilot'}
          </button>
        </form>
      </div>

      {answer && (
        <section className="answer-card" aria-live="polite">
          <h2>💡 Answer</h2>
          <div className="answer-content">{answer}</div>

          {sources.length > 0 && (
            <details className="sources-section">
              <summary>Retrieved Sources ({sources.length} passages)</summary>
              <div className="sources-list">
                {sources.map((s, idx) => (
                  <pre key={idx} className="source-item">{s}</pre>
                ))}
              </div>
            </details>
          )}
        </section>
      )}

      {history.length > 0 && (
        <section className="history-card">
          <h2>📜 Previous Questions & Answers</h2>
          <div className="history-list">
            {history.map((item) => (
              <div key={item.id} className="history-item">
                <p className="history-q"><strong>Q:</strong> {item.question}</p>
                <p className="history-a"><strong>A:</strong> {item.answer}</p>
              </div>
            ))}
          </div>
        </section>
      )}
    </div>
  );
}
