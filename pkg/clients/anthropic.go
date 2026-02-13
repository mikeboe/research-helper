package clients

// import (
// 	"fmt"
// 	"log"
// 	"os"

// 	"github.com/joho/godotenv"
// 	"github.com/mikeboe/thesis-agent/types"
// 	"github.com/tmc/langchaingo/llms/anthropic"
// )

// const (
// 	Claude37Sonnet types.ModelType = "claude-3-7-sonnet-latest"
// 	Claude4Sonnet  types.ModelType = "claude-sonnet-4-20250514"
// 	Claude4Opus    types.ModelType = "claude-opus-4-20250514"
// 	Claude35Haiku  types.ModelType = "claude-3-5-haiku-20241022"
// )

// func AnthropicAI(model types.ModelType) (*anthropic.LLM, error) {
// 	err := godotenv.Load()
// 	if err != nil {
// 		log.Fatalf("Error loading .env file: %v", err)
// 		return nil, err
// 	}
// 	apiKey := os.Getenv("ANTHROPIC_API_KEY")

// 	var modelName string
// 	switch model {
// 	case Claude37Sonnet:
// 		modelName = string(Claude37Sonnet)
// 	case Claude4Sonnet:
// 		modelName = string(Claude4Sonnet)
// 	case Claude4Opus:
// 		modelName = string(Claude4Opus)
// 	case Claude35Haiku:
// 		modelName = string(Claude35Haiku)
// 	default:
// 		return nil, fmt.Errorf("invalid model type: %s", model)
// 	}

// 	llm, err := anthropic.New(anthropic.WithToken(apiKey), anthropic.WithModel(modelName))
// 	if err != nil {
// 		log.Fatal(err)
// 		return nil, err
// 	}

// 	return llm, nil
// }
