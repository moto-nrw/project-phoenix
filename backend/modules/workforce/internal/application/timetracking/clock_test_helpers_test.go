package timetracking

import "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"

// Behaviour tests pin the absence service's calendar-day clock through this option.

// WithAbsenceToday overrides the absence service's calendar-day clock.
func WithAbsenceToday(today func() calendar.Date) StaffAbsenceOption {
	return func(s *staffAbsenceService) { s.todayFunc = today }
}
