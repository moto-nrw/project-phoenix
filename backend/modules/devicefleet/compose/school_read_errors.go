package compose

import "github.com/moto-nrw/project-phoenix/modules/devicefleet/internal/adapters/postgres"

// NewSchoolReadErrors binds the SQL error classifier without exposing a driver to HTTP.
func NewSchoolReadErrors() interface {
	Missing(error) bool
	Transient(error) bool
} {
	return postgres.SchoolReadErrors{}
}
