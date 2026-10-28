import React, { useState, useEffect } from 'react';
import { User, API_BASE_URL } from '../types';
import { GoogleAuthDialog } from './GoogleAuthDialog';

interface AuthModalProps {
  isOpen: boolean;
  onClose: () => void;
  onAuthSuccess: (token: string, user: User, message: string) => void;
}

interface ProvidersResponse {
  google?: { enabled: boolean; client_id: string };
  github?: { enabled: boolean; client_id: string };
  demo_mode?: boolean;
}

export const AuthModal: React.FC<AuthModalProps> = ({
  isOpen,
  onClose,
  onAuthSuccess,
}) => {
  const [authMode, setAuthMode] = useState<'login' | 'register'>('login');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [showPassword, setShowPassword] = useState(false);
  const [providers, setProviders] = useState<ProvidersResponse | null>(null);
  const [oauthLoading, setOauthLoading] = useState<'google' | 'github' | null>(null);
  const [showGoogleDialog, setShowGoogleDialog] = useState(false);
  const [googleDialogError, setGoogleDialogError] = useState<string | null>(null);

  useEffect(() => {
    if (!isOpen) return;

    fetch(`${API_BASE_URL}/api/auth/providers`)
      .then((res) => (res.ok ? res.json() : null))
      .then((data: ProvidersResponse | null) => {
        if (data) {
          setProviders(data);
          if (data.google?.client_id && !document.getElementById('google-gsi-client')) {
            const script = document.createElement('script');
            script.id = 'google-gsi-client';
            script.src = 'https://accounts.google.com/gsi/client';
            script.async = true;
            script.defer = true;
            document.body.appendChild(script);
          }
        }
      })
      .catch(() => {});
  }, [isOpen]);

  const handleOAuthSubmit = async (provider: 'google' | 'github', tokenOrCredential: string) => {
    setOauthLoading(provider);
    setError(null);
    try {
      const endpoint = `${API_BASE_URL}/api/auth/oauth/${provider}`;
      const payload = provider === 'google'
        ? { credential: tokenOrCredential }
        : { token: tokenOrCredential };

      const res = await fetch(endpoint, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      });

      const data = await res.json();
      if (!res.ok) {
        throw new Error(data.error || `${provider} authentication failed`);
      }

      const providerLabel = provider === 'google' ? 'Google' : 'GitHub';
      onAuthSuccess(data.token, data.user, `Signed in successfully with ${providerLabel}!`);
      onClose();
    } catch (err: unknown) {
      if (err instanceof Error) {
        setError(err.message);
      } else {
        setError(`An unexpected error occurred during ${provider} sign in.`);
      }
    } finally {
      setOauthLoading(null);
    }
  };

  const handleGooglePopupSignIn = () => {
    setError(null);
    setGoogleDialogError(null);
    setOauthLoading('google');

    const width = 500;
    const height = 620;
    const left = window.screenX + (window.outerWidth - width) / 2;
    const top = window.screenY + (window.outerHeight - height) / 2;

    const popup = window.open(
      `${API_BASE_URL}/api/auth/google/login`,
      'docupilot_google_oauth',
      `width=${width},height=${height},left=${left},top=${top},status=no,menubar=no,toolbar=no`
    );

    if (!popup || popup.closed || typeof popup.closed === 'undefined') {
      // Browser popup blocker triggered - fall back to account dialog
      setOauthLoading(null);
      setShowGoogleDialog(true);
      return;
    }

    const messageHandler = (event: MessageEvent) => {
      if (event.data?.type === 'GOOGLE_AUTH_SUCCESS') {
        window.removeEventListener('message', messageHandler);
        clearInterval(popupWatcher);
        const { token, user } = event.data;
        onAuthSuccess(token, user, `Signed in successfully with Google (${user.email || 'account'})!`);
        setShowGoogleDialog(false);
        onClose();
        setOauthLoading(null);
      } else if (event.data?.type === 'GOOGLE_AUTH_ERROR') {
        window.removeEventListener('message', messageHandler);
        clearInterval(popupWatcher);
        setError(event.data.error || 'Google authentication failed');
        setOauthLoading(null);
      }
    };

    window.addEventListener('message', messageHandler);

    const popupWatcher = setInterval(() => {
      if (popup.closed) {
        clearInterval(popupWatcher);
        window.removeEventListener('message', messageHandler);
        setOauthLoading(null);
      }
    }, 800);
  };

  const handleGoogleSignIn = () => {
    setError(null);
    setGoogleDialogError(null);

    const googleClientId = providers?.google?.client_id;
    if (googleClientId && typeof window !== 'undefined' && window.google?.accounts?.id) {
      window.google.accounts.id.initialize({
        client_id: googleClientId,
        callback: async (resp: { credential: string }) => {
          if (resp.credential) {
            await handleOAuthSubmit('google', resp.credential);
          }
        },
      });
      window.google.accounts.id.prompt((notification) => {
        if (notification.isNotDisplayed() || (notification.isSkippedMoment && notification.isSkippedMoment())) {
          handleGooglePopupSignIn();
        }
      });
      return;
    }

    handleGooglePopupSignIn();
  };

  const handleGoogleAccountSelected = async (email: string) => {
    setOauthLoading('google');
    setGoogleDialogError(null);
    try {
      const endpoint = `${API_BASE_URL}/api/auth/oauth/google`;
      const res = await fetch(endpoint, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ credential: email }),
      });

      const data = await res.json();
      if (!res.ok) {
        throw new Error(data.error || 'Google authentication failed');
      }

      setShowGoogleDialog(false);
      onAuthSuccess(data.token, data.user, `Signed in successfully with Google (${email})!`);
      onClose();
    } catch (err: unknown) {
      if (err instanceof Error) {
        setGoogleDialogError(err.message);
      } else {
        setGoogleDialogError('An unexpected error occurred during Google sign in.');
      }
    } finally {
      setOauthLoading(null);
    }
  };

  const handleGitHubSignIn = async () => {
    setError(null);
    const githubClientId = providers?.github?.client_id;

    if (githubClientId) {
      const redirectUri = encodeURIComponent(window.location.origin);
      window.location.href = `https://github.com/login/oauth/authorize?client_id=${githubClientId}&scope=user:email&redirect_uri=${redirectUri}`;
      return;
    }

    // Demo fallback / fast testing
    await handleOAuthSubmit('github', 'demo-github-token');
  };

  if (!isOpen) return null;

  const isMinLength = password.length >= 8 && password.length <= 72;
  const hasLetterAndNum = /[a-zA-Z]/.test(password) && /[^a-zA-Z]/.test(password);
  const passwordsMatch = password.length > 0 && password === confirmPassword;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    const cleanEmail = email.trim();
    if (!cleanEmail || !cleanEmail.includes('@')) {
      setError('Please enter a valid email address.');
      return;
    }

    if (password.length < 8) {
      setError('Password must be at least 8 characters long.');
      return;
    }

    if (authMode === 'register' && password !== confirmPassword) {
      setError('Passwords do not match.');
      return;
    }

    setLoading(true);
    try {
      const res = await fetch(`${API_BASE_URL}/api/auth/${authMode}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: cleanEmail, password }),
      });

      const data = await res.json();
      if (!res.ok) {
        throw new Error(data.error || 'Authentication request failed');
      }

      onAuthSuccess(
        data.token,
        data.user,
        authMode === 'register' ? 'Account created successfully! Welcome to DocuPilot.' : 'Signed in successfully.'
      );
      setPassword('');
      setConfirmPassword('');
      onClose();
    } catch (err: unknown) {
      if (err instanceof Error) {
        setError(err.message);
      } else {
        setError('An unexpected authentication error occurred.');
      }
    } finally {
      setLoading(false);
    }
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
            onClick={onClose}
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
              setError(null);
            }}
          >
            Sign In
          </button>
          <button
            type="button"
            className={`auth-segment-btn ${authMode === 'register' ? 'active' : ''}`}
            onClick={() => {
              setAuthMode('register');
              setError(null);
            }}
          >
            Create Account
          </button>
        </div>

        {/* Social OAuth Sign In Options */}
        <div className="oauth-btn-group">
          <button
            type="button"
            className="oauth-btn oauth-google"
            onClick={handleGoogleSignIn}
            disabled={loading || oauthLoading !== null}
          >
            {oauthLoading === 'google' ? (
              <span className="btn-loading-content">
                <span className="spinner" />
                Connecting to Google...
              </span>
            ) : (
              <>
                <svg width="18" height="18" viewBox="0 0 24 24" aria-hidden="true">
                  <path fill="#4285F4" d="M23.745 12.27c0-.7-.06-1.4-.19-2.07H12v4.51h6.6c-.29 1.52-1.14 2.8-2.4 3.66v3.05h3.87c2.26-2.09 3.67-5.17 3.67-9.15z" />
                  <path fill="#34A853" d="M12 24c3.24 0 5.95-1.08 7.93-2.91l-3.87-3.05c-1.08.72-2.45 1.16-4.06 1.16-3.13 0-5.78-2.11-6.73-4.96H1.28v3.15C3.26 21.36 7.34 24 12 24z" />
                  <path fill="#FBBC05" d="M5.27 14.24c-.25-.72-.38-1.49-.38-2.24s.13-1.52.38-2.24V6.61H1.28C.46 8.23 0 10.06 0 12s.46 3.77 1.28 5.39l3.99-3.15z" />
                  <path fill="#EA4335" d="M12 4.75c1.77 0 3.35.61 4.6 1.8l3.42-3.42C17.95 1.19 15.24 0 12 0 7.34 0 3.26 2.64 1.28 6.61l3.99 3.15c.95-2.85 3.6-4.96 6.73-4.96z" />
                </svg>
                <span>Continue with Google</span>
                {providers?.demo_mode && <span className="oauth-demo-badge">Demo</span>}
              </>
            )}
          </button>

          <button
            type="button"
            className="oauth-btn oauth-github"
            onClick={handleGitHubSignIn}
            disabled={loading || oauthLoading !== null}
          >
            {oauthLoading === 'github' ? (
              <span className="btn-loading-content">
                <span className="spinner" />
                Connecting to GitHub...
              </span>
            ) : (
              <>
                <svg width="18" height="18" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
                  <path fillRule="evenodd" clipRule="evenodd" d="M12 2C6.477 2 2 6.484 2 12.017c0 4.425 2.865 8.18 6.839 9.504.5.092.682-.217.682-.483 0-.237-.008-.868-.013-1.703-2.782.605-3.369-1.343-3.369-1.343-.454-1.158-1.11-1.466-1.11-1.466-.908-.62.069-.608.069-.608 1.003.07 1.53 1.032 1.53 1.032.892 1.53 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.113-4.555-4.951 0-1.093.39-1.988 1.029-2.688-.103-.253-.446-1.272.098-2.65 0 0 .84-.27 2.75 1.026A9.564 9.564 0 0 1 12 6.844c.85.004 1.705.115 2.504.337 1.909-1.296 2.747-1.027 2.747-1.027.546 1.379.202 2.398.1 2.651.64.7 1.028 1.595 1.028 2.688 0 3.848-2.339 4.695-4.566 4.943.359.309.678.92.678 1.855 0 1.338-.012 2.419-.012 2.747 0 .268.18.58.688.482A10.019 10.019 0 0 0 22 12.017C22 6.484 17.522 2 12 2z" />
                </svg>
                <span>Continue with GitHub</span>
                {providers?.demo_mode && <span className="oauth-demo-badge">Demo</span>}
              </>
            )}
          </button>
        </div>

        <button
          type="button"
          className="auth-gmail-direct-btn"
          onClick={() => {
            setError(null);
            setGoogleDialogError(null);
            setShowGoogleDialog(true);
          }}
        >
          Or choose account / enter Gmail ID directly
        </button>

        <div className="oauth-divider">
          <span>or continue with email</span>
        </div>

        {error && (
          <div className="alert-error" role="alert">
            <span className="alert-icon">⚠️</span>
            <span>{error}</span>
          </div>
        )}

        <form onSubmit={handleSubmit} className="auth-form">
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
                value={email}
                onChange={(e) => setEmail(e.target.value)}
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
                value={password}
                onChange={(e) => setPassword(e.target.value)}
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
                    value={confirmPassword}
                    onChange={(e) => setConfirmPassword(e.target.value)}
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
                {confirmPassword && (
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
              onClick={onClose}
              disabled={loading}
            >
              Cancel
            </button>
            <button type="submit" className="auth-submit-btn" disabled={loading}>
              {loading ? (
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
                  setError(null);
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
                  setError(null);
                }}
              >
                Sign in
              </button>
            </span>
          )}
        </div>
      </div>

      <GoogleAuthDialog
        isOpen={showGoogleDialog}
        onClose={() => setShowGoogleDialog(false)}
        onSelectAccount={handleGoogleAccountSelected}
        onLaunchPopup={handleGooglePopupSignIn}
        loading={oauthLoading === 'google'}
        error={googleDialogError}
      />
    </div>
  );
};
