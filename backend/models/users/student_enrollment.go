package users

import "github.com/moto-nrw/project-phoenix/internal/timezone"

// EnrolledOn reports whether a child is enrolled in the OGS on the given
// calendar date. It is the single definition of that question; every reader
// that answers it for a date other than "right now" must call this rather
// than compare enrolled_from/enrolled_until by hand (#1565, #2606).
//
// The enrollment interval is the source of truth: it is correct for past and
// future dates alike, whereas the lifecycle status is only the scheduler's
// projection of "enrolled today" and is wrong for any other date — a currently
// active child whose enrollment ends before a future date would otherwise
// still be counted, and a pending child whose enrollment has already started
// would be missed.
//
// Immediate activation (enrollment.default_activation_mode = "immediate") is
// the deliberate exception: the enrollment decision service creates an already
// 'active' student while keeping enrolled_from at the phase's future service
// start date, so the child may check in from today. An active status therefore
// overrides the enrolled_from lower bound — but only from today onward. The
// override must NOT make the child retroactively enrolled for every past date
// before enrolled_from, or a roster, a slot list or a statistics denominator
// would count them for days their OGS time had not begun.
//
// When neither bound is recorded (legacy rows, manual create) the interval
// carries no information, so the current lifecycle status is the only signal
// and an inactive student is treated as no longer enrolled.
//
// today is a parameter rather than a fresh timezone.TodayDate() read so a
// request spanning Berlin midnight keeps one notion of "today" across every
// date it decides — re-reading the process clock per call could admit a child
// for one date and drop them for the next.
//
// Readers that must express the same rule in SQL (the statistics student and
// room aggregates) mirror it as an effective lower bound:
//
//	CASE WHEN status = 'active' THEN LEAST(enrolled_from, today) ELSE enrolled_from END
//
// with NULL meaning "no bound". Keep those in step with this function.
func EnrolledOn(student *Student, date, today timezone.Date) bool {
	if student == nil {
		return false
	}
	if student.EnrolledFrom != nil && date.Before(*student.EnrolledFrom) {
		// Before the recorded start date, an active child is only eligible
		// from today onward; past dates keep the lower bound.
		if student.Status != StudentStatusActive || date.Before(today) {
			return false
		}
	}
	if student.EnrolledUntil != nil && date.After(*student.EnrolledUntil) {
		return false
	}
	if student.EnrolledFrom == nil && student.EnrolledUntil == nil {
		return student.Status != StudentStatusInactive
	}
	return true
}
