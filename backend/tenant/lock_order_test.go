package tenant_test

import (
	"context"
	"errors"
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
// Staff rows come after the days, ascending (#6583, #6702); leaf locks come
// last in any key order.
func TestOrderLockAdmitsTheCanonicalOrder(t *testing.T) {
	t.Parallel()
	err := inLedgerTransaction(t, func(ctx context.Context) error {
		for _, step := range []struct {
			class tenant.LockClass
			key   string
		}{
			{tenant.LockAccount, tenant.LockIDKey(4)},
			{tenant.LockAccount, tenant.LockIDKey(5)},
			{tenant.LockStudentClassWrites, tenant.LockIDKey(7)},
			{tenant.LockPayrollRules, tenant.LockIDKey(7)},
			{tenant.LockStaffWorkTime, tenant.StaffWorkTimeKey(9, false)},
			{tenant.LockStaffWorkTime, tenant.StaffWorkTimeKey(9, true)},
			{tenant.LockStaffWorkTime, tenant.StaffWorkTimeKey(12, false)},
			{tenant.LockStaffShifts, tenant.LockIDKey(9)},
			{tenant.LockStaffShifts, tenant.LockIDKey(12)},
			{tenant.LockRecurrence, "7"},
			{tenant.LockGradeTransitions, "7"},
			{tenant.LockReference, "schedule.planning_tracks"},
			{tenant.LockReference, "activities.categories"},
			{tenant.LockScheduleDay, "2026-08-31"},
			{tenant.LockScheduleDay, "2026-09-07"},
			{tenant.LockStaffShifts, tenant.LockIDKey(9)},
			{tenant.LockScheduleDay, "2026-08-31"},
			{tenant.LockScheduleDay, "2026-09-14"},
			{tenant.LockStaffPersonMerge, tenant.LockIDKey(7)},
			{tenant.LockStaffRows, tenant.LockIDKey(9)},
			{tenant.LockStaffRows, tenant.LockIDKey(12)},
			{tenant.LockScheduleDay, "2026-09-07"},
			{tenant.LockStaffRows, tenant.LockIDKey(9)},
			{tenant.LockLeaf, "schedule.activity_instances"},
			{tenant.LockLeaf, "activities.groups"},
			{tenant.LockStaffRows, tenant.LockIDKey(12)},
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
		{name: "day after a staff row", first: tenant.LockStaffRows, firstKey: tenant.LockIDKey(3), later: tenant.LockScheduleDay, laterKey: "2026-08-31"},
		{name: "staff-shift gate after a staff row", first: tenant.LockStaffRows, firstKey: tenant.LockIDKey(3), later: tenant.LockStaffShifts, laterKey: tenant.LockIDKey(3)},
		{name: "lower staff after a higher staff", first: tenant.LockStaffShifts, firstKey: tenant.LockIDKey(12), later: tenant.LockStaffShifts, laterKey: tenant.LockIDKey(9)},
		{name: "work-time gate after a staff-shift gate", first: tenant.LockStaffShifts, firstKey: tenant.LockIDKey(3), later: tenant.LockStaffWorkTime, laterKey: tenant.StaffWorkTimeKey(3, false)},
		{name: "payroll rules after a work-time gate", first: tenant.LockStaffWorkTime, firstKey: tenant.StaffWorkTimeKey(3, false), later: tenant.LockPayrollRules, laterKey: tenant.LockIDKey(7)},
		{name: "balance after the same person's absence gate", first: tenant.LockStaffWorkTime, firstKey: tenant.StaffWorkTimeKey(3, true), later: tenant.LockStaffWorkTime, laterKey: tenant.StaffWorkTimeKey(3, false)},
		{name: "lower person's work time after a higher one", first: tenant.LockStaffWorkTime, firstKey: tenant.StaffWorkTimeKey(12, false), later: tenant.LockStaffWorkTime, laterKey: tenant.StaffWorkTimeKey(9, true)},
		{name: "merge gate after a staff row", first: tenant.LockStaffRows, firstKey: tenant.LockIDKey(3), later: tenant.LockStaffPersonMerge, laterKey: tenant.LockIDKey(7)},
		{name: "catalogue row after a day", first: tenant.LockScheduleDay, firstKey: "2026-08-31", later: tenant.LockReference, laterKey: "activities.categories"},
		{name: "recurrence after a catalogue row", first: tenant.LockReference, firstKey: "activities.categories", later: tenant.LockRecurrence, laterKey: "7"},
		{name: "account after a work-time gate", first: tenant.LockStaffWorkTime, firstKey: tenant.StaffWorkTimeKey(3, false), later: tenant.LockAccount, laterKey: tenant.LockIDKey(4)},
		{name: "class-write gate after a work-time gate", first: tenant.LockStaffWorkTime, firstKey: tenant.StaffWorkTimeKey(3, false), later: tenant.LockStudentClassWrites, laterKey: tenant.LockIDKey(7)},
		{name: "lower staff row after a higher staff row", first: tenant.LockStaffRows, firstKey: tenant.LockIDKey(12), later: tenant.LockStaffRows, laterKey: tenant.LockIDKey(9)},
		{name: "staff row after a leaf", first: tenant.LockLeaf, firstKey: "schedule.activity_instances", later: tenant.LockStaffRows, laterKey: tenant.LockIDKey(3)},
		{name: "day after a leaf", first: tenant.LockLeaf, firstKey: "workforce.staff_absences", later: tenant.LockScheduleDay, laterKey: "2026-08-31"},
		{name: "staff-shift gate after a leaf", first: tenant.LockLeaf, firstKey: "workforce.staff_absences", later: tenant.LockStaffShifts, laterKey: tenant.LockIDKey(3)},
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

func TestOrderLockRestoresSavepointLocks(t *testing.T) {
	t.Parallel()
	operationErr := errors.New("completion failed")
	err := inLedgerTransaction(t, func(ctx context.Context) error {
		require.NoError(t, tenant.OrderLock(ctx, tenant.LockStaffShifts, tenant.LockIDKey(9)))
		err := tenant.WithSavepoint(ctx, func(ctx context.Context) error {
			require.NoError(t, tenant.OrderLock(ctx, tenant.LockScheduleDay, "2026-08-31"))
			return operationErr
		})
		require.ErrorIs(t, err, operationErr)
		// The rolled-back day must neither advance the frontier nor count as held.
		require.NoError(t, tenant.OrderLock(ctx, tenant.LockStaffShifts, tenant.LockIDKey(12)))
		require.NoError(t, tenant.OrderLock(ctx, tenant.LockScheduleDay, "2026-09-07"))
		require.ErrorIs(t, tenant.OrderLock(ctx, tenant.LockScheduleDay, "2026-08-31"), tenant.ErrLockOrder)
		// Locks from before the savepoint remain held.
		return tenant.OrderLock(ctx, tenant.LockStaffShifts, tenant.LockIDKey(9))
	})
	require.NoError(t, err)
}

func TestOrderLockNestedSavepointsRestoreTheirOwnLocks(t *testing.T) {
	t.Parallel()
	operationErr := errors.New("completion failed")
	err := inLedgerTransaction(t, func(ctx context.Context) error {
		err := tenant.WithSavepoint(ctx, func(ctx context.Context) error {
			require.NoError(t, tenant.WithSavepoint(ctx, func(ctx context.Context) error {
				return tenant.OrderLock(ctx, tenant.LockScheduleDay, "2026-08-31")
			}))
			err := tenant.WithSavepoint(ctx, func(ctx context.Context) error {
				require.NoError(t, tenant.OrderLock(ctx, tenant.LockScheduleDay, "2026-09-14"))
				return operationErr
			})
			require.ErrorIs(t, err, operationErr)
			require.NoError(t, tenant.OrderLock(ctx, tenant.LockScheduleDay, "2026-09-07"))
			require.ErrorIs(t, tenant.OrderLock(ctx, tenant.LockRecurrence, "7"), tenant.ErrLockOrder)
			return operationErr
		})
		require.ErrorIs(t, err, operationErr)
		// The outer rollback also releases locks kept by the successful inner savepoint.
		return tenant.OrderLock(ctx, tenant.LockStaffShifts, tenant.LockIDKey(9))
	})
	require.NoError(t, err)
}

func TestHoldsLockReportsTheLedger(t *testing.T) {
	t.Parallel()
	assert.False(t, tenant.HoldsLock(context.Background(), tenant.LockStaffRows, tenant.LockIDKey(3)), "outside a transaction nothing is held")
	err := inLedgerTransaction(t, func(ctx context.Context) error {
		assert.False(t, tenant.HoldsLock(ctx, tenant.LockStaffRows, tenant.LockIDKey(3)))
		require.NoError(t, tenant.OrderLock(ctx, tenant.LockStaffRows, tenant.LockIDKey(3)))
		assert.True(t, tenant.HoldsLock(ctx, tenant.LockStaffRows, tenant.LockIDKey(3)))
		assert.False(t, tenant.HoldsLock(ctx, tenant.LockStaffRows, tenant.LockIDKey(4)))
		assert.False(t, tenant.HoldsLock(ctx, tenant.LockStaffShifts, tenant.LockIDKey(3)), "the class is part of the key")
		return nil
	})
	require.NoError(t, err)
}

// A row the transaction inserted is held without an order position: locking
// it later never waits, so even a staff row after a leaf is admitted, while a
// staff row the transaction did not insert still follows the order (#6702).
func TestHoldInsertedAdmitsTheOwnRowOutOfOrder(t *testing.T) {
	t.Parallel()
	err := inLedgerTransaction(t, func(ctx context.Context) error {
		require.NoError(t, tenant.OrderLock(ctx, tenant.LockLeaf, "users.persons"))
		tenant.HoldInserted(ctx, tenant.LockStaffRows, tenant.LockIDKey(30))
		require.NoError(t, tenant.OrderLock(ctx, tenant.LockStaffRows, tenant.LockIDKey(30)))
		return tenant.OrderLock(ctx, tenant.LockStaffRows, tenant.LockIDKey(40))
	})
	require.ErrorIs(t, err, tenant.ErrLockOrder)
}
