package timetable

import "context"

func (m *Module) CountPlannedStudentAssignmentsAfter(ctx context.Context, studentIDs []int64, after string, removals []InstanceStudent) (map[int64]int, error) {
	if hasInvalidID(studentIDs) || !validDate(after) || !validCareExitAssignments(removals) {
		return nil, m.reject("count_planned_student_assignments_after", ErrInvalidInstanceStudent)
	}
	return m.engine.CountPlannedStudentAssignmentsAfter(ctx, studentIDs, after, removals)
}

func (m *Module) RemovePlannedStudentAssignmentsAfter(ctx context.Context, studentIDs []int64, after string) ([]InstanceStudent, error) {
	if hasInvalidID(studentIDs) || !validDate(after) {
		return nil, m.reject("remove_planned_student_assignments_after", ErrInvalidInstanceStudent)
	}
	return m.engine.RemovePlannedStudentAssignmentsAfter(ctx, studentIDs, after)
}

func (m *Module) RestoreCareExitStudentAssignments(ctx context.Context, studentIDs, roomIDs, statusDayIDs, pickupExceptionIDs []int64, removals []InstanceStudent) (int64, error) {
	if hasInvalidID(studentIDs) || hasInvalidID(roomIDs) || hasInvalidID(statusDayIDs) || hasInvalidID(pickupExceptionIDs) || !validCareExitAssignments(removals) {
		return 0, m.reject("restore_care_exit_student_assignments", ErrInvalidInstanceStudent)
	}
	return m.engine.RestoreCareExitStudentAssignments(ctx, studentIDs, roomIDs, statusDayIDs, pickupExceptionIDs, removals)
}

func validCareExitAssignments(removals []InstanceStudent) bool {
	for _, row := range removals {
		if row.TenantID <= 0 || row.StudentID <= 0 || row.InstanceID <= 0 {
			return false
		}
	}
	return true
}
