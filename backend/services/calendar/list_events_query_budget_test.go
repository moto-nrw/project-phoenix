package calendar_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	calModels "github.com/moto-nrw/project-phoenix/models/calendar"
	calendarSvc "github.com/moto-nrw/project-phoenix/services/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestListMyStaffEventsQueryBudget guards the staff calendar list against
// per-appointment N+1 regressions: the query count must not grow with the
// number of appointments in the window, and the total per call stays within
// the registered budget (#2940).
func TestListMyStaffEventsQueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)

	service := setupCalendarService(t, db)
	_, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "Calendar", "BudgetOrganizer")
	invitedStaff, invitedAccount := testpkg.CreateTestCalendarStaff(t, db, "Calendar", "BudgetInvitee")

	weekStart := timezone.NewDate(2026, 3, 2)
	created := 0
	addAppointments := func(n int) {
		for range n {
			day := weekStart.AddDays(created % 5)
			_, err := service.CreateStaffAppointment(calendarContext(t, organizerAccount.ID), calendarSvc.CreateAppointmentRequest{
				Title:        fmt.Sprintf("Budget appointment %d", created),
				StartDate:    day,
				EndDate:      day,
				StartTime:    wallClock(9+created%4, 0),
				EndTime:      wallClock(10+created%4, 0),
				DeliveryMode: calModels.DeliveryModeRSVPRequired,
				Targets: []calendarSvc.AppointmentTarget{
					{Type: calModels.TargetTypeStaff, ID: &invitedStaff.ID},
				},
			})
			require.NoError(t, err)
			created++
		}
	}

	counter := testpkg.CaptureQueries(t, db)

	run := func() int {
		counter.Reset()
		events, err := service.ListMyStaffEvents(calendarContext(t, invitedAccount.ID), weekStart, weekStart.AddDays(6))
		require.NoError(t, err)
		require.Len(t, eventDates(events, calModels.EventSourceAppointment), created)
		return counter.Total()
	}

	addAppointments(3)
	smallCount := run()

	addAppointments(5)
	largeCount := run()

	t.Logf("query budget: 3 appointments → %d queries, 8 appointments → %d queries", smallCount, largeCount)

	assert.Equal(t, smallCount, largeCount,
		"query count must be independent of the appointment count (no per-appointment N+1)")
	testpkg.AssertQueryBudget(t, "services.calendar.list_my_staff_events", counter.Queries())
}
