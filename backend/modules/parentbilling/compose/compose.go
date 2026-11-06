// Package compose builds the Elternabrechnung (parent-billing, ADR 0088).
package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/modules/parentbilling/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/parentbilling/internal/adapters/rendering"
	"github.com/moto-nrw/project-phoenix/modules/parentbilling/internal/application"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// NewContributionRules builds the contribution rules and cost bearers (#5627)
// of a Träger, in the Träger transaction of the portal route (ADR 0105).
func NewContributionRules() parentbilling.ContributionRules {
	return application.NewRules(postgres.NewStore())
}

// NewHolidayContributions builds the holiday contribution (#4664, ADR 0115)
// over Enrollment's holiday registrations, which the root binds.
func NewHolidayContributions(registrations parentbilling.HolidayRegistrations, settings parentbilling.HolidayBillingSettings) parentbilling.HolidayContributions {
	return application.NewHolidayContributions(postgres.NewStore(), registrations, settings)
}

// NewBillingRuns builds the billing runs (#4668, ADR 0119, ADR 0122) over the
// holiday registrations the root binds and Document Rendering's lists.
func NewBillingRuns(registrations parentbilling.HolidayBillingRegistrations, settings parentbilling.HolidayBillingSettings) parentbilling.BillingRuns {
	return application.NewBillingRuns(postgres.NewStore(), registrations, settings, rendering.NewRenderer(), calendar.TodayDate)
}

// NewBillingLetterMail builds the mail and print of the letters (#4670,
// ADR 0123) over the post the root binds.
func NewBillingLetterMail(post parentbilling.BillingLetterPost) parentbilling.BillingLetterMailService {
	return application.NewBillingLetterMail(postgres.NewStore(), post, rendering.NewRenderer())
}

// NewLetterMailRenderer renders the mail of a letter for the Delivery worker.
func NewLetterMailRenderer() parentbilling.LetterMailRenderer { return application.RenderLetterMail }

// NewBillingLetters builds the confirmation letters of the released runs
// (#4669, ADR 0121) over the letterhead of the Träger, which the root
// binds, and the letter layout of Document Rendering.
func NewBillingLetters(letterheads parentbilling.BillingLetterheads) parentbilling.BillingLetterService {
	return application.NewBillingLetters(postgres.NewStore(), letterheads, rendering.NewRenderer(), calendar.TodayDate)
}

// NewParentDocuments builds the inbox of the parents portal (#4671, ADR
// 0126). It runs in the Träger transaction the parents portal opens.
func NewParentDocuments() parentbilling.ParentDocuments {
	return application.NewParentDocuments(postgres.NewStore())
}

// NewBillingFamilies builds the invoices and amounts per family (#4691) in
// the Träger transaction of the portal route.
func NewBillingFamilies() parentbilling.BillingFamilies {
	return application.NewBillingFamilies(postgres.NewStore())
}
