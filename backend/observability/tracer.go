package observability

import (
	"context"
	"log/slog"
)

// FailureRecorder records bounded runtime-failure labels. Correlation IDs and
// error details are deliberately absent so metrics cannot gain unbounded or
// personal-data cardinality.
type FailureRecorder func(entryPoint, operation, outcome string)

// Tracer binds one injected logger and metric recorder to request and Worker
// correlation contexts.
type Tracer struct {
	logger        *slog.Logger
	recordFailure FailureRecorder
}

func NewTracer(logger *slog.Logger, recordFailure FailureRecorder) *Tracer {
	if logger == nil {
		logger = slog.Default()
	}
	return &Tracer{logger: logger, recordFailure: recordFailure}
}

func (t *Tracer) StartRequest(ctx context.Context, requestValue string) (context.Context, CorrelationID, error) {
	id, err := CorrelationIDFromRequest(requestValue)
	if err != nil {
		return ctx, CorrelationID{}, err
	}
	return WithCorrelationID(ctx, id), id, nil
}

func (t *Tracer) StartJob(ctx context.Context, _ string) (context.Context, CorrelationID, error) {
	id, err := NewCorrelationID()
	if err != nil {
		return ctx, CorrelationID{}, err
	}
	return WithCorrelationID(ctx, id), id, nil
}

func (t *Tracer) Logger(ctx context.Context) *slog.Logger {
	if id, ok := CorrelationIDFromContext(ctx); ok {
		return t.logger.With(slog.String("correlation_id", id.String()))
	}
	return t.logger
}

// Failure emits one stable error record and one bounded metric. Error details
// stay at Debug so student data embedded in an upstream error cannot enter
// Info-or-higher logs.
func (t *Tracer) Failure(ctx context.Context, entryPoint, operation, outcome string, err error) {
	logger := t.Logger(ctx)
	logger.ErrorContext(ctx, "runtime operation failed",
		slog.String("entry_point", entryPoint),
		slog.String("operation", operation),
		slog.String("outcome", outcome),
	)
	if err != nil {
		logger.DebugContext(ctx, "runtime failure detail", slog.String("error", err.Error()))
	}
	if t.recordFailure != nil {
		t.recordFailure(entryPoint, operation, outcome)
	}
}
