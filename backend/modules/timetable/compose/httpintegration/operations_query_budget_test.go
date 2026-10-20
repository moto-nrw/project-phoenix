package httpintegration_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	fixtures "github.com/moto-nrw/project-phoenix/modules/timetable/fixturetest"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The planned-now read runs over the retained repository rows the way the
// composition root binds them (repositories.TimetableOwnerRows); the People
// Directory, Settings Platform and Care Plan are fixed stand-ins, so the
// count covers the timetable's own statements: the day's blocks, and one
// staff, one participant and one display read (room name and planning
// track, #4413) for every block.
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
			testpkg.CreateTestInstanceStudent(t, db, instance.ID, student.ID, fixtures.AttendanceStatusExpected())
			created++
		}
	}

	require.NoError(t, testpkg.WithinTenantContext(t, ctx, db, testpkg.TenantIDFromContext(ctx), func(ctx context.Context) error {
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
		return nil
	}))
}

// newPlannedNowBudgetOperations composes the operational day over the
// retained repositories; the caller is the given staff member.
func newPlannedNowBudgetOperations(t *testing.T, db *bun.DB, staffFixture *testpkg.Staff, now time.Time) timetable.OperationCapability {
	t.Helper()
	rooms := owners.MustNewUnobservedRooms(testpkg.NewDatabase(t, db))
	repos := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
	reposStructure := owners.NewSchoolStructureSuiteRepositories(testpkg.NewDatabase(t, db))
	reposPresence := api.NewPresenceSuiteRepositories(testpkg.NewDatabase(t, db))
	staff, err := testpkg.TenantQuery1(t, testpkg.Ctx(t), db, owners.NewTimetableStaff(repos.Staff).FindWithPerson, staffFixture.ID)
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
		Instances:       fixtures.IntoActivityInstanceRecords(fixtures.FromActivityInstanceRecords(timetableRepos.ActivityInstance)),
		InstanceStaff:   fixtures.IntoInstanceStaffRecords(fixtures.FromInstanceStaffRecords(timetableRepos.InstanceStaff)),
		Participants:    fixtures.IntoInstanceStudentRecords(fixtures.FromInstanceStudentRecords(timetableRepos.InstanceStudent)),
		Templates:       timetableRepos.ActivityGroup,
		Students:        owners.NewTimetableRosterStudents(repos.Student),
		EducationGroups: owners.TimetableEducationGroupNames(reposStructure.Group),
		Rooms:           owners.NewTimetableRoomNames(rooms),
	}
	templates, err := compose.NewOperationTemplates(rows.Templates)
	require.NoError(t, err)
	deps.Instances, deps.InstanceStaff, deps.Participants = fixtures.IntoOperationInstances(fixtures.FromActivityInstanceRecords(rows.Instances)), fixtures.IntoOperationInstanceStaff(fixtures.FromInstanceStaffRecords(rows.InstanceStaff)), fixtures.IntoOperationParticipants(fixtures.FromInstanceStudentRecords(rows.Participants))
	deps.Templates, deps.Students = templates, rows.Students
	deps.EducationGroups, deps.Rooms = rows.EducationGroups, rows.Rooms
	deps.BlockMetadata = api.NewTimetableBlockDisplayForTests(testpkg.MustDatabase(db))
	operations, err := compose.NewOperations(deps, owners.TimetableOperationCaller{RequestSession: deps.Session})
	require.NoError(t, err)
	return operations
}

// The Aufsicht's „Als Nächstes“ reads the default (now) scope. Its blocks
// carry the planning-track colour of their template like the Tagesplan does
// (#4413), and the colour costs no statement of its own: it is read along
// with the day's blocks.
func TestTimetableOperationsPlannedNowCarriesPlanningTrackColour(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	date := calendar.NewDate(2026, time.October, 19)
	now := date.BerlinMidnight().Add(13*time.Hour + 50*time.Minute)
	staff := testpkg.CreateTestStaff(t, db, "PlannedTrack", "Teacher")
	room := testpkg.CreateTestRoom(t, db, "PlannedTrackRoom")
	template := testpkg.CreateTestActivityGroup(t, db, "PlannedTrackTemplate")
	track, err := scenarioPlanningTracks(t, db).AddPlanningTrack(ctx, timetable.PlanningTrackDraft{
		Name: "Angebote", Color: "#5080D8", SortOrder: 0,
	})
	require.NoError(t, err)
	_, err = db.NewUpdate().Table("activities.groups").
		Set("planning_track_id = ?", track.ID).
		Where("tenant_id = ?", testpkg.Tenant(t)).
		Where("id = ?", template.ID).
		Exec(ctx)
	require.NoError(t, err)
	operations := newPlannedNowBudgetOperations(t, db, staff, now)

	tracked := testpkg.CreateTestActivityInstance(t, db, date, room.ID, testpkg.ActivityInstanceOpts{
		ActivityGroupID: &template.ID, StartHHMM: "14:00", EndHHMM: "15:00", Title: "Mit Spur",
	})
	untracked := testpkg.CreateTestActivityInstance(t, db, date, room.ID, testpkg.ActivityInstanceOpts{
		StartHHMM: "14:00", EndHHMM: "15:00", Title: "Ohne Spur",
	})
	for _, instance := range []*testpkg.ActivityInstance{tracked, untracked} {
		testpkg.CreateTestInstanceStaff(t, db, instance.ID, staff.ID, testpkg.InstanceStaffOpts{IsPrimary: true})
	}

	require.NoError(t, testpkg.WithinTenantContext(t, ctx, db, testpkg.TenantIDFromContext(ctx), func(ctx context.Context) error {
		counter := testpkg.CaptureQueries(t, db)
		result, err := operations.PlannedNow(ctx, 1, false, date, now, timetable.PlannedNowOptions{})
		require.NoError(t, err)
		testpkg.AssertQueryBudget(t, "services.schedule.planned_now", counter.Queries())

		byID := map[int64]timetable.OperationPlannedInstance{}
		for _, planned := range result {
			byID[planned.ID] = planned
		}
		require.Contains(t, byID, tracked.ID)
		require.NotNil(t, byID[tracked.ID].PlanningTrackColor)
		assert.Equal(t, "#5080D8", *byID[tracked.ID].PlanningTrackColor)
		require.NotNil(t, byID[tracked.ID].PlanningTrackName)
		assert.Equal(t, "Angebote", *byID[tracked.ID].PlanningTrackName)
		require.Contains(t, byID, untracked.ID)
		assert.Nil(t, byID[untracked.ID].PlanningTrackColor)
		assert.Nil(t, byID[untracked.ID].PlanningTrackName)
		return nil
	}))
}
