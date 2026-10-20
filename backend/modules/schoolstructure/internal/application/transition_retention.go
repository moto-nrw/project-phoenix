package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure/internal/domain"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// TransitionRetentionStore takes the transition gate and deletes the
// transitions of a school that ended before cutoff, their mappings, history,
// class-list ledger, class teachers and roster removals by cascade.
type TransitionRetentionStore interface {
	LockTransitions(ctx context.Context, tenantID int64) (domain.OperationStats, error)
	DeleteTransitionsEndedBefore(ctx context.Context, tenantID int64, cutoff time.Time) (int64, domain.OperationStats, error)
}

// TransitionRetention is the School Structure grade-transition retention.
type TransitionRetention struct{ store TransitionRetentionStore }

// NewTransitionRetention composes the deletion of expired grade transitions
// (#4045) over the store, counted in Berlin calendar days.
func NewTransitionRetention(store TransitionRetentionStore) *TransitionRetention {
	if store == nil {
		panic("school structure application: transition retention requires its store")
	}
	return &TransitionRetention{store: store}
}

// DeleteExpiredGradeTransitions deletes the transitions that ended before the
// Berlin midnight of the first calendar day a window of retentionDays keeps.
// It holds the transition gate, so no apply or revert of the school runs
// meanwhile.
func (r *TransitionRetention) DeleteExpiredGradeTransitions(ctx context.Context, retentionDays int) (int, error) {
	if retentionDays <= 0 {
		return 0, fmt.Errorf("%w, got %d", schoolstructure.ErrGradeTransitionRetentionWindow, retentionDays)
	}
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return 0, err
	}
	if _, ok := tenant.TransactionFromContext(ctx); !ok {
		return 0, errors.New("school structure: the transition retention requires a tenant transaction")
	}
	if _, err := r.store.LockTransitions(ctx, tenantID.Int64()); err != nil {
		return 0, err
	}
	deleted, _, err := r.store.DeleteTransitionsEndedBefore(ctx, tenantID.Int64(), calendar.TodayDate().AddDays(-retentionDays).BerlinMidnight())
	return int(deleted), err
}
