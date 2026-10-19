package owners

import (
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// parseCapabilityDate reads a public YYYY-MM-DD day the way Workforce
// refuses a malformed one.
func parseCapabilityDate(value, field string) (timezone.Date, error) {
	return workforce.ParseTimeTrackingDate(value, field)
}

// ParseCapabilityRange parses the from/to pair every range read takes.
func ParseCapabilityRange(from, to string) (timezone.Date, timezone.Date, error) {
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
