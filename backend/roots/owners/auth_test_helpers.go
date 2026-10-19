//go:build testhooks

package owners

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityPorts "github.com/moto-nrw/project-phoenix/modules/identityaccess/ports"
)

// UnsentGuardianMail serves a root composed without the guardian mail — a
// CLI or a fixture that only reads. The flows still run; nothing is sent.
type UnsentGuardianMail struct{}

func (UnsentGuardianMail) InvitationExpiry(context.Context) time.Duration {
	return identityaccess.GuardianInvitationExpiryFallback
}

func (UnsentGuardianMail) SchoolName(context.Context, int64) string { return "" }

func (UnsentGuardianMail) EnqueueInvitationEmail(context.Context, identityaccess.GuardianInvitation, identityPorts.GuardianProfile, string) (int64, bool) {
	return 0, false
}

func (UnsentGuardianMail) EnqueueExistingAccountEmail(context.Context, identityPorts.GuardianProfile, string) (int64, bool) {
	return 0, false
}

func (UnsentGuardianMail) EnqueueWelcomeEmail(context.Context, identityPorts.GuardianProfile, int64, string, int64) {
}
