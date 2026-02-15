package main

import (
	"context"
	"fmt"
	"log"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/mikeboe/research-helper/pkg/chat"
	"github.com/mikeboe/research-helper/pkg/config"
	"github.com/mikeboe/research-helper/pkg/database"
	"github.com/mikeboe/research-helper/pkg/embeddings"
	"github.com/mikeboe/research-helper/pkg/research"
	"github.com/mikeboe/research-helper/pkg/server"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	config := config.Load()

	// Database Connection
	db, err := database.NewPostgresDB(context.Background(), config.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Initialize Schema
	if err := db.InitSchema(context.Background()); err != nil {
		log.Fatalf("Failed to initialize schema: %v", err)
	}

	// Service Configuration
	cfg := research.Config{
		Collection: config.CollectionName,
		LLMApiKey:  config.GoogleApiKey,
	}

	// Initialize Embedder
	embedder, err := embeddings.NewGoogleEmbedder(context.Background(), config.EmbeddingModel, cfg.LLMApiKey)
	if err != nil {
		log.Fatalf("Failed to init embedder: %v", err)
	}

	// Initialize Chat Service
	chatSvc, err := chat.NewService(context.Background(), db, config)
	if err != nil {
		log.Fatalf("Failed to init chat service: %v", err)
	}

	// Initialize RAG Tools
	ragTools := chat.NewRagToolset(db, embedder, config)

	// Initialize Service & Handler
	svc := server.NewService(db, cfg, config)
	handler := server.NewHandler(svc, chatSvc, ragTools)

	// Web Server Setup
	r := gin.Default()

	// CORS Setup
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"}, // Allow all for dev
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Mcp-Session-Id", "X-API-Key"}, // Added MCP headers
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	handler.RegisterRoutes(r)

	fmt.Printf("Server starting on port %s\n", config.Port)
	if err := r.Run(":" + config.Port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
