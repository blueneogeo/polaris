# Configuration

## Server startup configuration

Configured via environment variables using varlock-encrypted secrets. Schema is defined in `.env.schema`.

| Variable | Required | Default | Purpose |
|---|---|---|---|
| `PORT` | No | `8777` | HTTP server port |
| `DATA_DIR` | No | `~/.polaris` | Directory for model wallet, rules, memory |

The server starts without any upstream model configured. Models are managed at runtime via the [Admin API](admin-api.md).

## Model wallet

Runtime-managed collection of AI models. Stored at `$DATA_DIR/models.toml`. Survives server restarts.

### Format

```toml
[models.openai]
url = "https://api.openai.com"
api_key = "sk-..."

[models.deepseek]
url = "https://api.deepseek.com"
api_key = "sk-..."

worker = "openai"
brain = "deepseek"
```

- `worker` — the model Polaris proxies actual work to (optional, no proxy without it)
- `brain` — the model Polaris uses for its own reasoning (optional, no judging without it)
- Each model in `[models.<name>]` has a `url` and `api_key`

Models are added/removed via the `/polaris-models-add` and `/polaris-models-remove` commands, which call the [Admin API](admin-api.md).

## Rules

Stored as TOML files with a scope hierarchy:

| Scope | Location | Applies to |
|---|---|---|
| Global | `$DATA_DIR/rules.toml` | Every project, every session |
| Project | `.polaris/rules.toml` | Current project only |
| Session | In-memory | Current session only |

### Format (planned)

```toml
[[rules]]
id = 1
description = "never use `any` in TypeScript"
scope = "global"
created = "2026-06-30T12:00:00Z"

[[rules]]
id = 2
description = "functions must not exceed 30 lines"
scope = "project"
created = "2026-06-30T12:01:00Z"
```

## Memory

Similar to rules, but for learned preferences rather than hard enforcement. Stored as TOML files:

| Scope | Location |
|---|---|
| Global | `$DATA_DIR/memory.toml` |
| Project | `.polaris/memory.toml` |
| Session | In-memory |

### Format (planned)

```toml
[[memories]]
id = 1
content = "prefers functional patterns over classes"
scope = "project"
source = "auto"
created = "2026-06-30T12:00:00Z"
```

## POLARIS.md

Instructions for Polaris itself — tells Polaris how to act as a co-programmer. Follows a hierarchy:

| Level | Location | Purpose |
|---|---|---|
| Built-in | Embedded in binary | Default Polaris behavior (core identity) |
| Global | `$DATA_DIR/POLARIS.md` | User's personal overrides |
| Project | `POLARIS.md` (project root) | Per-project instructions |

### Example POLARIS.md (user-created)

```markdown
When evaluating code, prioritize readability over cleverness.
Flag any functions that lack error handling.
Prefer composition over inheritance.
```

This is loaded into Polaris's own system prompt — it shapes how Polaris judges output.

## OpenCode auto-import (future)

When running alongside OpenCode, Polaris can detect OpenCode's configuration and import all configured providers automatically. The user doesn't need to re-enter API keys in the Polaris wallet.

Activation: `POST /admin/models/import-opencode`

Polaris reads:
- `opencode.json` for provider definitions and model names
- `~/.local/share/opencode/auth.json` for API keys

All providers are imported into the wallet automatically. The user then selects worker and brain models from the imported set.
