package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

func Logger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			wrapped := &responseWriter{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(wrapped, r)

			duration := time.Since(start).String()
			attrs := []any{
				"method", r.Method,
				"path", r.URL.Path,
				"status", wrapped.status,
				"duration", duration,
				"remote_addr", r.RemoteAddr,
			}

			if wrapped.status >= 500 {
				logger.Error("request", attrs...)
			} else if wrapped.status >= 400 {
				logger.Warn("request", attrs...)
			} else {
				logger.Info("request", attrs...)
			}
		})
	}
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}
