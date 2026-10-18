//go:build testhooks

package compose

import (
	activities "github.com/moto-nrw/project-phoenix/models/activities"
	schedule "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

func NativeGroupRecords(rows activities.GroupRepository) ports.NativeGroupRecords {
	return application.NativeGroupRecords(rows)
}
func NativeScheduleRecords(rows activities.ScheduleRepository) ports.NativeScheduleRecords {
	return application.NativeScheduleRecords(rows)
}
func NativeEnrollmentRecords(rows activities.StudentEnrollmentRepository) ports.NativeEnrollmentRecords {
	return application.NativeEnrollmentRecords(rows)
}
func NativeExceptionRecords(rows schedule.ActivityExceptionRepository) ports.NativeExceptionRecords {
	return application.NativeExceptionRecords(rows)
}
func NativeTimeframeRecords(rows schedule.TimeframeRepository) ports.NativeTimeframeRecords {
	return application.NativeTimeframeRecords(rows)
}
