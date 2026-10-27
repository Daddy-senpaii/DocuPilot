import React, { useRef } from 'react';
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

  return (
    <div className="upload-section">
      <label className="upload-dropzone">
        <input
          ref={fileInputRef}
          type="file"
          accept=".pdf,application/pdf"
          onChange={handleFileChange}
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
  );
};
