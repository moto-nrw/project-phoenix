package application

import (
	"context"
	"errors"
	"fmt"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
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
	var access string
	// The profile reads are scoped by RLS only, so they run in the
	// request's tenant transaction rather than the token lookup's.
	if err := s.deps.Runtime.TenantTx(ctx, tenantID, func(txCtx context.Context) error {
		var resolveErr error
		access, resolveErr = s.primaryGuardianPortalAccess(txCtx, req)
		return resolveErr
	}); err != nil {
		return "", fmt.Errorf("status: load primary guardian profile: %w", err)
	}
	return access, nil
}

func (s *ChangeRequests) primaryGuardianPortalAccess(ctx context.Context, req *enrollmentModels.Request) (string, error) {
	profile, err := s.primaryGuardianProfile(ctx, req)
	if err != nil {
		return "", err
	}
	if profile == nil {
		return parentPortalAccessInvitation, nil
	}
	if profile.AccountID == nil {
		return s.accountlessGuardianPortalAccess(ctx, profile.ID)
	}
	return s.accountLinkedGuardianPortalAccess(ctx, profile.ID)
}

func (s *ChangeRequests) accountlessGuardianPortalAccess(ctx context.Context, profileID int64) (string, error) {
	if s.deps.GuardianInvitations == nil {
		return "", errors.New("status: guardian invitations not configured")
	}
	redeemable, err := s.deps.GuardianInvitations.HasRedeemableGuardianInvitation(ctx, profileID)
	if err != nil {
		return "", err
	}
	if redeemable {
		return parentPortalAccessInvitation, nil
	}
	return parentPortalAccessContactOGS, nil
}

func (s *ChangeRequests) accountLinkedGuardianPortalAccess(ctx context.Context, profileID int64) (string, error) {
	reachable, err := s.deps.People.GuardianProfiles.GuardianProfileHasActivePortalAccount(ctx, profileID)
	if err != nil {
		return "", err
	}
	if reachable {
		return parentPortalAccessAccount, nil
	}
	// The profile is already linked to an account. An invitation cannot safely
	// restore it: existing accounts require their owner's access token unless
	// they are fully dormant. Let the OGS resolve it.
	return parentPortalAccessContactOGS, nil
}
