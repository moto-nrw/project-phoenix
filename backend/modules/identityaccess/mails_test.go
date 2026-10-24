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
	assert.Equal(t, "https://staff.example/invite?token=tok", content(t, mail)["InvitationURL"])
	assert.Equal(t, "Betreuung", content(t, mail)["RoleName"])
	assert.Equal(t, 48, content(t, mail)["ExpiryHours"])
	assert.Equal(t, "Einladung zu moto", identityaccess.SchoolInvitationMail(invitation, "", "o", time.Hour).Subject)
	assert.Equal(t, "Förderkraft", identityaccess.RoleDisplayName("Förderkraft"), "a school's own role keeps its name")
	assert.Equal(t, "AG-Leitung", identityaccess.RoleDisplayName(identityaccess.AGLeitungRoleName))
	assert.Equal(t, "Gast", identityaccess.RoleDisplayName(identityaccess.GuestRoleName))
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

	_, err := identityaccess.StaffWelcomeMail(identityaccess.StaffWelcome{Recipient: "a@example.test"}, func() bool {
		t.Fatal("the reply identity is resolved for a complete payload only")
		return false
	})
	require.Error(t, err)
}

// The guardian token lifetime falls back from the school's setting to the
// deployment's and then to 48 hours; the mail never advertises zero hours.
func TestGuardianInvitationExpiryAndMail(t *testing.T) {
	t.Parallel()

	expiry, err := identityaccess.GuardianInvitationExpiry(12)
	require.NoError(t, err)
	assert.Equal(t, 12*time.Hour, expiry)
	_, err = identityaccess.GuardianInvitationExpiry(0)
	require.Error(t, err)

	recipient := identityaccess.GuardianMailRecipient{FirstName: " Kim ", LastName: "Beispiel", Email: " kim@example.test "}
	invitation := identityaccess.GuardianInvitationMailOf(recipient, "", "tok", time.Now().Add(10*time.Minute), "")
	assert.Equal(t, 1, invitation.ExpiryHours)
	assert.Equal(t, "http://localhost:3000/accept-guardian-invite/tok", invitation.InvitationURL)
	assert.Equal(t, "kim@example.test", invitation.Recipient)

	_, addressed := identityaccess.GuardianExistingAccountMailOf(identityaccess.GuardianMailRecipient{Email: "  "}, "", "")
	assert.False(t, addressed)
	existing, addressed := identityaccess.GuardianExistingAccountMailOf(recipient, "https://eltern.example/", "OGS")
	require.True(t, addressed)
	assert.Equal(t, "https://eltern.example/login", existing.InvitationURL)
	mail, err := identityaccess.GuardianInvitationMail(existing)
	require.NoError(t, err)
	assert.Equal(t, "Ihr Zugang zum Eltern-Portal – OGS", mail.Subject)
	assert.Equal(t, 48, content(t, mail)["ExpiryHours"], "a mail without a lifetime states the fallback")
	_, err = identityaccess.GuardianInvitationMail(identityaccess.GuardianMail{Recipient: "kim@example.test"})
	require.Error(t, err, "an invitation needs its link")
}

// The guardian welcome names its predecessor, is sent once per contact and
// renders only after that predecessor was accepted.
func TestGuardianWelcomeFollowsItsPredecessor(t *testing.T) {
	t.Parallel()

	recipient := identityaccess.GuardianMailRecipient{Email: "kim@example.test"}
	_, _, err := identityaccess.GuardianWelcomeMailOf(recipient, "", "", "https://help", "https://info", 0)
	require.Error(t, err)
	_, addressed, err := identityaccess.GuardianWelcomeMailOf(identityaccess.GuardianMailRecipient{}, "", "", "https://help", "https://info", 7)
	require.NoError(t, err)
	assert.False(t, addressed)

	welcome, addressed, err := identityaccess.GuardianWelcomeMailOf(recipient, "", "OGS", "https://help", "https://info", 7)
	require.NoError(t, err)
	require.True(t, addressed)
	assert.Empty(t, welcome.InvitationURL, "the welcome carries no accept link")
	assert.EqualValues(t, 7, welcome.PrecedingOutboxID)
	assert.Equal(t, "guardian_welcome:9", identityaccess.GuardianWelcomeIdempotencyKey(9))

	assert.Equal(t, identityaccess.MailCancelled, identityaccess.GuardianWelcomeRelease(true, true))
	assert.Equal(t, identityaccess.MailDeferred, identityaccess.GuardianWelcomeRelease(false, false))
	assert.Equal(t, identityaccess.MailReleased, identityaccess.GuardianWelcomeRelease(true, false))

	mail, err := identityaccess.GuardianWelcomeMail(welcome)
	require.NoError(t, err)
	assert.Equal(t, "Willkommen bei moto – OGS", mail.Subject)
	assert.Equal(t, "https://info", content(t, mail)["ParentInfoURL"])
}

func content(t *testing.T, mail identityaccess.Mail) map[string]any {
	t.Helper()
	values, ok := mail.Content.(map[string]any)
	require.True(t, ok, "mail content is a template value map")
	return values
}

// An operator invitation without a known inviter names "Ein Operator".
func TestOperatorInvitationMailNamesAFallbackInviter(t *testing.T) {
	t.Parallel()

	mail := identityaccess.OperatorInvitationMail(identityaccess.OperatorInvitation{Token: "tok"}, "", "https://moto.example", "https://operator.example", 72*time.Hour)
	assert.Equal(t, "Ein Operator", content(t, mail)["InviterName"])
	assert.Equal(t, "https://operator.example/invite?token=tok", content(t, mail)["InvitationURL"])
	assert.Equal(t, "https://moto.example/images/moto-logo-mit-schriftzug.png", content(t, mail)["LogoURL"])
	assert.Equal(t, 72, content(t, mail)["ExpiryHours"])
}

// The demo link mail carries nothing the public form typed.
func TestDemoAccessLinkMailCarriesNoFormData(t *testing.T) {
	t.Parallel()

	mail := identityaccess.DemoAccessLinkMail("https://demo.example/enter#t", "https://demo.example/")
	assert.Equal(t, map[string]any{"EntryURL": "https://demo.example/enter#t", "LogoURL": "https://demo.example/images/moto-logo-mit-schriftzug.png"}, content(t, mail))
	lead := identityaccess.DemoLeadMail(identityaccess.DemoAccessMessage{FirstName: "Kim", LastName: "Beispiel", SchoolName: "OGS"}, "")
	assert.Equal(t, "Neuer Demo-Zugang: OGS", lead.Subject)
	assert.Equal(t, "Kim Beispiel", content(t, lead)["PersonName"])
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

// An operator e-mail change link lives as long as a reset link, but never
// shorter than 15 minutes.
func TestOperatorEmailChangeExpiryFloorsAtFifteenMinutes(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 30*time.Minute, identityaccess.OperatorEmailChangeExpiry(30*time.Minute))
	assert.Equal(t, 15*time.Minute, identityaccess.OperatorEmailChangeExpiry(15*time.Minute))
	assert.Equal(t, 15*time.Minute, identityaccess.OperatorEmailChangeExpiry(5*time.Minute))
}

// Provisioning matches a platform system role by name in any case and never
// a school's own role of the same name; a username suffix is six characters,
// or empty when the random source fails.
func TestProvisioningRoleAndUsernameRules(t *testing.T) {
	t.Parallel()

	school := int64(77)
	roles := []identityaccess.Role{
		{ID: 1, Name: "Admin", TenantID: &school, IsSystem: true},
		{ID: 2, Name: "admin", IsSystem: false},
		{ID: 3, Name: "ADMIN", IsSystem: true},
	}
	role, found := identityaccess.PlatformSystemRole(roles, "admin")
	require.True(t, found)
	assert.EqualValues(t, 3, role.ID)
	_, found = identityaccess.PlatformSystemRole(roles, "lehrkraft")
	assert.False(t, found)

	var asked int
	suffix := identityaccess.ProvisioningUsernameSuffix(func(length int) (string, error) {
		asked = length
		return "abc123", nil
	})
	assert.Equal(t, 6, asked)
	assert.Equal(t, "abc123", suffix)
	assert.Empty(t, identityaccess.ProvisioningUsernameSuffix(func(int) (string, error) { return "partial", fmt.Errorf("no entropy") }))
}

// An audit names the account by its person, or by its username without one.
func TestAuditActorName(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "Ada Lovelace", identityaccess.AuditActorName(" Ada", "Lovelace ", "ada"))
	assert.Equal(t, "ada", identityaccess.AuditActorName(" ", "", " ada "))
	assert.Equal(t, "Ada", identityaccess.AuditActorName("Ada", "", "ada"))
}
