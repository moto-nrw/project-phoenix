package delivery

import (
	"context"
	"errors"
	"fmt"
	"strings"

	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
)

type workerEngine interface {
	RunOnce(context.Context, int, int) (deliveryContract.WorkerStats, error)
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
	if err == nil && stats.DeadLettered > 0 {
		err = fmt.Errorf("%w: %d", deliveryContract.ErrDeadLettered, stats.DeadLettered)
		if len(stats.DeadLetters) > 0 {
			err = fmt.Errorf("%w (%s)", err, strings.Join(stats.DeadLetters, "; "))
		}
	}
	return stats.Claimed, err
}

func (w *Worker) Backlog(ctx context.Context) (int, error) { return w.engine.Backlog(ctx) }

// EmailWakeups reports committed e-mail enqueues until ctx ends, so the
// outbox can run at once instead of at its next poll. A wake-up may be
// lost; the poll stays the fallback. Nil means polling only.
func (w *Worker) EmailWakeups(ctx context.Context) <-chan struct{} {
	engine, ok := w.engine.(interface {
		EmailWakeups(context.Context) <-chan struct{}
	})
	if !ok {
		return nil
	}
	return engine.EmailWakeups(ctx)
}
