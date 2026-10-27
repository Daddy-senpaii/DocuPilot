import React, { useState, useEffect } from 'react';

interface ApiKeyModalProps {
  isOpen: boolean;
  apiKey: string;
  onClose: () => void;
  onSaveKey: (newKey: string) => void;
}

export const ApiKeyModal: React.FC<ApiKeyModalProps> = ({
  isOpen,
  apiKey,
  onClose,
  onSaveKey,
}) => {
  const [tempKey, setTempKey] = useState(apiKey);

  useEffect(() => {
    setTempKey(apiKey);
  }, [apiKey, isOpen]);

  if (!isOpen) return null;

  const handleSave = () => {
    onSaveKey(tempKey.trim());
    onClose();
  };

  return (
    <div
      className="modal-backdrop"
      onClick={(e) => {
        if (e.target === e.currentTarget) {
          onClose();
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
            onClick={onClose}
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
          <button type="button" className="btn-secondary" onClick={onClose}>
            Cancel
          </button>
          <button type="button" className="btn-primary-inline" onClick={handleSave}>
            Save Key
          </button>
        </div>
      </div>
    </div>
  );
};
