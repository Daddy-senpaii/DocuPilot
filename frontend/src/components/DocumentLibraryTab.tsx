import React from 'react';
import { DocumentItem } from '../types';

interface DocumentLibraryTabProps {
  documents: DocumentItem[];
  activeDoc: DocumentItem | null;
  onSelectDocument: (doc: DocumentItem) => void;
  onDeleteDocument: (id: number) => void;
}

export const DocumentLibraryTab: React.FC<DocumentLibraryTabProps> = ({
  documents,
  activeDoc,
  onSelectDocument,
  onDeleteDocument,
}) => {
  return (
    <div className="library-section">
      {documents.length === 0 ? (
        <div className="empty-library-state">
          <div className="empty-icon">📚</div>
          <h4>Your Document Library is Empty</h4>
          <p className="empty-msg">Upload a PDF or convert a document to build your intelligent knowledge base.</p>
        </div>
      ) : (
        <ul className="doc-list">
          {documents.map((doc) => {
            const isSelected = activeDoc?.id === doc.id;
            return (
              <li
                key={doc.id}
                className={`doc-item ${isSelected ? 'selected' : ''}`}
              >
                <div className="doc-item-main" onClick={() => onSelectDocument(doc)}>
                  <div className="doc-type-icon">📄</div>
                  <div className="doc-info">
                    <strong className="doc-title">{doc.filename}</strong>
                    <div className="doc-meta-row">
                      <span className="doc-meta">
                        {doc.char_count.toLocaleString()} chars
                      </span>
                      <span className="doc-meta-bullet">•</span>
                      <span className="doc-meta">
                        {new Date(doc.created_at).toLocaleDateString()}
                      </span>
                      <span className={`status-pill ${doc.status}`}>
                        <span className="status-dot" />
                        {doc.status}
                      </span>
                    </div>
                  </div>
                </div>
                <button
                  type="button"
                  className="btn-delete"
                  onClick={(e) => {
                    e.stopPropagation();
                    onDeleteDocument(doc.id);
                  }}
                  title="Delete document"
                  aria-label={`Delete ${doc.filename}`}
                >
                  <span className="delete-icon">🗑️</span>
                </button>
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
};

