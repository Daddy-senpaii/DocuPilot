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
        <p className="empty-msg">No documents saved yet. Upload a PDF to get started!</p>
      ) : (
        <ul className="doc-list">
          {documents.map((doc) => (
            <li
              key={doc.id}
              className={`doc-item ${activeDoc?.id === doc.id ? 'selected' : ''}`}
            >
              <div className="doc-info" onClick={() => onSelectDocument(doc)}>
                <strong>{doc.filename}</strong>
                <span className="doc-meta">
                  {doc.char_count.toLocaleString()} chars • {new Date(doc.created_at).toLocaleDateString()}
                </span>
                <span className={`status-pill ${doc.status}`}>{doc.status}</span>
              </div>
              <button
                type="button"
                className="btn-delete"
                onClick={() => onDeleteDocument(doc.id)}
                title="Delete document"
              >
                🗑️
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
};
