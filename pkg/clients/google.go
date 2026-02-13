package clients

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/tmc/langchaingo/llms/googleai"
)

// ModelType is an enum for the available Google AI models.
type ModelType string

const (
	// DefaultModel is the default model to use if none is specified
	DefaultModel ModelType = "gemini-3-flash-preview"
	ProModel     ModelType = "gemini-3-pro-preview"
)

func GoogleAi(model ModelType) (*googleai.GoogleAI, error) {
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
		return nil, err
	}
	ctx := context.Background()
	apiKey := os.Getenv("GOOGLE_API_KEY")

	var modelName string
	switch model {
	case DefaultModel:
		modelName = string(DefaultModel)
	case ProModel:
		modelName = string(ProModel)
	default:
		return nil, fmt.Errorf("invalid model type: %s", model)
	}

	// See https://ai.google.dev/gemini-api/docs/models/gemini for possible models
	llm, err := googleai.New(ctx, googleai.WithAPIKey(apiKey), googleai.WithDefaultModel(modelName))
	if err != nil {
		log.Fatal(err)
		return nil, err
	}

	return llm, nil
}
