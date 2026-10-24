package postgres

import (
	"errors"
	"fmt"

	"github.com/uptrace/bun"
)

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

// ResolveDatabase validates the context scope and resolves only an existing
// SQL transaction. Context interpretation stays in the owner's composition.
func ResolveDatabase(raw any, present bool, schoolID int64, schoolErr error, admin bool) (bun.IDB, TenantScope, error) {
	scope := TenantScope{TenantID: schoolID}
	if schoolErr != nil {
		if !admin {
			return nil, TenantScope{}, schoolErr
		}
		scope.AcrossSchools = true
	}
	if err := scope.Validate(); err != nil {
		return nil, TenantScope{}, err
	}
	if !present {
		return nil, TenantScope{}, errors.New("transaction is required")
	}
	switch tx := raw.(type) {
	case bun.Tx:
		return tx, scope, nil
	case *bun.Tx:
		if tx != nil {
			return *tx, scope, nil
		}
	}
	return nil, TenantScope{}, fmt.Errorf("unsupported transaction %T", raw)
}
