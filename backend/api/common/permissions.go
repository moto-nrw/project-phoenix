package common

// HasAdminPermissions reports whether the supplied JWT permissions grant
// administrative wildcard access (`admin:*` or `*:*`). Several handlers use
// this as a short-circuit before per-resource scope checks. Single source
// of truth — do not redeclare in-package copies.
func HasAdminPermissions(permissions []string) bool {
	for _, p := range permissions {
		if p == "admin:*" || p == "*:*" {
			return true
		}
	}
	return false
}
