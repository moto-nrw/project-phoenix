package httpintegration_test

import (
	"context"
	"errors"
	"testing"
	"time"

	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTimetableOperationsRosterCombinesPlannedStudentsAndLiveDropIns(t *testing.T) {
	t.Parallel()

	instanceID := int64(360)
	activeGroupID := int64(260)
	groupID := int64(270)
	visitID := int64(370)
	deps := newTimetableOpsDeps()
	wireAssignedStaff(deps, 650, 450, 240, instanceID)
	deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, activeGroupID)
	deps.studentRepo.byInstance[instanceID] = []*scheduleModels.InstanceStudent{
		{StudentID: 530, Status: scheduleModels.AttendanceStatusExpected},
	}
	deps.visitRepo.byActiveGroup[activeGroupID] = []*studentpresence.Visit{
		{StudentID: 531, ActiveGroupID: activeGroupID, EntryTime: time.Date(2026, time.May, 10, 14, 5, 0, 0, time.UTC)},
	}
	deps.visitRepo.byActiveGroup[activeGroupID][0].ID = visitID
	deps.students.byID[530] = &usersModels.Student{PersonID: 460, SchoolClass: "3a", GroupID: &groupID}
	deps.students.byID[531] = &usersModels.Student{PersonID: 461, SchoolClass: "4b"}
	deps.personService.people[460] = &usersModels.Person{FirstName: "Zoe", LastName: "Zimmer"}
	deps.personService.people[461] = &usersModels.Person{FirstName: "Anna", LastName: "Anlauf"}
	deps.groups.names[groupID] = "OGS Blau"

	roster, err := deps.service.Roster(context.Background(), 650, false, instanceID)

	require.NoError(t, err)
	require.Len(t, roster.Rows, 2)
	assert.Equal(t, "Anna Anlauf", roster.Rows[0].StudentName)
	assert.True(t, roster.Rows[0].IsUnplanned)
	assert.True(t, roster.Rows[0].CurrentlyPresent)
	assert.Equal(t, &visitID, roster.Rows[0].VisitID)
	assert.Equal(t, "Zoe Zimmer", roster.Rows[1].StudentName)
	assert.True(t, roster.Rows[1].Planned)
	assert.Equal(t, "OGS Blau", roster.Rows[1].GroupName)
}

// A child recorded present in another running block of the same day gets a
// parallel-presence marker on their roster row, so two supervisors working
// consecutive blocks of the same lane see the overlap instead of a seemingly
// contradictory attendance state (#2265). Rows without such an overlap stay
// unmarked, and rosters of non-active instances never query for it.
func TestTimetableOperationsRosterFlagsParallelPresence(t *testing.T) {
	t.Parallel()

	instanceID := int64(368)
	activeGroupID := int64(268)
	otherInstanceID := int64(369)
	deps := newTimetableOpsDeps()
	wireAssignedStaff(deps, 660, 480, 250, instanceID)
	deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, activeGroupID)
	deps.studentRepo.byInstance[instanceID] = []*scheduleModels.InstanceStudent{
		{StudentID: 540, Status: scheduleModels.AttendanceStatusExpected},
		{StudentID: 541, Status: scheduleModels.AttendanceStatusExpected},
	}
	deps.studentRepo.parallelPresence = []scheduleModels.ParallelPresence{
		{
			StudentID:  540,
			InstanceID: otherInstanceID,
			Title:      "GT 1",
			StartTime:  time.Date(2026, time.May, 10, 12, 45, 0, 0, time.UTC),
			EndTime:    time.Date(2026, time.May, 10, 13, 45, 0, 0, time.UTC),
		},
	}
	deps.students.byID[540] = &usersModels.Student{PersonID: 481, SchoolClass: "1a"}
	deps.students.byID[541] = &usersModels.Student{PersonID: 482, SchoolClass: "1a"}
	deps.personService.people[481] = &usersModels.Person{FirstName: "Mia", LastName: "Muster"}
	deps.personService.people[482] = &usersModels.Person{FirstName: "Tom", LastName: "Test"}

	roster, err := deps.service.Roster(context.Background(), 660, false, instanceID)

	require.NoError(t, err)
	require.Len(t, roster.Rows, 2)
	rowsByStudent := map[int64]timetable.OperationRosterRow{}
	for _, row := range roster.Rows {
		rowsByStudent[row.StudentID] = row
	}
	flagged := rowsByStudent[540]
	require.NotNil(t, flagged.ParallelPresentIn)
	assert.Equal(t, otherInstanceID, flagged.ParallelPresentIn.InstanceID)
	assert.Equal(t, "GT 1", flagged.ParallelPresentIn.Title)
	assert.Equal(t, "12:45", flagged.ParallelPresentIn.StartTime)
	assert.Equal(t, "13:45", flagged.ParallelPresentIn.EndTime)
	assert.Nil(t, rowsByStudent[541].ParallelPresentIn)
}

func TestTimetableOperationsRosterLoadsEffectivePickupTimesForBlockDate(t *testing.T) {
	t.Parallel()

	instanceID := int64(374)
	activeGroupID := int64(274)
	blockDate := calendar.NewDate(2026, time.May, 12)
	deps := newTimetableOpsDeps()
	wireAssignedStaff(deps, 664, 489, 254, instanceID)
	deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, activeGroupID)
	deps.instanceRepo.byID[instanceID].Date = scheduleModels.Date(blockDate)
	deps.studentRepo.byInstance[instanceID] = []*scheduleModels.InstanceStudent{
		{StudentID: 548, Status: scheduleModels.AttendanceStatusExpected},
		{StudentID: 549, Status: scheduleModels.AttendanceStatusExpected},
		{StudentID: 550, Status: scheduleModels.AttendanceStatusExpected},
	}
	deps.students.byID[548] = &usersModels.Student{PersonID: 490, SchoolClass: "1a"}
	deps.students.byID[549] = &usersModels.Student{PersonID: 491, SchoolClass: "1a"}
	deps.students.byID[550] = &usersModels.Student{PersonID: 492, SchoolClass: "1a"}
	deps.personService.people[490] = &usersModels.Person{FirstName: "Wochenplan", LastName: "Kind"}
	deps.personService.people[491] = &usersModels.Person{FirstName: "Tagesänderung", LastName: "Kind"}
	deps.personService.people[492] = &usersModels.Person{FirstName: "Ohne", LastName: "Gehzeit"}
	weekly := time.Date(1, time.January, 1, 15, 0, 0, 0, time.UTC)
	override := time.Date(1, time.January, 1, 13, 30, 0, 0, time.UTC)
	// Care Plan's effective time: the weekly plan, a day exception, and a
	// successful lookup without a time.
	deps.pickupService.byStudent[548] = &weekly
	deps.pickupService.byStudent[549] = &override
	deps.pickupService.byStudent[550] = nil

	roster, err := deps.service.Roster(context.Background(), 664, false, instanceID)

	require.NoError(t, err)
	assert.True(t, roster.PickupTimesLoaded)
	assert.Equal(t, 1, deps.pickupService.calls, "one roster must use one bulk pickup lookup")
	assert.Equal(t, blockDate, deps.pickupService.date, "the block date, not today, selects the effective time")
	assert.ElementsMatch(t, []int64{548, 549, 550}, deps.pickupService.studentIDs)
	rowsByStudent := make(map[int64]timetable.OperationRosterRow, len(roster.Rows))
	for _, row := range roster.Rows {
		rowsByStudent[row.StudentID] = row
	}
	require.NotNil(t, rowsByStudent[548].PickupTime)
	assert.Equal(t, "15:00", *rowsByStudent[548].PickupTime)
	require.NotNil(t, rowsByStudent[549].PickupTime)
	assert.Equal(t, "13:30", *rowsByStudent[549].PickupTime)
	assert.Nil(t, rowsByStudent[550].PickupTime, "a successful lookup without a time stays an explicit empty value")
}

func TestTimetableOperationsRosterKeepsAttendanceUsableWhenPickupTimesFail(t *testing.T) {
	t.Parallel()

	instanceID := int64(375)
	deps := newTimetableOpsDeps()
	wireAssignedStaff(deps, 665, 493, 255, instanceID)
	deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, 275)
	deps.studentRepo.byInstance[instanceID] = []*scheduleModels.InstanceStudent{
		{StudentID: 551, Status: scheduleModels.AttendanceStatusExpected},
	}
	deps.students.byID[551] = &usersModels.Student{PersonID: 494, SchoolClass: "2b"}
	deps.personService.people[494] = &usersModels.Person{FirstName: "Nora", LastName: "Nutzbar"}
	deps.pickupService.err = errors.New("pickup lookup failed")

	roster, err := deps.service.Roster(context.Background(), 665, false, instanceID)

	require.NoError(t, err, "pickup data must not take down attendance control")
	assert.False(t, roster.PickupTimesLoaded)
	require.Len(t, roster.Rows, 1)
	assert.Equal(t, int64(551), roster.Rows[0].StudentID)
	assert.Nil(t, roster.Rows[0].PickupTime)
}

func TestTimetableOperationsRosterSkipsParallelPresenceForInactiveInstance(t *testing.T) {
	t.Parallel()

	instanceID := int64(371)
	deps := newTimetableOpsDeps()
	wireAssignedStaff(deps, 661, 483, 251, instanceID)
	completed := instanceWithTimes(instanceID, scheduleModels.InstanceStatusCompleted,
		time.Date(2026, time.May, 10, 14, 0, 0, 0, time.UTC),
		time.Date(2026, time.May, 10, 15, 0, 0, 0, time.UTC))
	completed.ID = instanceID
	deps.instanceRepo.byID[instanceID] = completed
	deps.studentRepo.byInstance[instanceID] = []*scheduleModels.InstanceStudent{
		{StudentID: 545, Status: scheduleModels.AttendanceStatusPresent},
	}
	deps.students.byID[545] = &usersModels.Student{PersonID: 484, SchoolClass: "2b"}
	deps.personService.people[484] = &usersModels.Person{FirstName: "Lea", LastName: "Lang"}

	roster, err := deps.service.Roster(context.Background(), 661, false, instanceID)

	require.NoError(t, err)
	require.Len(t, roster.Rows, 1)
	assert.Nil(t, roster.Rows[0].ParallelPresentIn)
	assert.Zero(t, deps.studentRepo.parallelPresenceCall)
}

func TestTimetableOperationsRosterParallelPresenceLookupErrorFails(t *testing.T) {
	t.Parallel()

	instanceID := int64(372)
	activeGroupID := int64(272)
	deps := newTimetableOpsDeps()
	wireAssignedStaff(deps, 662, 485, 252, instanceID)
	deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, activeGroupID)
	deps.studentRepo.byInstance[instanceID] = []*scheduleModels.InstanceStudent{
		{StudentID: 546, Status: scheduleModels.AttendanceStatusExpected},
	}
	deps.students.byID[546] = &usersModels.Student{PersonID: 486, SchoolClass: "2b"}
	deps.personService.people[486] = &usersModels.Person{FirstName: "Ben", LastName: "Berg"}
	deps.studentRepo.parallelPresenceErr = errors.New("boom")

	_, err := deps.service.Roster(context.Background(), 662, false, instanceID)

	require.Error(t, err)
}

// Two independent reads of the same instance after an attendance write must
// return the same roster state — the backend truth two parallel clients
// converge on via SSE-triggered refetches (#2265 acceptance criterion).
func TestTimetableOperationsTwoReadsAfterAttendanceWriteAgree(t *testing.T) {
	t.Parallel()

	instanceID := int64(373)
	activeGroupID := int64(273)
	deps := newTimetableOpsDeps()
	wireAssignedStaff(deps, 663, 487, 253, instanceID)
	deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, activeGroupID)
	row := &scheduleModels.InstanceStudent{StudentID: 547, Status: scheduleModels.AttendanceStatusExpected}
	row.ID = 900
	deps.studentRepo.byInstance[instanceID] = []*scheduleModels.InstanceStudent{row}
	deps.studentRepo.byInstanceStudent[instanceStudentKey{instanceID, 547}] = row
	deps.students.byID[547] = &usersModels.Student{PersonID: 488, SchoolClass: "3c"}
	deps.personService.people[488] = &usersModels.Person{FirstName: "Ida", LastName: "Igel"}

	status := scheduleModels.AttendanceStatusAbsent
	_, err := deps.service.PatchAttendance(context.Background(), 663, false, instanceID, 547, timetable.AttendancePatch{Status: &status})
	require.NoError(t, err)
	// The fake stores patches in `updates` without mutating the row; apply it
	// the way the real repository would so both reads see the written state.
	require.Len(t, deps.studentRepo.updates, 1)
	row.Status = status

	first, err := deps.service.Roster(context.Background(), 663, false, instanceID)
	require.NoError(t, err)
	second, err := deps.service.Roster(context.Background(), 663, false, instanceID)
	require.NoError(t, err)

	assert.Equal(t, first.Rows, second.Rows)
	require.Len(t, first.Rows, 1)
	assert.Equal(t, scheduleModels.AttendanceStatusAbsent, first.Rows[0].Status)
}

// A completed block's verdict is frozen in the stored marker: the care plan may
// have been edited or deleted since, and reading it here would relabel a
// historical row while the weekly list, parent calendar, and attendance history
// keep the completion-time answer (#1747 review).
func TestTimetableOperationsRosterFreezesCareDayVerdictOnCompletedInstance(t *testing.T) {
	t.Parallel()

	instanceID := int64(366)
	deps := newTimetableOpsDeps()
	wireAssignedStaff(deps, 656, 456, 246, instanceID)
	completed := instanceWithTimes(instanceID, scheduleModels.InstanceStatusCompleted,
		time.Date(2026, time.May, 10, 14, 0, 0, 0, time.UTC),
		time.Date(2026, time.May, 10, 15, 0, 0, 0, time.UTC))
	completed.ID = instanceID
	deps.instanceRepo.byID[instanceID] = completed
	deps.studentRepo.byInstance[instanceID] = []*scheduleModels.InstanceStudent{
		{StudentID: 536, Status: scheduleModels.AttendanceStatusExpected, NotScheduled: true},
	}
	deps.students.byID[536] = &usersModels.Student{PersonID: 466, SchoolClass: "3a"}
	deps.personService.people[466] = &usersModels.Person{FirstName: "Nora", LastName: "Neu"}
	// The plan says "booked" today — a later edit. It must not win over the marker.
	deps.careDayService.byStudent[536] = timetable.CareDayScheduled

	roster, err := deps.service.Roster(context.Background(), 656, false, instanceID)

	require.NoError(t, err)
	require.Len(t, roster.Rows, 1)
	assert.Equal(t, timetable.CareDayNotScheduled, roster.Rows[0].CareDayStatus)
	assert.False(t, deps.careDayService.Expected(roster.Rows[0].CareDayStatus))
}

// The counterpart: a completed row without the marker reports "unknown" rather
// than a re-derived plan verdict, so a plan edit cannot retroactively push a
// finished row out of the expected block either.
func TestTimetableOperationsRosterCompletedWithoutMarkerReportsUnknown(t *testing.T) {
	t.Parallel()

	instanceID := int64(367)
	deps := newTimetableOpsDeps()
	wireAssignedStaff(deps, 657, 457, 247, instanceID)
	completed := instanceWithTimes(instanceID, scheduleModels.InstanceStatusCompleted,
		time.Date(2026, time.May, 10, 14, 0, 0, 0, time.UTC),
		time.Date(2026, time.May, 10, 15, 0, 0, 0, time.UTC))
	completed.ID = instanceID
	deps.instanceRepo.byID[instanceID] = completed
	deps.studentRepo.byInstance[instanceID] = []*scheduleModels.InstanceStudent{
		{StudentID: 537, Status: scheduleModels.AttendanceStatusAbsent},
	}
	deps.students.byID[537] = &usersModels.Student{PersonID: 467, SchoolClass: "3a"}
	deps.personService.people[467] = &usersModels.Person{FirstName: "Ole", LastName: "Ohm"}
	deps.careDayService.byStudent[537] = timetable.CareDayNotScheduled

	roster, err := deps.service.Roster(context.Background(), 657, false, instanceID)

	require.NoError(t, err)
	require.Len(t, roster.Rows, 1)
	assert.Equal(t, timetable.CareDayUnknown, roster.Rows[0].CareDayStatus)
}

// A broad day status (sick / excused / class trip) stamps every expected row of
// the day, including days the care plan never booked. Until the block ends and
// MarkNotScheduled undoes it, that absence is a claim about care that was never
// owed — the roster has to report the non-booking verdict so the frontend groups
// the row under "Heute nicht eingeplant" instead of "Abwesend" (#1747 review).
func TestTimetableOperationsRosterReportsStatusDayAbsenceOnUnbookedDay(t *testing.T) {
	t.Parallel()

	statusDayID := int64(9100)
	instanceID := int64(368)
	deps := newTimetableOpsDeps()
	wireAssignedStaff(deps, 658, 458, 248, instanceID)
	deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, 268)
	deps.studentRepo.byInstance[instanceID] = []*scheduleModels.InstanceStudent{
		// Owned by a day status, on a day the plan does not book.
		{StudentID: 538, Status: scheduleModels.AttendanceStatusAbsent, StudentStatusDayID: &statusDayID},
		// Same verdict, but the absence is a human decision: it stays an absence.
		{StudentID: 539, Status: scheduleModels.AttendanceStatusAbsent},
	}
	deps.students.byID[538] = &usersModels.Student{PersonID: 468, SchoolClass: "3a"}
	deps.students.byID[539] = &usersModels.Student{PersonID: 469, SchoolClass: "3a"}
	deps.personService.people[468] = &usersModels.Person{FirstName: "Pia", LastName: "Plan"}
	deps.personService.people[469] = &usersModels.Person{FirstName: "Rudi", LastName: "Rot"}
	deps.careDayService.byStudent[538] = timetable.CareDayNotScheduled
	deps.careDayService.byStudent[539] = timetable.CareDayNotScheduled

	roster, err := deps.service.Roster(context.Background(), 658, false, instanceID)

	require.NoError(t, err)
	require.Len(t, roster.Rows, 2)
	byStudent := map[int64]timetable.CareDayStatus{}
	for _, row := range roster.Rows {
		byStudent[row.StudentID] = row.CareDayStatus
	}
	assert.Equal(t, timetable.CareDayNotScheduled, byStudent[538])
	assert.Equal(t, timetable.CareDayUnknown, byStudent[539])
}

// The planned-now card counts the same rows the roster groups: a status-day
// absence on an unbooked day belongs under "nicht eingeplant", or the card
// reports 0 while the slide-over shows one (#1747 review).
func TestTimetableOperationsRosterFlagsArrivalAndClassMismatch(t *testing.T) {
	t.Parallel()

	instanceID := int64(361)
	activeGroupID := int64(261)
	activityGroupID := int64(271)
	expectedGroupID := int64(281)
	actualGroupID := int64(282)
	studentID := int64(532)
	deps := newTimetableOpsDeps()
	wireAssignedStaff(deps, 651, 451, 241, instanceID)
	deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, activeGroupID)
	deps.instanceRepo.byID[instanceID].ActivityGroupID = &activityGroupID
	deps.studentRepo.byInstance[instanceID] = []*scheduleModels.InstanceStudent{
		{StudentID: studentID, Status: scheduleModels.AttendanceStatusExpected},
	}
	deps.activityGroups.byID[activityGroupID] = &activitiesModels.Group{EducationGroupID: &expectedGroupID}
	deps.students.byID[studentID] = &usersModels.Student{PersonID: 462, SchoolClass: "3b", GroupID: &actualGroupID}
	deps.personService.people[462] = &usersModels.Person{FirstName: "Nina", LastName: "Nachmittag"}
	deps.groups.names[expectedGroupID] = "Klasse 2a"
	deps.groups.names[actualGroupID] = "Klasse 3b"
	lateArrival := time.Date(2000, time.January, 1, 14, 30, 0, 0, time.UTC)
	deps.arrivalService.byStudent[studentID] = &compose.ExpectedArrival{ArrivalTime: &lateArrival}

	roster, err := deps.service.Roster(context.Background(), 651, false, instanceID)

	require.NoError(t, err)
	require.Len(t, roster.Rows, 1)
	require.Len(t, roster.Rows[0].Warnings, 2)
	assert.Equal(t, "arrival_after_slot_start", roster.Rows[0].Warnings[0].Kind)
	assert.Equal(t, "14:30", *roster.Rows[0].Warnings[0].ExpectedArrival)
	assert.Equal(t, "14:00", *roster.Rows[0].Warnings[0].SlotStart)
	assert.Equal(t, "template_class_mismatch", roster.Rows[0].Warnings[1].Kind)
	assert.Equal(t, expectedGroupID, *roster.Rows[0].Warnings[1].ExpectedGroupID)
	assert.Equal(t, "Klasse 2a", *roster.Rows[0].Warnings[1].ExpectedGroupName)
	assert.Equal(t, actualGroupID, *roster.Rows[0].Warnings[1].CurrentEducationGroup)
}

func TestTimetableOperationsRosterChecksAllGroupTargets(t *testing.T) {
	t.Parallel()

	instanceID := int64(363)
	activeGroupID := int64(263)
	activityGroupID := int64(273)
	firstGroupID := int64(283)
	secondGroupID := int64(284)
	studentID := int64(534)
	outsideStudentID := int64(535)
	outsideGroupID := int64(285)
	deps := newTimetableOpsDeps()
	wireAssignedStaff(deps, 653, 453, 243, instanceID)
	deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, activeGroupID)
	deps.instanceRepo.byID[instanceID].ActivityGroupID = &activityGroupID
	deps.studentRepo.byInstance[instanceID] = []*scheduleModels.InstanceStudent{
		{StudentID: studentID, Status: scheduleModels.AttendanceStatusExpected},
		{StudentID: outsideStudentID, Status: scheduleModels.AttendanceStatusExpected},
	}
	deps.activityGroups.byID[activityGroupID] = &activitiesModels.Group{EducationGroupID: &firstGroupID}
	deps.activityGroups.targetsByGroup[activityGroupID] = []*activitiesModels.GroupTarget{
		{TargetGroupType: activitiesModels.TargetGroupTypeGruppe, EducationGroupID: &firstGroupID},
		{TargetGroupType: activitiesModels.TargetGroupTypeGruppe, EducationGroupID: &secondGroupID},
	}
	deps.students.byID[studentID] = &usersModels.Student{PersonID: 464, GroupID: &secondGroupID}
	deps.students.byID[outsideStudentID] = &usersModels.Student{PersonID: 465, GroupID: &outsideGroupID}
	deps.personService.people[464] = &usersModels.Person{FirstName: "Mia", LastName: "Mehrfach"}
	deps.personService.people[465] = &usersModels.Person{FirstName: "Noah", LastName: "Außerhalb"}

	roster, err := deps.service.Roster(context.Background(), 653, false, instanceID)

	require.NoError(t, err)
	require.Len(t, roster.Rows, 2)
	rowsByStudent := make(map[int64]timetable.OperationRosterRow, len(roster.Rows))
	for _, row := range roster.Rows {
		rowsByStudent[row.StudentID] = row
	}
	for _, warning := range rowsByStudent[studentID].Warnings {
		assert.NotEqual(t, "template_class_mismatch", warning.Kind)
	}
	var mismatch *timetable.OperationRosterWarning
	outsideWarnings := rowsByStudent[outsideStudentID].Warnings
	for i := range outsideWarnings {
		if outsideWarnings[i].Kind == "template_class_mismatch" {
			mismatch = &outsideWarnings[i]
			break
		}
	}
	require.NotNil(t, mismatch)
	assert.Nil(t, mismatch.ExpectedGroupID)
	assert.Nil(t, mismatch.ExpectedGroupName)
	assert.Equal(t, outsideGroupID, *mismatch.CurrentEducationGroup)
}

func TestTimetableOperationsRosterWarningsBranches(t *testing.T) {
	t.Parallel()

	instanceID := int64(362)
	activeGroupID := int64(262)
	studentID := int64(533)

	t.Run("missing arrival schedule warning is skipped for exceptions", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		wireAssignedStaff(deps, 652, 452, 242, instanceID)
		deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, activeGroupID)
		deps.studentRepo.byInstance[instanceID] = []*scheduleModels.InstanceStudent{
			{StudentID: studentID, Status: scheduleModels.AttendanceStatusExpected},
		}
		deps.students.byID[studentID] = &usersModels.Student{PersonID: 463, SchoolClass: "3c"}
		deps.personService.people[463] = &usersModels.Person{FirstName: "Kai", LastName: "Kurz"}
		deps.arrivalService.byStudent[studentID] = &compose.ExpectedArrival{IsException: true}

		roster, err := deps.service.Roster(context.Background(), 652, false, instanceID)

		require.NoError(t, err)
		require.Len(t, roster.Rows, 1)
		assert.Empty(t, roster.Rows[0].Warnings)
	})

	t.Run("arrival lookup errors do not break roster building", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		wireAssignedStaff(deps, 653, 453, 243, instanceID)
		deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, activeGroupID)
		deps.studentRepo.byInstance[instanceID] = []*scheduleModels.InstanceStudent{
			{StudentID: studentID, Status: scheduleModels.AttendanceStatusExpected},
		}
		deps.students.byID[studentID] = &usersModels.Student{PersonID: 464, SchoolClass: "3c"}
		deps.personService.people[464] = &usersModels.Person{FirstName: "Eli", LastName: "Error"}
		deps.arrivalService.err = errors.New("arrival failed")

		roster, err := deps.service.Roster(context.Background(), 653, false, instanceID)

		require.NoError(t, err)
		require.Len(t, roster.Rows, 1)
		assert.Empty(t, roster.Rows[0].Warnings)
	})
}

func TestTimetableOperationsRosterByActiveGroupReturnsNotFound(t *testing.T) {
	t.Parallel()

	deps := newTimetableOpsDeps()

	result, err := deps.service.RosterByActiveGroup(context.Background(), 674, false, 295)

	require.ErrorIs(t, err, timetable.ErrTimetableOperationNotFound)
	assert.Nil(t, result)
}

func TestTimetableOperationsRosterByActiveGroupSuccess(t *testing.T) {
	t.Parallel()

	instanceID := int64(416)
	activeGroupID := int64(301)
	deps := newTimetableOpsDeps()
	wireAssignedStaff(deps, 684, 506, 264, instanceID)
	deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, activeGroupID)

	roster, err := deps.service.RosterByActiveGroup(context.Background(), 684, false, activeGroupID)

	require.NoError(t, err)
	require.NotNil(t, roster)
	assert.Equal(t, instanceID, roster.Instance.ID)
}
