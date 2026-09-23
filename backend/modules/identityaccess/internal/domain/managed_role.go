package domain

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// Validate preserves the stored role contract, including legacy roles without a base tier.
func (r *ManagedRole) Validate() error {
	if r.Name == "" {
		return errors.New("role name is required")
	}
	r.Name = strings.ToLower(r.Name)
	if r.BaseRole != nil {
		trimmed := strings.TrimSpace(*r.BaseRole)
		if trimmed == "" {
			r.BaseRole = nil
		} else {
			r.BaseRole = &trimmed
		}
	}
	valid := []string{AdminRoleName, BaseRoleUser, BaseRoleGuardian}
	if r.BaseRole != nil && !slices.Contains(valid, *r.BaseRole) {
		return fmt.Errorf("base_role must be one of %v", valid)
	}
	return nil
}
