import React, { useState, useEffect } from 'react';

interface GoogleAuthDialogProps {
  isOpen: boolean;
  onClose: () => void;
  onSelectAccount: (email: string) => void;
  onLaunchPopup?: () => void;
  loading: boolean;
  error: string | null;
}

interface SavedAccount {
  email: string;
  name: string;
  avatarChar: string;
}

const STORAGE_KEY = 'docupilot_saved_google_accounts';

export const GoogleAuthDialog: React.FC<GoogleAuthDialogProps> = ({
  isOpen,
  onClose,
  onSelectAccount,
  onLaunchPopup,
  loading,
  error,
}) => {
  const [emailInput, setEmailInput] = useState('');
  const [inputError, setInputError] = useState<string | null>(null);
  const [savedAccounts, setSavedAccounts] = useState<SavedAccount[]>([]);
  const [showInputMode, setShowInputMode] = useState(false);

  useEffect(() => {
    if (!isOpen) return;

    try {
      const stored = localStorage.getItem(STORAGE_KEY);
      if (stored) {
        const parsed: SavedAccount[] = JSON.parse(stored);
        if (Array.isArray(parsed) && parsed.length > 0) {
          setSavedAccounts(parsed);
          setShowInputMode(false);
          return;
        }
      }
    } catch {
      // Ignore JSON error
    }
    // Default to input mode if no accounts are saved yet
    setShowInputMode(true);
  }, [isOpen]);

  if (!isOpen) return null;

  const handleEmailSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    setInputError(null);

    const clean = emailInput.trim().toLowerCase();
    if (!clean) {
      setInputError('Please enter your Gmail address.');
      return;
    }

    if (!clean.includes('@') || !clean.includes('.')) {
      setInputError('Please enter a valid email address.');
      return;
    }

    // Save to recent accounts list
    const username = clean.split('@')[0];
    const namePart = username
      .split('.')
      .map((w) => w.charAt(0).toUpperCase() + w.slice(1))
      .join(' ');

    const newAccount: SavedAccount = {
      email: clean,
      name: namePart,
      avatarChar: clean.charAt(0).toUpperCase(),
    };

    try {
      const updated = [newAccount, ...savedAccounts.filter((a) => a.email !== clean)].slice(0, 4);
      localStorage.setItem(STORAGE_KEY, JSON.stringify(updated));
      setSavedAccounts(updated);
    } catch {
      // Ignore
    }

    onSelectAccount(clean);
  };

  const handleSelectSaved = (email: string) => {
    onSelectAccount(email);
  };

  const handleRemoveAccount = (e: React.MouseEvent, email: string) => {
    e.stopPropagation();
    const updated = savedAccounts.filter((a) => a.email !== email);
    setSavedAccounts(updated);
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(updated));
    } catch {
      // Ignore
    }
    if (updated.length === 0) {
      setShowInputMode(true);
    }
  };

  return (
    <div
      className="modal-backdrop google-dialog-backdrop"
      onClick={(e) => {
        if (e.target === e.currentTarget && !loading) {
          onClose();
        }
      }}
    >
      <div className="google-auth-card" role="dialog" aria-modal="true" aria-labelledby="google-auth-title">
        {/* Google Brand Header */}
        <div className="google-card-header">
          <svg className="google-brand-logo" width="40" height="40" viewBox="0 0 24 24" aria-hidden="true">
            <path fill="#4285F4" d="M23.745 12.27c0-.7-.06-1.4-.19-2.07H12v4.51h6.6c-.29 1.52-1.14 2.8-2.4 3.66v3.05h3.87c2.26-2.09 3.67-5.17 3.67-9.15z" />
            <path fill="#34A853" d="M12 24c3.24 0 5.95-1.08 7.93-2.91l-3.87-3.05c-1.08.72-2.45 1.16-4.06 1.16-3.13 0-5.78-2.11-6.73-4.96H1.28v3.15C3.26 21.36 7.34 24 12 24z" />
            <path fill="#FBBC05" d="M5.27 14.24c-.25-.72-.38-1.49-.38-2.24s.13-1.52.38-2.24V6.61H1.28C.46 8.23 0 10.06 0 12s.46 3.77 1.28 5.39l3.99-3.15z" />
            <path fill="#EA4335" d="M12 4.75c1.77 0 3.35.61 4.6 1.8l3.42-3.42C17.95 1.19 15.24 0 12 0 7.34 0 3.26 2.64 1.28 6.61l3.99 3.15c.95-2.85 3.6-4.96 6.73-4.96z" />
          </svg>
          <h2 id="google-auth-title" className="google-card-title">Sign in with Google</h2>
          <p className="google-card-subtitle">
            Choose an account or enter your Gmail address to continue to <strong>DocuPilot</strong>
          </p>
        </div>

        {/* Error notification */}
        {(error || inputError) && (
          <div className="google-error-banner" role="alert">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
              <circle cx="12" cy="12" r="10" />
              <line x1="12" y1="8" x2="12" y2="12" />
              <line x1="12" y1="16" x2="12.01" y2="16" />
            </svg>
            <span>{error || inputError}</span>
          </div>
        )}

        {/* Primary OAuth Popup Action */}
        {onLaunchPopup && (
          <>
            <button
              type="button"
              className="google-launch-popup-btn"
              onClick={onLaunchPopup}
              disabled={loading}
            >
              <svg width="18" height="18" viewBox="0 0 24 24" aria-hidden="true">
                <path fill="#4285F4" d="M23.745 12.27c0-.7-.06-1.4-.19-2.07H12v4.51h6.6c-.29 1.52-1.14 2.8-2.4 3.66v3.05h3.87c2.26-2.09 3.67-5.17 3.67-9.15z" />
                <path fill="#34A853" d="M12 24c3.24 0 5.95-1.08 7.93-2.91l-3.87-3.05c-1.08.72-2.45 1.16-4.06 1.16-3.13 0-5.78-2.11-6.73-4.96H1.28v3.15C3.26 21.36 7.34 24 12 24z" />
                <path fill="#FBBC05" d="M5.27 14.24c-.25-.72-.38-1.49-.38-2.24s.13-1.52.38-2.24V6.61H1.28C.46 8.23 0 10.06 0 12s.46 3.77 1.28 5.39l3.99-3.15z" />
                <path fill="#EA4335" d="M12 4.75c1.77 0 3.35.61 4.6 1.8l3.42-3.42C17.95 1.19 15.24 0 12 0 7.34 0 3.26 2.64 1.28 6.61l3.99 3.15c.95-2.85 3.6-4.96 6.73-4.96z" />
              </svg>
              <span>Sign in with Google OAuth Popup</span>
            </button>
            <div className="google-divider">
              <span>or choose account / enter gmail</span>
            </div>
          </>
        )}

        {/* Account Chooser List */}
        {!showInputMode && savedAccounts.length > 0 && (
          <div className="google-accounts-list">
            <span className="google-list-label">Choose an account:</span>
            {savedAccounts.map((account) => (
              <div
                key={account.email}
                className="google-account-item"
                onClick={() => !loading && handleSelectSaved(account.email)}
                role="button"
                tabIndex={0}
              >
                <div className="google-avatar-circle">
                  {account.avatarChar}
                </div>
                <div className="google-account-info">
                  <span className="google-account-name">{account.name}</span>
                  <span className="google-account-email">{account.email}</span>
                </div>
                <button
                  type="button"
                  className="google-account-remove"
                  onClick={(e) => handleRemoveAccount(e, account.email)}
                  title="Remove account from list"
                  aria-label={`Remove ${account.email}`}
                >
                  ✕
                </button>
              </div>
            ))}

            <button
              type="button"
              className="google-use-another-btn"
              onClick={() => {
                setShowInputMode(true);
                setInputError(null);
              }}
            >
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <circle cx="12" cy="12" r="10" />
                <line x1="12" y1="8" x2="12" y2="16" />
                <line x1="8" y1="12" x2="16" y2="12" />
              </svg>
              <span>Use another account</span>
            </button>
          </div>
        )}

        {/* Enter Gmail Input Mode */}
        {showInputMode && (
          <form onSubmit={handleEmailSubmit} className="google-email-form">
            <div className="google-input-wrapper">
              <label htmlFor="google-email-input" className="google-input-label">
                Email or phone
              </label>
              <input
                id="google-email-input"
                type="email"
                className="google-native-input"
                placeholder="name@gmail.com"
                value={emailInput}
                onChange={(e) => setEmailInput(e.target.value)}
                autoFocus
                disabled={loading}
                autoComplete="email"
              />
            </div>

            {savedAccounts.length > 0 && (
              <button
                type="button"
                className="google-back-to-list-btn"
                onClick={() => {
                  setShowInputMode(false);
                  setInputError(null);
                }}
              >
                ← Back to saved accounts
              </button>
            )}

            <div className="google-action-buttons">
              <button
                type="button"
                className="google-btn-cancel"
                onClick={onClose}
                disabled={loading}
              >
                Cancel
              </button>
              <button
                type="submit"
                className="google-btn-next"
                disabled={loading}
              >
                {loading ? (
                  <span className="btn-loading-content">
                    <span className="spinner" />
                    Signing in...
                  </span>
                ) : (
                  'Next'
                )}
              </button>
            </div>
          </form>
        )}

        {/* Google Terms Footer */}
        <div className="google-card-footer">
          <p>
            To continue, Google will share your name, email address, and profile with <strong>DocuPilot</strong>.
          </p>
        </div>
      </div>
    </div>
  );
};
