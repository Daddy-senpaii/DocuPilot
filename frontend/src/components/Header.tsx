import React from 'react';
import { User } from '../types';

interface HeaderProps {
  apiKey: string;
  onOpenKeyModal: () => void;
  currentUser: User | null;
  onLogout: () => void;
  onOpenAuthModal: () => void;
}

export const Header: React.FC<HeaderProps> = ({
  apiKey,
  onOpenKeyModal,
  currentUser,
  onLogout,
  onOpenAuthModal,
}) => {
  return (
    <header className="header">
      <div className="header-top">
        <div className="brand-badge-group">
          <div className="brand-logo-icon" aria-hidden="true">
            <span className="logo-symbol">✈️</span>
            <div className="logo-glow" />
          </div>
          <div>
            <div className="brand-title-row">
              <h1 className="brand-title">DocuPilot</h1>
              <span className="brand-status-badge">
                <span className="pulse-dot" />
                <span>AI v2.0</span>
              </span>
            </div>
            <p className="brand-tagline">Intelligent Document QA & High-Speed Conversion Studio</p>
          </div>
        </div>
        <div className="header-controls">
          <button
            type="button"
            className={`btn-api-key ${apiKey ? 'configured' : ''}`}
            onClick={onOpenKeyModal}
            title={apiKey ? 'Gemini API Key is configured and ready' : 'Configure your Gemini API key'}
          >
            <span className="key-icon">🔑</span>
            <span>{apiKey ? 'API Key Active' : 'Set Gemini Key'}</span>
            {apiKey && <span className="active-indicator-dot" />}
          </button>

          {currentUser ? (
            <div className="user-badge-wrap">
              <span className="user-badge" title={`Signed in as ${currentUser.email}`}>
                <span className="user-avatar-initial">
                  {currentUser.email.charAt(0).toUpperCase()}
                </span>
                <span className="user-email-text">{currentUser.email}</span>
              </span>
              <button type="button" className="btn-logout" onClick={onLogout}>
                Log Out
              </button>
            </div>
          ) : (
            <button
              type="button"
              className="btn-auth"
              onClick={onOpenAuthModal}
            >
              <span className="auth-btn-icon">✨</span>
              <span>Sign In / Register</span>
            </button>
          )}
        </div>
      </div>
    </header>
  );
};

