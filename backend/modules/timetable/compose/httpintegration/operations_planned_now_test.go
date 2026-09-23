package httpintegration_test

import (
	"context"
	"errors"
	"testing"
	"time"

	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTimetableOperationsPlannedNowFiltersByAssignmentAndWindow(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.May, 10, 14, 0, 0, 0, time.UTC)
	assignedID := int64(210)
	absentID := int64(211)
	instanceID := int64(320)
	outsideWindowID := int64(321)

	deps := newTimetableOpsDeps()
	deps.personService.accountPerson = &usersModels.Person{}
	deps.personService.accountPerson.ID = 410
	deps.personService.staffByPersonID[410] = &usersModels.Staff{}
	deps.personService.staffByPersonID[410].ID = assignedID
	deps.instanceRepo.byDate = []*scheduleModels.ActivityInstance{
		instanceWithTimes(instanceID, scheduleModels.InstanceStatusPlanned, now.Add(10*time.Minute), now.Add(time.Hour)),
		instanceWithTimes(outsideWindowID, scheduleModels.InstanceStatusPlanned, now.Add(20*time.Minute), now.Add(time.Hour)),
	}
	deps.staffRepo.byInstance[instanceID] = []*scheduleModels.InstanceStaff{
		{StaffID: assignedID, IsPrimary: true},
		{StaffID: absentID, IsAbsent: true},
	}
	deps.staffRepo.byInstance[outsideWindowID] = []*scheduleModels.InstanceStaff{{StaffID: assignedID}}
	deps.studentRepo.byInstance[instanceID] = []*scheduleModels.InstanceStudent{
		{StudentID: 520, Status: scheduleModels.AttendanceStatusExpected},
		{StudentID: 521, Status: scheduleModels.AttendanceStatusPresent},
	}

	result, err := deps.service.PlannedNow(context.Background(), 610, false, calendar.DateFromTime(now), now, timetable.PlannedNowOptions{})
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, instanceID, result[0].ID)
	require.NotNil(t, result[0].RoomName)
	assert.Equal(t, "Lernraum", *result[0].RoomName)
	assert.Equal(t, []int64{assignedID}, result[0].AssignedStaffIDs)
	assert.True(t, result[0].IsAssigned)
	assert.True(t, result[0].IsPrimary)
	assert.Equal(t, 1, result[0].ExpectedStudentsCount)
	assert.Equal(t, 1, result[0].PresentStudentsCount)
	assert.False(t, result[0].IsOverdue)
	assert.Equal(t, 10, result[0].MinutesUntilStart)
	assert.NotEmpty(t, result[0].StartExpiresAt)
}

func TestTimetableOperationsPlannedNowKeepsSpontaneousAfterEnd(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.May, 10, 14, 0, 0, 0, time.UTC)
	assignedID := int64(212)
	instanceID := int64(340)
	deps := newTimetableOpsDeps()
	deps.personService.accountPerson = &usersModels.Person{}
	deps.personService.accountPerson.ID = 411
	deps.personService.staffByPersonID[411] = &usersModels.Staff{}
	deps.personService.staffByPersonID[411].ID = assignedID
	inst := instanceWithTimes(instanceID, scheduleModels.InstanceStatusPlanned, now.Add(-time.Hour), now)
	inst.IsSpontaneous = true
	deps.instanceRepo.byDate = []*scheduleModels.ActivityInstance{inst}
	deps.staffRepo.byInstance[instanceID] = []*scheduleModels.InstanceStaff{{StaffID: assignedID}}

	result, err := deps.service.PlannedNow(context.Background(), 611, false, calendar.DateFromTime(now), now, timetable.PlannedNowOptions{})
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, instanceID, result[0].ID)
	assert.True(t, result[0].CanStart)
	assert.Empty(t, result[0].StartExpiresAt)
}

func TestTimetableOperationsPlannedNowAllowsAdminOverview(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.May, 10, 14, 0, 0, 0, time.UTC)
	deps := newTimetableOpsDeps()
	deps.settings.scope = overviewScopeAdmins
	deps.instanceRepo.byDate = []*scheduleModels.ActivityInstance{
		instanceWithTimes(330, scheduleModels.InstanceStatusPlanned, now.Add(-time.Minute), now.Add(time.Hour)),
	}
	deps.staffRepo.byInstance[330] = []*scheduleModels.InstanceStaff{{StaffID: 220}}

	result, err := deps.service.PlannedNow(context.Background(), 620, true, calendar.DateFromTime(now), now, timetable.PlannedNowOptions{})

	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.True(t, result[0].IsOverdue)
}

// Since #2161 the Schulhof is a regular plannable room: its planned blocks
// appear in the "Jetzt starten" list exactly like any other room's.
func TestTimetableOperationsPlannedNowIncludesSchulhof(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.May, 10, 14, 0, 0, 0, time.UTC)
	const schulhofRoomID int64 = 811
	deps := newTimetableOpsDeps()
	deps.settings.scope = overviewScopeAdmins
	deps.rooms.names[schulhofRoomID] = "Schulhof"
	deps.instanceRepo.byDate = []*scheduleModels.ActivityInstance{
		instanceWithRoomAndTimes(330, schulhofRoomID, scheduleModels.InstanceStatusPlanned, now.Add(-time.Minute), now.Add(time.Hour)),
		instanceWithTimes(331, scheduleModels.InstanceStatusPlanned, now.Add(-time.Minute), now.Add(time.Hour)),
	}
	deps.staffRepo.byInstance[330] = []*scheduleModels.InstanceStaff{{StaffID: 220}}
	deps.staffRepo.byInstance[331] = []*scheduleModels.InstanceStaff{{StaffID: 220}}

	result, err := deps.service.PlannedNow(context.Background(), 620, true, calendar.DateFromTime(now), now, timetable.PlannedNowOptions{})

	require.NoError(t, err)
	require.Len(t, result, 2)
	ids := []int64{result[0].ID, result[1].ID}
	assert.ElementsMatch(t, []int64{330, 331}, ids)
}

func TestTimetableOperationsPlannedNowUsesInstanceDate(t *testing.T) {
	t.Parallel()

	t.Run("does not return future-date instances as overdue today", func(t *testing.T) {
		now := time.Date(2026, time.May, 10, 14, 0, 0, 0, time.UTC)
		tomorrowStart := time.Date(2026, time.May, 11, 8, 0, 0, 0, time.UTC)
		deps := newTimetableOpsDeps()
		deps.settings.scope = overviewScopeAdmins
		deps.instanceRepo.byDate = []*scheduleModels.ActivityInstance{
			instanceWithTimes(334, scheduleModels.InstanceStatusPlanned, tomorrowStart, tomorrowStart.Add(time.Hour)),
		}
		deps.staffRepo.byInstance[334] = []*scheduleModels.InstanceStaff{{StaffID: 224}}

		result, err := deps.service.PlannedNow(context.Background(), 625, true, calendar.DateFromTime(tomorrowStart), now, timetable.PlannedNowOptions{})

		require.NoError(t, err)
		assert.Empty(t, result)
	})
	// "calculates overdue metadata from instance date" maps one block
	// directly: compose's TestTimetableOperationsPlannedNowOverdueMetadataUsesInstanceDate.
}

// An account without a person profile has no staff identity. (The People
// Directory's ErrPersonNotFound sentinel lives in services/users, which the
// Timetable tests may not import; the directory binding answers the same
// missing profile.)
func TestTimetableOperationsPlannedNowErrorBranches(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.May, 10, 14, 0, 0, 0, time.UTC)

	t.Run("forbids accounts without staff when admin overview is disabled", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		deps.personService.accountPerson = nil

		result, err := deps.service.PlannedNow(context.Background(), 621, false, calendar.DateFromTime(now), now, timetable.PlannedNowOptions{})

		require.ErrorIs(t, err, timetable.ErrTimetableOperationForbidden)
		assert.Nil(t, result)
	})

	t.Run("admin overview allows missing person profile", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		deps.settings.scope = overviewScopeAdmins
		deps.personService.accountPerson = nil
		deps.instanceRepo.byDate = []*scheduleModels.ActivityInstance{
			instanceWithTimes(336, scheduleModels.InstanceStatusPlanned, now.Add(-time.Minute), now.Add(time.Hour)),
		}
		deps.staffRepo.byInstance[336] = []*scheduleModels.InstanceStaff{{StaffID: 226}}

		result, err := deps.service.PlannedNow(context.Background(), 626, true, calendar.DateFromTime(now), now, timetable.PlannedNowOptions{})

		require.NoError(t, err)
		require.Len(t, result, 1)
		assert.Equal(t, int64(336), result[0].ID)
	})

	t.Run("propagates unexpected person lookup errors", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		deps.personService.accountErr = errors.New("person lookup failed")

		result, err := deps.service.PlannedNow(context.Background(), 627, true, calendar.DateFromTime(now), now, timetable.PlannedNowOptions{})

		require.EqualError(t, err, "person lookup failed")
		assert.Nil(t, result)
	})

	t.Run("propagates instance listing errors", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		wireAssignedStaff(deps, 622, 431, 221, 331)
		deps.instanceRepo.findByDateErr = errors.New("date query failed")

		result, err := deps.service.PlannedNow(context.Background(), 622, false, calendar.DateFromTime(now), now, timetable.PlannedNowOptions{})

		require.EqualError(t, err, "date query failed")
		assert.Nil(t, result)
	})

	t.Run("propagates staff lookup errors", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		wireAssignedStaff(deps, 623, 432, 222, 332)
		deps.instanceRepo.byDate = []*scheduleModels.ActivityInstance{
			instanceWithTimes(332, scheduleModels.InstanceStatusPlanned, now, now.Add(time.Hour)),
		}
		deps.staffRepo.err = errors.New("staff query failed")

		result, err := deps.service.PlannedNow(context.Background(), 623, false, calendar.DateFromTime(now), now, timetable.PlannedNowOptions{})

		require.EqualError(t, err, "staff query failed")
		assert.Nil(t, result)
	})

	t.Run("propagates student lookup errors", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		wireAssignedStaff(deps, 624, 433, 223, 333)
		deps.instanceRepo.byDate = []*scheduleModels.ActivityInstance{
			instanceWithTimes(333, scheduleModels.InstanceStatusPlanned, now, now.Add(time.Hour)),
		}
		deps.studentRepo.err = errors.New("student query failed")

		result, err := deps.service.PlannedNow(context.Background(), 624, false, calendar.DateFromTime(now), now, timetable.PlannedNowOptions{})

		require.EqualError(t, err, "student query failed")
		assert.Nil(t, result)
	})

	t.Run("propagates room lookup errors", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		wireAssignedStaff(deps, 629, 435, 228, 339)
		deps.instanceRepo.byDate = []*scheduleModels.ActivityInstance{
			instanceWithTimes(339, scheduleModels.InstanceStatusPlanned, now, now.Add(time.Hour)),
		}
		deps.rooms.err = errors.New("room query failed")

		result, err := deps.service.PlannedNow(context.Background(), 629, false, calendar.DateFromTime(now), now, timetable.PlannedNowOptions{})

		require.EqualError(t, err, "room query failed")
		assert.Nil(t, result)
	})
}

func TestTimetableOperationsPlannedNowIncludesStartLeadWindow(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.May, 10, 14, 0, 0, 0, time.UTC)
	deps := newTimetableOpsDeps()
	deps.settings.leadMinutes = 60
	wireAssignedStaff(deps, 631, 436, 229, 341)
	deps.instanceRepo.byDate = []*scheduleModels.ActivityInstance{
		instanceWithTimes(341, scheduleModels.InstanceStatusPlanned, now.Add(45*time.Minute), now.Add(2*time.Hour)),
	}

	result, err := deps.service.PlannedNow(context.Background(), 631, false, calendar.DateFromTime(now), now, timetable.PlannedNowOptions{})

	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, int64(341), result[0].ID)
}

func TestTimetableOperationsPlannedNowSupportsUpcomingOptions(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.May, 10, 14, 0, 0, 0, time.UTC)
	deps := newTimetableOpsDeps()
	wireAssignedStaff(deps, 628, 434, 227, 337)
	deps.instanceRepo.byDate = []*scheduleModels.ActivityInstance{
		instanceWithTimes(337, scheduleModels.InstanceStatusPlanned, now.Add(90*time.Minute), now.Add(2*time.Hour)),
		instanceWithTimes(338, scheduleModels.InstanceStatusPlanned, now.Add(100*time.Minute), now.Add(2*time.Hour)),
	}
	for _, inst := range deps.instanceRepo.byDate {
		deps.instanceRepo.byID[inst.ID] = inst
	}
	deps.staffRepo.byInstance[338] = []*scheduleModels.InstanceStaff{{StaffID: 227}}
	deps.studentRepo.byInstance[337] = []*scheduleModels.InstanceStudent{
		{StudentID: 527, Status: scheduleModels.AttendanceStatusExpected},
	}
	deps.students.byID[527] = &usersModels.Student{PersonID: 437, SchoolClass: "2a"}
	deps.personService.people[437] = &usersModels.Person{FirstName: "Lina", LastName: "Lang"}
	pickup := time.Date(1, time.January, 1, 15, 20, 0, 0, time.UTC)
	deps.pickupService.byStudent[527] = &pickup

	result, err := deps.service.PlannedNow(context.Background(), 628, false, calendar.DateFromTime(now), now, timetable.PlannedNowOptions{
		HorizonMinutes: 120,
		Limit:          1,
		IncludeRoster:  true,
	})

	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, int64(337), result[0].ID)
	require.Len(t, result[0].RosterPreview, 1)
	assert.Equal(t, "Lina Lang", result[0].RosterPreview[0].StudentName)
	assert.True(t, result[0].PickupTimesLoaded)
	require.NotNil(t, result[0].RosterPreview[0].PickupTime)
	assert.Equal(t, "15:20", *result[0].RosterPreview[0].PickupTime)
}

// scope=past is the complement of the default window (#2335): completed
// blocks and never-started planned blocks whose end has passed. Running,
// cancelled, still-open planned, and spontaneous planned instances stay out.
func TestTimetableOperationsPlannedNowPastScopeSelectsFinishedBlocks(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.May, 10, 15, 0, 0, 0, time.UTC)
	deps := newTimetableOpsDeps()
	wireAssignedStaff(deps, 632, 438, 231, 360)
	completed := instanceWithTimes(361, scheduleModels.InstanceStatusCompleted, now.Add(-3*time.Hour), now.Add(-2*time.Hour))
	expiredSpontaneous := instanceWithTimes(364, scheduleModels.InstanceStatusPlanned, now.Add(-3*time.Hour), now.Add(-2*time.Hour))
	expiredSpontaneous.IsSpontaneous = true
	deps.instanceRepo.byDate = []*scheduleModels.ActivityInstance{
		instanceWithTimes(360, scheduleModels.InstanceStatusPlanned, now.Add(-135*time.Minute), now.Add(-30*time.Minute)),
		completed,
		instanceWithTimes(362, scheduleModels.InstanceStatusPlanned, now.Add(-time.Hour), now.Add(time.Hour)),
		instanceWithTimes(363, scheduleModels.InstanceStatusCancelled, now.Add(-3*time.Hour), now.Add(-2*time.Hour)),
		expiredSpontaneous,
		instanceWithTimes(365, scheduleModels.InstanceStatusActive, now.Add(-time.Hour), now.Add(-30*time.Minute)),
	}
	for _, inst := range deps.instanceRepo.byDate {
		deps.staffRepo.byInstance[inst.ID] = []*scheduleModels.InstanceStaff{{StaffID: 231}}
	}

	result, err := deps.service.PlannedNow(context.Background(), 632, false, calendar.DateFromTime(now), now, timetable.PlannedNowOptions{Scope: timetable.PlannedNowScopePast})

	require.NoError(t, err)
	require.Len(t, result, 2)
	ids := []int64{result[0].ID, result[1].ID}
	assert.ElementsMatch(t, []int64{360, 361}, ids)
	for _, inst := range result {
		assert.False(t, inst.CanStart)
		assert.Empty(t, inst.StartExpiresAt)
	}
}

// Past-scope visibility mirrors the default scope: unassigned staff see
// nothing, the admin overview sees everything (#2335).
func TestTimetableOperationsPlannedNowPastScopeKeepsVisibilityRules(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.May, 10, 15, 0, 0, 0, time.UTC)
	deps := newTimetableOpsDeps()
	wireAssignedStaff(deps, 633, 439, 232, 370)
	deps.instanceRepo.byDate = []*scheduleModels.ActivityInstance{
		instanceWithTimes(370, scheduleModels.InstanceStatusCompleted, now.Add(-3*time.Hour), now.Add(-2*time.Hour)),
		instanceWithTimes(371, scheduleModels.InstanceStatusCompleted, now.Add(-3*time.Hour), now.Add(-2*time.Hour)),
	}
	deps.staffRepo.byInstance[370] = []*scheduleModels.InstanceStaff{{StaffID: 232}}
	deps.staffRepo.byInstance[371] = []*scheduleModels.InstanceStaff{{StaffID: 999}}

	result, err := deps.service.PlannedNow(context.Background(), 633, false, calendar.DateFromTime(now), now, timetable.PlannedNowOptions{Scope: timetable.PlannedNowScopePast})
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, int64(370), result[0].ID)

	deps.settings.scope = overviewScopeAdmins
	result, err = deps.service.PlannedNow(context.Background(), 633, true, calendar.DateFromTime(now), now, timetable.PlannedNowOptions{Scope: timetable.PlannedNowScopePast})
	require.NoError(t, err)
	require.Len(t, result, 2)
}

// Day scope follows the operational-overview rule (#2383): under all_staff a
// verified staff member sees the whole day including foreign blocks, decorated
// with the planned colour, Zielgruppe and staff names; without the setting the
// day stays own-only (#2527).
func TestTimetableOperationsPlannedNowDayScopeFollowsOperationalOverview(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.May, 10, 15, 0, 0, 0, time.UTC)
	deps := newTimetableOpsDeps()
	wireAssignedStaff(deps, 634, 441, 233, 380)

	activeGroupID := int64(910)
	activityGroupID := int64(720)
	trackID := int64(55)
	educationGroupID := int64(66)
	cancelReason := "Personalausfall"

	own := instanceWithTimes(380, scheduleModels.InstanceStatusPlanned, now.Add(time.Hour), now.Add(2*time.Hour))
	foreignActive := instanceWithTimes(381, scheduleModels.InstanceStatusActive, now.Add(-30*time.Minute), now.Add(30*time.Minute))
	foreignActive.ActiveGroupID = &activeGroupID
	foreignActive.ActivityGroupID = &activityGroupID
	foreignCancelled := instanceWithTimes(382, scheduleModels.InstanceStatusCancelled, now.Add(-2*time.Hour), now.Add(-time.Hour))
	foreignCancelled.CancelReason = &cancelReason
	deps.instanceRepo.byDate = []*scheduleModels.ActivityInstance{own, foreignActive, foreignCancelled}
	deps.staffRepo.byInstance[381] = []*scheduleModels.InstanceStaff{{StaffID: 998, IsSubstitute: true}}
	deps.staffRepo.byInstance[382] = []*scheduleModels.InstanceStaff{{StaffID: 999}}

	deps.tracks.byID[trackID] = timetable.PlanningTrack{ID: trackID, Name: "Angebote", Color: "#5080D8"}
	activityGroup := &activitiesModels.Group{PlanningTrackID: &trackID, EducationGroupID: &educationGroupID}
	activityGroup.ID = activityGroupID
	deps.activityGroups.byID[activityGroupID] = activityGroup
	deps.groups.names[educationGroupID] = "Gruppe Sonne"
	substitute := &usersModels.Staff{Person: &usersModels.Person{FirstName: "Vera", LastName: "Vertretung"}}
	substitute.ID = 998
	deps.personService.staffWithPerson[998] = substitute

	// Without the school-wide overview the day stays own-only.
	result, err := deps.service.PlannedNow(context.Background(), 634, false, calendar.DateFromTime(now), now, timetable.PlannedNowOptions{Scope: timetable.PlannedNowScopeDay})
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, int64(380), result[0].ID)

	deps.settings.scope = overviewScopeAllStaff
	result, err = deps.service.PlannedNow(context.Background(), 634, false, calendar.DateFromTime(now), now, timetable.PlannedNowOptions{Scope: timetable.PlannedNowScopeDay})
	require.NoError(t, err)
	require.Len(t, result, 3)
	byID := map[int64]timetable.OperationPlannedInstance{}
	for _, inst := range result {
		byID[inst.ID] = inst
	}
	require.Contains(t, byID, int64(381))
	require.NotNil(t, byID[381].ActiveGroupID)
	assert.Equal(t, activeGroupID, *byID[381].ActiveGroupID)
	require.NotNil(t, byID[381].PlanningTrackColor)
	assert.Equal(t, "#5080D8", *byID[381].PlanningTrackColor)
	require.NotNil(t, byID[381].PlanningTrackName)
	assert.Equal(t, "Angebote", *byID[381].PlanningTrackName)
	require.NotNil(t, byID[381].GroupName)
	assert.Equal(t, "Gruppe Sonne", *byID[381].GroupName)
	require.Len(t, byID[381].StaffNames, 1)
	assert.Equal(t, "Vera Vertretung", byID[381].StaffNames[0].DisplayName)
	assert.True(t, byID[381].StaffNames[0].IsSubstitute)
	// A running or cancelled foreign block is never startable from the day list.
	assert.False(t, byID[381].CanStart)
	require.Contains(t, byID, int64(382))
	require.NotNil(t, byID[382].CancelReason)
	assert.Equal(t, cancelReason, *byID[382].CancelReason)
	assert.False(t, byID[382].CanStart)
}

// ActiveSessions lists today's running instances with their plan windows so
// the supervision UI can label session tabs "Aktivitätsname · Planzeit"
// (#2265). Planned and completed instances stay out; so do active rows
// without a live session.
func TestTimetableOperationsActiveSessions(t *testing.T) {
	t.Parallel()

	deps := newTimetableOpsDeps()
	now := time.Date(2026, time.May, 10, 13, 0, 0, 0, time.UTC)

	running := instanceWithTimes(910, scheduleModels.InstanceStatusActive,
		time.Date(2026, time.May, 10, 12, 45, 0, 0, time.UTC),
		time.Date(2026, time.May, 10, 13, 45, 0, 0, time.UTC))
	running.ID = 910
	running.Title = "GT 1"
	runningGroup := int64(310)
	running.ActiveGroupID = &runningGroup

	planned := instanceWithTimes(911, scheduleModels.InstanceStatusPlanned,
		time.Date(2026, time.May, 10, 14, 0, 0, 0, time.UTC),
		time.Date(2026, time.May, 10, 15, 0, 0, 0, time.UTC))
	planned.ID = 911

	orphan := instanceWithTimes(912, scheduleModels.InstanceStatusActive,
		time.Date(2026, time.May, 10, 12, 0, 0, 0, time.UTC),
		time.Date(2026, time.May, 10, 13, 0, 0, 0, time.UTC))
	orphan.ID = 912

	deps.instanceRepo.byDate = []*scheduleModels.ActivityInstance{running, planned, orphan}

	sessions, err := deps.service.ActiveSessions(context.Background(), calendar.DateFromTime(now))

	require.NoError(t, err)
	require.Len(t, sessions, 1)
	assert.Equal(t, runningGroup, sessions[0].ActiveGroupID)
	assert.Equal(t, int64(910), sessions[0].InstanceID)
	assert.Equal(t, "GT 1", sessions[0].Title)
	assert.Equal(t, "12:45", sessions[0].StartTime)
	assert.Equal(t, "13:45", sessions[0].EndTime)
}
