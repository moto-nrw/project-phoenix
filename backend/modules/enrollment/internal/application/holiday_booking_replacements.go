package application

import (
	"slices"

	"github.com/moto-ogs/project-phoenix/modules/enrollment"
)

// holidayBookingReplacements projects the durable rebooking history onto
// current bookings, including replacements that waited before admission.
func holidayBookingReplacements(period enrollment.HolidayPeriod, detail *enrollment.HolidayRegistrationDetail) {
	weeks := make(map[int64]enrollment.HolidayWeek, len(period.Weeks))
	for _, week := range period.Weeks {
		weeks[week.ID] = week
	}
	lineage := map[int64]map[int64][]int64{}
	for _, event := range detail.History {
		if event.Action != enrollment.HolidayAuditRebook {
			continue
		}
		child := event.Details.ChildID
		if lineage[child] == nil {
			lineage[child] = map[int64][]int64{}
		}
		advanceHolidayReplacements(lineage[child], event.Details, weeks)
	}
	for i := range detail.Children {
		child := &detail.Children[i]
		for j := range child.Bookings {
			booking := &child.Bookings[j]
			booking.ReplacedWeeks = holidayReplacementRanges(lineage[child.ID][booking.WeekID], weeks)
		}
	}
}

func advanceHolidayReplacements(lineage map[int64][]int64, change enrollment.HolidayRegistrationEventDetails, weeks map[int64]enrollment.HolidayWeek) {
	removed := changedHolidayWeeks(change.Before, change.After, weeks)
	added := changedHolidayWeeks(change.After, change.Before, weeks)
	next := make(map[int64][]int64, min(len(removed), len(added)))
	// With several changes at once, pair removed and added weeks in calendar
	// order. Additional weeks remain additions, never replacements.
	for i := 0; i < min(len(removed), len(added)); i++ {
		previous := slices.Clone(lineage[removed[i]])
		if !slices.Contains(previous, removed[i]) {
			previous = append(previous, removed[i])
		}
		next[added[i]] = previous
	}
	for _, id := range removed {
		delete(lineage, id)
	}
	for id, previous := range next {
		lineage[id] = previous
	}
}

func changedHolidayWeeks(from, other []enrollment.HolidayBookingChange, weeks map[int64]enrollment.HolidayWeek) []int64 {
	kept := make(map[int64]bool, len(other))
	for _, booking := range other {
		kept[booking.WeekID] = true
	}
	var changed []int64
	for _, booking := range from {
		if !kept[booking.WeekID] {
			changed = append(changed, booking.WeekID)
		}
	}
	slices.SortFunc(changed, func(a, b int64) int {
		if order := weeks[a].StartsOn.Compare(weeks[b].StartsOn); order != 0 {
			return order
		}
		if a < b {
			return -1
		}
		if a > b {
			return 1
		}
		return 0
	})
	return changed
}

func holidayReplacementRanges(ids []int64, weeks map[int64]enrollment.HolidayWeek) []enrollment.HolidayBookingReplacement {
	var ranges []enrollment.HolidayBookingReplacement
	for _, id := range ids {
		if week, ok := weeks[id]; ok {
			ranges = append(ranges, enrollment.HolidayBookingReplacement{WeekID: id, WeekStartsOn: week.StartsOn, WeekEndsOn: week.EndsOn})
		}
	}
	return ranges
}
