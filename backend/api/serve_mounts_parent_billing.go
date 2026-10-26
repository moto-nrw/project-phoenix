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
// form at /traeger/holiday-quotes.
func mountParentBillingRoutes(root chi.Router, organizations HolidayOrganizations, delivery deliveryRuntime.Capability, logger *slog.Logger) {
	rules, holiday := NewParentBillingHTTP(organizations, delivery, logger)
	root.Mount("/traeger/contribution-rules", rules.TraegerRouter(ProtectedTraegerRoutes))
	root.Mount("/traeger/holiday-contributions", holiday.TraegerRouter(ProtectedTraegerRoutes))
	root.Mount("/traeger/holiday-quotes", holiday.QuoteRouter())
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
