package api

import (
	userModels "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The bindings below hand owner compositions the People Directory's retained
// rules and rows through narrow ports, so no composition names the legacy
// users model (#4004).

// ClassDayEnrolledOn reports whether a child with the given status and
// enrollment window is enrolled on date.
func ClassDayEnrolledOn(status string, enrolledFrom, enrolledUntil *calendar.Date, date, today calendar.Date) bool {
	return userModels.EnrolledOn(&userModels.Student{
		Status: userModels.StudentStatus(status), EnrolledFrom: enrolledFrom, EnrolledUntil: enrolledUntil,
	}, date, today)
}
