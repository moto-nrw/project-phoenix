package ports

import (
	"context"
)

// SchoolDirectory names the schools of the inbox rows and the announcement
// feed. Organisation & Tenancy owns the schools; the composition root binds
// it.
type SchoolDirectory interface {
	// SchoolName is the name of one school.
	SchoolName(ctx context.Context, tenantID int64) (string, error)
	// SchoolNames names the schools with the given IDs; an unknown school is
	// absent.
	SchoolNames(ctx context.Context, ids []int64) (map[int64]string, error)
}
