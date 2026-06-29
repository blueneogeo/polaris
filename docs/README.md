# Polaris Documentation

## What is Polaris?

Polaris is a co-programmer proxy — an OpenAI-compatible server that sits between you and an AI model. It steers the model toward correct, aligned responses by enforcing your rules and learning your preferences over time.

You connect your harness (OpenCode, or any OpenAI-compatible client) to Polaris, and Polaris proxies to one or more worker models. It uses its own AI models to judge output, enhance prompts, and enforce rules.

Polaris is not an agent harness. It does not run subagents, planners, or auto-loops. It is a safety net — it stays out of the way and only intervenes when a rule is violated or a prompt needs enhancement.

## How it works (high-level)

```
You → OpenCode → Polaris → Worker Model
                    ↑            ↓
              [enhance]     [response]
                    ↑            ↓
              [inspect] ← [validate]
                    ↓
              [PASS: forward] or [VIOLATION: feedback → retry]
```

1. Your prompt arrives at Polaris.
2. Polaris can **enhance** it — fix text-to-speech mistakes, inject relevant rules, add context from memory.
3. The enhanced prompt is forwarded to the worker model.
4. The worker model's response is streamed through Polaris's **block pipeline** (text blocks, tool calls, thinking blocks).
5. Each block is **validated** against your rules.
6. If it passes — streamed to you immediately.
7. If it violates a rule — blocked, corrective feedback is sent to the worker model, and it retries. Polaris tells you what happened.

The whole system automates the correction loop you already perform manually: watching the output, hitting escape when it goes wrong, and telling the model to try again.

## Current implementation status

**v0.1:** Transparent streaming proxy. Passes chat completions through with a typed block pipeline. All extension points (validation, enhancement, model management) are wired as interfaces but act as no-ops. No rules engine or LLM judging yet.

## Document index

| Document | Covers |
|---|---|
| [Architecture](architecture.md) | Server structure, proxy pipeline, block system, interfaces |
| [Commands](commands.md) | User-facing slash commands (from OpenCode) |
| [Admin API](admin-api.md) | HTTP API for runtime model/rules management |
| [Configuration](configuration.md) | Environment variables, model wallet TOML, rules TOML, POLARIS.md |
| [Rules & Memory](rules-and-memory.md) | How rules and memory work, the distinction, scope system |
| [Development](development.md) | Project structure, build.sh commands, testing |
