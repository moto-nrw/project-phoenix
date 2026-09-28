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
	// closing days on purpose (holiday care); they are only selected when the
	// caller asks for them with BulkCancelOptions.IncludeClosingDaySeries.
	SeriesIncludesClosingDays bool
	// SeriesName names the series (the template) so the dialog can list the
	// series that stay.
	SeriesName string
}

// BulkCancelOptions carries the switches of one bulk cancellation.
type BulkCancelOptions struct {
	// DryRun only counts; nothing is cancelled.
	DryRun bool
	// IncludeClosingDaySeries also cancels occurrences of series planned on
	// closing days on purpose (holiday care). Off by default.
	IncludeClosingDaySeries bool
}

// BulkCancelKeptSeries names one series whose occurrences stay in the plan
// and how many of them lie in the range.
type BulkCancelKeptSeries struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
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
	// Kept counts the planned occurrences in the range that stay because
	// their series includes closing days (holiday care), so the dialog only
	// mentions them when there are some.
	Kept int `json:"kept"`
	// KeptSeries lists those series by name, ordered by name.
	KeptSeries []BulkCancelKeptSeries `json:"kept_series"`
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
// that include closing days unless opts.IncludeClosingDaySeries asks for
// them. Running, finished, cancelled and past
// occurrences stay untouched. The IDs come back ordered by date so the
// caller takes its per-day locks in ascending order; the per-day counts feed
// the confirmation dialog.
func SelectBulkCancel(candidates []BulkCancelCandidate, from, to, today string, opts BulkCancelOptions) ([]int64, BulkCancelResult) {
	lower := max(from, today)
	selected := make([]BulkCancelCandidate, 0, len(candidates))
	kept := 0
	keptBySeries := map[string]int{}
	for _, candidate := range candidates {
		if candidate.Status != InstanceStatusPlanned || candidate.Date < lower || candidate.Date > to {
			continue
		}
		if candidate.SeriesIncludesClosingDays && !opts.IncludeClosingDaySeries {
			kept++
			keptBySeries[candidate.SeriesName]++
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

	result := BulkCancelResult{
		From: from, To: to, DryRun: opts.DryRun, Count: len(selected),
		Days: []BulkCancelDay{}, Kept: kept, KeptSeries: keptSeries(keptBySeries),
	}
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

// keptSeries turns the per-series counts into a list ordered by name, empty
// rather than nil so the dialog always reads a list.
func keptSeries(counts map[string]int) []BulkCancelKeptSeries {
	series := make([]BulkCancelKeptSeries, 0, len(counts))
	for name, count := range counts {
		series = append(series, BulkCancelKeptSeries{Name: name, Count: count})
	}
	sort.Slice(series, func(i, j int) bool { return series[i].Name < series[j].Name })
	return series
}
