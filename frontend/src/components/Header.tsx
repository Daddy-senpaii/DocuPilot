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
            onClick={onOpenKeyModal}
          >
            {apiKey ? '🔑 API Key: Active' : '🔑 Set Gemini API Key'}
          </button>

          {currentUser ? (
            <>
              <span className="user-badge" title={`Signed in as ${currentUser.email}`}>
                👤 {currentUser.email}
              </span>
              <button type="button" className="btn-logout" onClick={onLogout}>
                Log Out
              </button>
            </>
          ) : (
            <button
              type="button"
              className="btn-auth"
              onClick={onOpenAuthModal}
            >
              Sign In / Register
            </button>
          )}
        </div>
      </div>
    </header>
  );
};
