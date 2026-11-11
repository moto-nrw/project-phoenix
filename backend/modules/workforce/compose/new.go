// Package compose wires the Workforce work-time module over the shared tenant
// runtime, the Bun database, and the School Membership capability that owns
// the staff rows a template is assigned to.
package compose

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/planning"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type Observation = ports.Observation

// Dependencies are the collaborators the Workforce work-time module cannot
// own. The template binding lives on Workforce's employment profile (#2753);
// LiveStaffIDs asks School Membership which of the bound memberships are
// still live. It is required so a missing wiring fails at startup instead of
// silently skipping the assigned-staff refresh.
type Dependencies struct {
	DB *bun.DB
	// LiveStaffIDs returns the given memberships that are not offboarded,
	// sorted by ID.
	LiveStaffIDs        func(ctx context.Context, membershipIDs []int64) ([]int64, error)
	LockStaffAssignment func(context.Context, int64) error
	Observe             func(Observation)
	// Now is the clock the live work-session window and the calendar day
	// are measured against; nil means the wall clock. Tests pin it.
	Now func() time.Time
	// StatutoryHolidays answers the tenant's statutory holidays from the
	// School Calendar; a Sonderarbeitszeit sets no target on them. Optional
	// for compositions that never resolve a Soll: nil treats no day as a
	// statutory holiday.
	StatutoryHolidays func(ctx context.Context, from, to string) (map[string]bool, error)
}

// WorkTime is the owner composition: its public capability and native shift
// reads share one application service, transaction runtime and observer.
type WorkTime interface {
	workforce.Capability
	planning.ShiftReads
}

// New composes the Workforce work-time module. Every operation runs on the
// caller's ambient tenant transaction when one exists and otherwise opens one,
// so row-level security decides visibility exactly as it did for the legacy
// repositories.
func New(dependencies Dependencies) (WorkTime, error) {
	service, err := newApplication(dependencies)
	if err != nil {
		return nil, err
	}
	return service, nil
}

func newApplication(dependencies Dependencies) (*application.Service, error) {
	if dependencies.DB == nil || dependencies.LiveStaffIDs == nil ||
		dependencies.LockStaffAssignment == nil || dependencies.Observe == nil {
		return nil, errors.New("workforce compose: all dependencies are required")
	}
	store := postgres.New(postgres.DatabaseRuntime(dependencies.DB))
	now := dependencies.Now
	if now == nil {
		now = time.Now
	}
	service := application.New(
		store,
		transaction{lock: store.AcquireXactLock},
		postgres.Assignments{Store: store, Live: dependencies.LiveStaffIDs},
		dependencies.LockStaffAssignment,
		application.Clock{Time: now},
		application.AllowanceUses,
		application.NewObserver(dependencies.Observe),
		dependencies.StatutoryHolidays,
	)
	return service, nil
}

// transaction runs units of work on the tenant runtime. lock is the Postgres
// adapter's advisory-lock statement, used when no runtime is bound.
type transaction struct {
	lock func(context.Context, string) error
}

func (transaction) RunWrite(ctx context.Context, callback func(context.Context) error) error {
	return postgres.TransactionRunWriteBinding(ctx, callback, tenant.WithinCurrentTenant)
}

func (t transaction) LockStaffQualifications(ctx context.Context, staffID int64) error {
	return postgres.LockStaffQualificationsGate(ctx, staffID, t.acquireXactLock)
}

// LockStaffBalance serializes every writer that changes one staff member's
// target working time, so a template refresh and a manual schedule change
// cannot interleave into a half-closed version history.
func (t transaction) LockStaffBalance(ctx context.Context, staffID int64) error {
	return postgres.LockStaffBalanceGate(ctx, staffID, t.acquireXactLock)
}

// acquireXactLock takes the transaction-scoped advisory lock through the
// tenant runtime. Without a runtime, or without a transaction at all, the
// Postgres adapter issues the statement itself, which is what the legacy
// repositories did: outside a transaction the lock releases at statement
// end, so a caller that never opened one keeps its behavior.
func (t transaction) acquireXactLock(ctx context.Context, key string) error {
	return postgres.AcquireXactLock(ctx, key, t.lock)
}

func (t transaction) LockStaffShifts(ctx context.Context, staffID int64) error {
	return postgres.LockStaffShiftsGate(ctx, staffID, t.acquireXactLock)
}

// RunRead establishes RLS for standalone owner queries; workflows and HTTP
// requests keep their existing transaction.
func (t transaction) RunRead(ctx context.Context, callback func(context.Context) error) error {
	return postgres.TransactionRunReadBinding(ctx, callback, t.RunWrite)
}
