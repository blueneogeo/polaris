package proxy

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/polaris/internal/config"
	"github.com/polaris/pkg/apierror"
)

type Handler struct {
	cfg    *config.Config
	logger *slog.Logger
	client *http.Client
}

func NewHandler(cfg *config.Config, logger *slog.Logger) *Handler {
	return &Handler{
		cfg:    cfg,
		logger: logger,
		client: &http.Client{},
	}
}

// ChatCompletions handles POST /v1/chat/completions
func (h *Handler) ChatCompletions(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Error("failed to read request body", "error", err)
		apierror.WriteError(w, apierror.Internal(apierror.CodeInternalError, "failed to read request body"))
		return
	}
	defer r.Body.Close()

	var req chatRequest
	if err := json.Unmarshal(body, &req); err != nil {
		h.logger.Warn("invalid request body", "error", err)
		apierror.WriteError(w, apierror.BadRequest(apierror.CodeInvalidRequest, "invalid JSON request body"))
		return
	}

	isStream := req.Stream

	upstreamURL := strings.TrimRight(h.cfg.UpstreamURL, "/") + "/v1/chat/completions"
	upstreamReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, upstreamURL, strings.NewReader(string(body)))
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

	// Copy response headers
	for key, values := range upstreamResp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(upstreamResp.StatusCode)

	if upstreamResp.StatusCode >= 400 {
		// Non-2xx: just copy the error body
		if _, err := io.Copy(w, upstreamResp.Body); err != nil {
			h.logger.Error("failed to copy upstream error response", "error", err)
		}
		return
	}

	if isStream {
		h.streamResponse(w, upstreamResp)
	} else {
		if _, err := io.Copy(w, upstreamResp.Body); err != nil {
			h.logger.Error("failed to copy upstream response", "error", err)
		}
	}
}

func (h *Handler) streamResponse(w http.ResponseWriter, upstreamResp *http.Response) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		h.logger.Error("streaming not supported by response writer")
		return
	}

	buf := make([]byte, 4096)
	for {
		n, readErr := upstreamResp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := w.Write(buf[:n]); writeErr != nil {
				h.logger.Error("failed to write stream chunk", "error", writeErr)
				return
			}
			flusher.Flush()
		}
		if readErr != nil {
			if readErr != io.EOF {
				h.logger.Error("error reading upstream stream", "error", readErr)
			}
			break
		}
	}
}

// chatRequest is a minimal representation of a chat completion request.
// We pass the raw JSON body through, but unmarshal to check the stream field.
type chatRequest struct {
	Stream bool `json:"stream"`
}
