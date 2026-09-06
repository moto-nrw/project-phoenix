package calendar_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	calModels "github.com/moto-nrw/project-phoenix/models/calendar"
	calendarSvc "github.com/moto-nrw/project-phoenix/services/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCalendarTargetResolutionQueryBudget(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db := testpkg.SetupTestDB(t)
	service := setupCalendarServiceWithOutbox(t, db, &recordingOutbox{})
	_, organizer := testpkg.CreateTestCalendarStaff(t, db, "TargetBudget", "Organizer")
	ctx := calendarContext(t, organizer.ID)
	targets := make([]calendarSvc.AppointmentTarget, 0, 8)
	for range 8 {
		chain := testpkg.CreateTestParentGuardianChain(t, db)
		targets = append(targets, calendarSvc.AppointmentTarget{Type: calModels.TargetTypeGuardianProfile, ID: &chain.GuardianProfileID})
	}
	counter := testpkg.CaptureQueries(t, db)
	run := func(n int) []string {
		counter.Reset()
		_, err := service.CreateStaffAppointment(ctx, calendarSvc.CreateAppointmentRequest{
			Title:        fmt.Sprintf("Target budget %d", n),
			StartDate:    testpkg.Date(2026, 10, 12),
			EndDate:      testpkg.Date(2026, 10, 12),
			StartTime:    wallClock(14, 0),
			EndTime:      wallClock(15, 0),
			DeliveryMode: calModels.DeliveryModeInformational,
			Targets:      targets[:n],
		})
		require.NoError(t, err)
		return counter.Operation("SELECT")
	}

	small := run(3)
	large := run(8)
	t.Logf("query budget: 3 targets → %d reads, 8 targets → %d reads", len(small), len(large))
	assert.Equal(t, len(small), len(large))
	testpkg.AssertQueryBudget(t, "services.calendar.resolve_targets.reads", large)
}

func TestCalendarReminderScanQueryBudget(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db := testpkg.SetupTestDB(t)
	outbox := &recordingOutbox{}
	service := setupCalendarServiceWithOutbox(t, db, outbox)
	_, organizer := testpkg.CreateTestCalendarStaff(t, db, "ReminderBudget", "Organizer")
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	ctx := calendarContext(t, organizer.ID)
	date := testpkg.Date(2026, 10, 13)
	startsAt := berlinInstant(t, date, 14, 0)
	created := 0
	addAppointments := func(n int) {
		for range n {
			_, err := service.CreateStaffAppointment(ctx, calendarSvc.CreateAppointmentRequest{
				Title:        fmt.Sprintf("Reminder budget %d", created),
				StartDate:    date,
				EndDate:      date,
				StartTime:    wallClock(14, 0),
				EndTime:      wallClock(15, 0),
				DeliveryMode: calModels.DeliveryModeInformational,
				SendEmail:    true,
				Targets:      []calendarSvc.AppointmentTarget{{Type: calModels.TargetTypeGuardianProfile, ID: &chain.GuardianProfileID}},
			})
			require.NoError(t, err)
			created++
		}
	}
	counter := testpkg.CaptureQueries(t, db)
	run := func(expectedQueued int) []string {
		counter.Reset()
		queued, err := testutil.ComposeCalendarReminderCommand(db, service).EnqueueDueAppointmentReminders(ctx, startsAt.Add(-5*time.Minute), startsAt.Add(5*time.Minute))
		require.NoError(t, err)
		require.Equal(t, expectedQueued, queued)
		return counter.Operation("SELECT")
	}

	addAppointments(3)
	small := run(3)
	addAppointments(5)
	large := run(5)
	t.Logf("query budget: 3 reminders → %d reads, 8 reminders → %d reads", len(small), len(large))
	assert.Equal(t, len(small), len(large))
	testpkg.AssertQueryBudget(t, "services.calendar.reminder_scan.reads", large)
}
