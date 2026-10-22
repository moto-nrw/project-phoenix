package users

import "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"

// CareEndedOn reports whether the child's care has already ended on the given
// calendar day — the enrollment interval's upper bound is INCLUSIVE, so the
// last care day itself still counts as care.
//
// Pure derivation from two fields the row already carries: no policy, no
// thresholds, no clock (backend-conventions rule 12). Every operational gate
// spells the question this way so "ab dem Folgetag" cannot be interpreted
// half a day differently in two places.
func (s *Student) CareEndedOn(day calendar.Date) bool {
	return s != nil && s.EnrolledUntil != nil && day.After(*s.EnrolledUntil)
}
