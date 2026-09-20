package securityruntime

import "github.com/moto-nrw/project-phoenix/auth/authorize"

// HasPermission evaluates a required permission with the shared wildcard rules.
func HasPermission(required string, permissions []string) bool {
	return authorize.HasPermission(required, permissions)
}
