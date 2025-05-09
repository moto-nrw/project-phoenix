package auth

import (
	"errors"
	"strings"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// Permission represents a system permission
type Permission struct {
	base.Model
	Name        string `bun:"name,notnull,unique" json:"name"`
	Description string `bun:"description" json:"description"`
	Resource    string `bun:"resource,notnull" json:"resource"`
	Action      string `bun:"action,notnull" json:"action"`
}

// TableName returns the database table name
func (p *Permission) TableName() string {
	return "auth.permissions"
}

// Validate ensures permission data is valid
func (p *Permission) Validate() error {
	if p.Name == "" {
		return errors.New("permission name is required")
	}

	if p.Resource == "" {
		return errors.New("resource is required")
	}

	if p.Action == "" {
		return errors.New("action is required")
	}

	// Normalize permission name to lowercase and remove spaces
	p.Name = strings.ToLower(strings.ReplaceAll(p.Name, " ", "_"))

	// Normalize resource and action
	p.Resource = strings.ToLower(p.Resource)
	p.Action = strings.ToLower(p.Action)

	return nil
}

// GetFullName returns the permission's full name in format "resource:action"
func (p *Permission) GetFullName() string {
	return p.Resource + ":" + p.Action
}

// IsAdminPermission checks if this is an admin-level permission
func (p *Permission) IsAdminPermission() bool {
	return p.Resource == "admin" || strings.HasPrefix(p.Name, "admin:")
}

// Clone creates a copy of the permission
func (p *Permission) Clone() *Permission {
	return &Permission{
		Model: base.Model{
			ID:        p.ID,
			CreatedAt: p.CreatedAt,
			UpdatedAt: p.UpdatedAt,
		},
		Name:        p.Name,
		Description: p.Description,
		Resource:    p.Resource,
		Action:      p.Action,
	}
}
