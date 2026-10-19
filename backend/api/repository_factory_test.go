package api

import (
	"testing"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRepositoryFactory(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	factory := NewRepositoryTestFactory(testpkg.NewDatabase(t, db))
	factoryPeople := NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
	factoryPeopleStructure := owners.NewSchoolStructureSuiteRepositories(testpkg.NewDatabase(t, db))
	factoryAccounts := NewAccountSuiteRepositories(testpkg.NewDatabase(t, db))
	factorySchedules := NewCareScheduleSuiteRepositories(testpkg.NewDatabase(t, db))
	factoryPresence := NewPresenceSuiteRepositories(testpkg.NewDatabase(t, db))
	factoryEnrollment := NewEnrollmentSuiteRepositories(testpkg.NewDatabase(t, db))
	require.NotNil(t, factory)
	// The retained activities and schedule repositories belong to Timetable
	// and are built where they are used (#2743).
	timetable := NewUnobservedTimetableRepositories(testpkg.MustDatabase(db))

	// Verify users repositories are initialized
	t.Run("users repositories", func(t *testing.T) {
		assert.NotNil(t, factoryPeople.Person)
		assert.NotNil(t, factoryAccounts.RFIDCard)
		assert.NotNil(t, factoryPeople.Staff)
		assert.NotNil(t, factoryPeople.Student)
		assert.NotNil(t, factoryPeople.Teacher)
		assert.NotNil(t, factoryPeople.Guest)
		assert.NotNil(t, factoryAccounts.Profile)
		assert.NotNil(t, factoryPeople.StudentGuardian)
		assert.NotNil(t, factoryPeople.GuardianProfile)
	})

	// Verify facilities repositories are initialized
	t.Run("facilities repositories", func(t *testing.T) {
	})

	// Verify education repositories are initialized
	t.Run("education repositories", func(t *testing.T) {
		assert.NotNil(t, factoryPeopleStructure.Group)
		assert.NotNil(t, factoryPeopleStructure.GroupTeacher)
		assert.NotNil(t, factoryPeople.GroupSubstitution)
	})

	// Verify schedule repositories are initialized
	t.Run("schedule repositories", func(t *testing.T) {
		assert.NotNil(t, factorySchedules.Dateframe)
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
		assert.NotNil(t, factoryPresence.ActiveGroup)
		assert.NotNil(t, factoryPresence.GroupSupervisor)
	})

	// Verify feedback repositories are initialized
	t.Run("feedback repositories", func(t *testing.T) {
	})

	t.Run("enrollment repositories", func(t *testing.T) {
		assert.NotNil(t, factory.CarePlan())
		assert.NotNil(t, factoryEnrollment.Enrollment())
	})
}
