package identityaccess

import (
	"context"
)

var ErrRoleNotFound = notFoundError("role not found")

// SchoolRole contains the stored facts used by school-role assignment and
// grant checks. It exposes no persistence model or relation types.
type SchoolRole struct {
	ID          int64
	TenantID    *int64
	Name        string
	IsSystem    bool
	BaseRole    *string
	Permissions []string
	// SchoolPortal marks a school role made from the Lehrkraft template (#4574).
	SchoolPortal bool
	// NameKey is LOWER(name) as PostgreSQL computes it. ListSchoolRoles fills
	// it so a caller can match names exactly as FindSchoolRoleByName does,
	// which compares the trimmed, lowered input with LOWER(name) (#5887).
	NameKey string
}

func (r *SchoolRole) AuthorizationGrantData() (bool, string, *string, bool, bool, []string) {
	if r == nil {
		return false, "", nil, false, false, nil
	}
	return true, r.Name, r.BaseRole, r.IsSystem, r.TenantID != nil, append([]string(nil), r.Permissions...)
}

type SchoolRoleQuery interface {
	ListSchoolRoles(context.Context) ([]*SchoolRole, error)
	FindSchoolRoleByName(context.Context, string) (*SchoolRole, error)
}

// RolePermissionQuery reads the permission names of the given roles of the
// caller's school or of the system in one statement (#5887). A role without
// permissions, or one of another school, has no entry.
type RolePermissionQuery interface {
	FindRolePermissionsByRoleIDs(context.Context, []int64) (map[int64][]string, error)
}
