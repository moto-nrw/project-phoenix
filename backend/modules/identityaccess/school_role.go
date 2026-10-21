package identityaccess

import (
	"context"
	"errors"
)

var ErrRoleNotFound = errors.New("role not found")

// SchoolRole contains the stored facts used by school-role assignment and
// grant checks. It exposes no persistence model or relation types.
type SchoolRole struct {
	ID          int64
	TenantID    *int64
	Name        string
	IsSystem    bool
	BaseRole    *string
	Permissions []string
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

type RolePermissionQuery interface {
	FindRolePermissions(context.Context, int64) ([]string, error)
}
