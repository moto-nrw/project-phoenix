package supervisiondashboardhttp

import "github.com/moto-nrw/project-phoenix/auth/authorize/permissions"

// The permissions the dashboard checks for its caller: reading the schedules
// of the supervised groups and reading their students.
const (
	ScheduleReadPermission = permissions.SchedulesRead
	StudentReadPermission  = permissions.UsersRead
)
