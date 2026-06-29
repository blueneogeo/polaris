package middleware

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLogger_200(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	handler := Logger(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !bytes.Contains(buf.Bytes(), []byte(`"level":"INFO"`)) {
		t.Errorf("expected INFO level log for 200, got: %s", buf.String())
	}
}

func TestLogger_400(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	handler := Logger(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))

	req := httptest.NewRequest(http.MethodGet, "/bad", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if !bytes.Contains(buf.Bytes(), []byte(`"level":"WARN"`)) {
		t.Errorf("expected WARN level log for 400, got: %s", buf.String())
	}
}

func TestLogger_500(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	handler := Logger(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))

	req := httptest.NewRequest(http.MethodGet, "/error", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if !bytes.Contains(buf.Bytes(), []byte(`"level":"ERROR"`)) {
		t.Errorf("expected ERROR level log for 500, got: %s", buf.String())
	}
}

func TestLogger_LogIncludesMethodPathStatus(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	handler := Logger(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	logOutput := buf.String()
	for _, key := range []string{"method", "path", "status", "duration"} {
		if !bytes.Contains(buf.Bytes(), []byte(`"`+key+`"`)) {
			t.Errorf("log output missing key %q: %s", key, logOutput)
		}
	}
	if !bytes.Contains(buf.Bytes(), []byte(`"POST"`)) {
		t.Errorf("log output missing method POST: %s", logOutput)
	}
}

func TestRecovery_NoPanic(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	handler := Recovery(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestRecovery_PanicReturns500(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	handler := Recovery(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}))

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("internal_error")) {
		t.Errorf("response body should contain internal_error: %s", rec.Body.String())
	}
}

func TestRecovery_PanicLogsStacktrace(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	handler := Recovery(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}))

	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	logOutput := buf.String()
	if !bytes.Contains(buf.Bytes(), []byte(`"level":"ERROR"`)) {
		t.Errorf("expected ERROR level log for panic, got: %s", logOutput)
	}
	if !bytes.Contains(buf.Bytes(), []byte(`panic recovered`)) {
		t.Errorf("expected 'panic recovered' in log: %s", logOutput)
	}
	if !bytes.Contains(buf.Bytes(), []byte(`"stack"`)) {
		t.Errorf("expected stack trace in log: %s", logOutput)
	}
}

func TestResponseWriter_WritesDefaultStatus(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		_, _ = rw.Write([]byte("hello"))
		if rw.status != http.StatusOK {
			t.Errorf("default status = %d, want %d", rw.status, http.StatusOK)
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
}

func TestResponseWriter_CaptureExplicitStatus(t *testing.T) {
	var capturedStatus int
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		rw.WriteHeader(http.StatusNotFound)
		capturedStatus = rw.status
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if capturedStatus != http.StatusNotFound {
		t.Errorf("capturedStatus = %d, want %d", capturedStatus, http.StatusNotFound)
	}
	if rec.Code != http.StatusNotFound {
		t.Errorf("rec.Code = %d, want %d", rec.Code, http.StatusNotFound)
	}
}
