package studentpresencehttp

import "github.com/moto-nrw/project-phoenix/auth/authorize/permissions"

// StatisticsReportPermissions are the permissions the caller of the Statistik
// report and its export must hold all of: config:read (the report gate of
// the other admin reports) and users:read (the per-child rows are personal
// data).
func StatisticsReportPermissions() []string {
	return []string{permissions.ConfigRead, permissions.UsersRead}
}
