package application

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/gofrs/uuid"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// The staff decision on a parent-initiated guardian request: approving it
// links the child and mails the access, rejecting it closes the request. An
// approval whose mail cannot be queued rolls back with it (#4602).

func (l *AccountLifecycle) approveInvitation(ctx context.Context, invitationID, approverAccountID int64) error {
	invitation, err := l.pendingInvitation(ctx, opGuardianInviteApprove, invitationID)
	if err != nil {
		return err
	}

	profile, found, err := l.guardians.FindGuardianProfile(ctx, invitation.GuardianProfileID)
	if err != nil {
		return failed(opGuardianInviteApprove, err)
	}
	if !found {
		return failed(opGuardianInviteApprove, fmt.Errorf("guardian profile not found"))
	}
	if err := l.attachApprovalAccount(ctx, &profile); err != nil {
		return err
	}
	if err := l.approveInvitationStudentLink(ctx, invitation, profile); err != nil {
		return err
	}
	now := time.Now()
	invitation.ApprovalStatus = domain.GuardianInvitationApprovalApproved
	invitation.ApprovedBy = &approverAccountID
	invitation.ApprovedAt = &now

	// Existing account: access is granted by the link; close the invitation.
	if profile.HasAccount {
		return l.approveForExistingAccount(ctx, invitation, profile, approverAccountID, now)
	}

	// No account yet: refresh expiry, clear stale email tracking, dispatch.
	// The frozen link was never mailed and only its hash is stored (#4902):
	// the released mail carries a new one.
	expiry, err := l.delivery.InvitationExpiry(ctx)
	if err != nil {
		return failed(opGuardianInviteApprove, err)
	}
	invitation.Token = uuid.Must(uuid.NewV4()).String()
	invitation.ExpiresAt = now.Add(expiry)
	invitation.EmailSentAt = nil
	invitation.EmailError = nil
	if err := l.invitations.UpdateGuardianInvitation(ctx, invitation); err != nil {
		return failed(opGuardianInviteApprove, err)
	}
	l.logger.Info("guardian invitation approved",
		slog.Int64("invitation_id", invitation.ID),
		slog.Int64("approver_account_id", approverAccountID),
	)
	if err := l.mailNewInvitation(ctx, invitation, profile); err != nil {
		return failed(opGuardianInviteApprove, err)
	}
	return nil
}

// approveForExistingAccount closes the approved invitation of an account
// holder and tells them about the access; the approval rolls back when the
// mail cannot be queued (#4602).
func (l *AccountLifecycle) approveForExistingAccount(ctx context.Context, invitation domain.GuardianInvitation, profile domain.GuardianProfile, approverAccountID int64, now time.Time) error {
	invitation.AcceptedAt = &now
	if err := l.invitations.UpdateGuardianInvitation(ctx, invitation); err != nil {
		return failed(opGuardianInviteApprove, err)
	}
	l.logger.Info("guardian invitation approved (existing account)",
		slog.Int64("invitation_id", invitation.ID),
		slog.Int64("approver_account_id", approverAccountID),
	)
	if err := l.mailExistingAccountAccess(ctx, profile, invitation.TenantID); err != nil {
		return failed(opGuardianInviteApprove, err)
	}
	return nil
}

func (l *AccountLifecycle) rejectInvitation(ctx context.Context, invitationID, approverAccountID int64) error {
	invitation, err := l.pendingInvitation(ctx, opGuardianInviteReject, invitationID)
	if err != nil {
		return err
	}
	now := time.Now()
	invitation.ApprovalStatus = domain.GuardianInvitationApprovalRejected
	invitation.ApprovedBy = &approverAccountID
	invitation.ApprovedAt = &now
	if err := l.invitations.UpdateGuardianInvitation(ctx, invitation); err != nil {
		return failed(opGuardianInviteReject, err)
	}

	l.cleanupOrphanProfile(ctx, invitation)

	l.logger.Info("guardian invitation rejected",
		slog.Int64("invitation_id", invitation.ID),
		slog.Int64("approver_account_id", approverAccountID),
	)
	return nil
}
