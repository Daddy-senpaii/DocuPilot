import React, { useRef, useState } from 'react';
import { DocumentItem } from '../types';

interface DocumentUploadTabProps {
  activeDoc: DocumentItem | null;
  text: string;
  uploading: boolean;
  onUploadFile: (file: File) => void;
}

export const DocumentUploadTab: React.FC<DocumentUploadTabProps> = ({
  activeDoc,
  text,
  uploading,
  onUploadFile,
}) => {
  const [dragOver, setDragOver] = useState(false);
  const fileInputRef = useRef<HTMLInputElement | null>(null);

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (file) {
      onUploadFile(file);
    }
    if (fileInputRef.current) {
      fileInputRef.current.value = '';
    }
  };

  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault();
    setDragOver(false);
    const file = e.dataTransfer.files?.[0];
    if (file) {
      onUploadFile(file);
    }
  };

  return (
    <div className="upload-section">
      <label
        className={`upload-dropzone ${dragOver ? 'drag-over' : ''} ${uploading ? 'is-uploading' : ''}`}
        onDragOver={(e) => {
          e.preventDefault();
          setDragOver(true);
        }}
        onDragLeave={() => setDragOver(false)}
        onDrop={handleDrop}
      >
        <input
          ref={fileInputRef}
          type="file"
          accept=".pdf,application/pdf"
          onChange={handleFileChange}
          disabled={uploading}
          style={{ display: 'none' }}
        />
        <div className="upload-icon-wrapper">
          <div className="upload-icon">{uploading ? '⏳' : '📄'}</div>
          {uploading && <div className="upload-spinner-ring" />}
        </div>
        <p className="upload-prompt">
          {uploading ? 'Extracting text and indexing passages...' : 'Drop your PDF here or click to browse'}
        </p>
        <div className="upload-features-row">
          <span className="upload-pill">⚡ Instant Extraction</span>
          <span className="upload-pill">🔍 Vector RAG Indexing</span>
          <span className="upload-pill">🛡️ Up to 10MB</span>
        </div>

        {activeDoc && (
          <div className="active-doc-banner">
            <span className="active-doc-icon">📑</span>
            <div className="active-doc-details">
              <span className="active-doc-label">Active Document:</span>
              <strong className="active-doc-title">{activeDoc.filename}</strong>
              <span className="active-doc-chars">({activeDoc.char_count.toLocaleString()} chars)</span>
            </div>
            <span className={`status-pill ${activeDoc.status}`}>
              <span className="status-dot" />
              {activeDoc.status}
            </span>
          </div>
        )}
      </label>

      {text && (
        <details className="text-preview">
          <summary>
            <span>📜 View Extracted Text Preview</span>
            <span className="preview-badge">{text.length.toLocaleString()} characters</span>
          </summary>
          <pre>{text.slice(0, 1000)}{text.length > 1000 ? '...\n\n[preview truncated — full document indexed into RAG context]' : ''}</pre>
        </details>
      )}
    </div>
  );
};

