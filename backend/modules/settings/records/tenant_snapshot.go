package records

import "context"

// TenantSnapshot is one school's settings resolved ahead of use. Bind returns
// a context whose settings reads the snapshot serves.
type TenantSnapshot interface {
	Bind(ctx context.Context) context.Context
}

// TenantSnapshots resolves the named settings of many schools in one read.
type TenantSnapshots func(ctx context.Context, tenantIDs []int64, keys []string) (map[int64]TenantSnapshot, error)
