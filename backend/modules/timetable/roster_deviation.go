package timetable

import (
	"maps"
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
// of the offerings that obviously stand behind its roster.
type RosterDeviation struct {
	// Offerings is the suggested source, best first: the fewest offerings
	// passing the rule that together explain the most planned children
	// (#5139). A Randstunde for a whole Klasse is explained by Randstunde and
	// Ganztag together; one of them alone would drop the other's children.
	Offerings []OfferingRef
	// GradeLevels / SchoolClasses are the suggested source filter, taken
	// from the Regeltermin's Jahrgang or Klasse targets.
	GradeLevels   []int
	SchoolClasses []string
	// UnbookedCount counts children planned on a weekday without a booking
	// of any suggested offering for that weekday.
	UnbookedCount int
	// MissingCount counts children a suggested offering books on a weekday
	// without being planned that weekday.
	MissingCount int
	// Candidates are the suggested offerings, then every other offering
	// passing the rule; when Ambiguous, only their equal alternatives follow.
	Candidates []OfferingRef
	// Combinations lists complete equal choices, retaining complementary offerings.
	// It includes the original suggestion when Ambiguous; otherwise it is empty.
	Combinations [][]OfferingRef
	// Ambiguous is set when an offering outside the suggestion covers the
	// roster fully and could replace a suggested one without losing a
	// planned child (#5138): the rule cannot tell them apart, so the school
	// picks, nothing is preselected, and the counts belong to no named
	// choice.
	Ambiguous bool
}

type rosterSlot struct {
	studentID int64
	weekday   int
}

type deviationCandidate struct {
	offering OfferingRef
	phaseID  int64
	booked   map[rosterSlot]bool
	coverage float64
	jaccard  float64
	// passes is the rule of #5103; only a passing offering is suggested on
	// its own or offered as a choice.
	passes bool
}

// DetectRosterDeviation applies the rule to one roster. It returns nil when
// no offering passes or the suggested ones match the roster exactly.
func DetectRosterDeviation(roster TemplateFutureRoster, bookings []OfferingWeekdayBooking) *RosterDeviation {
	deviation := SuggestOfferingSource(roster, bookings)
	if deviation == nil || (deviation.UnbookedCount == 0 && deviation.MissingCount == 0) {
		return nil
	}
	return deviation
}

// SuggestOfferingSource applies the rule to the children of a Regeltermin
// that is still being planned (#5104). Unlike DetectRosterDeviation it also
// answers when the suggested offerings match the roster exactly: nothing
// deviates yet, but connecting keeps the roster in step with later
// bookings. It returns nil when no offering passes.
func SuggestOfferingSource(roster TemplateFutureRoster, bookings []OfferingWeekdayBooking) *RosterDeviation {
	planned := plannedRosterSlots(roster)
	candidates := rosterDeviationCandidates(roster, planned, bookings)
	if len(candidates) == 0 || !candidates[0].passes {
		return nil
	}
	chosen := combineDeviationCandidates(candidates, planned)
	alternatives, combinations := interchangeableCandidates(candidates, chosen, planned)
	booked := unionBookedSlots(candidates, chosen)
	refs := make([]OfferingRef, 0, len(chosen))
	for _, index := range chosen {
		refs = append(refs, candidates[index].offering)
	}
	return &RosterDeviation{
		Offerings:     refs,
		GradeLevels:   slices.Clone(roster.TargetGradeLevels),
		SchoolClasses: slices.Clone(roster.TargetSchoolClasses),
		UnbookedCount: distinctSlotStudents(planned, booked),
		MissingCount:  distinctSlotStudents(booked, planned),
		Candidates:    deviationChoices(candidates, chosen, alternatives),
		Combinations:  combinations,
		Ambiguous:     len(alternatives) > 0,
	}
}

// combineDeviationCandidates starts with the best passing offering and adds,
// one at a time, the offering of the same enrollment phase that explains the
// most planned child-weekdays not explained yet. An offering joins when it
// explains at least one more and the combination keeps the rule's coverage:
// two Randstunde children of a Ganztag class join, an offering booked mostly
// outside the roster never does. It returns candidate indexes in the order
// they joined.
func combineDeviationCandidates(candidates []deviationCandidate, planned map[rosterSlot]bool) []int {
	chosen := []int{0}
	covered := maps.Clone(candidates[0].booked)
	matched := countPlannedSlots(covered, planned)
	for {
		next, nextGain := -1, 0
		for index := range candidates {
			if slices.Contains(chosen, index) || candidates[index].phaseID != candidates[0].phaseID {
				continue
			}
			gain, added := uncoveredSlots(candidates[index].booked, covered, planned)
			coverage := float64(matched+gain) / float64(len(covered)+added)
			if gain > nextGain && coverage >= RosterDeviationMinCoverage {
				next, nextGain = index, gain
			}
		}
		if next < 0 {
			return chosen
		}
		chosen = append(chosen, next)
		maps.Copy(covered, candidates[next].booked)
		matched += nextGain
	}
}

// uncoveredSlots counts the booked child-weekdays the combination does not
// hold yet: the planned ones it would explain and all it would add.
func uncoveredSlots(booked, covered, planned map[rosterSlot]bool) (gain, added int) {
	for slot := range booked {
		if covered[slot] {
			continue
		}
		added++
		if planned[slot] {
			gain++
		}
	}
	return gain, added
}

// interchangeableCandidates lists the candidates outside the combination
// that cover the roster fully and could replace one of its offerings while
// explaining as many planned child-weekdays.
func interchangeableCandidates(candidates []deviationCandidate, chosen []int, planned map[rosterSlot]bool) ([]int, [][]OfferingRef) {
	explained := countPlannedSlots(unionBookedSlots(candidates, chosen), planned)
	alternatives := make([]int, 0)
	combinations := [][]OfferingRef{}
	for index, candidate := range candidates {
		if slices.Contains(chosen, index) || !candidate.passes || candidate.coverage < 1 || candidate.phaseID != candidates[chosen[0]].phaseID {
			continue
		}
		for position := range chosen {
			swapped := slices.Clone(chosen)
			swapped[position] = index
			if countPlannedSlots(unionBookedSlots(candidates, swapped), planned) == explained {
				if len(combinations) == 0 {
					combinations = append(combinations, candidateOfferingRefs(candidates, chosen))
				}
				alternatives = append(alternatives, index)
				combinations = append(combinations, candidateOfferingRefs(candidates, swapped))
				break
			}
		}
	}
	return alternatives, combinations
}

func candidateOfferingRefs(candidates []deviationCandidate, indexes []int) []OfferingRef {
	refs := make([]OfferingRef, 0, len(indexes))
	for _, index := range indexes {
		refs = append(refs, candidates[index].offering)
	}
	return refs
}

// deviationChoices names the suggested offerings first, then every other
// candidate; an ambiguous choice offers only the equal alternatives, a
// weaker one would look like one.
func deviationChoices(candidates []deviationCandidate, chosen, alternatives []int) []OfferingRef {
	refs := make([]OfferingRef, 0, len(candidates))
	for _, index := range chosen {
		refs = append(refs, candidates[index].offering)
	}
	for index, candidate := range candidates {
		if slices.Contains(chosen, index) || !candidate.passes || (len(alternatives) > 0 && !slices.Contains(alternatives, index)) {
			continue
		}
		refs = append(refs, candidate.offering)
	}
	return refs
}

func unionBookedSlots(candidates []deviationCandidate, indexes []int) map[rosterSlot]bool {
	union := make(map[rosterSlot]bool)
	for _, index := range indexes {
		maps.Copy(union, candidates[index].booked)
	}
	return union
}

func countPlannedSlots(slots, planned map[rosterSlot]bool) int {
	count := 0
	for slot := range slots {
		if planned[slot] {
			count++
		}
	}
	return count
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

// rosterDeviationCandidates scores every offering booking a planned weekday
// and orders them: those passing the rule first, then highest coverage, then
// the closest match to the roster (Jaccard), then the lowest offering id.
func rosterDeviationCandidates(roster TemplateFutureRoster, planned map[rosterSlot]bool, bookings []OfferingWeekdayBooking) []deviationCandidate {
	candidates := make([]deviationCandidate, 0)
	for _, candidate := range bookedSlotsByOffering(roster, bookings) {
		if len(candidate.booked) > 0 {
			candidate.passes = scoreDeviationCandidate(candidate, planned)
			candidates = append(candidates, *candidate)
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].passes != candidates[j].passes {
			return candidates[i].passes
		}
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
				phaseID:  booking.PhaseID,
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

// scoreDeviationCandidate fills coverage and Jaccard of an offering with
// booked slots and reports whether it passes the rule.
func scoreDeviationCandidate(candidate *deviationCandidate, planned map[rosterSlot]bool) bool {
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
