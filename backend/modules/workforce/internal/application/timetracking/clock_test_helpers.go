package timetracking

import "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"

// The composition root never overrides the calendar-day clock: production
// resolves today from the Berlin wall clock. Behaviour tests and the root test
// modules pin it through these options.

// WithAbsenceToday overrides the absence service's calendar-day clock.
func WithAbsenceToday(today func() calendar.Date) StaffAbsenceOption {
	return func(s *staffAbsenceService) { s.todayFunc = today }
}

// WithAdjustmentToday overrides the ledger's calendar-day clock.
func WithAdjustmentToday(today func() calendar.Date) StaffBalanceAdjustmentOption {
	return func(s *staffBalanceAdjustmentService) { s.todayFunc = today }
}
