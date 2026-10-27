package ai

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

// AIProvider defines the contract for an AI service capable of answering questions grounded in document context.
type AIProvider interface {
	GenerateAnswer(ctx context.Context, contextText, question string) (string, error)
	GenerateAnswerStream(ctx context.Context, contextText, question string, onChunk func(chunk string) error) error
}

// GeminiProvider implements AIProvider using Google's Gemini API.
type GeminiProvider struct {
	apiKey    string
	modelName string
}

// NewGeminiProvider creates a new GeminiProvider.
func NewGeminiProvider(apiKey, modelName string) *GeminiProvider {
	if modelName == "" {
		modelName = "gemini-flash-latest"
	}
	return &GeminiProvider{
		apiKey:    apiKey,
		modelName: modelName,
	}
}

// GenerateAnswerStream streams the Gemini response chunk-by-chunk using Gemini's streaming API.
func (g *GeminiProvider) GenerateAnswerStream(ctx context.Context, contextText, question string, onChunk func(chunk string) error) error {
	if g.apiKey == "" {
		return errors.New("GEMINI_API_KEY is not set. Please set the GEMINI_API_KEY environment variable")
	}

	client, err := genai.NewClient(ctx, option.WithAPIKey(g.apiKey))
	if err != nil {
		return fmt.Errorf("failed to initialize Gemini client: %w", err)
	}
	defer client.Close()

	model := client.GenerativeModel(g.modelName)
	model.SetTemperature(0.2) // Low temperature for grounded, factual document answers

	prompt := fmt.Sprintf(
		"You are DocuPilot, an AI document assistant.\n"+
			"Answer the user's question accurately using ONLY the provided document context.\n"+
			"If the context does not contain enough information to answer the question, state: "+
			"\"Based on the provided document, I cannot answer this question.\"\n\n"+
			"DOCUMENT CONTEXT:\n---\n%s\n---\n\n"+
			"QUESTION:\n%s\n\n"+
			"ANSWER:",
		contextText,
		question,
	)

	iter := model.GenerateContentStream(ctx, genai.Text(prompt))
	for {
		resp, err := iter.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return fmt.Errorf("Gemini API error: %w", err)
		}

		for _, cand := range resp.Candidates {
			if cand.Content != nil {
				for _, part := range cand.Content.Parts {
					if textPart, ok := part.(genai.Text); ok {
						if err := onChunk(string(textPart)); err != nil {
							return err
						}
					}
				}
			}
		}
	}

	return nil
}

// GenerateAnswer calls GenerateAnswerStream and accumulates the complete response.
func (g *GeminiProvider) GenerateAnswer(ctx context.Context, contextText, question string) (string, error) {
	var builder strings.Builder
	err := g.GenerateAnswerStream(ctx, contextText, question, func(chunk string) error {
		builder.WriteString(chunk)
		return nil
	})
	if err != nil {
		return "", err
	}

	answer := strings.TrimSpace(builder.String())
	if answer == "" {
		return "", errors.New("received empty answer from Gemini")
	}

	return answer, nil
}

// MockProvider is used for unit tests and local testing when no API key is present.
type MockProvider struct {
	Answer string
	Err    error
}

func (m *MockProvider) GenerateAnswer(_ context.Context, _, _ string) (string, error) {
	if m.Err != nil {
		return "", m.Err
	}
	if m.Answer != "" {
		return m.Answer, nil
	}
	return "This is a mock answer for testing without an API key.", nil
}

func (m *MockProvider) GenerateAnswerStream(_ context.Context, _, _ string, onChunk func(chunk string) error) error {
	if m.Err != nil {
		return m.Err
	}
	ans := m.Answer
	if ans == "" {
		ans = "This is a mock answer for testing without an API key."
	}
	words := strings.Fields(ans)
	for i, word := range words {
		chunk := word
		if i < len(words)-1 {
			chunk += " "
		}
		if err := onChunk(chunk); err != nil {
			return err
		}
	}
	return nil
}
