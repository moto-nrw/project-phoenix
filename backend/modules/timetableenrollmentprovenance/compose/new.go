// Package compose builds the enrollment provenance projection for the roots,
// which bind it to Timetable (ADR 0058).
package compose

import timetableenrollmentprovenance "github.com/moto-nrw/project-phoenix/modules/timetableenrollmentprovenance/internal/adapters/postgres"

// NewReader builds the reader the root binds to Timetable.
func NewReader() timetableenrollmentprovenance.Reader {
	return timetableenrollmentprovenance.Reader{}
}
