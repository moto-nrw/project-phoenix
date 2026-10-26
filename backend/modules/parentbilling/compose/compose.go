// Package compose builds the Elternabrechnung (parent-billing, ADR 0088).
package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/modules/parentbilling/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/parentbilling/internal/application"
)

// NewContributionRules builds the contribution rules of a Träger. They run
// in the Träger transaction the Träger-Portal route opens (ADR 0105).
func NewContributionRules() parentbilling.ContributionRules {
	return application.NewRules(postgres.NewStore())
}

// NewHolidayContributions builds the holiday contribution (#4664, ADR 0115)
// over Enrollment's holiday registrations, which the root binds.
func NewHolidayContributions(registrations parentbilling.HolidayRegistrations) parentbilling.HolidayContributions {
	return application.NewHolidayContributions(postgres.NewStore(), registrations)
}
