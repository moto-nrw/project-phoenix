package test

import (
	"github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/models/schedule"
)

// AuditStammdatenSectionSchoolClasses is the Stammdaten audit section of a
// staff member's class assignments (#1772).
const AuditStammdatenSectionSchoolClasses = audit.StammdatenSectionSchoolClasses

// The roster statuses the grade transition suites arrange and assert.
const (
	ScheduleAttendanceStatusExpected = schedule.AttendanceStatusExpected
	ScheduleAttendanceStatusPresent  = schedule.AttendanceStatusPresent
	ScheduleAttendanceStatusAbsent   = schedule.AttendanceStatusAbsent
	ScheduleInstanceStatusCompleted  = schedule.InstanceStatusCompleted
)
