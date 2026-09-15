package compose

import "context"

func (e engine) FindInvitedPersonIDs(ctx context.Context, email string) ([]int64, error) {
	ids, err := e.service.FindInvitedPersonIDs(ctx, email)
	return ids, mapError(err)
}
