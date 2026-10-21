package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

func (s *SchoolInvitation) requireInvitationSchool(txCtx context.Context, invitation domain.SchoolInvitation) error {
	// A shared lock on the school serializes with the exclusive lock a
	// soft delete takes, so the school cannot vanish under the acceptance.
	if invitation.TenantID > 0 {
		school, found, schoolErr := s.schools.LockSchoolShared(txCtx, invitation.TenantID)
		if schoolErr != nil {
			return failed(opAcceptInvitation, schoolErr)
		}
		if !found || school.Deleted {
			return failed(opAcceptInvitation, domain.ErrInvitationTenantDeleted)
		}
	}
	return nil
}

func (s *SchoolInvitation) invitationPassword(txCtx context.Context, account domain.LoginAccount, hasAccount bool, registration domain.InvitationRegistration) (passwordHash string, restore bool, err error) {
	if hasAccount {
		dormant, dormantErr := s.accountDormant(txCtx, account)
		if dormantErr != nil {
			return passwordHash, restore, failed(opAcceptInvitation, dormantErr)
		}
		if dormant {
			hash, hashErr := s.hashPassword(registration)
			if hashErr != nil {
				return passwordHash, restore, hashErr
			}
			passwordHash, restore = hash, true
		} else if ownerErr := s.verifyOwner(account, registration.OwnerAccessToken); ownerErr != nil {
			return passwordHash, restore, failed(opAcceptInvitation, ownerErr)
		}
	} else {
		hash, hashErr := s.hashPassword(registration)
		if hashErr != nil {
			return passwordHash, restore, hashErr
		}
		passwordHash = hash
	}
	return passwordHash, restore, nil
}

func (s *SchoolInvitation) provisionInvitationIdentity(ctx context.Context, invitation domain.SchoolInvitation, accountID int64, firstName, lastName string) error {
	finishReset, err := s.prepareInvitationReset(ctx, invitation, accountID)
	if err != nil {
		return err
	}

	role, found, err := s.roles.FindRoleIgnoringTenant(ctx, invitation.RoleID)
	if err != nil {
		return failed("provision school identity", err)
	}
	if !found {
		return failed("provision school identity", domain.ErrRoleNotFound)
	}
	facts := *role.Facts()
	lehrkraft := s.policy.IsLehrkraftSystemRole(&facts)
	if err := s.assignCaregiverRole(ctx, accountID, invitation, facts, lehrkraft); err != nil {
		return err
	}
	var position string
	if invitation.Position != nil {
		position = *invitation.Position
	}
	identity, err := s.identity.EnsureSchoolIdentity(ctx, domain.SchoolIdentityInput{
		AccountID: accountID, TenantID: invitation.TenantID, Role: &facts,
		FirstName: firstName, LastName: lastName, Position: position,
		// The staff import created the person before the invitation went out;
		// accepting reuses it instead of filing a second one (#2600).
		PersonID:         invitation.PersonID,
		EmploymentAction: invitation.EmploymentAction,
		// A Lehrkraft never gets a caregiver profile, caregiver_enabled or
		// not — the same invariant the role assignment applies (#1772).
		CaregiverUpgrade: invitation.CaregiverEnabled && !lehrkraft,
		CreatePerson:     true,
	})
	if err != nil {
		return failed("provision school identity", err)
	}
	return finishReset(ctx, identity)
}
