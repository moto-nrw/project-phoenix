package operator

import "context"

// SchoolLookup reads the slug of one school for the operator settings
// routes. The root binds it to Organisation & Tenancy.
type SchoolLookup interface {
	SchoolSlug(ctx context.Context, schoolID int64) (string, error)
}
