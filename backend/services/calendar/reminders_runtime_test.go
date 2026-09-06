package calendar_test

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	calModels "github.com/moto-nrw/project-phoenix/models/calendar"
	calendarSvc "github.com/moto-nrw/project-phoenix/services/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// This is local flow evidence, not a replacement for accepted checkpoint #3019.
// Transport is a deterministic local sink; the real outbox and claim stores run.
func TestReminderRuntimeEvidence(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db, calendar := testutil.SetupCalendarModule(t)
	_, organizer := testpkg.CreateTestCalendarStaff(t, db, "Reminder", "Runtime")
	parent := testpkg.CreateTestParentGuardianChain(t, db)
	ctx := calendarContext(t, organizer.ID)
	date := testpkg.Date(2026, 9, 4)
	detail, err := calendar.Calendar.CreateStaffAppointment(ctx, calendarSvc.CreateAppointmentRequest{
		Title: "Runtime fixture", StartDate: date, EndDate: date,
		StartTime: wallClock(18, 0), EndTime: wallClock(19, 0),
		DeliveryMode: calModels.DeliveryModeInformational, SendEmail: true,
		Targets: []calendarSvc.AppointmentTarget{{Type: calModels.TargetTypeGuardianProfile, ID: &parent.GuardianProfileID}},
	})
	require.NoError(t, err)
	effects := calendar.Calendar.ReminderEffects()
	effects.ParentsURL = "https://parents.test"
	effects.FilterEmail = func(_ context.Context, ids []int64) ([]int64, error) { return ids, nil }
	effects.FilterPush = effects.FilterEmail
	effects.Push = func(context.Context, int64, []int64, []int64, string) (bool, error) { return true, nil }
	command := testutil.ComposeCalendarReminderCommand(db, reminderEffectsSource{effects})
	startsAt := berlinInstant(t, date, 18, 0)
	_, reminders := testutil.SetupRemindersModule(t, func() time.Time { return berlinInstant(t, date, 12, 0) })
	require.NoError(t, reminders.Settings.SetValue(testpkg.Ctx(t), "reminders.pickup_upcoming_enabled", true, nil, nil))
	for index := range 8 {
		testpkg.CreateTestRoom(t, db, fmt.Sprintf("Runtime room %d", index))
	}
	counter := testpkg.CaptureQueriesForContext(t, db)
	measuredCtx := counter.Context(ctx)
	for _, operation := range []string{"reminder-delivery.query.admin", "reminder-delivery.command.prepare"} {
		t.Run(operation, func(t *testing.T) {
			var durations []float64
			var queryCounts []int
			var poolWaitCount int64
			var poolWait time.Duration
			var stopSampling func() testpkg.RuntimeCheckpointLockSamples
			for iteration := range 35 {
				if operation == "reminder-delivery.command.prepare" {
					// A new revision gives each sample a fresh idempotency key. This
					// fixture edit is outside the measured operation and query count.
					_, err := db.NewRaw("UPDATE calendar.appointments SET revision = revision + 1 WHERE id = ? AND tenant_id = ?", detail.Appointment.ID, testpkg.Tenant(t)).Exec(ctx)
					require.NoError(t, err)
				}
				if iteration == 5 {
					stopSampling = testpkg.SampleCheckpointLocks(func(sampleCtx context.Context) (int, error) {
						var waiting int
						err := db.NewRaw("SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'").Scan(sampleCtx, &waiting)
						return waiting, err
					})
					t.Cleanup(func() { _ = stopSampling() })
				}
				counter.Reset()
				before := db.Stats()
				started := time.Now()
				if operation == "reminder-delivery.query.admin" {
					_, err = reminders.Reminders.ComputeForCaller(measuredCtx, true)
				} else {
					var queued int
					queued, err = command.EnqueueDueAppointmentReminders(measuredCtx, startsAt.Add(-5*time.Minute), startsAt.Add(5*time.Minute))
					require.Equal(t, 1, queued)
				}
				elapsed := time.Since(started)
				after := db.Stats()
				require.NoError(t, err)
				if iteration >= 5 {
					durations = append(durations, float64(elapsed)/float64(time.Millisecond))
					queryCounts = append(queryCounts, counter.Total())
					poolWaitCount += after.WaitCount - before.WaitCount
					poolWait += after.WaitDuration - before.WaitDuration
				}
			}
			locks := stopSampling()
			require.Empty(t, locks.Error)
			slices.Sort(durations)
			t.Logf("operation=%s samples=30 errors=0 queries_min=%d queries_max=%d p50_ms=%.3f p95_ms=%.3f pool_wait_count=%d pool_wait_ms=%.3f lock_samples=%d waiting_backend_samples=%d max_sample_gap_ms=%.3f",
				operation, slices.Min(queryCounts), slices.Max(queryCounts), durations[14], durations[28], poolWaitCount, float64(poolWait)/float64(time.Millisecond), locks.Samples, locks.WaitingBackendSamples, locks.MaxSampleGapMS)
		})
	}
}
