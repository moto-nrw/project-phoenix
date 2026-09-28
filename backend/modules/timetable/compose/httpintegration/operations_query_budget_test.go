package httpintegration_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
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
func newPlannedNowBudgetOperations(t *testing.T, db *bun.DB, staff *usersModels.Staff, now time.Time) timetable.OperationCapability {
	t.Helper()
	repos := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	stand := newTimetableOpsDeps()
	stand.personService.accountPerson = staff.Person
	stand.personService.staffByPersonID[staff.PersonID] = staff
	deps := stand.dependencies()
	deps.Sessions = repos.ActiveGroup
	deps.Supervisions = repos.GroupSupervisor
	deps.Visits = repositories.NewStudentPresenceForTests(db)
	deps.PlanningTracks = nil
	deps.Now = func() time.Time { return now }
	rows := repositories.TimetableOwnerRows{
		Instances:       repos.ActivityInstance,
		InstanceStaff:   repos.InstanceStaff,
		Participants:    repos.InstanceStudent,
		Templates:       repos.ActivityGroup,
		Students:        repos.Student,
		EducationGroups: repos.Group,
		Rooms:           repos.Room,
	}
	templates, err := rows.OperationTemplates()
	require.NoError(t, err)
	deps.Instances, deps.InstanceStaff, deps.Participants = rows.Instances, rows.InstanceStaff, rows.Participants
	deps.Templates, deps.Students = templates, rows.Students
	deps.EducationGroups, deps.Rooms = rows.EducationGroupNames(), rows.RoomNames()
	operations, err := compose.NewOperations(deps)
	require.NoError(t, err)
	return operations
}
