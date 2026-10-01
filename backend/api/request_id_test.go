package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
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

func TestRequestIDGenerationFailureHasProblemResponse(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelInfo}))
	tracer := newRuntimeTracer(logger)
	handler := requestIDMiddlewareWithStartRequest(
		tracer,
		func(ctx context.Context, _ string) (context.Context, string, error) {
			return ctx, "", errors.New("random source unavailable")
		},
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			t.Fatal("handler ran after RequestID generation failed")
		}),
	)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", response.Code)
	}
	if got := response.Header().Get("Content-Type"); got != "application/problem+json" {
		t.Errorf("Content-Type = %q, want application/problem+json", got)
	}
	if got := response.Header().Get(middleware.RequestIDHeader); got != "" {
		t.Errorf("%s = %q, want empty because generation failed", middleware.RequestIDHeader, got)
	}
	var body map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	want := map[string]any{
		"status":   "error",
		"error":    http.StatusText(http.StatusInternalServerError),
		"code":     "general.server",
		"type":     "https://moto-app.de/help/fehlermeldungen#anleitung-unerwarteter-fehler",
		"title":    http.StatusText(http.StatusInternalServerError),
		"detail":   http.StatusText(http.StatusInternalServerError),
		"instance": "", // No correlation ID exists when generation itself fails.
	}
	if !reflect.DeepEqual(body, want) {
		t.Errorf("body = %#v, want %#v", body, want)
	}
	if !bytes.Contains(logs.Bytes(), []byte(`"outcome":"generation_failure"`)) {
		t.Errorf("missing bounded failure log: %s", logs.String())
	}
	if bytes.Contains(logs.Bytes(), []byte("random source unavailable")) {
		t.Fatal("failure log leaked the generation error at Info-or-higher")
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
