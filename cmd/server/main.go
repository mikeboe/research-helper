package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/mikeboe/research-helper/pkg/chat"
	"github.com/mikeboe/research-helper/pkg/database"
	"github.com/mikeboe/research-helper/pkg/research"
	"github.com/mikeboe/research-helper/pkg/server"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// Database Connection
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		// Default fallback for dev
		dbURL = "postgres://postgres:postgres@localhost:5432/research_agent?sslmode=disable"
	}

	db, err := database.NewPostgresDB(context.Background(), dbURL)
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
		Collection: "thesis_db", // Default collection
		LLMApiKey:  os.Getenv("GEMINI_API_KEY"),
	}

	// Initialize Chat Service
	chatSvc, err := chat.NewService(context.Background(), db)
	if err != nil {
		log.Fatalf("Failed to init chat service: %v", err)
	}

	// Initialize Service & Handler
	svc := server.NewService(db, cfg)
	handler := server.NewHandler(svc, chatSvc)

	// Web Server Setup
	r := gin.Default()

	// CORS Setup
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"}, // Allow all for dev
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	handler.RegisterRoutes(r)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	fmt.Printf("Server starting on port %s\n", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
