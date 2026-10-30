package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
)

// PayrollDispatchStore persists active.payroll_dispatches (#4805) in the
// tenant transaction of the context.
type PayrollDispatchStore interface {
	// NextPayrollDispatchSequence locks the month's dispatches and returns
	// the sequence a new one takes: 0 for the first file, then corrections.
	NextPayrollDispatchSequence(ctx context.Context, year, month int) (int, error)
	InsertPayrollDispatch(context.Context, domain.PayrollDispatch) error
	LatestPayrollDispatch(context.Context) (*domain.PayrollDispatch, error)
	// ClaimPayrollDispatches leases due pending and mailing dispatches.
	ClaimPayrollDispatches(ctx context.Context, now time.Time, lease time.Duration, limit int) ([]domain.PayrollDispatch, error)
	// FinishPayrollDispatch writes the outcome of one claimed pass, fenced by
	// its lease token. It reports false when the lease was lost.
	FinishPayrollDispatch(ctx context.Context, dispatch domain.PayrollDispatch, now time.Time) (bool, error)
	// OpenPayrollDownload spends a live link: it marks the download and
	// clears the file in one statement, and returns the file.
	OpenPayrollDownload(ctx context.Context, tokenHash string, now time.Time) (*domain.PayrollDispatch, error)
	// PurgePayrollDispatches clears files of expired links and deletes rows
	// that finished before the cutoff.
	PurgePayrollDispatches(ctx context.Context, now, finishedBefore time.Time) error
}
