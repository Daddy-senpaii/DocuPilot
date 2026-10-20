import React, { useState, useEffect, useRef, useCallback } from 'react';

const API_BASE_URL = 'http://localhost:8080';

interface User {
  id: number;
  email: string;
  created_at: string;
}

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

interface ConversionPair {
  source_format: string;
  target_format: string;
  title: string;
  description: string;
  icon: string;
}

const DEFAULT_CONVERSION_PAIRS: ConversionPair[] = [
  { source_format: 'pdf', target_format: 'docx', title: 'PDF to Word', description: 'Convert PDF into editable Word document (.docx)', icon: '📄 ➡️ 📝' },
  { source_format: 'docx', target_format: 'pdf', title: 'Word to PDF', description: 'Render Word document (.docx) into standard PDF', icon: '📝 ➡️ 📄' },
  { source_format: 'pdf', target_format: 'xlsx', title: 'PDF to Excel', description: 'Convert PDF tables into Excel spreadsheet (.xlsx)', icon: '📄 ➡️ 📊' },
  { source_format: 'xlsx', target_format: 'pdf', title: 'Excel to PDF', description: 'Render Excel spreadsheet (.xlsx) into clean PDF', icon: '📊 ➡️ 📄' },
  { source_format: 'pdf', target_format: 'pptx', title: 'PDF to PowerPoint', description: 'Transform PDF pages into presentation slides (.pptx)', icon: '📄 ➡️ 📽️' },
  { source_format: 'pptx', target_format: 'pdf', title: 'PowerPoint to PDF', description: 'Render presentation slides (.pptx) into standard PDF deck', icon: '📽 ➡️ 📄' },
  { source_format: 'docx', target_format: 'txt', title: 'Word to Text', description: 'Extract plain text from Word document (.docx)', icon: '📝 ➡️ 📋' },
  { source_format: 'txt', target_format: 'pdf', title: 'Text to PDF', description: 'Typeset plain text (.txt) into paginated PDF document', icon: '📋 ➡️ 📄' },
];

const QUICK_PROMPTS = [
  'Summarize the key takeaways in 3 bullets',
  'What are the main findings or recommendations?',
  'Extract any important dates, numbers, or action items',
];

export default function App() {
  const [activeTab, setActiveTab] = useState<'upload' | 'paste' | 'library' | 'converter'>('upload');
  const [text, setText] = useState('');
  const [activeDoc, setActiveDoc] = useState<DocumentItem | null>(null);
  const [documents, setDocuments] = useState<DocumentItem[]>([]);
  const [history, setHistory] = useState<QAItem[]>([]);

  const [question, setQuestion] = useState('');
  const [answer, setAnswer] = useState<string | null>(null);
  const [sources, setSources] = useState<string[]>([]);
  const [copiedAnswer, setCopiedAnswer] = useState(false);

  const [loading, setLoading] = useState(false);
  const [uploading, setUploading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [statusMessage, setStatusMessage] = useState<string | null>(null);

  // Authentication state
  const [authToken, setAuthToken] = useState<string | null>(() => localStorage.getItem('docupilot_auth_token'));
  const [currentUser, setCurrentUser] = useState<User | null>(null);
  const [showAuthModal, setShowAuthModal] = useState<boolean>(false);
  const [authMode, setAuthMode] = useState<'login' | 'register'>('login');
  const [authEmail, setAuthEmail] = useState<string>('');
  const [authPassword, setAuthPassword] = useState<string>('');
  const [authConfirmPassword, setAuthConfirmPassword] = useState<string>('');
  const [authError, setAuthError] = useState<string | null>(null);
  const [authLoading, setAuthLoading] = useState<boolean>(false);
  const [showPassword, setShowPassword] = useState<boolean>(false);

  // Password validation helper flags
  const isMinLength = authPassword.length >= 8 && authPassword.length <= 72;
  const hasLetterAndNum = /[a-zA-Z]/.test(authPassword) && /[^a-zA-Z]/.test(authPassword);
  const passwordsMatch = authPassword.length > 0 && authPassword === authConfirmPassword;

  // Gemini API key state (stored in localStorage)
  const [apiKey, setApiKey] = useState<string>(() => localStorage.getItem('docupilot_gemini_key') || '');
  const [showKeyModal, setShowKeyModal] = useState<boolean>(false);
  const [tempKey, setTempKey] = useState<string>(apiKey);

  const fileInputRef = useRef<HTMLInputElement | null>(null);

  // Document Converter state
  const [convFile, setConvFile] = useState<File | null>(null);
  const [selectedPair, setSelectedPair] = useState<ConversionPair>(DEFAULT_CONVERSION_PAIRS[0]);
  const [convLoading, setConvLoading] = useState<boolean>(false);
  const [convError, setConvError] = useState<string | null>(null);
  const [convResult, setConvResult] = useState<{
    filename: string;
    blob: Blob;
    blobUrl: string;
    size: number;
    targetFormat: string;
    textPreview?: string;
  } | null>(null);
  const [convDragOver, setConvDragOver] = useState<boolean>(false);
  const convFileInputRef = useRef<HTMLInputElement | null>(null);

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
          // Token expired or invalid
          localStorage.removeItem('docupilot_auth_token');
          setAuthToken(null);
          setCurrentUser(null);
        }
      } catch {
        // Offline or backend unavailable
      }
    };

    verifyToken();
  }, [authToken]);

  // Handle ESC key to dismiss open modals smoothly
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        if (showAuthModal) {
          setShowAuthModal(false);
          setAuthError(null);
        }
        if (showKeyModal) {
          setShowKeyModal(false);
        }
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
      // Non-fatal if backend is starting up
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
      if (fileInputRef.current) {
        fileInputRef.current.value = '';
      }
    }
  };

  const handleFileUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (file) {
      await uploadPdfFile(file);
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

      const response = await fetch(`${API_BASE_URL}/api/ask`, {
        method: 'POST',
        headers: getAuthHeaders({
          'Content-Type': 'application/json',
        }),
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

  const handleAuthSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setAuthError(null);

    const cleanEmail = authEmail.trim();
    if (!cleanEmail || !cleanEmail.includes('@')) {
      setAuthError('Please enter a valid email address.');
      return;
    }

    if (authPassword.length < 8) {
      setAuthError('Password must be at least 8 characters long.');
      return;
    }

    if (authMode === 'register' && authPassword !== authConfirmPassword) {
      setAuthError('Passwords do not match.');
      return;
    }

    setAuthLoading(true);
    try {
      const res = await fetch(`${API_BASE_URL}/api/auth/${authMode}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: cleanEmail, password: authPassword }),
      });

      const data = await res.json();
      if (!res.ok) {
        throw new Error(data.error || 'Authentication request failed');
      }

      localStorage.setItem('docupilot_auth_token', data.token);
      setAuthToken(data.token);
      setCurrentUser(data.user);
      setShowAuthModal(false);
      setAuthPassword('');
      setAuthConfirmPassword('');
      setStatusMessage(authMode === 'register' ? 'Account created successfully! Welcome to DocuPilot.' : 'Signed in successfully.');
    } catch (err: unknown) {
      if (err instanceof Error) {
        setAuthError(err.message);
      } else {
        setAuthError('An unexpected authentication error occurred.');
      }
    } finally {
      setAuthLoading(false);
    }
  };

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
      <header className="header">
        <div className="header-top">
          <div className="brand-badge-group">
            <span className="brand-logo-icon">✈️</span>
            <div>
              <h1 className="brand-title">DocuPilot</h1>
              <p className="brand-tagline">AI PDF Assistant & Document Conversion Studio</p>
            </div>
          </div>
          <div className="header-controls">
            <button
              type="button"
              className={`btn-api-key ${apiKey ? 'configured' : ''}`}
              onClick={() => {
                setTempKey(apiKey);
                setShowKeyModal(true);
              }}
            >
              {apiKey ? '🔑 API Key: Active' : '🔑 Set Gemini API Key'}
            </button>

            {currentUser ? (
              <>
                <span className="user-badge" title={`Signed in as ${currentUser.email}`}>
                  👤 {currentUser.email}
                </span>
                <button type="button" className="btn-logout" onClick={handleLogout}>
                  Log Out
                </button>
              </>
            ) : (
              <button
                type="button"
                className="btn-auth"
                onClick={() => {
                  setAuthError(null);
                  setAuthMode('login');
                  setShowPassword(false);
                  setShowAuthModal(true);
                }}
              >
                Sign In / Register
              </button>
            )}
          </div>
        </div>
      </header>

      {/* Authentication Modal */}
      {showAuthModal && (
        <div
          className="modal-backdrop"
          onClick={(e) => {
            if (e.target === e.currentTarget) {
              setShowAuthModal(false);
              setAuthError(null);
            }
          }}
        >
          <div className="modal-content auth-modal">
            <div className="modal-header">
              <div className="modal-title-wrap auth-title-block">
                <span className="auth-brand-badge">
                  <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
                    <rect width="18" height="11" x="3" y="11" rx="2" ry="2" />
                    <path d="M7 11V7a5 5 0 0 1 10 0v4" />
                  </svg>
                  {authMode === 'login' ? 'Secure Sign In' : 'DocuPilot Access'}
                </span>
                <h2 className="auth-modal-title">
                  {authMode === 'login' ? 'Welcome back' : 'Create your account'}
                </h2>
                <p className="auth-modal-subtitle">
                  {authMode === 'login'
                    ? 'Enter your credentials to access your document library.'
                    : 'Start asking questions about your documents with grounded AI.'}
                </p>
              </div>
              <button
                type="button"
                className="modal-close-btn"
                onClick={() => {
                  setShowAuthModal(false);
                  setAuthError(null);
                }}
                aria-label="Close modal"
              >
                <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
                  <line x1="18" y1="6" x2="6" y2="18" />
                  <line x1="6" y1="6" x2="18" y2="18" />
                </svg>
              </button>
            </div>

            <div className="auth-segmented-switch">
              <button
                type="button"
                className={`auth-segment-btn ${authMode === 'login' ? 'active' : ''}`}
                onClick={() => {
                  setAuthMode('login');
                  setAuthError(null);
                }}
              >
                Sign In
              </button>
              <button
                type="button"
                className={`auth-segment-btn ${authMode === 'register' ? 'active' : ''}`}
                onClick={() => {
                  setAuthMode('register');
                  setAuthError(null);
                }}
              >
                Create Account
              </button>
            </div>

            {authError && (
              <div className="alert-error" role="alert">
                <span className="alert-icon">⚠️</span>
                <span>{authError}</span>
              </div>
            )}

            <form onSubmit={handleAuthSubmit} className="auth-form">
              <div className="form-group">
                <label htmlFor="auth-email">Email Address</label>
                <div className="input-with-icon">
                  <svg className="field-svg-icon" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                    <rect width="20" height="16" x="2" y="4" rx="2" />
                    <path d="m22 7-8.97 5.7a1.94 1.94 0 0 1-2.06 0L2 7" />
                  </svg>
                  <input
                    id="auth-email"
                    type="email"
                    required
                    placeholder="name@company.com"
                    value={authEmail}
                    onChange={(e) => setAuthEmail(e.target.value)}
                    autoComplete="email"
                  />
                </div>
              </div>

              <div className="form-group">
                <div className="label-row">
                  <label htmlFor="auth-password">Password</label>
                  {authMode === 'register' && (
                    <span className="label-hint">8–72 characters</span>
                  )}
                </div>
                <div className="input-with-icon">
                  <svg className="field-svg-icon" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                    <rect width="18" height="11" x="3" y="11" rx="2" ry="2" />
                    <path d="M7 11V7a5 5 0 0 1 10 0v4" />
                  </svg>
                  <input
                    id="auth-password"
                    type={showPassword ? 'text' : 'password'}
                    required
                    placeholder="••••••••"
                    value={authPassword}
                    onChange={(e) => setAuthPassword(e.target.value)}
                    autoComplete={authMode === 'login' ? 'current-password' : 'new-password'}
                  />
                  <button
                    type="button"
                    className="btn-toggle-password"
                    onClick={() => setShowPassword((prev) => !prev)}
                    title={showPassword ? 'Hide password' : 'Show password'}
                    aria-label={showPassword ? 'Hide password' : 'Show password'}
                  >
                    {showPassword ? (
                      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                        <path d="M9.88 9.88a3 3 0 1 0 4.24 4.24" />
                        <path d="M10.73 5.08A10.43 10.43 0 0 1 12 5c7 0 10 7 10 7a13.16 13.16 0 0 1-1.67 2.68" />
                        <path d="M6.61 6.61A13.526 13.526 0 0 0 2 12s3 7 10 7a9.74 9.74 0 0 0 5.39-1.61" />
                        <line x1="2" x2="22" y1="2" y2="22" />
                      </svg>
                    ) : (
                      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                        <path d="M2 12s3-7 10-7 10 7 10 7-3 7-10 7-10-7-10-7Z" />
                        <circle cx="12" cy="12" r="3" />
                      </svg>
                    )}
                  </button>
                </div>
              </div>

              {authMode === 'register' && (
                <>
                  <div className="form-group">
                    <label htmlFor="auth-confirm-password">Confirm Password</label>
                    <div className="input-with-icon">
                      <svg className="field-svg-icon" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                        <rect width="18" height="11" x="3" y="11" rx="2" ry="2" />
                        <path d="M7 11V7a5 5 0 0 1 10 0v4" />
                      </svg>
                      <input
                        id="auth-confirm-password"
                        type={showPassword ? 'text' : 'password'}
                        required
                        placeholder="••••••••"
                        value={authConfirmPassword}
                        onChange={(e) => setAuthConfirmPassword(e.target.value)}
                        autoComplete="new-password"
                      />
                    </div>
                  </div>

                  <div className="auth-criteria">
                    <div className={`criteria-item ${isMinLength ? 'met' : ''}`}>
                      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="3" strokeLinecap="round" strokeLinejoin="round">
                        <polyline points="20 6 9 17 4 12" />
                      </svg>
                      <span>8 to 72 characters</span>
                    </div>
                    <div className={`criteria-item ${hasLetterAndNum ? 'met' : ''}`}>
                      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="3" strokeLinecap="round" strokeLinejoin="round">
                        <polyline points="20 6 9 17 4 12" />
                      </svg>
                      <span>Contains letters & numbers/symbols</span>
                    </div>
                    {authConfirmPassword && (
                      <div className={`criteria-item ${passwordsMatch ? 'met' : 'unmet'}`}>
                        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="3" strokeLinecap="round" strokeLinejoin="round">
                          {passwordsMatch ? (
                            <polyline points="20 6 9 17 4 12" />
                          ) : (
                            <>
                              <line x1="18" y1="6" x2="6" y2="18" />
                              <line x1="6" y1="6" x2="18" y2="18" />
                            </>
                          )}
                        </svg>
                        <span>{passwordsMatch ? 'Passwords match' : 'Passwords do not match'}</span>
                      </div>
                    )}
                  </div>
                </>
              )}

              <div className="modal-actions auth-actions">
                <button
                  type="button"
                  className="btn-secondary"
                  onClick={() => {
                    setShowAuthModal(false);
                    setAuthError(null);
                  }}
                  disabled={authLoading}
                >
                  Cancel
                </button>
                <button type="submit" className="auth-submit-btn" disabled={authLoading}>
                  {authLoading ? (
                    <span className="btn-loading-content">
                      <span className="spinner" />
                      Please wait...
                    </span>
                  ) : authMode === 'login' ? (
                    'Sign In'
                  ) : (
                    'Create Account'
                  )}
                </button>
              </div>
            </form>

            <div className="auth-footer">
              {authMode === 'login' ? (
                <span>
                  Don&apos;t have an account yet?{' '}
                  <button
                    type="button"
                    className="auth-link"
                    onClick={() => {
                      setAuthMode('register');
                      setAuthError(null);
                    }}
                  >
                    Create one now
                  </button>
                </span>
              ) : (
                <span>
                  Already have an account?{' '}
                  <button
                    type="button"
                    className="auth-link"
                    onClick={() => {
                      setAuthMode('login');
                      setAuthError(null);
                    }}
                  >
                    Sign in
                  </button>
                </span>
              )}
            </div>
          </div>
        </div>
      )}

      {showKeyModal && (
        <div
          className="modal-backdrop"
          onClick={(e) => {
            if (e.target === e.currentTarget) {
              setShowKeyModal(false);
            }
          }}
        >
          <div className="modal-content">
            <div className="modal-header">
              <div className="modal-title-wrap">
                <div className="modal-icon-badge">🔑</div>
                <div>
                  <h3>Configure Gemini API Key</h3>
                  <p className="modal-desc">
                    Your API key will be safely stored in your browser session and sent directly with requests to your local backend.
                  </p>
                </div>
              </div>
              <button
                type="button"
                className="modal-close-btn"
                onClick={() => setShowKeyModal(false)}
                aria-label="Close modal"
              >
                ✕
              </button>
            </div>

            <div className="form-group">
              <label htmlFor="api-key-input">Gemini API Key</label>
              <div className="input-with-icon">
                <span className="input-icon">🔐</span>
                <input
                  id="api-key-input"
                  type="password"
                  placeholder="AIzaSy..."
                  value={tempKey}
                  onChange={(e) => setTempKey(e.target.value)}
                />
              </div>
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

        {activeTab === 'converter' && (
          <div className="converter-section">
            <div className="converter-header">
              <h3>Fast & High-Fidelity Document Converter</h3>
              <p className="converter-subtitle">
                Convert between PDF, Word (.docx), Excel (.xlsx), PowerPoint (.pptx), and Text (.txt).
              </p>
            </div>

            {/* Quick Conversion Workflows */}
            <div className="conversion-presets">
              <span className="presets-label">Choose Conversion:</span>
              <div className="presets-grid">
                {DEFAULT_CONVERSION_PAIRS.map((pair) => {
                  const isSelected =
                    selectedPair.source_format === pair.source_format &&
                    selectedPair.target_format === pair.target_format;
                  return (
                    <button
                      key={`${pair.source_format}->${pair.target_format}`}
                      type="button"
                      className={`preset-chip ${isSelected ? 'selected' : ''}`}
                      onClick={() => {
                        setSelectedPair(pair);
                        setConvError(null);
                        setConvResult(null);
                      }}
                    >
                      <span className="preset-icon">{pair.icon}</span>
                      <span className="preset-title">{pair.title}</span>
                    </button>
                  );
                })}
              </div>
            </div>

            {/* Drag & Drop Zone */}
            <div
              className={`upload-dropzone converter-dropzone ${convDragOver ? 'drag-over' : ''} ${convFile ? 'has-file' : ''}`}
              onDragOver={(e) => {
                e.preventDefault();
                setConvDragOver(true);
              }}
              onDragLeave={() => setConvDragOver(false)}
              onDrop={(e) => {
                e.preventDefault();
                setConvDragOver(false);
                const file = e.dataTransfer.files?.[0];
                if (file) handleConverterFileSelect(file);
              }}
              onClick={() => convFileInputRef.current?.click()}
            >
              <input
                ref={convFileInputRef}
                type="file"
                accept=".pdf,.docx,.xlsx,.pptx,.txt"
                onChange={(e) => {
                  const file = e.target.files?.[0];
                  if (file) handleConverterFileSelect(file);
                }}
                disabled={convLoading}
                style={{ display: 'none' }}
              />

              <div className="upload-icon">
                {convFile ? '📑' : '🔄'}
              </div>

              {convFile ? (
                <div className="converter-file-details">
                  <p className="converter-selected-name">
                    <strong>{convFile.name}</strong>
                  </p>
                  <span className="converter-file-size">
                    {(convFile.size / 1024).toFixed(1)} KB • Source: .{selectedPair.source_format.toUpperCase()}
                  </span>
                  <p className="converter-change-hint">Click or drop another file to change</p>
                </div>
              ) : (
                <>
                  <p className="upload-prompt">
                    Click or drop your <strong>.{selectedPair.source_format.toUpperCase()}</strong> file here
                  </p>
                  <span className="converter-hint">
                    Accepts .{selectedPair.source_format.toUpperCase()} (up to 10MB)
                  </span>
                </>
              )}
            </div>

            {/* Target Format Selector & Convert Action */}
            <div className="converter-action-bar">
              <div className="format-flow-badge">
                <span className="flow-badge-part source">.{selectedPair.source_format.toUpperCase()}</span>
                <span className="flow-arrow">➡️</span>
                <span className="flow-badge-part target">.{selectedPair.target_format.toUpperCase()}</span>
              </div>

              <button
                type="button"
                className="btn-primary converter-submit-btn"
                onClick={handleRunConversion}
                disabled={!convFile || convLoading}
              >
                {convLoading ? (
                  <span className="btn-loading-content">
                    <span className="spinner" />
                    Converting Document...
                  </span>
                ) : (
                  `Convert to .${selectedPair.target_format.toUpperCase()}`
                )}
              </button>
            </div>

            {convError && (
              <div className="alert-error converter-alert" role="alert">
                <strong>Conversion Error:</strong> {convError}
                {convError.includes('LibreOffice') && (
                  <div className="install-hint">
                    <code>sudo apt update && sudo apt install -y libreoffice</code>
                  </div>
                )}
              </div>
            )}

            {/* Converted Output Card */}
            {convResult && (
              <div className="converter-result-card" aria-live="polite">
                <div className="result-header">
                  <span className="result-badge-check">✅</span>
                  <div>
                    <h4 className="result-title">Conversion Complete!</h4>
                    <p className="result-meta">
                      <strong>{convResult.filename}</strong> ({(convResult.size / 1024).toFixed(1)} KB)
                    </p>
                  </div>
                </div>

                <div className="result-actions">
                  <a
                    href={convResult.blobUrl}
                    download={convResult.filename}
                    className="btn-download"
                  >
                    📥 Download {convResult.filename}
                  </a>

                  {(convResult.targetFormat === 'pdf' || convResult.targetFormat === 'txt') && (
                    <button
                      type="button"
                      className="btn-chat-doc"
                      onClick={handleSendConvertedToQA}
                    >
                      🤖 Ask Questions in DocuPilot
                    </button>
                  )}

                  <button
                    type="button"
                    className="btn-secondary"
                    onClick={() => {
                      setConvFile(null);
                      setConvResult(null);
                      setConvError(null);
                    }}
                  >
                    Convert Another
                  </button>
                </div>
              </div>
            )}
          </div>
        )}

        {activeTab !== 'converter' && (
          <>
            <hr className="divider" />

            <form onSubmit={handleAsk}>
              {(activeDoc || text.trim()) && (
                <div className="quick-prompts-bar">
                  <span className="quick-prompts-label">✨ Quick Prompts:</span>
                  <div className="quick-prompts-list">
                    {QUICK_PROMPTS.map((prompt, idx) => (
                      <button
                        key={idx}
                        type="button"
                        className="quick-prompt-btn"
                        onClick={() => setQuestion(prompt)}
                        disabled={loading || uploading}
                      >
                        {prompt}
                      </button>
                    ))}
                  </div>
                </div>
              )}

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
          </>
        )}
      </div>

      {answer && (
        <section className="answer-card" aria-live="polite">
          <div className="answer-header">
            <h2>💡 AI Grounded Answer</h2>
            <button
              type="button"
              className="btn-copy-answer"
              onClick={handleCopyAnswer}
              title="Copy answer to clipboard"
            >
              {copiedAnswer ? '✓ Copied!' : '📋 Copy Answer'}
            </button>
          </div>
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
