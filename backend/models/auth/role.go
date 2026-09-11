package auth

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// System role names used as base role targets for custom roles.
const (
	BaseRoleAdmin    = "admin"
	BaseRoleUser     = "user"
	BaseRoleGuardian = "guardian"
)

// ValidBaseRoles returns the system roles that custom roles can map to.
// When adding a new base role, also update:
//   - DB CHECK constraint in migration 001015031_roles_base_role.go
//   - Frontend options in roles.config.tsx and role-detail-modal.tsx
func ValidBaseRoles() []string {
	return []string{BaseRoleAdmin, BaseRoleUser, BaseRoleGuardian}
}

// Role represents a user role
type Role struct {
	base.Model  `bun:"schema:auth,table:roles"`
	TenantID    *int64  `bun:"tenant_id" json:"tenant_id,omitempty"`
	Name        string  `bun:"name,notnull" json:"name"`
	Description string  `bun:"description" json:"description"`
	IsSystem    bool    `bun:"is_system,notnull,default:false" json:"is_system"`
	BaseRole    *string `bun:"base_role" json:"base_role,omitempty"`

	// Relations
	Permissions []*Permission `bun:"-" json:"permissions,omitempty"`
}

// AuthorizationGrantData exposes the stored facts used by role-grant policy.
// The policy itself remains outside the persistence model.
func (r *Role) AuthorizationGrantData() (present bool, name string, baseRole *string, system, tenantBound bool, permissions []string) {
	if r == nil {
		return false, "", nil, false, false, nil
	}
	permissions = make([]string, 0, len(r.Permissions))
	for _, permission := range r.Permissions {
		if permission == nil {
			// Preserve the policy's fail-closed treatment of a malformed loaded
			// relation instead of silently dropping it from the subset check.
			permissions = append(permissions, "")
			continue
		}
		permissions = append(permissions, permission.Name)
	}
	return true, r.Name, r.BaseRole, r.IsSystem, r.TenantID != nil, permissions
}

// Validate ensures role data is valid
func (r *Role) Validate() error {
	if r.Name == "" {
		return errors.New("role name is required")
	}

	// Normalize role name to lowercase
	r.Name = strings.ToLower(r.Name)

	if r.BaseRole != nil {
		trimmed := strings.TrimSpace(*r.BaseRole)
		if trimmed == "" {
			r.BaseRole = nil
		} else {
			r.BaseRole = &trimmed
		}
	}
	if r.BaseRole != nil {
		if !slices.Contains(ValidBaseRoles(), *r.BaseRole) {
			return fmt.Errorf("base_role must be one of %v", ValidBaseRoles())
		}
	}

	return nil
}

// GetTenantID returns the tenant ID (0 if nil/system role).
func (r *Role) GetTenantID() int64 {
	if r.TenantID != nil {
		return *r.TenantID
	}
	return 0
}

// SetTenantID sets the tenant ID.
func (r *Role) SetTenantID(id int64) {
	r.TenantID = &id
}
