package careplan

import "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"

// BookingViewDate is the reference date for SHOWING a child's booked care
// offerings to a human — today, falling back to the end of the care period
// once that period is over so a finished period still shows its final state.
//
// Deliberately different from the enrollment reports' offering date, and
// deliberately shared between the parent portal and the staff views (#2185):
// both must answer "what is booked, and what starts later" from the SAME day,
// or a guardian on the phone and the staff member looking at the child see
// different rows. Before the phase starts, that means every booking is
// correctly reported as not yet effective, on both sides.
//
// The reports clamp to the phase START as well, because a report about a
// future phase should describe the state that phase will have — a report is
// about the phase, this is about today.
func BookingViewDate(today, serviceEnd calendar.Date) calendar.Date {
	if !serviceEnd.IsZero() && serviceEnd.Before(today) {
		return serviceEnd
	}
	return today
}
