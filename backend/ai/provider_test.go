package ai

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestMockProviderSuccess(t *testing.T) {
	mock := &MockProvider{Answer: "DocuPilot is a learning-first project."}
	answer, err := mock.GenerateAnswer(context.Background(), "Some context", "What is DocuPilot?")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if answer != "DocuPilot is a learning-first project." {
		t.Fatalf("unexpected answer: %q", answer)
	}
}

func TestMockProviderError(t *testing.T) {
	expectedErr := errors.New("simulated AI failure")
	mock := &MockProvider{Err: expectedErr}
	_, err := mock.GenerateAnswer(context.Background(), "Some context", "What is DocuPilot?")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "simulated AI failure") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestGeminiProviderMissingKey(t *testing.T) {
	gemini := NewGeminiProvider("", "")
	_, err := gemini.GenerateAnswer(context.Background(), "Some context", "What is DocuPilot?")
	if err == nil {
		t.Fatalf("expected error for missing API key, got nil")
	}
	if !strings.Contains(err.Error(), "GEMINI_API_KEY is not set") {
		t.Fatalf("expected missing key message, got %v", err)
	}
}
