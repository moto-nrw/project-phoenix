package operator

import (
	"strings"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

// parseInvoiceDate turns an optional "YYYY-MM-DD" wire value into a calendar
// date. An empty string means "not set" and yields (nil, nil) — that is how
// the operator clears a payment date. Anything else that does not parse is an
// error, never a silently dropped value.
//
// timezone.Date rather than time.Time on purpose: due_date and paid_on are
// DATE columns, and a Berlin-midnight instant binds one day early
// (.claude/rules/calendar-dates.md).
func parseInvoiceDate(raw string) (*timezone.Date, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}
	parsed, err := timezone.ParseDate(trimmed)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}
