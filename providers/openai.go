package providers

import (
	"context"
	"fmt"
	"time"

	"github.com/openai/openai-go/v3"
	"github.com/raghavchitkara36/llmscope/models"
	"github.com/raghavchitkara36/llmscope/tracer"
)

// OpenAIClient wraps the official OpenAI client with tracing.
// It embeds *openai.Client so all original methods pass through
// transparently — only CreateChatCompletion is overridden to add tracing.
type OpenAIClient struct {
	*openai.Client // embedded — all original methods available automatically
	tracer         *tracer.Tracer
	projectID      string
}

// WrapOpenAI wraps the user's existing OpenAI client with llmscope tracing.
// The user passes their already-configured client — we never touch API keys
// or client configuration.
//
// Usage:
//
//	client := openai.NewClient()
//	wrapped := providers.WrapOpenAI(client, tracer, "my-project-id")
//	resp, err := wrapped.CreateChatCompletion(ctx, params)
func WrapOpenAI(client *openai.Client, t *tracer.Tracer, projectID string) *OpenAIClient {
	return &OpenAIClient{
		Client:    client,
		tracer:    t,
		projectID: projectID,
	}
}

// CreateChatCompletion intercepts the OpenAI chat completion call,
// traces it, and returns the original response unchanged.
// The user calls this exactly like the original client — nothing changes.
func (c *OpenAIClient) CreateChatCompletion(
	ctx context.Context,
	params openai.ChatCompletionNewParams,
) (*openai.ChatCompletion, error) {

	// 1. build trace with request data — NewTrace sets RequestTimestamp
	trace := models.NewTrace(c.projectID, "openai", string(params.Model))
	trace.Prompt = extractMessages(params.Messages)

	// 2. record exact timestamp just before the API call
	trace.RequestTimestamp = time.Now().UnixMilli()

	// 3. make the REAL API call via the embedded client
	resp, err := c.Client.Chat.Completions.New(ctx, params)

	// 4. record response timestamp immediately after
	trace.ResponseTimestamp = time.Now().UnixMilli()

	// 5. handle error case — failed calls are still traced
	if err != nil {
		trace.Error = &models.TraceError{
			Code:    extractErrorCode(err),
			Message: err.Error(),
		}
		// record the failed trace async — never block on error
		c.tracer.Record(trace)
		return nil, err
	}

	// 6. extract response data
	if len(resp.Choices) > 0 {
		trace.Response = resp.Choices[0].Message.Content
	}
	trace.InputTokens = resp.Usage.PromptTokens
	trace.OutputTokens = resp.Usage.CompletionTokens

	// 7. enqueue trace for async storage — never blocks the caller
	c.tracer.Record(trace)

	return resp, nil
}

// --- helpers ---

// extractMessages converts openai SDK message params to our models.Message slice.
// We extract role and content from each message in the conversation.
func extractMessages(params []openai.ChatCompletionMessageParamUnion) []models.Message {
	messages := make([]models.Message, 0, len(params))

	for _, p := range params {
		var role, content string

		switch {
		case p.OfSystem != nil:
			role = "system"
			if p.OfSystem.Content.OfString.Valid() {
				content = p.OfSystem.Content.OfString.Value
			}
		case p.OfUser != nil:
			role = "user"
			if p.OfUser.Content.OfString.Valid() {
				content = p.OfUser.Content.OfString.Value
			}
		case p.OfAssistant != nil:
			role = "assistant"
			if p.OfAssistant.Content.OfString.Valid() {
				content = p.OfAssistant.Content.OfString.Value
			}
		default:
			continue // skip unknown message types
		}

		messages = append(messages, models.Message{
			Role:    role,
			Content: content,
		})
	}

	return messages
}

// extractErrorCode extracts a short error code from an OpenAI API error.
// Falls back to "unknown" if the error is not an OpenAI API error.
func extractErrorCode(err error) string {
	var apiErr *openai.Error
	if apierr, ok := err.(*openai.Error); ok {
		apiErr = apierr
		return fmt.Sprintf("%d", apiErr.StatusCode)
	}
	return "unknown"
}
