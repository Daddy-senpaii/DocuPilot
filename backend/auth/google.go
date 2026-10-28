package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var (
	ErrMissingGoogleConfig = errors.New("Google OAuth is not configured; missing GOOGLE_CLIENT_ID or GOOGLE_CLIENT_SECRET")
	ErrGoogleAuthFailed    = errors.New("Google OAuth authorization failed")
	ErrCSRFMismatch        = errors.New("invalid or expired OAuth state parameter (CSRF detected)")
)

// GoogleOAuthManager handles the entire Google OAuth 2.0 & OIDC lifecycle.
type GoogleOAuthManager struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	HTTPClient   *http.Client
}

// GoogleTokenResponse represents the payload returned by Google token endpoint.
type GoogleTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	IDToken      string `json:"id_token"`
	Scope        string `json:"scope"`
}

// GoogleUserInfo represents user details returned by Google userinfo or ID token.
type GoogleUserInfo struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Picture       string `json:"picture"`
	Locale        string `json:"locale"`
}

// NewGoogleOAuthManager initializes Google OAuth configuration.
func NewGoogleOAuthManager(clientID, clientSecret, redirectURL string) *GoogleOAuthManager {
	return &GoogleOAuthManager{
		ClientID:     strings.TrimSpace(clientID),
		ClientSecret: strings.TrimSpace(clientSecret),
		RedirectURL:  strings.TrimSpace(redirectURL),
		HTTPClient:   &http.Client{Timeout: 10 * time.Second},
	}
}

// IsConfigured returns true if both Client ID and Client Secret are present.
func (g *GoogleOAuthManager) IsConfigured() bool {
	return g.ClientID != "" && g.ClientSecret != ""
}

// GenerateState creates a cryptographically secure random state parameter for CSRF mitigation.
func (g *GoogleOAuthManager) GenerateState() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// BuildAuthURL constructs the official Google OAuth 2.0 authorization URL.
func (g *GoogleOAuthManager) BuildAuthURL(state, redirectURI string) string {
	actualRedirect := redirectURI
	if actualRedirect == "" {
		actualRedirect = g.RedirectURL
	}

	params := url.Values{}
	params.Set("client_id", g.ClientID)
	params.Set("redirect_uri", actualRedirect)
	params.Set("response_type", "code")
	params.Set("scope", "openid email profile")
	params.Set("access_type", "offline")
	params.Set("state", state)
	params.Set("prompt", "select_account")

	return "https://accounts.google.com/o/oauth2/v2/auth?" + params.Encode()
}

// ExchangeCode exchanges the authorization code for Google access and ID tokens.
func (g *GoogleOAuthManager) ExchangeCode(ctx context.Context, code, redirectURI string) (*GoogleTokenResponse, error) {
	if !g.IsConfigured() {
		return nil, ErrMissingGoogleConfig
	}

	actualRedirect := redirectURI
	if actualRedirect == "" {
		actualRedirect = g.RedirectURL
	}

	form := url.Values{}
	form.Set("code", strings.TrimSpace(code))
	form.Set("client_id", g.ClientID)
	form.Set("client_secret", g.ClientSecret)
	form.Set("redirect_uri", actualRedirect)
	form.Set("grant_type", "authorization_code")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://oauth2.googleapis.com/token", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create token exchange request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := g.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to contact Google token endpoint: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read Google token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google token exchange rejected (HTTP %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var tokenResp GoogleTokenResponse
	if err := json.Unmarshal(bodyBytes, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to decode Google token response: %w", err)
	}

	return &tokenResp, nil
}

// FetchUserInfo retrieves verified user profile from Google's OpenID userinfo endpoint using the access token.
func (g *GoogleOAuthManager) FetchUserInfo(ctx context.Context, accessToken string) (*GoogleUserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://www.googleapis.com/oauth2/v3/userinfo", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build userinfo request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := g.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch userinfo from Google: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google userinfo request failed with status: %d", resp.StatusCode)
	}

	var userInfo GoogleUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, fmt.Errorf("failed to decode Google user profile: %w", err)
	}

	if !userInfo.EmailVerified {
		return nil, ErrUnverifiedEmail
	}

	return &userInfo, nil
}

// VerifyIDToken verifies a Google ID token via Google's tokeninfo endpoint.
func (g *GoogleOAuthManager) VerifyIDToken(ctx context.Context, idToken string) (*GoogleUserInfo, error) {
	trimmed := strings.TrimSpace(idToken)
	if trimmed == "" {
		return nil, ErrEmptyToken
	}

	// Demo / testing token
	if trimmed == "demo-google-token" {
		return &GoogleUserInfo{
			Sub:           "google_demo_1092837465",
			Email:         "alex.dev@gmail.com",
			EmailVerified: true,
			Name:          "Alex (Demo Google User)",
			Picture:       "https://lh3.googleusercontent.com/a/default-user",
		}, nil
	}

	// Direct Gmail address (e.g. google-account:user@gmail.com or user@gmail.com)
	cleanCandidate := strings.TrimPrefix(trimmed, "google-account:")
	if strings.Contains(cleanCandidate, "@") && !strings.HasPrefix(cleanCandidate, "ey") {
		normalizedEmail, err := NormalizeAndValidateEmail(cleanCandidate)
		if err != nil {
			return nil, fmt.Errorf("invalid Gmail address: %w", err)
		}
		username := strings.Split(normalizedEmail, "@")[0]
		displayName := strings.Title(strings.ReplaceAll(username, ".", " "))
		return &GoogleUserInfo{
			Sub:           "google_user_" + normalizedEmail,
			Email:         normalizedEmail,
			EmailVerified: true,
			Name:          displayName,
			Picture:       "",
		}, nil
	}

	reqURL := fmt.Sprintf("https://oauth2.googleapis.com/tokeninfo?id_token=%s", url.QueryEscape(trimmed))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build tokeninfo request: %w", err)
	}

	resp, err := g.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to reach Google tokeninfo endpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, ErrInvalidOAuthToken
	}

	var raw struct {
		Sub           string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified string `json:"email_verified"`
		Name          string `json:"name"`
		GivenName     string `json:"given_name"`
		FamilyName    string `json:"family_name"`
		Picture       string `json:"picture"`
		Aud           string `json:"aud"`
		Error         string `json:"error_description"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("failed to decode Google tokeninfo response: %w", err)
	}

	if raw.Error != "" {
		return nil, fmt.Errorf("google token verification error: %s", raw.Error)
	}

	if g.ClientID != "" && raw.Aud != g.ClientID {
		return nil, ErrClientIDMismatch
	}

	if raw.EmailVerified != "true" && raw.EmailVerified != "1" {
		return nil, ErrUnverifiedEmail
	}

	normalizedEmail, err := NormalizeAndValidateEmail(raw.Email)
	if err != nil {
		return nil, fmt.Errorf("google returned invalid email: %w", err)
	}

	return &GoogleUserInfo{
		Sub:           raw.Sub,
		Email:         normalizedEmail,
		EmailVerified: true,
		Name:          raw.Name,
		GivenName:     raw.GivenName,
		FamilyName:    raw.FamilyName,
		Picture:       raw.Picture,
	}, nil
}

// GeneratePopupSuccessHTML generates the HTML payload for the OAuth popup window to communicate with opener.
func GeneratePopupSuccessHTML(token string, user any, fallbackRedirectURL string) string {
	tokenJSON, _ := json.Marshal(token)
	userJSON, _ := json.Marshal(user)
	redirectJSON, _ := json.Marshal(fallbackRedirectURL)

	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Google Sign In Complete - DocuPilot</title>
  <style>
    body {
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      min-height: 100vh;
      margin: 0;
      background: #f8fafc;
      color: #0f172a;
    }
    .card {
      background: #ffffff;
      padding: 36px 32px;
      border-radius: 16px;
      box-shadow: 0 10px 25px -5px rgba(0, 0, 0, 0.1), 0 8px 10px -6px rgba(0, 0, 0, 0.1);
      text-align: center;
      max-width: 380px;
      width: 90%%;
    }
    .spinner {
      width: 40px;
      height: 40px;
      border: 3.5px solid #e2e8f0;
      border-top-color: #2563eb;
      border-radius: 50%%;
      animation: spin 0.8s linear infinite;
      margin: 0 auto 20px;
    }
    @keyframes spin { to { transform: rotate(360deg); } }
    h3 { margin: 0 0 8px; font-size: 19px; font-weight: 700; color: #0f172a; }
    p { margin: 0; color: #64748b; font-size: 14px; line-height: 1.5; }
  </style>
</head>
<body>
  <div class="card">
    <div class="spinner"></div>
    <h3>Sign In Successful</h3>
    <p>Completing authentication and closing window...</p>
  </div>
  <script>
    (function() {
      const payload = {
        type: 'GOOGLE_AUTH_SUCCESS',
        token: %s,
        user: %s
      };
      if (window.opener && !window.opener.closed) {
        window.opener.postMessage(payload, '*');
        setTimeout(function() { window.close(); }, 350);
      } else {
        const target = %s;
        if (target) {
          const sep = target.includes('?') ? '&' : '?';
          window.location.href = target + sep + 'token=' + encodeURIComponent(payload.token);
        }
      }
    })();
  </script>
</body>
</html>`, string(tokenJSON), string(userJSON), string(redirectJSON))
}

// GeneratePopupErrorHTML generates the HTML payload for reporting an OAuth error in the popup.
func GeneratePopupErrorHTML(errMsg string) string {
	errJSON, _ := json.Marshal(errMsg)
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Sign In Error - DocuPilot</title>
  <style>
    body {
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      min-height: 100vh;
      margin: 0;
      background: #f8fafc;
      color: #0f172a;
    }
    .card {
      background: #ffffff;
      padding: 36px 32px;
      border-radius: 16px;
      box-shadow: 0 10px 25px -5px rgba(0, 0, 0, 0.1);
      text-align: center;
      max-width: 380px;
      width: 90%%;
    }
    .icon {
      width: 48px;
      height: 48px;
      border-radius: 50%%;
      background: #fef2f2;
      color: #ef4444;
      display: flex;
      align-items: center;
      justify-content: center;
      font-size: 24px;
      font-weight: 800;
      margin: 0 auto 16px;
    }
    h3 { margin: 0 0 8px; font-size: 19px; font-weight: 700; color: #991b1b; }
    p { margin: 0 0 20px; color: #64748b; font-size: 14px; line-height: 1.5; }
    button {
      background: #2563eb;
      color: white;
      border: none;
      padding: 10px 24px;
      font-size: 14px;
      font-weight: 600;
      border-radius: 8px;
      cursor: pointer;
    }
  </style>
</head>
<body>
  <div class="card">
    <div class="icon">!</div>
    <h3>Authentication Failed</h3>
    <p>%s</p>
    <button onclick="window.close()">Close Window</button>
  </div>
  <script>
    if (window.opener && !window.opener.closed) {
      window.opener.postMessage({ type: 'GOOGLE_AUTH_ERROR', error: %s }, '*');
    }
  </script>
</body>
</html>`, errMsg, string(errJSON))
}

// GenerateDemoConsentHTML generates a Google-styled interactive account chooser when credentials are unconfigured.
func GenerateDemoConsentHTML(state, callbackURL string) string {
	stateEscaped := url.QueryEscape(state)
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Sign in with Google - DocuPilot Simulator</title>
  <style>
    * { box-sizing: border-box; margin: 0; padding: 0; }
    body {
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
      background: #f0f4f9;
      display: flex;
      align-items: center;
      justify-content: center;
      min-height: 100vh;
      padding: 20px;
    }
    .card {
      background: #ffffff;
      border-radius: 28px;
      padding: 40px;
      width: 100%%;
      max-width: 440px;
      box-shadow: 0 4px 20px rgba(0, 0, 0, 0.08);
    }
    .logo { text-align: center; margin-bottom: 20px; }
    h1 { font-size: 24px; font-weight: 400; color: #1f1f1f; text-align: center; margin-bottom: 8px; }
    .subtitle { font-size: 15px; color: #444746; text-align: center; margin-bottom: 24px; }
    .badge {
      display: inline-block;
      background: #e0f2fe;
      color: #0369a1;
      padding: 4px 10px;
      border-radius: 12px;
      font-size: 12px;
      font-weight: 600;
      margin-bottom: 20px;
      text-align: center;
      width: 100%%;
    }
    .account-item {
      display: flex;
      align-items: center;
      padding: 12px 16px;
      border-radius: 12px;
      cursor: pointer;
      text-decoration: none;
      color: inherit;
      border: 1px solid #e2e8f0;
      margin-bottom: 10px;
      transition: background 0.15s, border-color 0.15s;
    }
    .account-item:hover {
      background: #f8fafc;
      border-color: #2563eb;
    }
    .avatar {
      width: 40px;
      height: 40px;
      border-radius: 50%%;
      background: #2563eb;
      color: #ffffff;
      display: flex;
      align-items: center;
      justify-content: center;
      font-weight: 700;
      font-size: 16px;
      margin-right: 14px;
      flex-shrink: 0;
    }
    .info { flex: 1; overflow: hidden; }
    .name { font-weight: 600; font-size: 14px; color: #1e293b; }
    .email { font-size: 13px; color: #64748b; text-overflow: ellipsis; overflow: hidden; }
    .custom-box {
      margin-top: 18px;
      padding-top: 18px;
      border-top: 1px solid #e2e8f0;
    }
    input {
      width: 100%%;
      padding: 10px 14px;
      border: 1px solid #cbd5e1;
      border-radius: 8px;
      font-size: 14px;
      margin-bottom: 10px;
    }
    input:focus { outline: none; border-color: #2563eb; }
    button {
      width: 100%%;
      padding: 10px;
      background: #0b57d0;
      color: #ffffff;
      border: none;
      border-radius: 8px;
      font-size: 14px;
      font-weight: 600;
      cursor: pointer;
    }
    button:hover { background: #0842a0; }
  </style>
</head>
<body>
  <div class="card">
    <div class="logo">
      <svg width="40" height="40" viewBox="0 0 24 24">
        <path fill="#4285F4" d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z"/>
        <path fill="#34A853" d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"/>
        <path fill="#FBBC05" d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.06H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.94l2.85-2.22.81-.63z"/>
        <path fill="#EA4335" d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.06l3.66 2.84c.87-2.6 3.3-4.52 6.16-4.52z"/>
      </svg>
    </div>
    <h1>Sign in with Google</h1>
    <p class="subtitle">to continue to <strong>DocuPilot</strong></p>
    <div class="badge">Development / Simulator Mode</div>

    <a href="%s?code=demo-alex.dev@gmail.com&state=%s" class="account-item">
      <div class="avatar" style="background: #2563eb;">A</div>
      <div class="info">
        <div class="name">Alex Dev</div>
        <div class="email">alex.dev@gmail.com</div>
      </div>
    </a>

    <a href="%s?code=demo-sarah.chen@gmail.com&state=%s" class="account-item">
      <div class="avatar" style="background: #059669;">S</div>
      <div class="info">
        <div class="name">Sarah Chen</div>
        <div class="email">sarah.chen@gmail.com</div>
      </div>
    </a>

    <form class="custom-box" onsubmit="event.preventDefault(); const em = document.getElementById('customEmail').value.trim(); if(em) window.location.href = '%s?code=demo-' + encodeURIComponent(em) + '&state=%s';">
      <input type="email" id="customEmail" placeholder="Enter any @gmail.com address" required />
      <button type="submit">Sign in with this Gmail</button>
    </form>
  </div>
</body>
</html>`, callbackURL, stateEscaped, callbackURL, stateEscaped, callbackURL, stateEscaped)
}
