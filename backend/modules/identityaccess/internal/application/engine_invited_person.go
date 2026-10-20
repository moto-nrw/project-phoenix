package application

import "context"

func (e engine) FindInvitedPersonIDs(ctx context.Context, email string) ([]int64, error) {
	ids, err := e.Service.FindInvitedPersonIDs(ctx, email)
	return ids, MapError(err)
}

func (e engine) CountStudentGuardianInvitations(ctx context.Context, studentID int64) (int, error) {
	count, err := e.Service.CountStudentGuardianInvitations(ctx, studentID)
	return count, MapError(err)
}
