package calendar_test

import (
	"testing"

	calModels "github.com/moto-nrw/project-phoenix/models/calendar"
	calendarSvc "github.com/moto-nrw/project-phoenix/services/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCalendarClassTargetMatchesNormalizedSchoolClass(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	service := setupCalendarService(t, db)
	_, organizer := testpkg.CreateTestCalendarStaff(t, db, "Class", "Organizer")
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)
	className := " 1A "

	detail, err := service.CreateStaffAppointment(calendarContext(t, organizer.ID), calendarSvc.CreateAppointmentRequest{
		Title:        "Normalized class target",
		StartDate:    testpkg.Date(2026, 10, 14),
		EndDate:      testpkg.Date(2026, 10, 14),
		StartTime:    wallClock(14, 0),
		EndTime:      wallClock(15, 0),
		DeliveryMode: calModels.DeliveryModeInformational,
		Targets: []calendarSvc.AppointmentTarget{
			{Type: calModels.TargetTypeParentsByClass, Value: &className},
		},
	})

	require.NoError(t, err)
	assert.NotZero(t, findRecipientByGuardian(t, detail, parentChain.GuardianProfileID))
}
