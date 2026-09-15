package compose

import "context"

func (e engine) FindInvitedPersonIDs(ctx context.Context, email string) ([]int64, error) {
	ids, err := e.service.FindInvitedPersonIDs(ctx, email)
	return ids, mapError(err)
}

func (e engine) CountStudentGuardianInvitations(ctx context.Context, studentID int64) (int, error) {
	count, err := e.service.CountStudentGuardianInvitations(ctx, studentID)
	return count, mapError(err)
}
