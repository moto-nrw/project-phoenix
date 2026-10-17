package application

import (
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
)

// OffboardingCountsToPublic maps the offboarding counts onto the contract.
func OffboardingCountsToPublic(value domain.OffboardingCounts) workforce.OffboardingCounts {
	return workforce.OffboardingCounts{Absences: value.Absences, Shifts: value.Shifts, Series: value.Series, Substitutions: value.Substitutions}
}
