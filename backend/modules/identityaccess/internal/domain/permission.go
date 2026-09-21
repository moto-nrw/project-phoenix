package domain

import (
	"errors"
	"strings"
)

// Validate checks and normalizes a permission before it enters the catalog.
func (p *ManagedPermission) Validate() error {
	if p.Name == "" {
		return errors.New("permission name is required")
	}
	if p.Resource == "" {
		return errors.New("resource is required")
	}
	if p.Action == "" {
		return errors.New("action is required")
	}
	p.Name = strings.ToLower(strings.ReplaceAll(p.Name, " ", "_"))
	p.Resource = strings.ToLower(p.Resource)
	p.Action = strings.ToLower(p.Action)
	return nil
}
