package compose

import (
	"github.com/moto-ogs/project-phoenix/modules/peopledirectory/internal/application"
)

func NewAttendanceRouteStaff(source application.AttendanceRouteStaffSource) application.AttendanceRouteStaff {
	return application.NewAttendanceRouteStaff(source)
}
