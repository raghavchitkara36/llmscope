package providers

import (
	"context"
	"fmt"
	"time"

	"github.com/ollama/ollama/api"
	"github.com/raghavchitkara36/llmscope/models"
	"github.com/raghavchitkara36/llmscope/tracer"
)

// OllamaClient wraps the official Ollama API client with tracing.
// It embeds *api.Client so all original methods pass through
// transparently — only Chat is overridden to add tracing.
//
// Note: Unlike OpenAI, Ollama runs locally so CostUSD is always 0.0.
// Token counts are taken from the response metrics when available.
type OllamaClient struct {
	*api.Client // embedded — all original methods available automatically
	tracer      *tracer.Tracer
	projectID   string
}

// WrapOllama wraps the user's existing Ollama client with llmscope tracing.
// The user passes their already-configured client — we never touch host
// configuration or client settings.
//
// Usage:
//
//	client, _ := api.ClientFromEnvironment()
//	wrapped := providers.WrapOllama(client, tracer, "my-project-id")
//	err := wrapped.Chat(ctx, req, handler)
func WrapOllama(client *api.Client, t *tracer.Tracer, projectID string) *OllamaClient {
	return &OllamaClient{
		Client:    client,
		tracer:    t,
		projectID: projectID,
	}
}

// Chat intercepts the Ollama chat call, traces it, and calls the original
// response handler unchanged. The user calls this exactly like the original client.
//
// Ollama's Chat method uses a streaming callback — the handler function is called
// for each response chunk. We accumulate the full response before building the trace.
func (c *OllamaClient) Chat(
	ctx context.Context,
	req *api.ChatRequest,
	handler func(api.ChatResponse) error,
) error {

	// 1. build trace with request data
	trace := models.NewTrace(c.projectID, "ollama", req.Model)
	trace.Prompt = extractOllamaMessages(req.Messages)

	// 2. record exact timestamp just before the API call
	trace.RequestTimestamp = time.Now().UnixMilli()

	// 3. accumulate full response across streaming chunks
	var (
		fullResponse string
		finalResp    api.ChatResponse
	)

	// wrap the user's handler to accumulate response content
	wrappedHandler := func(resp api.ChatResponse) error {
		fullResponse += resp.Message.Content
		if resp.Done {
			finalResp = resp
		}
		// call the user's original handler unchanged
		return handler(resp)
	}

	// 4. make the REAL API call via the embedded client
	err := c.Client.Chat(ctx, req, wrappedHandler)

	// 5. record response timestamp immediately after
	trace.ResponseTimestamp = time.Now().UnixMilli()

	// 6. handle error case — failed calls are still traced
	if err != nil {
		trace.Error = &models.TraceError{
			Code:    "unknown",
			Message: err.Error(),
		}
		c.tracer.Record(trace)
		return err
	}

	// 7. populate response data from accumulated chunks
	trace.Response = fullResponse

	// ollama returns token counts in the final response metrics
	if finalResp.Done {
		trace.InputTokens = int64(finalResp.PromptEvalCount)
		trace.OutputTokens = int64(finalResp.EvalCount)
	}

	// ollama is local — cost is always 0.0, no pricing needed
	trace.CostUSD = 0.0

	// 8. enqueue trace for async storage — never blocks the caller
	c.tracer.Record(trace)

	return nil
}

// --- helpers ---

// extractOllamaMessages converts Ollama API messages to our models.Message slice.
// Ollama uses api.Message which has Role and Content fields directly —
// much simpler than the OpenAI union type.
func extractOllamaMessages(messages []api.Message) []models.Message {
	result := make([]models.Message, 0, len(messages))

	for _, m := range messages {
		result = append(result, models.Message{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	return result
}

// ollamaErrorCode extracts a short error code string from an Ollama error.
// Ollama errors are plain Go errors — no structured error type in the SDK.
func ollamaErrorCode(err error) string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf("ollama: %s", err.Error())
}
