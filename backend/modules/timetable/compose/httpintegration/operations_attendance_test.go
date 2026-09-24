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
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTimetableOperationsCheckInCreatesVisitAndMarksPlannedPresent(t *testing.T) {
	t.Parallel()

	instanceID := int64(380)
	activeGroupID := int64(280)
	studentID := int64(540)
	rowID := int64(390)
	deps := newTimetableOpsDeps()
	wireAssignedStaff(deps, 660, 470, 250, instanceID)
	deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, activeGroupID)
	deps.studentRepo.byInstance[instanceID] = []*scheduleModels.InstanceStudent{
		{StudentID: studentID, Status: scheduleModels.AttendanceStatusExpected},
	}
	deps.studentRepo.byInstanceStudent[instanceStudentKey{instanceID, studentID}] = &scheduleModels.InstanceStudent{
		InstanceID: instanceID,
		StudentID:  studentID,
		Status:     scheduleModels.AttendanceStatusExpected,
	}
	deps.studentRepo.byInstanceStudent[instanceStudentKey{instanceID, studentID}].ID = rowID
	deps.students.byID[studentID] = &usersModels.Student{PersonID: 480, SchoolClass: "2c"}
	deps.personService.people[480] = &usersModels.Person{FirstName: "Mila", LastName: "Muster"}

	roster, err := deps.service.CheckInStudent(tenant.WithTenantID(context.Background(), 720), 660, false, instanceID, studentID)

	require.NoError(t, err)
	require.Len(t, deps.activeService.created, 1)
	assert.Equal(t, int64(720), deps.activeService.created[0].TenantID)
	assert.Equal(t, activeGroupID, deps.activeService.created[0].ActiveGroupID)
	require.Len(t, deps.studentRepo.updates, 1)
	assert.Equal(t, rowID, deps.studentRepo.updates[0].rowID)
	assert.Equal(t, scheduleModels.AttendanceStatusPresent, *deps.studentRepo.updates[0].patch.Status)
	assert.True(t, deps.studentRepo.updates[0].patch.SubstatusClear)
	assert.True(t, deps.activeGroups.lastActivity[activeGroupID].After(time.Time{}))
	assert.Equal(t, studentID, roster.Rows[0].StudentID)
}

func TestTimetableOperationsCheckInStopsWhenPresenceReadFails(t *testing.T) {
	t.Parallel()

	const instanceID, studentID = int64(381), int64(541)
	deps := newTimetableOpsDeps()
	wireAssignedStaff(deps, 661, 471, 251, instanceID)
	deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, 281)
	readErr := errors.New("presence unavailable")
	deps.visitRepo.err = readErr

	roster, err := deps.service.CheckInStudent(context.Background(), 661, false, instanceID, studentID)

	require.ErrorIs(t, err, readErr)
	assert.Nil(t, roster)
	assert.Empty(t, deps.activeService.created)
	assert.Empty(t, deps.activeService.moveCalls)
	assert.Empty(t, deps.studentRepo.updates)
	assert.Empty(t, deps.activeGroups.lastActivity)
}

func TestTimetableOperationsCheckInMovesVisitCreatedDuringCheckIn(t *testing.T) {
	t.Parallel()

	instanceID := int64(381)
	targetActiveGroupID := int64(281)
	originActiveGroupID := int64(282)
	studentID := int64(541)
	rowID := int64(391)
	deps := newTimetableOpsDeps()
	wireAssignedStaff(deps, 661, 471, 251, instanceID)
	deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, targetActiveGroupID)
	row := &scheduleModels.InstanceStudent{InstanceID: instanceID, StudentID: studentID, Status: scheduleModels.AttendanceStatusExpected}
	row.ID = rowID
	deps.studentRepo.byInstance[instanceID] = []*scheduleModels.InstanceStudent{row}
	deps.studentRepo.byInstanceStudent[instanceStudentKey{instanceID, studentID}] = row
	deps.students.byID[studentID] = &usersModels.Student{PersonID: 481, SchoolClass: "2c"}
	deps.personService.people[481] = &usersModels.Person{FirstName: "Mila", LastName: "Muster"}
	deps.visitRepo.currentByStudentSequence[studentID] = []*studentpresence.Visit{
		nil,
		{StudentID: studentID, ActiveGroupID: originActiveGroupID, EntryTime: opsNow},
	}
	deps.activeService.createErr = studentpresence.ErrStudentAlreadyActive

	roster, err := deps.service.CheckInStudent(context.Background(), 661, false, instanceID, studentID)

	require.NoError(t, err)
	require.NotNil(t, roster)
	require.Len(t, deps.activeService.moveCalls, 1)
	assert.Equal(t, []int64{studentID}, deps.activeService.moveCalls[0].studentIDs)
	assert.Equal(t, targetActiveGroupID, deps.activeService.moveCalls[0].activeGroupID)
	require.Len(t, deps.studentRepo.updates, 1)
	assert.Equal(t, rowID, deps.studentRepo.updates[0].rowID)
	assert.Equal(t, scheduleModels.AttendanceStatusPresent, *deps.studentRepo.updates[0].patch.Status)
}

// The cross-group 409 was retired with #2386: a child still recorded present
// in another running block now moves automatically instead of being rejected.
func TestTimetableOperationsCheckInMovesStudentActiveElsewhere(t *testing.T) {
	t.Parallel()

	instanceID := int64(400)
	activeGroupID := int64(290)
	originInstanceID := int64(405)
	originActiveGroupID := int64(291)
	studentID := int64(550)
	rowID := int64(395)

	newDeps := func() *timetableOpsTestDeps {
		deps := newTimetableOpsDeps()
		wireAssignedStaff(deps, 670, 490, 251, instanceID)
		deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, activeGroupID)
		deps.visitRepo.currentByStudent[studentID] = &studentpresence.Visit{StudentID: studentID, ActiveGroupID: originActiveGroupID, EntryTime: opsNow}
		deps.activeService.moveResult = &studentpresence.StudentMoveResult{
			Moved:                  []int64{studentID},
			PreviousActiveGroupIDs: map[int64]int64{studentID: originActiveGroupID},
			ActiveGroupID:          &activeGroupID,
		}
		return deps
	}

	t.Run("moves via active service and names the origin instance", func(t *testing.T) {
		deps := newDeps()
		origin := activeInstance(originInstanceID, originActiveGroupID)
		origin.Title = "GT 1"
		deps.instanceRepo.byID[originInstanceID] = origin
		row := &scheduleModels.InstanceStudent{InstanceID: instanceID, StudentID: studentID, Status: scheduleModels.AttendanceStatusExpected}
		row.ID = rowID
		deps.studentRepo.byInstanceStudent[instanceStudentKey{instanceID, studentID}] = row
		deps.studentRepo.byInstance[instanceID] = []*scheduleModels.InstanceStudent{row}
		deps.students.byID[studentID] = &usersModels.Student{PersonID: 491, SchoolClass: "2b"}
		deps.personService.people[491] = &usersModels.Person{FirstName: "Marie", LastName: "Muster"}

		roster, err := deps.service.CheckInStudent(context.Background(), 670, false, instanceID, studentID)

		require.NoError(t, err)
		assert.Empty(t, deps.activeService.created)
		require.Len(t, deps.activeService.moveCalls, 1)
		assert.Equal(t, []int64{studentID}, deps.activeService.moveCalls[0].studentIDs)
		assert.Equal(t, activeGroupID, deps.activeService.moveCalls[0].activeGroupID)
		assert.Equal(t, int64(251), deps.activeService.moveCalls[0].auth.StaffID)
		assert.True(t, deps.activeService.moveCalls[0].auth.BypassResourceChecks)
		require.NotNil(t, roster.MovedFrom)
		assert.Equal(t, "GT 1", *roster.MovedFrom)
		require.Len(t, deps.studentRepo.updates, 1)
		assert.Equal(t, rowID, deps.studentRepo.updates[0].rowID)
		assert.Equal(t, scheduleModels.AttendanceStatusPresent, *deps.studentRepo.updates[0].patch.Status)
	})

	t.Run("names the origin observed by the serialized move", func(t *testing.T) {
		deps := newDeps()
		const concurrentOriginActiveGroupID int64 = 292
		origin := activeInstance(originInstanceID, concurrentOriginActiveGroupID)
		origin.Title = "GT 2"
		deps.instanceRepo.byID[originInstanceID] = origin
		deps.activeService.moveResult.PreviousActiveGroupIDs[studentID] = concurrentOriginActiveGroupID

		roster, err := deps.service.CheckInStudent(context.Background(), 670, false, instanceID, studentID)

		require.NoError(t, err)
		require.NotNil(t, roster.MovedFrom)
		assert.Equal(t, "GT 2", *roster.MovedFrom)
	})

	t.Run("falls back to the activity group name when no instance owns the origin session", func(t *testing.T) {
		deps := newDeps()
		originGroup := &studentpresence.LiveGroup{ActivityGroupID: testpkg.Int64Ptr(640), RoomID: 810}
		originGroup.ID = originActiveGroupID
		deps.activeGroups.byID = map[int64]*studentpresence.LiveGroup{originActiveGroupID: originGroup}
		activityGroup := &activitiesModels.Group{Name: "Fußball AG"}
		activityGroup.ID = 640
		deps.activityGroups.byID[640] = activityGroup

		roster, err := deps.service.CheckInStudent(context.Background(), 670, false, instanceID, studentID)

		require.NoError(t, err)
		require.NotNil(t, roster.MovedFrom)
		assert.Equal(t, "Fußball AG", *roster.MovedFrom)
	})

	t.Run("still reports a move when no origin name is resolvable", func(t *testing.T) {
		deps := newDeps()

		roster, err := deps.service.CheckInStudent(context.Background(), 670, false, instanceID, studentID)

		require.NoError(t, err)
		require.NotNil(t, roster.MovedFrom)
		assert.Equal(t, "", *roster.MovedFrom)
	})

	t.Run("propagates move errors", func(t *testing.T) {
		deps := newDeps()
		deps.activeService.moveErr = errors.New("move failed")

		_, err := deps.service.CheckInStudent(context.Background(), 670, false, instanceID, studentID)

		require.EqualError(t, err, "move failed")
	})

	t.Run("maps a skipped move to a conflict", func(t *testing.T) {
		deps := newDeps()
		deps.activeService.moveResult = &studentpresence.StudentMoveResult{
			Skipped: []studentpresence.StudentMoveSkipped{{StudentID: studentID, Reason: studentpresence.StudentMoveSkipConflict}},
		}
		ctx := tenant.WithRollbackMarker(context.Background())

		_, err := deps.service.CheckInStudent(ctx, 670, false, instanceID, studentID)

		require.ErrorIs(t, err, timetable.ErrTimetableOperationConflict)
		assert.True(t, tenant.RollbackRequested(ctx))
		assert.Empty(t, deps.studentRepo.updates)
	})

	t.Run("treats an unchanged result as same-group success without move notice", func(t *testing.T) {
		deps := newDeps()
		deps.activeService.moveResult = &studentpresence.StudentMoveResult{Unchanged: []int64{studentID}}

		roster, err := deps.service.CheckInStudent(context.Background(), 670, false, instanceID, studentID)

		require.NoError(t, err)
		assert.Nil(t, roster.MovedFrom)
	})
}

func TestTimetableOperationsCheckOutEndsMatchingVisit(t *testing.T) {
	t.Parallel()

	instanceID := int64(401)
	activeGroupID := int64(292)
	studentID := int64(551)
	visitID := int64(402)
	deps := newTimetableOpsDeps()
	wireAssignedStaff(deps, 671, 491, 252, instanceID)
	deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, activeGroupID)
	deps.visitRepo.byActiveGroup[activeGroupID] = []*studentpresence.Visit{{StudentID: studentID, ActiveGroupID: activeGroupID, EntryTime: opsNow}}
	deps.visitRepo.byActiveGroup[activeGroupID][0].ID = visitID
	deps.students.byID[studentID] = &usersModels.Student{PersonID: 492, SchoolClass: "1a"}
	deps.personService.people[492] = &usersModels.Person{FirstName: "Ben", LastName: "Beispiel"}

	_, err := deps.service.CheckOutStudent(context.Background(), 671, false, instanceID, studentID)

	require.NoError(t, err)
	assert.Equal(t, []int64{visitID}, deps.activeService.ended)
}

func TestTimetableOperationsCheckOutAlreadyEndedReturnsRoster(t *testing.T) {
	t.Parallel()

	instanceID := int64(413)
	activeGroupID := int64(299)
	studentID := int64(559)
	visitID := int64(414)
	deps := newTimetableOpsDeps()
	wireAssignedStaff(deps, 682, 504, 263, instanceID)
	deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, activeGroupID)
	deps.visitRepo.byActiveGroup[activeGroupID] = []*studentpresence.Visit{{StudentID: studentID, ActiveGroupID: activeGroupID, EntryTime: opsNow}}
	deps.visitRepo.byActiveGroup[activeGroupID][0].ID = visitID
	deps.activeService.endErr = studentpresence.ErrVisitAlreadyEnded

	roster, err := deps.service.CheckOutStudent(context.Background(), 682, false, instanceID, studentID)

	require.NoError(t, err)
	require.NotNil(t, roster)
	assert.Equal(t, int64(413), roster.Instance.ID)
}

func TestTimetableOperationsPatchAttendanceUpdatesRowAndBroadcasts(t *testing.T) {
	t.Parallel()

	instanceID := int64(403)
	activeGroupID := int64(293)
	studentID := int64(552)
	rowID := int64(404)
	deps := newTimetableOpsDeps()
	wireAssignedStaff(deps, 672, 493, 253, instanceID)
	deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, activeGroupID)
	deps.studentRepo.byInstanceStudent[instanceStudentKey{instanceID, studentID}] = &scheduleModels.InstanceStudent{
		InstanceID: instanceID,
		StudentID:  studentID,
		Status:     scheduleModels.AttendanceStatusExpected,
	}
	deps.studentRepo.byInstanceStudent[instanceStudentKey{instanceID, studentID}].ID = rowID
	deps.studentRepo.byInstance[instanceID] = []*scheduleModels.InstanceStudent{
		deps.studentRepo.byInstanceStudent[instanceStudentKey{instanceID, studentID}],
	}
	deps.students.byID[studentID] = &usersModels.Student{PersonID: 494, SchoolClass: "1b"}
	deps.personService.people[494] = &usersModels.Person{FirstName: "Lea", LastName: "Lern"}
	status := scheduleModels.AttendanceStatusAbsent
	note := "krank gemeldet"

	row, err := deps.service.PatchAttendance(tenant.WithTenantID(context.Background(), 721), 672, false, instanceID, studentID, timetable.AttendancePatch{
		Status: &status,
		Note:   &note,
	})

	require.NoError(t, err)
	require.NotNil(t, row)
	assert.Equal(t, studentID, row.StudentID)
	require.Len(t, deps.studentRepo.updates, 1)
	assert.Equal(t, rowID, deps.studentRepo.updates[0].rowID)
	assert.Equal(t, &note, deps.studentRepo.updates[0].patch.Note)
	// #2085: the owner hands over the tenant, the session and the block —
	// never the child whose attendance was patched. The tenant-wide event
	// shape is the composition root's announcer
	// (services.TestTimetableAttendanceAnnouncerNamesTheBlockNotTheChild).
	assert.Equal(t, []announcement{{tenantID: 721, activeGroupID: activeGroupID, instanceID: instanceID}}, deps.announcer.calls)
}

func TestTimetableOperationsPatchAttendanceRejectsCompletedInstance(t *testing.T) {
	t.Parallel()

	instanceID := int64(427)
	studentID := int64(567)
	deps := newTimetableOpsDeps()
	wireAssignedStaff(deps, 693, 515, 274, instanceID)
	completed := activeInstance(instanceID, 305)
	completed.Status = scheduleModels.InstanceStatusCompleted
	deps.instanceRepo.byID[instanceID] = completed
	deps.studentRepo.byInstanceStudent[instanceStudentKey{instanceID, studentID}] = &scheduleModels.InstanceStudent{
		InstanceID: instanceID,
		StudentID:  studentID,
		Status:     scheduleModels.AttendanceStatusPresent,
	}
	status := scheduleModels.AttendanceStatusAbsent

	row, err := deps.service.PatchAttendance(context.Background(), 693, false, instanceID, studentID, timetable.AttendancePatch{Status: &status})

	require.ErrorIs(t, err, timetable.ErrTimetableOperationConflict)
	assert.Nil(t, row)
	assert.Empty(t, deps.studentRepo.updates)
}

func TestTimetableOperationsCheckInBranches(t *testing.T) {
	t.Parallel()

	instanceID := int64(410)
	activeGroupID := int64(297)
	studentID := int64(557)

	t.Run("rejects planned instance", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		wireAssignedStaff(deps, 678, 498, 259, instanceID)
		deps.instanceRepo.byID[instanceID] = instanceWithTimes(instanceID, scheduleModels.InstanceStatusPlanned, opsNow, opsNow.Add(time.Hour))

		_, err := deps.service.CheckInStudent(context.Background(), 678, false, instanceID, studentID)

		require.ErrorIs(t, err, timetable.ErrTimetableOperationConflict)
	})

	t.Run("marks planned present when active visit already belongs to same group", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		wireAssignedStaff(deps, 679, 499, 260, instanceID)
		deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, activeGroupID)
		row := &scheduleModels.InstanceStudent{InstanceID: instanceID, StudentID: studentID, Status: scheduleModels.AttendanceStatusExpected}
		row.ID = 411
		deps.studentRepo.byInstanceStudent[instanceStudentKey{instanceID, studentID}] = row
		deps.studentRepo.byInstance[instanceID] = []*scheduleModels.InstanceStudent{row}
		deps.visitRepo.currentByStudent[studentID] = &studentpresence.Visit{StudentID: studentID, ActiveGroupID: activeGroupID, EntryTime: opsNow}
		deps.visitRepo.byActiveGroup[activeGroupID] = []*studentpresence.Visit{deps.visitRepo.currentByStudent[studentID]}
		deps.students.byID[studentID] = &usersModels.Student{PersonID: 500, SchoolClass: "4a"}
		deps.personService.people[500] = &usersModels.Person{FirstName: "Tom", LastName: "Test"}

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
		deps.students.byID[studentID] = &usersModels.Student{PersonID: 502, SchoolClass: "4a"}
		deps.personService.people[502] = &usersModels.Person{FirstName: "Noa", LastName: "Neben"}

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
		row := &scheduleModels.InstanceStudent{InstanceID: instanceID, StudentID: studentID, Status: scheduleModels.AttendanceStatusExpected}
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
	t.Parallel()

	instanceID := int64(412)
	deps := newTimetableOpsDeps()
	wireAssignedStaff(deps, 681, 503, 262, instanceID)
	deps.instanceRepo.byID[instanceID] = instanceWithTimes(instanceID, scheduleModels.InstanceStatusActive, opsNow, opsNow.Add(time.Hour))

	_, err := deps.service.CheckOutStudent(context.Background(), 681, false, instanceID, 558)

	require.ErrorIs(t, err, timetable.ErrTimetableOperationConflict)

	deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, 298)
	_, err = deps.service.CheckOutStudent(context.Background(), 681, false, instanceID, 558)
	require.ErrorIs(t, err, timetable.ErrTimetableOperationNotFound)
}

func TestTimetableOperationsActiveVisitLookupPropagatesErrors(t *testing.T) {
	t.Parallel()

	// The check-out's lookup of the child's open visit in the session.
	const instanceID = int64(418)
	deps := newTimetableOpsDeps()
	wireAssignedStaff(deps, 683, 505, 264, instanceID)
	deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, 305)
	deps.visitRepo.err = errors.New("visit query failed")

	roster, err := deps.service.CheckOutStudent(context.Background(), 683, false, instanceID, 567)

	require.EqualError(t, err, "visit query failed")
	assert.Nil(t, roster)
	assert.Empty(t, deps.activeService.ended)
}

func TestTimetableOperationsPatchAttendanceBranches(t *testing.T) {
	t.Parallel()

	deps := newTimetableOpsDeps()
	instanceID := int64(413)
	wireAssignedStaff(deps, 682, 504, 263, instanceID)
	deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, 299)

	row, err := deps.service.PatchAttendance(context.Background(), 682, false, instanceID, 559, timetable.AttendancePatch{})

	require.ErrorIs(t, err, timetable.ErrTimetableOperationNotFound)
	assert.Nil(t, row)

	t.Run("forbidden before attendance row validation", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		instanceID := int64(423)
		studentID := int64(565)
		wireAssignedStaff(deps, 691, 513, 271, instanceID)
		deps.staffRepo.byInstance[instanceID] = []*scheduleModels.InstanceStaff{{StaffID: 272}}
		deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, 303)
		excused := scheduleModels.AttendanceSubstatusExcused
		row := &scheduleModels.InstanceStudent{InstanceID: instanceID, StudentID: studentID, Status: scheduleModels.AttendanceStatusPresent, Substatus: &excused}
		row.ID = 424
		deps.studentRepo.byInstanceStudent[instanceStudentKey{instanceID, studentID}] = row
		expected := scheduleModels.AttendanceStatusExpected

		result, err := deps.service.PatchAttendance(context.Background(), 691, false, instanceID, studentID, timetable.AttendancePatch{Status: &expected})

		require.ErrorIs(t, err, timetable.ErrTimetableOperationForbidden)
		assert.Nil(t, result)
		assert.Empty(t, deps.studentRepo.updates)
	})

	t.Run("authorized invalid patch returns field validation error", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		instanceID := int64(425)
		studentID := int64(566)
		wireAssignedStaff(deps, 692, 514, 273, instanceID)
		deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, 304)
		row := &scheduleModels.InstanceStudent{InstanceID: instanceID, StudentID: studentID, Status: scheduleModels.AttendanceStatusExpected}
		row.ID = 426
		deps.studentRepo.byInstanceStudent[instanceStudentKey{instanceID, studentID}] = row
		late := scheduleModels.AttendanceSubstatusLate

		result, err := deps.service.PatchAttendance(context.Background(), 692, false, instanceID, studentID, timetable.AttendancePatch{Substatus: &late})

		var validationErr *timetable.AttendanceValidationError
		require.ErrorAs(t, err, &validationErr)
		require.Len(t, validationErr.Fields, 1)
		assert.Equal(t, "substatus", validationErr.Fields[0].Field)
		assert.Nil(t, result)
		assert.Empty(t, deps.studentRepo.updates)
	})
}

func TestTimetableOperationsDependencyErrorsPropagate(t *testing.T) {
	t.Parallel()

	instanceID := int64(417)
	activeGroupID := int64(302)

	t.Run("check-out propagates end visit errors", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		wireAssignedStaff(deps, 685, 507, 265, instanceID)
		deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, activeGroupID)
		visit := &studentpresence.Visit{StudentID: 561, ActiveGroupID: activeGroupID, EntryTime: opsNow}
		visit.ID = 418
		deps.visitRepo.byActiveGroup[activeGroupID] = []*studentpresence.Visit{visit}
		deps.activeService.endErr = errors.New("end failed")

		result, err := deps.service.CheckOutStudent(context.Background(), 685, false, instanceID, 561)

		require.EqualError(t, err, "end failed")
		assert.Nil(t, result)
	})

	t.Run("patch propagates update errors", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		wireAssignedStaff(deps, 686, 508, 266, instanceID)
		deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, activeGroupID)
		row := &scheduleModels.InstanceStudent{InstanceID: instanceID, StudentID: 562, Status: scheduleModels.AttendanceStatusExpected}
		row.ID = 419
		deps.studentRepo.byInstanceStudent[instanceStudentKey{instanceID, 562}] = row
		deps.studentRepo.updateErr = errors.New("update failed")

		result, err := deps.service.PatchAttendance(context.Background(), 686, false, instanceID, 562, timetable.AttendancePatch{})

		require.EqualError(t, err, "update failed")
		assert.Nil(t, result)
	})

	t.Run("patch propagates roster rebuild errors", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		wireAssignedStaff(deps, 689, 511, 269, instanceID)
		deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, activeGroupID)
		row := &scheduleModels.InstanceStudent{InstanceID: instanceID, StudentID: 563, Status: scheduleModels.AttendanceStatusExpected}
		row.ID = 421
		deps.studentRepo.byInstanceStudent[instanceStudentKey{instanceID, 563}] = row
		deps.studentRepo.err = errors.New("roster failed")

		result, err := deps.service.PatchAttendance(context.Background(), 689, false, instanceID, 563, timetable.AttendancePatch{})

		require.EqualError(t, err, "roster failed")
		assert.Nil(t, result)
	})

	t.Run("patch returns not found when rebuilt roster omits student", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		wireAssignedStaff(deps, 690, 512, 270, instanceID)
		deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, activeGroupID)
		row := &scheduleModels.InstanceStudent{InstanceID: instanceID, StudentID: 564, Status: scheduleModels.AttendanceStatusExpected}
		row.ID = 422
		deps.studentRepo.byInstanceStudent[instanceStudentKey{instanceID, 564}] = row

		result, err := deps.service.PatchAttendance(context.Background(), 690, false, instanceID, 564, timetable.AttendancePatch{})

		require.ErrorIs(t, err, timetable.ErrTimetableOperationNotFound)
		assert.Nil(t, result)
	})

	t.Run("complete returns permission errors before delegation", func(t *testing.T) {
		deps := newTimetableOpsDeps()

		result, err := deps.service.Complete(context.Background(), 0, false, instanceID)

		require.ErrorIs(t, err, timetable.ErrTimetableOperationForbidden)
		assert.Nil(t, result)
		assert.Empty(t, deps.instanceService.completed)
	})
}
