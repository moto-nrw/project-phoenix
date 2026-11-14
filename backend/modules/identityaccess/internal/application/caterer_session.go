package application

import (
	"context"
	"fmt"
	"slices"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/internal/domain"
)

// The caterer's session (ADR 0077). A caterer belongs to no school: its
// token carries no tenant, its claims come from the caterer it belongs to,
// and its successor session follows the schools the caterer still supplies.
//
// auth.tokens and audit.auth_events both require a school, so a caterer's
// session row and its audit events are kept at a school it supplies: the
// one the presented session is kept at while that supply lives, else the
// first live one. That school is a storage key, never an access right; the
// token stays tenant-less.

// catererSessionSchool is the school a caterer's successor session is kept
// at: the current one while its supply is live, else the first live school
// the caterer supplies. A caterer without a live supplied school has
// nothing left to see (ADR 0077).
func (s *AccountAuthentication) catererSessionSchool(ctx context.Context, accountID int64, current domain.AccountSession) (int64, error) {
	_, schools, err := s.suppliedSchools(ctx, accountID)
	if err != nil {
		return 0, err
	}
	if len(schools) == 0 {
		return 0, identityaccess.ErrCatererWithoutSchool
	}
	if slices.Contains(schools, current.TenantID) {
		return current.TenantID, nil
	}
	return schools[0], nil
}

// suppliedSchools names the caterer of an account and the schools it
// supplies now. School liveness belongs to Organisation & Tenancy, so the
// linked schools are resolved in bulk.
func (s *AccountAuthentication) suppliedSchools(ctx context.Context, accountID int64) (domain.Caterer, []int64, error) {
	if s.caterers == nil {
		return domain.Caterer{}, nil, domain.ErrAccountNotCaterer
	}
	caterer, isCaterer, _, err := s.caterers.FindCatererOfAccount(ctx, accountID)
	if err != nil {
		return domain.Caterer{}, nil, fmt.Errorf("find caterer: %w", err)
	}
	if !isCaterer {
		return domain.Caterer{}, nil, domain.ErrAccountNotCaterer
	}
	schools, _, err := s.caterers.ListCatererSchools(ctx, caterer.ID)
	if err != nil {
		return domain.Caterer{}, nil, fmt.Errorf("list caterer schools: %w", err)
	}
	schools, err = s.liveSchoolIDs(ctx, schools)
	if err != nil {
		return domain.Caterer{}, nil, fmt.Errorf("resolve caterer schools: %w", err)
	}
	return caterer, schools, nil
}

// isCatererAccount reports whether the account belongs to a caterer, read
// through the global identity read: a caterer has no school to read in.
func (s *AccountAuthentication) isCatererAccount(ctx context.Context, accountID int64) (bool, error) {
	if s.caterers == nil {
		return false, nil
	}
	var isCaterer bool
	err := globalIdentityRead(ctx, s.runtime, func(readCtx context.Context) error {
		var err error
		_, isCaterer, _, err = s.caterers.FindCatererOfAccount(readCtx, accountID)
		return err
	})
	return isCaterer, err
}
