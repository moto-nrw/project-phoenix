package compose

import "context"

func (e engine) HasActiveSchoolMembership(ctx context.Context, accountID int64) (bool, error) {
	active, err := e.service.HasActiveSchoolMembership(ctx, accountID)
	return active, mapError(err)
}

func (e engine) ListAccountRoleIDs(ctx context.Context, accountID int64) ([]int64, error) {
	ids, err := e.service.ListAccountRoleIDs(ctx, accountID)
	return ids, mapError(err)
}

func (e engine) ListActiveSchoolAccountIDs(ctx context.Context) ([]int64, error) {
	ids, err := e.service.ListActiveSchoolAccountIDs(ctx)
	return ids, mapError(err)
}
