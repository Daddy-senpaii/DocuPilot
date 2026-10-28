package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

var (
	ErrEmptyToken         = errors.New("authentication token or credential cannot be empty")
	ErrUnverifiedEmail    = errors.New("email address associated with this account is not verified")
	ErrClientIDMismatch   = errors.New("OAuth token client ID does not match expected application client ID")
	ErrInvalidOAuthToken  = errors.New("invalid or expired OAuth token")
)

// OAuthProfile holds verified user identity data from third-party OAuth providers.
type OAuthProfile struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Provider string `json:"provider"` // "google" or "github"
	Sub      string `json:"sub"`      // Unique provider subject ID
}

// OAuthVerifier defines the contract for verifying third-party OAuth tokens.
type OAuthVerifier interface {
	VerifyGoogleToken(ctx context.Context, idToken string, expectedClientID string) (*OAuthProfile, error)
	VerifyGitHubToken(ctx context.Context, token string) (*OAuthProfile, error)
}

// GoogleTokenInfo represents the payload returned by Google's tokeninfo verification endpoint.
type GoogleTokenInfo struct {
	Email         string `json:"email"`
	EmailVerified string `json:"email_verified"`
	Name          string `json:"name"`
	Sub           string `json:"sub"`
	Aud           string `json:"aud"`
	Error         string `json:"error_description"`
}

// GitHubUser represents the user payload returned by GitHub's /user API.
type GitHubUser struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// GitHubEmailEntry represents an item from GitHub's /user/emails API.
type GitHubEmailEntry struct {
	Email    string `json:"email"`
	Primary  bool   `json:"primary"`
	Verified bool   `json:"verified"`
}

// DefaultOAuthVerifier verifies OAuth tokens against Google and GitHub API endpoints.
type DefaultOAuthVerifier struct {
	httpClient *http.Client
}

// NewDefaultOAuthVerifier creates a new OAuth verifier with a sensible HTTP timeout.
func NewDefaultOAuthVerifier() *DefaultOAuthVerifier {
	return &DefaultOAuthVerifier{
		httpClient: &http.Client{Timeout: 7 * time.Second},
	}
}

// VerifyGoogleToken verifies a Google ID token via Google's OAuth2 tokeninfo endpoint.
func (v *DefaultOAuthVerifier) VerifyGoogleToken(ctx context.Context, idToken string, expectedClientID string) (*OAuthProfile, error) {
	trimmed := strings.TrimSpace(idToken)
	if trimmed == "" {
		return nil, ErrEmptyToken
	}

	// Demo / testing fallback token
	if trimmed == "demo-google-token" {
		return &OAuthProfile{
			Email:    "alex.dev@gmail.com",
			Name:     "Alex (Demo Google User)",
			Provider: "google",
			Sub:      "google_demo_1092837465",
		}, nil
	}

	reqURL := fmt.Sprintf("https://oauth2.googleapis.com/tokeninfo?id_token=%s", trimmed)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build Google verification request: %w", err)
	}

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to reach Google verification endpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, ErrInvalidOAuthToken
	}

	var info GoogleTokenInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("failed to decode Google token response: %w", err)
	}

	if info.Error != "" {
		return nil, fmt.Errorf("google token verification error: %s", info.Error)
	}

	// Verify audience matches configured Google Client ID (if configured)
	if expectedClientID != "" && info.Aud != expectedClientID {
		return nil, ErrClientIDMismatch
	}

	// Ensure email is verified by Google
	if info.EmailVerified != "true" && info.EmailVerified != "1" {
		return nil, ErrUnverifiedEmail
	}

	normalizedEmail, err := NormalizeAndValidateEmail(info.Email)
	if err != nil {
		return nil, fmt.Errorf("google returned invalid email address: %w", err)
	}

	return &OAuthProfile{
		Email:    normalizedEmail,
		Name:     info.Name,
		Provider: "google",
		Sub:      info.Sub,
	}, nil
}

// VerifyGitHubToken verifies a GitHub access token against GitHub's /user API.
func (v *DefaultOAuthVerifier) VerifyGitHubToken(ctx context.Context, token string) (*OAuthProfile, error) {
	trimmed := strings.TrimSpace(token)
	if trimmed == "" {
		return nil, ErrEmptyToken
	}

	// Demo / testing fallback token
	if trimmed == "demo-github-token" {
		return &OAuthProfile{
			Email:    "octo.developer@github.com",
			Name:     "Octo Developer (Demo)",
			Provider: "github",
			Sub:      "github_demo_987654321",
		}, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build GitHub request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+trimmed)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "DocuPilot-App")

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to contact GitHub API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, ErrInvalidOAuthToken
	}

	var ghUser GitHubUser
	if err := json.NewDecoder(resp.Body).Decode(&ghUser); err != nil {
		return nil, fmt.Errorf("failed to parse GitHub user profile: %w", err)
	}

	email := ghUser.Email
	// If email is private on GitHub profile, query the /user/emails endpoint
	if email == "" {
		emailsReq, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user/emails", nil)
		if err == nil {
			emailsReq.Header.Set("Authorization", "Bearer "+trimmed)
			emailsReq.Header.Set("Accept", "application/vnd.github+json")
			emailsReq.Header.Set("User-Agent", "DocuPilot-App")

			if emailsResp, err := v.httpClient.Do(emailsReq); err == nil {
				defer emailsResp.Body.Close()
				if emailsResp.StatusCode == http.StatusOK {
					var emails []GitHubEmailEntry
					if err := json.NewDecoder(emailsResp.Body).Decode(&emails); err == nil {
						for _, entry := range emails {
							if entry.Primary && entry.Verified {
								email = entry.Email
								break
							}
						}
					}
				}
			}
		}
	}

	if email == "" {
		return nil, errors.New("no verified primary email found on this GitHub account")
	}

	normalizedEmail, err := NormalizeAndValidateEmail(email)
	if err != nil {
		return nil, fmt.Errorf("github returned invalid email: %w", err)
	}

	name := ghUser.Name
	if name == "" {
		name = ghUser.Login
	}

	return &OAuthProfile{
		Email:    normalizedEmail,
		Name:     name,
		Provider: "github",
		Sub:      fmt.Sprintf("%d", ghUser.ID),
	}, nil
}

// MockOAuthVerifier implements OAuthVerifier for unit and integration testing.
type MockOAuthVerifier struct {
	GoogleProfile *OAuthProfile
	GoogleErr     error
	GitHubProfile *OAuthProfile
	GitHubErr     error
}

func (m *MockOAuthVerifier) VerifyGoogleToken(_ context.Context, _ string, _ string) (*OAuthProfile, error) {
	if m.GoogleErr != nil {
		return nil, m.GoogleErr
	}
	if m.GoogleProfile != nil {
		return m.GoogleProfile, nil
	}
	return &OAuthProfile{
		Email:    "test.google@example.com",
		Name:     "Test Google User",
		Provider: "google",
		Sub:      "sub_google_123",
	}, nil
}

func (m *MockOAuthVerifier) VerifyGitHubToken(_ context.Context, _ string) (*OAuthProfile, error) {
	if m.GitHubErr != nil {
		return nil, m.GitHubErr
	}
	if m.GitHubProfile != nil {
		return m.GitHubProfile, nil
	}
	return &OAuthProfile{
		Email:    "test.github@example.com",
		Name:     "Test GitHub User",
		Provider: "github",
		Sub:      "sub_github_456",
	}, nil
}
