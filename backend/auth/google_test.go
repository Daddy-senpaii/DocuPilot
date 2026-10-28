package auth

import (
	"context"
	"net/url"
	"strings"
	"testing"
)

func TestGoogleOAuthIsConfigured(t *testing.T) {
	unconfigured := NewGoogleOAuthManager("", "", "")
	if unconfigured.IsConfigured() {
		t.Error("expected IsConfigured() to be false when credentials are empty")
	}

	partial := NewGoogleOAuthManager("client-id-123", "", "")
	if partial.IsConfigured() {
		t.Error("expected IsConfigured() to be false when secret is missing")
	}

	configured := NewGoogleOAuthManager("client-id-123", "secret-456", "https://app.com/callback")
	if !configured.IsConfigured() {
		t.Error("expected IsConfigured() to be true when both client_id and secret are provided")
	}
}

func TestGoogleOAuthGenerateState(t *testing.T) {
	manager := NewGoogleOAuthManager("client", "secret", "https://app.com/callback")
	state1, err := manager.GenerateState()
	if err != nil {
		t.Fatalf("failed to generate state: %v", err)
	}
	if len(state1) != 64 { // 32 bytes hex encoded = 64 characters
		t.Errorf("expected 64 character state hex string, got %d", len(state1))
	}

	state2, _ := manager.GenerateState()
	if state1 == state2 {
		t.Error("expected state generation to be cryptographically random and unique")
	}
}

func TestGoogleOAuthBuildAuthURL(t *testing.T) {
	manager := NewGoogleOAuthManager("my-google-client-id", "my-secret", "https://docupilot.com/api/auth/google/callback")
	authURL := manager.BuildAuthURL("csrf-token-xyz", "")

	if !strings.HasPrefix(authURL, "https://accounts.google.com/o/oauth2/v2/auth?") {
		t.Errorf("unexpected base URL: %s", authURL)
	}

	parsed, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("failed to parse generated auth URL: %v", err)
	}

	q := parsed.Query()
	if q.Get("client_id") != "my-google-client-id" {
		t.Errorf("expected client_id 'my-google-client-id', got %q", q.Get("client_id"))
	}
	if q.Get("redirect_uri") != "https://docupilot.com/api/auth/google/callback" {
		t.Errorf("expected redirect_uri, got %q", q.Get("redirect_uri"))
	}
	if q.Get("response_type") != "code" {
		t.Errorf("expected response_type 'code', got %q", q.Get("response_type"))
	}
	if q.Get("state") != "csrf-token-xyz" {
		t.Errorf("expected state 'csrf-token-xyz', got %q", q.Get("state"))
	}
	if !strings.Contains(q.Get("scope"), "openid") || !strings.Contains(q.Get("scope"), "email") {
		t.Errorf("expected OIDC scopes, got %q", q.Get("scope"))
	}
}

func TestGoogleOAuthExchangeCodeMissingConfig(t *testing.T) {
	manager := NewGoogleOAuthManager("", "", "")
	_, err := manager.ExchangeCode(context.Background(), "some-code", "")
	if err != ErrMissingGoogleConfig {
		t.Fatalf("expected ErrMissingGoogleConfig, got: %v", err)
	}
}

func TestGoogleOAuthVerifyIDToken(t *testing.T) {
	manager := NewGoogleOAuthManager("test-client-id", "test-secret", "https://app.com/callback")
	ctx := context.Background()

	// Empty token
	if _, err := manager.VerifyIDToken(ctx, "   "); err != ErrEmptyToken {
		t.Errorf("expected ErrEmptyToken, got: %v", err)
	}

	// Demo token
	info, err := manager.VerifyIDToken(ctx, "demo-google-token")
	if err != nil {
		t.Fatalf("failed to verify demo token: %v", err)
	}
	if info.Email != "alex.dev@gmail.com" || !info.EmailVerified {
		t.Errorf("unexpected demo token profile: %+v", info)
	}

	// Direct Gmail address
	directInfo, err := manager.VerifyIDToken(ctx, "google-account:sam.developer@gmail.com")
	if err != nil {
		t.Fatalf("failed to verify direct gmail: %v", err)
	}
	if directInfo.Email != "sam.developer@gmail.com" || directInfo.Name != "Sam Developer" {
		t.Errorf("unexpected direct gmail profile: %+v", directInfo)
	}
}

func TestGoogleOAuthHTMLGenerators(t *testing.T) {
	// Success HTML
	successHTML := GeneratePopupSuccessHTML("test-jwt-token-123", map[string]string{"email": "alex@gmail.com"}, "https://docupilot.com")
	if !strings.Contains(successHTML, "GOOGLE_AUTH_SUCCESS") || !strings.Contains(successHTML, "test-jwt-token-123") {
		t.Errorf("success HTML missing expected token payload")
	}

	// Error HTML
	errorHTML := GeneratePopupErrorHTML("Google account access was denied")
	if !strings.Contains(errorHTML, "GOOGLE_AUTH_ERROR") || !strings.Contains(errorHTML, "access was denied") {
		t.Errorf("error HTML missing error payload")
	}

	// Demo consent HTML
	consentHTML := GenerateDemoConsentHTML("csrf-state-abc", "/api/auth/google/callback")
	if !strings.Contains(consentHTML, "Sign in with Google") || !strings.Contains(consentHTML, "csrf-state-abc") {
		t.Errorf("consent HTML missing state or title")
	}
}
