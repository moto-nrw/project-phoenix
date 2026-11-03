package application

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
)

// HolidayStore persists the rules and the frozen contributions and knows
// which registrations an issued invoice bills (#5621).
type HolidayStore interface {
	Store
	HolidaySiblingStore
	// IssuedRegistrations names the registrations of the period that an
	// invoice of the service already bills.
	IssuedRegistrations(ctx context.Context, organizationID int64, service string, periodID int64) (map[int64]bool, error)
	// LockPeriodBilling holds the invoices of the service and period until
	// the transaction ends, so the release of a run and a change of what it
	// copies run one after another.
	LockPeriodBilling(ctx context.Context, organizationID int64, service string, periodID int64) error
}

// HolidayContributions serves parentbilling.HolidayContributions. The
// registration and its bookings come from Enrollment through the root-bound
// port, whether the Träger asks for the debtor number from the settings;
// the rules and the frozen contributions are the owner's own rows.
type HolidayContributions struct {
	store         HolidayStore
	registrations parentbilling.HolidayRegistrations
	settings      parentbilling.HolidayBillingSettings
}

var _ parentbilling.HolidayContributions = (*HolidayContributions)(nil)

// NewHolidayContributions builds the holiday contribution.
func NewHolidayContributions(store HolidayStore, registrations parentbilling.HolidayRegistrations, settings parentbilling.HolidayBillingSettings) *HolidayContributions {
	if store == nil || registrations == nil || settings == nil {
		panic("parent billing: the store, the holiday registrations and the settings are required")
	}
	return &HolidayContributions{store: store, registrations: registrations, settings: settings}
}

// ResolveHolidayLink resolves a public link; an empty key names no link.
func (h *HolidayContributions) ResolveHolidayLink(ctx context.Context, token string) (parentbilling.HolidayLink, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return parentbilling.HolidayLink{}, parentbilling.ErrHolidayLinkNotFound
	}
	link, err := h.registrations.ResolveHolidayLink(ctx, token)
	if err != nil {
		return parentbilling.HolidayLink{}, err
	}
	if link.OrganizationID <= 0 || link.PeriodID <= 0 {
		return parentbilling.HolidayLink{}, parentbilling.ErrHolidayLinkNotFound
	}
	return link, nil
}

// QuoteHolidayContribution computes the contribution of the public form
// from the period's current prices and the Träger's current rules. Every
// booking must name a time model of a week of the period, at most one per
// week and child.
func (h *HolidayContributions) QuoteHolidayContribution(ctx context.Context, link parentbilling.HolidayLink, children []parentbilling.HolidayQuoteChild, benefitsDeclared bool) (parentbilling.HolidayContribution, error) {
	if link.OrganizationID <= 0 || link.PeriodID <= 0 {
		return parentbilling.HolidayContribution{}, parentbilling.ErrHolidayLinkNotFound
	}
	if len(children) == 0 || len(children) > 3 {
		return parentbilling.HolidayContribution{}, fmt.Errorf("%w: one to three children", parentbilling.ErrInvalidHolidayQuote)
	}
	weeks, err := h.registrations.HolidayWeeks(ctx, link)
	if err != nil {
		return parentbilling.HolidayContribution{}, err
	}
	priced, err := quotedChildren(weeks, children)
	if err != nil {
		return parentbilling.HolidayContribution{}, err
	}
	return h.compute(ctx, link.OrganizationID, priced, benefitsDeclared)
}

// HolidayContribution reads the registration through Enrollment, which
// checks that it belongs to the period of the session's Träger and records
// the read, and shows its frozen contribution, with whether an issued
// invoice bills it (#5621), or a preview with the exemption the Träger-Büro
// is about to choose. In a sibling group (#5628) it reads the other
// registrations too, ranks the preview across the group and names them.
func (h *HolidayContributions) HolidayContribution(ctx context.Context, reviewer parentbilling.HolidayReviewer, periodID, registrationID int64, benefitsExemption *bool) (parentbilling.HolidayContribution, error) {
	if reviewer.OrganizationID <= 0 || reviewer.AccountID <= 0 {
		return parentbilling.HolidayContribution{}, errTraegerRequired
	}
	registration, err := h.registrations.HolidayRegistration(ctx, reviewer, periodID, registrationID)
	if err != nil {
		return parentbilling.HolidayContribution{}, err
	}
	members, grouped, err := h.siblingsOf(ctx, reviewer, periodID, registration)
	if err != nil {
		return parentbilling.HolidayContribution{}, err
	}
	contribution, err := h.memberContribution(ctx, reviewer.OrganizationID, periodID, members, registration, benefitsExemption)
	if err != nil {
		return parentbilling.HolidayContribution{}, err
	}
	if grouped {
		issued, err := h.store.IssuedRegistrations(ctx, reviewer.OrganizationID, parentbilling.ServiceHoliday, periodID)
		if err != nil {
			return parentbilling.HolidayContribution{}, fmt.Errorf("parent billing: read issued invoices: %w", err)
		}
		contribution.Siblings = siblingView(members, registration.ID)
		contribution.SiblingDebtorNumber = groupDebtorNumber(members, registration.ID)
		contribution.SiblingsInvoiced = refuseInvoiced(memberIDs(members), issued) != nil
	}
	return contribution, nil
}

// memberContribution is the frozen contribution of the registration with
// whether an issued invoice bills it, or else its preview across its group.
func (h *HolidayContributions) memberContribution(ctx context.Context, organizationID, periodID int64, members []siblingMember,
	registration parentbilling.HolidayRegistration, benefitsExemption *bool,
) (parentbilling.HolidayContribution, error) {
	for _, member := range members {
		if member.id() != registration.ID || member.frozen == nil {
			continue
		}
		issued, err := h.store.IssuedRegistrations(ctx, organizationID, parentbilling.ServiceHoliday, periodID)
		if err != nil {
			return parentbilling.HolidayContribution{}, fmt.Errorf("parent billing: read issued invoices: %w", err)
		}
		frozen := *member.frozen
		frozen.Invoiced = issued[registration.ID]
		return frozen, nil
	}
	exemption := registration.BenefitsDeclared
	if benefitsExemption != nil {
		exemption = *benefitsExemption
	}
	computed, err := h.computeSiblings(ctx, organizationID, members, map[int64]bool{registration.ID: exemption})
	if err != nil {
		return parentbilling.HolidayContribution{}, err
	}
	if contribution, ok := computed[registration.ID]; ok {
		return contribution, nil
	}
	// A cancelled registration ranks with no sibling.
	return h.compute(ctx, organizationID, registration.Children, exemption)
}

// ConfirmHolidayRegistration confirms the registration through Enrollment
// with the debtor number and freezes the contribution it computes from the
// confirmed bookings, in the caller's Träger transaction. It holds the
// invoices of the period first, like a merge of siblings, and Enrollment
// holds the period and the registration, so no change of a price, a
// booking or a group runs in between. In a sibling group (#5628) the
// contribution ranks across the group, a blank debtor number takes the
// group's, and the other confirmed registrations of the group get their
// contribution again and the number. A missing debtor number the Träger
// asks for refuses the confirmation before anything is written. The caller
// rolls the transaction back when this fails after the confirmation.
func (h *HolidayContributions) ConfirmHolidayRegistration(ctx context.Context, reviewer parentbilling.HolidayReviewer, periodID, registrationID int64, benefitsExemption bool, debtorNumber string) (parentbilling.HolidayConfirmation, error) {
	if reviewer.OrganizationID <= 0 || reviewer.AccountID <= 0 {
		return parentbilling.HolidayConfirmation{}, errTraegerRequired
	}
	others, issued, debtorNumber, err := h.confirmingSiblings(ctx, reviewer, periodID, registrationID, debtorNumber)
	if err != nil {
		return parentbilling.HolidayConfirmation{}, err
	}
	if err := h.requireDebtorNumber(ctx, reviewer.OrganizationID, debtorNumber); err != nil {
		return parentbilling.HolidayConfirmation{}, err
	}
	registration, err := h.registrations.ConfirmHolidayRegistration(ctx, reviewer, periodID, registrationID, debtorNumber)
	if err != nil {
		return parentbilling.HolidayConfirmation{}, err
	}
	if registration.Status != parentbilling.HolidayRegistrationConfirmed {
		return parentbilling.HolidayConfirmation{}, parentbilling.ErrHolidayRegistrationStateConflict
	}
	if err := h.freezeConfirmed(ctx, reviewer, periodID, registration, others, issued, benefitsExemption); err != nil {
		return parentbilling.HolidayConfirmation{}, err
	}
	frozen, found, err := h.store.HolidayContributions(ctx, reviewer.OrganizationID, registration.ID)
	if err != nil {
		return parentbilling.HolidayConfirmation{}, fmt.Errorf("parent billing: read holiday contribution: %w", err)
	}
	if !found {
		return parentbilling.HolidayConfirmation{}, errors.New("parent billing: the frozen holiday contribution is missing")
	}
	return parentbilling.HolidayConfirmation{RegistrationID: registration.ID, Status: registration.Status, Contribution: frozen}, nil
}

// confirmingSiblings holds the invoices of the period and reads the other
// registrations of the group of the registration about to be confirmed; a
// blank debtor number takes the group's (#5628).
func (h *HolidayContributions) confirmingSiblings(ctx context.Context, reviewer parentbilling.HolidayReviewer, periodID, registrationID int64,
	debtorNumber string,
) ([]siblingMember, map[int64]bool, string, error) {
	groups, issued, err := h.lockSiblings(ctx, reviewer, periodID)
	if err != nil {
		return nil, nil, "", err
	}
	group, grouped := groupOf(groups, registrationID)
	if !grouped {
		return nil, issued, debtorNumber, nil
	}
	ids := slices.DeleteFunc(slices.Clone(group.RegistrationIDs), func(id int64) bool { return id == registrationID })
	others, err := h.readSiblings(ctx, reviewer, periodID, ids, nil)
	if err != nil {
		return nil, nil, "", err
	}
	if strings.TrimSpace(debtorNumber) == "" {
		debtorNumber = groupDebtorNumber(others, 0)
	}
	return others, issued, debtorNumber, nil
}

// freezeConfirmed freezes the contribution of the confirmed registration,
// ranked across its group, and gives the other confirmed registrations of
// the group their contribution again and the debtor number.
func (h *HolidayContributions) freezeConfirmed(ctx context.Context, reviewer parentbilling.HolidayReviewer, periodID int64,
	registration parentbilling.HolidayRegistration, others []siblingMember, issued map[int64]bool, benefitsExemption bool,
) error {
	members := slices.Concat(others, []siblingMember{{registration: registration}})
	computed, err := h.computeSiblings(ctx, reviewer.OrganizationID, members, map[int64]bool{registration.ID: benefitsExemption})
	if err != nil {
		return err
	}
	contribution := computed[registration.ID]
	if benefitsExemption && !contribution.BenefitsExemption {
		return parentbilling.ErrBenefitsExemptionUnavailable
	}
	if err := h.store.InsertHolidayContributions(ctx, reviewer.OrganizationID, periodID, registration.ID, reviewer.AccountID, contribution); err != nil {
		return fmt.Errorf("parent billing: freeze holiday contribution: %w", err)
	}
	if err := h.refreezeSiblings(ctx, reviewer, periodID, members, issued); err != nil {
		return err
	}
	return h.shareDebtorNumber(ctx, reviewer, periodID, others, registration.DebtorNumber, issued)
}

// compute runs the chain with the rules valid on the first booked day.
func (h *HolidayContributions) compute(ctx context.Context, organizationID int64, children []parentbilling.HolidayChild, exemption bool) (parentbilling.HolidayContribution, error) {
	rules, err := h.store.ListContributionRules(ctx, organizationID, parentbilling.ServiceHoliday)
	if err != nil {
		return parentbilling.HolidayContribution{}, fmt.Errorf("parent billing: read contribution rules: %w", err)
	}
	return computeHolidayContribution(children, validHolidayRules(rules, firstHolidayDay(children)), exemption), nil
}

// quotedChildren prices the bookings of the form with the period's time
// models.
func quotedChildren(weeks []parentbilling.HolidayWeek, children []parentbilling.HolidayQuoteChild) ([]parentbilling.HolidayChild, error) {
	type offered struct {
		week  parentbilling.HolidayWeek
		model parentbilling.HolidayTimeModel
	}
	models := map[int64]offered{}
	for _, week := range weeks {
		for _, model := range week.TimeModels {
			models[model.ID] = offered{week: week, model: model}
		}
	}
	priced := make([]parentbilling.HolidayChild, len(children))
	for i, child := range children {
		priced[i] = parentbilling.HolidayChild{Position: i + 1, Bookings: make([]parentbilling.HolidayBooking, 0, len(child.Bookings))}
		booked := map[int64]bool{}
		for _, booking := range child.Bookings {
			known, ok := models[booking.TimeModelID]
			if !ok || known.week.ID != booking.WeekID || booked[booking.WeekID] {
				return nil, fmt.Errorf("%w: child %d books a time model the period does not offer", parentbilling.ErrInvalidHolidayQuote, i+1)
			}
			booked[booking.WeekID] = true
			priced[i].Bookings = append(priced[i].Bookings, parentbilling.HolidayBooking{
				WeekID: known.week.ID, WeekStartsOn: known.week.StartsOn, WeekEndsOn: known.week.EndsOn,
				TimeModelID: known.model.ID, TimeModelName: known.model.Name, PriceCents: known.model.PriceCents,
			})
		}
	}
	return priced, nil
}
