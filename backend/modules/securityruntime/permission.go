package securityruntime

import "github.com/moto-nrw/project-phoenix/auth/authorize"

// HasPermission evaluates a required permission with the shared wildcard rules.
func HasPermission(required string, permissions []string) bool {
	return authorize.HasPermission(required, permissions)
}

// HasAdminWildcard reports a system-wide admin permission (admin:* or *:*).
func HasAdminWildcard(permissions []string) bool {
	return authorize.HasAdminWildcard(permissions)
}

// DatabaseStatsCapabilities evaluates which database statistics the
// permissions reveal.
func DatabaseStatsCapabilities(permissions []string) authorize.DatabaseStatsCapabilities {
	return authorize.NewDatabaseStatsCapabilities(permissions)
}
