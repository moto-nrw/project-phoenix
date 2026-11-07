//go:build testhooks

package owners

import (
	"context"
	"time"

	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"

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

// GuardianMailBlocks reports no block: a fixture without mail has none.
func (UnsentGuardianMail) GuardianMailBlocks(context.Context, int64, []string) (map[string]string, error) {
	return nil, nil
}

// NoEmailOutcomes answers a fixture that reads no mail state: no mark and no mail.
type NoEmailOutcomes struct{}

func (NoEmailOutcomes) EmailOutcomes(context.Context, int64, deliveryContract.EmailOutcomeQuery) ([]deliveryContract.EmailOutcomeReport, error) {
	return nil, nil
}

func (NoEmailOutcomes) LatestEmailOutcomes(context.Context, int64, []string, []string) (map[string]deliveryContract.EmailOutcomeReport, error) {
	return nil, nil
}

func (NoEmailOutcomes) TraegerMailHolds(context.Context, int64, string) (bool, bool, error) {
	return false, false, nil
}

func (NoEmailOutcomes) TraegerMailHoldsOf(context.Context, int64, []string) (map[string]bool, map[string]bool, error) {
	return nil, nil, nil
}

func (NoEmailOutcomes) ProviderEmailBlocks(context.Context, []string) map[string]deliveryContract.EmailProviderBlock {
	return nil
}

func (NoEmailOutcomes) StoppedEmailAddresses(context.Context, int64, []string) (map[string]bool, error) {
	return nil, nil
}

func (NoEmailOutcomes) EmailAddressMarks(context.Context, int64, []string) (map[string]deliveryContract.EmailAddressMark, error) {
	return nil, nil
}

func (UnsentGuardianMail) EnqueueInvitationEmail(context.Context, identityaccess.GuardianInvitation, identityPorts.GuardianProfile, string) (int64, error) {
	return 0, nil
}

func (UnsentGuardianMail) EnqueueExistingAccountEmail(context.Context, identityPorts.GuardianProfile, string) (int64, bool, error) {
	return 0, false, nil
}

func (UnsentGuardianMail) EnqueueWelcomeEmail(context.Context, identityPorts.GuardianProfile, int64, string, int64) {
}
