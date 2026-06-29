# Commands

User-facing slash commands. These are registered by the Polaris OpenCode plugin (not yet built) and call the Polaris [Admin API](admin-api.md).

## Model management

### `/polaris-models-add`

Add a new model to the wallet.

```
> /polaris-models-add

Polaris asks:
  Q1: "What's the upstream URL?"  →  https://api.openai.com
  Q2: "What's the API key?"       →  sk-...

Polaris: "✓ Validating... API call successful."
Polaris: "✓ Model 'openai' added to wallet."
```

The name is auto-derived from the URL hostname (`api.openai.com` → `openai`). If the name already exists, Polaris asks for a different name or offers to replace.

### `/polaris-models-remove`

Remove a model from the wallet.

```
> /polaris-models-remove

Polaris shows interactive list:
  1. openai     (api.openai.com)
  2. deepseek   (api.deepseek.com)
  3. claude     (api.anthropic.com)

Pick one → confirm → removed.
```

### `/polaris-models`

Quick wallet status.

```
> /polaris-models

Polaris: "3 models in wallet. Worker: openai. Polaris brain: not set."
```

### `/polaris-models-list`

Detailed model list.

```
> /polaris-models-list

Polaris:
  openai     api.openai.com         [worker]
  deepseek   api.deepseek.com
  claude     api.anthropic.com
```

## Setup

### `/polaris-setup`

First-run wizard. Also re-runnable to change worker or brain model assignments.

```
> /polaris-setup

Polaris detects empty wallet:
  "Welcome to Polaris setup. You don't have any models configured yet.
   Let's add your first one."

  Q1: "Upstream URL?"    →  https://api.openai.com
  Q2: "API key?"         →  sk-...

  "✓ Validating... success."
  "✓ Model 'openai' added."

  Q3: "Use 'openai' as your worker model? [yes/no]"

  "✓ Worker model set to 'openai'."

  (Future) Q4: "Add a model for Polaris's brain (for judging/validation)?
               Or use the same model? [add separate / use worker / skip]"

  "Setup complete. Polaris is ready."
```

If re-run with models already configured:

```
> /polaris-setup

  "Polaris is already configured. What would you like to change?"
  1. Change worker model (current: openai)
  2. Change brain model (current: not set)
  3. Add a new model
  4. Remove a model
  5. Exit
```

## Rules

### `/polaris-rules-add`

```
> /polaris-rules-add

Polaris asks:
  Q1: "Describe the rule:"  → "never use any in TypeScript"
  Q2: "Scope:" → [global | project | session]

Polaris: "✓ Rule added (global #1: never use any in TypeScript)"
```

### `/polaris-rules-list`

```
> /polaris-rules-list

Global rules:
  1. never use any in TypeScript
  2. keep responses brief

Project rules:
  (none)

Session rules:
  (none)
```

### `/polaris-rules-remove`

```
> /polaris-rules-remove

Polaris shows interactive list of all rules.
Pick one → confirm → removed.
```

## Memory

### `/polaris-forget`

Undoes the most recent memory that Polaris auto-learned.

```
> /polaris-forget

Polaris: "Removed memory #3: 'The user dislikes verbose responses'"
```

## Status

### `/polaris-status`

Snapshot of current configuration.

```
> /polaris-status

Worker model:   openai (api.openai.com)
Brain model:    not set
Models:         3 in wallet
Rules:          2 global, 0 project, 0 session
Memory:         5 entries (3 global, 2 project)
```
