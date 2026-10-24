package parent

import "context"

// EnrollmentSchool is the part of a school the parent enrollment routes
// read.
type EnrollmentSchool struct {
	ID      int64
	Active  bool
	Hidden  bool
	Deleted bool
}

// SchoolDirectory resolves the school behind a parent enrollment link.
// Organisation & Tenancy owns the rows; the root binds this port to its
// capability. A school that does not exist is (nil, nil).
type SchoolDirectory interface {
	GetSchoolBySubdomain(ctx context.Context, subdomain string) (*EnrollmentSchool, error)
}
