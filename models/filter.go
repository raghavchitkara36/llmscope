package models

// TraceFilter holds all filter criteria for querying traces
type TraceFilter struct {
    ProjectID  string
    SessionID  string
    Provider   string
    Model      string
    StartDate  int64
    EndDate    int64
    HasError   *bool             // pointer — nil means "don't filter", false/true means filter
    Tags       map[string]string
}

// TraceFilterBuilder builds a TraceFilter step by step
type TraceFilterBuilder struct {
    filter TraceFilter
}

// NewTraceFilter starts a new filter builder
func NewTraceFilter() *TraceFilterBuilder {
    return &TraceFilterBuilder{}
}

// WithProject filters traces by project
func (b *TraceFilterBuilder) WithProject(projectID string) *TraceFilterBuilder {
    b.filter.ProjectID = projectID
    return b  // return builder for chaining
}

// WithSession filters traces by session
func (b *TraceFilterBuilder) WithSession(sessionID string) *TraceFilterBuilder {
    b.filter.SessionID = sessionID
    return b
}

// WithProvider filters traces by provider (e.g. "openai", "ollama")
func (b *TraceFilterBuilder) WithProvider(provider string) *TraceFilterBuilder {
    b.filter.Provider = provider
    return b
}

// WithModel filters traces by model (e.g. "gpt-4o")
func (b *TraceFilterBuilder) WithModel(model string) *TraceFilterBuilder {
    b.filter.Model = model
    return b
}

// WithDateRange filters traces between two unix millisecond timestamps
func (b *TraceFilterBuilder) WithDateRange(start, end int64) *TraceFilterBuilder {
    b.filter.StartDate = start
    b.filter.EndDate = end
    return b
}

// WithHasError filters traces that have or don't have errors
func (b *TraceFilterBuilder) WithHasError(hasError bool) *TraceFilterBuilder {
    b.filter.HasError = &hasError
    return b
}

// WithTags filters traces by tag key-value pairs
func (b *TraceFilterBuilder) WithTags(tags map[string]string) *TraceFilterBuilder {
    b.filter.Tags = tags
    return b
}

// Build returns the final TraceFilter
func (b *TraceFilterBuilder) Build() TraceFilter {
    return b.filter
}