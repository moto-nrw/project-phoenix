package compose

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// GuardianInvitationDelivery mails a guardian invitation: the token expiry
// the tenant configured, the school name for the mail and the outbox
// enqueue the worker dispatches from.
type GuardianInvitationDelivery interface {
	InvitationExpiry(ctx context.Context) time.Duration
	SchoolName(ctx context.Context, tenantID int64) string
	EnqueueInvitationEmail(ctx context.Context, invitation identityaccess.GuardianInvitation, profile GuardianProfile, schoolName string)
	EnqueueExistingAccountEmail(ctx context.Context, profile GuardianProfile, schoolName string)
	// EnqueueWelcomeEmail queues the welcome that follows the first mail of
	// a new access (#3534), at most once per guardian and school.
	EnqueueWelcomeEmail(ctx context.Context, profile GuardianProfile, tenantID int64, schoolName string)
}

func (d guardianInvitationDelivery) EnqueueWelcomeEmail(ctx context.Context, profile domain.GuardianProfile, tenantID int64, schoolName string) {
	d.source.EnqueueWelcomeEmail(ctx, GuardianProfile(profile), tenantID, schoolName)
}
