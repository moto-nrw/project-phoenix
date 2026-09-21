package compose_test

import (
	"testing"

	calModels "github.com/moto-nrw/project-phoenix/models/calendar"
	calendarSvc "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// The migration must retain the shared authorization helper's interpretation
// of stored JSON, including historical non-boolean values.
func TestCalendarGuardianProjectionPreservesPermissionInterpretation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	service := setupCalendarServiceWithOutbox(t, db, &recordingOutbox{})
	_, organizer := testpkg.CreateTestCalendarStaff(t, db, "Permission", "Organizer")
	ctx := calendarContext(t, organizer.ID)
	for _, tc := range []struct {
		name, json string
		granted    bool
	}{
		{"true", "true", true},
		{"false", "false", false},
		{"string_false", "\"false\"", true},
		{"zero", "0", true},
		{"object", "{}", true},
		{"null", "null", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			chain := testpkg.CreateTestParentGuardianChain(t, db)
			_, err := db.NewRaw(`UPDATE users.students_guardians SET permissions = jsonb_build_object('parent_portal.access', ?::jsonb)
				WHERE guardian_profile_id = ? AND student_id = ?`, tc.json, chain.GuardianProfileID, chain.StudentID).Exec(ctx)
			require.NoError(t, err)
			detail, err := service.CreateStaffAppointment(ctx, calendarSvc.CreateAppointmentRequest{
				Title: "Stored permission interpretation", StartDate: portalDate(2026, 10, 12), EndDate: portalDate(2026, 10, 12),
				StartTime: wallClock(14, 0), EndTime: wallClock(15, 0), DeliveryMode: calModels.DeliveryModeInformational,
				Targets: []calendarSvc.AppointmentTarget{{Type: calModels.TargetTypeGuardianProfile, ID: &chain.GuardianProfileID}},
			})
			if !tc.granted {
				require.ErrorContains(t, err, "guardian target is not portal-visible")
				return
			}
			require.NoError(t, err)
			audiences, err := service.GuardianNotificationAudiences(ctx, []int64{detail.Appointment.ID})
			require.NoError(t, err)
			require.Equal(t, []int64{chain.GuardianProfileID}, audiences[detail.Appointment.ID].GuardianIDs)
			require.Equal(t, []int64{chain.StudentID}, audiences[detail.Appointment.ID].StudentsByGuardian[chain.GuardianProfileID])
		})
	}
}
