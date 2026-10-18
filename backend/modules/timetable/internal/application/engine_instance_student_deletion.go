package application

import "context"

func (e Engine) CountStudentAssignments(ctx context.Context, studentID int64) (int, error) {
	count, err := e.service.CountStudentAssignments(ctx, studentID)
	return count, MapError(err)
}

func (e Engine) CountStudentRosterRemovals(ctx context.Context, studentID int64) (int, error) {
	count, err := e.service.CountStudentRosterRemovals(ctx, studentID)
	return count, MapError(err)
}

func (e Engine) DeleteStudentAssignments(ctx context.Context, studentID int64) (int64, error) {
	rows, err := e.service.DeleteStudentAssignments(ctx, studentID)
	return rows, MapError(err)
}
