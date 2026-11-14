package application

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// HolidayStore persists the rules and the frozen contributions and knows
// which registrations an issued invoice bills (#5621).
type HolidayStore interface {
	Store
	HolidaySiblingStore
	FamilyMemoryStore
	// IssuedRegistrations names the registrations of the period that an
	// invoice of the service already bills.
	IssuedRegistrations(ctx context.Context, organizationID int64, service string, periodID int64) (map[int64]bool, error)
	// IssuedContributions names them with what their invoice billed the
	// family and the cost bearer, without the corrections of the run (#5662).
	IssuedContributions(ctx context.Context, organizationID int64, service string, periodID int64) (map[int64]parentbilling.IssuedContribution, error)
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
	// today is the Berlin day the family's number is needed from (#6692).
	today func() calendar.Date
}

var _ parentbilling.HolidayContributions = (*HolidayContributions)(nil)

// NewHolidayContributions builds the holiday contribution; a nil today uses
// the Berlin day.
func NewHolidayContributions(store HolidayStore, registrations parentbilling.HolidayRegistrations, settings parentbilling.HolidayBillingSettings,
	today func() calendar.Date,
) (parentbilling.HolidayContributions, parentbilling.HolidayAllocationBilling) {
	if store == nil || registrations == nil || settings == nil {
		panic("parent billing: the store, the holiday registrations and the settings are required")
	}
	if today == nil {
		today = calendar.TodayDate
	}
	contributions := &HolidayContributions{store: store, registrations: registrations, settings: settings, today: today}
	return contributions, contributions
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
	return h.compute(ctx, link.OrganizationID, priced, benefitsDeclared, nil)
}

// HolidayContribution reads the registration through Enrollment, which
// checks that it belongs to the period of the session's Träger and records
// the read, and shows its frozen contribution, with whether an issued
// invoice bills it (#5621), or a preview with the exemption or the cost
// bearer the Träger-Büro is about to choose (#5627). A chosen cost bearer
// replaces the parents' statement on benefits. In a sibling group (#5628)
// it reads the other registrations too, ranks the preview across the group
// and names them.
func (h *HolidayContributions) HolidayContribution(ctx context.Context, reviewer parentbilling.HolidayReviewer, periodID, registrationID int64, benefitsExemption *bool, costBearerID int64) (parentbilling.HolidayContribution, error) {
	if reviewer.OrganizationID <= 0 || reviewer.AccountID <= 0 {
		return parentbilling.HolidayContribution{}, errTraegerRequired
	}
	bearer, err := h.chosenCostBearer(ctx, reviewer.OrganizationID, benefitsExemption != nil && *benefitsExemption, costBearerID, false)
	if err != nil {
		return parentbilling.HolidayContribution{}, err
	}
	registration, err := h.registrations.HolidayRegistration(ctx, reviewer, periodID, registrationID)
	if err != nil {
		return parentbilling.HolidayContribution{}, err
	}
	members, grouped, err := h.siblingsOf(ctx, reviewer, periodID, registration)
	if err != nil {
		return parentbilling.HolidayContribution{}, err
	}
	contribution, err := h.memberContribution(ctx, reviewer.OrganizationID, periodID, members, registration, benefitsExemption, bearer)
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
// whether an issued invoice bills it and whether that invoice billed
// another amount (#5662), or else its preview across its group
// with the exemption or the cost bearer the Träger-Büro is about to choose.
func (h *HolidayContributions) memberContribution(ctx context.Context, organizationID, periodID int64, members []siblingMember,
	registration parentbilling.HolidayRegistration, benefitsExemption *bool, bearer *parentbilling.CostBearer,
) (parentbilling.HolidayContribution, error) {
	for _, member := range members {
		if member.id() != registration.ID || member.frozen == nil {
			continue
		}
		billed, err := h.store.IssuedContributions(ctx, organizationID, parentbilling.ServiceHoliday, periodID)
		if err != nil {
			return parentbilling.HolidayContribution{}, fmt.Errorf("parent billing: read issued invoices: %w", err)
		}
		frozen := *member.frozen
		issued, invoiced := billed[registration.ID]
		frozen.Invoiced = invoiced
		frozen.CorrectionNeeded = invoiced && !sameBilled(issued, billedBy(frozen, issued.Children))
		return frozen, nil
	}
	exemption := registration.BenefitsDeclared && bearer == nil
	if benefitsExemption != nil {
		exemption = *benefitsExemption
	}
	computed, err := h.computeSiblings(ctx, organizationID, members, map[int64]holidayMemberChoice{
		registration.ID: {exemption: exemption, bearer: bearer},
	})
	if err != nil {
		return parentbilling.HolidayContribution{}, err
	}
	if contribution, ok := computed[registration.ID]; ok {
		return contribution, nil
	}
	// A cancelled registration ranks with no sibling.
	return h.compute(ctx, organizationID, registration.Children, exemption, bearer)
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
// asks for, a cost bearer together with the exemption, an unknown or a
// switched-off cost bearer refuse the confirmation before anything is
// written (#5627). The caller rolls the transaction back when this fails
// after the confirmation.
func (h *HolidayContributions) ConfirmHolidayRegistration(ctx context.Context, reviewer parentbilling.HolidayReviewer, periodID, registrationID int64,
	benefitsExemption bool, costBearerID int64, debtorNumber, familyChoice string, childNumbers []parentbilling.HolidayChildDebtorNumberInput,
) (parentbilling.HolidayConfirmation, error) {
	if reviewer.OrganizationID <= 0 || reviewer.AccountID <= 0 {
		return parentbilling.HolidayConfirmation{}, errTraegerRequired
	}
	if err := validFamilyChoice(familyChoice); err != nil {
		return parentbilling.HolidayConfirmation{}, err
	}
	debtorNumber, err := h.debtorNumberOf(ctx, reviewer.OrganizationID, debtorNumber)
	if err != nil {
		return parentbilling.HolidayConfirmation{}, err
	}
	bearer, err := h.chosenCostBearer(ctx, reviewer.OrganizationID, benefitsExemption, costBearerID, true)
	if err != nil {
		return parentbilling.HolidayConfirmation{}, err
	}
	others, issued, debtorNumber, err := h.confirmingSiblings(ctx, reviewer, periodID, registrationID, debtorNumber)
	if err != nil {
		return parentbilling.HolidayConfirmation{}, err
	}
	byChild, err := h.confirmsChildNumbers(ctx, reviewer.OrganizationID, debtorNumber, childNumbers)
	if err != nil {
		return parentbilling.HolidayConfirmation{}, err
	}
	registration, err := h.registrations.ConfirmHolidayRegistration(ctx, reviewer, periodID, registrationID, debtorNumber)
	if err != nil {
		return parentbilling.HolidayConfirmation{}, err
	}
	if registration.Status != parentbilling.HolidayRegistrationConfirmed {
		return parentbilling.HolidayConfirmation{}, parentbilling.ErrHolidayRegistrationStateConflict
	}
	choice := holidayMemberChoice{exemption: benefitsExemption, bearer: bearer}
	if err := h.freezeConfirmed(ctx, reviewer, periodID, registration, others, issued, choice); err != nil {
		return parentbilling.HolidayConfirmation{}, err
	}
	if err := h.confirmChildNumbers(ctx, reviewer, registration, byChild, childNumbers); err != nil {
		return parentbilling.HolidayConfirmation{}, err
	}
	// The family keeps the number of the registration and of the group that
	// took it (#6692); the caller rolls everything back on a conflict.
	number := registration.DebtorNumber
	if err := h.rememberFamilies(ctx, reviewer, slices.Concat([]parentbilling.HolidayRegistration{registration}, confirmedSharers(others, issued)),
		&number, familyChoice, registration.ID, others); err != nil {
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
// ranked across its group with the exemption or the cost bearer chosen for
// it (#5627), and gives the other confirmed registrations of
// the group their contribution again and the debtor number.
func (h *HolidayContributions) freezeConfirmed(ctx context.Context, reviewer parentbilling.HolidayReviewer, periodID int64,
	registration parentbilling.HolidayRegistration, others []siblingMember, issued map[int64]bool, choice holidayMemberChoice,
) error {
	members := slices.Concat(others, []siblingMember{{registration: registration}})
	computed, err := h.computeSiblings(ctx, reviewer.OrganizationID, members, map[int64]holidayMemberChoice{registration.ID: choice})
	if err != nil {
		return err
	}
	contribution := computed[registration.ID]
	if choice.exemption && !contribution.BenefitsExemption {
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
func (h *HolidayContributions) compute(ctx context.Context, organizationID int64, children []parentbilling.HolidayChild, exemption bool, bearer *parentbilling.CostBearer) (parentbilling.HolidayContribution, error) {
	rules, err := h.store.ListContributionRules(ctx, organizationID, parentbilling.ServiceHoliday)
	if err != nil {
		return parentbilling.HolidayContribution{}, fmt.Errorf("parent billing: read contribution rules: %w", err)
	}
	chain, err := h.chainSettings(ctx, organizationID)
	if err != nil {
		return parentbilling.HolidayContribution{}, err
	}
	valid := validHolidayRules(rules, firstHolidayDay(placedHolidayChildren(children)))
	valid.exemption = validHolidayRules(rules, firstHolidayDay(children)).exemption
	valid.chain = chain
	return computeHolidayContribution(children, valid, exemption, bearer), nil
}

// chainSettings reads how the Träger ranks siblings and when a cost bearer
// takes over (#5664). A value the chain does not know fails, it never falls
// back to another way of billing.
func (h *HolidayContributions) chainSettings(ctx context.Context, organizationID int64) (parentbilling.HolidayChainSettings, error) {
	chain, err := h.settings.HolidayChain(ctx, organizationID)
	if err != nil {
		return parentbilling.HolidayChainSettings{}, fmt.Errorf("parent billing: read chain settings: %w", err)
	}
	switch {
	case chain.SiblingRank != parentbilling.SiblingRankHighestContribution && chain.SiblingRank != parentbilling.SiblingRankOldestChild:
		return parentbilling.HolidayChainSettings{}, fmt.Errorf("parent billing: unknown sibling rank %q", chain.SiblingRank)
	case chain.CostTakeoverOrder != parentbilling.CostTakeoverAfterDiscount && chain.CostTakeoverOrder != parentbilling.CostTakeoverBeforeDiscount:
		return parentbilling.HolidayChainSettings{}, fmt.Errorf("parent billing: unknown cost takeover order %q", chain.CostTakeoverOrder)
	}
	return chain, nil
}

// chosenCostBearer reads the cost bearer the Träger-Büro chose (#5627):
// none for zero, otherwise an active cost bearer of the Träger, and never
// together with the exemption. A confirmation holds the cost bearer's row
// in a shared lock until it commits the lines that name it, so a parallel
// deletion waits and then refuses (DeleteCostBearer).
func (h *HolidayContributions) chosenCostBearer(ctx context.Context, organizationID int64, exemption bool, costBearerID int64, confirm bool) (*parentbilling.CostBearer, error) {
	switch {
	case costBearerID == 0:
		return nil, nil
	case exemption:
		return nil, parentbilling.ErrCostTakeoverWithExemption
	case costBearerID < 0:
		return nil, parentbilling.ErrCostBearerNotFound
	}
	read := h.store.CostBearer
	if confirm {
		read = func(ctx context.Context, organizationID, costBearerID int64) (parentbilling.CostBearer, error) {
			return h.store.LockCostBearer(ctx, organizationID, costBearerID, false)
		}
	}
	bearer, err := read(ctx, organizationID, costBearerID)
	if err != nil {
		return nil, err
	}
	if !bearer.Active {
		return nil, parentbilling.ErrCostBearerInactive
	}
	return &bearer, nil
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
		if child.AgeRank < 0 || child.AgeRank > len(children) {
			return nil, fmt.Errorf("%w: child %d has an age rank out of range", parentbilling.ErrInvalidHolidayQuote, i+1)
		}
		priced[i] = parentbilling.HolidayChild{Position: i + 1, AgeRank: child.AgeRank, Bookings: make([]parentbilling.HolidayBooking, 0, len(child.Bookings))}
		booked := map[int64]bool{}
		for _, booking := range child.Bookings {
			known, found := models[booking.TimeModelID]
			quoted, daysOffered := quotedBooking(known.week, known.model, booking.Days)
			if !found || known.week.ID != booking.WeekID || booked[booking.WeekID] || !daysOffered {
				return nil, fmt.Errorf("%w: child %d books a time model the period does not offer", parentbilling.ErrInvalidHolidayQuote, i+1)
			}
			booked[booking.WeekID] = true
			priced[i].Bookings = append(priced[i].Bookings, quoted)
		}
	}
	return priced, nil
}

// quotedBooking prices a quoted booking: a time model of a week by days
// prices one day (#4701).
func quotedBooking(week parentbilling.HolidayWeek, model parentbilling.HolidayTimeModel, chosen []calendar.Date) (parentbilling.HolidayBooking, bool) {
	days, ok := quotedDays(week, chosen)
	price := model.PriceCents
	if days > 0 {
		price *= int64(days)
	}
	return parentbilling.HolidayBooking{
		WeekID: week.ID, WeekStartsOn: week.StartsOn, WeekEndsOn: week.EndsOn,
		TimeModelID: model.ID, TimeModelName: model.Name, PriceCents: price, BookedDays: days, Days: chosen,
	}, ok
}

// quotedDays counts the distinct chosen days of a quoted booking (#4701):
// a week by days needs at least one of its care days and only those, a week
// booked as a whole none; zero is a whole week.
func quotedDays(week parentbilling.HolidayWeek, days []calendar.Date) (int, bool) {
	if len(week.CareDays) == 0 {
		return 0, len(days) == 0
	}
	chosen := map[calendar.Date]bool{}
	for _, day := range days {
		if !slices.Contains(week.CareDays, day) {
			return 0, false
		}
		chosen[day] = true
	}
	return len(chosen), len(chosen) > 0
}

// billedBy is what a contribution bills the family and the cost bearer. A
// full cost takeover keeps the family at nothing when the contribution
// changes, so the invoice is compared by both (review of #5671). With
// children only their part: the invoices per child bill those (#7007).
func billedBy(contribution parentbilling.HolidayContribution, children []int64) parentbilling.IssuedContribution {
	if len(children) == 0 {
		return billedOf(contribution.Children, contribution.TotalCents)
	}
	billed := make([]parentbilling.HolidayChildContribution, 0, len(children))
	var total int64
	for _, child := range contribution.Children {
		if slices.Contains(children, child.ChildID) {
			billed = append(billed, child)
			total += child.TotalCents
		}
	}
	return billedOf(billed, total)
}

// sameBilled compares what the invoices billed with what the contribution
// bills now.
func sameBilled(issued, now parentbilling.IssuedContribution) bool {
	return issued.TotalCents == now.TotalCents && issued.CostTakeoverCents == now.CostTakeoverCents
}

func billedOf(children []parentbilling.HolidayChildContribution, total int64) parentbilling.IssuedContribution {
	billed := parentbilling.IssuedContribution{TotalCents: total}
	for _, child := range children {
		for _, line := range child.Lines {
			if line.Kind == parentbilling.LineCostTakeover {
				billed.CostTakeoverCents += line.AmountCents
			}
		}
	}
	return billed
}
