package application

import (
	"context"
	"time"

	"github.com/gofrs/uuid"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

func (l *AccountLifecycle) insertStudentInvitation(ctx context.Context, req domain.InviteToStudentRequest, profile domain.GuardianProfile, tenantID int64, approvalStatus string, profileCreated, roleUpgrade bool) (domain.GuardianInvitation, error) {
	studentID := req.StudentID
	invitation, err := l.invitations.InsertGuardianInvitation(ctx, domain.GuardianInvitation{
		TenantID:                    tenantID,
		Token:                       uuid.Must(uuid.NewV4()).String(),
		GuardianProfileID:           profile.ID,
		CreatedBy:                   req.CreatedBy,
		ExpiresAt:                   time.Now().Add(l.delivery.InvitationExpiry(ctx)),
		StudentID:                   &studentID,
		RequestedByAccountID:        req.RequestedByParentAccountID,
		ApprovalStatus:              approvalStatus,
		ProfileCreatedForInvitation: profileCreated,
		RoleUpgrade:                 roleUpgrade,
	})
	if err != nil {
		return domain.GuardianInvitation{}, failed(opGuardianInviteToStudent, err)
	}
	return invitation, nil
}
