package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// OperatorInvitationDelivery mails the operator invitation link. The module
// decides what the link is worth and when it is sent; the root owns the
// transport, the portal host and the template. The dispatch is
// fire-and-forget: it is queued for after the caller's transaction and
// records its own outcome back through the module.
type OperatorInvitationDelivery interface {
	// DispatchOperatorInvitation mails the link. inviterName is the display
	// name shown in the mail, empty when the inviter could not be read.
	DispatchOperatorInvitation(ctx context.Context, invitation domain.OperatorInvitation, inviterName string)
}

// OperatorEmailChangeDelivery mails the three messages an operator e-mail
// change sends: the confirmation link to the new address, the heads-up to
// the current one, and the "it happened" notice to the address that was
// replaced. The last two are what lets the legitimate owner of a
// compromised account notice.
type OperatorEmailChangeDelivery interface {
	DispatchOperatorEmailChangeVerification(ctx context.Context, change domain.OperatorEmailChange)
	// DispatchOperatorEmailChangeRequested tells the current address that a
	// change to maskedNewEmail was requested.
	DispatchOperatorEmailChangeRequested(ctx context.Context, operator domain.Operator, maskedNewEmail string)
	// DispatchOperatorEmailChangeConfirmed tells the replaced address that
	// the change went through.
	DispatchOperatorEmailChangeConfirmed(ctx context.Context, oldEmail, displayName string)
}

// EmailFormat is the People Directory contact rule for a routable address.
// The module applies it rather than carrying a second copy of the pattern.
type EmailFormat interface {
	IsRoutable(address string) bool
}
