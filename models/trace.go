package models

import "time"

// Message represents a single turn in a prompt conversation
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// TraceError captures error details when an LLM API call fails
type TraceError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Trace represents a single captured LLM API call
type Trace struct {
	TraceID           string            `json:"trace_id"             db:"trace_id"`
	ProjectID         string            `json:"project_id"           db:"project_id"`
	SessionID         string            `json:"session_id,omitempty" db:"session_id"`
	Provider          string            `json:"provider"             db:"provider"`
	Model             string            `json:"model"                db:"model"`
	Prompt            []Message         `json:"prompt"               db:"prompt"`
	Response          string            `json:"response"             db:"response"`
	RequestTimestamp  int64             `json:"request_timestamp"    db:"request_timestamp"`
	ResponseTimestamp int64             `json:"response_timestamp"   db:"response_timestamp"`
	InputTokens       int64             `json:"input_tokens"         db:"input_tokens"`
	OutputTokens      int64             `json:"output_tokens"        db:"output_tokens"`
	CostUSD           float64           `json:"cost_usd"             db:"cost_usd"`
	Tags              map[string]string `json:"tags,omitempty"        db:"tags"`
	Error             *TraceError       `json:"error,omitempty"      db:"error"`
}

func NewTrace(projectID, provider, model string) *Trace {
	return &Trace{
		TraceID:          generateID(),
		ProjectID:        projectID,
		Provider:         provider,
		Model:            model,
		RequestTimestamp: time.Now().UnixMilli(),
	}
}
