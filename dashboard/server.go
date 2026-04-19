package dashboard

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/raghavchitkara36/llmscope/models"
	"github.com/raghavchitkara36/llmscope/storage"
	"github.com/raghavchitkara36/llmscope/tracer"
)

//go:embed static
var staticFiles embed.FS

// Dashboard serves the llmscope browser UI and JSON API.
type Dashboard struct {
	tracer *tracer.Tracer
}

// Start creates the dashboard and starts the HTTP server on the given port.
// Runs in the calling goroutine — call via `go dashboard.Start(...)` to avoid blocking.
func Start(t *tracer.Tracer, port int) {
	d := &Dashboard{tracer: t}

	mux := http.NewServeMux()

	// serve embedded static files at root
	// strips the "static/" prefix so index.html is served at /
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		slog.Error("dashboard: failed to load static files", "error", err)
		return
	}
	mux.Handle("/", http.FileServer(http.FS(staticFS)))

	// JSON API routes — Go 1.22+ method+path pattern
	mux.HandleFunc("GET /api/projects", d.handleProjects)
	mux.HandleFunc("GET /api/stats", d.handleStats)
	mux.HandleFunc("GET /api/traces", d.handleTraces)
	mux.HandleFunc("GET /api/traces/{id}", d.handleTrace)

	addr := fmt.Sprintf(":%d", port)
	slog.Info("llmscope dashboard started",
		"url", fmt.Sprintf("http://localhost:%d", port),
	)

	if err := http.ListenAndServe(addr, mux); err != nil {
		slog.Error("dashboard: server stopped", "error", err)
	}
}

// --- API handlers ---

// handleProjects returns all projects as JSON.
func (d *Dashboard) handleProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := d.tracer.GetProjects(r.Context())
	if err != nil {
		writeError(w, "dashboard: getting projects", err, http.StatusInternalServerError)
		return
	}

	writeJSON(w, projects)
}

// handleStats returns aggregated stats for a project.
// Query params: project_id (required)
func (d *Dashboard) handleStats(w http.ResponseWriter, r *http.Request) {
	projectID := r.URL.Query().Get("project_id")
	if projectID == "" {
		http.Error(w, "project_id is required", http.StatusBadRequest)
		return
	}

	stats, err := d.tracer.GetStats(r.Context(), projectID)
	if err != nil {
		writeError(w, "dashboard: getting stats", err, http.StatusInternalServerError)
		return
	}

	writeJSON(w, stats)
}

// handleTraces returns a filtered list of traces for a project.
// Query params: project_id (required), provider, model, session_id,
//
//	start_date, end_date, has_error
func (d *Dashboard) handleTraces(w http.ResponseWriter, r *http.Request) {
	projectID := r.URL.Query().Get("project_id")
	if projectID == "" {
		http.Error(w, "project_id is required", http.StatusBadRequest)
		return
	}

	// build filter from query params using our builder pattern
	builder := models.NewTraceFilter()

	if provider := r.URL.Query().Get("provider"); provider != "" {
		builder.WithProvider(provider)
	}
	if model := r.URL.Query().Get("model"); model != "" {
		builder.WithModel(model)
	}
	if sessionID := r.URL.Query().Get("session_id"); sessionID != "" {
		builder.WithSession(sessionID)
	}
	if hasError := r.URL.Query().Get("has_error"); hasError == "true" {
		builder.WithHasError(true)
	} else if hasError == "false" {
		builder.WithHasError(false)
	}

	filter := builder.Build()

	// parse pagination params with sensible defaults
	limit := parseIntParam(r, "limit", 20)
	offset := parseIntParam(r, "offset", 0)

	traces, total, err := d.tracer.GetTraces(r.Context(), projectID, filter, limit, offset)
	if err != nil {
		writeError(w, "dashboard: getting traces", err, http.StatusInternalServerError)
		return
	}

	// return empty array instead of null when no traces found
	if traces == nil {
		traces = []*models.Trace{}
	}

	writeJSON(w, map[string]any{
		"traces": traces,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// handleTrace returns a single trace by ID.
// Path param: id
func (d *Dashboard) handleTrace(w http.ResponseWriter, r *http.Request) {
	// extract {id} from path — Go 1.22+
	traceID := r.PathValue("id")
	if traceID == "" {
		http.Error(w, "trace id is required", http.StatusBadRequest)
		return
	}

	trace, err := d.tracer.GetTrace(r.Context(), traceID)
	if err != nil {
		// check if it's a not-found error
		if errors.Is(err, storage.ErrTraceNotFound) {
			http.Error(w, "trace not found", http.StatusNotFound)
			return
		}
		writeError(w, "dashboard: getting trace", err, http.StatusInternalServerError)
		return
	}

	writeJSON(w, trace)
}

// --- helpers ---

// writeJSON writes v as JSON to w with correct content-type header.
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("dashboard: encoding json response", "error", err)
	}
}

// writeError logs the error and writes an HTTP error response.
// Follows single handling rule — error is logged here, not returned.
func writeError(w http.ResponseWriter, context string, err error, status int) {
	slog.Error(context, "error", err)
	http.Error(w, http.StatusText(status), status)
}

// parseIntParam reads an int64 query param with a fallback default
func parseIntParam(r *http.Request, key string, defaultVal int64) int64 {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return defaultVal
	}
	val, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return defaultVal
	}
	return val
}
