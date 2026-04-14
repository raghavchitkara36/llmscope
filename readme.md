# llmscope

**LLM observability for Go developers. Zero external services. 3 lines of code.**

Every team building LLM-powered apps in Python or TypeScript has access to mature observability tools — Langfuse, LangSmith, Helicone. Go developers have nothing equivalent.

llmscope is a zero-dependency, embeddable Go package that gives you complete LLM observability by wrapping your existing provider client. Every API call is automatically captured — prompts, responses, latency, token usage, cost, and errors — stored locally in SQLite and viewable in a lightweight browser dashboard.

No external services. No API keys. No infrastructure. Just import and go.

---

## Features

- **Drop-in instrumentation** — wrap your existing client in one line, nothing else changes
- **Automatic trace capture** — prompts, responses, latency, tokens, cost, errors
- **Local SQLite storage** — data never leaves your machine
- **Async writes** — zero added latency on your LLM calls
- **Browser dashboard** — view, filter, and search traces locally *(coming soon)*
- **Project grouping** — organise traces by feature, service, or use case
- **Session tracking** — group related calls from one conversation

---

## Provider Support

| Provider | Status | Notes |
|---|---|---|
| OpenAI | ✅ Full | Chat completions |
| Ollama | ✅ Full | All local models |
| Anthropic | 🔜 Coming soon | — |
| Gemini | 🔜 Coming soon | — |

---

## Installation

```bash
go get github.com/raghavchitkara36/llmscope
```

Requires Go 1.21+

---

## Quick Start

### With Ollama

```go
package main

import (
    "context"
    "fmt"
    "log"

    ollama "github.com/ollama/ollama/api"
    "github.com/raghavchitkara36/llmscope"
)

func main() {
    ctx := context.Background()

    // 1 — init llmscope
    scope, err := llmscope.New(llmscope.Config{
        StoragePath: "./traces",
    })
    if err != nil {
        log.Fatal(err)
    }
    defer scope.Close()

    // 2 — wrap your existing Ollama client
    ollamaClient, _ := ollama.ClientFromEnvironment()
    client := scope.WrapOllama(ollamaClient, "my-project")

    // 3 — use exactly as before — everything is traced automatically
    err = client.Chat(ctx, &ollama.ChatRequest{
        Model: "llama3",
        Messages: []ollama.Message{
            {Role: "user", Content: "Why is Go great for backend services?"},
        },
    }, func(resp ollama.ChatResponse) error {
        fmt.Print(resp.Message.Content)
        return nil
    })
}
```

### With OpenAI

```go
// 1 — init llmscope
scope, err := llmscope.New(llmscope.Config{
    StoragePath: "./traces",
})
defer scope.Close()

// 2 — wrap your existing OpenAI client
client := scope.WrapOpenAI(openai.NewClient(), "my-project")

// 3 — use exactly as before
resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionNewParams{
    Model: openai.ChatModelGPT4o,
    Messages: []openai.ChatCompletionMessageParamUnion{
        openai.UserMessage("Explain distributed tracing in one paragraph."),
    },
})
```

---

## What Gets Captured

Every LLM call produces one trace record:

| Field | Description |
|---|---|
| `trace_id` | Auto-generated UUID |
| `project_id` | Your project grouping |
| `session_id` | Optional — group related calls |
| `provider` | `openai` / `ollama` |
| `model` | Exact model name (e.g. `gpt-4o`, `llama3`) |
| `prompt` | Full message history with roles |
| `response` | Complete LLM response text |
| `request_timestamp` | When the call was made (unix ms) |
| `response_timestamp` | When the response arrived (unix ms) |
| `input_tokens` | Tokens consumed in prompt |
| `output_tokens` | Tokens generated in response |
| `cost_usd` | Calculated cost in USD *(best-effort)* |
| `tags` | Your custom metadata (e.g. `env=prod`) |
| `error` | Error code and message if the call failed |

---

## Configuration

```go
scope, err := llmscope.New(llmscope.Config{
    StoragePath: "./traces", // where SQLite db is created. Default: ./llmscope
})
```

---

## Project Grouping

Use projects to separate traces by feature, service, or environment:

```go
// traces for your chatbot feature
chatClient := scope.WrapOpenAI(openaiClient, "chatbot")

// traces for your summarizer feature
summaryClient := scope.WrapOpenAI(openaiClient, "summarizer")
```

---

## Session Tracking

Group multiple related calls from one user conversation:

```go
trace := models.NewTrace(projectID, "openai", "gpt-4o")
trace.SessionID = "user-session-abc123"
```

---

## Filtering Traces

```go
filter := models.NewTraceFilter().
    WithProvider("openai").
    WithModel("gpt-4o").
    WithDateRange(from, to).
    WithHasError(true).
    Build()
```

---

## Architecture

```
Your Code
    │
    ▼
Provider Wrapper (Decorator + Adapter)
    │  intercepts call, extracts data, builds Trace
    ▼
Tracer (async)
    │  buffered channel — never blocks your LLM call
    ▼
Storage (Strategy)
    │  SQLite (default) or JSON flat files
    ▼
./traces/llmscope.db
```

---

## Non-Goals (v1)

- No cloud backend or SaaS
- No data ever leaves your machine
- No streaming support *(coming in v2)*
- No real-time alerting

---

## Roadmap

- [ ] Browser dashboard
- [ ] Anthropic provider
- [ ] Gemini provider
- [ ] JSON flat file storage backend
- [ ] Streaming support
- [ ] Cost explorer
- [ ] Prompt versioning
- [ ] OpenTelemetry export

---

## Contributing

Contributions welcome. Please open an issue first to discuss what you'd like to change.

---

## License

MIT — see [LICENSE](LICENSE)

---

*llmscope — Open Source | github.com/raghavchitkara36/llmscope*