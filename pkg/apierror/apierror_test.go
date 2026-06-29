package apierror

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAPIError_Error(t *testing.T) {
	err := BadRequest(CodeInvalidRequest, "test message")
	if err.Error() != "test message" {
		t.Errorf("Error() = %q, want %q", err.Error(), "test message")
	}
}

func TestBadRequest(t *testing.T) {
	err := BadRequest("bad_code", "bad input")
	if err.Status != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", err.Status, http.StatusBadRequest)
	}
	if err.Code != "bad_code" {
		t.Errorf("Code = %q, want %q", err.Code, "bad_code")
	}
	if err.Message != "bad input" {
		t.Errorf("Message = %q, want %q", err.Message, "bad input")
	}
}

func TestInternal(t *testing.T) {
	err := Internal("err_code", "something went wrong")
	if err.Status != http.StatusInternalServerError {
		t.Errorf("Status = %d, want %d", err.Status, http.StatusInternalServerError)
	}
	if err.Code != "err_code" {
		t.Errorf("Code = %q, want %q", err.Code, "err_code")
	}
}

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	WriteJSON(w, http.StatusOK, map[string]string{"key": "value"})

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}
	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["key"] != "value" {
		t.Errorf("body[key] = %q, want %q", body["key"], "value")
	}
}

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()
	apiErr := BadRequest("invalid", "bad value")
	WriteError(w, apiErr)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}
	var decoded APIError
	if err := json.NewDecoder(w.Body).Decode(&decoded); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if decoded.Status != http.StatusBadRequest {
		t.Errorf("decoded.Status = %d, want %d", decoded.Status, http.StatusBadRequest)
	}
	if decoded.Code != "invalid" {
		t.Errorf("decoded.Code = %q, want %q", decoded.Code, "invalid")
	}
	if decoded.Message != "bad value" {
		t.Errorf("decoded.Message = %q, want %q", decoded.Message, "bad value")
	}
}

func TestErrorCodes(t *testing.T) {
	tests := []struct {
		name string
		code string
	}{
		{"CodeInvalidRequest", CodeInvalidRequest},
		{"CodeUpstreamError", CodeUpstreamError},
		{"CodeInternalError", CodeInternalError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.code == "" {
				t.Errorf("expected non-empty error code")
			}
		})
	}
}
