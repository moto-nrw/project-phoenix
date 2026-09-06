package compose

import "context"

func (e engine) CountStudentAssignments(ctx context.Context, studentID int64) (int, error) {
	count, err := e.service.CountStudentAssignments(ctx, studentID)
	return count, mapError(err)
}

func (e engine) DeleteStudentAssignments(ctx context.Context, studentID int64) (int64, error) {
	rows, err := e.service.DeleteStudentAssignments(ctx, studentID)
	return rows, mapError(err)
}
