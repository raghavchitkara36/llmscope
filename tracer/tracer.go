package tracer

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/raghavchitkara36/llmscope/models"
	"github.com/raghavchitkara36/llmscope/storage"
)

const (
	// defaultChannelSize is the number of traces that can be buffered
	// before the channel is considered full. 512 chosen to handle bursts
	// of LLM calls without dropping traces — each trace is ~2KB so this
	// is ~1MB of memory at most.
	defaultChannelSize = 512
)

// Tracer receives traces from provider wrappers and persists them
// asynchronously — LLM call is never blocked by storage I/O.
type Tracer struct {
	storage storage.Storage    // unexported — storage is an implementation detail
	writeCh chan *models.Trace // buffered channel — decouples LLM calls from storage writes
	done    chan struct{}      // signals the background goroutine to stop
	wg      sync.WaitGroup     // waits for background goroutine to finish draining
}

// New creates a Tracer and starts the background write goroutine.
// The caller must call Close() when done to flush pending traces and
// release resources.
func New(s storage.Storage) *Tracer {
	t := &Tracer{
		storage: s,
		writeCh: make(chan *models.Trace, defaultChannelSize),
		done:    make(chan struct{}),
	}

	t.wg.Add(1)
	go t.writeLoop()

	return t
}

// Record enqueues a trace for async storage. Never blocks — if the channel
// is full the trace is dropped and a warning is logged. This ensures the
// LLM call latency is never affected by storage pressure.
func (t *Tracer) Record(trace *models.Trace) {
	select {
	case t.writeCh <- trace:
		// trace enqueued successfully
	default:
		// channel full — drop trace, log warning
		// this should be rare — indicates storage is falling behind
		slog.Warn("tracer: write channel full, dropping trace",
			"trace_id", trace.TraceID,
			"provider", trace.Provider,
		)
	}
}

// writeLoop runs in a background goroutine. It drains the write channel
// and persists each trace to storage. On shutdown it drains all remaining
// traces before returning.
func (t *Tracer) writeLoop() {
	defer t.wg.Done()

	for {
		select {
		case trace := <-t.writeCh:
			if err := t.storage.SaveTrace(context.Background(), trace); err != nil {
				slog.Error("tracer: failed to save trace",
					"trace_id", trace.TraceID,
					"error", err,
				)
			}

		case <-t.done:
			// drain remaining traces in the channel before exiting
			// so no data is lost on graceful shutdown
			for {
				select {
				case trace := <-t.writeCh:
					if err := t.storage.SaveTrace(context.Background(), trace); err != nil {
						slog.Error("tracer: failed to save trace during shutdown",
							"trace_id", trace.TraceID,
							"error", err,
						)
					}
				default:
					// channel is empty — safe to exit
					return
				}
			}
		}
	}
}

// Close signals the background goroutine to stop, waits for all pending
// traces to be flushed, then closes the storage connection.
// Always call Close() when done — typically via defer.
func (t *Tracer) Close() error {
	// signal writeLoop to stop after draining
	close(t.done)

	// wait for writeLoop to finish draining the channel
	t.wg.Wait()

	// close the storage connection
	if err := t.storage.Close(); err != nil {
		return fmt.Errorf("tracer: closing storage: %w", err)
	}

	return nil
}

// --- Trace query methods ---
// These are used by the dashboard and CLI to read trace data.
// Pagination (limit/offset) lives here — storage returns all matching
// traces and Tracer slices them.

// GetTrace retrieves a single trace by ID.
func (t *Tracer) GetTrace(ctx context.Context, traceID string) (*models.Trace, error) {
	trace, err := t.storage.GetTrace(ctx, traceID)
	if err != nil {
		return nil, fmt.Errorf("tracer: getting trace %s: %w", traceID, err)
	}
	return trace, nil
}

// GetTraces retrieves traces for a project with filtering and pagination.
// Returns the page of traces, the total count of matching traces, and any error.
// Total count is used by the dashboard to render pagination controls.
func (t *Tracer) GetTraces(
	ctx context.Context,
	projectID string,
	filter models.TraceFilter,
	limit, offset int64,
) ([]*models.Trace, int64, error) {

	// fetch all matching traces from storage
	all, err := t.storage.GetTraces(ctx, projectID, filter, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("tracer: getting traces: %w", err)
	}

	total := int64(len(all))

	// apply offset — if offset is beyond the total, return empty
	if offset >= total {
		return []*models.Trace{}, total, nil
	}

	// apply limit — slice from offset to offset+limit
	end := offset + limit
	if end > total {
		end = total
	}

	return all[offset:end], total, nil
}

// --- Project methods ---

// SaveProject persists a new project.
func (t *Tracer) SaveProject(ctx context.Context, project *models.Project) error {
	if err := t.storage.SaveProject(ctx, project); err != nil {
		return fmt.Errorf("tracer: saving project %s: %w", project.ProjectID, err)
	}
	return nil
}

// GetProject retrieves a single project by ID.
func (t *Tracer) GetProject(ctx context.Context, projectID string) (*models.Project, error) {
	project, err := t.storage.GetProject(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("tracer: getting project %s: %w", projectID, err)
	}
	return project, nil
}

// GetProjects retrieves all projects.
func (t *Tracer) GetProjects(ctx context.Context) ([]*models.Project, error) {
	projects, err := t.storage.GetProjects(ctx)
	if err != nil {
		return nil, fmt.Errorf("tracer: getting projects: %w", err)
	}
	return projects, nil
}

// --- Stats ---

// GetStats returns aggregated statistics for a project.
// Used by the dashboard overview page.
func (t *Tracer) GetStats(ctx context.Context, projectID string) (*models.Stats, error) {
	stats, err := t.storage.GetStats(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("tracer: getting stats for project %s: %w", projectID, err)
	}
	return stats, nil
}
