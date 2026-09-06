package compose_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	reminder "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPickupQueriesPreserveResolvedClockAndDisplayIdentity(t *testing.T) {
	t.Parallel()
	// A fixed weekday exercises pickup resolution even when the suite runs on weekends.
	today := testpkg.Date(2026, time.September, 4)
	now := today.BerlinMidnight().Add(12 * time.Hour)
	db, module := testutil.SetupRemindersModule(t, func() time.Time { return now })
	ctx := testpkg.Ctx(t)
	for _, key := range []string{"reminders.pickup_upcoming_enabled", "reminders.pickup_overdue_enabled"} {
		require.NoError(t, module.Settings.SetValue(ctx, key, true, nil, nil))
	}
	staff := testpkg.CreateTestStaff(t, db, "Reminder", "Staff")
	device := testpkg.CreateTestDevice(t, db, "reminder-pickup-query")
	student := testpkg.CreateTestStudent(t, db, "Anna", "Müller", "3b")
	testpkg.CreateTestAttendanceForDate(t, db, student.ID, staff.ID, device.ID, today, today.BerlinMidnight(), nil)
	// Midnight is either upcoming or overdue throughout this calendar day.
	// Enabling both states keeps the projection assertion independent of time.
	testpkg.CreateTestPickupException(t, db, student.ID, today, staff.ID, "00:00", "fixture")

	require.NoError(t, testpkg.WithinTenantContext(t, ctx, db, testpkg.Tenant(t), func(ctx context.Context) error {
		single, err := module.Reminders.Compute(ctx, reminder.Scope{IsAdmin: true})
		require.NoError(t, err)
		require.Len(t, single.Reminders, 1)
		actual := single.Reminders[0]
		require.NotNil(t, actual.StudentID)
		assert.Equal(t, strconv.FormatInt(student.ID, 10), *actual.StudentID)
		assert.Equal(t, "Anna Müller", actual.Title)
		assert.Equal(t, "3b", actual.Subtitle)
		assert.Equal(t, "00:00", actual.DueTime)

		batch, err := module.Reminders.ComputeBatch(ctx, []reminder.BatchScope{{Scope: reminder.Scope{IsAdmin: true, StaffID: staff.ID}}})
		require.NoError(t, err)
		require.Contains(t, batch, staff.ID)
		assert.Equal(t, single.Reminders, batch[staff.ID].Reminders)
		return nil
	}))
}
