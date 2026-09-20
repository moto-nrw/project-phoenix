package compose

import "context"

func (e engine) ListActiveAccountSchoolIDs(ctx context.Context, accountID int64) ([]int64, error) {
	ids, err := e.accountRoleQueries.ListActiveAccountSchoolIDs(ctx, accountID)
	return ids, mapError(err)
}

func (e engine) ListSchoolAccountRoleNames(ctx context.Context, accountID int64) ([]string, error) {
	names, err := e.accountRoleQueries.ListSchoolAccountRoleNames(ctx, accountID)
	return names, mapError(err)
}

func (e engine) FindSystemRoleID(ctx context.Context, name string) (int64, bool, error) {
	id, found, err := e.accountRoleQueries.FindSystemRoleID(ctx, name)
	return id, found, mapError(err)
}
