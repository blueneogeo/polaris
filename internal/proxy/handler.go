package proxy

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/openai/openai-go/v3"
	"github.com/polaris/internal/config"
	"github.com/polaris/pkg/apierror"
)

// Handler is the HTTP handler for the proxy. It accepts OpenAI-compatible
// chat completion requests, forwards them to the upstream worker model,
// and streams the response back through a block pipeline.
type Handler struct {
	cfg       *config.Config
	logger    *slog.Logger
	client    *http.Client
	validator Validator
	enhancer  PromptEnhancer
}

// NewHandler creates a new proxy handler with the given configuration.
// Uses PassValidator and NoopEnhancer by default.
func NewHandler(cfg *config.Config, logger *slog.Logger) *Handler {
	return &Handler{
		cfg:       cfg,
		logger:    logger,
		client:    &http.Client{},
		validator: &PassValidator{},
		enhancer:  &NoopEnhancer{},
	}
}

// streamRequest is a minimal struct to detect the stream flag from raw JSON.
// We can't rely on ChatCompletionNewParams.Stream because the SDK doesn't
// expose stream as a typed field (it sets it internally via options).
type streamRequest struct {
	Stream bool `json:"stream"`
}

// ChatCompletions handles POST /v1/chat/completions.
func (h *Handler) ChatCompletions(w http.ResponseWriter, r *http.Request) {
	// 1. Read the raw request body
	rawBody, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Error("failed to read request body", "error", err)
		apierror.WriteError(w, apierror.Internal(apierror.CodeInternalError, "failed to read request body"))
		return
	}
	defer r.Body.Close()

	// 2. Validate and detect stream flag
	var sr streamRequest
	if err := json.Unmarshal(rawBody, &sr); err != nil {
		h.logger.Warn("invalid request body", "error", err)
		apierror.WriteError(w, apierror.BadRequest(apierror.CodeInvalidRequest, "invalid JSON request body"))
		return
	}
	isStream := sr.Stream

	// 3. Optionally parse into typed params for enhancement
	var params openai.ChatCompletionNewParams
	if err := json.Unmarshal(rawBody, &params); err != nil {
		h.logger.Warn("invalid request body for typed parsing", "error", err)
		// Continue with raw body only — enhancement will be skipped
	}

	// 4. Enhance the prompt (noop in v0 — modifies params in place)
	if err := h.enhancer.Enhance(r.Context(), &params); err != nil {
		h.logger.Error("prompt enhancement failed", "error", err)
		apierror.WriteError(w, apierror.Internal(apierror.CodeInternalError, "prompt enhancement failed"))
		return
	}

	// 5. Forward to upstream
	upstreamURL := strings.TrimRight(h.cfg.UpstreamURL, "/") + "/v1/chat/completions"

	// For v0: forward raw body to preserve unknown fields.
	// When enhancement is active, serialize enhanced params instead.
	upstreamReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, upstreamURL, strings.NewReader(string(rawBody)))
	if err != nil {
		h.logger.Error("failed to create upstream request", "error", err)
		apierror.WriteError(w, apierror.Internal(apierror.CodeInternalError, "failed to create upstream request"))
		return
	}
	upstreamReq.Header.Set("Content-Type", "application/json")
	upstreamReq.Header.Set("Authorization", "Bearer "+h.cfg.UpstreamAPIKey)

	upstreamResp, err := h.client.Do(upstreamReq)
	if err != nil {
		h.logger.Error("upstream request failed", "error", err)
		apierror.WriteError(w, apierror.Internal(apierror.CodeUpstreamError, fmt.Sprintf("upstream request failed: %v", err)))
		return
	}
	defer upstreamResp.Body.Close()

	// 6. Copy response headers and status
	for key, values := range upstreamResp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(upstreamResp.StatusCode)

	if upstreamResp.StatusCode >= 400 {
		_, _ = io.Copy(w, upstreamResp.Body)
		return
	}

	// 7. Stream or non-streaming path
	if isStream {
		h.handleStream(w, upstreamResp)
	} else {
		h.handleNonStream(w, upstreamResp)
	}
}

// handleStream processes a streaming response through the block pipeline.
func (h *Handler) handleStream(w http.ResponseWriter, upstreamResp *http.Response) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		h.logger.Error("streaming not supported by response writer")
		return
	}

	detector := NewBlockDetector(upstreamResp)
	writer := NewBlockWriter(w, flusher, h.logger)

	for detector.Next() {
		block := detector.Block()

		// Validate the block based on its type
		var decision Decision
		var err error
		switch block.Type {
		case BlockText:
			decision, err = h.validator.ValidateText(upstreamResp.Request.Context(), block)
		case BlockToolCall:
			decision, err = h.validator.ValidateToolCall(upstreamResp.Request.Context(), block)
		default:
			// Thinking blocks pass through without validation
			decision = Decision{Allowed: true}
		}
		if err != nil {
			h.logger.Error("block validation error", "error", err)
			decision = Decision{Allowed: true}
		}

		// For v0 (PassValidator): everything passes. In v0.2+:
		// if !decision.Allowed { inject feedback, cancel, retry }
		_ = decision

		if err := writer.WriteBlock(block); err != nil {
			h.logger.Error("failed to write block", "error", err)
			return
		}
	}

	if err := detector.Err(); err != nil {
		h.logger.Error("block detector error", "error", err)
	}

	if err := writer.WriteDone(); err != nil {
		h.logger.Error("failed to write [DONE]", "error", err)
	}
}

// handleNonStream processes a non-streaming response.
func (h *Handler) handleNonStream(w http.ResponseWriter, upstreamResp *http.Response) {
	body, err := io.ReadAll(upstreamResp.Body)
	if err != nil {
		h.logger.Error("failed to read upstream response body", "error", err)
		return
	}

	// Parse into typed response for future validation
	var completion openai.ChatCompletion
	if err := json.Unmarshal(body, &completion); err != nil {
		h.logger.Error("failed to unmarshal upstream response", "error", err)
		return
	}

	// For v0: pass through. In v0.2+: validate completion content here.
	if _, err := w.Write(body); err != nil {
		h.logger.Error("failed to write response", "error", err)
	}
}
