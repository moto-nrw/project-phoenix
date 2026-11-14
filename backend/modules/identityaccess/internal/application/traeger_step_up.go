package application

import (
	"context"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/internal/domain"
)

// Code step in the running Träger session (#7068). A school that switched
// remembered devices off refuses an Absprung whose Träger session proved its
// factor only by a remembered device (#6955). Instead of signing out and in
// again, the person redeems an e-mailed code in the running session: the
// code is an org-scope challenge like the sign-in's, with its limits and
// mail, and the session rotates into a new family whose factor the code
// proved. The presented family is retired in the mint transaction, under the
// account lock refresh and logout take; the new family keeps its switch
// lineage, so the Träger logout still reaches the school sessions the old
// family opened.

// StartTraegerStepUp mails the code of a step for the account of the
// presented Träger session.
func (s *AccountAuthentication) StartTraegerStepUp(ctx context.Context, request identityaccess.TraegerStepUpRequest) (identityaccess.TraegerStepUpChallenge, error) {
	return runFlow(ctx, s.flow(), func(ctx context.Context) (identityaccess.TraegerStepUpChallenge, error) {
		account, err := s.traegerStepUpAccount(ctx, request)
		if err != nil {
			return identityaccess.TraegerStepUpChallenge{}, err
		}
		result, err := s.mfaChallengeResult(ctx, account, 0, domain.PortalOrg, request.IPAddress)
		if err != nil {
			return identityaccess.TraegerStepUpChallenge{}, err
		}
		return identityaccess.TraegerStepUpChallenge{ChallengeToken: result.ChallengeToken, MaskedEmail: result.MaskedEmail}, nil
	})
}

// CompleteTraegerStepUp rotates the presented Träger session into a family
// whose factor the redeemed code proved. The mint re-checks the code against
// the MFA reset generation, the membership and the presented family under
// the account lock.
func (s *AccountAuthentication) CompleteTraegerStepUp(ctx context.Context, request identityaccess.TraegerStepUpRequest, proof identityaccess.VerifiedMFAChallenge) (*identityaccess.TraegerLoginResult, error) {
	return runFlow(ctx, s.flow(), func(ctx context.Context) (*identityaccess.TraegerLoginResult, error) {
		if proof.Scope != identityaccess.MFAChallengeScopeOrg || proof.AccountID != request.AccountID {
			return nil, failed("Träger step-up", identityaccess.ErrInvalidToken)
		}
		account, err := s.traegerStepUpAccount(ctx, request)
		if err != nil {
			return nil, err
		}
		minted, err := s.mintAccountSession(ctx, mintRequest{
			portal: domain.PortalOrg, account: account, orgID: request.OrganizationID, proof: challengeProof(&proof),
			op: "Träger step-up",
			precondition: func(ctx context.Context, locked domain.LoginAccount) error {
				return s.requireLiveTraegerFamily(ctx, locked.ID, request.OrganizationID, request.FamilyID)
			},
			sourceFamilyID: request.FamilyID,
			retireFamilyID: request.FamilyID,
			audit:          s.traegerLoginAudit(ctx, account.ID, request.IPAddress, request.UserAgent),
		})
		if err != nil {
			return nil, err
		}
		return &identityaccess.TraegerLoginResult{
			Status: identityaccess.TraegerLoginAuthenticated, AccessToken: minted.accessToken, RefreshToken: minted.refreshToken,
			OrganizationID: request.OrganizationID,
		}, nil
	})
}

// traegerStepUpAccount checks the presented session before any code is sent
// or redeemed: a live account and its live Träger session at that Träger.
func (s *AccountAuthentication) traegerStepUpAccount(ctx context.Context, request identityaccess.TraegerStepUpRequest) (domain.LoginAccount, error) {
	if s.traeger == nil {
		return domain.LoginAccount{}, failed("Träger step-up", errTraegerMembershipsUnavailable)
	}
	if !s.mfa.Configured() {
		return domain.LoginAccount{}, failed("Träger step-up", identityaccess.ErrMFAStatusUnavailable)
	}
	if request.AccountID <= 0 || request.OrganizationID <= 0 || request.FamilyID == "" {
		return domain.LoginAccount{}, failed("Träger step-up", identityaccess.ErrInvalidToken)
	}
	account, err := s.authenticatedAccount(ctx, "Träger step-up", request.AccountID)
	if err != nil {
		return domain.LoginAccount{}, err
	}
	if err := globalIdentityRead(ctx, s.runtime, func(txCtx context.Context) error {
		return s.requireLiveTraegerFamily(txCtx, account.ID, request.OrganizationID, request.FamilyID)
	}); err != nil {
		return domain.LoginAccount{}, err
	}
	return account, nil
}

// requireLiveTraegerFamily refuses a family that is not the account's live,
// unrotated and verified Träger session at that Träger: a rotated, retired
// or foreign family cannot be stepped up.
func (s *AccountAuthentication) requireLiveTraegerFamily(ctx context.Context, accountID, organizationID int64, familyID string) error {
	verified, err := s.switchMFAProof(ctx, accountID, familyID, domain.PortalOrg.Name)
	if err != nil {
		return err
	}
	if !verified {
		return failed("Träger step-up", identityaccess.ErrInvalidToken)
	}
	source, err := s.sessions.LatestAccountSessionInFamily(ctx, familyID)
	if err != nil {
		return identityaccess.ErrMFAStatusUnavailable
	}
	if source.OrganizationID != organizationID {
		return failed("Träger step-up", identityaccess.ErrInvalidToken)
	}
	return nil
}
