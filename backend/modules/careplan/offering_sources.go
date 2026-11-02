package careplan

import "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"

// OfferingSourcedTemplate is one template already sourcing an offering, so
// the editor can warn about overlapping Jahrgang subsets before the admin
// saves a second Regeltermin over the same children.
type OfferingSourcedTemplate struct {
	ID          int64
	Name        string
	GradeLevels []int
	// SchoolClasses is the template's Klassenfilter (#2482); mutually
	// exclusive with GradeLevels.
	SchoolClasses []string
}

// OfferingSourceOption is one selectable Betreuungsangebot in the Regeltermin
// editor (#2137), with per-grade counts of approved children (live filter
// preview) and the templates already sourcing it (overlap Hinweis).
type OfferingSourceOption struct {
	ID        int64
	Name      string
	PhaseID   int64
	PhaseName string
	// PhaseServiceStart is the phase's service_start_date, or the period start
	// when the phase begins earlier. Sourced enrollments never start earlier,
	// so materialised occurrences before this day stay empty of offering-fed
	// children (OGS am Berg, 2026-08).
	PhaseServiceStart calendar.Date
	// TotalCount is the number of distinct approved children currently or
	// prospectively enrolled in the offering.
	TotalCount int
	// GradeCounts maps Jahrgang → approved children; key 0 collects children
	// whose school class carries no derivable grade number.
	GradeCounts            map[int]int
	SourcedTemplates       []OfferingSourcedTemplate
	LegacyLinkedTemplateID *int64
}

// OfferingSourceCombinedCounts is the deduplicated child count across a
// SELECTION of offerings (multi-source follow-up to #2137): a child enrolled
// in two of the selected offerings counts once, so the editor's Jahrgang
// preview shows the exact roster size the union resync would seed.
type OfferingSourceCombinedCounts struct {
	// TotalCount is the number of DISTINCT approved children across the
	// selected offerings.
	TotalCount int
	// GradeCounts maps Jahrgang → distinct approved children; key 0 collects
	// children whose school class carries no derivable grade number.
	GradeCounts map[int]int
	// Students is the deduplicated child list behind the counts (#2482), each
	// with the school class the filters match on. Names are not carried: the
	// editor resolves them from the student list it already holds.
	Students []OfferingSourceStudent
}

// OfferingSourceStudent is one deduplicated child of a source selection with
// the school class the Jahrgang/Klassen filters are evaluated against.
type OfferingSourceStudent struct {
	StudentID   int64
	SchoolClass string
}

// The kinds of EmptyOfferingRosterExplanation.
const (
	EmptyOfferingRosterBeforeServiceStart = "before_offering_start"
	EmptyOfferingRosterSourceEmpty        = "offering_source_empty"
)

// EmptyOfferingRosterExplanation describes why an occurrence backed by care
// offerings currently has no concrete children. After the service start the
// explanation deliberately stays neutral: an empty occurrence can result
// from weekday/filter rules, changed enrollments, or a stale materialization.
type EmptyOfferingRosterExplanation struct {
	Kind             string
	PhaseName        string
	ServiceStartDate calendar.Date
}

// ExplainEmptyOfferingRoster classifies an empty sourced occurrence from the
// authoritative offering-phase metadata.
func ExplainEmptyOfferingRoster(
	options []OfferingSourceOption,
	selectedOfferingIDs []int64,
	date calendar.Date,
) *EmptyOfferingRosterExplanation {
	if len(selectedOfferingIDs) == 0 {
		return nil
	}
	selected := make(map[int64]struct{}, len(selectedOfferingIDs))
	for _, offeringID := range selectedOfferingIDs {
		selected[offeringID] = struct{}{}
	}
	explanation := &EmptyOfferingRosterExplanation{Kind: EmptyOfferingRosterSourceEmpty}
	for _, option := range options {
		if _, ok := selected[option.ID]; !ok {
			continue
		}
		explanation.PhaseName = option.PhaseName
		explanation.ServiceStartDate = option.PhaseServiceStart
		if !option.PhaseServiceStart.IsZero() && date.Before(option.PhaseServiceStart) {
			explanation.Kind = EmptyOfferingRosterBeforeServiceStart
		}
		return explanation
	}
	return explanation
}

// OfferingWeekdayBooking is one child's approved booking of an active
// offering overlapping a calendar period, with the weekdays it plans (ISO,
// 1 = Monday) from a date on (#5103). A child with several links to the same
// offering appears once, with the union of their weekdays.
type OfferingWeekdayBooking struct {
	OfferingID   int64
	OfferingName string
	// PhaseID is the offering's enrollment phase; a combined source must
	// stay within one phase (#5139).
	PhaseID int64
	// HolidayPhase is true when that phase is of the kind Ferienbetreuung;
	// such an offering is never suggested as a Regeltermin source (#5139).
	// The offering's own IncludesHolidayCare flag plays no part.
	HolidayPhase bool
	StudentID    int64
	SchoolClass  string
	GradeLevel   *int16
	Weekdays     []int
	// CountsAsCare and IncludesLunch are the offering's catalog flags; they
	// tell care blocks, Mensa lists and courses apart.
	CountsAsCare  bool
	IncludesLunch bool
	// TemplateGapAckClasses are the classes the school decided need no
	// Regeltermin of this offering in the requested planning period (#5140);
	// empty without a decision there.
	TemplateGapAckClasses []string
	// ClassDayApplies is true when the offering runs on school days only:
	// its phase is a school year and it includes no holiday care. Only then
	// does the Klassentag tell when its children arrive (#5343).
	ClassDayApplies bool
	// PickupTimes are the offering's pickup times ("HH:MM") per ISO weekday;
	// a weekday without one has no known end.
	PickupTimes map[int]string
	// Windows are the child's links to the offering that still run on or
	// after from, each with its own validity and weekdays; Weekdays is their
	// union.
	Windows []OfferingBookingWindow
}

// OfferingBookingWindow is one booking link: the ISO weekdays it plans from
// ValidFrom (inclusive, nil = open) to ValidUntil (exclusive, nil = open).
type OfferingBookingWindow struct {
	ValidFrom  *calendar.Date
	ValidUntil *calendar.Date
	Weekdays   []int
	// Ended is true when the booking link itself ends at ValidUntil, by a
	// booking change or a withdrawal, not the end of its phase (#5102).
	Ended bool
}
