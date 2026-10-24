package compose_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"
	calModels "github.com/moto-nrw/project-phoenix/modules/appointments"
	calendarSvc "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func reminderOutboxFixture(t *testing.T, transport func() (bool, error), clocks ...func() time.Time) (*bun.DB, context.Context, func(context.Context, time.Time, time.Time) (int, func(context.Context) error, error), func(context.Context) error, time.Time, time.Time) {
	t.Helper()
	testpkg.SetupIsolatedTestDB(t)
	db, module := setupCalendarModule(t)
	_, organizer := testpkg.CreateTestCalendarStaff(t, db, "Outbox", "Reminder")
	parent := testpkg.CreateTestParentGuardianChain(t, db)
	ctx := calendarContext(t, organizer.ID)
	date := portalDate(2026, 9, 4)
	_, err := module.Calendar.CreateStaffAppointment(ctx, calendarSvc.CreateAppointmentRequest{
		Title: "Durable reminder", StartDate: date, EndDate: date, StartTime: wallClock(18, 0), EndTime: wallClock(19, 0),
		DeliveryMode: calModels.DeliveryModeInformational, SendEmail: true,
		Targets: []calendarSvc.AppointmentTarget{{Type: calModels.TargetTypeGuardianProfile, ID: &parent.GuardianProfileID}},
	})
	require.NoError(t, err)
	effects := module.Calendar.ReminderEffects()
	if len(clocks) > 0 {
		effects.Now = clocks[0]
	}
	effects.Email = nil
	effects.FilterPush = func(_ context.Context, ids []int64) ([]int64, error) { return ids, nil }
	effects.Push = func(context.Context, int64, []int64, []int64, string) (bool, error) {
		return transport()
	}
	prepare, worker := api.NewCalendarReminderOutboxFixture(testpkg.NewDatabase(t, db), reminderEffectsSource{effects})
	starts := berlinInstant(t, date, 18, 0)
	return db, ctx, prepare, worker, starts.Add(-5 * time.Minute), starts.Add(5 * time.Minute)
}

func reminderPushRows(t *testing.T, db bun.IDB, ctx context.Context) int {
	t.Helper()
	count, err := db.NewSelect().TableExpr("platform.push_outbox").Where("tenant_id = ?", testpkg.Tenant(t)).Where("kind = ?", "parent_appointment_reminder").Count(ctx)
	require.NoError(t, err)
	return int(count)
}

func TestReminderOutboxJoinsTriggeringTransaction(t *testing.T) {
	t.Parallel()
	db, ctx, preparation, _, from, to := reminderOutboxFixture(t, func() (bool, error) { t.Fatal("transport before commit"); return false, nil })
	rollback := errors.New("rollback reminder preparation")
	err := testpkg.WithinTenantTransaction(ctx, db, testpkg.Tenant(t), func(txCtx context.Context, tx bun.Tx) error {
		_, _, err := preparation(txCtx, from, to)
		require.NoError(t, err)
		assert.Equal(t, 1, reminderPushRows(t, tx, txCtx))
		return rollback
	})
	require.ErrorIs(t, err, rollback)
	assert.Zero(t, reminderPushRows(t, db, ctx))
	require.NoError(t, testpkg.WithinTenantTransaction(ctx, db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		_, _, err := preparation(txCtx, from, to)
		return err
	}))
	assert.Equal(t, 1, reminderPushRows(t, db, ctx))
}

func TestReminderOutboxTransportFailureIsTerminalAndManualRetryDoesNotDuplicate(t *testing.T) {
	t.Parallel()
	failure := errors.New("push transport failed")
	calls := 0
	db, ctx, preparation, worker, from, to := reminderOutboxFixture(t, func() (bool, error) { calls++; return false, failure })
	var send func(context.Context) error
	prepare := func() {
		require.NoError(t, testpkg.WithinTenantTransaction(ctx, db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
			var err error
			_, send, err = preparation(txCtx, from, to)
			return err
		}))
		require.NotNil(t, send)
	}
	prepare()
	require.ErrorIs(t, send(ctx), failure)
	var state string
	require.NoError(t, db.NewRaw(`SELECT status FROM platform.push_outbox WHERE tenant_id = ? AND kind = ?`, testpkg.Tenant(t), "parent_appointment_reminder").Scan(ctx, &state))
	assert.Equal(t, "dead_letter", state)
	err := worker(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, calls, "terminal entries have no automatic retry")
	for range 2 {
		prepare()
		require.Error(t, send(ctx))
	}
	assert.Equal(t, 1, calls, "the same idempotency key never invokes the transport twice")
	assert.Equal(t, 1, reminderPushRows(t, db, ctx))
}

func TestReminderOutboxWorkerRecoversCrashAfterCommit(t *testing.T) {
	t.Parallel()
	for _, claimed := range []bool{false, true} {
		t.Run(map[bool]string{false: "before_claim", true: "expired_lease"}[claimed], func(t *testing.T) {
			testpkg.OwnTenant(t)
			calls := 0
			db, ctx, preparation, worker, from, to := reminderOutboxFixture(t, func() (bool, error) { calls++; return true, nil })
			require.NoError(t, testpkg.WithinTenantTransaction(ctx, db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
				_, _, err := preparation(txCtx, from, to)
				return err
			}))
			assert.Equal(t, 1, reminderPushRows(t, db, ctx))
			if claimed {
				_, err := db.NewRaw(`UPDATE platform.push_outbox SET status = 'claimed', lease_token = 'crashed-holder', lease_expires_at = ? WHERE tenant_id = ? AND kind = ?`, time.Now().Add(-time.Minute), testpkg.Tenant(t), "parent_appointment_reminder").Exec(ctx)
				require.NoError(t, err)
			}
			err := worker(ctx)
			require.NoError(t, err)
			assert.Equal(t, 1, calls)
			var state string
			require.NoError(t, db.NewRaw(`SELECT status FROM platform.push_outbox WHERE tenant_id = ? AND kind = ?`, testpkg.Tenant(t), "parent_appointment_reminder").Scan(ctx, &state))
			assert.Equal(t, "sent", state)
		})
	}
}

func TestReminderOutboxDefersWhenNoTransportWasAttempted(t *testing.T) {
	t.Parallel()
	calls := 0
	db, ctx, preparation, _, from, to := reminderOutboxFixture(t, func() (bool, error) { calls++; return calls > 1, nil })
	for range 2 {
		var send func(context.Context) error
		require.NoError(t, testpkg.WithinTenantTransaction(ctx, db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
			var err error
			_, send, err = preparation(txCtx, from, to)
			return err
		}))
		require.NotNil(t, send)
		require.NoError(t, send(ctx))
	}
	assert.Equal(t, 2, calls)
	assert.Equal(t, 1, reminderPushRows(t, db, ctx))
	var state string
	require.NoError(t, db.NewRaw(`SELECT status FROM platform.push_outbox WHERE tenant_id = ? AND kind = ?`, testpkg.Tenant(t), "parent_appointment_reminder").Scan(ctx, &state))
	assert.Equal(t, "sent", state)
}

func TestReminderOutboxWorkerCancelsExpiredOccurrence(t *testing.T) {
	t.Parallel()
	now := berlinInstant(t, portalDate(2026, 9, 4), 17, 55)
	calls := 0
	db, ctx, prepare, worker, from, to := reminderOutboxFixture(t, func() (bool, error) { calls++; return true, nil }, func() time.Time { return now })
	require.NoError(t, testpkg.WithinTenantTransaction(ctx, db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		_, _, err := prepare(txCtx, from, to)
		return err
	}))
	now = berlinInstant(t, portalDate(2026, 9, 4), 18, 0)
	require.NoError(t, worker(ctx))
	assert.Zero(t, calls, "a recovered reminder must not announce an occurrence that has started")
	var state string
	require.NoError(t, db.NewRaw(`SELECT status FROM platform.push_outbox WHERE tenant_id = ? AND kind = ?`, testpkg.Tenant(t), "parent_appointment_reminder").Scan(ctx, &state))
	assert.Equal(t, "cancelled", state)
}
