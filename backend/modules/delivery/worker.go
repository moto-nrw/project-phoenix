package delivery

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

type ProviderResult struct {
	MessageID  string          `json:"message_id,omitempty"`
	StatusCode int             `json:"status_code,omitempty"`
	Details    json.RawMessage `json:"details,omitempty"`
}

type ClaimedIntent struct {
	ID             int64
	TenantID       int64
	Transport      Transport
	Template       string
	EmailRecipient EmailRecipient
	PushRecipient  PushRecipient
	EmailPayload   json.RawMessage
	PushPayload    PushPayload
	Attempts       int
	LeaseToken     string
	LeaseExpiresAt time.Time
}

type WorkerStats struct {
	Claimed      int
	Sent         int
	Cancelled    int
	Retried      int
	DeadLettered int
	LeaseLost    int
}

type workerEngine interface {
	RunOnce(context.Context, int) (WorkerStats, error)
	Backlog(context.Context) (int, error)
	SetMaxAttempts(int)
}

type Worker struct{ engine workerEngine }

func NewWorker(engine workerEngine) *Worker {
	if engine == nil {
		panic("delivery worker: engine is required")
	}
	return &Worker{engine: engine}
}

func (w *Worker) RunOnce(ctx context.Context, batchSize int) (int, error) {
	if batchSize <= 0 {
		return 0, errors.New("delivery worker: batch size must be positive")
	}
	stats, err := w.engine.RunOnce(ctx, batchSize)
	return stats.Claimed, err
}

func (w *Worker) Backlog(ctx context.Context) (int, error) { return w.engine.Backlog(ctx) }

func (w *Worker) SetMaxAttempts(attempts int) { w.engine.SetMaxAttempts(attempts) }
