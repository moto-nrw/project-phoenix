package calendar

import (
	"testing"

	calModels "github.com/moto-nrw/project-phoenix/models/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppointmentRepositoryLockReminderCandidates(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	staff := testpkg.CreateTestStaff(t, db, "CalendarBatch", "Organizer")
	repo := NewAppointmentRepository(db)

	create := func(title string, notify bool) *calModels.Appointment {
		row := &calModels.Appointment{
			OrganizerStaffID:   staff.ID,
			Title:              title,
			StartDate:          testpkg.Date(2026, 10, 12),
			EndDate:            testpkg.Date(2026, 10, 12),
			StartTime:          testpkg.WallClock(14, 0),
			EndTime:            testpkg.WallClock(15, 0),
			DeliveryMode:       calModels.DeliveryModeInformational,
			OverviewVisibility: calModels.OverviewVisibilityOrganizer,
			NotifyGuardians:    notify,
		}
		row.SetTenantID(testpkg.Tenant(t))
		require.NoError(t, repo.Create(ctx, row))
		return row
	}

	first := create("Batch reminder one", true)
	second := create("Batch reminder two", true)
	optedOut := create("Batch reminder opted out", false)

	rows, err := repo.LockReminderCandidates(ctx, []int64{second.ID, optedOut.ID, first.ID})
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, []int64{first.ID, second.ID}, []int64{rows[0].ID, rows[1].ID})

	rows, err = repo.LockReminderCandidates(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, rows)
}
