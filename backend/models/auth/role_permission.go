package auth

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// RolePermission represents a mapping between roles and permissions
type RolePermission struct {
	base.Model
	RoleID       int64 `bun:"role_id,notnull" json:"role_id"`
	PermissionID int64 `bun:"permission_id,notnull" json:"permission_id"`

	// Relations
	Role       *Role       `bun:"rel:belongs-to,join:role_id=id" json:"role,omitempty"`
	Permission *Permission `bun:"rel:belongs-to,join:permission_id=id" json:"permission,omitempty"`
}

// TableName returns the database table name
func (rp *RolePermission) TableName() string {
	return "auth.role_permissions"
}

// Validate ensures role permission mapping data is valid
func (rp *RolePermission) Validate() error {
	if rp.RoleID <= 0 {
		return errors.New("role ID is required")
	}

	if rp.PermissionID <= 0 {
		return errors.New("permission ID is required")
	}

	return nil
}
