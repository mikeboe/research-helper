package database

import (
	"context"
	"fmt"
)

func (db *PostgresDB) InitSchema(ctx context.Context) error {
	// 1. Research Jobs Table
	jobsQuery := `
		CREATE TABLE IF NOT EXISTS research_jobs (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			topic TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			config JSONB,
			report TEXT,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		);
	`
	if _, err := db.Pool.Exec(ctx, jobsQuery); err != nil {
		return fmt.Errorf("failed to create research_jobs table: %w", err)
	}

	// 2. Research Logs Table
	logsQuery := `
		CREATE TABLE IF NOT EXISTS research_logs (
			id SERIAL PRIMARY KEY,
			job_id UUID NOT NULL REFERENCES research_jobs(id) ON DELETE CASCADE,
			timestamp TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			level TEXT NOT NULL,
			message TEXT NOT NULL,
			metadata JSONB
		);
	`
	if _, err := db.Pool.Exec(ctx, logsQuery); err != nil {
		return fmt.Errorf("failed to create research_logs table: %w", err)
	}

	// Indexes for faster querying
	if _, err := db.Pool.Exec(ctx, "CREATE INDEX IF NOT EXISTS idx_research_logs_job_id ON research_logs(job_id)"); err != nil {
		return fmt.Errorf("failed to create index on research_logs: %w", err)
	}
	if _, err := db.Pool.Exec(ctx, "CREATE INDEX IF NOT EXISTS idx_research_jobs_created_at ON research_jobs(created_at DESC)"); err != nil {
		return fmt.Errorf("failed to create index on research_jobs: %w", err)
	}

	// 3. Add state column if it doesn't exist (Migration)
	_, err := db.Pool.Exec(ctx, `
		ALTER TABLE research_jobs 
		ADD COLUMN IF NOT EXISTS state JSONB
	`)
	if err != nil {
		return fmt.Errorf("failed to add state column: %w", err)
	}

	// 4. Conversations Table
	convQuery := `
		CREATE TABLE IF NOT EXISTS conversations (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			title TEXT NOT NULL DEFAULT 'New Conversation',
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		);
	`
	if _, err := db.Pool.Exec(ctx, convQuery); err != nil {
		return fmt.Errorf("failed to create conversations table: %w", err)
	}

	// 5. Messages Table
	msgQuery := `
		CREATE TABLE IF NOT EXISTS messages (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
			role TEXT NOT NULL,
			content TEXT NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		);
	`
	if _, err := db.Pool.Exec(ctx, msgQuery); err != nil {
		return fmt.Errorf("failed to create messages table: %w", err)
	}

	// Indexes for chat
	if _, err := db.Pool.Exec(ctx, "CREATE INDEX IF NOT EXISTS idx_messages_conversation_id ON messages(conversation_id)"); err != nil {
		return fmt.Errorf("failed to create index on messages: %w", err)
	}
	if _, err := db.Pool.Exec(ctx, "CREATE INDEX IF NOT EXISTS idx_conversations_updated_at ON conversations(updated_at DESC)"); err != nil {
		return fmt.Errorf("failed to create index on conversations: %w", err)
	}

	return nil
}
