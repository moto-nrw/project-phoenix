package compose

import (
	"context"
	"time"
)

func (e engine) ListOpenStudentAssignments(ctx context.Context, studentIDs []int64) ([]int64, error) {
	value, err := e.service.ListOpenStudentAssignments(ctx, studentIDs)
	return value, mapError(err)
}

func (e engine) LatestStudentAssignmentAttendanceDate(ctx context.Context, studentID int64) (*string, error) {
	value, err := e.service.LatestStudentAssignmentAttendanceDate(ctx, studentID)
	return value, mapError(err)
}

func (e engine) CloseOpenStudentAssignments(ctx context.Context, studentIDs []int64, at time.Time) (int64, error) {
	value, err := e.service.CloseOpenStudentAssignments(ctx, studentIDs, at)
	return value, mapError(err)
}
