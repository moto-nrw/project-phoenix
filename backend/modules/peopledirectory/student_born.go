package peopledirectory

import "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"

// StudentBorn is a child of the school born on a queried date, also one
// whose care has ended, with the interval it belonged to the school
// (#5994): a holiday booking of the past finds the child that belonged to
// the school on the booked day. A nil end is an open membership.
type StudentBorn struct {
	StudentID     int64
	FirstName     string
	LastName      string
	Birthday      calendar.Date
	EnrolledFrom  *calendar.Date
	EnrolledUntil *calendar.Date
}

// MemberOn reports whether the child belonged to the school on day.
func (s StudentBorn) MemberOn(day calendar.Date) bool {
	return (s.EnrolledFrom == nil || !day.Before(*s.EnrolledFrom)) && (s.EnrolledUntil == nil || !day.After(*s.EnrolledUntil))
}
