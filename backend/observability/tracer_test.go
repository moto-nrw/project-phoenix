package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"slices"
	"testing"
)

func TestTracerCorrelatesRequestAndWorkerFailures(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, &slog.HandlerOptions{Level: slog.LevelInfo}))
	var labels []string
	tracer := NewTracer(logger, func(entryPoint, operation, outcome string) {
		labels = []string{entryPoint, operation, outcome}
	})

	const suppliedRequestID = "8dc3a9ca-8ac7-4b8e-9bfa-3c17760d92c0"
	requestCtx, requestID, err := tracer.StartRequest(context.Background(), suppliedRequestID)
	if err != nil {
		t.Fatalf("StartRequest() error = %v", err)
	}
	if got := requestID.String(); got != suppliedRequestID {
		t.Fatalf("StartRequest() ID = %q, want %q", got, suppliedRequestID)
	}
	if got, ok := CorrelationIDFromContext(requestCtx); !ok || got != requestID {
		t.Fatalf("CorrelationIDFromContext() = (%q, %v), want (%q, true)", got.String(), ok, requestID.String())
	}

	workerCtx, workerID, err := tracer.StartJob(context.Background(), "pwa-usage-cleanup")
	if err != nil {
		t.Fatalf("StartJob() error = %v", err)
	}
	if workerID.String() == "" || workerID == requestID {
		t.Fatalf("StartJob() ID = %q, want a new non-empty ID", workerID.String())
	}

	tracer.Failure(
		workerCtx,
		"worker",
		"pwa-usage-cleanup",
		"transaction_failure",
		errors.New("student Ada Lovelace could not be processed"),
	)

	if got, want := labels, []string{"worker", "pwa-usage-cleanup", "transaction_failure"}; !slices.Equal(got, want) {
		t.Fatalf("failure metric labels = %v, want %v", got, want)
	}

	var record map[string]any
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatalf("decode failure log: %v\n%s", err, output.String())
	}
	for key, want := range map[string]string{
		"level":          "ERROR",
		"correlation_id": workerID.String(),
		"entry_point":    "worker",
		"operation":      "pwa-usage-cleanup",
		"outcome":        "transaction_failure",
	} {
		if got := record[key]; got != want {
			t.Errorf("log field %q = %v, want %q", key, got, want)
		}
	}
	if _, leaked := record["error"]; leaked {
		t.Error("error details leaked at Info-or-higher log level")
	}
	if bytes.Contains(output.Bytes(), []byte("Ada Lovelace")) {
		t.Error("student name leaked at Info-or-higher log level")
	}
}
