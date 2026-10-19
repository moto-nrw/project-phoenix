package owners

import (
	"context"
	"errors"
	"time"

	organizationModule "github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
)

// schedulerTenantDirectory lists the tenant IDs the worker iterates and
// captures the billing key-date counts (#2791), both owned by Organisation &
// Tenancy.
type schedulerTenantDirectory struct {
	schools organizationModule.Query
	billing organizationModule.BillingReport
}

func (d schedulerTenantDirectory) RecordDueBillingKeyDates(ctx context.Context, now time.Time) (int, error) {
	if d.billing == nil {
		return 0, errors.New("billing report is not composed")
	}
	return d.billing.RecordDueBillingKeyDates(ctx, now)
}

func (d schedulerTenantDirectory) ListActiveTenantIDs(ctx context.Context) ([]int64, error) {
	schools, err := d.schools.ListActiveSchools(ctx)
	return schoolIDs(schools), err
}

func (d schedulerTenantDirectory) ListNonDeletedTenantIDs(ctx context.Context) ([]int64, error) {
	schools, err := d.schools.ListNonDeletedSchools(ctx)
	return schoolIDs(schools), err
}

func schoolIDs(schools []organizationModule.School) []int64 {
	ids := make([]int64, 0, len(schools))
	for _, school := range schools {
		ids = append(ids, school.ID)
	}
	return ids
}
