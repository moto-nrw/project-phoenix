//go:build testhooks

package owners

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityPorts "github.com/moto-nrw/project-phoenix/modules/identityaccess/ports"
)

// UnsentGuardianMail serves a root composed without the guardian mail — a
// CLI or a fixture that only reads. The flows still run and every mail
// reports success with outbox ID 0; nothing is sent.
type UnsentGuardianMail struct{}

func (UnsentGuardianMail) InvitationExpiry(context.Context) (time.Duration, error) {
	return identityaccess.GuardianInvitationExpiryFallback, nil
}

func (UnsentGuardianMail) SchoolName(context.Context, int64) string { return "" }

func (UnsentGuardianMail) EnqueueInvitationEmail(context.Context, identityaccess.GuardianInvitation, identityPorts.GuardianProfile, string) (int64, error) {
	return 0, nil
}

func (UnsentGuardianMail) EnqueueExistingAccountEmail(context.Context, identityPorts.GuardianProfile, string) (int64, bool, error) {
	return 0, false, nil
}

func (UnsentGuardianMail) EnqueueWelcomeEmail(context.Context, identityPorts.GuardianProfile, int64, string, int64) {
}
