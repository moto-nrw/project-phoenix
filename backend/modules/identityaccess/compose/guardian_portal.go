package compose

import "context"

func (e engine) FindActiveGuardianMemberships(ctx context.Context, accountIDs []int64) (map[int64][]int64, error) {
	result, err := e.accountRoleQueries.FindActiveGuardianMemberships(ctx, accountIDs)
	return result, mapError(err)
}

func (e engine) FindLoginReadyGuardianMemberships(ctx context.Context, accountIDs []int64) (map[int64][]int64, error) {
	result, err := e.accountRoleQueries.FindLoginReadyGuardianMemberships(ctx, accountIDs)
	return result, mapError(err)
}
