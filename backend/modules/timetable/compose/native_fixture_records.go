//go:build testhooks

package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

func NativeGroupRecords(rows ports.ActivityGroupRecords) ports.NativeGroupRecords {
	return application.NativeGroupRecords(rows)
}
func NativeScheduleRecords(rows ports.ActivityScheduleRecords) ports.NativeScheduleRecords {
	return application.NativeScheduleRecords(rows)
}
func NativeEnrollmentRecords(rows ports.ActivityEnrollmentRecords) ports.NativeEnrollmentRecords {
	return application.NativeEnrollmentRecords(rows)
}
func NativeExceptionRecords(rows ports.ActivityExceptionRecords) ports.NativeExceptionRecords {
	return application.NativeExceptionRecords(rows)
}
func NativeTimeframeRecords(rows ports.TimeframeRecords) ports.NativeTimeframeRecords {
	return application.NativeTimeframeRecords(rows)
}
