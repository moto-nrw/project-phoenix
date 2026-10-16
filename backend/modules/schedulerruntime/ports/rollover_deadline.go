package ports

import (
	"context"
	"time"
)

// RolloverDeadlineRunner is the narrow contract the scheduler needs
// from the enrollment rollover service. Defined here so the scheduler
// doesn't import the full enrollment service package — mirrors the
// opt-in shape used by StudentLifecycleRepository.
type RolloverDeadlineRunner interface {
	RunDeadlineWorker(ctx context.Context, asOf time.Time) (any, error)
}
