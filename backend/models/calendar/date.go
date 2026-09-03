package calendar

import "github.com/moto-nrw/project-phoenix/modules/appointments"

// Date remains as a compatibility alias while appointment-owned domain types
// move behind the Appointments capability.
type Date = appointments.Date

var (
	NewDate   = appointments.NewDate
	ParseDate = appointments.ParseDate
)
