package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// GuardianInvitationDelivery mails a guardian invitation: the token expiry
// the tenant configured, the school name for the mail and the outbox
// enqueue the worker dispatches from.
type GuardianInvitationDelivery interface {
	InvitationExpiry(ctx context.Context) (time.Duration, error)
	SchoolName(ctx context.Context, tenantID int64) string
	// EnqueueInvitationEmail returns the enqueue failure; the invitation
	// rolls back with its mail (#4602).
	EnqueueInvitationEmail(ctx context.Context, invitation identityaccess.GuardianInvitation, profile GuardianProfile, schoolName string) (outboxID int64, err error)
	// EnqueueExistingAccountEmail reports queued false for a contact without
	// an address and returns the enqueue failure (#4602).
	EnqueueExistingAccountEmail(ctx context.Context, profile GuardianProfile, schoolName string) (outboxID int64, queued bool, err error)
	// EnqueueWelcomeEmail queues the welcome that follows the first mail of
	// a new access (#3534), at most once per guardian and school.
	EnqueueWelcomeEmail(ctx context.Context, profile GuardianProfile, tenantID int64, schoolName string, precedingOutboxID int64)
}
