package application

import (
	"context"
	"fmt"
	"reflect"
	"slices"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
)

// Siblings from separate holiday registrations (#5628, ADR 0115 addendum).
// The groups are the owner's own rows; the registrations of a group come
// from Enrollment through the port, which records every read in the
// registration's audit trail. A confirmed registration of a group ranks
// with the base prices it froze, a received one with its bookings at the
// current prices, a cancelled one not at all.

// HolidaySiblingStore keeps the sibling groups of a Träger.
type HolidaySiblingStore interface {
	// HolidaySiblingMembers lists the registrations of every group of the
	// period, by group and registration.
	HolidaySiblingMembers(ctx context.Context, organizationID, periodID int64) ([]parentbilling.HolidaySiblingMember, error)
	// ReplaceHolidaySiblingGroups deletes the replaced groups and creates
	// one group of the registrations.
	ReplaceHolidaySiblingGroups(ctx context.Context, organizationID, periodID, accountID int64, replaced, registrationIDs []int64) (int64, error)
	// RemoveHolidaySibling takes a registration out of its group; a group
	// left with one registration goes.
	RemoveHolidaySibling(ctx context.Context, organizationID, registrationID int64) error
}

var _ parentbilling.HolidaySiblingGroups = (*HolidayContributions)(nil)

// siblingMember is one registration of a sibling group with its newest
// frozen contribution, nil before the confirmation.
type siblingMember struct {
	registration parentbilling.HolidayRegistration
	frozen       *parentbilling.HolidayContribution
	// movedUp marks a confirmed registration whose bookings moved up from
	// the waiting list (#4699): it ranks with the base prices it froze and
	// the newly placed weeks at their current price.
	movedUp bool
}

func (m siblingMember) id() int64 { return m.registration.ID }

func (m siblingMember) cancelled() bool {
	return m.registration.Status == parentbilling.HolidayRegistrationCancelled
}

// holidayMemberChoice is what the Träger-Büro chooses for the registration
// it previews or confirms: the exemption or the cost bearer (#5627).
type holidayMemberChoice struct {
	exemption bool
	bearer    *parentbilling.CostBearer
}

// chainMember is the registration as the chain ranks it: a frozen one with
// the base prices, the exemption and the cost bearer it froze (#5627), so a
// later price or a renamed cost bearer changes nothing; else with its
// bookings and the parents' statement. The frozen lines hold no date of
// birth; it comes with the registration (#5664).
func (m siblingMember) chainMember() holidayMember {
	if m.frozen == nil {
		return holidayMember{children: m.registration.Children, exemption: m.registration.BenefitsDeclared}
	}
	registered := make(map[int64]parentbilling.HolidayChild, len(m.registration.Children))
	for _, child := range m.registration.Children {
		registered[child.ID] = child
	}
	children := make([]parentbilling.HolidayChild, len(m.frozen.Children))
	for i, child := range m.frozen.Children {
		children[i] = parentbilling.HolidayChild{ID: child.ChildID, Position: child.Position, BirthDate: registered[child.ChildID].BirthDate,
			Bookings: frozenHolidayBookings(child.Lines, registered[child.ChildID].Bookings)}
	}
	if m.movedUp {
		children = withMovedUpBookings(children, m.registration.Children)
	}
	member := holidayMember{children: children, exemption: m.frozen.BenefitsExemption}
	if m.frozen.CostBearerID != nil {
		member.bearer = &parentbilling.CostBearer{ID: *m.frozen.CostBearerID, Name: m.frozen.CostBearerName}
	}
	return member
}

// frozenHolidayBookings are the bookings the base lines of a frozen
// contribution price. A line of a week by days takes the days of the
// current booking of its week, so the rules see the first booked day
// (#4701).
func frozenHolidayBookings(lines []parentbilling.HolidayContributionLine, current []parentbilling.HolidayBooking) []parentbilling.HolidayBooking {
	var bookings []parentbilling.HolidayBooking
	for _, line := range lines {
		if line.Kind != parentbilling.LineBase || line.WeekStartsOn == nil || line.WeekEndsOn == nil {
			continue
		}
		booking := parentbilling.HolidayBooking{
			WeekID: line.WeekID, WeekStartsOn: *line.WeekStartsOn, WeekEndsOn: *line.WeekEndsOn, TimeModelName: line.TimeModelName, PriceCents: line.AmountCents,
		}
		if line.BookedDays != nil {
			booking.BookedDays = *line.BookedDays
			if i := slices.IndexFunc(current, func(b parentbilling.HolidayBooking) bool {
				return pricesHolidayWeek(b, booking.WeekID, booking.WeekStartsOn, booking.WeekEndsOn)
			}); i >= 0 {
				booking.Days = current[i].Days
			}
		}
		bookings = append(bookings, booking)
	}
	return bookings
}

// ListHolidaySiblingGroups lists the groups of the period. They name only
// registrations, no personal data.
func (h *HolidayContributions) ListHolidaySiblingGroups(ctx context.Context, reviewer parentbilling.HolidayReviewer, periodID int64) ([]parentbilling.HolidaySiblingGroup, error) {
	if reviewer.OrganizationID <= 0 || reviewer.AccountID <= 0 {
		return nil, errTraegerRequired
	}
	return h.siblingGroups(ctx, reviewer.OrganizationID, periodID)
}

// siblingGroups reads the groups of the period: one per group, with its
// registrations by number.
func (h *HolidayContributions) siblingGroups(ctx context.Context, organizationID, periodID int64) ([]parentbilling.HolidaySiblingGroup, error) {
	members, err := h.store.HolidaySiblingMembers(ctx, organizationID, periodID)
	if err != nil {
		return nil, fmt.Errorf("parent billing: read sibling groups: %w", err)
	}
	groups := []parentbilling.HolidaySiblingGroup{}
	for _, member := range members {
		if len(groups) == 0 || groups[len(groups)-1].ID != member.GroupID {
			groups = append(groups, parentbilling.HolidaySiblingGroup{ID: member.GroupID, RegistrationIDs: []int64{}})
		}
		last := &groups[len(groups)-1]
		last.RegistrationIDs = append(last.RegistrationIDs, member.RegistrationID)
	}
	return groups, nil
}

// MergeHolidaySiblings merges the registrations and the groups they belong
// to into one group, under the billing lock of the period, so no release
// runs in between. It computes the frozen contribution of every confirmed
// registration of the group again, gives the group one debtor number and
// records the merge in the audit trail of each registration.
func (h *HolidayContributions) MergeHolidaySiblings(ctx context.Context, reviewer parentbilling.HolidayReviewer, periodID int64, registrationIDs []int64) (parentbilling.HolidaySiblingGroup, error) {
	if reviewer.OrganizationID <= 0 || reviewer.AccountID <= 0 {
		return parentbilling.HolidaySiblingGroup{}, errTraegerRequired
	}
	selected := distinctIDs(registrationIDs)
	if len(selected) < 2 {
		return parentbilling.HolidaySiblingGroup{}, parentbilling.ErrInvalidHolidaySiblingGroup
	}
	groups, issued, err := h.lockSiblings(ctx, reviewer, periodID)
	if err != nil {
		return parentbilling.HolidaySiblingGroup{}, err
	}
	merged, replaced := mergedGroup(groups, selected)
	if err := refuseInvoiced(merged, issued); err != nil {
		return parentbilling.HolidaySiblingGroup{}, err
	}
	members, err := h.readSiblings(ctx, reviewer, periodID, merged, nil)
	if err != nil {
		return parentbilling.HolidaySiblingGroup{}, err
	}
	for _, member := range members {
		if member.cancelled() && slices.Contains(selected, member.id()) {
			return parentbilling.HolidaySiblingGroup{}, fmt.Errorf("%w: registration %d is cancelled", parentbilling.ErrHolidayRegistrationStateConflict, member.id())
		}
	}
	groupID, err := h.store.ReplaceHolidaySiblingGroups(ctx, reviewer.OrganizationID, periodID, reviewer.AccountID, replaced, merged)
	if err != nil {
		return parentbilling.HolidaySiblingGroup{}, fmt.Errorf("parent billing: merge siblings: %w", err)
	}
	if err := h.refreezeSiblings(ctx, reviewer, periodID, members, issued); err != nil {
		return parentbilling.HolidaySiblingGroup{}, err
	}
	number := groupDebtorNumber(members, 0)
	if err := h.shareDebtorNumber(ctx, reviewer, periodID, members, number, issued); err != nil {
		return parentbilling.HolidaySiblingGroup{}, err
	}
	// A merge adopts missing family numbers without replacing remembered ones.
	if err := h.rememberFamilies(ctx, reviewer, confirmedSharers(members, issued), &number, parentbilling.FamilyDebtorNumberKeep, 0); err != nil {
		return parentbilling.HolidaySiblingGroup{}, err
	}
	if err := h.recordSiblings(ctx, reviewer, periodID, members, parentbilling.HolidaySiblingsMerged); err != nil {
		return parentbilling.HolidaySiblingGroup{}, err
	}
	return parentbilling.HolidaySiblingGroup{ID: groupID, RegistrationIDs: merged}, nil
}

// SplitHolidaySibling takes the registration out of its group under the
// billing lock of the period and computes the frozen contributions of the
// registration and of the rest of the group again.
func (h *HolidayContributions) SplitHolidaySibling(ctx context.Context, reviewer parentbilling.HolidayReviewer, periodID, registrationID int64) error {
	if reviewer.OrganizationID <= 0 || reviewer.AccountID <= 0 {
		return errTraegerRequired
	}
	groups, issued, err := h.lockSiblings(ctx, reviewer, periodID)
	if err != nil {
		return err
	}
	group, found := groupOf(groups, registrationID)
	if !found {
		return parentbilling.ErrHolidaySiblingGroupNotFound
	}
	if err := refuseInvoiced(group.RegistrationIDs, issued); err != nil {
		return err
	}
	members, err := h.readSiblings(ctx, reviewer, periodID, group.RegistrationIDs, nil)
	if err != nil {
		return err
	}
	if err := h.store.RemoveHolidaySibling(ctx, reviewer.OrganizationID, registrationID); err != nil {
		return fmt.Errorf("parent billing: split siblings: %w", err)
	}
	left := slices.DeleteFunc(slices.Clone(members), func(member siblingMember) bool { return member.id() != registrationID })
	rest := slices.DeleteFunc(slices.Clone(members), func(member siblingMember) bool { return member.id() == registrationID })
	for _, part := range [][]siblingMember{left, rest} {
		if err := h.refreezeSiblings(ctx, reviewer, periodID, part, issued); err != nil {
			return err
		}
	}
	if err := h.recordSiblings(ctx, reviewer, periodID, left, parentbilling.HolidaySiblingsSplit, memberIDs(rest)...); err != nil {
		return err
	}
	return h.recordSiblings(ctx, reviewer, periodID, rest, parentbilling.HolidaySiblingsSplit, registrationID)
}

// lockSiblings holds the invoices of the period, then the period in
// Enrollment, before it reads anything: no release, cancellation or
// rebooking changes the group between the read and the decision. Then it
// reads the groups and the registrations an issued invoice bills. The order
// is the one of a change of the debtor number: invoices, then Enrollment.
func (h *HolidayContributions) lockSiblings(ctx context.Context, reviewer parentbilling.HolidayReviewer, periodID int64) ([]parentbilling.HolidaySiblingGroup, map[int64]bool, error) {
	organizationID := reviewer.OrganizationID
	if err := h.store.LockPeriodBilling(ctx, organizationID, parentbilling.ServiceHoliday, periodID); err != nil {
		return nil, nil, fmt.Errorf("parent billing: hold the invoices of the period: %w", err)
	}
	if err := h.registrations.HoldHolidayPeriod(ctx, reviewer, periodID); err != nil {
		return nil, nil, err
	}
	groups, err := h.siblingGroups(ctx, organizationID, periodID)
	if err != nil {
		return nil, nil, err
	}
	issued, err := h.store.IssuedRegistrations(ctx, organizationID, parentbilling.ServiceHoliday, periodID)
	if err != nil {
		return nil, nil, fmt.Errorf("parent billing: read issued invoices: %w", err)
	}
	return groups, issued, nil
}

// siblingsOf reads the group of a registration that Enrollment already
// read, with its frozen contributions; grouped is false without a group,
// and the registration is then the only member.
func (h *HolidayContributions) siblingsOf(ctx context.Context, reviewer parentbilling.HolidayReviewer, periodID int64, registration parentbilling.HolidayRegistration) ([]siblingMember, bool, error) {
	groups, err := h.siblingGroups(ctx, reviewer.OrganizationID, periodID)
	if err != nil {
		return nil, false, err
	}
	ids := []int64{registration.ID}
	group, grouped := groupOf(groups, registration.ID)
	if grouped {
		ids = group.RegistrationIDs
	}
	members, err := h.readSiblings(ctx, reviewer, periodID, ids, &registration)
	return members, grouped, err
}

// readSiblings reads the registrations through Enrollment, except the one
// already read, with their newest frozen contributions, in the order they
// were sent.
func (h *HolidayContributions) readSiblings(ctx context.Context, reviewer parentbilling.HolidayReviewer, periodID int64, ids []int64, known *parentbilling.HolidayRegistration) ([]siblingMember, error) {
	members := make([]siblingMember, 0, len(ids))
	for _, id := range ids {
		var registration parentbilling.HolidayRegistration
		if known != nil && known.ID == id {
			registration = *known
		} else {
			read, err := h.registrations.HolidayRegistration(ctx, reviewer, periodID, id)
			if err != nil {
				return nil, err
			}
			registration = read
		}
		member := siblingMember{registration: registration}
		frozen, found, err := h.store.HolidayContributions(ctx, reviewer.OrganizationID, id)
		if err != nil {
			return nil, fmt.Errorf("parent billing: read holiday contribution: %w", err)
		}
		if found {
			member.frozen = &frozen
		}
		members = append(members, member)
	}
	return sentOrder(members), nil
}

// sentOrder sorts the registrations of a group in the order they were sent,
// then by number: the tie-break of the sibling rule. Every calculation of a
// group runs in this order, so a confirmation freezes what the preview
// showed, whichever registration it confirms.
func sentOrder(members []siblingMember) []siblingMember {
	slices.SortStableFunc(members, func(a, b siblingMember) int {
		if order := a.registration.SubmittedAt.Compare(b.registration.SubmittedAt); order != 0 {
			return order
		}
		switch {
		case a.id() < b.id():
			return -1
		case a.id() > b.id():
			return 1
		}
		return 0
	})
	return members
}

// computeSiblings runs the chain over the registrations of a group that
// are not cancelled, in the order they were sent, with the rules valid on
// the first booked day of the group and the choices groupChain names. chosen
// overrides the exemption and the cost bearer of one registration: the one
// the Träger-Büro is about to confirm or preview (#5627).
func (h *HolidayContributions) computeSiblings(ctx context.Context, organizationID int64, members []siblingMember, chosen map[int64]holidayMemberChoice) (map[int64]parentbilling.HolidayContribution, error) {
	var ranked []siblingMember
	var chain []holidayMember
	var children []parentbilling.HolidayChild
	var requested []parentbilling.HolidayChild
	for _, member := range sentOrder(slices.Clone(members)) {
		if member.cancelled() {
			continue
		}
		next := member.chainMember()
		if choice, ok := chosen[member.id()]; ok {
			next.exemption, next.bearer = choice.exemption, choice.bearer
		}
		ranked = append(ranked, member)
		chain = append(chain, next)
		children = append(children, next.children...)
		requested = append(requested, member.registration.Children...)
	}
	rules, err := h.store.ListContributionRules(ctx, organizationID, parentbilling.ServiceHoliday)
	if err != nil {
		return nil, fmt.Errorf("parent billing: read contribution rules: %w", err)
	}
	current, err := h.chainSettings(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	valid := validHolidayRules(rules, firstHolidayDay(placedHolidayChildren(children)))
	valid.exemption = validHolidayRules(rules, firstHolidayDay(requested)).exemption
	valid.chain = groupChain(ranked, current)
	results := computeHolidaySiblings(chain, valid)
	computed := make(map[int64]parentbilling.HolidayContribution, len(ranked))
	for i, member := range ranked {
		computed[member.id()] = results[i]
	}
	return computed, nil
}

// groupChain names the choices a group computes with (#5664). A group with a
// frozen contribution keeps the choices it froze, so a later change of the
// settings changes no frozen contribution, also when the group is computed
// again and when a further registration of it is confirmed. Only registrations
// frozen under different choices, merged after the change, take the current
// ones.
func groupChain(members []siblingMember, current parentbilling.HolidayChainSettings) parentbilling.HolidayChainSettings {
	var frozen *parentbilling.HolidayChainSettings
	for _, member := range members {
		switch {
		case member.frozen == nil:
		case frozen == nil:
			frozen = &member.frozen.Chain
		case *frozen != member.frozen.Chain:
			return current
		}
	}
	if frozen == nil {
		return current
	}
	return *frozen
}

// refreezeSiblings computes the group again and stores a new version of the
// frozen contribution of every confirmed registration whose contribution
// changes. A registration an issued invoice bills keeps its own. An
// exemption it froze stays only while a rule allows it.
func (h *HolidayContributions) refreezeSiblings(ctx context.Context, reviewer parentbilling.HolidayReviewer, periodID int64, members []siblingMember, issued map[int64]bool) error {
	return h.refreeze(ctx, reviewer, periodID, members, issued, false)
}

// refreeze is refreezeSiblings; with keepExemptions a registration whose
// frozen exemption no rule allows any more keeps its contribution instead
// of refusing: an exempt family pays nothing either way (#5662).
func (h *HolidayContributions) refreeze(ctx context.Context, reviewer parentbilling.HolidayReviewer, periodID int64, members []siblingMember, issued map[int64]bool, keepExemptions bool) error {
	if len(members) == 0 {
		return nil
	}
	computed, err := h.computeSiblings(ctx, reviewer.OrganizationID, members, nil)
	if err != nil {
		return err
	}
	for _, member := range members {
		next, ok := computed[member.id()]
		if !ok || member.frozen == nil || issued[member.id()] ||
			member.registration.Status != parentbilling.HolidayRegistrationConfirmed {
			continue
		}
		changed, err := refrozen(*member.frozen, next, keepExemptions)
		if err != nil {
			return err
		}
		if !changed {
			continue
		}
		if err := h.store.InsertHolidayContributions(ctx, reviewer.OrganizationID, periodID, member.id(), reviewer.AccountID, next); err != nil {
			return fmt.Errorf("parent billing: freeze holiday contribution again: %w", err)
		}
	}
	return nil
}

// refrozen reports whether next is a new version of the frozen
// contribution: other amounts or other chain settings (#5664). A frozen exemption no rule allows any more refuses, or with
// keepExemptions keeps the frozen contribution.
func refrozen(frozen, next parentbilling.HolidayContribution, keepExemptions bool) (bool, error) {
	if frozen.BenefitsExemption && !next.BenefitsExemption {
		if keepExemptions {
			return false, nil
		}
		return false, parentbilling.ErrBenefitsExemptionUnavailable
	}
	return !sameContribution(frozen, next) || frozen.Chain != next.Chain, nil
}

// shareDebtorNumber gives every confirmed registration of the group that no
// issued invoice bills the debtor number of the group (#5621): all
// registrations of a family pay with one reference. An empty number removes
// it from them, so no registration keeps or passes on a removed number.
func (h *HolidayContributions) shareDebtorNumber(ctx context.Context, reviewer parentbilling.HolidayReviewer, periodID int64, members []siblingMember, number string, issued map[int64]bool) error {
	for _, member := range members {
		if member.registration.Status != parentbilling.HolidayRegistrationConfirmed || issued[member.id()] ||
			member.registration.DebtorNumber == number {
			continue
		}
		if _, err := h.registrations.SetHolidayDebtorNumber(ctx, reviewer, periodID, member.id(), number); err != nil {
			return err
		}
	}
	return nil
}

// recordSiblings records the change in the audit trail of every member,
// with the other registrations it concerns: the rest of the group, or
// those named.
func (h *HolidayContributions) recordSiblings(ctx context.Context, reviewer parentbilling.HolidayReviewer, periodID int64, members []siblingMember, action string, others ...int64) error {
	for _, member := range members {
		siblings := others
		if len(siblings) == 0 {
			siblings = slices.DeleteFunc(memberIDs(members), func(id int64) bool { return id == member.id() })
		}
		if err := h.registrations.RecordHolidaySiblings(ctx, reviewer, periodID, member.id(), action, siblings); err != nil {
			return err
		}
	}
	return nil
}

// siblingView names the other registrations of the group and their
// children for the registration's page.
func siblingView(members []siblingMember, registrationID int64) []parentbilling.HolidaySibling {
	siblings := []parentbilling.HolidaySibling{}
	for _, member := range members {
		if member.id() == registrationID {
			continue
		}
		sibling := parentbilling.HolidaySibling{
			RegistrationID: member.id(), Status: member.registration.Status,
			Children: make([]parentbilling.HolidaySiblingChild, len(member.registration.Children)),
		}
		for i, child := range member.registration.Children {
			sibling.Children[i] = parentbilling.HolidaySiblingChild{ID: child.ID, FirstName: child.FirstName, LastName: child.LastName}
		}
		siblings = append(siblings, sibling)
	}
	return siblings
}

// groupDebtorNumber is the debtor number of the first registration of the
// group that has one, except the one named. A cancelled registration keeps
// the number it had, which the group may since have changed or removed, so
// it passes on none.
func groupDebtorNumber(members []siblingMember, except int64) string {
	for _, member := range members {
		if member.id() != except && !member.cancelled() && member.registration.DebtorNumber != "" {
			return member.registration.DebtorNumber
		}
	}
	return ""
}

// mergedGroup is every registration of the selection and of the groups it
// touches, by number, and the groups it replaces.
func mergedGroup(groups []parentbilling.HolidaySiblingGroup, selected []int64) ([]int64, []int64) {
	merged := slices.Clone(selected)
	var replaced []int64
	for _, group := range groups {
		if !slices.ContainsFunc(group.RegistrationIDs, func(id int64) bool { return slices.Contains(selected, id) }) {
			continue
		}
		replaced = append(replaced, group.ID)
		merged = append(merged, group.RegistrationIDs...)
	}
	return distinctIDs(merged), replaced
}

func groupOf(groups []parentbilling.HolidaySiblingGroup, registrationID int64) (parentbilling.HolidaySiblingGroup, bool) {
	for _, group := range groups {
		if slices.Contains(group.RegistrationIDs, registrationID) {
			return group, true
		}
	}
	return parentbilling.HolidaySiblingGroup{}, false
}

func refuseInvoiced(ids []int64, issued map[int64]bool) error {
	for _, id := range ids {
		if issued[id] {
			return fmt.Errorf("%w: registration %d", parentbilling.ErrHolidaySiblingsInvoiced, id)
		}
	}
	return nil
}

// distinctIDs keeps the positive IDs once, in ascending order; a missing
// registration is Enrollment's not found.
func distinctIDs(ids []int64) []int64 {
	kept := slices.DeleteFunc(slices.Clone(ids), func(id int64) bool { return id <= 0 })
	slices.Sort(kept)
	return slices.Compact(kept)
}

func memberIDs(members []siblingMember) []int64 {
	ids := make([]int64, len(members))
	for i, member := range members {
		ids[i] = member.id()
	}
	return ids
}

// sameContribution reports whether two contributions bill the same lines.
func sameContribution(a, b parentbilling.HolidayContribution) bool {
	return slices.EqualFunc(a.Children, b.Children, func(x, y parentbilling.HolidayChildContribution) bool {
		return x.ChildID == y.ChildID && x.TotalCents == y.TotalCents && reflect.DeepEqual(x.Lines, y.Lines)
	})
}
