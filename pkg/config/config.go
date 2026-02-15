package config

import (
	"os"
	"strconv"
)

type Config struct {
	GoogleApiKey   string
	DatabaseURL    string
	ReasoningModel string
	FastModel      string
	Port           string
	ChunkSize      int
	ChunkOverlap   int
	EmbeddingModel string
	CollectionName string
}

func Load() *Config {

	if os.Getenv("GOOGLE_API_KEY") != "" {
		return &Config{
			GoogleApiKey:   getEnv("GOOGLE_API_KEY", ""),
			DatabaseURL:    getEnv("DATABASE_URL", ""),
			ReasoningModel: getEnv("REASONING_MODEL", "gemini-3-pro-preview"),
			FastModel:      getEnv("FAST_MODEL", "gemini-3-flash-preview"),
			Port:           getEnv("PORT", "3000"),
			ChunkSize:      getEnvAsInt("CHUNK_SIZE", 1000),
			ChunkOverlap:   getEnvAsInt("CHUNK_OVERLAP", 200),
			EmbeddingModel: getEnv("EMBEDDING_MODEL", "gemini-embedding-001"),
			CollectionName: getEnv("COLLECTION_NAME", "thesis_db"),
		}
	}

	return &Config{
		GoogleApiKey:   "",
		DatabaseURL:    "",
		ReasoningModel: "",
		FastModel:      "",
		Port:           "",
		ChunkSize:      1000,
		ChunkOverlap:   200,
		EmbeddingModel: "",
		CollectionName: "",
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}
