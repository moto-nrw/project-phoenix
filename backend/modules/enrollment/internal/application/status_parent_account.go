package application

import (
	"context"
	"errors"
	"fmt"
)

// PrimaryGuardianHasPortalAccount reports whether the primary guardian of
// the request behind a status token has a parent-portal account at the
// request's school. The status page shows the way to the parent app only
// when one exists (#3742). The token is the authorization: the answer never
// leaves the family's own status page, so the login and password-reset
// routes keep hiding which addresses hold an account.
func (s *ChangeRequests) PrimaryGuardianHasPortalAccount(ctx context.Context, token string) (bool, error) {
	if s.deps.People.GuardianProfiles == nil {
		return false, errors.New("status: guardian profiles not configured")
	}
	req, tenantID, err := s.requestByToken(ctx, token)
	if err != nil {
		return false, err
	}
	var hasAccount bool
	// The profile reads are scoped by RLS only, so they run in the
	// request's tenant transaction rather than the token lookup's.
	if err := s.deps.Runtime.TenantTx(ctx, tenantID, func(txCtx context.Context) error {
		profile, err := s.primaryGuardianProfile(txCtx, req)
		if err != nil {
			return err
		}
		hasAccount = profile != nil && guardianHasPortalAccount(profile)
		return nil
	}); err != nil {
		return false, fmt.Errorf("status: load primary guardian profile: %w", err)
	}
	return hasAccount, nil
}
