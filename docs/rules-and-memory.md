# Rules & Memory

## The distinction

| | Rules | Memory |
|---|---|---|
| **Purpose** | Enforcement | Steering |
| **Mechanism** | Block + retry with feedback | Prompt enhancement |
| **Violation handling** | Output rejected, model asked to retry | Prompt adjusted, no blocking |
| **User intent** | "Never do X" | "I prefer Y" |

### Example

- **Rule:** "Never use `any` in TypeScript" → Polaris blocks output containing `any`, tells the model to retry without it.
- **Memory:** "Prefers functional patterns over classes" → injected into the prompt as context, guides the model without enforcing.

## Rules

Rules are hard constraints. When a rule is violated, Polaris:
1. Blocks the output.
2. Injects corrective feedback into the conversation.
3. Asks the worker model to retry.
4. Reports the action to the user.

Rules are checked per-block during streaming. Text blocks and tool call blocks are validated as they complete. If a violation is detected mid-stream, Polaris cancels the stream and triggers a retry — the same as pressing escape and correcting the model manually.

### Rule scope

| Scope | Applies to | Stored in |
|---|---|---|
| **global** | Every project, every session | `~/.polaris/rules.toml` |
| **project** | Current project only | `.polaris/rules.toml` |
| **session** | Current session only | In-memory (lost on server restart) |

### Rule enforcement

Rules can be deterministic or LLM-judged:

- **Deterministic rules** are checked directly by Polaris without a model call. Examples: "no functions over 30 lines" (count lines), "no `any` in TypeScript" (string match), "don't ask me questions" (regex).
- **LLM-judged rules** require Polaris's brain model to evaluate. Used when semantic understanding is needed: "does this code accomplish the goal?", "is this approach architecturally sound?"

Clear rules produce more reliable enforcement. Vague rules ("code must be clean") rely on LLM judgment and may produce inconsistent results.

## Memory

Memory captures what Polaris learns about your preferences over time. It is soft steering — it influences prompts and guides responses but never blocks.

Memories are auto-generated when:
- You give feedback on model output.
- Polaris detects a pattern in your corrections.
- Polaris observes something important about your values or dislikes.

Each memory is reported to you when it's created. You can undo it with `/polaris-forget`.

### Memory scope

| Scope | Example | Applies to |
|---|---|---|
| **global** | "Dislikes verbose responses" | All your interactions |
| **project** | "Uses functional patterns in this project" | Current project only |
| **session** | "Currently working on auth module" | This session only |

Polaris decides the scope automatically based on what it learned.

## When rules vs. memory apply

During the proxy loop:

1. **Before forwarding** — Polaris enhances the prompt using memory (inject preferences) and rules (inject constraints as context).
2. **After response** — Polaris validates output using rules (block violations).
3. **Over time** — Polaris updates memory based on your feedback and behavior.
