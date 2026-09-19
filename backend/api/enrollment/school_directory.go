package enrollment

import "context"

// PublicSchool is the part of a school the public enrollment routes read.
type PublicSchool struct {
	ID      int64
	Deleted bool
}

// SchoolDirectory resolves the school behind a public enrollment link.
// Organisation & Tenancy owns the rows; the root binds this port to its
// capability. A school that does not exist is (nil, nil).
type SchoolDirectory interface {
	GetSchoolBySlug(ctx context.Context, slug string) (*PublicSchool, error)
}
