package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// PasswordResetDelivery mails a reset link asynchronously. The root resolves
// the portal host for the scope and records the outcome through
// RecordPasswordResetDelivery once the send settles.
type PasswordResetDelivery interface {
	DispatchPasswordReset(ctx context.Context, link identityaccess.PasswordResetLink, recipient string, scope identityaccess.PasswordResetScope, expiry time.Duration) error
}
