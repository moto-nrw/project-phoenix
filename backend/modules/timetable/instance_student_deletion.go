package timetable

import "context"

// CountStudentAssignments includes planned slots and recorded attendance for
// the permanent-deletion preview. Care-exit planning uses a separate query.
func (m *Module) CountStudentAssignments(ctx context.Context, studentID int64) (int, error) {
	if studentID <= 0 {
		return 0, m.reject("count_student_assignments", ErrInvalidInstanceStudentQuery)
	}
	return m.engine.CountStudentAssignments(ctx, studentID)
}

// DeleteStudentAssignments removes a child's assignments, never the shared
// instances. The caller's deletion workflow owns the outer transaction.
func (m *Module) DeleteStudentAssignments(ctx context.Context, studentID int64) (int64, error) {
	if studentID <= 0 {
		return 0, m.reject("delete_student_assignments", ErrInvalidInstanceStudent)
	}
	return m.engine.DeleteStudentAssignments(ctx, studentID)
}
