# llmobserve

**LLM observability for Go developers. Zero external services. 3 lines of code.**

Every team building LLM-powered apps in Python or TypeScript has access to mature observability tools — Langfuse, LangSmith, Helicone. Go developers have nothing equivalent.

llmobserve is a zero-dependency, embeddable Go package that gives you complete LLM observability by wrapping your existing provider client. Every API call is automatically captured — prompts, responses, latency, token usage, cost, and errors — stored locally in SQLite and viewable in a lightweight browser dashboard.

No external services. No API keys. No infrastructure. Just import and go.

---

## Features

- **Drop-in instrumentation** — wrap your existing client in one line, nothing else changes
- **Automatic trace capture** — prompts, responses, latency, tokens, cost, errors
- **Local SQLite storage** — data never leaves your machine
- **Async writes** — zero added latency on your LLM calls
- **Browser dashboard** — view, filter, and search traces at `http://localhost:7890`
- **Project grouping** — organise traces by feature, service, or use case
- **Session tracking** — group related calls from one conversation
- **Error log** — filtered view of all failed LLM calls with full context

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
go get github.com/llmobserve/llmobserve
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
    "github.com/llmobserve/llmobserve"
)

func main() {
    ctx := context.Background()

    // 1 — init llmobserve with dashboard enabled
    scope, err := llmobserve.New(llmobserve.Config{
        StoragePath: "./traces",
        DevMode:     true, // starts dashboard at http://localhost:7890
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
    if err != nil {
        log.Fatal(err)
    }

    // open http://localhost:7890 to see the trace
}
```

### With OpenAI

```go
// 1 — init llmobserve
scope, err := llmobserve.New(llmobserve.Config{
    StoragePath: "./traces",
    DevMode:     true,
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

## Browser Dashboard

Enable the dashboard by setting `DevMode: true` in your config. It starts automatically at `http://localhost:7890`.

```go
scope, err := llmobserve.New(llmobserve.Config{
    StoragePath: "./traces",
    DevMode:     true, // dashboard starts at http://localhost:7890
})
```

### Dashboard Views

**Overview** — summary stats (total requests, avg latency, total cost, error count, total tokens) and a recent traces list.

**Traces** — paginated, filterable table of all traces. Filter by provider, model, or error status. Click any row to see full detail.

**Trace Detail** — full prompt conversation with roles, complete response, token breakdown, latency, cost, tags, and error info.

**Error Log** — filtered view of all failed LLM calls with error messages and context.

> **Production note:** `DevMode: false` (the default) does not start the dashboard. Your production binary has zero HTTP overhead unless you explicitly enable it.

---

## Configuration

```go
scope, err := llmobserve.New(llmobserve.Config{
    StoragePath: "./traces", // where SQLite db is created. Default: ./llmobserve
    DevMode:     true,       // starts dashboard at http://localhost:7890. Default: false
})
```

| Field | Type | Default | Description |
|---|---|---|---|
| `StoragePath` | `string` | `./llmobserve` | Directory where `llmobserve.db` is created |
| `DevMode` | `bool` | `false` | Starts the browser dashboard automatically |

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
| `cost_usd` | Calculated cost in USD *(best-effort, 0.0 for local models)* |
| `tags` | Your custom metadata (e.g. `env=prod`) |
| `error` | Error code and message if the call failed |

---

## Project Grouping

Projects organise traces by feature, service, or use case. A project is created automatically the first time you wrap a client with a given project ID.

```go
// traces for your chatbot feature
chatClient := scope.WrapOpenAI(openaiClient, "chatbot")

// traces for your summarizer feature
summaryClient := scope.WrapOpenAI(openaiClient, "summarizer")

// traces for a different environment
prodClient := scope.WrapOllama(ollamaClient, "prod-assistant")
```

Each project appears as a separate entry in the dashboard's project selector.

---

## Session Tracking

Group multiple related LLM calls from one user conversation using a session ID:

```go
trace := models.NewTrace(projectID, "openai", "gpt-4o")
trace.SessionID = "user-session-abc123"
```

Filter by session in the dashboard to follow a full conversation thread.

---

## Filtering Traces

Use the builder pattern to construct filters programmatically:

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
    │  SQLite — persistent, queryable, zero external DB
    ▼
./traces/llmobserve.db
    │
    ▼
Dashboard (optional)
    HTTP server at :7890 — embedded static files, no Node.js
```

---

## Non-Goals (v1)

- No cloud backend or SaaS
- No data ever leaves your machine
- No streaming support *(coming in v2)*
- No real-time alerting

---

## Limitations (v0.1.0)

- **Single instance only** — each app instance has its own SQLite database
- **No distributed tracing** — traces from multiple app instances are not aggregated
- **Persistent storage required** — in containers, mount `/path/to/traces` as a volume


## Roadmap

- [x] Core tracer with async writes
- [x] SQLite storage
- [x] OpenAI provider
- [x] Ollama provider
- [x] Browser dashboard (Overview, Traces, Trace Detail, Error Log)
- [ ] Anthropic provider
- [ ] Gemini provider
- [ ] JSON flat file storage backend
- [ ] Streaming support
- [ ] Cost explorer with pricing table
- [ ] Prompt versioning
- [ ] OpenTelemetry export
- [ ] Distributed tracing are planned for v0.2.0+

---

## Contributing

Contributions welcome. Please open an issue first to discuss what you'd like to change.

---

## License

MIT — see [LICENSE](LICENSE)

---

*llmobserve — Open Source | github.com/llmobserve/llmobserve*