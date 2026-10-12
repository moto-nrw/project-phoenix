package httpintegration_test

import (
	"context"
	"testing"
	"time"

	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTimetableOperationsCheckInStudentsChecksInTheWholeSelection pins the
// bulk check-in of #3824: a child without a visit gets one in the block, a
// child still in another running block moves over, and the batch answers one
// roster without the single-child move notice.
func TestTimetableOperationsCheckInStudentsChecksInTheWholeSelection(t *testing.T) {
	t.Parallel()

	const (
		instanceID          = int64(610)
		activeGroupID       = int64(611)
		originActiveGroupID = int64(612)
		freshStudentID      = int64(613)
		movedStudentID      = int64(614)
	)
	deps := newTimetableOpsDeps()
	wireAssignedStaff(deps, 615, 616, 617, instanceID)
	deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, activeGroupID)
	deps.students.byID[freshStudentID] = &usersModels.Student{PersonID: 618, SchoolClass: "2a"}
	deps.personService.people[618] = &usersModels.Person{FirstName: "Lena", LastName: "Muster"}
	deps.students.byID[movedStudentID] = &usersModels.Student{PersonID: 619, SchoolClass: "3b"}
	deps.personService.people[619] = &usersModels.Person{FirstName: "Tom", LastName: "Muster"}
	deps.visitRepo.currentByStudent[movedStudentID] = &studentpresence.Visit{StudentID: movedStudentID, ActiveGroupID: originActiveGroupID, EntryTime: opsNow}
	deps.activeService.moveResult = &studentpresence.StudentMoveResult{
		Moved:                  []int64{movedStudentID},
		PreviousActiveGroupIDs: map[int64]int64{movedStudentID: originActiveGroupID},
	}

	roster, err := deps.service.CheckInStudents(context.Background(), 615, false, instanceID, []int64{freshStudentID, movedStudentID})

	require.NoError(t, err)
	require.NotNil(t, roster)
	require.Len(t, deps.activeService.created, 1)
	assert.Equal(t, freshStudentID, deps.activeService.created[0].StudentID)
	assert.Equal(t, activeGroupID, deps.activeService.created[0].ActiveGroupID)
	require.Len(t, deps.activeService.moveCalls, 1)
	assert.Equal(t, []int64{movedStudentID}, deps.activeService.moveCalls[0].studentIDs)
	assert.Equal(t, activeGroupID, deps.activeService.moveCalls[0].activeGroupID)
	assert.True(t, deps.activeGroups.lastActivity[activeGroupID].After(time.Time{}))
	assert.Nil(t, roster.MovedFrom, "a batch names no single previous session")
}

// TestTimetableOperationsCheckInStudentsFailsAsAWhole pins the all-or-nothing
// contract: a child that cannot be moved fails the batch, and an inactive
// block refuses before any write.
func TestTimetableOperationsCheckInStudentsFailsAsAWhole(t *testing.T) {
	t.Parallel()

	t.Run("a stale move fails the batch", func(t *testing.T) {
		t.Parallel()
		const instanceID, activeGroupID, studentID = int64(620), int64(621), int64(622)
		deps := newTimetableOpsDeps()
		wireAssignedStaff(deps, 623, 624, 625, instanceID)
		deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, activeGroupID)
		deps.visitRepo.currentByStudent[studentID] = &studentpresence.Visit{StudentID: studentID, ActiveGroupID: 626, EntryTime: opsNow}
		deps.activeService.moveResult = &studentpresence.StudentMoveResult{}

		roster, err := deps.service.CheckInStudents(context.Background(), 623, false, instanceID, []int64{studentID})

		require.ErrorIs(t, err, timetable.ErrTimetableOperationConflict)
		assert.Nil(t, roster)
		assert.Empty(t, deps.studentRepo.updates)
	})

	t.Run("an inactive block refuses before any write", func(t *testing.T) {
		t.Parallel()
		const instanceID, studentID = int64(630), int64(632)
		deps := newTimetableOpsDeps()
		wireAssignedStaff(deps, 633, 634, 635, instanceID)
		inst := activeInstance(instanceID, 631)
		inst.Status = scheduleModels.InstanceStatusCompleted
		deps.instanceRepo.byID[instanceID] = inst

		roster, err := deps.service.CheckInStudents(context.Background(), 633, false, instanceID, []int64{studentID})

		require.ErrorIs(t, err, timetable.ErrTimetableOperationConflict)
		assert.Nil(t, roster)
		assert.Empty(t, deps.activeService.created)
		assert.Empty(t, deps.activeService.moveCalls)
	})
}
