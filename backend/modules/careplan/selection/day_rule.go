package selection

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
)

// SelectionRuleAtMostOnePerDay lets a child hold several offerings of one
// selection group, but no two of them on the same weekday (#4514). Variants
// with fixed pickup times are the case: Monday "bis 13 Uhr", Tuesday
// "bis 17 Uhr" is a valid week, both on Monday is not.
const SelectionRuleAtMostOnePerDay = "at_most_one_per_day"

// weekdayOrder is the order a conflict reports its day in, so the first
// reported conflict is deterministic.
var weekdayOrder = []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"}

// DayConflictError names the first weekday on which a child holds more than
// one offering of a group with the per-day rule. errors.Is matches
// ErrCareOfferingDayConflict and therefore ErrCareOfferingRule and
// ErrInvalidSubmission.
type DayConflictError struct {
	Child       int
	Group       string
	Day         string
	OfferingIDs []int64
}

func (e *DayConflictError) Error() string {
	return fmt.Sprintf("%s: child %d group %q day %q offerings %v",
		ErrCareOfferingDayConflict.Error(), e.Child, e.Group, e.Day, e.OfferingIDs)
}

func (e *DayConflictError) Unwrap() error { return ErrCareOfferingDayConflict }

// checkDayRule reports the first weekday on which the child holds more than
// one offering of the group. It reads the materialized selection: a fixed
// offering covers all its days, a parent_choice offering its selected days,
// automatic days included, so an automatic addition cannot slip past it.
func checkDayRule(childIdx int, group string, child Child, openByID map[int64]*Offering) error {
	daysByOffering := make(map[int64][]string, len(child.OfferingDays))
	for _, row := range child.OfferingDays {
		daysByOffering[row.OfferingID] = row.SelectedDays
	}
	holders := make(map[string][]int64, len(weekdayOrder))
	seen := make(map[int64]bool, len(child.OfferingIDs))
	for _, id := range child.OfferingIDs {
		offering, ok := openByID[id]
		if !ok || seen[id] || strings.TrimSpace(offering.SelectionGroup) != group {
			continue
		}
		seen[id] = true
		days := daysByOffering[id]
		if offering.DaysOfWeekMode == "fixed" {
			days = offering.AvailableDays
		}
		for _, day := range uniqueDays(days) {
			holders[day] = append(holders[day], id)
		}
	}
	for _, day := range weekdayOrder {
		if ids := holders[day]; len(ids) > 1 {
			return &DayConflictError{Child: childIdx, Group: group, Day: day, OfferingIDs: ids}
		}
	}
	return nil
}

// CheckDayRules checks a selection that is already materialized, such as the
// bookings on file when an application is approved, against every per-day
// group of the catalog. It changes nothing; the other rules stay with
// MaterializeAdjustments and MaterializeChild.
func CheckDayRules(child Child, catalog map[int64]*Offering) error {
	groups := map[string]bool{}
	for _, offering := range catalog {
		group := strings.TrimSpace(offering.SelectionGroup)
		if group != "" && offering.SelectionRule == SelectionRuleAtMostOnePerDay {
			groups[group] = true
		}
	}
	for _, group := range slices.Sorted(maps.Keys(groups)) {
		if err := checkDayRule(0, group, child, catalog); err != nil {
			return err
		}
	}
	return nil
}

// withChildIndex names the caller's child in a weekday conflict: the
// engine checks one child at a time, so the check itself only sees index 0.
func withChildIndex(err error, index int) error {
	var conflict *DayConflictError
	if errors.As(err, &conflict) {
		conflict.Child = index
	}
	return err
}

func uniqueDays(days []string) []string {
	seen := make(map[string]bool, len(days))
	out := make([]string, 0, len(days))
	for _, day := range days {
		day = strings.ToLower(strings.TrimSpace(day))
		if seen[day] {
			continue
		}
		seen[day] = true
		out = append(out, day)
	}
	return out
}
