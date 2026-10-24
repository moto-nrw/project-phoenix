package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// OperatorInvitationDelivery mails the invitation link. The module decides
// what the link is worth and when it goes out; the root owns the transport,
// the portal host and the template, and records the outcome back through
// the module.
type OperatorInvitationDelivery interface {
	DispatchOperatorInvitation(ctx context.Context, invitation identityaccess.OperatorInvitation, inviterName string, expiry time.Duration) error
}

// OperatorEmailChangeDelivery mails the three messages an e-mail change
// sends: the confirmation link to the new address, the heads-up to the
// current one and the notice to the address that was replaced.
type OperatorEmailChangeDelivery interface {
	DispatchOperatorEmailChangeVerification(ctx context.Context, change identityaccess.OperatorEmailChange, expiry time.Duration) error
	DispatchOperatorEmailChangeRequested(ctx context.Context, operator identityaccess.Operator, maskedNewEmail string) error
	DispatchOperatorEmailChangeConfirmed(ctx context.Context, oldEmail, displayName string) error
}

// EmailFormat is the People Directory contact rule for a routable address.
type EmailFormat interface {
	IsRoutable(address string) bool
}
