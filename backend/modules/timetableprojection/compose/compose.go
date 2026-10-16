// Package compose names the timetable legacy view's readers for the roots.
package compose

import "github.com/moto-nrw/project-phoenix/modules/timetableprojection"

// The course group and request-sourced enrollment reads other owners' ports bind.
var (
	LockCourseGroups             = timetableprojection.LockCourseGroups
	CountActiveCourseEnrollments = timetableprojection.CountActiveCourseEnrollments
	CountSourceEnrollments       = timetableprojection.CountSourceEnrollments
)
