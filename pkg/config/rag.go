package config

import (
	"os"
	"strconv"
)

type RagConfig struct {
	GoogleApiKey string
	DatabaseURL  string
	GoogleModel  string
	Port         string
	ChunkSize    int
	ChunkOverlap int
}

func LoadRagConfig() *RagConfig {

	if os.Getenv("GOOGLE_API_KEY") != "" {
		return &RagConfig{
			GoogleApiKey: getEnv("GOOGLE_API_KEY", ""),
			DatabaseURL:  getEnv("DATABASE_URL", ""),
			GoogleModel:  getEnv("GOOGLE_MODEL", "gemini-embedding-001"),
			Port:         getEnv("PORT", "3000"),
			ChunkSize:    getEnvAsInt("CHUNK_SIZE", 1000),
			ChunkOverlap: getEnvAsInt("CHUNK_OVERLAP", 200),
		}
	}

	return &RagConfig{
		GoogleApiKey: "",
		DatabaseURL:  "",
		GoogleModel:  "",
		Port:         "",
		ChunkSize:    1000,
		ChunkOverlap: 200,
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
