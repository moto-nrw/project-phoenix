package application

import "context"

func (e engine) HasActiveSchoolMembership(ctx context.Context, accountID int64) (bool, error) {
	active, err := e.Service.HasActiveSchoolMembership(ctx, accountID)
	return active, MapError(err)
}

func (e engine) ListAccountRoleIDs(ctx context.Context, accountID int64) ([]int64, error) {
	ids, err := e.Service.ListAccountRoleIDs(ctx, accountID)
	return ids, MapError(err)
}

func (e engine) ListActiveSchoolAccountIDs(ctx context.Context) ([]int64, error) {
	ids, err := e.Service.ListActiveSchoolAccountIDs(ctx)
	return ids, MapError(err)
}
