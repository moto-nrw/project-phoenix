package httpintegration_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/studentpresence"
	"github.com/moto-ogs/project-phoenix/modules/timetable"
	"github.com/moto-ogs/project-phoenix/modules/timetable/compose"
	fixtures "github.com/moto-ogs/project-phoenix/modules/timetable/fixturetest"
	"github.com/moto-ogs/project-phoenix/modules/timetable/ports"
	owners "github.com/moto-ogs/project-phoenix/roots/owners"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTimetableCheckInStudentsSchoolPortalChecksEachChildInOrder pins the
// roster gate of the assignment-bound school portal in a batch (#5815): a
// planned child and a walk-in already in the session pass, and the first
// child off the roster or excluded refuses the batch after the children
// before it, in request order.
func TestTimetableCheckInStudentsSchoolPortalChecksEachChildInOrder(t *testing.T) {
	t.Parallel()
	const (
		instanceID, activeGroupID           = int64(640), int64(641)
		plannedID, walkInID, outsiderID     = int64(642), int64(643), int64(644)
		graduateID, laterPlannedID, staffID = int64(645), int64(646), int64(649)
	)
	setup := func(t *testing.T) (*timetableOpsTestDeps, timetable.OperationCapability) {
		t.Helper()
		deps := newTimetableOpsDeps()
		wireAssignedStaff(deps, 647, 648, staffID, instanceID)
		deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, activeGroupID)
		for _, id := range []int64{plannedID, graduateID, laterPlannedID} {
			deps.studentRepo.byInstance[instanceID] = append(deps.studentRepo.byInstance[instanceID], fixtures.NewInstanceStudent(func(row *fixtures.InstanceStudent) {
				row.StudentID = id
				row.Status = fixtures.AttendanceStatusExpected()
			}))
		}
		for _, id := range []int64{plannedID, walkInID, outsiderID, laterPlannedID} {
			deps.students.byID[id] = &timetable.OperationStudent{LifecycleStudent: timetable.LifecycleStudent{ID: id}, PersonID: id + 100}
		}
		deps.students.byID[graduateID] = &timetable.OperationStudent{LifecycleStudent: timetable.LifecycleStudent{ID: graduateID, Alumnus: true}, PersonID: graduateID + 100}
		walkIn := &studentpresence.Visit{StudentID: walkInID, ActiveGroupID: activeGroupID, EntryTime: opsNow}
		deps.visitRepo.byActiveGroup[activeGroupID] = []*studentpresence.Visit{walkIn}
		deps.visitRepo.currentByStudent[walkInID] = walkIn
		// The school portal reaches only today's blocks.
		deps.now = func() time.Time { return time.Date(2026, time.May, 10, 14, 30, 0, 0, time.UTC) }
		dependencies := deps.dependencies()
		school, err := compose.NewOperations(dependencies, owners.TimetableOperationCaller{RequestSession: fixedRequestSession{
			claims: ports.RequestClaims{AccountID: 647, Scope: "school", SchoolScope: true},
		}})
		require.NoError(t, err)
		return deps, school
	}

	t.Run("planned child and walk-in pass", func(t *testing.T) {
		t.Parallel()
		deps, school := setup(t)
		_, err := school.CheckInStudents(context.Background(), 647, false, instanceID, []int64{plannedID, walkInID})
		require.NoError(t, err)
		require.Len(t, deps.activeService.created, 1)
		assert.Equal(t, plannedID, deps.activeService.created[0].StudentID)
	})
	t.Run("a child off the roster refuses after the children before it", func(t *testing.T) {
		t.Parallel()
		deps, school := setup(t)
		_, err := school.CheckInStudents(context.Background(), 647, false, instanceID, []int64{plannedID, outsiderID, laterPlannedID})
		require.ErrorIs(t, err, timetable.ErrTimetableOperationForbidden)
		require.Len(t, deps.activeService.created, 1, "the child before the refusal was written, the one after it not")
		assert.Equal(t, plannedID, deps.activeService.created[0].StudentID)
	})
	t.Run("a graduate on the roster refuses", func(t *testing.T) {
		t.Parallel()
		deps, school := setup(t)
		_, err := school.CheckInStudents(context.Background(), 647, false, instanceID, []int64{graduateID, plannedID})
		require.ErrorIs(t, err, timetable.ErrTimetableOperationForbidden)
		assert.Empty(t, deps.activeService.created)
	})
}
