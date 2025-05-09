package auth

import (
	"errors"
	"strings"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// Role represents a user role
type Role struct {
	base.Model
	Name        string `bun:"name,notnull,unique" json:"name"`
	Description string `bun:"description" json:"description"`

	// Relations
	Permissions []*Permission `bun:"-" json:"permissions,omitempty"`
}

// TableName returns the database table name
func (r *Role) TableName() string {
	return "auth.roles"
}

// Validate ensures role data is valid
func (r *Role) Validate() error {
	if r.Name == "" {
		return errors.New("role name is required")
	}

	// Normalize role name to lowercase
	r.Name = strings.ToLower(r.Name)

	return nil
}

// HasPermission checks if the role has the specified permission
func (r *Role) HasPermission(permission string) bool {
	if r.Permissions == nil {
		return false
	}

	for _, p := range r.Permissions {
		if strings.EqualFold(p.Name, permission) {
			return true
		}
	}

	return false
}

// AddPermission adds a permission to the role if it doesn't already exist
func (r *Role) AddPermission(permission *Permission) {
	// Initialize permissions slice if nil
	if r.Permissions == nil {
		r.Permissions = make([]*Permission, 0)
	}

	// Check if permission already exists
	for _, p := range r.Permissions {
		if p.ID == permission.ID {
			return // Permission already exists
		}
	}

	r.Permissions = append(r.Permissions, permission)
}

// RemovePermission removes a permission from the role
func (r *Role) RemovePermission(permissionID int64) bool {
	if r.Permissions == nil {
		return false
	}

	for i, p := range r.Permissions {
		if p.ID == permissionID {
			// Remove permission by slicing
			r.Permissions = append(r.Permissions[:i], r.Permissions[i+1:]...)
			return true
		}
	}

	return false
}
