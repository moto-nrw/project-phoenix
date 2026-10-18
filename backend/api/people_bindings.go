package api

import (
	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The bindings below hand owner compositions the People Directory's retained
// rules and rows through narrow ports, so no composition names the legacy
// users model (#4004).

// ClassDayEnrolledOn reports whether a child with the given status and
// enrollment window is enrolled on date.
func ClassDayEnrolledOn(status string, enrolledFrom, enrolledUntil *calendar.Date, date, today calendar.Date) bool {
	return peopleValues.EnrolledOn(&peopleValues.StudentState{
		Status: string(status), EnrolledFrom: enrolledFrom, EnrolledUntil: enrolledUntil,
	}, date, today)
}
