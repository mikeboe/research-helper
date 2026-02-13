package embeddings

import (
	"context"
	"fmt"

	"google.golang.org/genai"
)

// GoogleEmbedder wraps Google Vertex AI / Gemini embeddings
type GoogleEmbedder struct {
	client *genai.Client
	model  string
}

// NewGoogleEmbedder creates a new Google Vertex AI embedder
func NewGoogleEmbedder(ctx context.Context, model, apiKey string) (*GoogleEmbedder, error) {

	// Initialize Gemini API client (API Key)
	geminiConfig := &genai.ClientConfig{
		APIKey: apiKey,
	}
	client, err := genai.NewClient(ctx, geminiConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini API client: %w", err)
	}

	return &GoogleEmbedder{
		client: client,
		model:  model,
	}, nil
}

// EmbedText generates embeddings for a single text
func (e *GoogleEmbedder) EmbedText(ctx context.Context, text string) ([]float32, error) {
	outputDim := int32(1536)
	res, err := e.client.Models.EmbedContent(ctx, e.model, []*genai.Content{
		{
			Parts: []*genai.Part{
				{Text: text},
			},
		},
	}, &genai.EmbedContentConfig{
		OutputDimensionality: &outputDim,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to embed text: %w", err)
	}

	if res.Embeddings == nil || len(res.Embeddings) == 0 || len(res.Embeddings[0].Values) == 0 {
		return nil, fmt.Errorf("empty embedding returned")
	}

	return res.Embeddings[0].Values, nil
}

// EmbedTexts generates embeddings for multiple texts
func (e *GoogleEmbedder) EmbedTexts(ctx context.Context, texts []string) ([][]float32, error) {
	// We can implement batching here if needed, but for now sequential is safer
	// as we don't know the exact batch limits/API of the SDK version.
	result := make([][]float32, 0, len(texts))

	for _, text := range texts {
		vec, err := e.EmbedText(ctx, text)
		if err != nil {
			return nil, err
		}
		result = append(result, vec)
	}

	return result, nil
}
