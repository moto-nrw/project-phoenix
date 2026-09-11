package delivery

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

type WebPushConfig struct {
	Subscriber string
	PublicKey  string
	PrivateKey string
}

func (c WebPushConfig) Configured() bool {
	return c.Subscriber != "" && c.PublicKey != "" && c.PrivateKey != ""
}

type PushSender interface {
	SendPush(context.Context, ClaimedIntent) (ProviderResult, error)
}

type ProviderResult struct {
	MessageID  string          `json:"message_id,omitempty"`
	StatusCode int             `json:"status_code,omitempty"`
	Details    json.RawMessage `json:"details,omitempty"`
}

const ProviderAcceptedStatusCode = 202

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
	RunOnce(context.Context, int, int) (WorkerStats, error)
	Backlog(context.Context) (int, error)
}

type Worker struct{ engine workerEngine }

func NewWorker(engine workerEngine) *Worker {
	if engine == nil {
		panic("delivery worker: engine is required")
	}
	return &Worker{engine: engine}
}

func (w *Worker) RunOnce(ctx context.Context, batchSize, maxAttempts int) (int, error) {
	if batchSize <= 0 {
		return 0, errors.New("delivery worker: batch size must be positive")
	}
	if maxAttempts <= 0 {
		return 0, errors.New("delivery worker: max attempts must be positive")
	}
	stats, err := w.engine.RunOnce(ctx, batchSize, maxAttempts)
	return stats.Claimed, err
}

func (w *Worker) Backlog(ctx context.Context) (int, error) { return w.engine.Backlog(ctx) }
