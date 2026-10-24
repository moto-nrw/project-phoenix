package departure

import "strings"

// dayRuleLabels phrase the departure modes so they read naturally after a
// pickup time in a weekday cell ("14:30 Uhr, wird abgeholt").
var dayRuleLabels = map[DepartureMode]string{
	DepartureAlone:       "geht alleine",
	DepartureBus:         "fährt Bus",
	DeparturePickup:      "wird abgeholt",
	DepartureAccompanied: "mit anderem Kind",
}

// DayRules renders the departure plan plus the "mit wem" detail, keyed by
// weekday code, for the weekday cells of both class lists (#2254, #4749).
// Every weekday gets an entry; a day without an explicit rule carries the
// business default "geht alleine".
//
// Both "mit wem" sources travel: a child whose Laufgemeinschaft answers
// "mit wem" needs no free-text note (the note requirement is satisfied per
// weekday by a link), so cells built from the note alone would print
// "mit anderem Kind" with no name on the sheet staff carry to the door.
//
// The links are printed only on the weekdays the rendered plan actually allows
// "Anderes Kind". They are the CURRENT links of the live child, while the plan
// may come from an approved enrollment phase — a plan that says Monday
// accompanied and Tuesday bus, next to a link the child gained on Tuesday
// since, would contradict itself in the Tuesday cell. Filtering is the honest
// reading: a day the printed plan does not allow has no "mit wem" to print.
// The free-text note answers the accompanied days no link covers — a day a
// link already names does not repeat the note.
func DayRules(allowed AllowedDepartureModes, fallback DepartureDays, companionNote string, companions []CompanionLink) map[string]string {
	allowed = allowed.Normalize()
	if !allowed.HasAny() {
		allowed = AllowedDepartureModesFromDeparture(fallback)
	}
	note := strings.TrimSpace(companionNote)
	out := make(map[string]string, len(PickupDayOrder))
	for _, day := range PickupDayOrder {
		out[day] = dayRule(allowed[day], day, note, companions)
	}
	return out
}

// dayRule renders one weekday's modes; an accompanied day names its
// companions of that day, or the free-text note when none does.
func dayRule(modes []DepartureMode, day, note string, companions []CompanionLink) string {
	if len(modes) == 0 {
		return dayRuleLabels[DepartureAlone]
	}
	labels := make([]string, 0, len(modes))
	accompanied := false
	for _, mode := range modes {
		if mode == DepartureAccompanied {
			accompanied = true
		}
		labels = append(labels, dayRuleLabels[mode])
	}
	cell := strings.Join(labels, " / ")
	if !accompanied {
		return cell
	}
	onDay := FilterCompanionLinksToDays(companions, map[string]bool{day: true})
	names := make([]string, 0, len(onDay))
	for _, link := range onDay {
		names = append(names, CompanionDisplayName(link))
	}
	detail := strings.Join(names, ", ")
	if detail == "" {
		detail = note
	}
	if detail != "" {
		cell += " (mit: " + detail + ")"
	}
	return cell
}
