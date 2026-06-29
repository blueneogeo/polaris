package apierror

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

type APIError struct {
	Status  int    `json:"status"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *APIError) Error() string {
	return e.Message
}

const (
	CodeInvalidRequest = "invalid_request"
	CodeUpstreamError  = "upstream_error"
	CodeInternalError  = "internal_error"
)

func BadRequest(code, msg string) *APIError {
	return &APIError{Status: http.StatusBadRequest, Code: code, Message: msg}
}

func Internal(code, msg string) *APIError {
	return &APIError{Status: http.StatusInternalServerError, Code: code, Message: msg}
}

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("failed to encode JSON response", "error", err)
	}
}

func WriteError(w http.ResponseWriter, err *APIError) {
	WriteJSON(w, err.Status, err)
}
