package postgres

import "errors"

// TenantScope distinguishes one required school from an explicitly selected
// cross-school administrative transaction. A zero school is never a scope.
type TenantScope struct {
	TenantID      int64
	AcrossSchools bool
}

func (s TenantScope) Validate() error {
	if s.AcrossSchools {
		return nil
	}
	if s.TenantID <= 0 {
		return errors.New("peopledirectory: tenant is required")
	}
	return nil
}
