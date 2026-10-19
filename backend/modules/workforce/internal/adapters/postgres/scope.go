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
		return errors.New("workforce: tenant is required")
	}
	return nil
}

func scopeQuery[Q interface {
	Where(string, ...any) Q
	Err(error) Q
}](scope TenantScope, query Q, predicate string) Q {
	if err := scope.Validate(); err != nil {
		return query.Err(err)
	}
	if scope.AcrossSchools {
		return query
	}
	return query.Where(predicate, scope.TenantID)
}
