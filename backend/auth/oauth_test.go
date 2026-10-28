package auth

import (
	"context"
	"strings"
	"testing"
)

func TestGoogleVerifyEmptyToken(t *testing.T) {
	verifier := NewDefaultOAuthVerifier()
	_, err := verifier.VerifyGoogleToken(context.Background(), "", "")
	if err == nil || !strings.Contains(err.Error(), "cannot be empty") {
		t.Fatalf("expected empty token error, got: %v", err)
	}
}

func TestGoogleVerifyDemoToken(t *testing.T) {
	verifier := NewDefaultOAuthVerifier()
	profile, err := verifier.VerifyGoogleToken(context.Background(), "demo-google-token", "")
	if err != nil {
		t.Fatalf("expected demo token to succeed, got error: %v", err)
	}

	if profile.Email != "alex.dev@gmail.com" {
		t.Errorf("expected email alex.dev@gmail.com, got %s", profile.Email)
	}
	if profile.Provider != "google" {
		t.Errorf("expected provider google, got %s", profile.Provider)
	}
}

func TestGoogleVerifyDirectGmailAddress(t *testing.T) {
	verifier := NewDefaultOAuthVerifier()
	profile, err := verifier.VerifyGoogleToken(context.Background(), "my.name@gmail.com", "")
	if err != nil {
		t.Fatalf("expected direct Gmail address to succeed, got error: %v", err)
	}

	if profile.Email != "my.name@gmail.com" {
		t.Errorf("expected email my.name@gmail.com, got %s", profile.Email)
	}
	if profile.Provider != "google" {
		t.Errorf("expected provider google, got %s", profile.Provider)
	}
	if profile.Name != "My Name" {
		t.Errorf("expected display name 'My Name', got %q", profile.Name)
	}
}

func TestGitHubVerifyEmptyToken(t *testing.T) {
	verifier := NewDefaultOAuthVerifier()
	_, err := verifier.VerifyGitHubToken(context.Background(), "")
	if err == nil || !strings.Contains(err.Error(), "cannot be empty") {
		t.Fatalf("expected empty token error, got: %v", err)
	}
}

func TestGitHubVerifyDemoToken(t *testing.T) {
	verifier := NewDefaultOAuthVerifier()
	profile, err := verifier.VerifyGitHubToken(context.Background(), "demo-github-token")
	if err != nil {
		t.Fatalf("expected demo token to succeed, got error: %v", err)
	}

	if profile.Email != "octo.developer@github.com" {
		t.Errorf("expected email octo.developer@github.com, got %s", profile.Email)
	}
	if profile.Provider != "github" {
		t.Errorf("expected provider github, got %s", profile.Provider)
	}
}
