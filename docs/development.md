# Development

## Prerequisites

Install with Homebrew on macOS:

```bash
brew install go golangci-lint dmno-dev/tap/varlock fswatch
```

## Project structure

```
polaris/
├── cmd/server/main.go              # Entry point
├── internal/
│   ├── server/server.go            # Server bootstrap, routing, graceful shutdown
│   ├── config/config.go            # Environment variable loading
│   ├── proxy/
│   │   ├── handler.go              # Main proxy handler (POST /v1/chat/completions)
│   │   ├── handler_test.go         # Proxy integration tests
│   │   ├── block.go                # BlockDetector — SSE stream to logical blocks
│   │   ├── writer.go               # BlockWriter — serialize blocks back to SSE
│   │   ├── types.go                # Block, BlockType, Decision types
│   │   ├── model.go                # Model interface + EchoModel
│   │   ├── validator.go            # Validator interface + PassValidator
│   │   └── enhancer.go             # PromptEnhancer interface + NoopEnhancer
│   ├── models/                     # Model wallet (planned)
│   ├── admin/                      # Admin API handlers (planned)
│   └── middleware/
│       ├── logger.go               # Slog structured request logging
│       └── recovery.go             # Panic recovery → 500 JSON
├── pkg/apierror/                   # Standardized API error types + JSON helpers
├── docs/                           # Documentation
├── build.sh                        # Dev tooling script
├── build-env.sh                    # Environment loading with varlock
├── .env.schema                     # Environment variable schema
├── .golangci.yml                   # Linter configuration
├── go.mod / go.sum
└── README.md
```

## build.sh commands

| Command | What it does |
|---|---|
| `./build.sh setup` | Interactive setup wizard — configures models, builds |
| `./build.sh build` | Lint + compile to `bin/polaris` |
| `./build.sh start` | Build + start server in background (requires env configured) |
| `./build.sh stop` | Stop background server |
| `./build.sh watch` | Start + auto-rebuild on `.go` file changes |
| `./build.sh test [N]` | Run all Go tests (optional timeout in seconds) |
| `./build.sh lint` | Run `golangci-lint` |
| `./build.sh doctor` | Check all prerequisites and env configuration |
| `./build.sh clean` | Remove `bin/` and `.watch/` |

### Environment setup

The `setup` command walks through configuring the server:

1. Checks prerequisites (Go, golangci-lint, varlock).
2. Bootstraps `.env.local` from `.env.schema`.
3. Guides through adding a model and setting it as worker.
4. Builds the binary.

### Watch mode

`./build.sh watch` uses `fswatch` to monitor `.go` files. On change, it re-runs lint, rebuilds, and restarts the server. Useful during development.

### Doctor

`./build.sh doctor` validates:
- All build tools are installed (go, golangci-lint, fswatch, varlock)
- `.env.schema` is valid and all required env vars are set

## Running tests

```bash
# All tests
./build.sh test

# With custom timeout
./build.sh test 60

# Direct go test
go test ./... -v -count=1

# Single package
go test ./internal/proxy/ -v -count=1
```

### Test structure

| Package | Test file | Type | What's tested |
|---|---|---|---|
| `pkg/apierror` | `apierror_test.go` | Unit | Error constructors, JSON writing |
| `internal/config` | `config_test.go` | Unit | Env var loading, defaults, required vars |
| `internal/middleware` | `middleware_test.go` | Unit | Logger (200/400/500 levels), recovery (panic→500) |
| `internal/proxy` | `handler_test.go` | Integration | Full proxy loop with mock upstream: streaming SSE pass-through, non-streaming pass-through, block type detection, upstream errors, header forwarding, auth forwarding, large responses, EchoModel |
| `internal/server` | `server_test.go` | Integration | Health endpoint, route registration, 404 |

Tests run against in-memory mock upstream servers — no external API calls.

## Dependencies

| Package | Purpose |
|---|---|
| `github.com/go-chi/chi/v5` | HTTP router |
| `github.com/openai/openai-go/v3` | Typed chat completion types + SSE stream parser |

No ORM, no framework. The proxy uses Go's standard library HTTP client for upstream calls.

## Upstream proxy request format

Polaris forwards the raw JSON body to the upstream. The `model` field in the request maps to a model name in the Polaris wallet, which provides the actual URL and API key.

```json
{
  "model": "claude-sonnet",
  "messages": [{"role": "user", "content": "Hello"}],
  "stream": true
}
```

If the model name is not found in the wallet, Polaris returns an error listing the available models.
