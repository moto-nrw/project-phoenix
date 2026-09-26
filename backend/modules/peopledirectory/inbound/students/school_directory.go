package students

import "context"

// ExportSchool is the part of a school the export routes read.
type ExportSchool struct {
	Name string
}

// SchoolDirectory reads the school an export is titled with. Organisation &
// Tenancy owns the rows; the root binds this port to its capability. A school
// that does not exist is (nil, nil).
type SchoolDirectory interface {
	GetSchoolByID(ctx context.Context, id int64) (*ExportSchool, error)
}
