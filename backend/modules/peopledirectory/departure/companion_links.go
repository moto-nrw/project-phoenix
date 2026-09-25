package departure

import (
	"strconv"
	"strings"
)

// CompanionWeekdayShortLabels are the German two-letter weekday labels the
// offline lists (Tagesliste, Wochenliste, Klassenliste) already use for the
// departure plan. Kept next to the weekday order so a companion detail never
// renders a different abbreviation than the plan it belongs to.
var CompanionWeekdayShortLabels = map[string]string{
	PickupDayMonday:    "Mo",
	PickupDayTuesday:   "Di",
	PickupDayWednesday: "Mi",
	PickupDayThursday:  "Do",
	PickupDayFriday:    "Fr",
}

// CompanionLink is the per-child view of the companion edges ("läuft mit",
// Laufgemeinschaft): one companion plus every weekday they walk together.
// This is the shape the child detail view edits and the API speaks; it is not
// persisted.
type CompanionLink struct {
	CompanionStudentID int64    `json:"companion_student_id"`
	FirstName          string   `json:"first_name,omitempty"`
	LastName           string   `json:"last_name,omitempty"`
	Weekdays           []string `json:"weekdays"`
}

// FilterCompanionLinksToDays keeps only the weekdays the given set allows and
// drops a link that has none left. Pure derivation: it writes nothing and never
// widens.
//
// It exists for readers that render a companion list NEXT TO a departure plan
// the links were not reconciled against — the class roster prints a plan taken
// from the approved enrollment phase, while the links belong to the live child
// and follow every later Stammdaten edit. Printing them unfiltered puts "Di:
// Bus" and "läuft dienstags mit Mia" on the same line of the sheet staff carry
// to the door. The update path has no use for this: there the links are trimmed
// against the plan being written (TrimCompanionsToDays), which also refuses to
// strand the child at the far end.
func FilterCompanionLinksToDays(links []CompanionLink, allowedDays map[string]bool) []CompanionLink {
	filtered := make([]CompanionLink, 0, len(links))
	for _, link := range links {
		kept := make([]string, 0, len(link.Weekdays))
		for _, day := range link.Weekdays {
			if allowedDays[day] {
				kept = append(kept, day)
			}
		}
		if len(kept) == 0 {
			continue
		}
		copied := link
		copied.Weekdays = kept
		filtered = append(filtered, copied)
	}
	return filtered
}

// CompanionDisplayName is the companion's full name, falling back to the id for
// a link whose names were not joined in.
func CompanionDisplayName(link CompanionLink) string {
	name := strings.TrimSpace(link.FirstName + " " + link.LastName)
	if name == "" {
		return "Kind #" + strconv.FormatInt(link.CompanionStudentID, 10)
	}
	return name
}

// FormatCompanionLinks renders the structured "läuft mit" links for the offline
// lists: "Mia Schulz (Mo, Di), Tom Meier".
//
// Every export that prints an accompanied departure needs it. A child whose
// "mit wem" is answered by links may legitimately have NO free-text note (the
// note is only required for a day no link covers), so a list built from the
// note alone would tell staff "Mit anderem Kind" and nothing else — on the one
// sheet they use when the app is not at hand. The weekdays are named unless the
// link covers all five, so a Monday-only Laufgemeinschaft cannot be read as a
// standing arrangement.
func FormatCompanionLinks(links []CompanionLink) string {
	parts := make([]string, 0, len(links))
	for _, link := range links {
		requested := make(map[string]bool, len(link.Weekdays))
		for _, day := range link.Weekdays {
			requested[day] = true
		}
		days := make([]string, 0, len(PickupDayOrder))
		for _, day := range PickupDayOrder {
			if requested[day] {
				days = append(days, CompanionWeekdayShortLabels[day])
			}
		}
		if len(days) == 0 {
			continue
		}
		part := CompanionDisplayName(link)
		if len(days) < len(PickupDayOrder) {
			part += " (" + strings.Join(days, ", ") + ")"
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, ", ")
}
