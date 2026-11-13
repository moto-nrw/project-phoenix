package application

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess/internal/domain"
)

func (l *AccountLifecycle) insertStudentInvitation(ctx context.Context, req domain.InviteToStudentRequest, profile domain.GuardianProfile, tenantID int64, approvalStatus string, profileCreated, roleUpgrade bool) (domain.GuardianInvitation, error) {
	studentID := req.StudentID
	var invitation domain.GuardianInvitation
	err := l.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		var err error
		profile, err = l.lockStudentInvitationProfile(txCtx, profile)
		if err != nil {
			return err
		}
		expiry, err := l.delivery.InvitationExpiry(txCtx)
		if err != nil {
			return failed(opGuardianInviteToStudent, err)
		}
		draft := domain.GuardianInvitation{
			TenantID:                    tenantID,
			GuardianProfileID:           profile.ID,
			CreatedBy:                   req.CreatedBy,
			ExpiresAt:                   time.Now().Add(expiry),
			StudentID:                   &studentID,
			RequestedByAccountID:        req.RequestedByParentAccountID,
			ApprovalStatus:              approvalStatus,
			ProfileCreatedForInvitation: profileCreated,
			RoleUpgrade:                 roleUpgrade,
		}
		// A queued approval has no usable mailed link yet. Issuing one does
		// supersede previous links, but not unrelated approval requests.
		if approvalStatus != domain.GuardianInvitationApprovalPending {
			if err := l.invitations.InvalidateGuardianInvitations(txCtx, profile.ID, time.Now(), false); err != nil {
				return failed(opGuardianInviteToStudent, err)
			}
		}
		issueGuardianInvitationLink(&draft, profile)
		var insertErr error
		invitation, insertErr = l.invitations.InsertGuardianInvitation(txCtx, draft)
		if insertErr != nil {
			return failed(opGuardianInviteToStudent, insertErr)
		}
		return nil
	})
	if err != nil {
		return domain.GuardianInvitation{}, err
	}
	return invitation, nil
}

func (l *AccountLifecycle) lockStudentInvitationProfile(ctx context.Context, profile domain.GuardianProfile) (domain.GuardianProfile, error) {
	// Account, school, profile, as acceptance locks them (40P01
	// otherwise); the address is checked again under the profile lock.
	if _, _, _, lockErr := l.logins.LockLoginAccountByEmail(ctx, profile.Email); lockErr != nil {
		return domain.GuardianProfile{}, failed(opGuardianInviteToStudent, lockErr)
	}
	if lockErr := l.lockInvitingSchool(ctx); lockErr != nil {
		return domain.GuardianProfile{}, failed(opGuardianInviteToStudent, lockErr)
	}
	readAddress := domain.GuardianInvitationAddress(profile.Email)
	var locked bool
	var lockErr error
	profile, locked, lockErr = l.guardians.LockGuardianProfile(ctx, profile.ID)
	if lockErr != nil {
		return domain.GuardianProfile{}, failed(opGuardianInviteToStudent, lockErr)
	}
	if !locked {
		return domain.GuardianProfile{}, failed(opGuardianInviteToStudent, fmt.Errorf("guardian profile not found"))
	}
	if domain.GuardianInvitationAddress(profile.Email) != readAddress {
		return domain.GuardianProfile{}, failed(opGuardianInviteToStudent, domain.ErrGuardianInvitationAddressChanged)
	}
	return profile, nil
}
