package server

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/google/uuid"
	"github.com/mikeboe/research-helper/pkg/database"
)

// DBLogHandler is a slog.Handler that writes records to the database
type DBLogHandler struct {
	DB    *database.PostgresDB
	JobID uuid.UUID

	// We wrap a text handler for formatting fallback or local console output if desired
	// But for this specific requirement, we want structured DB inserts.
}

func NewDBLogHandler(db *database.PostgresDB, jobID uuid.UUID) *DBLogHandler {
	return &DBLogHandler{
		DB:    db,
		JobID: jobID,
	}
}

func (h *DBLogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return true // Log everything
}

func (h *DBLogHandler) Handle(ctx context.Context, r slog.Record) error {
	// Extract attributes to JSON
	attrs := make(map[string]interface{})
	r.Attrs(func(a slog.Attr) bool {
		attrs[a.Key] = a.Value.Any()
		return true
	})

	metaJSON, err := json.Marshal(attrs)
	if err != nil {
		// Fallback for marshal error
		metaJSON = []byte("{}")
	}

	query := `
		INSERT INTO research_logs (job_id, timestamp, level, message, metadata)
		VALUES ($1, $2, $3, $4, $5)
	`

	// Use background context for insert to ensure logs persist even if request context cancels
	// (though usually we are running in a background worker context anyway)
	_, err = h.DB.Pool.Exec(context.Background(), query, h.JobID, r.Time, r.Level.String(), r.Message, metaJSON)
	return err
}

func (h *DBLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	// For simplicity in this implementation, we won't support WithAttrs fully
	// creating a new handler chain, as we just want the base functionality.
	// A full implementation would merge attributes.
	return h
}

func (h *DBLogHandler) WithGroup(name string) slog.Handler {
	return h
}
