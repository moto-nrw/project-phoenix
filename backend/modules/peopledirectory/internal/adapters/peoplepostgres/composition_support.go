package users

import (
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The helpers below serve the legacy composition seam
// (database/repositories), which assembles the staff, teacher and external
// caregiver record repositories over the School Membership owner and reports their failures in
// the repository error shape these repositories use. The caller's school
// comes from peopledirectory/compose and the permission matcher from
// modules/securityruntime (#2727).

// ParseCalendarDate parses a YYYY-MM-DD value; an empty or malformed value
// yields nil, which is how the legacy models spell "no date".
func ParseCalendarDate(value string) *calendar.Date {
	if value == "" {
		return nil
	}
	parsed, err := calendar.ParseDate(value)
	if err != nil {
		return nil
	}
	return &parsed
}
