# Admin API

HTTP API for runtime configuration of Polaris. Called by the Polaris OpenCode plugin, but accessible from any HTTP client.

Base URL: `http://localhost:8777` (or `$PORT`)

## Model management

### POST /admin/models

Add a new model to the wallet. Validates the URL and API key with a test API call before saving.

**Request:**

```json
{
  "name": "openai",
  "url": "https://api.openai.com",
  "api_key": "sk-..."
}
```

**Response (200):**

```json
{
  "ok": true,
  "name": "openai",
  "message": "Model added and validated successfully"
}
```

**Response (409):** Name already exists.

**Response (400):** Validation failed — URL unreachable or API key invalid.

---

### GET /admin/models

List all models in the wallet. API keys are never returned.

**Response (200):**

```json
{
  "models": [
    {"name": "openai", "url": "https://api.openai.com", "roles": ["worker"]},
    {"name": "deepseek", "url": "https://api.deepseek.com", "roles": []}
  ],
  "worker": "openai",
  "brain": null
}
```

---

### DELETE /admin/models/{name}

Remove a model from the wallet.

**Response (200):**

```json
{"ok": true, "message": "Model 'openai' removed"}
```

**Response (404):** Model not found.

---

### POST /admin/models/worker

Set which model is the current worker model (the model Polaris proxies work to).

**Request:**

```json
{"name": "openai"}
```

**Response (200):**

```json
{"ok": true, "worker": "openai"}
```

**Response (404):** No model named "openai" in wallet.

---

### POST /admin/models/brain

Set which model is Polaris's brain model (the model used for judging, validation, and prompt enhancement).

Same request/response structure as the worker endpoint.

---

## Rules management (future)

Endpoints for managing rules at runtime. Rules are stored in TOML files but can be modified via the API.

### POST /admin/rules

### GET /admin/rules

### DELETE /admin/rules/{id}

## Memory management (future)

### POST /admin/memory/forget

## Health

### GET /health

**Response (200):**

```json
{"ok": true}
```
