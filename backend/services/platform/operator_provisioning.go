package platform

import (
	"context"
	"errors"
	"net"

	"github.com/moto-nrw/project-phoenix/models/platform"
)

// The operator invitation and e-mail change flows belong to Identity &
// Access since #3332. What remains here is the envelope the operator routes
// classify on: they may not name the owner's contract, so the composition
// root translates its outcomes into the error shapes below and binds the
// flows through the consumer-owned ports.

// ErrOperatorInvitationsUnavailable reports a service composed without the
// invitation port.
var ErrOperatorInvitationsUnavailable = errors.New("operator invitations are not composed")

// ErrOperatorEmailChangeUnavailable reports a service composed without the
// e-mail change port.
var ErrOperatorEmailChangeUnavailable = errors.New("operator email change is not composed")

// clientAddress renders the request address for the operator ledger; an
// absent address is recorded as none, as it always was.
func clientAddress(clientIP net.IP) string {
	if clientIP == nil {
		return ""
	}
	return clientIP.String()
}

func (s *operatorAuthService) InviteOperator(ctx context.Context, inviteeEmail string, displayName *string, createdByID int64, clientIP net.IP) error {
	if s.Invitations == nil {
		return ErrOperatorInvitationsUnavailable
	}
	return s.Invitations.Invite(s.withTenantRuntime(ctx), inviteeEmail, displayName, createdByID, clientAddress(clientIP))
}

func (s *operatorAuthService) ValidateOperatorInvitation(ctx context.Context, tokenStr string) (*platform.OperatorInvitationToken, error) {
	if s.Invitations == nil {
		return nil, ErrOperatorInvitationsUnavailable
	}
	return s.Invitations.ValidateInvitation(s.withTenantRuntime(ctx), tokenStr)
}

func (s *operatorAuthService) AcceptOperatorInvitation(ctx context.Context, tokenStr, displayName, password string, clientIP net.IP) (*platform.Operator, error) {
	if s.Invitations == nil {
		return nil, ErrOperatorInvitationsUnavailable
	}
	return s.Invitations.AcceptInvitation(s.withTenantRuntime(ctx), tokenStr, displayName, password, clientAddress(clientIP))
}

func (s *operatorAuthService) ListPendingOperatorInvitations(ctx context.Context) ([]*platform.OperatorInvitationToken, error) {
	if s.Invitations == nil {
		return nil, nil
	}
	return s.Invitations.ListPendingInvitations(s.withTenantRuntime(ctx))
}

func (s *operatorAuthService) RevokeOperatorInvitation(ctx context.Context, invitationID int64, actorID int64, clientIP net.IP) error {
	if s.Invitations == nil {
		return ErrOperatorInvitationsUnavailable
	}
	return s.Invitations.RevokeInvitation(s.withTenantRuntime(ctx), invitationID, actorID, clientAddress(clientIP))
}

func (s *operatorAuthService) ResendOperatorInvitation(ctx context.Context, invitationID int64, actorID int64, clientIP net.IP) error {
	if s.Invitations == nil {
		return ErrOperatorInvitationsUnavailable
	}
	return s.Invitations.ResendInvitation(s.withTenantRuntime(ctx), invitationID, actorID, clientAddress(clientIP))
}

// CleanupExpiredOperatorInvitations removes the links nobody can spend. The
// owner deletes them; the scheduler drives it through here.
func (s *operatorAuthService) CleanupExpiredOperatorInvitations(ctx context.Context) (int, error) {
	if s.InvitationTokenRepo == nil {
		return 0, nil
	}
	return s.InvitationTokenRepo.DeleteExpired(s.withTenantRuntime(ctx))
}

func (s *operatorAuthService) InitiateEmailChange(ctx context.Context, operatorID int64, newEmail, currentPassword string, clientIP net.IP) error {
	if s.EmailChanges == nil {
		return ErrOperatorEmailChangeUnavailable
	}
	return s.EmailChanges.InitiateEmailChange(s.withTenantRuntime(ctx), operatorID, newEmail, currentPassword, clientAddress(clientIP))
}

func (s *operatorAuthService) ConfirmEmailChange(ctx context.Context, tokenStr string, clientIP net.IP) (string, error) {
	if s.EmailChanges == nil {
		return "", ErrOperatorEmailChangeUnavailable
	}
	return s.EmailChanges.ConfirmEmailChange(s.withTenantRuntime(ctx), tokenStr, clientAddress(clientIP))
}

func (s *operatorAuthService) CleanupExpiredEmailChangeTokens(ctx context.Context) (int, error) {
	if s.EmailChanges == nil {
		return 0, nil
	}
	return s.EmailChanges.CleanupEmailChangeTokens(s.withTenantRuntime(ctx))
}
