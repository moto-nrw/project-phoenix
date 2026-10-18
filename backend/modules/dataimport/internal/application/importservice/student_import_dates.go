package importpkg

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// createStudentFromRow creates a student from person and row
func enrollmentStartsInFuture(enrolledFrom *timezone.Date) bool {
	return enrollmentStartsAfter(enrolledFrom, timezone.TodayDate())
}

func enrollmentStartsAfter(enrolledFrom *timezone.Date, today timezone.Date) bool {
	return enrolledFrom != nil && enrolledFrom.After(today)
}

// parseSupportedDate tries all supported import date formats in order.
// parseDateFormats parses a date string in any supported format
// (YYYY-MM-DD, DD.MM.YYYY, DD.MM.YY) without applying semantic restrictions.
// Use this for dates that may legitimately lie in the future (e.g. enrollment
// start dates). Birthdays must go through parseSupportedDate instead.
func parseDateFormats(dateStr string) (time.Time, error) {
	var lastErr error
	for _, layout := range dateLayouts {
		parsed, err := time.Parse(layout, dateStr)
		if err == nil {
			return parsed, nil
		}
		lastErr = err
	}

	if shortDate, err := parseGermanShortDate(dateStr); err == nil {
		return shortDate, nil
	}

	return time.Time{}, lastErr
}

func parseSupportedDate(dateStr string) (time.Time, error) {
	parsed, err := parseDateFormats(dateStr)
	if err != nil {
		return time.Time{}, err
	}
	if err := validateBirthdayDate(parsed); err != nil {
		return time.Time{}, err
	}
	return parsed, nil
}

func parseGermanShortDate(dateStr string) (time.Time, error) {
	parts := strings.Split(dateStr, ".")
	if len(parts) != 3 || len(parts[2]) != 2 {
		return time.Time{}, fmt.Errorf("invalid short German date format")
	}

	day, err := strconv.Atoi(parts[0])
	if err != nil {
		return time.Time{}, err
	}
	month, err := strconv.Atoi(parts[1])
	if err != nil {
		return time.Time{}, err
	}
	shortYear, err := strconv.Atoi(parts[2])
	if err != nil {
		return time.Time{}, err
	}

	year := 2000 + shortYear
	parsed := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	if parsed.Year() != year || int(parsed.Month()) != month || parsed.Day() != day {
		return time.Time{}, fmt.Errorf("invalid short German date value")
	}

	return parsed, nil
}

func validateBirthdayDate(parsed time.Time) error {
	today := time.Now().In(time.UTC)
	currentDate := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	if parsed.After(currentDate) {
		return errFutureBirthday
	}

	return nil
}

// parseOptionalImportDate parses an optional date in any supported format,
// allowing future dates. Returns nil for empty or unparseable input (format
// errors are surfaced separately during validation). Shared by enrollment and
// consent date columns.
func parseOptionalImportDate(dateStr string) *time.Time {
	trimmed := strings.TrimSpace(dateStr)
	if trimmed == "" {
		return nil
	}
	parsed, err := parseDateFormats(trimmed)
	if err != nil {
		return nil
	}
	return &parsed
}

// parseOptionalImportCalendarDate is the calendar-date sibling of
// parseOptionalImportDate for DATE-typed columns (enrollment window).
func parseOptionalImportCalendarDate(dateStr string) *timezone.Date {
	parsed := parseOptionalImportDate(dateStr)
	if parsed == nil {
		return nil
	}
	d := timezone.DateFromTime(*parsed)
	return &d
}

// parseOptionalDate parses a date string or returns nil
func parseOptionalDate(dateStr string) (*timezone.Date, error) {
	trimmed := strings.TrimSpace(dateStr)
	if trimmed == "" {
		return nil, nil
	}

	t, err := parseSupportedDate(trimmed)
	if err != nil {
		return nil, err
	}

	d := timezone.DateFromTime(t)
	return &d, nil
}
