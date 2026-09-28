package application

import (
	"context"
	"errors"
	"fmt"
)

const (
	parentPortalAccessAccount    = "account"
	parentPortalAccessInvitation = "invitation"
	parentPortalAccessContactOGS = "contact_ogs"
)

// PrimaryGuardianPortalAccess reports the useful next step for the primary
// guardian of the request behind a status token. The token authorizes this
// answer for the family's own status page; the result deliberately omits why
// an existing account is unavailable, so login and password-reset routes keep
// hiding account state (#3742).
func (s *ChangeRequests) PrimaryGuardianPortalAccess(ctx context.Context, token string) (string, error) {
	if s.deps.People.GuardianProfiles == nil {
		return "", errors.New("status: guardian profiles not configured")
	}
	req, tenantID, err := s.requestByToken(ctx, token)
	if err != nil {
		return "", err
	}
	access := parentPortalAccessInvitation
	// The profile reads are scoped by RLS only, so they run in the
	// request's tenant transaction rather than the token lookup's.
	if err := s.deps.Runtime.TenantTx(ctx, tenantID, func(txCtx context.Context) error {
		profile, err := s.primaryGuardianProfile(txCtx, req)
		if err != nil {
			return err
		}
		if profile == nil || profile.AccountID == nil {
			return nil
		}
		reachable, lookupErr := s.deps.People.GuardianProfiles.GuardianProfileHasActivePortalAccount(txCtx, profile.ID)
		if lookupErr != nil {
			return lookupErr
		}
		if reachable {
			access = parentPortalAccessAccount
		} else {
			// The profile is already linked to an account. An invitation cannot
			// safely restore it: existing accounts require their owner's access
			// token unless they are fully dormant. Let the OGS resolve it.
			access = parentPortalAccessContactOGS
		}
		return nil
	}); err != nil {
		return "", fmt.Errorf("status: load primary guardian profile: %w", err)
	}
	return access, nil
}
