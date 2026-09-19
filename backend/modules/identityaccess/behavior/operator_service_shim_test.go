package behavior_test

import (
	"context"
	"net"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// The operator behaviour suites were written against the retained operator
// service's call shapes: a net.IP client address and one argument per field.
// The flows are the module's since #3252 and #3332; the shim below keeps the
// suites driving them without rewriting every call site (#3364).
type operatorServiceShim struct{ module *identityaccess.Module }

func clientAddressOf(clientIP net.IP) string {
	if clientIP == nil {
		return ""
	}
	return clientIP.String()
}

func (s operatorServiceShim) InitiateEmailChange(ctx context.Context, operatorID int64, newEmail, currentPassword string, clientIP net.IP) error {
	return s.module.InitiateOperatorEmailChange(ctx, identityaccess.OperatorEmailChangeRequest{
		OperatorID: operatorID, NewEmail: newEmail, CurrentPassword: currentPassword,
		IPAddress: clientAddressOf(clientIP),
	})
}

func (s operatorServiceShim) ConfirmEmailChange(ctx context.Context, token string, clientIP net.IP) (string, error) {
	result, err := s.module.ConfirmOperatorEmailChange(ctx, token, clientAddressOf(clientIP))
	return result.NewEmail, err
}

func (s operatorServiceShim) CleanupExpiredEmailChangeTokens(ctx context.Context) (int, error) {
	return s.module.CleanupOperatorEmailChanges(ctx)
}

func (s operatorServiceShim) InviteOperator(ctx context.Context, email string, displayName *string, createdByID int64, clientIP net.IP) error {
	return s.module.InviteOperator(ctx, identityaccess.OperatorInvitationRequest{
		Email: email, DisplayName: displayName, CreatedBy: createdByID, IPAddress: clientAddressOf(clientIP),
	})
}

func (s operatorServiceShim) ValidateOperatorInvitation(ctx context.Context, token string) (identityaccess.OperatorInvitationPreview, error) {
	return s.module.ValidateOperatorInvitation(ctx, token)
}

func (s operatorServiceShim) AcceptOperatorInvitation(ctx context.Context, token, displayName, password string, clientIP net.IP) (identityaccess.Operator, error) {
	return s.module.AcceptOperatorInvitation(ctx, identityaccess.OperatorInvitationAcceptance{
		Token: token, DisplayName: displayName, Password: password, IPAddress: clientAddressOf(clientIP),
	})
}

func (s operatorServiceShim) ListPendingOperatorInvitations(ctx context.Context) ([]identityaccess.OperatorInvitation, error) {
	return s.module.ListPendingOperatorInvitations(ctx)
}

func (s operatorServiceShim) RevokeOperatorInvitation(ctx context.Context, invitationID, actorID int64, clientIP net.IP) error {
	return s.module.RevokeOperatorInvitationByActor(ctx, invitationID, actorID, clientAddressOf(clientIP))
}

func (s operatorServiceShim) ResendOperatorInvitation(ctx context.Context, invitationID, actorID int64, clientIP net.IP) error {
	return s.module.ResendOperatorInvitationByActor(ctx, invitationID, actorID, clientAddressOf(clientIP))
}
