package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

func (q *AccountRoleQueries) FindActiveGuardianMemberships(ctx context.Context, accountIDs []int64) (result map[int64][]int64, err error) {
	err = q.service.run(ctx, q.service.tx.RunPlatform, "find_active_guardian_memberships", func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		var queryErr error
		result, queryStats, queryErr = q.store.FindActiveGuardianMemberships(txCtx, accountIDs)
		stats.Add(queryStats)
		return queryErr
	})
	return result, err
}
