package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/raghavchitkara36/llmscope/models"

	_ "modernc.org/sqlite"
)

// sentinel errors
var (
	ErrTraceNotFound   = errors.New("storage: trace not found")
	ErrProjectNotFound = errors.New("storage: project not found")
)

// SQLiteStorage implements Storage using an embedded SQLite database
type SQLiteStorage struct {
	db *sql.DB
}

// compile-time interface check
var _ Storage = (*SQLiteStorage)(nil)

// NewSQLiteStorage opens (or creates) a SQLite database at the given path
// and initialises the schema. Returns an error if the database cannot be opened.
func NewSQLiteStorage(path string) (*SQLiteStorage, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("storage: opening sqlite database: %w", err)
	}

	// verify connection is alive
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("storage: pinging sqlite database: %w", err)
	}

	s := &SQLiteStorage{db: db}

	if err := s.createSchema(); err != nil {
		return nil, fmt.Errorf("storage: creating schema: %w", err)
	}

	return s, nil
}

// createSchema creates tables if they don't already exist
func (s *SQLiteStorage) createSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS projects (
		project_id   TEXT PRIMARY KEY,
		project_name TEXT NOT NULL,
		created_at   INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS traces (
		trace_id            TEXT PRIMARY KEY,
		project_id          TEXT NOT NULL,
		session_id          TEXT,
		provider            TEXT NOT NULL,
		model               TEXT NOT NULL,
		prompt              TEXT NOT NULL,  -- stored as JSON
		response            TEXT NOT NULL,
		request_timestamp   INTEGER NOT NULL,
		response_timestamp  INTEGER NOT NULL,
		input_tokens        INTEGER NOT NULL DEFAULT 0,
		output_tokens       INTEGER NOT NULL DEFAULT 0,
		cost_usd            REAL NOT NULL DEFAULT 0.0,
		tags                TEXT,           -- stored as JSON
		error_code          TEXT,
		error_message       TEXT,
		FOREIGN KEY (project_id) REFERENCES projects(project_id)
	);

	CREATE INDEX IF NOT EXISTS idx_traces_project_id ON traces(project_id);
	CREATE INDEX IF NOT EXISTS idx_traces_provider   ON traces(provider);
	CREATE INDEX IF NOT EXISTS idx_traces_model      ON traces(model);
	CREATE INDEX IF NOT EXISTS idx_traces_request_ts ON traces(request_timestamp);
	`

	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("storage: executing schema: %w", err)
	}

	return nil
}

// SaveTrace persists a single trace to the database
func (s *SQLiteStorage) SaveTrace(ctx context.Context, trace *models.Trace) error {
	// serialize []Message to JSON for storage
	promptJSON, err := json.Marshal(trace.Prompt)
	if err != nil {
		return fmt.Errorf("storage: marshaling prompt: %w", err)
	}

	// serialize map[string]string tags to JSON
	tagsJSON, err := json.Marshal(trace.Tags)
	if err != nil {
		return fmt.Errorf("storage: marshaling tags: %w", err)
	}

	// extract error fields — stored as flat columns, not JSON
	var errorCode, errorMessage string
	if trace.Error != nil {
		errorCode = trace.Error.Code
		errorMessage = trace.Error.Message
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO traces (
			trace_id, project_id, session_id, provider, model,
			prompt, response, request_timestamp, response_timestamp,
			input_tokens, output_tokens, cost_usd,
			tags, error_code, error_message
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		trace.TraceID,
		trace.ProjectID,
		trace.SessionID,
		trace.Provider,
		trace.Model,
		string(promptJSON),
		trace.Response,
		trace.RequestTimestamp,
		trace.ResponseTimestamp,
		trace.InputTokens,
		trace.OutputTokens,
		trace.CostUSD,
		string(tagsJSON),
		errorCode,
		errorMessage,
	)
	if err != nil {
		return fmt.Errorf("storage: inserting trace %s: %w", trace.TraceID, err)
	}

	return nil
}

// GetTrace retrieves a single trace by ID
func (s *SQLiteStorage) GetTrace(ctx context.Context, traceID string) (*models.Trace, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT
			trace_id, project_id, session_id, provider, model,
			prompt, response, request_timestamp, response_timestamp,
			input_tokens, output_tokens, cost_usd,
			tags, error_code, error_message
		FROM traces
		WHERE trace_id = ?
	`, traceID)

	trace, err := scanTrace(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTraceNotFound
		}
		return nil, fmt.Errorf("storage: getting trace %s: %w", traceID, err)
	}

	return trace, nil
}

// GetTraces retrieves traces for a project with optional filtering.
// limit/offset are currently applied at the tracer layer; accepted here
// to satisfy the Storage interface.
func (s *SQLiteStorage) GetTraces(ctx context.Context, projectID string, filter models.TraceFilter, limit, offset int64) ([]*models.Trace, error) {
	_ = limit
	_ = offset
	// build query dynamically based on filter
	query := `
		SELECT
			trace_id, project_id, session_id, provider, model,
			prompt, response, request_timestamp, response_timestamp,
			input_tokens, output_tokens, cost_usd,
			tags, error_code, error_message
		FROM traces
		WHERE project_id = ?
	`
	args := []any{projectID}

	// apply filters dynamically
	if filter.Provider != "" {
		query += " AND provider = ?"
		args = append(args, filter.Provider)
	}
	if filter.Model != "" {
		query += " AND model = ?"
		args = append(args, filter.Model)
	}
	if filter.SessionID != "" {
		query += " AND session_id = ?"
		args = append(args, filter.SessionID)
	}
	if filter.StartDate > 0 {
		query += " AND request_timestamp >= ?"
		args = append(args, filter.StartDate)
	}
	if filter.EndDate > 0 {
		query += " AND request_timestamp <= ?"
		args = append(args, filter.EndDate)
	}
	if filter.HasError != nil {
		if *filter.HasError {
			query += " AND error_message != ''"
		} else {
			query += " AND (error_message = '' OR error_message IS NULL)"
		}
	}

	query += " ORDER BY request_timestamp DESC"
	query += " LIMIT ? OFFSET ?"

	args = append(args, limit, offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("storage: querying traces: %w", err)
	}
	defer rows.Close()

	// fmt.Println(query)
	// fmt.Println("args ", args)
	// fmt.Println(rows)

	var traces []*models.Trace
	for rows.Next() {
		trace, err := scanTrace(rows)
		if err != nil {
			return nil, fmt.Errorf("storage: scanning trace: %w", err)
		}
		traces = append(traces, trace)
	}

	// always check rows.Err() after iteration
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("storage: iterating traces: %w", err)
	}

	return traces, nil
}

// --- Project methods ---

// SaveProject persists a project to the database
func (s *SQLiteStorage) SaveProject(ctx context.Context, project *models.Project) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO projects (project_id, project_name, created_at)
		VALUES (?, ?, ?)
		ON CONFLICT(project_id) DO UPDATE SET project_name = excluded.project_name
	`,
		project.ProjectID,
		project.ProjectName,
		project.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("storage: saving project %s: %w", project.ProjectID, err)
	}

	return nil
}

// GetProject retrieves a single project by ID
func (s *SQLiteStorage) GetProject(ctx context.Context, projectID string) (*models.Project, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT project_id, project_name, created_at
		FROM projects
		WHERE project_id = ?
	`, projectID)

	var p models.Project
	if err := row.Scan(&p.ProjectID, &p.ProjectName, &p.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrProjectNotFound
		}
		return nil, fmt.Errorf("storage: getting project %s: %w", projectID, err)
	}

	return &p, nil
}

// GetProjects retrieves all projects ordered by creation time
func (s *SQLiteStorage) GetProjects(ctx context.Context) ([]*models.Project, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT project_id, project_name, created_at
		FROM projects
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("storage: querying projects: %w", err)
	}
	defer rows.Close()

	var projects []*models.Project
	for rows.Next() {
		var p models.Project
		if err := rows.Scan(&p.ProjectID, &p.ProjectName, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("storage: scanning project: %w", err)
		}
		projects = append(projects, &p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("storage: iterating projects: %w", err)
	}

	return projects, nil
}

// GetStats returns aggregated statistics for a project
func (s *SQLiteStorage) GetStats(ctx context.Context, projectID string) (*models.Stats, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*)                                          AS total_traces,
			COALESCE(SUM(cost_usd), 0)                       AS total_cost,
			COALESCE(AVG(response_timestamp - request_timestamp), 0) AS avg_latency_ms,
			COUNT(CASE WHEN error_message != '' AND error_message IS NOT NULL THEN 1 END) AS error_count,
			COALESCE(SUM(input_tokens + output_tokens), 0)   AS total_tokens,
			COALESCE(MIN(request_timestamp), 0)              AS start_time,
			COALESCE(MAX(request_timestamp), 0)              AS end_time
		FROM traces
		WHERE project_id = ?
	`, projectID)

	var stats models.Stats
	if err := row.Scan(
		&stats.TotalTraces,
		&stats.TotalCost,
		&stats.AvgLatencyMS,
		&stats.ErrorCount,
		&stats.TotalTokens,
		&stats.StartTime,
		&stats.EndTime,
	); err != nil {
		return nil, fmt.Errorf("storage: getting stats for project %s: %w", projectID, err)
	}

	return &stats, nil
}

// Close closes the underlying database connection
func (s *SQLiteStorage) Close() error {
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("storage: closing database: %w", err)
	}
	return nil
}

// scanner is implemented by both *sql.Row and *sql.Rows
type scanner interface {
	Scan(dest ...any) error
}

// scanTrace scans a row into a Trace — shared by GetTrace and GetTraces
func scanTrace(s scanner) (*models.Trace, error) {
	var (
		trace        models.Trace
		promptJSON   string
		tagsJSON     string
		errorCode    sql.NullString
		errorMessage sql.NullString
	)

	if err := s.Scan(
		&trace.TraceID,
		&trace.ProjectID,
		&trace.SessionID,
		&trace.Provider,
		&trace.Model,
		&promptJSON,
		&trace.Response,
		&trace.RequestTimestamp,
		&trace.ResponseTimestamp,
		&trace.InputTokens,
		&trace.OutputTokens,
		&trace.CostUSD,
		&tagsJSON,
		&errorCode,
		&errorMessage,
	); err != nil {
		return nil, err
	}

	// deserialize prompt JSON back to []Message
	if err := json.Unmarshal([]byte(promptJSON), &trace.Prompt); err != nil {
		return nil, fmt.Errorf("unmarshaling prompt: %w", err)
	}

	// deserialize tags JSON back to map[string]string
	if strings.TrimSpace(tagsJSON) != "" && tagsJSON != "null" {
		if err := json.Unmarshal([]byte(tagsJSON), &trace.Tags); err != nil {
			return nil, fmt.Errorf("unmarshaling tags: %w", err)
		}
	}

	// reconstruct TraceError if error columns are populated
	if errorMessage.Valid && errorMessage.String != "" {
		trace.Error = &models.TraceError{
			Code:    errorCode.String,
			Message: errorMessage.String,
		}
	}

	return &trace, nil
}
