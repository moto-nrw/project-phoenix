package application

import (
	"context"
	"time"

	"github.com/gofrs/uuid"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

func (l *AccountLifecycle) insertStudentInvitation(ctx context.Context, req domain.InviteToStudentRequest, profile domain.GuardianProfile, tenantID int64, approvalStatus string, profileCreated, roleUpgrade bool) (domain.GuardianInvitation, error) {
	studentID := req.StudentID
	var invitation domain.GuardianInvitation
	err := l.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		if _, _, _, lockErr := l.logins.LockLoginAccountByEmail(txCtx, profile.Email); lockErr != nil {
			return failed(opGuardianInviteToStudent, lockErr)
		}
		var insertErr error
		invitation, insertErr = l.invitations.InsertGuardianInvitation(txCtx, domain.GuardianInvitation{
			TenantID:                    tenantID,
			Token:                       uuid.Must(uuid.NewV4()).String(),
			GuardianProfileID:           profile.ID,
			CreatedBy:                   req.CreatedBy,
			ExpiresAt:                   time.Now().Add(l.delivery.InvitationExpiry(txCtx)),
			StudentID:                   &studentID,
			RequestedByAccountID:        req.RequestedByParentAccountID,
			ApprovalStatus:              approvalStatus,
			ProfileCreatedForInvitation: profileCreated,
			RoleUpgrade:                 roleUpgrade,
		})
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
