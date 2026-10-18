package identityaccess_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// The school invitation accepts on the portal its login lives on (#2207) and
// shows the role under its German name.
func TestSchoolInvitationMailPicksThePortalAndTheRoleName(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "http://localhost:3000", identityaccess.SchoolInvitationOrigin(identityaccess.InvitationPortalTenant, "", "https://schule.example"))
	assert.Equal(t, "https://staff.example", identityaccess.SchoolInvitationOrigin(identityaccess.InvitationPortalTenant, "https://staff.example", "https://schule.example"))
	assert.Equal(t, "https://schule.example", identityaccess.SchoolInvitationOrigin(identityaccess.InvitationPortalSchool, "https://staff.example", "https://schule.example"))
	assert.Equal(t, "https://staff.example", identityaccess.SchoolInvitationOrigin(identityaccess.InvitationPortalSchool, "https://staff.example", ""))

	first := "Ada"
	invitation := identityaccess.SchoolInvitation{Token: "tok", RoleName: "USER", FirstName: &first}
	mail := identityaccess.SchoolInvitationMail(invitation, "OGS Am Berg", "https://staff.example", 48*time.Hour)
	assert.Equal(t, "Einladung zu moto – OGS Am Berg", mail.Subject)
	assert.Equal(t, "invitation.html", mail.Template)
	assert.Equal(t, "https://staff.example/invite?token=tok", mail.Content["InvitationURL"])
	assert.Equal(t, "Betreuer", mail.Content["RoleName"])
	assert.Equal(t, 48, mail.Content["ExpiryHours"])
	assert.Equal(t, "Einladung zu moto", identityaccess.SchoolInvitationMail(invitation, "", "o", time.Hour).Subject)
	assert.Equal(t, "Förderkraft", identityaccess.RoleDisplayName("Förderkraft"), "a school's own role keeps its name")
}

// A welcome is cancelled once its invitation can no longer be redeemed and
// waits for any other lookup outcome.
func TestStaffWelcomeCancelledOnlyForSpentInvitations(t *testing.T) {
	t.Parallel()

	for _, gone := range []error{identityaccess.ErrInvitationUsed, identityaccess.ErrInvitationExpired, identityaccess.ErrInvitationNotFound, identityaccess.ErrInvitationDeliveryFailed} {
		assert.True(t, identityaccess.StaffWelcomeCancelled(fmt.Errorf("lookup: %w", gone)), gone.Error())
	}
	assert.False(t, identityaccess.StaffWelcomeCancelled(fmt.Errorf("connection reset")))
	assert.False(t, identityaccess.StaffWelcomeCancelled(nil))

	_, _, err := identityaccess.StaffWelcomeMail(map[string]any{identityaccess.StaffWelcomePayloadRecipientEmail: "a@example.test"}, func() bool {
		t.Fatal("the reply identity is resolved for a complete payload only")
		return false
	})
	require.Error(t, err)
}

// The guardian token lifetime falls back from the school's setting to the
// deployment's and then to 48 hours; the mail never advertises zero hours.
func TestGuardianInvitationExpiryAndPayload(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 12*time.Hour, identityaccess.GuardianInvitationExpiry(12, 24*time.Hour))
	assert.Equal(t, 24*time.Hour, identityaccess.GuardianInvitationExpiry(0, 24*time.Hour))
	assert.Equal(t, 48*time.Hour, identityaccess.GuardianInvitationExpiry(0, 0))

	recipient := identityaccess.GuardianMailRecipient{FirstName: " Kim ", LastName: "Beispiel", Email: " kim@example.test "}
	payload := identityaccess.GuardianInvitationPayload(recipient, "", "tok", time.Now().Add(10*time.Minute), "")
	assert.Equal(t, 1, payload[identityaccess.GuardianPayloadExpiryHours])
	assert.Equal(t, "http://localhost:3000/accept-guardian-invite/tok", payload[identityaccess.GuardianPayloadInvitationURL])
	assert.Equal(t, "kim@example.test", payload[identityaccess.GuardianPayloadRecipientEmail])

	assert.Nil(t, identityaccess.GuardianExistingAccountPayload(identityaccess.GuardianMailRecipient{Email: "  "}, "", ""))
	existing := identityaccess.GuardianExistingAccountPayload(recipient, "https://eltern.example/", "OGS")
	assert.Equal(t, "https://eltern.example/login", existing[identityaccess.GuardianPayloadInvitationURL])
	mail, to, err := identityaccess.GuardianInvitationMail(existing)
	require.NoError(t, err)
	assert.Equal(t, "kim@example.test", to)
	assert.Equal(t, "Ihr Zugang zum Eltern-Portal – OGS", mail.Subject)
	assert.Equal(t, 48, mail.Content["ExpiryHours"], "a payload without a lifetime states the fallback")
}

// The guardian welcome names its predecessor, is sent once per contact and
// renders only after that predecessor was accepted.
func TestGuardianWelcomeFollowsItsPredecessor(t *testing.T) {
	t.Parallel()

	recipient := identityaccess.GuardianMailRecipient{Email: "kim@example.test"}
	_, err := identityaccess.GuardianWelcomePayload(recipient, "", "", "https://help", "https://info", 0)
	require.Error(t, err)
	blank, err := identityaccess.GuardianWelcomePayload(identityaccess.GuardianMailRecipient{}, "", "", "https://help", "https://info", 7)
	require.NoError(t, err)
	assert.Nil(t, blank)

	payload, err := identityaccess.GuardianWelcomePayload(recipient, "", "OGS", "https://help", "https://info", 7)
	require.NoError(t, err)
	assert.NotContains(t, payload, identityaccess.GuardianPayloadInvitationURL)
	id, err := identityaccess.GuardianWelcomePrecedingOutboxID(map[string]any{identityaccess.GuardianPayloadPrecedingOutboxID: float64(7)})
	require.NoError(t, err)
	assert.EqualValues(t, 7, id)
	_, err = identityaccess.GuardianWelcomePrecedingOutboxID(map[string]any{identityaccess.GuardianPayloadPrecedingOutboxID: 7.5})
	require.Error(t, err)
	assert.Equal(t, "guardian_welcome:9", identityaccess.GuardianWelcomeIdempotencyKey(9))

	assert.Equal(t, identityaccess.MailCancelled, identityaccess.GuardianWelcomeRelease(true, true))
	assert.Equal(t, identityaccess.MailDeferred, identityaccess.GuardianWelcomeRelease(false, false))
	assert.Equal(t, identityaccess.MailReleased, identityaccess.GuardianWelcomeRelease(true, false))

	mail, _, err := identityaccess.GuardianWelcomeMail(payload)
	require.NoError(t, err)
	assert.Equal(t, "Willkommen bei moto – OGS", mail.Subject)
	assert.Equal(t, "https://info", mail.Content["ParentInfoURL"])
}

// An operator invitation without a known inviter names "Ein Operator".
func TestOperatorInvitationMailNamesAFallbackInviter(t *testing.T) {
	t.Parallel()

	mail := identityaccess.OperatorInvitationMail(identityaccess.OperatorInvitation{Token: "tok"}, "", "https://moto.example", "https://operator.example", 72*time.Hour)
	assert.Equal(t, "Ein Operator", mail.Content["InviterName"])
	assert.Equal(t, "https://operator.example/invite?token=tok", mail.Content["InvitationURL"])
	assert.Equal(t, "https://moto.example/images/moto-logo-mit-schriftzug.png", mail.Content["LogoURL"])
	assert.Equal(t, 72, mail.Content["ExpiryHours"])
}

// The demo link mail carries nothing the public form typed.
func TestDemoAccessLinkMailCarriesNoFormData(t *testing.T) {
	t.Parallel()

	mail := identityaccess.DemoAccessLinkMail("https://demo.example/enter#t", "https://demo.example/")
	assert.Equal(t, map[string]any{"EntryURL": "https://demo.example/enter#t", "LogoURL": "https://demo.example/images/moto-logo-mit-schriftzug.png"}, mail.Content)
	lead := identityaccess.DemoLeadMail(identityaccess.DemoAccessMessage{FirstName: "Kim", LastName: "Beispiel", SchoolName: "OGS"}, "")
	assert.Equal(t, "Neuer Demo-Zugang: OGS", lead.Subject)
	assert.Equal(t, "Kim Beispiel", lead.Content["PersonName"])
}

// Only an invitation that needed no approval or was approved is redeemable.
func TestGuardianInvitationApprovalAllowsRedemption(t *testing.T) {
	t.Parallel()

	for status, want := range map[string]bool{
		identityaccess.GuardianInvitationApprovalNotRequired: true,
		identityaccess.GuardianInvitationApprovalApproved:    true,
		identityaccess.GuardianInvitationApprovalPending:     false,
		identityaccess.GuardianInvitationApprovalRejected:    false,
	} {
		assert.Equal(t, want, identityaccess.GuardianInvitation{ApprovalStatus: status}.ApprovalAllowsRedemption(), status)
	}
}
