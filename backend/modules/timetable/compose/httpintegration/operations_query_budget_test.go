package httpintegration_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"

	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The planned-now read runs over the retained repository rows the way the
// composition root binds them (repositories.TimetableOwnerRows); the People
// Directory, Settings Platform and Care Plan are fixed stand-ins, so the
// count covers the timetable's own statements: the day's blocks, the room
// names, and one staff and one participant read for every block.
func TestTimetableOperationsPlannedNowQueryBudget(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	date := calendar.NewDate(2026, time.October, 19)
	now := date.BerlinMidnight().Add(13*time.Hour + 50*time.Minute)
	staff := testpkg.CreateTestStaff(t, db, "PlannedBudget", "Teacher")
	room := testpkg.CreateTestRoom(t, db, "PlannedBudgetRoom")
	operations := newPlannedNowBudgetOperations(t, db, staff, now)

	created := 0
	addInstances := func(n int) {
		for range n {
			instance := testpkg.CreateTestActivityInstance(t, db, date, room.ID, testpkg.ActivityInstanceOpts{
				StartHHMM: "14:00",
				EndHHMM:   "15:00",
				Title:     fmt.Sprintf("Planned budget %d", created),
			})
			student := testpkg.CreateTestStudent(t, db, "PlannedBudget", fmt.Sprintf("Student%d", created), "PB1")
			testpkg.CreateTestInstanceStaff(t, db, instance.ID, staff.ID, testpkg.InstanceStaffOpts{IsPrimary: true})
			testpkg.CreateTestInstanceStudent(t, db, instance.ID, student.ID, scheduleModels.AttendanceStatusExpected)
			created++
		}
	}

	counter := testpkg.CaptureQueries(t, db)
	run := func() int {
		counter.Reset()
		result, err := operations.PlannedNow(ctx, 1, false, date, now, timetable.PlannedNowOptions{})
		require.NoError(t, err)
		require.Len(t, result, created)
		return counter.Total()
	}

	addInstances(3)
	smallCount := run()
	addInstances(5)
	largeCount := run()
	t.Logf("query budget: 3 planned instances → %d queries, 8 planned instances → %d queries", smallCount, largeCount)
	assert.Equal(t, smallCount, largeCount, "query count must not grow with the planned instance count")
	testpkg.AssertQueryBudget(t, "services.schedule.planned_now", counter.Queries())
}

// newPlannedNowBudgetOperations composes the operational day over the
// retained repositories; the caller is the given staff member.
func newPlannedNowBudgetOperations(t *testing.T, db *bun.DB, staffFixture *testpkg.Staff, now time.Time) timetable.OperationCapability {
	t.Helper()
	rooms := api.MustNewUnobservedRooms(testpkg.NewDatabase(t, db))
	repos := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
	reposStructure := api.NewSchoolStructureSuiteRepositories(testpkg.NewDatabase(t, db))
	reposPresence := api.NewPresenceSuiteRepositories(testpkg.NewDatabase(t, db))
	staff, err := api.NewTimetableStaff(repos.Staff).FindWithPerson(testpkg.Ctx(t), staffFixture.ID)
	require.NoError(t, err)
	timetableRepos := api.NewUnobservedTimetableRepositories(testpkg.MustDatabase(db))
	stand := newTimetableOpsDeps()
	stand.personService.accountPerson = staff.Person
	stand.personService.staffByPersonID[staff.PersonID] = staff
	deps := stand.dependencies()
	deps.Sessions = reposPresence.ActiveGroup
	deps.Supervisions = reposPresence.GroupSupervisor
	deps.Visits = api.NewStudentPresenceForTests(testpkg.NewDatabase(t, db))
	deps.PlanningTracks = nil
	deps.Now = func() time.Time { return now }
	rows := compose.OwnerRows{
		Instances:       timetableRepos.ActivityInstance,
		InstanceStaff:   timetableRepos.InstanceStaff,
		Participants:    timetableRepos.InstanceStudent,
		Templates:       timetableRepos.ActivityGroup,
		Students:        api.NewTimetableRosterStudents(repos.Student),
		EducationGroups: api.TimetableEducationGroupNames(reposStructure.Group),
		Rooms:           api.NewTimetableRoomNames(rooms),
	}
	templates, err := rows.OperationTemplates()
	require.NoError(t, err)
	deps.Instances, deps.InstanceStaff, deps.Participants = rows.Instances, rows.InstanceStaff, rows.Participants
	deps.Templates, deps.Students = templates, rows.Students
	deps.EducationGroups, deps.Rooms = rows.EducationGroups, rows.Rooms
	operations, err := compose.NewOperations(deps, api.TimetableOperationCaller{RequestSession: deps.Session})
	require.NoError(t, err)
	return operations
}
