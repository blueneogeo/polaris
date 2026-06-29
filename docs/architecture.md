# Architecture

## Overview

Polaris is a Go HTTP server that presents an OpenAI-compatible API (`POST /v1/chat/completions`). It proxies requests to a configurable upstream worker model and inspects responses through a typed block pipeline.

```
cmd/server/main.go  →  internal/server/server.go  →  chi router
                                                       │
                          ┌────────────────────────────┼──────────────────────┐
                          │                            │                      │
                POST /v1/chat/completions        GET /health        /admin/*
                          │
                internal/proxy/handler.go
                          │
              ┌───────────┼───────────┐
              │           │           │
      PromptEnhancer  Validator   Model (Polaris's brain)
         (noop)        (noop)      (EchoModel)
              │           │
              └─────┬─────┘
                    │
              Block pipeline
              ┌─────┴─────┐
        BlockDetector  BlockWriter
```

## Server (`internal/server/server.go`)

Bootstrap and lifecycle:

- `Run(cfg)` wires all dependencies and starts the HTTP server.
- Uses `chi/v5` for routing.
- Middleware: RequestID → Logger (slog structured JSON) → Recovery (panic to 500).
- Graceful shutdown on SIGINT/SIGTERM (15s timeout).
- Health endpoint: `GET /health` → `{"ok":true}`.

## Proxy handler (`internal/proxy/handler.go`)

Handles `POST /v1/chat/completions`. The main request flow:

1. Read raw JSON request body.
2. Parse into `streamRequest` to detect the `stream` flag.
3. Parse into `openai.ChatCompletionNewParams` for typed messages (used when enhancement is active).
4. Call `PromptEnhancer.Enhance()` — modifies request params in place. No-op in v0.
5. Forward raw body to upstream worker model.
6. For non-streaming: read upstream response, unmarshal into `openai.ChatCompletion`, write back.
7. For streaming: route through the block pipeline.

### Streaming block pipeline

```
Upstream SSE stream
  │
  ├─ ssestream.Stream[ChatCompletionChunk]   (from openai/openai-go SDK)
  │
  ├─ BlockDetector.Next() → Block
  │   ├─ classifyChunk() determines block type from delta fields
  │   ├─ Accumulates chunks until type changes
  │   └─ Emits Block{Type, Chunks}
  │
  ├─ Validator.Validate{Text,ToolCall}(block) → Decision{Allowed, Reason, Retry, Feedback}
  │   └─ PassValidator always returns Allowed in v0
  │
  └─ BlockWriter.Write(block)
      ├─ Marshal each chunk → `data: {...}\n\n` + Flush
      ├─ WriteDone() → `data: [DONE]\n\n`
      └─ Future: WritePolarisThinking() during validation gaps
```

## Block system (`internal/proxy/block.go`, `types.go`, `writer.go`)

### Block types

| Type | Detection | Contents |
|---|---|---|
| **Text** | `delta.Content` is non-empty, no tool calls | Accumulated text chunks from stream |
| **ToolCall** | `delta.ToolCalls` is non-empty | Complete tool call arguments |
| **Thinking** | `delta.JSON.ExtraFields["reasoning_content"]` present | Model's internal reasoning (e.g., Anthropic thinking) |

### BlockDetector

Reads an SSE stream from an upstream response using the OpenAI SDK's `ssestream.Stream[ChatCompletionChunk]`. Emits one `Block` at a time at natural boundaries where the chunk type changes.

- Thinking blocks pass through without validation.
- Text and ToolCall blocks go through the `Validator`.

### BlockWriter

Serializes blocks back to SSE format and writes+flushes to the client HTTP response. Future: emits Polaris thinking indicators during validation gaps.

## Interfaces (extension points)

### Model (`internal/proxy/model.go`)

Polaris's own brain — used to call AI models for internal reasoning (judging, enhancing).

```go
type Model interface {
    Complete(ctx context.Context, params ChatCompletionNewParams) (*ChatCompletion, error)
}
```

Current implementation: `EchoModel` — echoes back the user's message. Used for development. Will be replaced with a real model client in v0.2+.

### Validator (`internal/proxy/validator.go`)

Checks blocks against rules.

```go
type Validator interface {
    ValidateText(ctx context.Context, block Block) (Decision, error)
    ValidateToolCall(ctx context.Context, block Block) (Decision, error)
}
```

Current implementation: `PassValidator` — everything passes through. Will become the rules engine backed by an LLM judge in v0.2+.

### PromptEnhancer (`internal/proxy/enhancer.go`)

Modifies the request before it reaches the worker model.

```go
type PromptEnhancer interface {
    Enhance(ctx context.Context, params *ChatCompletionNewParams) error
}
```

Current implementation: `NoopEnhancer` — does nothing. Will clean TTS mistakes, inject rules, and add memory context in v0.2+.

## Model wallet (`internal/models/`)

> Planned, not yet implemented.

A runtime-managed collection of AI models. Stored in `~/.polaris/models.toml`. Models can be:

- **Worker models** — the models Polaris proxies to for actual work.
- **Polaris brain models** — the models Polaris uses for its own reasoning (judging, enhancing).

Managed via the [Admin API](admin-api.md). The proxy handler looks up the worker model from the wallet using the `model` field in the request.

## Admin API (`internal/admin/`)

> Planned, not yet implemented.

HTTP API for runtime configuration. Endpoints for managing models, rules, and memory. See [Admin API](admin-api.md).

## Configuration (`internal/config/config.go`)

Loads server configuration from environment variables:

| Variable | Required | Default | Purpose |
|---|---|---|---|
| `PORT` | No | `8777` | HTTP server port |
| `DATA_DIR` | No | `~/.polaris` | Data directory for model wallet, rules, memory |

No upstream model is configured at startup — models are managed at runtime via the admin API.

## Dependencies

| Package | Purpose |
|---|---|
| `github.com/go-chi/chi/v5` | HTTP router |
| `github.com/openai/openai-go/v3` | Typed chat completion types + SSE stream parser |
