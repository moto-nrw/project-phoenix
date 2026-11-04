package timetable

import "slices"

// An offering carries a Regeltermin by the rule of #5342 (ADR 0128): its
// kind fits the Regeltermin's (RosterOfferingKind) and the Regeltermin ends
// no later than the offering's pickup time that day; the booked care runs
// until the child is picked up. The suggestion applies it in two ways:
//
//   - An offering picked up before the end on a weekday it books never
//     explains the roster, even when it would cover a planned child.
//   - For a Klasse or Jahrgang, every offering of the suggested phase that
//     carries the Regeltermin on every weekday it books joins the
//     suggestion, also when today's children need it for nobody: in a
//     class Randstunde, every child booking Ganztag bis 16 Uhr may also
//     book the Randstunde today, but a later child booking only that
//     Ganztag belongs on the list too. Lists of the same kind get the same
//     suggestion.
//
// Without a known end or pickup time the rule cannot decide; the offering
// then joins only when it explains the roster (#5139).

type offeringCarriage int

const (
	carriageUnknown offeringCarriage = iota
	carriageProven
	carriageRuledOut
)

// carriage decides the rule over the Regeltermin's weekdays the candidate
// books; the kind is already checked when the bookings are grouped.
func (candidate deviationCandidate) carriage(roster TemplateFutureRoster) offeringCarriage {
	result := carriageProven
	for _, weekday := range candidateWeekdays(candidate) {
		end, endKnown := ParseOfferingGapClock(roster.Ends[weekday])
		pickup, pickupKnown := ParseOfferingGapClock(candidate.pickupTimes[weekday])
		switch {
		case !endKnown || !pickupKnown:
			result = carriageUnknown
		case pickup < end:
			return carriageRuledOut
		}
	}
	return result
}

func candidateWeekdays(candidate deviationCandidate) []int {
	weekdays := make([]int, 0, len(candidate.booked))
	for slot := range candidate.booked {
		if !slices.Contains(weekdays, slot.weekday) {
			weekdays = append(weekdays, slot.weekday)
		}
	}
	return weekdays
}

// withCarryingOfferings adds to each best combination the offerings of its
// phase that carry the Regeltermin. Combinations that become equal merge,
// so a tie between Ganztag variants that both carry it resolves.
func withCarryingOfferings(roster TemplateFutureRoster, candidates []deviationCandidate, best []sourceCombination) []sourceCombination {
	if len(roster.TargetSchoolClasses) == 0 && len(roster.TargetGradeLevels) == 0 {
		return best
	}
	extended := make([]sourceCombination, 0, len(best))
	for _, combination := range best {
		members := slices.Clone(combination.members)
		phaseID := candidates[members[0]].phaseID
		for index, candidate := range candidates {
			if candidate.phaseID == phaseID && !slices.Contains(members, index) && candidate.carriage(roster) == carriageProven {
				members = append(members, index)
			}
		}
		slices.Sort(members)
		combination.members = members
		if !slices.ContainsFunc(extended, func(other sourceCombination) bool { return slices.Equal(other.members, members) }) {
			extended = append(extended, combination)
		}
	}
	return extended
}
