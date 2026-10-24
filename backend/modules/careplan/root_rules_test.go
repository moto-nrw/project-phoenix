package careplan

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func TestGroupRosterLeavesPendingWithdrawalsOut(t *testing.T) {
	t.Parallel()
	assert.False(t, GroupRosterIncludesPendingWithdrawals)
}

func TestLiveFlagsForApprovedAbsence(t *testing.T) {
	t.Parallel()
	assert.Equal(t, ApprovedAbsenceLiveFlags{Sick: true, ClearExcused: true}, LiveFlagsForApprovedAbsence(StudentStatusDaySick))
	assert.Equal(t, ApprovedAbsenceLiveFlags{}, LiveFlagsForApprovedAbsence(StudentStatusDayExcused))
	assert.Equal(t, ApprovedAbsenceLiveFlags{}, LiveFlagsForApprovedAbsence(StudentStatusDayClassTrip))
}

func TestOfferingChangeQueueFilters(t *testing.T) {
	t.Parallel()
	studentID := int64(47)
	assert.Equal(t, OfferingChangeFilter{StudentID: studentID, Statuses: []string{OfferingChangePending}, Order: ChangeOrderCreated}, PendingOfferingChanges(studentID))
	assert.Equal(t, OfferingChangeFilter{Statuses: []string{OfferingChangePending}, Order: ChangeOrderCreated}, PendingOfferingChanges(0))
	assert.Equal(t, OfferingChangeFilter{StudentID: studentID, Order: ChangeOrderReviewed}, StudentOfferingChangeHistory(studentID))
}

func TestManualPlanningWindow(t *testing.T) {
	t.Parallel()
	studentID := int64(47)
	from, to, err := ManualPlanningWindow(studentID, "2026-10-19", "2026-10-19")
	require.NoError(t, err)
	assert.Equal(t, calendar.Date("2026-10-19"), from)
	assert.Equal(t, calendar.Date("2026-10-19"), to)

	_, _, err = ManualPlanningWindow(0, "2026-10-19", "2026-10-23")
	assert.EqualError(t, err, "student id must be positive")
	for _, window := range [][2]string{{"2026-10-23", "2026-10-19"}, {"", "2026-10-19"}, {"2026-10-19", "19.10.2026"}} {
		_, _, err = ManualPlanningWindow(studentID, window[0], window[1])
		assert.EqualError(t, err, "valid planning date range is required", window)
	}
}
