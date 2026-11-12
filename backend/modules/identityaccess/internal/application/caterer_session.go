package application

import (
	"context"
	"fmt"
	"slices"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// The caterer's session (ADR 0077). A caterer belongs to no school: its
// token carries no tenant, its claims come from the caterer it belongs to
// (catererEligibility), and its successor session follows the schools the
// caterer still supplies.
//
// auth.tokens and audit.auth_events both require a school, so a caterer's
// session row and its audit events are kept at a school it supplies: the
// one the presented session is kept at while that supply lives, else the
// first live one. That school is a storage key, never an access right; the
// token stays tenant-less.

// lockCatererSessionSchool chooses and locks the successor's storage school
// under the account lock, before refresh locks any token. Revocation may have
// missed a first login in its account snapshot: it then holds the school while
// deleting the token, so taking these locks in reverse order would deadlock.
func (r *refreshRotation) lockCatererSessionSchool(ctx context.Context, current domain.AccountSession) error {
	schoolID, err := r.service.catererSessionSchool(ctx, r.account.ID, current)
	if err != nil {
		return err
	}
	if err := r.service.requireLiveSessionSchool(ctx, schoolID); err != nil {
		return err
	}
	// Keep the chosen school through persistence instead of resolving another
	// one after the token lock if the caterer's supplies change meanwhile.
	r.catererSchoolID = schoolID
	return nil
}

// catererSessionSchool is the school a caterer's successor session is kept
// at: the current one while its supply is live, else the first live school
// the caterer supplies. A caterer without a live supplied school has
// nothing left to see (ADR 0077).
func (s *AccountAuthentication) catererSessionSchool(ctx context.Context, accountID int64, current domain.AccountSession) (int64, error) {
	if s.caterers == nil {
		return 0, domain.ErrAccountNotCaterer
	}
	caterer, isCaterer, _, err := s.caterers.FindCatererOfAccount(ctx, accountID)
	if err != nil {
		return 0, fmt.Errorf("find caterer: %w", err)
	}
	if !isCaterer {
		return 0, domain.ErrAccountNotCaterer
	}
	schools, _, err := s.caterers.ListCatererSchools(ctx, caterer.ID)
	if err != nil {
		return 0, fmt.Errorf("list caterer schools: %w", err)
	}
	active, err := s.schools.ListActiveSchoolsByID(ctx, schools)
	if err != nil {
		return 0, fmt.Errorf("resolve caterer schools: %w", err)
	}
	live := make(map[int64]bool, len(active))
	for _, school := range active {
		live[school.ID] = true
	}
	schools = slices.DeleteFunc(schools, func(id int64) bool { return !live[id] })
	if len(schools) == 0 {
		return 0, identityaccess.ErrCatererWithoutSchool
	}
	if slices.Contains(schools, current.TenantID) {
		return current.TenantID, nil
	}
	return schools[0], nil
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
