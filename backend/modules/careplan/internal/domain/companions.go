package domain

import (
	"fmt"
	"slices"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

// CompanionStudent is what the companion rules read from one child's
// departure plan, which People Directory owns: whether a free-text "mit wem"
// note answers for every day, and the weekdays the plan lets the child leave
// with another child.
type CompanionStudent struct {
	ID               int64
	HasCompanionNote bool
	AccompaniedDays  map[string]bool
}

// CompanionWeekdayNumber translates a weekday key into its stored number.
func CompanionWeekdayNumber(day string) (int, error) {
	if index := slices.Index(careplan.CompanionWeekdays, day); index >= 0 {
		return index + 1, nil
	}
	return 0, fmt.Errorf("%w: got %q", careplan.ErrCompanionInvalidWeekday, day)
}

// CompanionWeekdayKey translates a stored weekday number into its key.
func CompanionWeekdayKey(weekday int) (string, bool) {
	if weekday < 1 || weekday > len(careplan.CompanionWeekdays) {
		return "", false
	}
	return careplan.CompanionWeekdays[weekday-1], true
}

// NewCompanionEdge builds an edge between two children, normalizing the pair
// into the stored low/high order.
func NewCompanionEdge(studentID, companionID int64, weekday int) (careplan.CompanionEdge, error) {
	if studentID <= 0 || companionID <= 0 {
		return careplan.CompanionEdge{}, careplan.ErrCompanionStudentIDRequired
	}
	if studentID == companionID {
		return careplan.CompanionEdge{}, careplan.ErrCompanionSelfLink
	}
	if _, ok := CompanionWeekdayKey(weekday); !ok {
		return careplan.CompanionEdge{}, fmt.Errorf("%w: got %d", careplan.ErrCompanionInvalidWeekday, weekday)
	}
	low, high := studentID, companionID
	if low > high {
		low, high = high, low
	}
	return careplan.CompanionEdge{StudentLowID: low, StudentHighID: high, Weekday: weekday}, nil
}

// OtherCompanion returns the far end of an edge as seen from studentID, and
// whether studentID is part of the edge at all.
func OtherCompanion(edge careplan.CompanionEdge, studentID int64) (int64, bool) {
	switch studentID {
	case edge.StudentLowID:
		return edge.StudentHighID, true
	case edge.StudentHighID:
		return edge.StudentLowID, true
	default:
		return 0, false
	}
}

// BuildCompanionEdges validates a submitted list and turns it into edges,
// also returning the requested weekdays per companion. Each link becomes one
// undirected edge per weekday, so adding Tom to Lina's card is the same row
// that shows Lina on Tom's card.
func BuildCompanionEdges(studentID int64, links []careplan.CompanionLink) ([]careplan.CompanionEdge, map[int64][]string, error) {
	edges := make([]careplan.CompanionEdge, 0, len(links)*len(careplan.CompanionWeekdays))
	byCompanion := make(map[int64][]string, len(links))
	for _, link := range links {
		if link.CompanionStudentID == studentID {
			return nil, nil, careplan.ErrCompanionSelfLink
		}
		if _, seen := byCompanion[link.CompanionStudentID]; seen {
			return nil, nil, careplan.ErrDuplicateCompanion
		}
		linkEdges, days, err := companionLinkEdges(studentID, link)
		if err != nil {
			return nil, nil, err
		}
		edges = append(edges, linkEdges...)
		byCompanion[link.CompanionStudentID] = days
	}
	return edges, byCompanion, nil
}

// companionLinkEdges turns one link into its weekday edges, folding repeated
// days, and returns the weekdays it kept.
func companionLinkEdges(studentID int64, link careplan.CompanionLink) ([]careplan.CompanionEdge, []string, error) {
	if len(link.Weekdays) == 0 {
		return nil, nil, careplan.ErrCompanionWeekdayRequired
	}
	edges := make([]careplan.CompanionEdge, 0, len(link.Weekdays))
	days := make([]string, 0, len(link.Weekdays))
	seenDays := make(map[int]bool, len(link.Weekdays))
	for _, day := range link.Weekdays {
		weekday, err := CompanionWeekdayNumber(day)
		if err != nil {
			return nil, nil, err
		}
		if seenDays[weekday] {
			continue
		}
		seenDays[weekday] = true
		edge, err := NewCompanionEdge(studentID, link.CompanionStudentID, weekday)
		if err != nil {
			return nil, nil, err
		}
		edges = append(edges, edge)
		days = append(days, day)
	}
	return edges, days, nil
}

// TrimCompanionLinks keeps only the weekdays allowedDays permits, dropping a
// link that has none left, and reports whether anything changed.
func TrimCompanionLinks(links []careplan.CompanionLink, allowedDays map[string]bool) ([]careplan.CompanionLink, bool) {
	trimmed := make([]careplan.CompanionLink, 0, len(links))
	changed := false
	for _, link := range links {
		kept := make([]string, 0, len(link.Weekdays))
		for _, day := range link.Weekdays {
			if allowedDays[day] {
				kept = append(kept, day)
			}
		}
		if len(kept) != len(link.Weekdays) {
			changed = true
		}
		if len(kept) == 0 {
			continue
		}
		link.Weekdays = kept
		trimmed = append(trimmed, link)
	}
	return trimmed, changed
}

// MissingAccompaniedDays returns the requested weekdays the plan does not
// (yet) allow, in Mon..Fri order.
func MissingAccompaniedDays(allowed map[string]bool, requested []string) []string {
	wanted := make(map[string]bool, len(requested))
	for _, day := range requested {
		wanted[day] = true
	}
	var missing []string
	for _, day := range careplan.CompanionWeekdays {
		if wanted[day] && !allowed[day] {
			missing = append(missing, day)
		}
	}
	return missing
}

// RemovedCompanionDays lists, per far child, the weekdays a list change
// removes, and the far children in ascending order.
func RemovedCompanionDays(before, after []careplan.CompanionLink) ([]int64, map[int64][]string) {
	keptDays := make(map[int64]map[string]bool, len(after))
	for _, link := range after {
		days := keptDays[link.CompanionStudentID]
		if days == nil {
			days = make(map[string]bool, len(link.Weekdays))
			keptDays[link.CompanionStudentID] = days
		}
		for _, day := range link.Weekdays {
			days[day] = true
		}
	}
	removedDays := make(map[int64][]string, len(before))
	removed := make([]int64, 0, len(before))
	for _, link := range before {
		for _, day := range link.Weekdays {
			if keptDays[link.CompanionStudentID][day] {
				continue
			}
			if _, seen := removedDays[link.CompanionStudentID]; !seen {
				removed = append(removed, link.CompanionStudentID)
			}
			removedDays[link.CompanionStudentID] = append(removedDays[link.CompanionStudentID], day)
		}
	}
	slices.Sort(removed)
	return removed, removedDays
}

// StrandsCompanion reports whether removing the given weekdays leaves the far
// child with an accompanied day and no "mit wem" detail: no free-text note
// and no other companion on that day. The check is PER WEEKDAY: another
// companion covering only Monday does not answer for Tuesday.
func StrandsCompanion(companion CompanionStudent, removedDays []string, covered map[string]bool) bool {
	if companion.HasCompanionNote {
		return false
	}
	for _, day := range removedDays {
		if companion.AccompaniedDays[day] && !covered[day] {
			return true
		}
	}
	return false
}
