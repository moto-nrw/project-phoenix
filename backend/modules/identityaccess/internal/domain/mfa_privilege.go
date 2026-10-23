package domain

import "strings"

// PrivilegedMFAPermissions is the minimum second-factor boundary (#3841-f).
// Keep the registry classification test in sync when a new permission is added.
// Role labels deliberately do not participate. Read-only role/catalog inspection
// is included: these are administrative capabilities, not care-day operations.
var PrivilegedMFAPermissions = []string{
	"admin:*", "*:*", "auth:manage", "config:manage", "system:manage",
	"users:manage",
	"roles:create", "roles:read", "roles:update", "roles:delete",
	"permissions:create", "permissions:read", "permissions:update", "permissions:delete",
	"staff:manage", "staff:financial", "staff_documents:health", "student_documents:health",
}

// HasPrivilegedMFAPermission uses the authorization contract's per-segment
// trailing wildcards, including user*:manage and *:health. A future concrete
// health permission is covered even before the catalog classification changes.
func HasPrivilegedMFAPermission(grants []string) bool {
	for _, grant := range grants {
		resource, action, ok := strings.Cut(grant, ":")
		if !ok {
			continue
		}
		if action == "health" {
			return true
		}
		for _, privileged := range PrivilegedMFAPermissions {
			r, a, _ := strings.Cut(privileged, ":")
			if mfaPermissionPartMatches(resource, r) && mfaPermissionPartMatches(action, a) {
				return true
			}
		}
	}
	return false
}

func mfaPermissionPartMatches(granted, required string) bool {
	return granted == required || granted == "*" ||
		(strings.HasSuffix(granted, "*") && strings.HasPrefix(required, strings.TrimSuffix(granted, "*")))
}
