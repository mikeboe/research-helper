package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/mikeboe/research-helper/pkg/database"
	"github.com/mikeboe/research-helper/pkg/research"
)

type Service struct {
	DB  *database.PostgresDB
	Cfg research.Config
}

func NewService(db *database.PostgresDB, cfg research.Config) *Service {
	return &Service{
		DB:  db,
		Cfg: cfg,
	}
}

type Job struct {
	ID        uuid.UUID       `json:"id"`
	Topic     string          `json:"topic"`
	Status    string          `json:"status"`
	Report    *string         `json:"report,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
	Config    json.RawMessage `json:"config"`
}

type CreateJobRequest struct {
	Topic string `json:"topic"`
}

func (s *Service) CreateJob(ctx context.Context, req CreateJobRequest) (*Job, error) {
	configJSON, _ := json.Marshal(map[string]interface{}{
		"max_iterations": 5,
		"collection":     s.Cfg.Collection,
	})

	jobID := uuid.New()
	query := `
		INSERT INTO research_jobs (id, topic, status, config)
		VALUES ($1, $2, 'pending', $3)
		RETURNING id, topic, status, created_at, updated_at
	`

	job := &Job{}
	err := s.DB.Pool.QueryRow(ctx, query, jobID, req.Topic, configJSON).Scan(
		&job.ID, &job.Topic, &job.Status, &job.CreatedAt, &job.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create job: %w", err)
	}

	// Start background worker
	go s.runWorker(job.ID, req.Topic)

	return job, nil
}

func (s *Service) GetJob(ctx context.Context, id uuid.UUID) (*Job, error) {
	query := `
		SELECT id, topic, status, report, created_at, updated_at, config
		FROM research_jobs
		WHERE id = $1
	`
	job := &Job{}
	err := s.DB.Pool.QueryRow(ctx, query, id).Scan(
		&job.ID, &job.Topic, &job.Status, &job.Report, &job.CreatedAt, &job.UpdatedAt, &job.Config,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get job: %w", err)
	}
	return job, nil
}

func (s *Service) ListJobs(ctx context.Context) ([]Job, error) {
	query := `
		SELECT id, topic, status, report, created_at, updated_at, config
		FROM research_jobs
		ORDER BY created_at DESC
		LIMIT 50
	`
	rows, err := s.DB.Pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs: %w", err)
	}
	defer rows.Close()

	var jobs []Job
	for rows.Next() {
		var job Job
		if err := rows.Scan(&job.ID, &job.Topic, &job.Status, &job.Report, &job.CreatedAt, &job.UpdatedAt, &job.Config); err != nil {
			continue
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}

type LogEntry struct {
	ID        int             `json:"id"`
	Timestamp time.Time       `json:"timestamp"`
	Level     string          `json:"level"`
	Message   string          `json:"message"`
	Metadata  json.RawMessage `json:"metadata"`
}

func (s *Service) GetJobLogs(ctx context.Context, jobID uuid.UUID) ([]LogEntry, error) {
	query := `
		SELECT id, timestamp, level, message, metadata
		FROM research_logs
		WHERE job_id = $1
		ORDER BY id ASC
	`
	rows, err := s.DB.Pool.Query(ctx, query, jobID)
	if err != nil {
		return nil, fmt.Errorf("failed to get logs: %w", err)
	}
	defer rows.Close()

	var logs []LogEntry
	for rows.Next() {
		var l LogEntry
		if err := rows.Scan(&l.ID, &l.Timestamp, &l.Level, &l.Message, &l.Metadata); err != nil {
			continue
		}
		logs = append(logs, l)
	}
	return logs, nil
}

func (s *Service) runWorker(jobID uuid.UUID, topic string) {
	ctx := context.Background()

	// Update status to running
	_, _ = s.DB.Pool.Exec(ctx, "UPDATE research_jobs SET status = 'running', updated_at = NOW() WHERE id = $1", jobID)

	// Configure engine with DB logger
	dbLogger := slog.New(NewDBLogHandler(s.DB, jobID))

	engine, err := research.NewEngine(s.Cfg, s.DB)
	if err != nil {
		s.failJob(ctx, jobID, fmt.Sprintf("Failed to init engine: %v", err))
		return
	}

	// Override logger
	engine.Logger = dbLogger

	// Hook for state persistence
	engine.OnStateUpdate = func(state research.ResearchState) {
		stateJSON, err := json.Marshal(state)
		if err != nil {
			dbLogger.Error("Failed to marshal state", "error", err)
			return
		}

		_, err = s.DB.Pool.Exec(context.Background(),
			"UPDATE research_jobs SET state = $2, updated_at = NOW() WHERE id = $1",
			jobID, stateJSON)

		if err != nil {
			dbLogger.Error("Failed to save state to DB", "error", err)
		}
	}

	report, err := engine.Run(ctx, topic)
	if err != nil {
		s.failJob(ctx, jobID, fmt.Sprintf("Research failed: %v", err))
		return
	}

	// Update job with report
	_, err = s.DB.Pool.Exec(ctx,
		"UPDATE research_jobs SET status = 'completed', report = $2, updated_at = NOW() WHERE id = $1",
		jobID, report)

	if err != nil {
		dbLogger.Error("Failed to save final report to DB", "error", err)
	}
}

func (s *Service) failJob(ctx context.Context, jobID uuid.UUID, reason string) {
	// Log the failure
	dbLogger := slog.New(NewDBLogHandler(s.DB, jobID))
	dbLogger.Error(reason)

	// Update status
	_, _ = s.DB.Pool.Exec(ctx, "UPDATE research_jobs SET status = 'failed', updated_at = NOW() WHERE id = $1", jobID)
}
