package owners

import (
	"testing"

	"github.com/moto-ogs/project-phoenix/modules/carecontract"
	"github.com/moto-ogs/project-phoenix/modules/communication"
	deliveryContract "github.com/moto-ogs/project-phoenix/modules/delivery/contract"
	"github.com/moto-ogs/project-phoenix/modules/enrollment"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
	"github.com/stretchr/testify/assert"
)

// The suppression list holds back parent mail by template name: the names must
// stay those of the owners' mail kinds (#5932).
func TestParentMailKindsMatchTheOwners(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{
		identityaccess.MailKindGuardianInvitation, identityaccess.MailKindGuardianWelcome,
		communication.MailKindParentAnnouncement, communication.MailKindParentMessage,
		enrollment.MailKindSubmitted, enrollment.MailKindApproved, enrollment.MailKindWaitlisted, enrollment.MailKindRejected,
		enrollment.MailKindRolloverOptIn, enrollment.MailKindRolloverOptOut,
		enrollment.MailKindChangeRequestQuestion, enrollment.MailKindChangeRequestApproved, enrollment.MailKindChangeRequestRejected,
		enrollment.MailKindHolidayRegistrationReceived,
		carecontract.MailKindContract,
	} {
		assert.True(t, deliveryContract.IsParentMail(kind), kind)
	}
	for _, kind := range []string{parentbilling.MailKindBillingLetter, TransactionalMailKind, enrollment.MailKindAdminNotification, enrollment.MailKindDecisionDigest, identityaccess.MailKindStaffWelcome} {
		assert.False(t, deliveryContract.IsParentMail(kind), kind)
	}
}
