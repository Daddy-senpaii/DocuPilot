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

func TestMockProviderStream(t *testing.T) {
	mock := &MockProvider{Answer: "DocuPilot streams responses seamlessly."}
	var received []string
	err := mock.GenerateAnswerStream(context.Background(), "Some context", "What is DocuPilot?", func(chunk string) error {
		received = append(received, chunk)
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected stream error: %v", err)
	}
	joined := strings.Join(received, "")
	if joined != "DocuPilot streams responses seamlessly." {
		t.Fatalf("expected joined answer %q, got %q", "DocuPilot streams responses seamlessly.", joined)
	}
}

func TestMockProviderStreamError(t *testing.T) {
	expectedErr := errors.New("simulated stream failure")
	mock := &MockProvider{Err: expectedErr}
	err := mock.GenerateAnswerStream(context.Background(), "Some context", "What is DocuPilot?", func(_ string) error {
		return nil
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "simulated stream failure") {
		t.Fatalf("expected error message to contain 'simulated stream failure', got %v", err)
	}
}

func TestGeminiProviderStreamMissingKey(t *testing.T) {
	gemini := NewGeminiProvider("", "")
	err := gemini.GenerateAnswerStream(context.Background(), "Some context", "What is DocuPilot?", func(_ string) error {
		return nil
	})
	if err == nil {
		t.Fatalf("expected error for missing API key, got nil")
	}
	if !strings.Contains(err.Error(), "GEMINI_API_KEY is not set") {
		t.Fatalf("expected missing key message, got %v", err)
	}
}
