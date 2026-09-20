package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

func (q *AccountRoleQueries) ListSchoolAccountListings(ctx context.Context, schoolIDs []int64) (result []domain.SchoolAccountListing, err error) {
	err = q.service.run(ctx, q.service.tx.RunPlatform, "list_school_account_listings", func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		var queryErr error
		result, queryStats, queryErr = q.store.ListSchoolAccountListings(txCtx, schoolIDs)
		stats.Add(queryStats)
		return queryErr
	})
	return result, err
}
