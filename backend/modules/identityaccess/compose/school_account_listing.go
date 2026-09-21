package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

func (e engine) ListSchoolAccountListings(ctx context.Context, schoolIDs []int64) ([]identityaccess.SchoolAccountListing, error) {
	rows, err := e.accountRoleQueries.ListSchoolAccountListings(ctx, schoolIDs)
	if err != nil {
		return nil, mapError(err)
	}
	result := make([]identityaccess.SchoolAccountListing, 0, len(rows))
	for _, row := range rows {
		result = append(result, identityaccess.SchoolAccountListing(row))
	}
	return result, nil
}
