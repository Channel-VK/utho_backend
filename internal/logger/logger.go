package logger

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
)

type contextKey string

const RequestIDKey contextKey = "request_id"

var defaultLogger *slog.Logger

func init() {
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}
	defaultLogger = slog.New(slog.NewTextHandler(os.Stdout, opts))
	slog.SetDefault(defaultLogger)
}

// GenerateRequestID generates a random 12-character hex ID.
func GenerateRequestID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return "req-" + hex.EncodeToString(b)
}

type responseRecorder struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int64
}

func (rec *responseRecorder) WriteHeader(code int) {
	rec.statusCode = code
	rec.ResponseWriter.WriteHeader(code)
}

func (rec *responseRecorder) Write(b []byte) (int, error) {
	if rec.statusCode == 0 {
		rec.statusCode = http.StatusOK
	}
	n, err := rec.ResponseWriter.Write(b)
	rec.bytesWritten += int64(n)
	return n, err
}

// HTTPLogger returns an HTTP middleware that logs structured details for every incoming request.
func HTTPLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		reqID := r.Header.Get("X-Request-ID")
		if reqID == "" {
			reqID = GenerateRequestID()
		}

		ctx := context.WithValue(r.Context(), RequestIDKey, reqID)
		w.Header().Set("X-Request-ID", reqID)

		rec := &responseRecorder{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		// Execute next handler
		next.ServeHTTP(rec, r.WithContext(ctx))

		duration := time.Since(start)
		status := rec.statusCode
		level := slog.LevelInfo
		if status >= 500 {
			level = slog.LevelError
		} else if status >= 400 {
			level = slog.LevelWarn
		}

		clientIP := r.RemoteAddr
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			clientIP = strings.Split(xff, ",")[0]
		}

		slog.Log(r.Context(), level, fmt.Sprintf("[%s] %s %s -> %d in %v (%d bytes)", r.Method, r.URL.Path, clientIP, status, duration, rec.bytesWritten),
			slog.String("request_id", reqID),
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", status),
			slog.Duration("latency", duration),
			slog.String("client_ip", clientIP),
			slog.Int64("bytes", rec.bytesWritten),
		)
	})
}
