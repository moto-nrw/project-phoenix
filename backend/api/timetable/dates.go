// Package timetable — shared date-parsing helpers. Used by WP-B11 (student
// day/week), WP-B12 (gaps, substitute), and WP-B13 (exception conflicts)
// handlers.
package timetable

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
)

var errTimetableWeekend = errors.New("timetable entries can only be scheduled from Monday to Friday")

// berlinDate parses a YYYY-MM-DD input into a calendar date. timezone.Date
// carries no instant, so the historical 00:00–02:00 CET/UTC anchoring pitfall
// cannot occur by construction.
func berlinDate(input string) (timezone.Date, error) {
	return timezone.ParseDate(input)
}

// validateTimetableWorkday keeps the planning contract consistent across the
// three planning views: the OGS timetable accepts appointments only Monday to
// Friday. It lives at the HTTP boundary so direct API clients cannot create
// entries the workweek views intentionally do not display.
func validateTimetableWorkday(date timezone.Date) error {
	switch date.Weekday() {
	case time.Saturday, time.Sunday:
		return errTimetableWeekend
	default:
		return nil
	}
}

// inclusiveDayCount returns the number of calendar days in the inclusive
// [from, to] range. Date arithmetic is UTC-anchored, so DST transitions
// (23h/25h Berlin days) can never skew the count.
func inclusiveDayCount(from, to timezone.Date) int {
	return from.DaysUntil(to) + 1
}

// parseTodayFutureDateRange parses the shared query shape of /gaps and
// /exception-conflicts: `date` required, `date_to` optional (defaults to
// `date`), the inclusive window capped at MaxTimetableReadRangeDays, and the
// start pinned to today-or-future on the Berlin calendar. Renders the 400 and
// returns ok=false on any violation. Error strings are a cross-repo contract —
// keep them byte-identical.
func parseTodayFutureDateRange(w http.ResponseWriter, r *http.Request) (from, to timezone.Date, ok bool) {
	q := r.URL.Query()

	dateStr := q.Get("date")
	if dateStr == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("date is required")))
		return timezone.Date{}, timezone.Date{}, false
	}
	parsedFrom, err := berlinDate(dateStr)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid date format, expected YYYY-MM-DD")))
		return timezone.Date{}, timezone.Date{}, false
	}

	parsedTo := parsedFrom
	if toStr := q.Get("date_to"); toStr != "" {
		parsedTo, err = berlinDate(toStr)
		if err != nil {
			common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid date_to format, expected YYYY-MM-DD")))
			return timezone.Date{}, timezone.Date{}, false
		}
	}

	if parsedFrom.After(parsedTo) {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("'date' must be before or equal to 'date_to'")))
		return timezone.Date{}, timezone.Date{}, false
	}
	if inclusiveDayCount(parsedFrom, parsedTo) > scheduleModel.MaxTimetableReadRangeDays {
		common.RenderError(w, r, common.ErrorInvalidRequest(
			fmt.Errorf("date range exceeds maximum of %d days", scheduleModel.MaxTimetableReadRangeDays)))
		return timezone.Date{}, timezone.Date{}, false
	}
	if parsedFrom.Before(timezone.TodayDate()) {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("'date' must be today or a future date")))
		return timezone.Date{}, timezone.Date{}, false
	}

	return parsedFrom, parsedTo, true
}
