package timetable

import (
	"slices"
	"sort"
	"strings"
)

// The detection rule of #5103, decided with the schools: an offering
// explains a hand-kept roster when at least 80 % of the child-weekdays it
// books (restricted to the Regeltermin's weekdays and its Klasse or Jahrgang)
// are on the roster, and at least three such children are. The system only
// suggests; the school confirms through the preview.
const (
	RosterDeviationMinCoverage = 0.8
	RosterDeviationMinChildren = 3
)

// RosterOfferingKind names which offerings can stand behind the children of
// a Regeltermin. A Regeltermin that holds a whole class matches every offering
// of that class; the kind keeps a Randstunde from being explained by the
// lunch or a course.
type RosterOfferingKind int

const (
	// RosterOfferingAny admits every offering.
	RosterOfferingAny RosterOfferingKind = iota
	// RosterOfferingCare admits offerings that count as care.
	RosterOfferingCare
	// RosterOfferingLunch admits offerings with lunch (Mensa lists).
	RosterOfferingLunch
	// RosterOfferingCourse admits offerings that are neither care nor lunch.
	RosterOfferingCourse
)

// RosterOfferingKindFor derives the kind from the Regeltermin's type and list
// kind: a Mensa list takes lunch offerings, any other care block care
// offerings, an AG course offerings; everything else takes any offering.
func RosterOfferingKindFor(templateType string, listKind *string) RosterOfferingKind {
	switch {
	case templateType == GroupTypeCare && listKind != nil && *listKind == "mensa":
		return RosterOfferingLunch
	case templateType == GroupTypeCare:
		return RosterOfferingCare
	case templateType == GroupTypeActivity:
		return RosterOfferingCourse
	}
	return RosterOfferingAny
}

func (kind RosterOfferingKind) admits(booking OfferingWeekdayBooking) bool {
	switch kind {
	case RosterOfferingCare:
		return booking.CountsAsCare
	case RosterOfferingLunch:
		return booking.IncludesLunch
	case RosterOfferingCourse:
		return !booking.CountsAsCare && !booking.IncludesLunch
	}
	return true
}

// RosterDeviation is a detected deviation of a Regeltermin from the bookings
// of an offering that obviously stands behind its roster.
type RosterDeviation struct {
	Offering OfferingRef
	// GradeLevels / SchoolClasses are the suggested source filter, taken
	// from the Regeltermin's Jahrgang or Klasse targets.
	GradeLevels   []int
	SchoolClasses []string
	// UnbookedCount counts children planned on a weekday without a booking
	// of the offering for that weekday.
	UnbookedCount int
	// MissingCount counts children booked on a weekday without being
	// planned that weekday.
	MissingCount int
	// Candidates are every offering passing the rule, best first.
	Candidates []OfferingRef
}

type rosterSlot struct {
	studentID int64
	weekday   int
}

type deviationCandidate struct {
	offering OfferingRef
	booked   map[rosterSlot]bool
	coverage float64
	jaccard  float64
}

// DetectRosterDeviation applies the rule to one roster. It returns nil when
// no offering passes or the best one matches the roster exactly.
func DetectRosterDeviation(roster TemplateFutureRoster, bookings []OfferingWeekdayBooking) *RosterDeviation {
	deviation := SuggestOfferingSource(roster, bookings)
	if deviation == nil || (deviation.UnbookedCount == 0 && deviation.MissingCount == 0) {
		return nil
	}
	return deviation
}

// SuggestOfferingSource applies the rule to the children of a Regeltermin
// that is still being planned (#5104). Unlike DetectRosterDeviation it also
// answers when the best offering matches the roster exactly: nothing
// deviates yet, but connecting keeps the roster in step with later
// bookings. It returns nil when no offering passes.
func SuggestOfferingSource(roster TemplateFutureRoster, bookings []OfferingWeekdayBooking) *RosterDeviation {
	planned := plannedRosterSlots(roster)
	candidates := rosterDeviationCandidates(roster, planned, bookings)
	if len(candidates) == 0 {
		return nil
	}
	best := candidates[0]
	unbooked, missing := distinctSlotStudents(planned, best.booked), distinctSlotStudents(best.booked, planned)
	refs := make([]OfferingRef, 0, len(candidates))
	for _, candidate := range candidates {
		refs = append(refs, candidate.offering)
	}
	return &RosterDeviation{
		Offering:      best.offering,
		GradeLevels:   slices.Clone(roster.TargetGradeLevels),
		SchoolClasses: slices.Clone(roster.TargetSchoolClasses),
		UnbookedCount: unbooked,
		MissingCount:  missing,
		Candidates:    refs,
	}
}

func plannedRosterSlots(roster TemplateFutureRoster) map[rosterSlot]bool {
	planned := make(map[rosterSlot]bool)
	for weekday, students := range roster.Weekdays {
		for _, studentID := range students {
			planned[rosterSlot{studentID: studentID, weekday: weekday}] = true
		}
	}
	return planned
}

// rosterDeviationCandidates keeps the offerings passing the rule and orders
// them: highest coverage, then the closest match to the roster (Jaccard),
// then the lowest offering id.
func rosterDeviationCandidates(roster TemplateFutureRoster, planned map[rosterSlot]bool, bookings []OfferingWeekdayBooking) []deviationCandidate {
	candidates := make([]deviationCandidate, 0)
	for _, candidate := range bookedSlotsByOffering(roster, bookings) {
		if scoreDeviationCandidate(candidate, planned) {
			candidates = append(candidates, *candidate)
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].coverage != candidates[j].coverage {
			return candidates[i].coverage > candidates[j].coverage
		}
		if candidates[i].jaccard != candidates[j].jaccard {
			return candidates[i].jaccard > candidates[j].jaccard
		}
		return candidates[i].offering.ID < candidates[j].offering.ID
	})
	return candidates
}

// bookedSlotsByOffering groups the bookings of the Regeltermin's cohort per
// offering into the child-weekdays it runs.
func bookedSlotsByOffering(roster TemplateFutureRoster, bookings []OfferingWeekdayBooking) map[int64]*deviationCandidate {
	byOffering := make(map[int64]*deviationCandidate)
	for _, booking := range bookings {
		if !roster.OfferingKind.admits(booking) || !bookingInTargetCohort(roster, booking) {
			continue
		}
		candidate := byOffering[booking.OfferingID]
		if candidate == nil {
			candidate = &deviationCandidate{
				offering: OfferingRef{ID: booking.OfferingID, Name: booking.OfferingName},
				booked:   make(map[rosterSlot]bool),
			}
			byOffering[booking.OfferingID] = candidate
		}
		addScheduledSlots(candidate.booked, roster, booking)
	}
	return byOffering
}

func addScheduledSlots(slots map[rosterSlot]bool, roster TemplateFutureRoster, booking OfferingWeekdayBooking) {
	for _, weekday := range booking.Weekdays {
		if _, scheduled := roster.Weekdays[weekday]; scheduled {
			slots[rosterSlot{studentID: booking.StudentID, weekday: weekday}] = true
		}
	}
}

// scoreDeviationCandidate fills coverage and Jaccard and reports whether the
// offering passes the rule.
func scoreDeviationCandidate(candidate *deviationCandidate, planned map[rosterSlot]bool) bool {
	if len(candidate.booked) == 0 {
		return false
	}
	matched := 0
	matchedStudents := make(map[int64]bool)
	for slot := range candidate.booked {
		if planned[slot] {
			matched++
			matchedStudents[slot.studentID] = true
		}
	}
	candidate.coverage = float64(matched) / float64(len(candidate.booked))
	candidate.jaccard = float64(matched) / float64(len(candidate.booked)+len(planned)-matched)
	return candidate.coverage >= RosterDeviationMinCoverage && len(matchedStudents) >= RosterDeviationMinChildren
}

// bookingInTargetCohort restricts the bookings to the Regeltermin's Klasse
// or Jahrgang; without such targets every booking counts.
func bookingInTargetCohort(roster TemplateFutureRoster, booking OfferingWeekdayBooking) bool {
	if len(roster.TargetSchoolClasses) > 0 {
		class := strings.TrimSpace(booking.SchoolClass)
		for _, target := range roster.TargetSchoolClasses {
			if class != "" && strings.EqualFold(class, strings.TrimSpace(target)) {
				return true
			}
		}
		return false
	}
	if len(roster.TargetGradeLevels) > 0 {
		return booking.GradeLevel != nil && slices.Contains(roster.TargetGradeLevels, *booking.GradeLevel)
	}
	return true
}

// distinctSlotStudents counts the children with a slot in from that is not
// in other.
func distinctSlotStudents(from, other map[rosterSlot]bool) int {
	students := make(map[int64]bool)
	for slot := range from {
		if !other[slot] {
			students[slot.studentID] = true
		}
	}
	return len(students)
}

// OfferingSourceFilter is a source selection with its Jahrgang or Klassen
// filter, as a Regeltermin would store it.
type OfferingSourceFilter struct {
	OfferingIDs   []int64
	GradeLevels   []int
	SchoolClasses []string
}

// OfferingSourceWeekdayChange is the preview of one weekday: who joins and
// who leaves when the offerings become the source, and how many stay.
type OfferingSourceWeekdayChange struct {
	Weekday   int
	Added     []int64
	Removed   []int64
	KeptCount int
}

// PreviewOfferingSource compares the roster's weekdays with the children the
// selection books on them (#5103). The filter matches like the resync: an
// empty filter admits everyone, a set one never admits a child without
// grade respectively class.
func PreviewOfferingSource(roster TemplateFutureRoster, filter OfferingSourceFilter, bookings []OfferingWeekdayBooking) []OfferingSourceWeekdayChange {
	booked := make(map[rosterSlot]bool)
	for _, booking := range bookings {
		if slices.Contains(filter.OfferingIDs, booking.OfferingID) && bookingPassesSourceFilter(filter, booking) {
			addScheduledSlots(booked, roster, booking)
		}
	}
	weekdays := make([]int, 0, len(roster.Weekdays))
	for weekday := range roster.Weekdays {
		weekdays = append(weekdays, weekday)
	}
	slices.Sort(weekdays)
	changes := make([]OfferingSourceWeekdayChange, 0, len(weekdays))
	for _, weekday := range weekdays {
		changes = append(changes, weekdayChange(weekday, roster.Weekdays[weekday], booked))
	}
	return changes
}

func weekdayChange(weekday int, planned []int64, booked map[rosterSlot]bool) OfferingSourceWeekdayChange {
	change := OfferingSourceWeekdayChange{Weekday: weekday, Added: []int64{}, Removed: []int64{}}
	onRoster := make(map[int64]bool, len(planned))
	for _, studentID := range planned {
		onRoster[studentID] = true
		if booked[rosterSlot{studentID: studentID, weekday: weekday}] {
			change.KeptCount++
		} else {
			change.Removed = append(change.Removed, studentID)
		}
	}
	for slot := range booked {
		if slot.weekday == weekday && !onRoster[slot.studentID] {
			change.Added = append(change.Added, slot.studentID)
		}
	}
	slices.Sort(change.Added)
	slices.Sort(change.Removed)
	return change
}

func bookingPassesSourceFilter(filter OfferingSourceFilter, booking OfferingWeekdayBooking) bool {
	if len(filter.GradeLevels) > 0 && (booking.GradeLevel == nil || !slices.Contains(filter.GradeLevels, *booking.GradeLevel)) {
		return false
	}
	if len(filter.SchoolClasses) == 0 {
		return true
	}
	class := strings.TrimSpace(booking.SchoolClass)
	for _, wanted := range filter.SchoolClasses {
		if class != "" && strings.EqualFold(class, strings.TrimSpace(wanted)) {
			return true
		}
	}
	return false
}
