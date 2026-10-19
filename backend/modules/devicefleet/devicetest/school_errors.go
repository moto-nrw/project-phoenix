package devicetest

import devicefleetCompose "github.com/moto-nrw/project-phoenix/modules/devicefleet/compose"

// SchoolErrors returns the same owner classifier Root binds in production.
func SchoolErrors() interface {
	Missing(error) bool
	Transient(error) bool
} {
	return devicefleetCompose.NewSchoolReadErrors()
}
