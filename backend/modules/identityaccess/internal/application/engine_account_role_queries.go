package application

import "context"

func (e engine) CountActiveAccountsBySchoolGroups(ctx context.Context, groups map[int64][]int64) (map[int64]int, error) {
	counts, err := e.accountRoleQueries.CountActiveAccountsBySchoolGroups(ctx, groups)
	return counts, MapError(err)
}

func (e engine) ListEffectiveAdminAccountIDs(ctx context.Context) ([]int64, error) {
	ids, err := e.accountRoleQueries.ListEffectiveAdminAccountIDs(ctx)
	return ids, MapError(err)
}

func (e engine) FindActiveSchoolMemberships(ctx context.Context, accountIDs, schoolIDs []int64) (map[int64][]int64, error) {
	result, err := e.accountRoleQueries.FindActiveSchoolMemberships(ctx, accountIDs, schoolIDs)
	return result, MapError(err)
}

func (e engine) ListActiveAccountSchoolIDs(ctx context.Context, accountID int64) ([]int64, error) {
	ids, err := e.accountRoleQueries.ListActiveAccountSchoolIDs(ctx, accountID)
	return ids, MapError(err)
}

func (e engine) ListSchoolAccountRoleNames(ctx context.Context, accountID int64) ([]string, error) {
	names, err := e.accountRoleQueries.ListSchoolAccountRoleNames(ctx, accountID)
	return names, MapError(err)
}

func (e engine) FindSystemRoleID(ctx context.Context, name string) (int64, bool, error) {
	id, found, err := e.accountRoleQueries.FindSystemRoleID(ctx, name)
	return id, found, MapError(err)
}
