// models/stats.go
package models

// Stats holds summary statistics for a project
type Stats struct {
	TotalTraces  int64   `json:"total_traces"`
	TotalCost    float64 `json:"total_cost"`
	AvgLatencyMS float64 `json:"avg_latency_ms"`
	ErrorCount   int64   `json:"error_count"`
	TotalTokens  int64   `json:"total_tokens"`
	StartTime    int64   `json:"start_time"`
	EndTime      int64   `json:"end_time"`
}
