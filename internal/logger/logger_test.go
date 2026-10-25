package logger

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func testHandler(w http.ResponseWriter, r *http.Request) {
	reqID := r.Context().Value(RequestIDKey)
	if reqID == nil {
		t := w
		http.Error(t, "missing request id", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"logged"}`))
}

func TestHTTPLoggerMiddleware(t *testing.T) {
	handler := HTTPLogger(http.HandlerFunc(testHandler))

	req := httptest.NewRequest(http.MethodGet, "/test/log", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	reqID := rec.Header().Get("X-Request-ID")
	if reqID == "" {
		t.Fatalf("expected X-Request-ID header to be set")
	}
}
