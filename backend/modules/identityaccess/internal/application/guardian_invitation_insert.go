package application

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

func (l *AccountLifecycle) insertStudentInvitation(ctx context.Context, req domain.InviteToStudentRequest, profile domain.GuardianProfile, tenantID int64, approvalStatus string, profileCreated, roleUpgrade bool) (domain.GuardianInvitation, error) {
	studentID := req.StudentID
	var invitation domain.GuardianInvitation
	err := l.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		var locked bool
		var lockErr error
		profile, locked, lockErr = l.guardians.LockGuardianProfile(txCtx, profile.ID)
		if lockErr != nil {
			return failed(opGuardianInviteToStudent, lockErr)
		}
		if !locked {
			return failed(opGuardianInviteToStudent, fmt.Errorf("guardian profile not found"))
		}
		if _, _, _, lockErr := l.logins.LockLoginAccountByEmail(txCtx, profile.Email); lockErr != nil {
			return failed(opGuardianInviteToStudent, lockErr)
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
