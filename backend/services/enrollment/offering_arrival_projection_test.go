package enrollment_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	educationModel "github.com/moto-nrw/project-phoenix/models/education"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/services/config/configtest"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// bookingModeArrivalBaseline mirrors projectedPickupReader: the arrival
// projection with enrollment.bookings_authoritative switched on, so the
// approved booking links decide which weekdays a child is in care (#2414).
func bookingModeArrivalBaseline(t *testing.T, env *decisionTestEnv, authoritative bool) scheduleService.ArrivalBaselineReader {
	t.Helper()
	settings := &configtest.Mock{
		ResolveBoolFn: func(_ context.Context, key string) (bool, error) {
			if key == configModel.KeyEnrollmentBookingsAuthoritative {
				return authoritative, nil
			}
			return false, nil
		},
	}
	return scheduleService.NewArrivalBaselineService(
		env.repos.StudentArrivalSchedule,
		env.repos.Student,
		env.repos.ClassArrivalTime,
		env.repos.RequestChildOffering,
		env.repos.CareOffering,
		settings,
	)
}

func createArrivalOffering(t *testing.T, env *decisionTestEnv, name string, days []string) *enrollmentModels.CareOffering {
	t.Helper()
	offering := &enrollmentModels.CareOffering{
		PhaseID:        env.sourcePhase.ID,
		Name:           uniqueSchemaName(name + "-" + t.Name()),
		DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:  days,
		IsActive:       true,
		CountsAsCare:   true,
	}
	offering.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, env.repos.CareOffering.Create(testpkg.Ctx(t), offering))
	return offering
}

func setArrivalClassTimes(t *testing.T, env *decisionTestEnv, class string, times map[string]string) {
	t.Helper()
	row := &educationModel.ClassArrivalTime{SchoolClass: class, ArrivalTimes: times}
	row.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, row.Validate())
	require.NoError(t, env.repos.ClassArrivalTime.Upsert(testpkg.Ctx(t), row))
}

func setStudentClass(t *testing.T, env *decisionTestEnv, studentID int64, class string) {
	t.Helper()
	ctx := testpkg.Ctx(t)
	student, err := env.repos.Student.FindByID(ctx, studentID)
	require.NoError(t, err)
	student.SchoolClass = class
	require.NoError(t, env.repos.Student.Update(ctx, student))
}

func TestArrivalProjection_BookingModeLimitsCareDaysToBookedWeekdays(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	setSourcePhaseServiceStartDate(t, env, timezone.TodayDate().AddDays(-30))
	ctx := testpkg.Ctx(t)

	offering := createArrivalOffering(t, env, "ankunft-montags", []string{"mon"})
	studentID, _ := submitAndApproveOfferingChild(
		t, env, offering.ID, "ankunft-montags@example.com", "Mona", 2,
	)
	setStudentClass(t, env, studentID, "3b")
	setArrivalClassTimes(t, env, "3b", map[string]string{"mon": "11:45", "tue": "11:45"})

	monday := nextWeekday(timezone.TodayDate(), time.Monday)
	baseline := bookingModeArrivalBaseline(t, env, true)
	projection, err := baseline.Project(ctx, []int64{studentID}, monday, monday.AddDays(1))
	require.NoError(t, err)

	t.Run("the booked weekday carries the class time", func(t *testing.T) {
		row := projection.ForDate(studentID, monday)
		require.NotNil(t, row)
		assert.Equal(t, "11:45", row.ExpectedArrival.Format("15:04"))
	})

	t.Run("an unbooked weekday carries nothing even though the class plans it", func(t *testing.T) {
		assert.Nil(t, projection.ForDate(studentID, monday.AddDays(1)))
	})

	t.Run("with the booking mode off the stored care days decide instead", func(t *testing.T) {
		classOnly := bookingModeArrivalBaseline(t, env, false)
		fallback, fallbackErr := classOnly.Project(ctx, []int64{studentID}, monday, monday.AddDays(1))
		require.NoError(t, fallbackErr)
		assert.Nil(t, fallback.ForDate(studentID, monday),
			"without stored care days and without the booking mode nothing is planned")

		staff := testpkg.CreateTestStaff(t, env.db, "Betreuung", "Fallback")
		testpkg.CreateTestArrivalSchedule(t, env.db, studentID, scheduleModels.WeekdayTuesday, staff.ID, "")
		withRow, rowErr := classOnly.Project(ctx, []int64{studentID}, monday, monday.AddDays(1))
		require.NoError(t, rowErr)
		require.NotNil(t, withRow.ForDate(studentID, monday.AddDays(1)))
		assert.Equal(t, "11:45", withRow.ForDate(studentID, monday.AddDays(1)).ExpectedArrival.Format("15:04"))
	})
}

// TestArrivalProjection_StaleRowOnUnbookedDayIsIgnored reproduces the OGS am
// Berg incident of 19.08.: two deregistered children kept arrival rows Mo-Fr
// and stayed "expected". With the booking mode on those rows no longer plan
// anything, and nothing has to be deleted from the database by hand.
func TestArrivalProjection_StaleRowOnUnbookedDayIsIgnored(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	setSourcePhaseServiceStartDate(t, env, timezone.TodayDate().AddDays(-30))
	ctx := testpkg.Ctx(t)

	offering := createArrivalOffering(t, env, "ankunft-altzeile", []string{"mon"})
	studentID, _ := submitAndApproveOfferingChild(
		t, env, offering.ID, "ankunft-altzeile@example.com", "Alt", 2,
	)
	setStudentClass(t, env, studentID, "3b")
	setArrivalClassTimes(t, env, "3b", map[string]string{"mon": "11:45", "thu": "11:45"})

	staff := testpkg.CreateTestStaff(t, env.db, "Betreuung", "Altzeile")
	testpkg.CreateTestArrivalSchedule(t, env.db, studentID, scheduleModels.WeekdayThursday, staff.ID, "11:45")

	monday := nextWeekday(timezone.TodayDate(), time.Monday)
	thursday := monday.AddDays(3)

	baseline := bookingModeArrivalBaseline(t, env, true)
	projection, err := baseline.Project(ctx, []int64{studentID}, monday, thursday)
	require.NoError(t, err)

	assert.NotNil(t, projection.ForDate(studentID, monday), "the booked day still plans")
	assert.Nil(t, projection.ForDate(studentID, thursday),
		"a stored row on an unbooked weekday must not make the child expected")

	stored, err := env.repos.StudentArrivalSchedule.FindByStudentID(ctx, studentID)
	require.NoError(t, err)
	assert.Len(t, stored, 1, "the stale row is ignored on read, never deleted behind the school's back")
}

func TestArrivalProjection_BookingEndStopsTheArrival(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	setSourcePhaseServiceStartDate(t, env, timezone.TodayDate().AddDays(-30))
	ctx := testpkg.Ctx(t)

	offering := createArrivalOffering(t, env, "ankunft-abmeldung", []string{"mon"})
	studentID, childID := submitAndApproveOfferingChild(
		t, env, offering.ID, "ankunft-abmeldung@example.com", "Enno", 2,
	)
	setStudentClass(t, env, studentID, "4a")
	setArrivalClassTimes(t, env, "4a", map[string]string{"mon": "12:45"})

	firstMonday := nextWeekday(timezone.TodayDate().AddDays(1), time.Monday)
	secondMonday := firstMonday.AddDays(7)

	// Abmeldung: the booking stops at the second Monday (half-open window).
	_, err := env.db.NewUpdate().
		Model((*enrollmentModels.RequestChildOffering)(nil)).
		ModelTableExpr(`enrollment.request_child_offerings AS "request_child_offering"`).
		Set("valid_until = ?", secondMonday).
		Where(`"request_child_offering".request_child_id = ?`, childID).
		Where(`"request_child_offering".care_offering_id = ?`, offering.ID).
		Exec(ctx)
	require.NoError(t, err)

	baseline := bookingModeArrivalBaseline(t, env, true)
	projection, projectErr := baseline.Project(ctx, []int64{studentID}, firstMonday, secondMonday)
	require.NoError(t, projectErr)

	t.Run("before the end date the arrival still stands", func(t *testing.T) {
		require.NotNil(t, projection.ForDate(studentID, firstMonday))
	})

	t.Run("from the end date on the arrival is gone without any cleanup job", func(t *testing.T) {
		assert.Nil(t, projection.ForDate(studentID, secondMonday))
	})
}

// TestArrivalProjection_StaleRowNoLongerMarksTheChildExpected closes the loop
// on the 19.08. incident through the surface that actually caused it: the
// care-day derivation behind the Erwartet-Status, not just the projection.
func TestArrivalProjection_StaleRowNoLongerMarksTheChildExpected(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	setSourcePhaseServiceStartDate(t, env, timezone.TodayDate().AddDays(-30))
	ctx := testpkg.Ctx(t)

	offering := createArrivalOffering(t, env, "erwartet-status", []string{"mon"})
	studentID, _ := submitAndApproveOfferingChild(
		t, env, offering.ID, "erwartet-status@example.com", "Erwin", 2,
	)
	setStudentClass(t, env, studentID, "3b")
	setArrivalClassTimes(t, env, "3b", map[string]string{"mon": "11:45", "thu": "11:45"})

	// The child is booked for Monday only, but a row from before the
	// Abmeldung still sits on Thursday.
	staff := testpkg.CreateTestStaff(t, env.db, "Betreuung", "Erwartet")
	testpkg.CreateTestArrivalSchedule(t, env.db, studentID, scheduleModels.WeekdayThursday, staff.ID, "11:45")

	careDays := scheduleService.NewCareDayService(scheduleService.CareDayDependencies{
		ArrivalBaselines:  bookingModeArrivalBaseline(t, env, true),
		ArrivalSchedules:  env.repos.StudentArrivalSchedule,
		ArrivalExceptions: env.repos.StudentArrivalException,
		PickupBaselines: scheduleService.NewPickupBaselineService(
			env.repos.StudentPickupSchedule,
			env.repos.RequestChildOffering,
			env.repos.CareOffering,
		),
		PickupExceptions: env.repos.StudentPickupException,
	})

	monday := nextWeekday(timezone.TodayDate(), time.Monday)
	thursday := monday.AddDays(3)

	booked, err := careDays.ResolveForDate(ctx, []int64{studentID}, monday)
	require.NoError(t, err)
	assert.True(t, booked[studentID].Expected(), "the booked day still expects the child")

	stale, err := careDays.ResolveForDate(ctx, []int64{studentID}, thursday)
	require.NoError(t, err)
	assert.False(t, stale[studentID].Expected(),
		"a stale arrival row on an unbooked weekday must not keep a deregistered child expected")
}
