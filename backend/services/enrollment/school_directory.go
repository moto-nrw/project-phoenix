package enrollment

import "context"

// School is the part of a school the enrollment flows read.
type School struct {
	Name      string
	Subdomain string
	// Settings is the school's raw settings JSON; the e-mail branding reads
	// the login image from it.
	Settings string
	Deleted  bool
}

// SchoolDirectory reads the school a request belongs to. Organisation &
// Tenancy owns the row; the root binds this port to its capability. A school
// that does not exist is (nil, nil).
type SchoolDirectory interface {
	FindSchool(ctx context.Context, id int64) (*School, error)
}
