package storage

import (
	"context"

	"github.com/raghavchitkara36/llmscope/models"
)

type Storage interface {
	SaveTrace(ctx context.Context, trace *models.Trace) error
	GetTrace(ctx context.Context, traceID string) (*models.Trace, error)
	GetTraces(ctx context.Context, projectID string, filter models.TraceFilter, limit, offset int64) ([]*models.Trace, error)
	SaveProject(ctx context.Context, project *models.Project) error
	GetProject(ctx context.Context, projectID string) (*models.Project, error)
	GetProjects(ctx context.Context) ([]*models.Project, error)
	GetStats(ctx context.Context, projectID string) (*models.Stats, error)
	Close() error
}
