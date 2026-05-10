package schedule

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	activeModel "github.com/moto-nrw/project-phoenix/models/active"
	educationModel "github.com/moto-nrw/project-phoenix/models/education"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModel "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
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
		{StaffID: assignedID},
		{StaffID: absentID, IsAbsent: true},
	}
	deps.staffRepo.byInstance[outsideWindowID] = []*scheduleModel.InstanceStaff{{StaffID: assignedID}}
	deps.studentRepo.byInstance[instanceID] = []*scheduleModel.InstanceStudent{
		{StudentID: 520, Status: scheduleModel.AttendanceStatusExpected},
		{StudentID: 521, Status: scheduleModel.AttendanceStatusPresent},
	}

	result, err := deps.service.PlannedNow(context.Background(), 610, false, now, now)
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, instanceID, result[0].ID)
	assert.Equal(t, []int64{assignedID}, result[0].AssignedStaffIDs)
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

	result, err := deps.service.PlannedNow(context.Background(), 620, true, now, now)

	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.True(t, result[0].IsOverdue)
}

func TestTimetableOperationsPlannedNowErrorBranches(t *testing.T) {
	now := time.Date(2026, time.May, 10, 14, 0, 0, 0, time.UTC)

	t.Run("forbids accounts without staff when admin overview is disabled", func(t *testing.T) {
		deps := newTimetableOpsDeps()

		result, err := deps.service.PlannedNow(context.Background(), 621, false, now, now)

		require.ErrorIs(t, err, ErrTimetableOperationForbidden)
		assert.Nil(t, result)
	})

	t.Run("propagates instance listing errors", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		wireAssignedStaff(deps, 622, 431, 221, 331)
		deps.instanceRepo.findByDateErr = errors.New("date query failed")

		result, err := deps.service.PlannedNow(context.Background(), 622, false, now, now)

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

		result, err := deps.service.PlannedNow(context.Background(), 623, false, now, now)

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

		result, err := deps.service.PlannedNow(context.Background(), 624, false, now, now)

		require.EqualError(t, err, "student query failed")
		assert.Nil(t, result)
	})
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
	require.Len(t, deps.broadcaster.events, 1)
	assert.Equal(t, int64(721), deps.broadcaster.events[0].tenantID)
	assert.Equal(t, realtime.EventActiveSupervisionChanged, deps.broadcaster.events[0].event.Type)
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
		wireAssignedStaff(deps, 676, 497, 257, instanceID)
		deps.staffRepo.byInstance[instanceID] = []*scheduleModel.InstanceStaff{{StaffID: 258}}
		deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, activeGroupID)

		_, err := deps.service.Roster(context.Background(), 676, false, instanceID)

		require.ErrorIs(t, err, ErrTimetableOperationForbidden)
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

func TestTimetableOperationsPatchAttendanceBranches(t *testing.T) {
	deps := newTimetableOpsDeps()
	instanceID := int64(413)
	wireAssignedStaff(deps, 682, 504, 263, instanceID)
	deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, 299)

	row, err := deps.service.PatchAttendance(context.Background(), 682, false, instanceID, 559, scheduleModel.AttendanceFieldPatch{})

	require.ErrorIs(t, err, ErrTimetableOperationNotFound)
	assert.Nil(t, row)
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
}

func TestTimetableOperationsBroadcastBranches(t *testing.T) {
	ctx := tenant.WithTenantID(context.Background(), 722)

	t.Run("skips when broadcaster is nil", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		deps.service.(*timetableOperationsService).deps.Broadcaster = nil

		deps.service.(*timetableOperationsService).broadcastAttendanceChanged(ctx, 414, 560)
	})

	t.Run("skips inactive instance without active group", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		deps.instanceRepo.byID[414] = instanceWithTimes(414, scheduleModel.InstanceStatusPlanned, time.Now(), time.Now().Add(time.Hour))

		deps.service.(*timetableOperationsService).broadcastAttendanceChanged(ctx, 414, 560)

		assert.Empty(t, deps.broadcaster.events)
	})

	t.Run("logs and continues when broadcast fails", func(t *testing.T) {
		deps := newTimetableOpsDeps()
		deps.broadcaster.err = errors.New("send failed")
		deps.instanceRepo.byID[414] = activeInstance(414, 300)

		deps.service.(*timetableOperationsService).broadcastAttendanceChanged(ctx, 414, 560)

		require.Len(t, deps.broadcaster.events, 1)
	})
}

func TestTimetableOperationsDependencyAndErrorBranches(t *testing.T) {
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
}

func TestTimetableOperationHelpers(t *testing.T) {
	now := time.Date(2026, time.May, 10, 14, 0, 0, 0, time.UTC)
	assert.True(t, plannedNowWindow(instanceWithTimes(406, scheduleModel.InstanceStatusPlanned, now.Add(-16*time.Minute), now), now))
	assert.True(t, plannedNowWindow(instanceWithTimes(407, scheduleModel.InstanceStatusPlanned, now.Add(14*time.Minute), now), now))
	assert.False(t, plannedNowWindow(instanceWithTimes(408, scheduleModel.InstanceStatusPlanned, now.Add(16*time.Minute), now), now))
	assert.True(t, staffAssigned([]*scheduleModel.InstanceStaff{{StaffID: 255}}, 255))
	assert.False(t, staffAssigned([]*scheduleModel.InstanceStaff{{StaffID: 255, IsAbsent: true}}, 255))
	planned, ok := findPlanned([]*scheduleModel.InstanceStudent{{StudentID: 556}}, 556)
	require.True(t, ok)
	assert.Equal(t, int64(556), planned.StudentID)
	assert.False(t, isNoRows(errors.New("ordinary error")))
}

func wireAssignedStaff(deps *timetableOpsTestDeps, accountID, personID, staffID, instanceID int64) {
	deps.personService.accountPerson = &usersModel.Person{}
	deps.personService.accountPerson.ID = personID
	deps.personService.staffByPersonID[personID] = &usersModel.Staff{}
	deps.personService.staffByPersonID[personID].ID = staffID
	deps.staffRepo.byInstance[instanceID] = []*scheduleModel.InstanceStaff{{StaffID: staffID}}
}

func instanceWithTimes(id int64, status string, start, end time.Time) *scheduleModel.ActivityInstance {
	inst := &scheduleModel.ActivityInstance{
		Date:      time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC),
		Title:     "Lernzeit",
		StartTime: start,
		EndTime:   end,
		RoomID:    810,
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
	activeService   *fakeOpsActiveService
	supervisors     *fakeOpsSupervisorRepo
	visitRepo       *fakeOpsVisitRepo
	students        *fakeOpsStudentRepo
	groups          *fakeOpsEducationGroupRepo
	personService   *fakeOpsPersonService
	settings        *fakeOpsSettings
	broadcaster     *fakeOpsBroadcaster
}

func newTimetableOpsDeps() *timetableOpsTestDeps {
	deps := &timetableOpsTestDeps{
		instanceRepo:    &fakeOpsInstanceRepo{byID: map[int64]*scheduleModel.ActivityInstance{}},
		staffRepo:       &fakeOpsStaffRepo{byInstance: map[int64][]*scheduleModel.InstanceStaff{}},
		studentRepo:     &fakeOpsInstanceStudentRepo{byInstance: map[int64][]*scheduleModel.InstanceStudent{}, byInstanceStudent: map[instanceStudentKey]*scheduleModel.InstanceStudent{}},
		instanceService: &fakeOpsInstanceService{},
		activeGroups:    &fakeOpsActiveGroupRepo{lastActivity: map[int64]time.Time{}},
		activeService:   &fakeOpsActiveService{},
		supervisors:     &fakeOpsSupervisorRepo{byActiveGroup: map[int64][]*activeModel.GroupSupervisor{}},
		visitRepo:       &fakeOpsVisitRepo{byActiveGroup: map[int64][]*activeModel.Visit{}, currentByStudent: map[int64]*activeModel.Visit{}},
		students:        &fakeOpsStudentRepo{byID: map[int64]*usersModel.Student{}},
		groups:          &fakeOpsEducationGroupRepo{byID: map[int64]*educationModel.Group{}},
		personService:   &fakeOpsPersonService{people: map[int64]*usersModel.Person{}, staffByPersonID: map[int64]*usersModel.Staff{}},
		settings:        &fakeOpsSettings{},
		broadcaster:     &fakeOpsBroadcaster{},
	}
	deps.service = NewTimetableOperationsService(TimetableOperationsDependencies{
		InstanceRepo:       deps.instanceRepo,
		InstanceStaffRepo:  deps.staffRepo,
		InstanceStudents:   deps.studentRepo,
		InstanceService:    deps.instanceService,
		ActiveGroupRepo:    deps.activeGroups,
		ActiveService:      deps.activeService,
		SupervisorRepo:     deps.supervisors,
		VisitRepo:          deps.visitRepo,
		StudentRepo:        deps.students,
		EducationGroupRepo: deps.groups,
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

func (r *fakeOpsInstanceRepo) FindByTenantAndDate(_ context.Context, _ time.Time) ([]*scheduleModel.ActivityInstance, error) {
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
}

func (r *fakeOpsVisitRepo) FindByActiveGroupID(_ context.Context, activeGroupID int64) ([]*activeModel.Visit, error) {
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

type fakeOpsPersonService struct {
	accountPerson   *usersModel.Person
	people          map[int64]*usersModel.Person
	staffByPersonID map[int64]*usersModel.Staff
	staffErr        error
}

func (s *fakeOpsPersonService) FindByAccountID(_ context.Context, _ int64) (*usersModel.Person, error) {
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

func (s *fakeOpsPersonService) StaffRepository() usersModel.StaffRepository {
	return fakeOpsStaffByPersonRepo{s: s}
}

type fakeOpsStaffByPersonRepo struct {
	usersModel.StaffRepository
	s *fakeOpsPersonService
}

func (r fakeOpsStaffByPersonRepo) FindByPersonID(_ context.Context, personID int64) (*usersModel.Staff, error) {
	if r.s.staffErr != nil {
		return nil, r.s.staffErr
	}
	return r.s.staffByPersonID[personID], nil
}

type fakeOpsSettings struct {
	enabled bool
	err     error
}

func (s *fakeOpsSettings) ResolveBool(_ context.Context, _ string) (bool, error) {
	return s.enabled, s.err
}

type fakeOpsBroadcaster struct {
	err    error
	events []struct {
		tenantID int64
		event    realtime.Event
	}
}

func (b *fakeOpsBroadcaster) BroadcastToTenant(tenantID int64, event realtime.Event) error {
	b.events = append(b.events, struct {
		tenantID int64
		event    realtime.Event
	}{tenantID: tenantID, event: event})
	return b.err
}

func (b *fakeOpsBroadcaster) BroadcastToGroup(tenantID int64, _ string, event realtime.Event) error {
	return b.BroadcastToTenant(tenantID, event)
}

func (b *fakeOpsBroadcaster) BroadcastToAll(event realtime.Event) error {
	b.events = append(b.events, struct {
		tenantID int64
		event    realtime.Event
	}{event: event})
	return nil
}
