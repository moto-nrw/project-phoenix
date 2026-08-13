package schedule

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/constants"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModel "github.com/moto-nrw/project-phoenix/models/active"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	educationModel "github.com/moto-nrw/project-phoenix/models/education"
	facilitiesModel "github.com/moto-nrw/project-phoenix/models/facilities"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModel "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestTimetableOperationsPlannedNowFiltersByAssignmentAndWindow(t *testing.T) {
	now := time.Date(2026, time.May, 10, 14, 0, 0, 0, time.UTC)
	assignedID := int64(210)
	absentID := int64(211)
	instanceID := int64(320)
	outsideWindowID := int64(321)

	deps := newTimetableOpsDeps()
	deps.personService.accountPerson = &usersModel.Person{}
	deps.personService.accountPerson.ID = 410
	deps.personService.staffByPersonID[410] = &usersModel.Staff{}
	deps.personService.staffByPersonID[410].ID = assignedID
	deps.instanceRepo.byDate = []*scheduleModel.ActivityInstance{
		instanceWithTimes(instanceID, scheduleModel.InstanceStatusPlanned, now.Add(10*time.Minute), now.Add(time.Hour)),
		instanceWithTimes(outsideWindowID, scheduleModel.InstanceStatusPlanned, now.Add(20*time.Minute), now.Add(time.Hour)),
	}
	deps.staffRepo.byInstance[instanceID] = []*scheduleModel.InstanceStaff{
		{StaffID: assignedID, IsPrimary: true},
		{StaffID: absentID, IsAbsent: true},
	}
	deps.staffRepo.byInstance[outsideWindowID] = []*scheduleModel.InstanceStaff{{StaffID: assignedID}}
	deps.studentRepo.byInstance[instanceID] = []*scheduleModel.InstanceStudent{
		{StudentID: 520, Status: scheduleModel.AttendanceStatusExpected},
		{StudentID: 521, Status: scheduleModel.AttendanceStatusPresent},
	}

	result, err := deps.service.PlannedNow(context.Background(), 610, false, timezone.DateFromTime(now), now, PlannedNowOptions{})
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
}

func TestTimetableOperationsPlannedNowAllowsAdminOverview(t *testing.T) {
	now := time.Date(2026, time.May, 10, 14, 0, 0, 0, time.UTC)
	deps := newTimetableOpsDeps()
	deps.settings.enabled = true
	deps.instanceRepo.byDate = []*scheduleModel.ActivityInstance{
		instanceWithTimes(330, scheduleModel.InstanceStatusPlanned, now.Add(-time.Minute), now.Add(time.Hour)),
	}
	deps.staffRepo.byInstance[330] = []*scheduleModel.InstanceStaff{{StaffID: 220}}

	result, err := deps.service.PlannedNow(context.Background(), 620, true, timezone.DateFromTime(now), now, PlannedNowOptions{})

	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.True(t, result[0].IsOverdue)
}

// Since #2161 the Schulhof is a regular plannable room: its planned blocks
// appear in the "Jetzt starten" list exactly like any other room's.
func TestTimetableOperationsPlannedNowIncludesSchulhof(t *testing.T) {
	now := time.Date(2026, time.May, 10, 14, 0, 0, 0, time.UTC)
	const schulhofRoomID int64 = 811
	deps := newTimetableOpsDeps()
	deps.settings.enabled = true
	deps.rooms.rooms = append(deps.rooms.rooms, &facilitiesModel.Room{
		Model: modelBase.Model{ID: schulhofRoomID},
		Name:  constants.SchulhofRoomName,
	})
	deps.instanceRepo.byDate = []*scheduleModel.ActivityInstance{
		instanceWithRoomAndTimes(330, schulhofRoomID, scheduleModel.InstanceStatusPlanned, now.Add(-time.Minute), now.Add(time.Hour)),
		instanceWithTimes(331, scheduleModel.InstanceStatusPlanned, now.Add(-time.Minute), now.Add(time.Hour)),
	}
	deps.staffRepo.byInstance[330] = []*scheduleModel.InstanceStaff{{StaffID: 220}}
	deps.staffRepo.byInstance[331] = []*scheduleModel.InstanceStaff{{StaffID: 220}}

	result, err := deps.service.PlannedNow(context.Background(), 620, true, timezone.DateFromTime(now), now, PlannedNowOptions{})

	require.NoError(t, err)
	require.Len(t, result, 2)
	ids := []int64{result[0].ID, result[1].ID}
	assert.ElementsMatch(t, []int64{330, 331}, ids)
}

func TestTimetableOperationsPlannedNowUsesInstanceDate(t *testing.T) {
	t.Run("does not return future-date instances as overdue today", func(t *testing.T) {
		now := time.Date(2026, time.May, 10, 14, 0, 0, 0, time.UTC)
		tomorrowStart := time.Date(2026, time.May, 11, 8, 0, 0, 0, time.UTC)
		deps := newTimetableOpsDeps()
		deps.settings.enabled = true
		deps.instanceRepo.byDate = []*scheduleModel.ActivityInstance{
			instanceWithTimes(334, scheduleModel.InstanceStatusPlanned, tomorrowStart, tomorrowStart.Add(time.Hour)),
		}
		deps.staffRepo.byInstance[334] = []*scheduleModel.InstanceStaff{{StaffID: 224}}

		result, err := deps.service.PlannedNow(context.Background(), 625, true, timezone.DateFromTime(tomorrowStart), now, PlannedNowOptions{})

		require.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("calculates overdue metadata from instance date", func(t *testing.T) {
		now := time.Date(2026, time.May, 10, 23, 55, 0, 0, time.UTC)
		tomorrowStart := time.Date(2026, time.May, 11, 0, 5, 0, 0, time.UTC)
		inst := instanceWithTimes(335, scheduleModel.InstanceStatusPlanned, tomorrowStart, tomorrowStart.Add(time.Hour))

		result := mapPlannedInstance(inst, []*scheduleModel.InstanceStaff{{StaffID: 225}}, nil, now, 225, nil, nil)

		assert.False(t, result.IsOverdue)
		assert.Equal(t, 10, result.MinutesUntilStart)
	})
}

func TestTimetableOperationsPlannedNowErrorBranches(t *testing.T) {
	now := time.Date(2026, time.May, 10, 14, 0, 0, 0, time.UTC)

	t.Run("forbids accounts without staff when admin overview is disabled", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		deps.personService.accountErr = usersSvc.ErrPersonNotFound

		result, err := deps.service.PlannedNow(context.Background(), 621, false, timezone.DateFromTime(now), now, PlannedNowOptions{})

		require.ErrorIs(t, err, ErrTimetableOperationForbidden)
		assert.Nil(t, result)
	})

	t.Run("admin overview allows missing person profile", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		deps.settings.enabled = true
		deps.personService.accountErr = usersSvc.ErrPersonNotFound
		deps.instanceRepo.byDate = []*scheduleModel.ActivityInstance{
			instanceWithTimes(336, scheduleModel.InstanceStatusPlanned, now.Add(-time.Minute), now.Add(time.Hour)),
		}
		deps.staffRepo.byInstance[336] = []*scheduleModel.InstanceStaff{{StaffID: 226}}

		result, err := deps.service.PlannedNow(context.Background(), 626, true, timezone.DateFromTime(now), now, PlannedNowOptions{})

		require.NoError(t, err)
		require.Len(t, result, 1)
		assert.Equal(t, int64(336), result[0].ID)
	})

	t.Run("propagates unexpected person lookup errors", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		deps.personService.accountErr = errors.New("person lookup failed")

		result, err := deps.service.PlannedNow(context.Background(), 627, true, timezone.DateFromTime(now), now, PlannedNowOptions{})

		require.EqualError(t, err, "person lookup failed")
		assert.Nil(t, result)
	})

	t.Run("propagates instance listing errors", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		wireAssignedStaff(deps, 622, 431, 221, 331)
		deps.instanceRepo.findByDateErr = errors.New("date query failed")

		result, err := deps.service.PlannedNow(context.Background(), 622, false, timezone.DateFromTime(now), now, PlannedNowOptions{})

		require.EqualError(t, err, "date query failed")
		assert.Nil(t, result)
	})

	t.Run("propagates staff lookup errors", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		wireAssignedStaff(deps, 623, 432, 222, 332)
		deps.instanceRepo.byDate = []*scheduleModel.ActivityInstance{
			instanceWithTimes(332, scheduleModel.InstanceStatusPlanned, now, now.Add(time.Hour)),
		}
		deps.staffRepo.err = errors.New("staff query failed")

		result, err := deps.service.PlannedNow(context.Background(), 623, false, timezone.DateFromTime(now), now, PlannedNowOptions{})

		require.EqualError(t, err, "staff query failed")
		assert.Nil(t, result)
	})

	t.Run("propagates student lookup errors", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		wireAssignedStaff(deps, 624, 433, 223, 333)
		deps.instanceRepo.byDate = []*scheduleModel.ActivityInstance{
			instanceWithTimes(333, scheduleModel.InstanceStatusPlanned, now, now.Add(time.Hour)),
		}
		deps.studentRepo.err = errors.New("student query failed")

		result, err := deps.service.PlannedNow(context.Background(), 624, false, timezone.DateFromTime(now), now, PlannedNowOptions{})

		require.EqualError(t, err, "student query failed")
		assert.Nil(t, result)
	})

	t.Run("propagates room lookup errors", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		wireAssignedStaff(deps, 629, 435, 228, 339)
		deps.instanceRepo.byDate = []*scheduleModel.ActivityInstance{
			instanceWithTimes(339, scheduleModel.InstanceStatusPlanned, now, now.Add(time.Hour)),
		}
		deps.rooms.err = errors.New("room query failed")

		result, err := deps.service.PlannedNow(context.Background(), 629, false, timezone.DateFromTime(now), now, PlannedNowOptions{})

		require.EqualError(t, err, "room query failed")
		assert.Nil(t, result)
	})
}

func TestTimetableOperationsPlannedNowSupportsUpcomingOptions(t *testing.T) {
	now := time.Date(2026, time.May, 10, 14, 0, 0, 0, time.UTC)
	deps := newTimetableOpsDeps()
	wireAssignedStaff(deps, 628, 434, 227, 337)
	deps.instanceRepo.byDate = []*scheduleModel.ActivityInstance{
		instanceWithTimes(337, scheduleModel.InstanceStatusPlanned, now.Add(90*time.Minute), now.Add(2*time.Hour)),
		instanceWithTimes(338, scheduleModel.InstanceStatusPlanned, now.Add(100*time.Minute), now.Add(2*time.Hour)),
	}
	for _, inst := range deps.instanceRepo.byDate {
		deps.instanceRepo.byID[inst.ID] = inst
	}
	deps.staffRepo.byInstance[338] = []*scheduleModel.InstanceStaff{{StaffID: 227}}
	deps.studentRepo.byInstance[337] = []*scheduleModel.InstanceStudent{
		{StudentID: 527, Status: scheduleModel.AttendanceStatusExpected},
	}
	deps.students.byID[527] = &usersModel.Student{PersonID: 437, SchoolClass: "2a"}
	deps.personService.people[437] = &usersModel.Person{FirstName: "Lina", LastName: "Lang"}

	result, err := deps.service.PlannedNow(context.Background(), 628, false, timezone.DateFromTime(now), now, PlannedNowOptions{
		HorizonMinutes: 120,
		Limit:          1,
		IncludeRoster:  true,
	})

	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, int64(337), result[0].ID)
	require.Len(t, result[0].RosterPreview, 1)
	assert.Equal(t, "Lina Lang", result[0].RosterPreview[0].StudentName)
}

func TestTimetableOperationsStartRequiresAStaffIdentity(t *testing.T) {
	deps := newTimetableOpsDeps()
	deps.settings.enabled = true
	deps.personService.accountPerson = &usersModel.Person{}
	deps.personService.accountPerson.ID = 430

	result, err := deps.service.Start(context.Background(), 630, true, 340)

	require.ErrorIs(t, err, ErrTimetableOperationForbidden)
	assert.Nil(t, result)
	assert.Empty(t, deps.instanceService.started)
}

func TestTimetableOperationsStartDelegatesWhenStaffIsAssigned(t *testing.T) {
	staffID := int64(230)
	instanceID := int64(350)
	deps := newTimetableOpsDeps()
	deps.personService.accountPerson = &usersModel.Person{}
	deps.personService.accountPerson.ID = 440
	deps.personService.staffByPersonID[440] = &usersModel.Staff{}
	deps.personService.staffByPersonID[440].ID = staffID
	deps.instanceRepo.byID[instanceID] = instanceWithTimes(instanceID, scheduleModel.InstanceStatusPlanned, time.Now(), time.Now().Add(time.Hour))
	deps.staffRepo.byInstance[instanceID] = []*scheduleModel.InstanceStaff{{StaffID: staffID}}

	result, err := deps.service.Start(context.Background(), 640, false, instanceID)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, instanceID, deps.instanceService.started[0].instanceID)
	assert.Equal(t, staffID, deps.instanceService.started[0].staffID)
}

func TestTimetableOperationsRosterCombinesPlannedStudentsAndLiveDropIns(t *testing.T) {
	instanceID := int64(360)
	activeGroupID := int64(260)
	groupID := int64(270)
	visitID := int64(370)
	deps := newTimetableOpsDeps()
	wireAssignedStaff(deps, 650, 450, 240, instanceID)
	deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, activeGroupID)
	deps.studentRepo.byInstance[instanceID] = []*scheduleModel.InstanceStudent{
		{StudentID: 530, Status: scheduleModel.AttendanceStatusExpected},
	}
	deps.visitRepo.byActiveGroup[activeGroupID] = []*activeModel.Visit{
		{StudentID: 531, ActiveGroupID: activeGroupID, EntryTime: time.Date(2026, time.May, 10, 14, 5, 0, 0, time.UTC)},
	}
	deps.visitRepo.byActiveGroup[activeGroupID][0].ID = visitID
	deps.students.byID[530] = &usersModel.Student{PersonID: 460, SchoolClass: "3a", GroupID: &groupID}
	deps.students.byID[531] = &usersModel.Student{PersonID: 461, SchoolClass: "4b"}
	deps.personService.people[460] = &usersModel.Person{FirstName: "Zoe", LastName: "Zimmer"}
	deps.personService.people[461] = &usersModel.Person{FirstName: "Anna", LastName: "Anlauf"}
	deps.groups.byID[groupID] = &educationModel.Group{Name: "OGS Blau"}

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

// A completed block's verdict is frozen in the stored marker: the care plan may
// have been edited or deleted since, and reading it here would relabel a
// historical row while the weekly list, parent calendar, and attendance history
// keep the completion-time answer (#1747 review).
func TestTimetableOperationsRosterFreezesCareDayVerdictOnCompletedInstance(t *testing.T) {
	instanceID := int64(366)
	deps := newTimetableOpsDeps()
	wireAssignedStaff(deps, 656, 456, 246, instanceID)
	completed := instanceWithTimes(instanceID, scheduleModel.InstanceStatusCompleted,
		time.Date(2026, time.May, 10, 14, 0, 0, 0, time.UTC),
		time.Date(2026, time.May, 10, 15, 0, 0, 0, time.UTC))
	completed.ID = instanceID
	deps.instanceRepo.byID[instanceID] = completed
	deps.studentRepo.byInstance[instanceID] = []*scheduleModel.InstanceStudent{
		{StudentID: 536, Status: scheduleModel.AttendanceStatusExpected, NotScheduled: true},
	}
	deps.students.byID[536] = &usersModel.Student{PersonID: 466, SchoolClass: "3a"}
	deps.personService.people[466] = &usersModel.Person{FirstName: "Nora", LastName: "Neu"}
	// The plan says "booked" today — a later edit. It must not win over the marker.
	deps.careDayService.byStudent[536] = CareDayScheduled

	roster, err := deps.service.Roster(context.Background(), 656, false, instanceID)

	require.NoError(t, err)
	require.Len(t, roster.Rows, 1)
	assert.Equal(t, CareDayNotScheduled, roster.Rows[0].CareDayStatus)
	assert.False(t, roster.Rows[0].CareDayStatus.Expected())
}

// The counterpart: a completed row without the marker reports "unknown" rather
// than a re-derived plan verdict, so a plan edit cannot retroactively push a
// finished row out of the expected block either.
func TestTimetableOperationsRosterCompletedWithoutMarkerReportsUnknown(t *testing.T) {
	instanceID := int64(367)
	deps := newTimetableOpsDeps()
	wireAssignedStaff(deps, 657, 457, 247, instanceID)
	completed := instanceWithTimes(instanceID, scheduleModel.InstanceStatusCompleted,
		time.Date(2026, time.May, 10, 14, 0, 0, 0, time.UTC),
		time.Date(2026, time.May, 10, 15, 0, 0, 0, time.UTC))
	completed.ID = instanceID
	deps.instanceRepo.byID[instanceID] = completed
	deps.studentRepo.byInstance[instanceID] = []*scheduleModel.InstanceStudent{
		{StudentID: 537, Status: scheduleModel.AttendanceStatusAbsent},
	}
	deps.students.byID[537] = &usersModel.Student{PersonID: 467, SchoolClass: "3a"}
	deps.personService.people[467] = &usersModel.Person{FirstName: "Ole", LastName: "Ohm"}
	deps.careDayService.byStudent[537] = CareDayNotScheduled

	roster, err := deps.service.Roster(context.Background(), 657, false, instanceID)

	require.NoError(t, err)
	require.Len(t, roster.Rows, 1)
	assert.Equal(t, CareDayUnknown, roster.Rows[0].CareDayStatus)
}

// A broad day status (sick / excused / class trip) stamps every expected row of
// the day, including days the care plan never booked. Until the block ends and
// MarkNotScheduled undoes it, that absence is a claim about care that was never
// owed — the roster has to report the non-booking verdict so the frontend groups
// the row under "Heute nicht eingeplant" instead of "Abwesend" (#1747 review).
func TestTimetableOperationsRosterReportsStatusDayAbsenceOnUnbookedDay(t *testing.T) {
	statusDayID := int64(9100)
	instanceID := int64(368)
	deps := newTimetableOpsDeps()
	wireAssignedStaff(deps, 658, 458, 248, instanceID)
	deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, 268)
	deps.studentRepo.byInstance[instanceID] = []*scheduleModel.InstanceStudent{
		// Owned by a day status, on a day the plan does not book.
		{StudentID: 538, Status: scheduleModel.AttendanceStatusAbsent, StudentStatusDayID: &statusDayID},
		// Same verdict, but the absence is a human decision: it stays an absence.
		{StudentID: 539, Status: scheduleModel.AttendanceStatusAbsent},
	}
	deps.students.byID[538] = &usersModel.Student{PersonID: 468, SchoolClass: "3a"}
	deps.students.byID[539] = &usersModel.Student{PersonID: 469, SchoolClass: "3a"}
	deps.personService.people[468] = &usersModel.Person{FirstName: "Pia", LastName: "Plan"}
	deps.personService.people[469] = &usersModel.Person{FirstName: "Rudi", LastName: "Rot"}
	deps.careDayService.byStudent[538] = CareDayNotScheduled
	deps.careDayService.byStudent[539] = CareDayNotScheduled

	roster, err := deps.service.Roster(context.Background(), 658, false, instanceID)

	require.NoError(t, err)
	require.Len(t, roster.Rows, 2)
	byStudent := map[int64]CareDayStatus{}
	for _, row := range roster.Rows {
		byStudent[row.StudentID] = row.CareDayStatus
	}
	assert.Equal(t, CareDayNotScheduled, byStudent[538])
	assert.Equal(t, CareDayUnknown, byStudent[539])
}

// The planned-now card counts the same rows the roster groups: a status-day
// absence on an unbooked day belongs under "nicht eingeplant", or the card
// reports 0 while the slide-over shows one (#1747 review).
func TestTimetableOperationsPlannedCardCountsStatusDayNonBookings(t *testing.T) {
	statusDayID := int64(9101)
	now := time.Date(2026, time.May, 10, 14, 0, 0, 0, time.UTC)
	inst := instanceWithTimes(369, scheduleModel.InstanceStatusPlanned, now, now.Add(time.Hour))
	rows := []*scheduleModel.InstanceStudent{
		{StudentID: 540, Status: scheduleModel.AttendanceStatusAbsent, StudentStatusDayID: &statusDayID},
		{StudentID: 541, Status: scheduleModel.AttendanceStatusAbsent},
		{StudentID: 542, Status: scheduleModel.AttendanceStatusExpected},
		{StudentID: 543, Status: scheduleModel.AttendanceStatusExpected},
	}
	careDay := map[int64]CareDayStatus{
		540: CareDayNotScheduled,
		541: CareDayNotScheduled,
		542: CareDayNotScheduled,
		543: CareDayScheduled,
	}

	result := mapPlannedInstance(inst, []*scheduleModel.InstanceStaff{{StaffID: 249}}, rows, now, 249, nil, careDay)

	assert.Equal(t, 1, result.ExpectedStudentsCount)
	assert.Equal(t, 0, result.PresentStudentsCount)
	assert.Equal(t, 2, result.NotScheduledCount, "the status-day non-booking and the unbooked expected row")
}

func TestTimetableOperationsRosterFlagsArrivalAndClassMismatch(t *testing.T) {
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
	deps.studentRepo.byInstance[instanceID] = []*scheduleModel.InstanceStudent{
		{StudentID: studentID, Status: scheduleModel.AttendanceStatusExpected},
	}
	deps.activityGroups.byID[activityGroupID] = &activitiesModel.Group{EducationGroupID: &expectedGroupID}
	deps.students.byID[studentID] = &usersModel.Student{PersonID: 462, SchoolClass: "3b", GroupID: &actualGroupID}
	deps.personService.people[462] = &usersModel.Person{FirstName: "Nina", LastName: "Nachmittag"}
	deps.groups.byID[expectedGroupID] = &educationModel.Group{Name: "Klasse 2a"}
	deps.groups.byID[actualGroupID] = &educationModel.Group{Name: "Klasse 3b"}
	lateArrival := time.Date(2000, time.January, 1, 14, 30, 0, 0, time.UTC)
	deps.arrivalService.byStudent[studentID] = &EffectiveArrivalTime{ArrivalTime: &lateArrival}

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
	deps.studentRepo.byInstance[instanceID] = []*scheduleModel.InstanceStudent{
		{StudentID: studentID, Status: scheduleModel.AttendanceStatusExpected},
		{StudentID: outsideStudentID, Status: scheduleModel.AttendanceStatusExpected},
	}
	deps.activityGroups.byID[activityGroupID] = &activitiesModel.Group{EducationGroupID: &firstGroupID}
	deps.activityGroups.targetsByGroup[activityGroupID] = []*activitiesModel.GroupTarget{
		{TargetGroupType: activitiesModel.TargetGroupTypeGruppe, EducationGroupID: &firstGroupID},
		{TargetGroupType: activitiesModel.TargetGroupTypeGruppe, EducationGroupID: &secondGroupID},
	}
	deps.students.byID[studentID] = &usersModel.Student{PersonID: 464, GroupID: &secondGroupID}
	deps.students.byID[outsideStudentID] = &usersModel.Student{PersonID: 465, GroupID: &outsideGroupID}
	deps.personService.people[464] = &usersModel.Person{FirstName: "Mia", LastName: "Mehrfach"}
	deps.personService.people[465] = &usersModel.Person{FirstName: "Noah", LastName: "Außerhalb"}

	roster, err := deps.service.Roster(context.Background(), 653, false, instanceID)

	require.NoError(t, err)
	require.Len(t, roster.Rows, 2)
	rowsByStudent := make(map[int64]OperationRosterRow, len(roster.Rows))
	for _, row := range roster.Rows {
		rowsByStudent[row.StudentID] = row
	}
	for _, warning := range rowsByStudent[studentID].Warnings {
		assert.NotEqual(t, "template_class_mismatch", warning.Kind)
	}
	var mismatch *OperationRosterWarning
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
	instanceID := int64(362)
	activeGroupID := int64(262)
	studentID := int64(533)

	t.Run("missing arrival schedule warning is skipped for exceptions", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		wireAssignedStaff(deps, 652, 452, 242, instanceID)
		deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, activeGroupID)
		deps.studentRepo.byInstance[instanceID] = []*scheduleModel.InstanceStudent{
			{StudentID: studentID, Status: scheduleModel.AttendanceStatusExpected},
		}
		deps.students.byID[studentID] = &usersModel.Student{PersonID: 463, SchoolClass: "3c"}
		deps.personService.people[463] = &usersModel.Person{FirstName: "Kai", LastName: "Kurz"}
		deps.arrivalService.byStudent[studentID] = &EffectiveArrivalTime{IsException: true}

		roster, err := deps.service.Roster(context.Background(), 652, false, instanceID)

		require.NoError(t, err)
		require.Len(t, roster.Rows, 1)
		assert.Empty(t, roster.Rows[0].Warnings)
	})

	t.Run("arrival lookup errors do not break roster building", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		wireAssignedStaff(deps, 653, 453, 243, instanceID)
		deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, activeGroupID)
		deps.studentRepo.byInstance[instanceID] = []*scheduleModel.InstanceStudent{
			{StudentID: studentID, Status: scheduleModel.AttendanceStatusExpected},
		}
		deps.students.byID[studentID] = &usersModel.Student{PersonID: 464, SchoolClass: "3c"}
		deps.personService.people[464] = &usersModel.Person{FirstName: "Eli", LastName: "Error"}
		deps.arrivalService.err = errors.New("arrival failed")

		roster, err := deps.service.Roster(context.Background(), 653, false, instanceID)

		require.NoError(t, err)
		require.Len(t, roster.Rows, 1)
		assert.Empty(t, roster.Rows[0].Warnings)
	})
}

func TestTimetableOperationsCheckInCreatesVisitAndMarksPlannedPresent(t *testing.T) {
	instanceID := int64(380)
	activeGroupID := int64(280)
	studentID := int64(540)
	rowID := int64(390)
	deps := newTimetableOpsDeps()
	wireAssignedStaff(deps, 660, 470, 250, instanceID)
	deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, activeGroupID)
	deps.studentRepo.byInstance[instanceID] = []*scheduleModel.InstanceStudent{
		{StudentID: studentID, Status: scheduleModel.AttendanceStatusExpected},
	}
	deps.studentRepo.byInstanceStudent[instanceStudentKey{instanceID, studentID}] = &scheduleModel.InstanceStudent{
		InstanceID: instanceID,
		StudentID:  studentID,
		Status:     scheduleModel.AttendanceStatusExpected,
	}
	deps.studentRepo.byInstanceStudent[instanceStudentKey{instanceID, studentID}].ID = rowID
	deps.students.byID[studentID] = &usersModel.Student{PersonID: 480, SchoolClass: "2c"}
	deps.personService.people[480] = &usersModel.Person{FirstName: "Mila", LastName: "Muster"}

	roster, err := deps.service.CheckInStudent(tenant.WithTenantID(context.Background(), 720), 660, false, instanceID, studentID)

	require.NoError(t, err)
	require.Len(t, deps.activeService.created, 1)
	assert.Equal(t, int64(720), deps.activeService.created[0].GetTenantID())
	assert.Equal(t, activeGroupID, deps.activeService.created[0].ActiveGroupID)
	require.Len(t, deps.studentRepo.updates, 1)
	assert.Equal(t, rowID, deps.studentRepo.updates[0].rowID)
	assert.Equal(t, scheduleModel.AttendanceStatusPresent, *deps.studentRepo.updates[0].patch.Status)
	assert.True(t, deps.studentRepo.updates[0].patch.SubstatusClear)
	assert.True(t, deps.activeGroups.lastActivity[activeGroupID].After(time.Time{}))
	assert.Equal(t, studentID, roster.Rows[0].StudentID)
}

func TestTimetableOperationsCheckInRejectsStudentActiveElsewhere(t *testing.T) {
	instanceID := int64(400)
	activeGroupID := int64(290)
	studentID := int64(550)
	deps := newTimetableOpsDeps()
	wireAssignedStaff(deps, 670, 490, 251, instanceID)
	deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, activeGroupID)
	deps.visitRepo.currentByStudent[studentID] = &activeModel.Visit{StudentID: studentID, ActiveGroupID: 291, EntryTime: time.Now()}

	_, err := deps.service.CheckInStudent(context.Background(), 670, false, instanceID, studentID)

	require.ErrorIs(t, err, ErrTimetableOperationConflict)
	assert.Empty(t, deps.activeService.created)
}

func TestTimetableOperationsCheckOutEndsMatchingVisit(t *testing.T) {
	instanceID := int64(401)
	activeGroupID := int64(292)
	studentID := int64(551)
	visitID := int64(402)
	deps := newTimetableOpsDeps()
	wireAssignedStaff(deps, 671, 491, 252, instanceID)
	deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, activeGroupID)
	deps.visitRepo.byActiveGroup[activeGroupID] = []*activeModel.Visit{{StudentID: studentID, ActiveGroupID: activeGroupID, EntryTime: time.Now()}}
	deps.visitRepo.byActiveGroup[activeGroupID][0].ID = visitID
	deps.students.byID[studentID] = &usersModel.Student{PersonID: 492, SchoolClass: "1a"}
	deps.personService.people[492] = &usersModel.Person{FirstName: "Ben", LastName: "Beispiel"}

	_, err := deps.service.CheckOutStudent(context.Background(), 671, false, instanceID, studentID)

	require.NoError(t, err)
	assert.Equal(t, []int64{visitID}, deps.activeService.ended)
}

func TestTimetableOperationsCheckOutAlreadyEndedReturnsRoster(t *testing.T) {
	instanceID := int64(413)
	activeGroupID := int64(299)
	studentID := int64(559)
	visitID := int64(414)
	deps := newTimetableOpsDeps()
	wireAssignedStaff(deps, 682, 504, 263, instanceID)
	deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, activeGroupID)
	deps.visitRepo.byActiveGroup[activeGroupID] = []*activeModel.Visit{{StudentID: studentID, ActiveGroupID: activeGroupID, EntryTime: time.Now()}}
	deps.visitRepo.byActiveGroup[activeGroupID][0].ID = visitID
	deps.activeService.endErr = activeSvc.ErrVisitAlreadyEnded

	roster, err := deps.service.CheckOutStudent(context.Background(), 682, false, instanceID, studentID)

	require.NoError(t, err)
	require.NotNil(t, roster)
	assert.Equal(t, int64(413), roster.Instance.ID)
}

func TestTimetableOperationsPatchAttendanceUpdatesRowAndBroadcasts(t *testing.T) {
	instanceID := int64(403)
	activeGroupID := int64(293)
	studentID := int64(552)
	rowID := int64(404)
	deps := newTimetableOpsDeps()
	wireAssignedStaff(deps, 672, 493, 253, instanceID)
	deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, activeGroupID)
	deps.studentRepo.byInstanceStudent[instanceStudentKey{instanceID, studentID}] = &scheduleModel.InstanceStudent{
		InstanceID: instanceID,
		StudentID:  studentID,
		Status:     scheduleModel.AttendanceStatusExpected,
	}
	deps.studentRepo.byInstanceStudent[instanceStudentKey{instanceID, studentID}].ID = rowID
	deps.studentRepo.byInstance[instanceID] = []*scheduleModel.InstanceStudent{
		deps.studentRepo.byInstanceStudent[instanceStudentKey{instanceID, studentID}],
	}
	deps.students.byID[studentID] = &usersModel.Student{PersonID: 494, SchoolClass: "1b"}
	deps.personService.people[494] = &usersModel.Person{FirstName: "Lea", LastName: "Lern"}
	status := scheduleModel.AttendanceStatusAbsent
	note := "krank gemeldet"

	row, err := deps.service.PatchAttendance(tenant.WithTenantID(context.Background(), 721), 672, false, instanceID, studentID, scheduleModel.AttendanceFieldPatch{
		Status: &status,
		Note:   &note,
	})

	require.NoError(t, err)
	require.NotNil(t, row)
	assert.Equal(t, studentID, row.StudentID)
	require.Len(t, deps.studentRepo.updates, 1)
	assert.Equal(t, rowID, deps.studentRepo.updates[0].rowID)
	assert.Equal(t, &note, deps.studentRepo.updates[0].patch.Note)
	calls := deps.broadcaster.Calls()
	require.Len(t, calls, 1)
	assert.Equal(t, int64(721), calls[0].TenantID)
	assert.Equal(t, realtime.EventActiveSupervisionChanged, calls[0].Event.Type)

	// #2085: the refresh is tenant-wide, so it must not name the child whose
	// attendance was patched — every staff client of the school receives it,
	// including colleagues outside gdpr.student_data_scope. The instance id
	// (block scope, not child identity) is what clients refetch on.
	assert.Equal(t, "tenant", calls[0].Method)
	testpkg.AssertNoTenantWideStudentIdentity(t, deps.broadcaster)
	require.NotNil(t, calls[0].Event.Data.InstanceID)
	assert.Equal(t, "403", *calls[0].Event.Data.InstanceID)
}

func TestTimetableOperationsPatchAttendanceRejectsCompletedInstance(t *testing.T) {
	instanceID := int64(427)
	studentID := int64(567)
	deps := newTimetableOpsDeps()
	wireAssignedStaff(deps, 693, 515, 274, instanceID)
	completed := activeInstance(instanceID, 305)
	completed.Status = scheduleModel.InstanceStatusCompleted
	deps.instanceRepo.byID[instanceID] = completed
	deps.studentRepo.byInstanceStudent[instanceStudentKey{instanceID, studentID}] = &scheduleModel.InstanceStudent{
		InstanceID: instanceID,
		StudentID:  studentID,
		Status:     scheduleModel.AttendanceStatusPresent,
	}
	status := scheduleModel.AttendanceStatusAbsent

	row, err := deps.service.PatchAttendance(context.Background(), 693, false, instanceID, studentID, scheduleModel.AttendanceFieldPatch{Status: &status})

	require.ErrorIs(t, err, ErrTimetableOperationConflict)
	assert.Nil(t, row)
	assert.Empty(t, deps.studentRepo.updates)
}

func TestTimetableOperationsReopenRequiresOperate(t *testing.T) {
	instanceID := int64(428)
	deps := newTimetableOpsDeps()
	completed := activeInstance(instanceID, 306)
	completed.Status = scheduleModel.InstanceStatusCompleted
	deps.instanceRepo.byID[instanceID] = completed

	result, err := deps.service.Reopen(context.Background(), 694, false, instanceID)
	require.ErrorIs(t, err, ErrTimetableOperationForbidden)
	assert.Nil(t, result)

	wireAssignedStaff(deps, 694, 516, 275, instanceID)
	result, err = deps.service.Reopen(context.Background(), 694, false, instanceID)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, instanceID, result.Instance.ID)
}

func TestTimetableOperationsCompleteDelegatesAfterPermissionCheck(t *testing.T) {
	instanceID := int64(405)
	deps := newTimetableOpsDeps()
	wireAssignedStaff(deps, 673, 495, 254, instanceID)
	deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, 294)

	result, err := deps.service.Complete(context.Background(), 673, false, instanceID)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, []int64{instanceID}, deps.instanceService.completed)
}

func TestTimetableOperationsRosterByActiveGroupReturnsNotFound(t *testing.T) {
	deps := newTimetableOpsDeps()

	result, err := deps.service.RosterByActiveGroup(context.Background(), 674, false, 295)

	require.ErrorIs(t, err, ErrTimetableOperationNotFound)
	assert.Nil(t, result)
}

func TestTimetableOperationsRosterByActiveGroupSuccess(t *testing.T) {
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

func TestTimetableOperationsPermissionBranches(t *testing.T) {
	instanceID := int64(409)
	activeGroupID := int64(296)

	t.Run("assigned active supervisor can operate without instance staff assignment", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		wireAssignedStaff(deps, 675, 496, 256, instanceID)
		deps.staffRepo.byInstance[instanceID] = nil
		deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, activeGroupID)
		deps.supervisors.byActiveGroup[activeGroupID] = []*activeModel.GroupSupervisor{{StaffID: 256}}

		_, err := deps.service.Complete(context.Background(), 675, false, instanceID)

		require.NoError(t, err)
		assert.Equal(t, []int64{instanceID}, deps.instanceService.completed)
	})

	t.Run("unassigned staff is forbidden", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		deps.settings.mode = configModel.GroupModeFixedGroups
		wireAssignedStaff(deps, 676, 497, 257, instanceID)
		deps.staffRepo.byInstance[instanceID] = []*scheduleModel.InstanceStaff{{StaffID: 258}}
		deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, activeGroupID)

		_, err := deps.service.Roster(context.Background(), 676, false, instanceID)

		require.ErrorIs(t, err, ErrTimetableOperationForbidden)
	})

	t.Run("open care allows staff without instance assignment or supervision", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		deps.settings.mode = configModel.GroupModeOpenCare
		wireAssignedStaff(deps, 693, 515, 274, instanceID)
		deps.staffRepo.byInstance[instanceID] = nil

		_, err := deps.service.Complete(context.Background(), 693, false, instanceID)

		require.NoError(t, err)
		assert.Equal(t, []int64{instanceID}, deps.instanceService.completed)
	})

	t.Run("group mode resolution failure keeps fixed-group checks", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		deps.settings.stringErr = errors.New("settings unavailable")
		wireAssignedStaff(deps, 694, 516, 275, instanceID)
		deps.staffRepo.byInstance[instanceID] = nil
		deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, activeGroupID)

		_, err := deps.service.Complete(context.Background(), 694, false, instanceID)

		require.ErrorIs(t, err, ErrTimetableOperationForbidden)
		assert.Empty(t, deps.instanceService.completed)
	})

	t.Run("missing settings keeps fixed-group checks", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		wireAssignedStaff(deps, 696, 517, 276, instanceID)
		deps.staffRepo.byInstance[instanceID] = nil
		deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, activeGroupID)
		deps.service.(*timetableOperationsService).deps.Settings = nil

		_, err := deps.service.Complete(context.Background(), 696, false, instanceID)

		require.ErrorIs(t, err, ErrTimetableOperationForbidden)
		assert.Empty(t, deps.instanceService.completed)
	})

	t.Run("open care still requires a staff identity", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		deps.settings.mode = configModel.GroupModeOpenCare

		_, err := deps.service.Roster(context.Background(), 695, false, instanceID)

		require.ErrorIs(t, err, ErrTimetableOperationForbidden)
	})

	t.Run("admin overview allows an admin without a staff identity", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		deps.settings.enabled = true

		_, err := deps.service.Complete(context.Background(), 697, true, instanceID)

		require.NoError(t, err)
		assert.Equal(t, []int64{instanceID}, deps.instanceService.completed)
	})

	t.Run("missing account id is forbidden before repository lookup", func(t *testing.T) {
		deps := newTimetableOpsDeps()

		_, err := deps.service.Roster(context.Background(), 0, false, instanceID)

		require.ErrorIs(t, err, ErrTimetableOperationForbidden)
	})

	t.Run("missing person cannot use staff-only operations", func(t *testing.T) {
		deps := newTimetableOpsDeps()

		_, err := deps.service.Start(context.Background(), 677, false, instanceID)

		require.ErrorIs(t, err, ErrTimetableOperationForbidden)
	})
}

func TestTimetableOperationsCheckInBranches(t *testing.T) {
	instanceID := int64(410)
	activeGroupID := int64(297)
	studentID := int64(557)

	t.Run("rejects planned instance", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		wireAssignedStaff(deps, 678, 498, 259, instanceID)
		deps.instanceRepo.byID[instanceID] = instanceWithTimes(instanceID, scheduleModel.InstanceStatusPlanned, time.Now(), time.Now().Add(time.Hour))

		_, err := deps.service.CheckInStudent(context.Background(), 678, false, instanceID, studentID)

		require.ErrorIs(t, err, ErrTimetableOperationConflict)
	})

	t.Run("marks planned present when active visit already belongs to same group", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		wireAssignedStaff(deps, 679, 499, 260, instanceID)
		deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, activeGroupID)
		row := &scheduleModel.InstanceStudent{InstanceID: instanceID, StudentID: studentID, Status: scheduleModel.AttendanceStatusExpected}
		row.ID = 411
		deps.studentRepo.byInstanceStudent[instanceStudentKey{instanceID, studentID}] = row
		deps.studentRepo.byInstance[instanceID] = []*scheduleModel.InstanceStudent{row}
		deps.visitRepo.currentByStudent[studentID] = &activeModel.Visit{StudentID: studentID, ActiveGroupID: activeGroupID, EntryTime: time.Now()}
		deps.visitRepo.byActiveGroup[activeGroupID] = []*activeModel.Visit{deps.visitRepo.currentByStudent[studentID]}
		deps.students.byID[studentID] = &usersModel.Student{PersonID: 500, SchoolClass: "4a"}
		deps.personService.people[500] = &usersModel.Person{FirstName: "Tom", LastName: "Test"}

		_, err := deps.service.CheckInStudent(context.Background(), 679, false, instanceID, studentID)

		require.NoError(t, err)
		assert.Empty(t, deps.activeService.created)
		require.Len(t, deps.studentRepo.updates, 1)
		assert.Equal(t, int64(411), deps.studentRepo.updates[0].rowID)
	})

	t.Run("does not fail when active group last-activity update fails", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		wireAssignedStaff(deps, 680, 501, 261, instanceID)
		deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, activeGroupID)
		deps.activeGroups.updateErr = errors.New("update failed")
		deps.students.byID[studentID] = &usersModel.Student{PersonID: 502, SchoolClass: "4a"}
		deps.personService.people[502] = &usersModel.Person{FirstName: "Noa", LastName: "Neben"}

		_, err := deps.service.CheckInStudent(context.Background(), 680, false, instanceID, studentID)

		require.NoError(t, err)
		require.Len(t, deps.activeService.created, 1)
	})

	t.Run("propagates visit creation errors", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		wireAssignedStaff(deps, 687, 509, 267, instanceID)
		deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, activeGroupID)
		deps.activeService.createErr = errors.New("create visit failed")

		result, err := deps.service.CheckInStudent(context.Background(), 687, false, instanceID, studentID)

		require.EqualError(t, err, "create visit failed")
		assert.Nil(t, result)
	})

	t.Run("propagates attendance update errors after visit creation", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		wireAssignedStaff(deps, 688, 510, 268, instanceID)
		deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, activeGroupID)
		row := &scheduleModel.InstanceStudent{InstanceID: instanceID, StudentID: studentID, Status: scheduleModel.AttendanceStatusExpected}
		row.ID = 420
		deps.studentRepo.byInstanceStudent[instanceStudentKey{instanceID, studentID}] = row
		deps.studentRepo.updateErr = errors.New("attendance update failed")

		result, err := deps.service.CheckInStudent(context.Background(), 688, false, instanceID, studentID)

		require.EqualError(t, err, "attendance update failed")
		assert.Nil(t, result)
		require.Len(t, deps.activeService.created, 1)
	})
}

func TestTimetableOperationsCheckOutBranches(t *testing.T) {
	instanceID := int64(412)
	deps := newTimetableOpsDeps()
	wireAssignedStaff(deps, 681, 503, 262, instanceID)
	deps.instanceRepo.byID[instanceID] = instanceWithTimes(instanceID, scheduleModel.InstanceStatusActive, time.Now(), time.Now().Add(time.Hour))

	_, err := deps.service.CheckOutStudent(context.Background(), 681, false, instanceID, 558)

	require.ErrorIs(t, err, ErrTimetableOperationConflict)

	deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, 298)
	_, err = deps.service.CheckOutStudent(context.Background(), 681, false, instanceID, 558)
	require.ErrorIs(t, err, ErrTimetableOperationNotFound)
}

func TestTimetableOperationsActiveVisitLookupPropagatesErrors(t *testing.T) {
	deps := newTimetableOpsDeps()
	deps.visitRepo.err = errors.New("visit query failed")

	visit, err := deps.service.(*timetableOperationsService).findActiveVisitForInstanceStudent(context.Background(), 305, 567)

	require.EqualError(t, err, "visit query failed")
	assert.Nil(t, visit)
}

func TestTimetableOperationsPatchAttendanceBranches(t *testing.T) {
	deps := newTimetableOpsDeps()
	instanceID := int64(413)
	wireAssignedStaff(deps, 682, 504, 263, instanceID)
	deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, 299)

	row, err := deps.service.PatchAttendance(context.Background(), 682, false, instanceID, 559, scheduleModel.AttendanceFieldPatch{})

	require.ErrorIs(t, err, ErrTimetableOperationNotFound)
	assert.Nil(t, row)

	t.Run("forbidden before attendance row validation", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		instanceID := int64(423)
		studentID := int64(565)
		wireAssignedStaff(deps, 691, 513, 271, instanceID)
		deps.staffRepo.byInstance[instanceID] = []*scheduleModel.InstanceStaff{{StaffID: 272}}
		deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, 303)
		excused := scheduleModel.AttendanceSubstatusExcused
		row := &scheduleModel.InstanceStudent{InstanceID: instanceID, StudentID: studentID, Status: scheduleModel.AttendanceStatusPresent, Substatus: &excused}
		row.ID = 424
		deps.studentRepo.byInstanceStudent[instanceStudentKey{instanceID, studentID}] = row
		expected := scheduleModel.AttendanceStatusExpected

		result, err := deps.service.PatchAttendance(context.Background(), 691, false, instanceID, studentID, scheduleModel.AttendanceFieldPatch{Status: &expected})

		require.ErrorIs(t, err, ErrTimetableOperationForbidden)
		assert.Nil(t, result)
		assert.Empty(t, deps.studentRepo.updates)
	})

	t.Run("authorized invalid patch returns field validation error", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		instanceID := int64(425)
		studentID := int64(566)
		wireAssignedStaff(deps, 692, 514, 273, instanceID)
		deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, 304)
		row := &scheduleModel.InstanceStudent{InstanceID: instanceID, StudentID: studentID, Status: scheduleModel.AttendanceStatusExpected}
		row.ID = 426
		deps.studentRepo.byInstanceStudent[instanceStudentKey{instanceID, studentID}] = row
		late := scheduleModel.AttendanceSubstatusLate

		result, err := deps.service.PatchAttendance(context.Background(), 692, false, instanceID, studentID, scheduleModel.AttendanceFieldPatch{Substatus: &late})

		var validationErr *TimetableAttendanceValidationError
		require.ErrorAs(t, err, &validationErr)
		require.Len(t, validationErr.Fields, 1)
		assert.Equal(t, "substatus", validationErr.Fields[0].Field)
		assert.Nil(t, result)
		assert.Empty(t, deps.studentRepo.updates)
	})
}

func TestTimetableOperationsDependencyErrorsPropagate(t *testing.T) {
	instanceID := int64(417)
	activeGroupID := int64(302)

	t.Run("check-out propagates end visit errors", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		wireAssignedStaff(deps, 685, 507, 265, instanceID)
		deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, activeGroupID)
		visit := &activeModel.Visit{StudentID: 561, ActiveGroupID: activeGroupID, EntryTime: time.Now()}
		visit.ID = 418
		deps.visitRepo.byActiveGroup[activeGroupID] = []*activeModel.Visit{visit}
		deps.activeService.endErr = errors.New("end failed")

		result, err := deps.service.CheckOutStudent(context.Background(), 685, false, instanceID, 561)

		require.EqualError(t, err, "end failed")
		assert.Nil(t, result)
	})

	t.Run("patch propagates update errors", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		wireAssignedStaff(deps, 686, 508, 266, instanceID)
		deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, activeGroupID)
		row := &scheduleModel.InstanceStudent{InstanceID: instanceID, StudentID: 562, Status: scheduleModel.AttendanceStatusExpected}
		row.ID = 419
		deps.studentRepo.byInstanceStudent[instanceStudentKey{instanceID, 562}] = row
		deps.studentRepo.updateErr = errors.New("update failed")

		result, err := deps.service.PatchAttendance(context.Background(), 686, false, instanceID, 562, scheduleModel.AttendanceFieldPatch{})

		require.EqualError(t, err, "update failed")
		assert.Nil(t, result)
	})

	t.Run("patch propagates roster rebuild errors", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		wireAssignedStaff(deps, 689, 511, 269, instanceID)
		deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, activeGroupID)
		row := &scheduleModel.InstanceStudent{InstanceID: instanceID, StudentID: 563, Status: scheduleModel.AttendanceStatusExpected}
		row.ID = 421
		deps.studentRepo.byInstanceStudent[instanceStudentKey{instanceID, 563}] = row
		deps.studentRepo.err = errors.New("roster failed")

		result, err := deps.service.PatchAttendance(context.Background(), 689, false, instanceID, 563, scheduleModel.AttendanceFieldPatch{})

		require.EqualError(t, err, "roster failed")
		assert.Nil(t, result)
	})

	t.Run("patch returns not found when rebuilt roster omits student", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		wireAssignedStaff(deps, 690, 512, 270, instanceID)
		deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, activeGroupID)
		row := &scheduleModel.InstanceStudent{InstanceID: instanceID, StudentID: 564, Status: scheduleModel.AttendanceStatusExpected}
		row.ID = 422
		deps.studentRepo.byInstanceStudent[instanceStudentKey{instanceID, 564}] = row

		result, err := deps.service.PatchAttendance(context.Background(), 690, false, instanceID, 564, scheduleModel.AttendanceFieldPatch{})

		require.ErrorIs(t, err, ErrTimetableOperationNotFound)
		assert.Nil(t, result)
	})

	t.Run("load instance propagates ordinary repository errors", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		deps.instanceRepo.err = errors.New("db down")

		inst, err := deps.service.(*timetableOperationsService).loadInstance(context.Background(), instanceID)

		require.EqualError(t, err, "db down")
		assert.Nil(t, inst)
	})

	t.Run("complete returns permission errors before delegation", func(t *testing.T) {
		deps := newTimetableOpsDeps()

		result, err := deps.service.Complete(context.Background(), 0, false, instanceID)

		require.ErrorIs(t, err, ErrTimetableOperationForbidden)
		assert.Nil(t, result)
		assert.Empty(t, deps.instanceService.completed)
	})
}

func TestTimetableOperationsBroadcastBranches(t *testing.T) {
	ctx := tenant.WithTenantID(context.Background(), 722)

	t.Run("skips when broadcaster is nil", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		deps.service.(*timetableOperationsService).deps.Broadcaster = nil

		deps.service.(*timetableOperationsService).broadcastAttendanceChanged(ctx, 414)
	})

	t.Run("skips inactive instance without active group", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		deps.instanceRepo.byID[414] = instanceWithTimes(414, scheduleModel.InstanceStatusPlanned, time.Now(), time.Now().Add(time.Hour))

		deps.service.(*timetableOperationsService).broadcastAttendanceChanged(ctx, 414)

		assert.Empty(t, deps.broadcaster.Calls())
	})

	t.Run("logs and continues when broadcast fails", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		deps.broadcaster.Err = errors.New("send failed")
		deps.instanceRepo.byID[414] = activeInstance(414, 300)

		deps.service.(*timetableOperationsService).broadcastAttendanceChanged(ctx, 414)

		require.Len(t, deps.broadcaster.Calls(), 1)
	})

	t.Run("logs and skips when instance lookup fails", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		deps.instanceRepo.err = errors.New("instance failed")

		deps.service.(*timetableOperationsService).broadcastAttendanceChanged(ctx, 414)

		assert.Empty(t, deps.broadcaster.Calls())
	})
}

func TestTimetableOperationsDependencyAndErrorBranches(t *testing.T) {
	assert.Equal(t, "timetable attendance validation failed", (&TimetableAttendanceValidationError{}).Error())

	assert.Panics(t, func() {
		NewTimetableOperationsService(TimetableOperationsDependencies{})
	})

	deps := newTimetableOpsDeps()
	deps.settings.err = errors.New("settings down")
	assert.False(t, deps.service.(*timetableOperationsService).adminOverviewEnabled(context.Background(), true))
	assert.False(t, deps.service.(*timetableOperationsService).adminOverviewEnabled(context.Background(), false))

	deps.personService.accountPerson = &usersModel.Person{}
	deps.personService.accountPerson.ID = 505
	deps.personService.staffErr = sql.ErrNoRows
	staffID, hasStaff, err := deps.service.(*timetableOperationsService).resolveStaffID(context.Background(), 683)
	require.NoError(t, err)
	assert.Zero(t, staffID)
	assert.False(t, hasStaff)

	deps.instanceRepo.err = sql.ErrNoRows
	inst, err := deps.service.(*timetableOperationsService).loadInstance(context.Background(), 415)
	require.ErrorIs(t, err, ErrTimetableOperationNotFound)
	assert.Nil(t, inst)

	deps.instanceRepo.err = nil
	deps.instanceRepo.byID[416] = nil
	inst, err = deps.service.(*timetableOperationsService).loadInstance(context.Background(), 416)
	require.ErrorIs(t, err, ErrTimetableOperationNotFound)
	assert.Nil(t, inst)
}

func TestTimetableOperationHelpers(t *testing.T) {
	now := time.Date(2026, time.May, 10, 14, 0, 0, 0, time.UTC)
	assert.True(t, plannedNowWindow(instanceWithTimes(406, scheduleModel.InstanceStatusPlanned, now.Add(-16*time.Minute), now.Add(time.Hour)), now, 0))
	assert.True(t, plannedNowWindow(instanceWithTimes(407, scheduleModel.InstanceStatusPlanned, now.Add(14*time.Minute), now.Add(2*time.Hour)), now, 0))
	assert.False(t, plannedNowWindow(instanceWithTimes(408, scheduleModel.InstanceStatusPlanned, now.Add(16*time.Minute), now.Add(2*time.Hour)), now, 0))
	assert.True(t, plannedNowWindow(instanceWithTimes(409, scheduleModel.InstanceStatusPlanned, now.Add(90*time.Minute), now.Add(3*time.Hour)), now, 120))
	assert.False(t, plannedNowWindow(instanceWithTimes(410, scheduleModel.InstanceStatusPlanned, now.Add(-time.Hour), now), now, 0))
	assert.True(t, staffAssigned([]*scheduleModel.InstanceStaff{{StaffID: 255}}, 255))
	assert.False(t, staffAssigned([]*scheduleModel.InstanceStaff{{StaffID: 255, IsAbsent: true}}, 255))
	planned, ok := findPlanned([]*scheduleModel.InstanceStudent{{StudentID: 556}}, 556)
	require.True(t, ok)
	assert.Equal(t, int64(556), planned.StudentID)
	assert.False(t, modelBase.IsNoRows(errors.New("ordinary error")))
}

func TestTimetableOperationDirectHelperBranches(t *testing.T) {
	t.Run("load roster template group ignores missing and unbound groups", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		svc := deps.service.(*timetableOperationsService)

		group, err := svc.loadRosterTemplateGroup(context.Background(), nil)
		require.NoError(t, err)
		assert.Nil(t, group)

		zero := int64(0)
		group, err = svc.loadRosterTemplateGroup(context.Background(), &zero)
		require.NoError(t, err)
		assert.Nil(t, group)

		missing := int64(599)
		group, err = svc.loadRosterTemplateGroup(context.Background(), &missing)
		require.NoError(t, err)
		assert.Nil(t, group)
	})

	t.Run("load roster template group propagates ordinary errors", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		deps.activityGroups.err = errors.New("group failed")
		groupID := int64(600)

		group, err := deps.service.(*timetableOperationsService).loadRosterTemplateGroup(context.Background(), &groupID)

		require.EqualError(t, err, "group failed")
		assert.Nil(t, group)
	})

	t.Run("room name map skips nil rooms", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		deps.rooms.rooms = append(deps.rooms.rooms, nil)

		names, err := deps.service.(*timetableOperationsService).roomNameMap(context.Background())

		require.NoError(t, err)
		require.NotNil(t, names[810])
		assert.Equal(t, "Lernraum", *names[810])
	})

	t.Run("logger falls back to default", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		deps.service.(*timetableOperationsService).deps.Logger = nil

		assert.NotNil(t, deps.service.(*timetableOperationsService).logger())
	})
}

func wireAssignedStaff(deps *timetableOpsTestDeps, accountID, personID, staffID, instanceID int64) {
	deps.personService.accountPerson = &usersModel.Person{}
	deps.personService.accountPerson.ID = personID
	deps.personService.staffByPersonID[personID] = &usersModel.Staff{}
	deps.personService.staffByPersonID[personID].ID = staffID
	deps.staffRepo.byInstance[instanceID] = []*scheduleModel.InstanceStaff{{StaffID: staffID}}
}

func instanceWithTimes(id int64, status string, start, end time.Time) *scheduleModel.ActivityInstance {
	return instanceWithRoomAndTimes(id, 810, status, start, end)
}

func instanceWithRoomAndTimes(id, roomID int64, status string, start, end time.Time) *scheduleModel.ActivityInstance {
	inst := &scheduleModel.ActivityInstance{
		Date:      timezone.NewDate(start.Year(), start.Month(), start.Day()),
		Title:     "Lernzeit",
		StartTime: start,
		EndTime:   end,
		RoomID:    roomID,
		Status:    status,
	}
	inst.ID = id
	return inst
}

func activeInstance(id, activeGroupID int64) *scheduleModel.ActivityInstance {
	inst := instanceWithTimes(id, scheduleModel.InstanceStatusActive, time.Date(2026, time.May, 10, 14, 0, 0, 0, time.UTC), time.Date(2026, time.May, 10, 15, 0, 0, 0, time.UTC))
	inst.ID = id
	inst.ActiveGroupID = &activeGroupID
	return inst
}

type timetableOpsTestDeps struct {
	service         TimetableOperationsService
	instanceRepo    *fakeOpsInstanceRepo
	staffRepo       *fakeOpsStaffRepo
	studentRepo     *fakeOpsInstanceStudentRepo
	instanceService *fakeOpsInstanceService
	activeGroups    *fakeOpsActiveGroupRepo
	activityGroups  *fakeOpsActivityGroupRepo
	activeService   *fakeOpsActiveService
	arrivalService  *fakeOpsArrivalService
	supervisors     *fakeOpsSupervisorRepo
	visitRepo       *fakeOpsVisitRepo
	students        *fakeOpsStudentRepo
	groups          *fakeOpsEducationGroupRepo
	rooms           *fakeOpsRoomRepo
	personService   *fakeOpsPersonService
	settings        *fakeOpsSettings
	broadcaster     *testpkg.RecordingBroadcaster
	careDayService  *fakeOpsCareDayService
}

// fakeOpsCareDayService reports the care-plan verdict per student. Empty by
// default, which reads as "unknown" everywhere — the pre-#1747 behaviour.
type fakeOpsCareDayService struct {
	byStudent map[int64]CareDayStatus
}

func (f *fakeOpsCareDayService) ResolveForDate(_ context.Context, studentIDs []int64, _ timezone.Date) (map[int64]CareDayStatus, error) {
	out := make(map[int64]CareDayStatus, len(studentIDs))
	for _, id := range studentIDs {
		if status, ok := f.byStudent[id]; ok {
			out[id] = status
		}
	}
	return out, nil
}

func (f *fakeOpsCareDayService) ResolveForRange(ctx context.Context, studentIDs []int64, from, to timezone.Date) (map[int64]map[timezone.Date]CareDayStatus, error) {
	byDate, err := f.ResolveForDate(ctx, studentIDs, from)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]map[timezone.Date]CareDayStatus, len(byDate))
	for studentID, status := range byDate {
		out[studentID] = map[timezone.Date]CareDayStatus{}
		for date := from; !date.After(to); date = date.AddDays(1) {
			out[studentID][date] = status
		}
	}
	return out, nil
}

func newTimetableOpsDeps() *timetableOpsTestDeps {
	deps := &timetableOpsTestDeps{
		instanceRepo:    &fakeOpsInstanceRepo{byID: map[int64]*scheduleModel.ActivityInstance{}},
		staffRepo:       &fakeOpsStaffRepo{byInstance: map[int64][]*scheduleModel.InstanceStaff{}},
		studentRepo:     &fakeOpsInstanceStudentRepo{byInstance: map[int64][]*scheduleModel.InstanceStudent{}, byInstanceStudent: map[instanceStudentKey]*scheduleModel.InstanceStudent{}},
		instanceService: &fakeOpsInstanceService{},
		activeGroups:    &fakeOpsActiveGroupRepo{lastActivity: map[int64]time.Time{}},
		activityGroups:  &fakeOpsActivityGroupRepo{byID: map[int64]*activitiesModel.Group{}, targetsByGroup: map[int64][]*activitiesModel.GroupTarget{}},
		activeService:   &fakeOpsActiveService{},
		arrivalService:  &fakeOpsArrivalService{byStudent: map[int64]*EffectiveArrivalTime{}},
		careDayService:  &fakeOpsCareDayService{byStudent: map[int64]CareDayStatus{}},
		supervisors:     &fakeOpsSupervisorRepo{byActiveGroup: map[int64][]*activeModel.GroupSupervisor{}},
		visitRepo:       &fakeOpsVisitRepo{byActiveGroup: map[int64][]*activeModel.Visit{}, currentByStudent: map[int64]*activeModel.Visit{}},
		students:        &fakeOpsStudentRepo{byID: map[int64]*usersModel.Student{}},
		groups:          &fakeOpsEducationGroupRepo{byID: map[int64]*educationModel.Group{}},
		rooms:           &fakeOpsRoomRepo{rooms: []*facilitiesModel.Room{{Model: modelBase.Model{ID: 810}, Name: "Lernraum"}}},
		personService:   &fakeOpsPersonService{people: map[int64]*usersModel.Person{}, staffByPersonID: map[int64]*usersModel.Staff{}},
		settings:        &fakeOpsSettings{},
		broadcaster:     testpkg.NewRecordingBroadcaster(),
	}
	deps.service = NewTimetableOperationsService(TimetableOperationsDependencies{
		InstanceRepo:       deps.instanceRepo,
		InstanceStaffRepo:  deps.staffRepo,
		InstanceStudents:   deps.studentRepo,
		InstanceService:    deps.instanceService,
		ActiveGroupRepo:    deps.activeGroups,
		ActivityGroupRepo:  deps.activityGroups,
		ActiveService:      deps.activeService,
		CareDayService:     deps.careDayService,
		ArrivalService:     deps.arrivalService,
		SupervisorRepo:     deps.supervisors,
		VisitRepo:          deps.visitRepo,
		StudentRepo:        deps.students,
		EducationGroupRepo: deps.groups,
		RoomRepo:           deps.rooms,
		PersonService:      deps.personService,
		Settings:           deps.settings,
		Broadcaster:        deps.broadcaster,
		DB:                 &bun.DB{},
	})
	return deps
}

type fakeOpsInstanceRepo struct {
	scheduleModel.ActivityInstanceRepository
	byID          map[int64]*scheduleModel.ActivityInstance
	byDate        []*scheduleModel.ActivityInstance
	err           error
	findByDateErr error
}

func (r *fakeOpsInstanceRepo) FindByID(_ context.Context, id interface{}) (*scheduleModel.ActivityInstance, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.byID[id.(int64)], nil
}

func (r *fakeOpsInstanceRepo) FindByTenantAndDate(_ context.Context, _ timezone.Date) ([]*scheduleModel.ActivityInstance, error) {
	if r.findByDateErr != nil {
		return nil, r.findByDateErr
	}
	return r.byDate, nil
}

func (r *fakeOpsInstanceRepo) FindByActiveGroupID(_ context.Context, activeGroupID int64) (*scheduleModel.ActivityInstance, error) {
	for _, inst := range r.byID {
		if inst.ActiveGroupID != nil && *inst.ActiveGroupID == activeGroupID {
			return inst, nil
		}
	}
	return nil, nil
}

type fakeOpsStaffRepo struct {
	scheduleModel.InstanceStaffRepository
	byInstance map[int64][]*scheduleModel.InstanceStaff
	err        error
}

func (r *fakeOpsStaffRepo) FindByInstanceID(_ context.Context, instanceID int64) ([]*scheduleModel.InstanceStaff, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.byInstance[instanceID], nil
}

type instanceStudentKey struct {
	instanceID int64
	studentID  int64
}

type fakeOpsInstanceStudentRepo struct {
	scheduleModel.InstanceStudentRepository
	byInstance        map[int64][]*scheduleModel.InstanceStudent
	byInstanceStudent map[instanceStudentKey]*scheduleModel.InstanceStudent
	err               error
	updateErr         error
	updates           []struct {
		rowID int64
		patch scheduleModel.AttendanceFieldPatch
	}
}

func (r *fakeOpsInstanceStudentRepo) FindByInstanceID(_ context.Context, instanceID int64) ([]*scheduleModel.InstanceStudent, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.byInstance[instanceID], nil
}

func (r *fakeOpsInstanceStudentRepo) FindByInstanceAndStudent(_ context.Context, instanceID, studentID int64) (*scheduleModel.InstanceStudent, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.byInstanceStudent[instanceStudentKey{instanceID, studentID}], nil
}

func (r *fakeOpsInstanceStudentRepo) UpdateAttendanceFields(_ context.Context, id int64, patch scheduleModel.AttendanceFieldPatch) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	r.updates = append(r.updates, struct {
		rowID int64
		patch scheduleModel.AttendanceFieldPatch
	}{rowID: id, patch: patch})
	return nil
}

type fakeOpsInstanceService struct {
	InstanceService
	started   []struct{ instanceID, staffID int64 }
	completed []int64
}

func (s *fakeOpsInstanceService) Start(_ context.Context, instanceID, startedByStaffID int64) (*StartInstanceResult, error) {
	s.started = append(s.started, struct{ instanceID, staffID int64 }{instanceID: instanceID, staffID: startedByStaffID})
	inst := &scheduleModel.ActivityInstance{Status: scheduleModel.InstanceStatusActive}
	inst.ID = instanceID
	return &StartInstanceResult{Instance: inst, ActiveGroupID: 910}, nil
}

func (s *fakeOpsInstanceService) Complete(_ context.Context, instanceID int64) (*scheduleModel.ActivityInstance, error) {
	s.completed = append(s.completed, instanceID)
	inst := &scheduleModel.ActivityInstance{Status: scheduleModel.InstanceStatusCompleted}
	inst.ID = instanceID
	return inst, nil
}

func (s *fakeOpsInstanceService) Reopen(_ context.Context, instanceID, _ int64, _ bool) (*StartInstanceResult, error) {
	inst := &scheduleModel.ActivityInstance{Status: scheduleModel.InstanceStatusActive}
	inst.ID = instanceID
	return &StartInstanceResult{Instance: inst, ActiveGroupID: 911}, nil
}

type fakeOpsActiveGroupRepo struct {
	activeModel.GroupRepository
	lastActivity map[int64]time.Time
	updateErr    error
}

func (r *fakeOpsActiveGroupRepo) UpdateLastActivity(_ context.Context, id int64, lastActivity time.Time) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	r.lastActivity[id] = lastActivity
	return nil
}

type fakeOpsActivityGroupRepo struct {
	activitiesModel.GroupRepository
	byID           map[int64]*activitiesModel.Group
	targetsByGroup map[int64][]*activitiesModel.GroupTarget
	err            error
}

func (r *fakeOpsActivityGroupRepo) FindTargetsByGroupIDs(_ context.Context, groupIDs []int64) (map[int64][]*activitiesModel.GroupTarget, error) {
	result := make(map[int64][]*activitiesModel.GroupTarget, len(groupIDs))
	for _, groupID := range groupIDs {
		result[groupID] = r.targetsByGroup[groupID]
	}
	return result, nil
}

func (r *fakeOpsActivityGroupRepo) FindByID(_ context.Context, id interface{}) (*activitiesModel.Group, error) {
	if r.err != nil {
		return nil, r.err
	}
	group := r.byID[id.(int64)]
	if group == nil {
		return nil, sql.ErrNoRows
	}
	return group, nil
}

type fakeOpsActiveService struct {
	created   []*activeModel.Visit
	ended     []int64
	createErr error
	endErr    error
}

func (s *fakeOpsActiveService) CreateVisit(_ context.Context, visit *activeModel.Visit) error {
	if s.createErr != nil {
		return s.createErr
	}
	s.created = append(s.created, visit)
	return nil
}

func (s *fakeOpsActiveService) EndVisit(_ context.Context, id int64) error {
	if s.endErr != nil {
		return s.endErr
	}
	s.ended = append(s.ended, id)
	return nil
}

type fakeOpsArrivalService struct {
	byStudent map[int64]*EffectiveArrivalTime
	err       error
}

func (s *fakeOpsArrivalService) GetBulkEffectiveArrivalTimesForDate(_ context.Context, studentIDs []int64, date timezone.Date) (map[int64]*EffectiveArrivalTime, error) {
	if s.err != nil {
		return nil, s.err
	}
	out := make(map[int64]*EffectiveArrivalTime, len(studentIDs))
	for _, studentID := range studentIDs {
		if arrival := s.byStudent[studentID]; arrival != nil {
			out[studentID] = arrival
			continue
		}
		out[studentID] = &EffectiveArrivalTime{Date: date}
	}
	return out, nil
}

type fakeOpsSupervisorRepo struct {
	activeModel.GroupSupervisorRepository
	byActiveGroup map[int64][]*activeModel.GroupSupervisor
}

func (r *fakeOpsSupervisorRepo) FindByActiveGroupID(_ context.Context, activeGroupID int64, _ bool) ([]*activeModel.GroupSupervisor, error) {
	return r.byActiveGroup[activeGroupID], nil
}

type fakeOpsVisitRepo struct {
	activeModel.VisitRepository
	byActiveGroup    map[int64][]*activeModel.Visit
	currentByStudent map[int64]*activeModel.Visit
	err              error
}

func (r *fakeOpsVisitRepo) FindByActiveGroupID(_ context.Context, activeGroupID int64) ([]*activeModel.Visit, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.byActiveGroup[activeGroupID], nil
}

func (r *fakeOpsVisitRepo) GetCurrentByStudentID(_ context.Context, studentID int64) (*activeModel.Visit, error) {
	return r.currentByStudent[studentID], nil
}

type fakeOpsStudentRepo struct {
	usersModel.StudentRepository
	byID map[int64]*usersModel.Student
}

func (r *fakeOpsStudentRepo) FindByIDs(_ context.Context, ids []int64) (map[int64]*usersModel.Student, error) {
	out := map[int64]*usersModel.Student{}
	for _, id := range ids {
		if st := r.byID[id]; st != nil {
			out[id] = st
		}
	}
	return out, nil
}

type fakeOpsEducationGroupRepo struct {
	educationModel.GroupRepository
	byID map[int64]*educationModel.Group
}

func (r *fakeOpsEducationGroupRepo) FindByIDs(_ context.Context, ids []int64) (map[int64]*educationModel.Group, error) {
	out := map[int64]*educationModel.Group{}
	for _, id := range ids {
		if group := r.byID[id]; group != nil {
			out[id] = group
		}
	}
	return out, nil
}

type fakeOpsRoomRepo struct {
	facilitiesModel.RoomRepository
	rooms []*facilitiesModel.Room
	err   error
}

func (r *fakeOpsRoomRepo) List(_ context.Context, _ map[string]interface{}) ([]*facilitiesModel.Room, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.rooms, nil
}

type fakeOpsPersonService struct {
	accountPerson   *usersModel.Person
	accountErr      error
	people          map[int64]*usersModel.Person
	staffByPersonID map[int64]*usersModel.Staff
	staffErr        error
}

func (s *fakeOpsPersonService) FindByAccountID(_ context.Context, _ int64) (*usersModel.Person, error) {
	if s.accountErr != nil {
		return nil, s.accountErr
	}
	return s.accountPerson, nil
}

func (s *fakeOpsPersonService) GetByIDs(_ context.Context, ids []int64) (map[int64]*usersModel.Person, error) {
	out := map[int64]*usersModel.Person{}
	for _, id := range ids {
		if person := s.people[id]; person != nil {
			out[id] = person
		}
	}
	return out, nil
}

func (s *fakeOpsPersonService) GetStaffByPersonID(_ context.Context, personID int64) (*usersModel.Staff, error) {
	if s.staffErr != nil {
		return nil, s.staffErr
	}
	return s.staffByPersonID[personID], nil
}

type fakeOpsSettings struct {
	enabled   bool
	err       error
	mode      string
	stringErr error
}

func (s *fakeOpsSettings) ResolveBool(_ context.Context, _ string) (bool, error) {
	return s.enabled, s.err
}

func (s *fakeOpsSettings) ResolveString(_ context.Context, _ string) (string, error) {
	if s.stringErr != nil {
		return "", s.stringErr
	}
	return s.mode, nil
}

func (s *fakeOpsSettings) ResolveInt(_ context.Context, _ string) (int, error) {
	return 15, s.err
}
