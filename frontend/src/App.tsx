import React, { useState, useEffect, useCallback } from 'react';
import {
  User,
  DocumentItem,
  QAItem,
  ConversionPair,
  ConversionResult,
  ActiveTab,
  DEFAULT_CONVERSION_PAIRS,
  API_BASE_URL,
} from './types';
import { Header } from './components/Header';
import { AuthModal } from './components/AuthModal';
import { ApiKeyModal } from './components/ApiKeyModal';
import { DocumentUploadTab } from './components/DocumentUploadTab';
import { PasteTextTab } from './components/PasteTextTab';
import { DocumentLibraryTab } from './components/DocumentLibraryTab';
import { ConverterStudioTab } from './components/ConverterStudioTab';
import { ChatSection } from './components/ChatSection';

export default function App() {
  const [activeTab, setActiveTab] = useState<ActiveTab>('upload');
  const [text, setText] = useState('');
  const [activeDoc, setActiveDoc] = useState<DocumentItem | null>(null);
  const [documents, setDocuments] = useState<DocumentItem[]>([]);
  const [history, setHistory] = useState<QAItem[]>([]);

  const [question, setQuestion] = useState('');
  const [answer, setAnswer] = useState<string | null>(null);
  const [sources, setSources] = useState<string[]>([]);
  const [copiedAnswer, setCopiedAnswer] = useState(false);

  const [loading, setLoading] = useState(false);
  const [isStreaming, setIsStreaming] = useState(false);
  const [uploading, setUploading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [statusMessage, setStatusMessage] = useState<string | null>(null);

  // Authentication state
  const [authToken, setAuthToken] = useState<string | null>(() => localStorage.getItem('docupilot_auth_token'));
  const [currentUser, setCurrentUser] = useState<User | null>(null);
  const [showAuthModal, setShowAuthModal] = useState(false);

  // Gemini API key state
  const [apiKey, setApiKey] = useState<string>(() => localStorage.getItem('docupilot_gemini_key') || '');
  const [showKeyModal, setShowKeyModal] = useState(false);

  // Document Converter state
  const [convFile, setConvFile] = useState<File | null>(null);
  const [selectedPair, setSelectedPair] = useState<ConversionPair>(DEFAULT_CONVERSION_PAIRS[0]);
  const [convLoading, setConvLoading] = useState(false);
  const [convError, setConvError] = useState<string | null>(null);
  const [convResult, setConvResult] = useState<ConversionResult | null>(null);

  // Builds standard headers including optional Bearer JWT token and Gemini API key
  const getAuthHeaders = useCallback((additionalHeaders: Record<string, string> = {}) => {
    const headers: Record<string, string> = { ...additionalHeaders };
    if (authToken) {
      headers['Authorization'] = `Bearer ${authToken}`;
    }
    if (apiKey.trim()) {
      headers['X-Gemini-Api-Key'] = apiKey.trim();
    }
    return headers;
  }, [authToken, apiKey]);

  // Verify and load current user profile on token change
  useEffect(() => {
    if (!authToken) {
      setCurrentUser(null);
      return;
    }

    const verifyToken = async () => {
      try {
        const res = await fetch(`${API_BASE_URL}/api/auth/me`, {
          headers: {
            Authorization: `Bearer ${authToken}`,
          },
        });
        if (res.ok) {
          const data = await res.json();
          setCurrentUser(data.user);
        } else if (res.status === 401) {
          localStorage.removeItem('docupilot_auth_token');
          setAuthToken(null);
          setCurrentUser(null);
        }
      } catch {
        // Backend temporarily unavailable
      }
    };

    verifyToken();
  }, [authToken]);

  // Handle ESC key to dismiss open modals smoothly
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        if (showAuthModal) setShowAuthModal(false);
        if (showKeyModal) setShowKeyModal(false);
      }
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [showAuthModal, showKeyModal]);

  const fetchDocuments = useCallback(async () => {
    try {
      const res = await fetch(`${API_BASE_URL}/api/documents`, {
        headers: getAuthHeaders(),
      });
      if (res.ok) {
        const data = await res.json();
        setDocuments(data || []);
      }
    } catch {
      // Backend starting up
    }
  }, [getAuthHeaders]);

  const fetchDocumentDetails = useCallback(async (docId: number) => {
    try {
      const res = await fetch(`${API_BASE_URL}/api/documents/${docId}`, {
        headers: getAuthHeaders(),
      });
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
  }, [getAuthHeaders]);

  useEffect(() => {
    fetchDocuments();
  }, [fetchDocuments, authToken]);

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

  const handleSaveApiKey = (newKey: string) => {
    setApiKey(newKey);
    if (newKey) {
      localStorage.setItem('docupilot_gemini_key', newKey);
    } else {
      localStorage.removeItem('docupilot_gemini_key');
    }
  };

  const uploadPdfFile = async (file: File) => {
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
        headers: getAuthHeaders(),
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
      setActiveTab('upload');
    } catch (err: unknown) {
      if (err instanceof Error) {
        setError(err.message);
      } else {
        setError('An unexpected error occurred during PDF upload.');
      }
    } finally {
      setUploading(false);
    }
  };

  const handleConverterFileSelect = (file: File) => {
    setConvFile(file);
    setConvError(null);
    setConvResult(null);

    const ext = file.name.split('.').pop()?.toLowerCase() || '';
    const matchingPair = DEFAULT_CONVERSION_PAIRS.find((p) => p.source_format === ext);
    if (matchingPair) {
      setSelectedPair(matchingPair);
    }
  };

  const handleRunConversion = async () => {
    if (!convFile) {
      setConvError('Please choose a document file to convert.');
      return;
    }

    setConvLoading(true);
    setConvError(null);
    setConvResult(null);

    const formData = new FormData();
    formData.append('file', convFile);
    formData.append('source_format', selectedPair.source_format);
    formData.append('target_format', selectedPair.target_format);

    try {
      const res = await fetch(`${API_BASE_URL}/api/convert`, {
        method: 'POST',
        headers: getAuthHeaders(),
        body: formData,
      });

      if (!res.ok) {
        let errorMsg = 'Conversion failed.';
        try {
          const errData = await res.json();
          if (errData.error) errorMsg = errData.error;
        } catch {
          // ignore
        }
        throw new Error(errorMsg);
      }

      const blob = await res.blob();
      let outputFilename = '';
      const disp = res.headers.get('content-disposition');
      if (disp && disp.includes('filename=')) {
        const match = disp.match(/filename="?([^";]+)"?/);
        if (match && match[1]) {
          outputFilename = match[1];
        }
      }
      if (!outputFilename) {
        const base = convFile.name.substring(0, convFile.name.lastIndexOf('.')) || convFile.name;
        outputFilename = `${base}.${selectedPair.target_format}`;
      }

      let textPreview: string | undefined;
      if (selectedPair.target_format === 'txt') {
        textPreview = await blob.text();
      }

      const blobUrl = URL.createObjectURL(blob);
      setConvResult((prev) => {
        if (prev?.blobUrl) {
          URL.revokeObjectURL(prev.blobUrl);
        }
        return {
          filename: outputFilename,
          blob,
          blobUrl,
          size: blob.size,
          targetFormat: selectedPair.target_format,
          textPreview,
        };
      });
    } catch (err: unknown) {
      if (err instanceof Error) {
        setConvError(err.message);
      } else {
        setConvError('An unexpected conversion error occurred.');
      }
    } finally {
      setConvLoading(false);
    }
  };

  const handleCopyAnswer = () => {
    if (!answer) return;
    navigator.clipboard.writeText(answer);
    setCopiedAnswer(true);
    setTimeout(() => setCopiedAnswer(false), 2000);
  };

  const handleSendConvertedToQA = async () => {
    if (!convResult) return;

    if (convResult.targetFormat === 'pdf') {
      const pdfFile = new File([convResult.blob], convResult.filename, { type: 'application/pdf' });
      await uploadPdfFile(pdfFile);
    } else if (convResult.targetFormat === 'txt' && convResult.textPreview) {
      setText(convResult.textPreview);
      setActiveDoc(null);
      setActiveTab('paste');
      setStatusMessage(`Loaded extracted text (${convResult.textPreview.length.toLocaleString()} characters) into Q&A!`);
    }
  };

  const handleDeleteDocument = async (id: number) => {
    try {
      const res = await fetch(`${API_BASE_URL}/api/documents/${id}`, {
        method: 'DELETE',
        headers: getAuthHeaders(),
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
      } else {
        const data = await res.json();
        setError(data.error || 'Failed to delete document');
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
    setIsStreaming(true);
    setError(null);
    setAnswer('');
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

      const response = await fetch(`${API_BASE_URL}/api/ask/stream`, {
        method: 'POST',
        headers: getAuthHeaders({
          'Content-Type': 'application/json',
        }),
        body: JSON.stringify(payload),
      });

      if (!response.ok) {
        const data = await response.json().catch(() => ({}));
        throw new Error(data.error || `Server responded with status ${response.status}`);
      }

      if (!response.body) {
        throw new Error('Streaming response is unavailable on this device/browser.');
      }

      const reader = response.body.getReader();
      const decoder = new TextDecoder();
      let buffer = '';

      while (true) {
        const { value, done } = await reader.read();
        if (done) break;

        buffer += decoder.decode(value, { stream: true });
        const blocks = buffer.split('\n\n');
        buffer = blocks.pop() || '';

        for (const block of blocks) {
          if (!block.trim()) continue;
          let eventType = 'message';
          let eventData = '';

          const lines = block.split('\n');
          for (const line of lines) {
            if (line.startsWith('event: ')) {
              eventType = line.slice(7).trim();
            } else if (line.startsWith('data: ')) {
              eventData = line.slice(6);
            }
          }

          if (!eventData) continue;

          try {
            const parsed = JSON.parse(eventData);
            if (eventType === 'sources' && Array.isArray(parsed.sources)) {
              setSources(parsed.sources);
            } else if (eventType === 'chunk' && typeof parsed.chunk === 'string') {
              setAnswer((prev) => (prev || '') + parsed.chunk);
            } else if (eventType === 'done') {
              if (parsed.answer) {
                setAnswer(parsed.answer);
              }
              if (activeDoc) {
                fetchDocumentDetails(activeDoc.id);
              }
            } else if (eventType === 'error') {
              throw new Error(parsed.error || 'Error streaming answer');
            }
          } catch (err: unknown) {
            if (eventType === 'error') {
              throw err;
            }
          }
        }
      }
    } catch (err: unknown) {
      if (err instanceof Error) {
        setError(err.message);
      } else {
        setError('An unexpected error occurred while communicating with the server.');
      }
    } finally {
      setLoading(false);
      setIsStreaming(false);
    }
  };

  const handleAuthSuccess = (token: string, user: User, message: string) => {
    localStorage.setItem('docupilot_auth_token', token);
    setAuthToken(token);
    setCurrentUser(user);
    setStatusMessage(message);
  };

  // Check for OAuth redirect callback (e.g. GitHub callback code)
  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const code = params.get('code');
    if (code) {
      window.history.replaceState({}, document.title, window.location.pathname);
      fetch(`${API_BASE_URL}/api/auth/oauth/github`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ code }),
      })
        .then((res) => (res.ok ? res.json() : null))
        .then((data) => {
          if (data?.token && data?.user) {
            handleAuthSuccess(data.token, data.user, 'Signed in successfully with GitHub!');
          }
        })
        .catch(() => {});
    }
  }, []);

  const handleLogout = () => {
    localStorage.removeItem('docupilot_auth_token');
    setAuthToken(null);
    setCurrentUser(null);
    setActiveDoc(null);
    setText('');
    setHistory([]);
    setAnswer(null);
    setSources([]);
    setStatusMessage('You have signed out successfully.');
  };

  return (
    <div className="container">
      <Header
        apiKey={apiKey}
        onOpenKeyModal={() => setShowKeyModal(true)}
        currentUser={currentUser}
        onLogout={handleLogout}
        onOpenAuthModal={() => setShowAuthModal(true)}
      />

      <AuthModal
        isOpen={showAuthModal}
        onClose={() => setShowAuthModal(false)}
        onAuthSuccess={handleAuthSuccess}
      />

      <ApiKeyModal
        isOpen={showKeyModal}
        apiKey={apiKey}
        onClose={() => setShowKeyModal(false)}
        onSaveKey={handleSaveApiKey}
      />

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
          <button
            type="button"
            className={`tab-btn ${activeTab === 'converter' ? 'active' : ''}`}
            onClick={() => setActiveTab('converter')}
          >
            🔄 File Converter
            <span className="tab-pill-badge">NEW</span>
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
          <DocumentUploadTab
            activeDoc={activeDoc}
            text={text}
            uploading={uploading}
            onUploadFile={uploadPdfFile}
          />
        )}

        {activeTab === 'paste' && (
          <PasteTextTab
            text={text}
            onChangeText={(newText) => {
              setText(newText);
              setActiveDoc(null);
            }}
            disabled={loading}
          />
        )}

        {activeTab === 'library' && (
          <DocumentLibraryTab
            documents={documents}
            activeDoc={activeDoc}
            onSelectDocument={handleSelectDocument}
            onDeleteDocument={handleDeleteDocument}
          />
        )}

        {activeTab === 'converter' && (
          <ConverterStudioTab
            selectedPair={selectedPair}
            onSelectPair={(pair) => {
              setSelectedPair(pair);
              setConvError(null);
              setConvResult(null);
            }}
            convFile={convFile}
            onFileSelect={handleConverterFileSelect}
            convLoading={convLoading}
            convError={convError}
            convResult={convResult}
            onRunConversion={handleRunConversion}
            onSendConvertedToQA={handleSendConvertedToQA}
            onReset={() => {
              setConvFile(null);
              setConvResult(null);
              setConvError(null);
            }}
          />
        )}

        {activeTab !== 'converter' && (
          <ChatSection
            activeDoc={activeDoc}
            text={text}
            question={question}
            onChangeQuestion={setQuestion}
            onAsk={handleAsk}
            loading={loading}
            isStreaming={isStreaming}
            uploading={uploading}
            answer={answer}
            sources={sources}
            history={history}
            copiedAnswer={copiedAnswer}
            onCopyAnswer={handleCopyAnswer}
          />
        )}
      </div>
    </div>
  );
}
