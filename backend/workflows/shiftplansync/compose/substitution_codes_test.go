package compose_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	substitution "github.com/moto-nrw/project-phoenix/modules/schoolstructure/contract"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// TestScheduleSubstitutionExternalInterfaceKeepsRefusalCodes pins that a
// refused schedule substitution reaches /api/substitutions with the
// Vertretungsplan's own code, the request field and the values it names,
// instead of the folded invalid_target (#2516).
func TestScheduleSubstitutionExternalInterfaceKeepsRefusalCodes(t *testing.T) {
	t.Parallel()
	db, _, service := newScheduleSubstitutionModule(t)

	room := testpkg.CreateTestRoom(t, db, "Fehlercodes")
	absent := testpkg.CreateTestStaff(t, db, "Ida", "Abwesend")
	replacement := testpkg.CreateTestStaff(t, db, "Jan", "Vertretung")
	firstDate := timezone.NewDate(2030, 8, 27)
	secondDate := firstDate.AddDays(1)
	first := testpkg.CreateTestActivityInstance(t, db, firstDate, room.ID, testpkg.ActivityInstanceOpts{Title: "Erster Tag"})
	second := testpkg.CreateTestActivityInstance(t, db, secondDate, room.ID, testpkg.ActivityInstanceOpts{Title: "Zweiter Tag"})
	testpkg.CreateTestInstanceStaff(t, db, first.ID, absent.ID, testpkg.InstanceStaffOpts{})
	testpkg.CreateTestInstanceStaff(t, db, second.ID, absent.ID, testpkg.InstanceStaffOpts{})
	testpkg.CreateTestInstanceStaff(t, db, second.ID, replacement.ID, testpkg.InstanceStaffOpts{IsAbsent: true})
	caller := scheduleSubstitutionCaller(t)

	_, err := service.Assign(testpkg.Ctx(t), caller, substitution.Assignment{
		Type: substitution.TargetScheduleSubstitution,
		ScheduleSubstitution: &substitution.ScheduleSubstitutionAssignment{
			InstanceID: first.ID,
			Substitutions: []substitution.ScheduleSubstitutionChange{{
				AbsentStaffID: absent.ID, SubstituteStaffID: absent.ID,
			}},
		},
	})
	require.ErrorIs(t, err, substitution.ErrInvalidTarget)
	var operationError *substitution.OperationError
	require.ErrorAs(t, err, &operationError)
	require.Equal(t, "timetable.substitute_self", operationError.Code)
	require.Equal(t, "substitute_staff_id", operationError.Field)

	_, err = service.Assign(testpkg.Ctx(t), caller, substitution.Assignment{
		Type: substitution.TargetScheduleSubstitution,
		ScheduleSubstitution: &substitution.ScheduleSubstitutionAssignment{
			WholeDays: &substitution.ScheduleWholeDayAssignment{
				AbsentStaffID: absent.ID, SubstituteStaffID: &replacement.ID,
				Dates: []timezone.Date{firstDate, secondDate},
			},
		},
	})
	require.ErrorIs(t, err, substitution.ErrInvalidTarget)
	require.ErrorAs(t, err, &operationError)
	require.Equal(t, "timetable.substitute_absent_on_date", operationError.Code)
	require.Equal(t, substitution.SubstitutionRefusalValues{Date: secondDate.Format("02.01.2006")}, operationError.Details)
	require.Equal(t, "substitute_staff_id", operationError.Field)
}
