package repositories_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFactory(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	owners := repositories.NewUnobservedTimetableDependencies(db)
	factory := repositories.NewFactory(db, owners)
	require.NotNil(t, factory)
	// The retained activities and schedule repositories belong to Timetable
	// and are built where they are used (#2743).
	timetable := repositories.NewTimetableRepositories(db, owners)

	// Verify users repositories are initialized
	t.Run("users repositories", func(t *testing.T) {
		assert.NotNil(t, factory.Person)
		assert.NotNil(t, factory.RFIDCard)
		assert.NotNil(t, factory.Staff)
		assert.NotNil(t, factory.Student)
		assert.NotNil(t, factory.Teacher)
		assert.NotNil(t, factory.Guest)
		assert.NotNil(t, factory.Profile)
		assert.NotNil(t, factory.StudentGuardian)
		assert.NotNil(t, factory.GuardianProfile)
	})

	// Verify facilities repositories are initialized
	t.Run("facilities repositories", func(t *testing.T) {
	})

	// Verify education repositories are initialized
	t.Run("education repositories", func(t *testing.T) {
		assert.NotNil(t, factory.Group)
		assert.NotNil(t, factory.GroupTeacher)
		assert.NotNil(t, factory.GroupSubstitution)
	})

	// Verify schedule repositories are initialized
	t.Run("schedule repositories", func(t *testing.T) {
		assert.NotNil(t, factory.Dateframe)
		assert.NotNil(t, timetable.Timeframe)
		assert.NotNil(t, timetable.PlanningTrack)
		assert.NotNil(t, timetable.RecurrenceRule)
	})

	// Verify activities repositories are initialized
	t.Run("activities repositories", func(t *testing.T) {
		assert.NotNil(t, timetable.ActivityGroup)
		assert.NotNil(t, timetable.ActivityCategory)
		assert.NotNil(t, timetable.ActivitySchedule)
		assert.NotNil(t, timetable.ActivitySupervisor)

		assert.NotNil(t, timetable.ActivitySchedule)
		assert.NotNil(t, timetable.ActivitySupervisor)
		assert.NotNil(t, timetable.StudentEnrollment)
		assert.NotNil(t, timetable.Timeframe)
		assert.NotNil(t, timetable.PlanningTrack)
		assert.NotNil(t, timetable.RecurrenceRule)
	})

	// Verify active repositories are initialized
	t.Run("active repositories", func(t *testing.T) {
		assert.NotNil(t, factory.ActiveGroup)
		assert.NotNil(t, factory.GroupSupervisor)
	})

	// Verify feedback repositories are initialized
	t.Run("feedback repositories", func(t *testing.T) {
	})

	t.Run("enrollment repositories", func(t *testing.T) {
		assert.NotNil(t, factory.CarePlan())
		assert.NotNil(t, factory.Enrollment())
	})
}
