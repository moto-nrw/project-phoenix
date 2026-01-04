package auth

import (
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// tableAuthRolePermissions is the schema-qualified table name for role permissions
const tableAuthRolePermissions = "auth.role_permissions"

// RolePermission represents a mapping between roles and permissions
type RolePermission struct {
	base.Model   `bun:"schema:auth,table:role_permissions"`
	RoleID       int64 `bun:"role_id,notnull" json:"role_id"`
	PermissionID int64 `bun:"permission_id,notnull" json:"permission_id"`

	// Relations
	Role       *Role       `bun:"rel:belongs-to,join:role_id=id" json:"role,omitempty"`
	Permission *Permission `bun:"rel:belongs-to,join:permission_id=id" json:"permission,omitempty"`
}

// TableName returns the database table name
func (rp *RolePermission) TableName() string {
	return tableAuthRolePermissions
}

func (rp *RolePermission) BeforeAppendModel(query any) error {
	switch q := query.(type) {
	case *bun.SelectQuery:
		q.ModelTableExpr(tableAuthRolePermissions)
	case *bun.UpdateQuery:
		q.ModelTableExpr(tableAuthRolePermissions)
	case *bun.DeleteQuery:
		q.ModelTableExpr(tableAuthRolePermissions)
	}
	return nil
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

// GetID returns the entity's ID
func (m *RolePermission) GetID() interface{} {
	return m.ID
}

// GetCreatedAt returns the creation timestamp
func (m *RolePermission) GetCreatedAt() time.Time {
	return m.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (m *RolePermission) GetUpdatedAt() time.Time {
	return m.UpdatedAt
}
