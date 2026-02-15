package main

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/mikeboe/research-helper/pkg/config"
	"github.com/mikeboe/research-helper/pkg/database"
	"github.com/mikeboe/research-helper/pkg/research"
	"github.com/spf13/cobra"
)

var (
	topic          string
	collectionName string
)

func main() {
	// Setup structured logging
	handler := slog.NewTextHandler(os.Stdout, nil)
	slog.SetDefault(slog.New(handler))
	config := config.Load()

	// Load .env file
	if err := godotenv.Load(); err != nil {
		// It's okay if .env doesn't exist, as long as env vars are set
	}

	rootCmd := &cobra.Command{
		Use:   "research-helper",
		Short: "A terminal-based research agent",
		Long:  `ResearchHelper-CLI is an autonomous agent that researches a thesis topic by iterating through a Plan-Execute-Reflect loop.`,
		Run: func(cmd *cobra.Command, args []string) {

			// Check if topic provided via flags
			topicFlagChanged := cmd.Flags().Changed("topic")

			if !topicFlagChanged {
				// Interactive Mode
				reader := bufio.NewReader(os.Stdin)

				fmt.Print("Enter research topic: ")
				input, _ := reader.ReadString('\n')
				topic = strings.TrimSpace(input)
				if topic == "" {
					slog.Error("Topic cannot be empty")
					os.Exit(1)
				}

				fmt.Printf("Enter collection name (default: %s): ", collectionName)
				input, _ = reader.ReadString('\n')
				input = strings.TrimSpace(input)
				if input != "" {
					collectionName = input
				}
			} else {
				// Non-Interactive Mode (Flag provided)
				if topic == "" {
					slog.Error("--topic flag provided but empty")
					os.Exit(1)
				}
				// Collection uses default from flag definition if not set
			}

			if collectionName == "" {
				collectionName = "thesis_db"
			}

			slog.Info("Starting research", "topic", topic, "collection", collectionName)

			// Initialize DB
			dbURL := os.Getenv("DATABASE_URL")
			if dbURL == "" {
				dbURL = "postgres://postgres:postgres@localhost:5432/research_agent?sslmode=disable"
			}
			db, err := database.NewPostgresDB(context.Background(), dbURL)
			if err != nil {
				slog.Error("Failed to connect to database", "error", err)
				os.Exit(1)
			}
			defer db.Close()

			if err := db.InitSchema(context.Background()); err != nil {
				slog.Error("Failed to initialize schema", "error", err)
				os.Exit(1)
			}

			// Configure Engine
			cfg := research.Config{
				Collection: collectionName,
				LLMApiKey:  os.Getenv("GEMINI_API_KEY"),
			}

			// Initialize Engine
			engine, err := research.NewEngine(cfg, db, config)
			if err != nil {
				slog.Error("Error initializing engine", "error", err)
				os.Exit(1)
			}

			// Run Research Loop
			if _, err := engine.Run(context.Background(), topic); err != nil {
				slog.Error("Error running research", "error", err)
				os.Exit(1)
			}
		},
	}

	rootCmd.Flags().StringVarP(&topic, "topic", "t", "", "The research topic")
	rootCmd.Flags().StringVarP(&collectionName, "collection", "c", "thesis_db", "The target vector DB collection name")

	if err := rootCmd.Execute(); err != nil {
		slog.Error("Command execution failed", "error", err)
		os.Exit(1)
	}
}
