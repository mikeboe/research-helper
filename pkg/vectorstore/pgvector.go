package vectorstore

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgvector/pgvector-go"
)

// Document represents a document with embeddings
type Document struct {
	ID        string                 `json:"id"`
	Content   string                 `json:"content"`
	Metadata  map[string]interface{} `json:"metadata"`
	Embedding []float32              `json:"embedding,omitempty"`
}

// PGVectorStore handles pgvector operations
type PGVectorStore struct {
	pool      *pgxpool.Pool
	tableName string
}

// isValidTableName validates that a table name contains only safe characters
// to prevent SQL injection attacks
func isValidTableName(name string) bool {
	// Only allow alphanumeric characters and underscores
	// Table names must start with a letter or underscore and be between 1-63 chars (PostgreSQL limit)
	matched, _ := regexp.MatchString(`^[a-z_][a-zA-Z0-9_]{0,62}$`, name)
	return matched
}

// NewPGVectorStore creates a new PGVector store
func NewPGVectorStore(pool *pgxpool.Pool, tableName string) (*PGVectorStore, error) {
	if !isValidTableName(tableName) {
		return nil, fmt.Errorf("invalid table name: must contain only alphanumeric characters and underscores, start with a letter or underscore, and be 1-63 characters long")
	}
	return &PGVectorStore{
		pool:      pool,
		tableName: tableName,
	}, nil
}

// AddDocuments adds documents with embeddings to the vector store
func (vs *PGVectorStore) AddDocuments(ctx context.Context, docs []Document) error {
	query := fmt.Sprintf(`
		INSERT INTO %s (content, metadata, embedding)
		VALUES ($1, $2, $3)
	`, pgx.Identifier{vs.tableName}.Sanitize())

	batch := &pgx.Batch{}
	for _, doc := range docs {
		metadataJSON, err := json.Marshal(doc.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}

		embedding := pgvector.NewVector(doc.Embedding)
		batch.Queue(query, doc.Content, metadataJSON, embedding)
	}

	br := vs.pool.SendBatch(ctx, batch)
	defer br.Close()

	for range docs {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("failed to insert document: %w", err)
		}
	}

	return nil
}

// SimilaritySearchResult represents a search result with score
type SimilaritySearchResult struct {
	Document Document
	Score    float64
}

// SimilaritySearch performs a similarity search
func (vs *PGVectorStore) SimilaritySearch(ctx context.Context, queryEmbedding []float32, topK int, sourceFilter string) ([]SimilaritySearchResult, error) {
	var query string
	var args []interface{}

	embedding := pgvector.NewVector(queryEmbedding)

	if sourceFilter != "" {
		query = fmt.Sprintf(`
			SELECT id, content, metadata, 1 - (embedding <=> $1) as similarity
			FROM %s
			WHERE metadata->>'source' = $2
			ORDER BY embedding <=> $1
			LIMIT $3
		`, pgx.Identifier{vs.tableName}.Sanitize())
		args = []interface{}{embedding, sourceFilter, topK}
	} else {
		query = fmt.Sprintf(`
			SELECT id, content, metadata, 1 - (embedding <=> $1) as similarity
			FROM %s
			ORDER BY embedding <=> $1
			LIMIT $2
		`, pgx.Identifier{vs.tableName}.Sanitize())
		args = []interface{}{embedding, topK}
	}

	rows, err := vs.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute similarity search: %w", err)
	}
	defer rows.Close()

	var results []SimilaritySearchResult
	for rows.Next() {
		var doc Document
		var metadataJSON []byte
		var similarity float64

		if err := rows.Scan(&doc.ID, &doc.Content, &metadataJSON, &similarity); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		if err := json.Unmarshal(metadataJSON, &doc.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}

		results = append(results, SimilaritySearchResult{
			Document: doc,
			Score:    similarity,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return results, nil
}

// GetContentBySource retrieves all documents for a specific source
func (vs *PGVectorStore) GetContentBySource(ctx context.Context, source string) ([]Document, error) {
	query := fmt.Sprintf(`
		SELECT id, content, metadata
		FROM %s
		WHERE metadata->>'source' = $1
	`, pgx.Identifier{vs.tableName}.Sanitize())

	rows, err := vs.pool.Query(ctx, query, source)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	var documents []Document
	for rows.Next() {
		var doc Document
		var metadataJSON []byte

		if err := rows.Scan(&doc.ID, &doc.Content, &metadataJSON); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		if err := json.Unmarshal(metadataJSON, &doc.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}

		documents = append(documents, doc)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return documents, nil
}

// GetContentByMetadata retrieves documents matching a complex JSON filter
// Supports logical operators $and, $or, $not within the filter map
func (vs *PGVectorStore) GetContentByMetadata(ctx context.Context, filter map[string]interface{}) ([]Document, error) {
	var args []interface{}
	whereClause, err := vs.buildMetadataQuery(filter, &args)
	if err != nil {
		return nil, fmt.Errorf("failed to build metadata query: %w", err)
	}

	query := fmt.Sprintf(`
		SELECT id, content, metadata
		FROM %s
		WHERE %s
	`, pgx.Identifier{vs.tableName}.Sanitize(), whereClause)

	rows, err := vs.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	var documents []Document
	for rows.Next() {
		var doc Document
		var metadataJSON []byte

		if err := rows.Scan(&doc.ID, &doc.Content, &metadataJSON); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		if err := json.Unmarshal(metadataJSON, &doc.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}

		documents = append(documents, doc)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return documents, nil
}

// buildMetadataQuery recursively builds a SQL WHERE clause for list of conditions
func (vs *PGVectorStore) buildMetadataQuery(filter map[string]interface{}, args *[]interface{}) (string, error) {
	if len(filter) == 0 {
		return "TRUE", nil
	}

	var conditions []string

	for key, value := range filter {
		switch key {
		case "$and", "$or":
			list, ok := value.([]interface{})
			if !ok {
				return "", fmt.Errorf("value for %s must be a list of conditions", key)
			}
			var subConditions []string
			for _, item := range list {
				subMap, ok := item.(map[string]interface{})
				if !ok {
					return "", fmt.Errorf("item in %s list must be a JSON object", key)
				}
				subQuery, err := vs.buildMetadataQuery(subMap, args)
				if err != nil {
					return "", err
				}
				subConditions = append(subConditions, "("+subQuery+")")
			}

			if len(subConditions) == 0 {
				continue
			}

			op := " AND "
			if key == "$or" {
				op = " OR "
			}
			conditions = append(conditions, "("+strings.Join(subConditions, op)+")")

		case "$not":
			subMap, ok := value.(map[string]interface{})
			if !ok {
				return "", fmt.Errorf("value for $not must be a JSON object")
			}
			subQuery, err := vs.buildMetadataQuery(subMap, args)
			if err != nil {
				return "", err
			}
			conditions = append(conditions, "NOT ("+subQuery+")")

		default:
			// Treat as simple equality match: metadata @> '{"key": value}'
			pair := map[string]interface{}{key: value}
			jsonBytes, err := json.Marshal(pair)
			if err != nil {
				return "", fmt.Errorf("failed to marshal metadata pair: %w", err)
			}
			*args = append(*args, jsonBytes)
			conditions = append(conditions, fmt.Sprintf("metadata @> $%d", len(*args)))
		}
	}

	if len(conditions) == 0 {
		return "TRUE", nil
	}

	return strings.Join(conditions, " AND "), nil
}

// UpdateMetadata updates specific fields in the metadata for a document with the given ID.
// It merges the provided updates with the existing metadata using the JSONB concatenation operator (||).
// Existing keys will be overwritten, and new keys will be added.
func (vs *PGVectorStore) UpdateMetadata(ctx context.Context, id string, updates map[string]interface{}) error {
	metadataJSON, err := json.Marshal(updates)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata updates: %w", err)
	}

	query := fmt.Sprintf(`
		UPDATE %s
		SET metadata = metadata || $1
		WHERE id = $2
	`, pgx.Identifier{vs.tableName}.Sanitize())

	result, err := vs.pool.Exec(ctx, query, metadataJSON, id)
	if err != nil {
		return fmt.Errorf("failed to execute update query: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("no document found with id %s", id)
	}

	return nil
}
