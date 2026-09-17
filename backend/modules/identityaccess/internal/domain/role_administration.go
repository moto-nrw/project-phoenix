package domain

import (
	"errors"
	"time"
)

// Role and permission administration (#3314): role and permission CRUD,
// account role assignment with its identity side effects, direct account
// grants and role-permission selections. The error texts are the wire
// contract the retained auth service established.

var (
	ErrSystemRoleImmutable = errors.New("system roles cannot be modified")
	ErrPermissionNotFound  = errors.New("permission not found")
	// ErrBaseRoleRequired rejects a custom role without the system role it
	// maps to for announcement targeting.
	ErrBaseRoleRequired = errors.New("base_role is required for custom roles")

	ErrRoleCaregiverNeedsProfile     = errors.New("Ein Lehrkraft-Konto hat kein Betreuungsprofil und kann nicht auf eine Betreuer-Rolle umgestellt werden") //nolint:staticcheck // ST1005: user-facing German message
	ErrLehrkraftRoleImmutable        = errors.New("Ein Lehrkraft-Konto kann nicht umgestellt werden")                                                       //nolint:staticcheck // ST1005: user-facing German message
	ErrRoleLehrkraftCaregiverProfile = errors.New("Das Konto hat ein Betreuungsprofil an dieser Schule und kann nicht auf Lehrkraft umgestellt werden")     //nolint:staticcheck // ST1005: user-facing German message
)

// ManagedRole is one auth.roles row as the administration reads and writes
// it. TenantID is nil for the platform system roles.
type ManagedRole struct {
	ID          int64
	TenantID    *int64
	Name        string
	Description string
	IsSystem    bool
	BaseRole    *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Facts projects the role onto its classification facts.
func (r ManagedRole) Facts() *RoleFacts {
	return &RoleFacts{ID: r.ID, TenantID: r.TenantID, Name: r.Name, IsSystem: r.IsSystem, BaseRole: r.BaseRole}
}

// ManagedPermission is one auth.permissions row.
type ManagedPermission struct {
	ID          int64
	Name        string
	Description string
	Resource    string
	Action      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// RoleFilter narrows a role listing. Zero values do not filter.
type RoleFilter struct {
	Name string
}

// PermissionFilter narrows a permission listing. Zero values do not filter.
type PermissionFilter struct {
	Resource string
	Action   string
}
