package common

import "github.com/moto-nrw/project-phoenix/auth/authorize"

// HasPermission reports whether granted satisfies required, wildcards
// included. Handlers that branch on a permission (rather than gate a route
// with RequiresPermission) call this instead of importing the authorization
// package directly.
func HasPermission(required string, granted []string) bool {
	return authorize.HasPermission(required, granted)
}
