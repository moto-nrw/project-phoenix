package dataimport

// RoleGrantSource supplies the role facts required for a grant decision.
// Identity & Access owns these facts; Security Runtime decides permission.
type RoleGrantSource interface {
	AuthorizationGrantData() (present bool, name string, baseRole *string, system, tenantBound bool, permissions []string)
}

// Authorization is the import workflow's permission decision port.
type Authorization interface {
	HasPermission(required string, permissions []string) bool
	CanGrantRole(role RoleGrantSource, permissions []string) bool
}
