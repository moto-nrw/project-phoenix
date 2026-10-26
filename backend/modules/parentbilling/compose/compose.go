// Package compose builds the Elternabrechnung (parent-billing, ADR 0088).
package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/modules/parentbilling/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/parentbilling/internal/adapters/rendering"
	"github.com/moto-nrw/project-phoenix/modules/parentbilling/internal/application"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
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

// NewBillingRuns builds the billing runs (#4668, ADR 0119, ADR 0122) over the
// holiday registrations the root binds and Document Rendering's lists.
func NewBillingRuns(registrations parentbilling.HolidayBillingRegistrations) parentbilling.BillingRuns {
	return application.NewBillingRuns(postgres.NewStore(), registrations, rendering.NewRenderer(), calendar.TodayDate)
}

// NewBillingLetters builds the confirmation letters of the released runs
// (#4669, ADR 0121) over the letterhead of the Träger, which the root
// binds, and the letter layout of Document Rendering.
func NewBillingLetters(letterheads parentbilling.BillingLetterheads) parentbilling.BillingLetterService {
	return application.NewBillingLetters(postgres.NewStore(), letterheads, rendering.NewRenderer(), calendar.TodayDate)
}
