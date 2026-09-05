package timetable

import "context"

func (m *Module) LockOpenStudentAssignments(ctx context.Context, studentIDs []int64) error {
	if hasInvalidID(studentIDs) {
		return m.reject("lock_open_student_assignments", ErrInvalidInstanceStudent)
	}
	return m.engine.LockOpenStudentAssignments(ctx, studentIDs)
}

func (m *Module) LockPlannedStudentAssignmentsAfter(ctx context.Context, studentIDs []int64, after string) error {
	if hasInvalidID(studentIDs) || !validDate(after) {
		return m.reject("lock_planned_student_assignments_after", ErrInvalidInstanceStudent)
	}
	return m.engine.LockPlannedStudentAssignmentsAfter(ctx, studentIDs, after)
}

func (m *Module) ReconnectCareExitAssignmentPickupExceptions(ctx context.Context, studentIDs, pickupExceptionIDs []int64, removals []InstanceStudent) error {
	if hasInvalidID(studentIDs) || hasInvalidID(pickupExceptionIDs) || !validCareExitAssignments(removals) {
		return m.reject("reconnect_care_exit_assignment_pickup_exceptions", ErrInvalidInstanceStudent)
	}
	return m.engine.ReconnectCareExitAssignmentPickupExceptions(ctx, studentIDs, pickupExceptionIDs, removals)
}
