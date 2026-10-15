package api

import (
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// parseCapabilityDate turns a public YYYY-MM-DD day into the calendar type
// the retained services take; a malformed day is an invalid request.
func parseCapabilityDate(value, field string) (timezone.Date, error) {
	date, err := timezone.ParseDate(value)
	if err != nil {
		return timezone.Date(""), &workforce.TimeTrackingError{Kind: workforce.ErrTimeTrackingInvalid, Cause: fmt.Errorf("%s must be YYYY-MM-DD", field)}
	}
	return date, nil
}

// parseCapabilityRange parses the from/to pair every range read takes.
func parseCapabilityRange(from, to string) (timezone.Date, timezone.Date, error) {
	fromDate, err := parseCapabilityDate(from, "from")
	if err != nil {
		return timezone.Date(""), timezone.Date(""), err
	}
	toDate, err := parseCapabilityDate(to, "to")
	if err != nil {
		return timezone.Date(""), timezone.Date(""), err
	}
	return fromDate, toDate, nil
}
