package common

// HasAdminPermissions reports whether the supplied JWT permissions grant
// administrative wildcard access (`admin:*` or `*:*`). Several handlers use
// this as a short-circuit before per-resource scope checks. Centralised
// here so new endpoints don't need a fourth in-package copy — three legacy
// duplicates in api/students, api/guardians and api/groups predate this
// helper and are pending consolidation.
func HasAdminPermissions(permissions []string) bool {
	for _, p := range permissions {
		if p == "admin:*" || p == "*:*" {
			return true
		}
	}
	return false
}
