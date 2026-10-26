package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/go-chi/chi/v5"

	"github.com/moto-nrw/project-phoenix/api/common"
	deliveryRuntime "github.com/moto-nrw/project-phoenix/modules/delivery/runtime"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	parentBillingCompose "github.com/moto-nrw/project-phoenix/modules/parentbilling/compose"
	parentBillingHTTP "github.com/moto-nrw/project-phoenix/modules/parentbilling/http"
)

// mountParentBillingRoutes serves the Elternabrechnung (ADR 0088, #4664, ADR
// 0115): the contribution rules of a Träger at /traeger/contribution-rules
// behind the right "Abrechnung", the contribution and confirmation of a
// holiday registration at /traeger/holiday-contributions behind the right
// "Anmeldung verwalten", and the public contribution of the registration
// form at /traeger/holiday-quotes, and the billing runs and number ranges
// at /traeger/billing-runs and /traeger/number-ranges behind the right
// "Abrechnung" (#4668, ADR 0119).
func mountParentBillingRoutes(root chi.Router, organizations HolidayOrganizations, delivery deliveryRuntime.Capability, logger *slog.Logger) {
	rules, holiday := NewParentBillingHTTP(organizations, delivery, logger)
	root.Mount("/traeger/contribution-rules", rules.TraegerRouter(ProtectedTraegerRoutes))
	root.Mount("/traeger/holiday-contributions", holiday.TraegerRouter(ProtectedTraegerRoutes))
	root.Mount("/traeger/holiday-quotes", holiday.QuoteRouter())
	runs := NewBillingRunHTTP()
	root.Mount("/traeger/billing-runs", runs.RunRouter(ProtectedTraegerRoutes))
	root.Mount("/traeger/number-ranges", runs.NumberRangeRouter(ProtectedTraegerRoutes))
}

// NewBillingRunHTTP binds the billing runs to the right "Abrechnung" and to
// Enrollment's holiday periods and the review of their registrations.
func NewBillingRunHTTP() *parentBillingHTTP.BillingRunResource {
	registrations := parentBillingBillingRegistrations{
		periods: enrollmentCompose.NewHolidayPeriods(), review: enrollmentCompose.NewHolidayRegistrationReview(),
	}
	return parentBillingHTTP.NewBillingRunResource(parentBillingCompose.NewBillingRuns(registrations),
		common.RequiresPermission(identityaccess.TraegerRightBillingManage))
}

// NewParentBillingHTTP binds the Elternabrechnung's routes to the Träger
// rights and its holiday contribution to Enrollment's holiday
// registrations.
func NewParentBillingHTTP(organizations HolidayOrganizations, delivery deliveryRuntime.Capability, logger *slog.Logger) (*parentBillingHTTP.RuleResource, *parentBillingHTTP.HolidayResource) {
	registrations := parentBillingHolidayRegistrations{
		links: enrollmentCompose.NewHolidayRegistrations(holidayTraegerDirectory{organizations: organizations},
			holidayTraegerMail{delivery: delivery}, logger.With("service", "parent-billing-holiday-registrations")),
		review: enrollmentCompose.NewHolidayRegistrationReview(),
	}
	rules := parentBillingHTTP.NewRuleResource(parentBillingCompose.NewContributionRules(),
		common.RequiresPermission(identityaccess.TraegerRightBillingManage))
	holiday := parentBillingHTTP.NewHolidayResource(parentBillingCompose.NewHolidayContributions(registrations),
		common.RequiresPermission(identityaccess.TraegerRightEnrollmentManage))
	return rules, holiday
}

// parentBillingHolidayRegistrations binds the holiday contribution's port
// to Enrollment: the public link and its period, and the review of a
// registration, which checks the Träger and records every read and the
// confirmation in the registration's audit trail (ADR 0114).
type parentBillingHolidayRegistrations struct {
	links  enrollmentOwner.HolidayRegistrations
	review enrollmentOwner.HolidayRegistrationReview
}

func (b parentBillingHolidayRegistrations) ResolveHolidayLink(ctx context.Context, token string) (parentbilling.HolidayLink, error) {
	link, err := b.links.ResolveHolidayRegistrationLink(ctx, token)
	if err != nil {
		return parentbilling.HolidayLink{}, parentBillingHolidayError(err)
	}
	return parentbilling.HolidayLink{OrganizationID: link.OrganizationID, PeriodID: link.PeriodID}, nil
}

func (b parentBillingHolidayRegistrations) HolidayWeeks(ctx context.Context, link parentbilling.HolidayLink) ([]parentbilling.HolidayWeek, error) {
	offer, err := b.links.HolidayRegistrationOffer(ctx, enrollmentOwner.HolidayRegistrationLink{OrganizationID: link.OrganizationID, PeriodID: link.PeriodID})
	if err != nil {
		return nil, parentBillingHolidayError(err)
	}
	weeks := make([]parentbilling.HolidayWeek, len(offer.Weeks))
	for i, week := range offer.Weeks {
		models := make([]parentbilling.HolidayTimeModel, len(week.TimeModels))
		for j, model := range week.TimeModels {
			models[j] = parentbilling.HolidayTimeModel{ID: model.ID, Name: model.Name, PriceCents: model.PriceCents}
		}
		weeks[i] = parentbilling.HolidayWeek{ID: week.ID, StartsOn: week.StartsOn, EndsOn: week.EndsOn, TimeModels: models}
	}
	return weeks, nil
}

func (b parentBillingHolidayRegistrations) HolidayRegistration(ctx context.Context, reviewer parentbilling.HolidayReviewer, periodID, registrationID int64) (parentbilling.HolidayRegistration, error) {
	detail, err := b.review.HolidayRegistration(ctx, enrollmentOwner.HolidayReviewer(reviewer), periodID, registrationID)
	if err != nil {
		return parentbilling.HolidayRegistration{}, parentBillingHolidayError(err)
	}
	return parentBillingRegistration(detail), nil
}

func (b parentBillingHolidayRegistrations) ConfirmHolidayRegistration(ctx context.Context, reviewer parentbilling.HolidayReviewer, periodID, registrationID int64) (parentbilling.HolidayRegistration, error) {
	detail, err := b.review.ChangeHolidayRegistration(ctx, enrollmentOwner.HolidayReviewer(reviewer), periodID, registrationID, enrollmentOwner.HolidayAuditConfirm)
	if err != nil {
		return parentbilling.HolidayRegistration{}, parentBillingHolidayError(err)
	}
	return parentBillingRegistration(detail), nil
}

// parentBillingBillingRegistrations binds the billing run's port to
// Enrollment: the period of the session's Träger and its confirmed
// registrations through the review, which records the read in the period's
// audit trail (ADR 0114).
type parentBillingBillingRegistrations struct {
	periods enrollmentOwner.HolidayPeriods
	review  enrollmentOwner.HolidayRegistrationReview
}

func (b parentBillingBillingRegistrations) HolidayBillingPeriod(ctx context.Context, reviewer parentbilling.HolidayReviewer, periodID int64) (parentbilling.HolidayBillingPeriod, error) {
	period, err := b.periods.HolidayPeriod(ctx, reviewer.OrganizationID, periodID)
	if err != nil {
		return parentbilling.HolidayBillingPeriod{}, parentBillingHolidayError(err)
	}
	billing := parentbilling.HolidayBillingPeriod{ID: period.ID, Name: period.Name}
	for _, week := range period.Weeks {
		if billing.StartsOn.IsZero() || week.StartsOn.Before(billing.StartsOn) {
			billing.StartsOn = week.StartsOn
		}
		if billing.EndsOn.IsZero() || week.EndsOn.After(billing.EndsOn) {
			billing.EndsOn = week.EndsOn
		}
	}
	return billing, nil
}

func (b parentBillingBillingRegistrations) ConfirmedHolidayRegistrations(ctx context.Context, reviewer parentbilling.HolidayReviewer, periodID int64) ([]parentbilling.HolidayBillingRegistration, error) {
	list, err := b.review.ListHolidayRegistrations(ctx, enrollmentOwner.HolidayReviewer(reviewer), periodID,
		enrollmentOwner.HolidayRegistrationFilter{Status: enrollmentOwner.HolidayRegistrationConfirmed})
	if err != nil {
		return nil, parentBillingHolidayError(err)
	}
	registrations := make([]parentbilling.HolidayBillingRegistration, len(list.Registrations))
	for i, registration := range list.Registrations {
		children := make([]parentbilling.HolidayBillingChild, len(registration.Children))
		for j, child := range registration.Children {
			children[j] = parentbilling.HolidayBillingChild{ID: child.ID, FirstName: child.FirstName, LastName: child.LastName}
		}
		registrations[i] = parentbilling.HolidayBillingRegistration{
			ID: registration.ID, GuardianFirstName: registration.GuardianFirstName, GuardianLastName: registration.GuardianLastName,
			GuardianEmail: registration.GuardianEmail, Children: children,
		}
	}
	return registrations, nil
}

func parentBillingRegistration(detail enrollmentOwner.HolidayRegistrationDetail) parentbilling.HolidayRegistration {
	registration := parentbilling.HolidayRegistration{
		ID: detail.ID, Status: detail.Status, BenefitsDeclared: detail.BenefitsDeclared,
		Children: make([]parentbilling.HolidayChild, len(detail.Children)),
	}
	for i, child := range detail.Children {
		bookings := make([]parentbilling.HolidayBooking, len(child.Bookings))
		for j, booking := range child.Bookings {
			bookings[j] = parentbilling.HolidayBooking{
				WeekID: booking.WeekID, WeekStartsOn: booking.WeekStartsOn, WeekEndsOn: booking.WeekEndsOn,
				TimeModelID: booking.TimeModelID, TimeModelName: booking.TimeModelName, PriceCents: booking.PriceCents,
			}
		}
		registration.Children[i] = parentbilling.HolidayChild{ID: child.ID, Position: i + 1, Bookings: bookings}
	}
	return registration
}

// parentBillingHolidayError names Enrollment's refusals in the
// Elternabrechnung's vocabulary; anything else stays as it is.
func parentBillingHolidayError(err error) error {
	for _, known := range []struct{ from, to error }{
		{enrollmentOwner.ErrHolidayRegistrationNotFound, parentbilling.ErrHolidayLinkNotFound},
		{enrollmentOwner.ErrHolidayPeriodNotFound, parentbilling.ErrHolidayPeriodNotFound},
		{enrollmentOwner.ErrHolidayRegistrationEntryNotFound, parentbilling.ErrHolidayRegistrationNotFound},
		{enrollmentOwner.ErrHolidayRegistrationStateConflict, parentbilling.ErrHolidayRegistrationStateConflict},
	} {
		if errors.Is(err, known.from) {
			return fmt.Errorf("%w: %w", known.to, err)
		}
	}
	return err
}
