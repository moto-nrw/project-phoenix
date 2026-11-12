package api

import (
	"context"
	"errors"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	"github.com/moto-ogs/project-phoenix/modules/organizationtenancy"
	"github.com/moto-ogs/project-phoenix/tenant"
)

// The Data Import's staff ports read two foreign owners; the root binds them
// (ADR 0059 point 6: cross-owner glue lives in the root).

// NewImportSchoolName binds the invitation label to the current school's
// owner query.
func NewImportSchoolName(schools organizationtenancy.Capability) func(context.Context) (string, error) {
	return func(ctx context.Context) (string, error) {
		id, err := tenant.TenantFromContext(ctx)
		if err != nil {
			return "", err
		}
		school, err := schools.FindSchool(ctx, id.Int64())
		if err != nil {
			return "", err
		}
		return school.Name, nil
	}
}

// NewImportSchoolAccountLookup exposes only the identity facts the duplicate
// matcher needs. A missing or inactive mapping is an ordinary absence, not a
// failure.
func NewImportSchoolAccountLookup(accounts identityaccess.SchoolAccountQuery) func(context.Context, string) (int64, bool, error) {
	return func(ctx context.Context, email string) (int64, bool, error) {
		account, err := accounts.FindSchoolAccountByEmail(ctx, email)
		if errors.Is(err, identityaccess.ErrAccountNotFound) {
			return 0, false, nil
		}
		if err != nil {
			return 0, false, err
		}
		return account.ID, true, nil
	}
}

// NewImportSchoolEmailLookup binds the separate batch query capability without
// widening the identity owner's login and command facade.
func NewImportSchoolEmailLookup(accounts identityaccess.SchoolAccountQuery) func(context.Context, []string) (map[string]int64, map[string][]int64, error) {
	queries := accounts.(identityaccess.SchoolEmailQuery)
	return queries.FindSchoolEmailIdentities
}
