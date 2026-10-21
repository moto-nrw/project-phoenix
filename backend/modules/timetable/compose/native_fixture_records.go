//go:build testhooks

package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

func NativeGroupRecords(rows ports.GroupRows) ports.NativeGroupRecords {
	return application.NativeGroupRecords(rows)
}
func NativeScheduleRecords(schedules timetable.ScheduleCapability) ports.NativeScheduleRecords {
	return application.NativeScheduleRecords(schedules)
}
func NativeEnrollmentRecords(enrollments timetable.StudentEnrollmentCapability) ports.NativeEnrollmentRecords {
	return application.NativeEnrollmentRecords(enrollments)
}
func NativeExceptionRecords(exceptions timetable.ActivityExceptionCapability) ports.NativeExceptionRecords {
	return application.NativeExceptionRecords(exceptions)
}
func NativeTimeframeRecords(timeframes timetable.TimeframeCapability) ports.NativeTimeframeRecords {
	return application.NativeTimeframeRecords(timeframes)
}

// NativeInstances serves the fixtures' legacy instance rows.
func (r Repositories) NativeInstances() ports.NativeInstanceRecords {
	return application.NativeInstanceRecords(r.ActivityInstance)
}
