package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresDB wraps the database connection pool
type PostgresDB struct {
	Pool *pgxpool.Pool
}

// NewPostgresDB creates a new PostgreSQL database connection
func NewPostgresDB(ctx context.Context, databaseURL string) (*PostgresDB, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database URL: %w", err)
	}

	// Configure connection pool
	config.MaxConns = 25
	config.MinConns = 5

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Verify connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &PostgresDB{Pool: pool}, nil
}

// Close closes the database connection pool
func (db *PostgresDB) Close() {
	db.Pool.Close()
}

// EnsureVectorExtension ensures the pgvector extension is installed
func (db *PostgresDB) EnsureVectorExtension(ctx context.Context) error {
	_, err := db.Pool.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS vector")
	return err
}

// CreateEmbeddingsTable creates the embeddings table if it doesn't exist
func (db *PostgresDB) CreateEmbeddingsTable(ctx context.Context, tableName string, dimension int) error {
	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			content TEXT NOT NULL,
			metadata JSONB,
			embedding vector(%d),
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)
	`, tableName, dimension)

	_, err := db.Pool.Exec(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to create table %s: %w", tableName, err)
	}

	// Create index for vector similarity search
	// HNSW and IVFFlat support up to 2000 dimensions.
	// If dimensions > 2000, we skip index creation and rely on exact search (slower but works).
	if dimension <= 2000 {
		indexQuery := fmt.Sprintf(`
			CREATE INDEX IF NOT EXISTS %s_embedding_idx
			ON %s USING hnsw (embedding vector_cosine_ops)
		`, tableName, tableName)

		_, err = db.Pool.Exec(ctx, indexQuery)
		if err != nil {
			return fmt.Errorf("failed to create index on %s: %w", tableName, err)
		}
	}

	return nil
}
