package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

func (q *AccountRoleQueries) ListSchoolAccountListings(ctx context.Context, schoolIDs []int64) ([]identityaccess.SchoolAccountListing, error) {
	var result []domain.SchoolAccountListing
	err := q.service.run(ctx, q.service.tx.RunPlatform, "list_school_account_listings", func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		var queryErr error
		result, queryStats, queryErr = q.store.ListSchoolAccountListings(txCtx, schoolIDs)
		stats.Add(queryStats)
		return queryErr
	})
	if err != nil {
		return nil, MapError(err)
	}
	return publicSchoolAccountListings(result), nil
}

func publicSchoolAccountListings(rows []domain.SchoolAccountListing) []identityaccess.SchoolAccountListing {
	result := make([]identityaccess.SchoolAccountListing, 0, len(rows))
	for _, row := range rows {
		result = append(result, identityaccess.SchoolAccountListing(row))
	}
	return result
}
