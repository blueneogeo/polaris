# Polaris

<p align="center">
  <img src="resources/polaris.jpg" alt="Polaris — the North Star" width="600">
</p>

Polaris is a co-programmer proxy that sits between you and an AI model. It steers the model toward correct, aligned responses by automating the manual correction loop you already perform: watching the output, hitting escape when it goes wrong, and telling the model to try again.

Polaris acts as an OpenAI-compatible server. You connect your harness (OpenCode, or any OpenAI-compatible client) to Polaris, and Polaris proxies to one or more worker models. It uses its own model(s) to reason about quality, rules, and alignment.

## Current status (v0.1)

- **Transparent streaming proxy** — forwards chat completions to an upstream model, passes SSE chunks through via a typed block pipeline (text, tool_call, thinking).
- **Extensible architecture** — `Validator`, `PromptEnhancer`, and `Model` interfaces are wired but act as no-ops. Ready for rules, memory, and LLM judging.
- **Build tooling** — `./build.sh` with varlock-encrypted secrets (mirrors Turn project patterns).

Not yet implemented: rules engine, LLM judging, memory, `@polaris` communication, OpenCode plugin.

## Quick start

**Prerequisites:** Go, golangci-lint, varlock, fswatch (for watch mode).

```bash
# Install prerequisites
brew install go golangci-lint dmno-dev/tap/varlock fswatch

# Configure upstream provider (URL + API key)
./build.sh setup

# Start the proxy
./build.sh start
```

## build.sh commands

| Command | Description |
|---|---|
| `./build.sh setup` | Interactive setup wizard — configures upstream provider URL and API key |
| `./build.sh upstream-setup` | Just configure the upstream provider |
| `./build.sh build` | Lint + compile Go binary to `bin/polaris` |
| `./build.sh start` | Build + start server in background (requires env configured) |
| `./build.sh stop` | Stop background server |
| `./build.sh watch` | Start + auto-rebuild on `.go` file changes (requires fswatch) |
| `./build.sh test` | Run all tests (`go test ./...`) |
| `./build.sh lint` | Run `golangci-lint` |
| `./build.sh doctor` | Check all prerequisites and env configuration |
| `./build.sh clean` | Remove `bin/` and `.watch/` |

### Environment variables

Configured via `.env.local` using varlock. Schema defined in `.env.schema`:

| Variable | Required | Default | Description |
|---|---|---|---|
| `PORT` | No | `8777` | HTTP server port |
| `UPSTREAM_URL` | Yes | — | Upstream provider base URL (e.g. `https://api.openai.com`) |
| `UPSTREAM_API_KEY` | Yes | — | API key for the upstream provider |

### Connecting OpenCode

Add a custom provider to your `opencode.json`:

```json
{
  "provider": {
    "polaris": {
      "npm": "@ai-sdk/openai-compatible",
      "name": "Polaris",
      "options": { "baseURL": "http://localhost:8777/v1" },
      "models": { "gpt-4o": { "name": "GPT-4o (via Polaris)" } }
    }
  }
}
```

Then select **Polaris** as your provider in OpenCode.

---

## What Polaris is

- A **co-programmer**, not an agent harness. You stay in control. The model does the work. Polaris keeps it between the lines.
- A **transparent safety net** that stays out of the way and only intervenes when a rule is violated or a prompt needs enhancement.
- A tool that **learns your preferences** over time and steers models toward what you value.

## What Polaris is not

- Not an auto-running agent or agent orchestrator.
- Not a one-shot harness with subagents, planners, and review loops that burn tokens without your involvement.
- Not a presentation filter that silently rewrites model output (the model must learn, not be patched).

## How it works

### The basic loop

1. You send a prompt to your harness (e.g., OpenCode).
2. The harness sends the request to Polaris (an OpenAI-compatible endpoint).
3. **Polaris optionally enhances the prompt** — fixing text-to-speech mistakes, injecting relevant rules, adding context from memory.
4. Polaris forwards the enhanced prompt to the worker model.
5. The worker model responds.
6. **Polaris evaluates the response** against your rules.
7. If it passes — forwarded to you.
8. If it violates a rule — **blocked, corrective feedback sent to the worker model, retry.** Polaris tells you what happened.

```
You → Harness → Polaris → Worker Model
                   ↑            ↓
              [inspect]   [response]
                   ↓            ↑
              [enhance] ← [retry with feedback]
```

### Early intervention

Polaris inspects streaming output and can cancel early if it detects a rule violation — the same as you pressing escape when you see the model going sideways.

### No output rewriting

Polaris never silently rewrites model output. If the output is wrong, the model must fix it. Polaris is a coach, not a copy editor. This means the model learns within the session and produces better output on the next turn.

### Token cache

Polaris maintains an append-only conversation history with the worker model. Past turns are never modified, preserving the token cache across requests.

### Model tiering

Polaris can use different models for different purposes:
- A cheap model for Polaris's own rule validation and prompt enhancement.
- A stronger model for the actual work.
- Polaris can choose the tier based on task complexity.

## Rules

Rules define what the model must follow. They are **hard enforcement**: Polaris blocks violating output and asks the model to retry.

### Scopes

- `global` — applies everywhere, across all projects.
- `project` — applies to the current project only.
- `session` — applies to the current session only.

### Commands

```
/rules <scope> add <rule description>
/rules <scope> remove <rule number>
/rules <scope> list
```

Examples:

```
/rules global add you may not ask me questions
/rules project add functions must not exceed 30 lines
/rules session add no hardcoded secrets
```

Rules can be broad ("code must be cleanly formed") or specific ("never use `any` in TypeScript"). Clearer rules produce more reliable enforcement.

## Memory

Memory captures what Polaris learns about your preferences over time. Unlike rules (hard enforcement), memory is **soft steering** — it influences prompts and guides responses.

Every time you give feedback to output, or Polaris learns something about what you value or dislike, it records a memory. Memories are auto-generated, reported to you, and can be undone.

### Scopes

- `global` — something about you in general ("dislikes verbose responses").
- `project` — specific to this project ("uses functional patterns").
- `session` — temporary, just for this session.

### Commands

```
/memory <scope> add <preference description>
/memory <scope> remove <memory number>
/memory <scope> list
/forget
```

`/forget` undoes the most recent memory that was added automatically.

## Rules vs. Memory

| | Rules | Memory |
|---|---|---|
| Purpose | Enforcement | Steering |
| Mechanism | Block + retry | Prompt enhancement |
| Violation | Output rejected | Prompt adjusted |
| User intent | "Never do X" | "I prefer Y" |

Example:

- Rule: "Never use `any` in TypeScript" → blocks output, asks model to retry.
- Memory: "Prefers functional patterns over classes" → injected into prompt as context, no blocking.

## Direct communication with Polaris

Prefix a message with `@polaris` to talk to Polaris directly, bypassing the worker model:

```
@polaris what does rule #3 mean by "clean code"?
@polaris why was that last response blocked?
@polaris add a rule: no console.log in production code
```

## User feedback

Polaris reports its actions inline in the chat with a consistent prefix:

```
[Polaris] ENHANCED — Added contextual rule: "use functional patterns"
[Polaris] BLOCKED — Rule #3 violation: function exceeds 30 lines. Retrying...
[Polaris] PASSED (after 2 retries)
```

## Prompt enhancement

Before forwarding your prompt to the worker model, Polaris can:

- Clean up text-to-speech mistakes (double words, transcription errors).
- Inject relevant rules as context for the task at hand.
- Add preferences from memory.
- Clarify vague instructions.

## Auto-planning

Polaris can insert a planning phase when a task benefits from it. It works with the model to create a plan, verifies it against project rules and vision, and only then proceeds with implementation. Polaris self-determines whether planning is necessary based on task complexity.

## Clarification

If your instructions are unclear, Polaris can ask you questions — preferably using the question tool — rather than letting the model guess and produce bad output.

## Project structure

Polaris looks for configuration in these locations:

```
.polaris/
  rules.toml    — project-level rules
  memory.toml   — project-level memory
POLARIS.md      — custom Polaris instructions at project root
```

The OpenCode system directory and user home directory each get their own `.polaris/` directory for system-wide and global scope configurations.

## OpenCode integration

Polaris runs as a local OpenAI-compatible server. Connect OpenCode by pointing its base URL to Polaris. An OpenCode extension can host Polaris automatically, register slash commands, and handle `@polaris` message routing.
