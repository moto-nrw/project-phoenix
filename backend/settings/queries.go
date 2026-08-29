// Package settings exposes typed tenant-settings queries.
package settings

import (
	"context"
	"errors"
)

var ErrQueriesUnavailable = errors.New("settings queries are unavailable")

type enrollmentReader interface {
	EnrollmentEnabledForTenants(ctx context.Context, tenantIDs []int64) (map[int64]bool, error)
}

// Queries is the public, typed read seam for tenant settings.
type Queries struct {
	enrollment enrollmentReader
}

func NewQueries(enrollment enrollmentReader) *Queries {
	return &Queries{enrollment: enrollment}
}

// EnrollmentEnabledForTenants reports the resolved enrollment master switch
// for every requested tenant. Missing overrides retain the registered platform
// default; lookup failures are returned to the caller.
func (q *Queries) EnrollmentEnabledForTenants(ctx context.Context, tenantIDs []int64) (map[int64]bool, error) {
	if q == nil || q.enrollment == nil {
		return nil, ErrQueriesUnavailable
	}
	return q.enrollment.EnrollmentEnabledForTenants(ctx, tenantIDs)
}
