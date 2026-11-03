package application

import (
	"slices"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The rule chain of the holiday contribution (ADR 0088 section 4, ADR 0115):
// base price, sibling rule, exemption. It is pure; the caller reads the
// rules and the bookings and decides whether the exemption applies.

// holidayRules are the rules of one Träger valid on one day.
type holidayRules struct {
	sibling   *parentbilling.ContributionRule
	exemption *parentbilling.ContributionRule
}

// validHolidayRules picks the holiday rules valid on day. Rules of one kind
// never overlap, so at most one of each kind is valid.
func validHolidayRules(rules []parentbilling.ContributionRule, day calendar.Date) holidayRules {
	var valid holidayRules
	for i := range rules {
		rule := &rules[i]
		if rule.Service != parentbilling.ServiceHoliday || day.Before(rule.ValidFrom) || (rule.ValidUntil != nil && day.After(*rule.ValidUntil)) {
			continue
		}
		switch rule.Kind {
		case parentbilling.RuleSibling:
			valid.sibling = rule
		case parentbilling.RuleBenefitsExemption:
			valid.exemption = rule
		}
	}
	return valid
}

// firstHolidayDay is the first booked day of the children, the day whose
// rules apply. Without a booking it is the zero date and no rule applies.
func firstHolidayDay(children []parentbilling.HolidayChild) calendar.Date {
	var first calendar.Date
	for _, child := range children {
		for _, booking := range child.Bookings {
			if first == "" || booking.WeekStartsOn.Before(first) {
				first = booking.WeekStartsOn
			}
		}
	}
	return first
}

// computeHolidayContribution runs the chain over the children of one
// registration. exemption applies the exemption rule when there is one.
func computeHolidayContribution(children []parentbilling.HolidayChild, rules holidayRules, exemption bool) parentbilling.HolidayContribution {
	return computeHolidaySiblings([]holidayMember{{children: children, exemption: exemption}}, rules)[0]
}

// holidayMember is one registration of a sibling group (#5628) as the chain
// sees it: its children and whether its exemption applies.
type holidayMember struct {
	children  []parentbilling.HolidayChild
	exemption bool
}

// computeHolidaySiblings runs the chain over the registrations of a sibling
// group, in the order they were sent, and returns one contribution per
// registration. The sibling rule ranks the children of all of them
// together; the exemption stays a matter of each registration.
func computeHolidaySiblings(members []holidayMember, rules holidayRules) []parentbilling.HolidayContribution {
	results := make([]parentbilling.HolidayContribution, len(members))
	var ranked []*parentbilling.HolidayChildContribution
	for m, member := range members {
		results[m] = parentbilling.HolidayContribution{
			Children:           make([]parentbilling.HolidayChildContribution, len(member.children)),
			ExemptionAvailable: rules.exemption != nil,
			BenefitsExemption:  member.exemption && rules.exemption != nil,
		}
		for i, child := range member.children {
			results[m].Children[i] = holidayBaseLines(child)
		}
		ranked = append(ranked, formOrder(results[m].Children)...)
	}
	applySiblingRule(ranked, rules.sibling)
	for m := range results {
		result := &results[m]
		for i := range result.Children {
			if result.BenefitsExemption {
				applyExemption(&result.Children[i], rules.exemption)
			}
			result.TotalCents += result.Children[i].TotalCents
		}
	}
	return results
}

// formOrder points to the children of one registration in the order of the
// form.
func formOrder(children []parentbilling.HolidayChildContribution) []*parentbilling.HolidayChildContribution {
	ordered := make([]*parentbilling.HolidayChildContribution, len(children))
	for i := range children {
		ordered[i] = &children[i]
	}
	slices.SortStableFunc(ordered, func(a, b *parentbilling.HolidayChildContribution) int { return a.Position - b.Position })
	return ordered
}

// holidayBaseLines writes one base line per booked week, in the order of
// the weeks.
func holidayBaseLines(child parentbilling.HolidayChild) parentbilling.HolidayChildContribution {
	bookings := slices.Clone(child.Bookings)
	slices.SortStableFunc(bookings, func(a, b parentbilling.HolidayBooking) int { return a.WeekStartsOn.Compare(b.WeekStartsOn) })
	contribution := parentbilling.HolidayChildContribution{
		ChildID: child.ID, Position: child.Position, Lines: make([]parentbilling.HolidayContributionLine, 0, len(bookings)+2),
	}
	for _, booking := range bookings {
		startsOn, endsOn := booking.WeekStartsOn, booking.WeekEndsOn
		contribution.Lines = append(contribution.Lines, parentbilling.HolidayContributionLine{
			Kind: parentbilling.LineBase, AmountCents: booking.PriceCents,
			WeekStartsOn: &startsOn, WeekEndsOn: &endsOn, TimeModelName: booking.TimeModelName,
		})
		contribution.TotalCents += booking.PriceCents
	}
	return contribution
}

// applySiblingRule lowers the contribution of every child from the rule's
// rank on. children come in the order of the registrations, then of the
// form. Children with a booking count; the one with the highest base price
// is the first child, equal prices keep the given order, so the discount
// never lands on the cheaper child by chance.
func applySiblingRule(children []*parentbilling.HolidayChildContribution, rule *parentbilling.ContributionRule) {
	if rule == nil || rule.SiblingFromChild == nil {
		return
	}
	ranked := make([]*parentbilling.HolidayChildContribution, 0, len(children))
	for _, child := range children {
		if len(child.Lines) > 0 {
			ranked = append(ranked, child)
		}
	}
	slices.SortStableFunc(ranked, func(a, b *parentbilling.HolidayChildContribution) int {
		switch {
		case a.TotalCents > b.TotalCents:
			return -1
		case a.TotalCents < b.TotalCents:
			return 1
		}
		return 0
	})
	for index, child := range ranked {
		rank := index + 1
		if rank < *rule.SiblingFromChild {
			continue
		}
		discount := siblingDiscount(child.TotalCents, rule)
		if discount == 0 {
			continue
		}
		child.Lines = append(child.Lines, parentbilling.HolidayContributionLine{
			Kind: parentbilling.LineSibling, AmountCents: -discount, ChildRank: &rank,
			DiscountBasisPoints: rule.DiscountBasisPoints, DiscountCents: rule.DiscountCents,
		})
		child.TotalCents -= discount
	}
}

// siblingDiscount is the discount of the rule on base, rounded half up to
// whole cent and never more than base.
func siblingDiscount(base int64, rule *parentbilling.ContributionRule) int64 {
	if base <= 0 {
		return 0
	}
	var discount int64
	switch {
	case rule.DiscountBasisPoints != nil:
		discount = (base*int64(*rule.DiscountBasisPoints) + parentbilling.BasisPointsWhole/2) / parentbilling.BasisPointsWhole
	case rule.DiscountCents != nil:
		discount = *rule.DiscountCents
	}
	return min(discount, base)
}

// applyExemption frees the child from what is left of its contribution and
// writes the rule's note, even when nothing is left.
func applyExemption(child *parentbilling.HolidayChildContribution, rule *parentbilling.ContributionRule) {
	if len(child.Lines) == 0 {
		return
	}
	child.Lines = append(child.Lines, parentbilling.HolidayContributionLine{
		Kind: parentbilling.LineBenefitsExemption, AmountCents: -child.TotalCents, Note: rule.Note,
	})
	child.TotalCents = 0
}
