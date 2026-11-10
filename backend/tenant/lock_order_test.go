package tenant_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// inLedgerTransaction runs fn in a tenant transaction of a unit of work
// without a database, so the ledger is the one the runtime installs.
func inLedgerTransaction(t *testing.T, fn func(context.Context) error) error {
	t.Helper()
	uow, err := tenant.NewUnitOfWork(
		func(ctx context.Context, _ int64, fn func(context.Context, any) error) error {
			return fn(ctx, struct{}{})
		},
		func(ctx context.Context, fn func(context.Context, any) error) error {
			return fn(ctx, struct{}{})
		},
		func(context.Context, tenant.SavepointAction) error { return nil },
		func(error) bool { return false },
	)
	require.NoError(t, err)
	id, err := tenant.NewTenantID(7)
	require.NoError(t, err)
	return tenant.WithinTenant(tenant.WithUnitOfWork(context.Background(), uow), id, fn)
}

// The canonical order (#6232): classes ascending, keys ascending within a
// class, held locks again at any time.
func TestOrderLockAdmitsTheCanonicalOrder(t *testing.T) {
	t.Parallel()
	err := inLedgerTransaction(t, func(ctx context.Context) error {
		for _, step := range []struct {
			class tenant.LockClass
			key   string
		}{
			{tenant.LockStaffShifts, tenant.LockIDKey(9)},
			{tenant.LockStaffShifts, tenant.LockIDKey(12)},
			{tenant.LockRecurrence, "7"},
			{tenant.LockGradeTransitions, "7"},
			{tenant.LockScheduleDay, "2026-08-31"},
			{tenant.LockScheduleDay, "2026-09-07"},
			{tenant.LockStaffShifts, tenant.LockIDKey(9)},
			{tenant.LockScheduleDay, "2026-08-31"},
			{tenant.LockScheduleDay, "2026-09-14"},
		} {
			if err := tenant.OrderLock(ctx, step.class, step.key); err != nil {
				return err
			}
		}
		return nil
	})
	require.NoError(t, err)
}

func TestOrderLockRefusesALockAgainstTheOrder(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name         string
		first, later tenant.LockClass
		firstKey     string
		laterKey     string
	}{
		{name: "earlier day after a later day", first: tenant.LockScheduleDay, firstKey: "2026-09-07", later: tenant.LockScheduleDay, laterKey: "2026-08-31"},
		{name: "staff-shift gate after a day", first: tenant.LockScheduleDay, firstKey: "2026-08-31", later: tenant.LockStaffShifts, laterKey: tenant.LockIDKey(3)},
		{name: "recurrence after a day", first: tenant.LockScheduleDay, firstKey: "2026-08-31", later: tenant.LockRecurrence, laterKey: "7"},
		{name: "lower staff after a higher staff", first: tenant.LockStaffShifts, firstKey: tenant.LockIDKey(12), later: tenant.LockStaffShifts, laterKey: tenant.LockIDKey(9)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := inLedgerTransaction(t, func(ctx context.Context) error {
				require.NoError(t, tenant.OrderLock(ctx, tc.first, tc.firstKey))
				return tenant.OrderLock(ctx, tc.later, tc.laterKey)
			})
			assert.ErrorIs(t, err, tenant.ErrLockOrder)
		})
	}
}

// A nested transaction joins the outer ledger; a new transaction behind a
// masked one starts its own.
func TestOrderLockLedgerFollowsTheOwningTransaction(t *testing.T) {
	t.Parallel()
	err := inLedgerTransaction(t, func(ctx context.Context) error {
		require.NoError(t, tenant.OrderLock(ctx, tenant.LockScheduleDay, "2026-09-07"))
		nested := tenant.WithinCurrentTenant(ctx, func(nestedCtx context.Context) error {
			return tenant.OrderLock(nestedCtx, tenant.LockScheduleDay, "2026-08-31")
		})
		require.ErrorIs(t, nested, tenant.ErrLockOrder, "the nested call shares the outer transaction's locks")
		return tenant.WithinCurrentTenant(tenant.ContextWithoutTransaction(ctx), func(ownCtx context.Context) error {
			return tenant.OrderLock(ownCtx, tenant.LockScheduleDay, "2026-08-31")
		})
	})
	require.NoError(t, err, "a separate transaction starts with no locks")
}

func TestOrderLockOutsideATransactionAdmitsEverything(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	require.NoError(t, tenant.OrderLock(ctx, tenant.LockScheduleDay, "2026-09-07"))
	require.NoError(t, tenant.OrderLock(ctx, tenant.LockStaffShifts, tenant.LockIDKey(1)))
}
