package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5/middleware"
)

func TestRequestIDMiddlewarePreservesChiContextContract(t *testing.T) {
	t.Parallel()

	const requestValue = "8dc3a9ca-8ac7-4b8e-9bfa-3c17760d92c0"
	tracer := newRuntimeTracer(nil)
	var got string
	handler := requestIDMiddleware(tracer, http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = middleware.GetReqID(r.Context())
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(middleware.RequestIDHeader, requestValue)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if got != requestValue {
		t.Fatalf("GetReqID() = %q, want %q", got, requestValue)
	}
}

func TestRequestFailureLogUsesRootCorrelationAndRedactsErrorDetail(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelInfo}))
	tracer := newRuntimeTracer(logger)
	handler := requestIDMiddleware(tracer, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tracer.Failure(r.Context(), "http", "tenant-transaction", "transaction_failure",
			errors.New("student Erika Mustermann failed"))
		w.WriteHeader(http.StatusInternalServerError)
	}))
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set(middleware.RequestIDHeader, "8dc3a9ca-8ac7-4b8e-9bfa-3c17760d92c0")

	handler.ServeHTTP(httptest.NewRecorder(), request)

	var record map[string]any
	if err := json.Unmarshal(logs.Bytes(), &record); err != nil {
		t.Fatalf("decode failure log: %v\n%s", err, logs.String())
	}
	for key, want := range map[string]string{
		"correlation_id": "8dc3a9ca-8ac7-4b8e-9bfa-3c17760d92c0",
		"entry_point":    "http",
		"operation":      "tenant-transaction",
		"outcome":        "transaction_failure",
	} {
		if got := record[key]; got != want {
			t.Errorf("log field %q = %v, want %q", key, got, want)
		}
	}
	if bytes.Contains(logs.Bytes(), []byte("Erika Mustermann")) {
		t.Fatal("request failure leaked a student name at Info-or-higher")
	}
	if _, leaked := record["error"]; leaked {
		t.Fatal("request failure leaked raw error detail at Info-or-higher")
	}
}

func TestRequestIDMiddlewareSetsResponseHeaderBeforeHandler(t *testing.T) {
	t.Parallel()

	const requestValue = "8dc3a9ca-8ac7-4b8e-9bfa-3c17760d92c0"
	tracer := newRuntimeTracer(nil)
	handler := requestIDMiddleware(tracer, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if got := w.Header().Get(middleware.RequestIDHeader); got != requestValue {
			t.Errorf("response %s header = %q, want %q", middleware.RequestIDHeader, got, requestValue)
		}
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(middleware.RequestIDHeader, requestValue)
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if got := res.Header().Get(middleware.RequestIDHeader); got != requestValue {
		t.Errorf("response %s header = %q, want %q", middleware.RequestIDHeader, got, requestValue)
	}
}
