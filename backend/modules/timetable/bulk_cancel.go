package timetable

import (
	"errors"
	"fmt"
	"sort"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// MaxBulkCancelDays bounds one bulk cancellation (#3594) to a school year,
// long enough for every closure a school enters at once.
const MaxBulkCancelDays = 366

// ErrInvalidBulkCancelRange rejects a malformed, reversed or too long range.
var ErrInvalidBulkCancelRange = errors.New("invalid bulk cancel range")

// BulkCancelCandidate is one occurrence inside the requested range, with the
// facts the selection needs.
type BulkCancelCandidate struct {
	InstanceID int64
	Date       string
	Status     string
	// SeriesIncludesClosingDays marks occurrences of a series planned on
	// closing days on purpose (holiday care); they are never selected.
	SeriesIncludesClosingDays bool
}

// BulkCancelDay counts the selected occurrences of one day.
type BulkCancelDay struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

// BulkCancelResult reports what a bulk cancellation selected or removed.
type BulkCancelResult struct {
	From   string          `json:"from"`
	To     string          `json:"to"`
	DryRun bool            `json:"dry_run"`
	Count  int             `json:"count"`
	Days   []BulkCancelDay `json:"days"`
}

// ValidateBulkCancelRange accepts two "YYYY-MM-DD" dates, from <= to, at
// most MaxBulkCancelDays days inclusive.
func ValidateBulkCancelRange(from, to string) error {
	start, err := calendar.ParseDate(from)
	if err != nil {
		return fmt.Errorf("%w: from must be a date in YYYY-MM-DD format", ErrInvalidBulkCancelRange)
	}
	end, err := calendar.ParseDate(to)
	if err != nil {
		return fmt.Errorf("%w: to must be a date in YYYY-MM-DD format", ErrInvalidBulkCancelRange)
	}
	if end.Before(start) {
		return fmt.Errorf("%w: to must not be before from", ErrInvalidBulkCancelRange)
	}
	if days := start.DaysUntil(end) + 1; days > MaxBulkCancelDays {
		return fmt.Errorf("%w: range exceeds %d days", ErrInvalidBulkCancelRange, MaxBulkCancelDays)
	}
	return nil
}

// SelectBulkCancel picks the occurrences a bulk cancellation removes: planned
// ones dated in [from, to] and not before today, except occurrences of series
// that include closing days. Running, finished, cancelled and past
// occurrences stay untouched. The IDs come back ordered by date so the
// caller takes its per-day locks in ascending order; the per-day counts feed
// the confirmation dialog.
func SelectBulkCancel(candidates []BulkCancelCandidate, from, to, today string) ([]int64, BulkCancelResult) {
	lower := max(from, today)
	selected := make([]BulkCancelCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.Status != InstanceStatusPlanned || candidate.SeriesIncludesClosingDays {
			continue
		}
		if candidate.Date < lower || candidate.Date > to {
			continue
		}
		selected = append(selected, candidate)
	}
	sort.Slice(selected, func(i, j int) bool {
		if selected[i].Date != selected[j].Date {
			return selected[i].Date < selected[j].Date
		}
		return selected[i].InstanceID < selected[j].InstanceID
	})

	result := BulkCancelResult{From: from, To: to, Count: len(selected), Days: []BulkCancelDay{}}
	ids := make([]int64, 0, len(selected))
	for _, candidate := range selected {
		ids = append(ids, candidate.InstanceID)
		last := len(result.Days) - 1
		if last >= 0 && result.Days[last].Date == candidate.Date {
			result.Days[last].Count++
			continue
		}
		result.Days = append(result.Days, BulkCancelDay{Date: candidate.Date, Count: 1})
	}
	return ids, result
}
