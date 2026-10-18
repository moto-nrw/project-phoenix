package appointments_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/appointments"
	"github.com/moto-nrw/project-phoenix/modules/appointments/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestPublicAppointmentContractPersistsNormalizedFieldsAndTargets(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
	require.NoError(t, err)
	staff := testpkg.CreateTestStaff(t, db, "Appointment", "Organizer")
	day := appointments.NewDate(2026, 8, 24)
	created, targets, err := module.CreateAppointment(testpkg.Ctx(t), appointments.CreateAppointment{
		AppointmentFields: appointments.AppointmentFields{OrganizerStaffID: staff.ID, Title: "  Teamtreffen  ", StartDate: day, EndDate: day, StartTime: testpkg.WallClock(9, 0), EndTime: testpkg.WallClock(10, 0), DeliveryMode: appointments.DeliveryModeInformational},
		Targets:           []appointments.AppointmentTargetFields{{TargetType: appointments.TargetTypeAllStaff}},
	})
	require.NoError(t, err)
	require.Positive(t, created.ID)
	require.Equal(t, testpkg.Tenant(t), created.TenantID)
	require.Equal(t, "Teamtreffen", created.Title)
	require.Equal(t, day, created.StartDate)
	require.Len(t, targets, 1)
	require.Equal(t, created.ID, targets[0].AppointmentID)
	found, err := module.FindAppointment(testpkg.Ctx(t), created.ID)
	require.NoError(t, err)
	require.Equal(t, created, found)
	actualTargets, err := module.FindAppointmentTargets(testpkg.Ctx(t), created.ID)
	require.NoError(t, err)
	require.Equal(t, targets, actualTargets)
}
