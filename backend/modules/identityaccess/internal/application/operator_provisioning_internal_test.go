package application

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// The operator invitation and e-mail change flows this owner took over from
// the retained platform service (#3332). The cases below are the behaviour
// the retained tests pinned, asked of the owner.

// --- invitation ------------------------------------------------------------

func TestInviteOperatorStoresAndMailsTheLink(t *testing.T) {
	t.Parallel()

	f := newOperatorProvisioningFixture(t)
	inviter := f.seedInviter(1)
	displayName := "Neue Operatorin"

	require.NoError(t, f.provisioning.InviteOperator(context.Background(), domain.OperatorInvitationRequest{
		Email: "  Invitee@Example.COM  ", DisplayName: &displayName, CreatedBy: inviter.ID, IPAddress: "10.0.0.1",
	}))

	require.Len(t, f.invites.dispatched, 1)
	sent := f.invites.dispatched[0]
	// The address is normalized before it is stored, so the link and the
	// later uniqueness check agree on one spelling.
	assert.Equal(t, "invitee@example.com", sent.invitation.Email)
	assert.Equal(t, displayName, *sent.invitation.DisplayName)
	assert.NotEmpty(t, sent.invitation.Token)
	assert.Equal(t, inviter.DisplayName, sent.inviterName)

	entries := f.audit.actions(domain.OperatorAuditActionInvitationCreated)
	require.Len(t, entries, 1)
	assert.Equal(t, inviter.ID, entries[0].OperatorID)
	assert.Equal(t, domain.OperatorAuditResourceInvitation, entries[0].ResourceType)
	assert.Equal(t, "10.0.0.1", entries[0].IPAddress)
}

func TestInviteOperatorRefusesAnUnusableAddress(t *testing.T) {
	t.Parallel()

	for name, address := range map[string]string{
		"no domain": "not-an-email",
		"no tld":    "invitee@localhost",
		"empty":     "",
		"oversized": strings.Repeat("a", 250) + "@example.com",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := newOperatorProvisioningFixture(t)
			inviter := f.seedInviter(1)

			err := f.provisioning.InviteOperator(context.Background(), domain.OperatorInvitationRequest{
				Email: address, CreatedBy: inviter.ID,
			})
			var invalid *domain.InvalidInputError
			require.ErrorAs(t, err, &invalid)
			assert.Empty(t, f.invites.dispatched, "a refused invitation mails nothing")
			assert.Empty(t, f.tokens.recorded(), "a refused invitation writes nothing")
		})
	}
}

func TestInviteOperatorRefusesAnAddressThatIsAlreadyAnOperator(t *testing.T) {
	t.Parallel()

	f := newOperatorProvisioningFixture(t)
	inviter := f.seedInviter(1)
	f.seedOperatorWithEmail(2, "taken@example.com")

	err := f.provisioning.InviteOperator(context.Background(), domain.OperatorInvitationRequest{
		Email: "taken@example.com", CreatedBy: inviter.ID,
	})
	require.ErrorIs(t, err, domain.ErrOperatorEmailExists)
	assert.Empty(t, f.invites.dispatched)
}

// A new invitation spends the invitee's older links, so only the newest one
// can be redeemed.
func TestInviteOperatorSpendsThePreviousLinks(t *testing.T) {
	t.Parallel()

	f := newOperatorProvisioningFixture(t)
	inviter := f.seedInviter(1)
	stale := f.seedInvitation("invitee@example.com", "stale-token", inviter.ID)

	require.NoError(t, f.provisioning.InviteOperator(context.Background(), domain.OperatorInvitationRequest{
		Email: "invitee@example.com", CreatedBy: inviter.ID,
	}))

	_, err := f.provisioning.ValidateOperatorInvitation(context.Background(), stale.Token)
	require.ErrorIs(t, err, domain.ErrOperatorInvitationNotFound)
	require.Len(t, f.invites.dispatched, 1)
	_, err = f.provisioning.ValidateOperatorInvitation(context.Background(), f.invites.dispatched[0].invitation.Token)
	require.NoError(t, err)
}

// The rate limit is checked before anything is spent, so a blocked request
// never destroys the invitee's last usable link.
func TestInviteOperatorRateLimitRefusesWithoutSpendingTheExistingLink(t *testing.T) {
	t.Parallel()

	f := newOperatorProvisioningFixture(t)
	inviter := f.seedInviter(1)
	for i := 0; i < domain.OperatorInvitationRateLimit; i++ {
		f.seedInvitation("earlier@example.com", "token-"+string(rune('a'+i)), inviter.ID)
	}
	live := f.seedInvitation("invitee@example.com", "live-token", inviter.ID)

	err := f.provisioning.InviteOperator(context.Background(), domain.OperatorInvitationRequest{
		Email: "invitee@example.com", CreatedBy: inviter.ID,
	})
	require.ErrorIs(t, err, domain.ErrOperatorInvitationRateLimited)
	assert.Empty(t, f.invites.dispatched)

	preview, err := f.provisioning.ValidateOperatorInvitation(context.Background(), live.Token)
	require.NoError(t, err, "the refused request must leave the live link alone")
	assert.Equal(t, live.Email, preview.Email)
}

func TestInviteOperatorAllowsTheRequestBelowTheLimit(t *testing.T) {
	t.Parallel()

	f := newOperatorProvisioningFixture(t)
	inviter := f.seedInviter(1)
	for i := 0; i < domain.OperatorInvitationRateLimit-1; i++ {
		f.seedInvitation("earlier@example.com", "token-"+string(rune('a'+i)), inviter.ID)
	}

	require.NoError(t, f.provisioning.InviteOperator(context.Background(), domain.OperatorInvitationRequest{
		Email: "invitee@example.com", CreatedBy: inviter.ID,
	}))
	assert.Len(t, f.invites.dispatched, 1)
}

// The inviter's row is locked before the limit is counted: without it two
// parallel invitations would both read the same count and both pass.
func TestInviteOperatorLocksTheInviterBeforeCounting(t *testing.T) {
	t.Parallel()

	f := newOperatorProvisioningFixture(t)
	inviter := f.seedInviter(1)

	require.NoError(t, f.provisioning.InviteOperator(context.Background(), domain.OperatorInvitationRequest{
		Email: "invitee@example.com", CreatedBy: inviter.ID,
	}))

	assert.Contains(t, f.store.recorded(), "FindOperator(forUpdate)")
	calls := f.tokens.recorded()
	require.Contains(t, calls, "CountOperatorInvitationsCreatedAfter")
	assert.Less(t,
		indexOfCall(calls, "CountOperatorInvitationsCreatedAfter"),
		indexOfCall(calls, "RevokeOperatorInvitationsForEmail"),
		"the limit is counted before an existing link is spent")
}

func TestInviteOperatorReportsALockFailure(t *testing.T) {
	t.Parallel()

	f := newOperatorProvisioningFixture(t)
	f.seedInviter(1)
	f.store.findOperatorErr = nil

	err := f.provisioning.InviteOperator(context.Background(), domain.OperatorInvitationRequest{
		Email: "invitee@example.com", CreatedBy: 404,
	})
	require.Error(t, err)
	assert.Empty(t, f.invites.dispatched)
}

func TestInviteOperatorReportsACountFailure(t *testing.T) {
	t.Parallel()

	f := newOperatorProvisioningFixture(t)
	inviter := f.seedInviter(1)
	f.tokens.countInvitationsErr = errFakeStore

	err := f.provisioning.InviteOperator(context.Background(), domain.OperatorInvitationRequest{
		Email: "invitee@example.com", CreatedBy: inviter.ID,
	})
	require.ErrorIs(t, err, errFakeStore)
	assert.Empty(t, f.invites.dispatched)
}

// The ledger is evidence, not a gate: a failed append is logged and the
// invitation still goes out.
func TestInviteOperatorSurvivesAFailedAuditAppend(t *testing.T) {
	t.Parallel()

	f := newOperatorProvisioningFixture(t)
	inviter := f.seedInviter(1)
	f.audit.err = errFakeStore

	require.NoError(t, f.provisioning.InviteOperator(context.Background(), domain.OperatorInvitationRequest{
		Email: "invitee@example.com", CreatedBy: inviter.ID,
	}))
	assert.Len(t, f.invites.dispatched, 1)
}

func TestValidateOperatorInvitationRefusesASpentLink(t *testing.T) {
	t.Parallel()

	f := newOperatorProvisioningFixture(t)
	inviter := f.seedInviter(1)
	invitation := f.seedInvitation("invitee@example.com", "live-token", inviter.ID)

	preview, err := f.provisioning.ValidateOperatorInvitation(context.Background(), invitation.Token)
	require.NoError(t, err)
	assert.Equal(t, invitation.Email, preview.Email)

	require.NoError(t, f.provisioning.RevokeOperatorInvitation(context.Background(), invitation.ID, inviter.ID, "10.0.0.1"))

	_, err = f.provisioning.ValidateOperatorInvitation(context.Background(), invitation.Token)
	require.ErrorIs(t, err, domain.ErrOperatorInvitationNotFound)
}

func TestAcceptOperatorInvitationCreatesTheOperator(t *testing.T) {
	t.Parallel()

	f := newOperatorProvisioningFixture(t)
	inviter := f.seedInviter(1)
	invitation := f.seedInvitation("invitee@example.com", "live-token", inviter.ID)

	operator, err := f.provisioning.AcceptOperatorInvitation(context.Background(), domain.OperatorInvitationAcceptance{
		Token: invitation.Token, DisplayName: "  Neue Operatorin  ", Password: "StrongPass1!", IPAddress: "10.0.0.2",
	})
	require.NoError(t, err)
	assert.Equal(t, invitation.Email, operator.Email)
	assert.Equal(t, "Neue Operatorin", operator.DisplayName)
	assert.True(t, operator.Active)

	// The link is spent exactly once.
	_, err = f.provisioning.AcceptOperatorInvitation(context.Background(), domain.OperatorInvitationAcceptance{
		Token: invitation.Token, DisplayName: "Zweite", Password: "StrongPass1!",
	})
	require.ErrorIs(t, err, domain.ErrOperatorInvitationNotFound)

	entries := f.audit.actions(domain.OperatorAuditActionInvitationAccepted)
	require.Len(t, entries, 1)
	assert.Equal(t, operator.ID, entries[0].OperatorID)
}

// The token is spent before the password is hashed, so a bogus link cannot
// buy Argon2id work.
func TestAcceptOperatorInvitationSpendsTheTokenBeforeHashing(t *testing.T) {
	t.Parallel()

	f := newOperatorProvisioningFixture(t)

	_, err := f.provisioning.AcceptOperatorInvitation(context.Background(), domain.OperatorInvitationAcceptance{
		Token: "never-issued", DisplayName: "Niemand", Password: "StrongPass1!",
	})
	require.ErrorIs(t, err, domain.ErrOperatorInvitationNotFound)
	assert.Equal(t, []string{"RedeemOperatorInvitation"}, f.tokens.recorded())
}

func TestAcceptOperatorInvitationRefusesTheCallersMistakesFirst(t *testing.T) {
	t.Parallel()

	for name, acceptance := range map[string]domain.OperatorInvitationAcceptance{
		"weak password":   {Token: "live-token", DisplayName: "Operatorin", Password: "short"},
		"no display name": {Token: "live-token", DisplayName: "   ", Password: "StrongPass1!"},
		"oversized name":  {Token: "live-token", DisplayName: strings.Repeat("a", 101), Password: "StrongPass1!"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := newOperatorProvisioningFixture(t)
			inviter := f.seedInviter(1)
			invitation := f.seedInvitation("invitee@example.com", "live-token", inviter.ID)

			_, err := f.provisioning.AcceptOperatorInvitation(context.Background(), acceptance)
			var invalid *domain.InvalidInputError
			require.ErrorAs(t, err, &invalid)

			// The link survives a refused acceptance.
			_, validateErr := f.provisioning.ValidateOperatorInvitation(context.Background(), invitation.Token)
			require.NoError(t, validateErr)
		})
	}
}

func TestAcceptOperatorInvitationRefusesAnAddressTakenInTheMeantime(t *testing.T) {
	t.Parallel()

	f := newOperatorProvisioningFixture(t)
	inviter := f.seedInviter(1)
	invitation := f.seedInvitation("invitee@example.com", "live-token", inviter.ID)
	f.seedOperatorWithEmail(2, invitation.Email)

	_, err := f.provisioning.AcceptOperatorInvitation(context.Background(), domain.OperatorInvitationAcceptance{
		Token: invitation.Token, DisplayName: "Operatorin", Password: "StrongPass1!",
	})
	require.ErrorIs(t, err, domain.ErrOperatorEmailExists)
}

// A resend extends the link, restarts its delivery tracking and mails it
// again; an expired or spent link is never revived.
func TestResendOperatorInvitationExtendsAndRedispatches(t *testing.T) {
	t.Parallel()

	f := newOperatorProvisioningFixture(t)
	inviter := f.seedInviter(1)
	invitation := f.seedInvitation("invitee@example.com", "live-token", inviter.ID)
	_, err := f.tokens.RecordOperatorInvitationDelivery(context.Background(), invitation.ID, domain.TokenDelivery{RetryCount: 3})
	require.NoError(t, err)

	require.NoError(t, f.provisioning.ResendOperatorInvitation(context.Background(), invitation.ID, inviter.ID, "10.0.0.3"))

	require.Len(t, f.invites.dispatched, 1)
	sent := f.invites.dispatched[0].invitation
	assert.True(t, sent.ExpiresAt.After(invitation.ExpiresAt), "the link is extended")
	assert.Zero(t, sent.Delivery.RetryCount, "the mail the invitee gets now is the one the screens report on")

	entries := f.audit.actions(domain.OperatorAuditActionInvitationResent)
	require.Len(t, entries, 1)
	assert.Equal(t, inviter.ID, entries[0].OperatorID)
}

func TestResendOperatorInvitationRefusesASpentLink(t *testing.T) {
	t.Parallel()

	f := newOperatorProvisioningFixture(t)
	inviter := f.seedInviter(1)
	invitation := f.seedInvitation("invitee@example.com", "live-token", inviter.ID)
	require.NoError(t, f.provisioning.RevokeOperatorInvitation(context.Background(), invitation.ID, inviter.ID, ""))

	err := f.provisioning.ResendOperatorInvitation(context.Background(), invitation.ID, inviter.ID, "")
	require.ErrorIs(t, err, domain.ErrOperatorInvitationNotFound)
	assert.Empty(t, f.invites.dispatched)
}

func TestRevokeOperatorInvitationRefusesAnUnknownLink(t *testing.T) {
	t.Parallel()

	f := newOperatorProvisioningFixture(t)
	err := f.provisioning.RevokeOperatorInvitation(context.Background(), 404, 1, "")
	require.ErrorIs(t, err, domain.ErrOperatorInvitationNotFound)
	assert.Empty(t, f.audit.actions(domain.OperatorAuditActionInvitationRevoked))
}

func TestListPendingOperatorInvitationsSkipsTheSpentOnes(t *testing.T) {
	t.Parallel()

	f := newOperatorProvisioningFixture(t)
	inviter := f.seedInviter(1)
	live := f.seedInvitation("live@example.com", "live-token", inviter.ID)
	spent := f.seedInvitation("spent@example.com", "spent-token", inviter.ID)
	require.NoError(t, f.provisioning.RevokeOperatorInvitation(context.Background(), spent.ID, inviter.ID, ""))

	pending, err := f.provisioning.ListPendingOperatorInvitations(context.Background())
	require.NoError(t, err)
	require.Len(t, pending, 1)
	assert.Equal(t, live.ID, pending[0].ID)
}

// --- e-mail change ---------------------------------------------------------

func TestInitiateOperatorEmailChangeMailsTheLinkAndTheHeadsUp(t *testing.T) {
	t.Parallel()

	f := newOperatorProvisioningFixture(t)
	operator := f.seedOperatorWithEmail(1, "old@example.com")

	require.NoError(t, f.provisioning.InitiateOperatorEmailChange(context.Background(), domain.OperatorEmailChangeRequest{
		OperatorID: operator.ID, NewEmail: "  New@Example.COM  ", CurrentPassword: "secret", IPAddress: "10.0.0.4",
	}))

	require.Len(t, f.changes.verifications, 1)
	assert.Equal(t, "new@example.com", f.changes.verifications[0].NewEmail)
	// The current address learns that a change was requested, with the new
	// one masked: that is what lets the owner of a compromised account react.
	require.Len(t, f.changes.requested, 1)
	assert.Equal(t, "old@example.com -> n***@example.com", f.changes.requested[0])
	assert.Empty(t, f.changes.confirmed)

	entries := f.audit.actions(domain.OperatorAuditActionEmailChangeStarted)
	require.Len(t, entries, 1)
	require.NotNil(t, entries[0].EmailChange)
	assert.Equal(t, "n***@example.com", entries[0].EmailChange.MaskedNewEmail)
	assert.Empty(t, entries[0].EmailChange.MaskedOldEmail, "nothing has been replaced yet")
}

func TestInitiateOperatorEmailChangeRefusesAWrongPassword(t *testing.T) {
	t.Parallel()

	f := newOperatorProvisioningFixture(t)
	operator := f.seedOperatorWithEmail(1, "old@example.com")

	err := f.provisioning.InitiateOperatorEmailChange(context.Background(), domain.OperatorEmailChangeRequest{
		OperatorID: operator.ID, NewEmail: "new@example.com", CurrentPassword: "wrong",
	})
	require.ErrorIs(t, err, domain.ErrOperatorPasswordMismatch)
	assert.Empty(t, f.changes.verifications)
	assert.Empty(t, f.tokens.recorded(), "a refused request writes nothing")
}

// A password rotation that commits between the credential check and the
// link's transaction invalidates the request: the proof is stale.
func TestInitiateOperatorEmailChangeDetectsAConcurrentPasswordRotation(t *testing.T) {
	t.Parallel()

	f := newOperatorProvisioningFixture(t)
	operator := f.seedOperatorWithEmail(1, "old@example.com")
	f.operators.rotate = func() {
		rotated := f.store.operators[operator.ID]
		rotated.PasswordHash = "hash:rotated"
		f.store.operators[operator.ID] = rotated
	}

	err := f.provisioning.InitiateOperatorEmailChange(context.Background(), domain.OperatorEmailChangeRequest{
		OperatorID: operator.ID, NewEmail: "new@example.com", CurrentPassword: "secret",
	})
	require.ErrorIs(t, err, domain.ErrOperatorPasswordMismatch)
	assert.Empty(t, f.changes.verifications)
}

func TestInitiateOperatorEmailChangeRefusesTheCurrentAddress(t *testing.T) {
	t.Parallel()

	f := newOperatorProvisioningFixture(t)
	operator := f.seedOperatorWithEmail(1, "old@example.com")

	err := f.provisioning.InitiateOperatorEmailChange(context.Background(), domain.OperatorEmailChangeRequest{
		OperatorID: operator.ID, NewEmail: "OLD@example.com", CurrentPassword: "secret",
	})
	require.ErrorIs(t, err, domain.ErrOperatorEmailChangeSameEmail)
}

func TestInitiateOperatorEmailChangeRefusesAnInactiveOperator(t *testing.T) {
	t.Parallel()

	f := newOperatorProvisioningFixture(t)
	f.store.addOperator(1, "old@example.com", "hash:secret", false)

	err := f.provisioning.InitiateOperatorEmailChange(context.Background(), domain.OperatorEmailChangeRequest{
		OperatorID: 1, NewEmail: "new@example.com", CurrentPassword: "secret",
	})
	require.ErrorIs(t, err, domain.ErrOperatorInactive)
}

// An address another operator already holds is answered with success and no
// mail: reporting it would turn this endpoint into an operator directory.
// The rate limit runs first, so the observable behavior does not depend on
// whether the address exists.
func TestInitiateOperatorEmailChangeStaysSilentForATakenAddress(t *testing.T) {
	t.Parallel()

	f := newOperatorProvisioningFixture(t)
	operator := f.seedOperatorWithEmail(1, "old@example.com")
	f.seedOperatorWithEmail(2, "taken@example.com")

	require.NoError(t, f.provisioning.InitiateOperatorEmailChange(context.Background(), domain.OperatorEmailChangeRequest{
		OperatorID: operator.ID, NewEmail: "taken@example.com", CurrentPassword: "secret",
	}))

	assert.Empty(t, f.changes.verifications, "a taken address is never mailed a link")
	assert.Empty(t, f.changes.requested)
	assert.Empty(t, f.audit.actions(domain.OperatorAuditActionEmailChangeStarted))
	calls := f.tokens.recorded()
	assert.Contains(t, calls, "CountOperatorEmailChangesCreatedAfter", "the limit still ran")
	assert.NotContains(t, calls, "InsertOperatorEmailChange")
}

func TestInitiateOperatorEmailChangeRateLimitKeepsTheExistingLink(t *testing.T) {
	t.Parallel()

	f := newOperatorProvisioningFixture(t)
	operator := f.seedOperatorWithEmail(1, "old@example.com")
	for i := 0; i < domain.OperatorEmailChangeRateLimit; i++ {
		f.seedEmailChange(operator.ID, "earlier@example.com", "token-"+string(rune('a'+i)))
	}

	err := f.provisioning.InitiateOperatorEmailChange(context.Background(), domain.OperatorEmailChangeRequest{
		OperatorID: operator.ID, NewEmail: "new@example.com", CurrentPassword: "secret",
	})
	require.ErrorIs(t, err, domain.ErrOperatorEmailChangeRateLimited)
	assert.NotContains(t, f.tokens.recorded(), "RevokeOperatorEmailChanges",
		"a refused request must not spend the operator's existing link")
}

func TestInitiateOperatorEmailChangeSurvivesAFailedAuditAppend(t *testing.T) {
	t.Parallel()

	f := newOperatorProvisioningFixture(t)
	operator := f.seedOperatorWithEmail(1, "old@example.com")
	f.audit.err = errFakeStore

	require.NoError(t, f.provisioning.InitiateOperatorEmailChange(context.Background(), domain.OperatorEmailChangeRequest{
		OperatorID: operator.ID, NewEmail: "new@example.com", CurrentPassword: "secret",
	}))
	assert.Len(t, f.changes.verifications, 1)
	assert.Len(t, f.changes.requested, 1)
}

func TestConfirmOperatorEmailChangeWritesTheAddressAndNotifiesTheOldOne(t *testing.T) {
	t.Parallel()

	f := newOperatorProvisioningFixture(t)
	operator := f.seedOperatorWithEmail(1, "old@example.com")
	change := f.seedEmailChange(operator.ID, "new@example.com", "live-token")

	result, err := f.provisioning.ConfirmOperatorEmailChange(context.Background(), change.Token, "10.0.0.5")
	require.NoError(t, err)
	assert.Equal(t, "old@example.com", result.OldEmail)
	assert.Equal(t, "new@example.com", result.NewEmail)
	assert.Equal(t, "new@example.com", f.store.operators[operator.ID].Email)

	require.Len(t, f.changes.confirmed, 1)
	assert.Contains(t, f.changes.confirmed[0], "old@example.com")

	entries := f.audit.actions(domain.OperatorAuditActionEmailChangeDone)
	require.Len(t, entries, 1)
	require.NotNil(t, entries[0].EmailChange)
	assert.Equal(t, "o***@example.com", entries[0].EmailChange.MaskedOldEmail)
	assert.Equal(t, "n***@example.com", entries[0].EmailChange.MaskedNewEmail)
}

func TestConfirmOperatorEmailChangeSpendsTheLinkOnce(t *testing.T) {
	t.Parallel()

	f := newOperatorProvisioningFixture(t)
	operator := f.seedOperatorWithEmail(1, "old@example.com")
	change := f.seedEmailChange(operator.ID, "new@example.com", "live-token")

	_, err := f.provisioning.ConfirmOperatorEmailChange(context.Background(), change.Token, "")
	require.NoError(t, err)

	_, err = f.provisioning.ConfirmOperatorEmailChange(context.Background(), change.Token, "")
	require.ErrorIs(t, err, domain.ErrOperatorEmailChangeNotFound)
	assert.Len(t, f.changes.confirmed, 1)
}

func TestConfirmOperatorEmailChangeRefusesAnAddressTakenSinceTheRequest(t *testing.T) {
	t.Parallel()

	f := newOperatorProvisioningFixture(t)
	operator := f.seedOperatorWithEmail(1, "old@example.com")
	change := f.seedEmailChange(operator.ID, "new@example.com", "live-token")
	f.seedOperatorWithEmail(2, "new@example.com")

	_, err := f.provisioning.ConfirmOperatorEmailChange(context.Background(), change.Token, "")
	require.ErrorIs(t, err, domain.ErrOperatorEmailInUse)
	assert.Equal(t, "old@example.com", f.store.operators[operator.ID].Email)
	assert.Empty(t, f.changes.confirmed)
}

func TestConfirmOperatorEmailChangeRefusesAnInactiveOperator(t *testing.T) {
	t.Parallel()

	f := newOperatorProvisioningFixture(t)
	f.store.addOperator(1, "old@example.com", "hash:secret", false)
	change := f.seedEmailChange(1, "new@example.com", "live-token")

	_, err := f.provisioning.ConfirmOperatorEmailChange(context.Background(), change.Token, "")
	require.ErrorIs(t, err, domain.ErrOperatorInactive)
	assert.Equal(t, "old@example.com", f.store.operators[1].Email)
}

func TestCleanupOperatorEmailChangesSpendsExpiredLinksBeforeDeleting(t *testing.T) {
	t.Parallel()

	f := newOperatorProvisioningFixture(t)
	operator := f.seedOperatorWithEmail(1, "old@example.com")
	expired, _, err := f.tokens.InsertOperatorEmailChange(context.Background(), domain.OperatorEmailChange{
		OperatorID: operator.ID, NewEmail: "expired@example.com", Token: "expired-token",
		Expiry: time.Now().Add(-time.Hour),
	})
	require.NoError(t, err)

	_, err = f.provisioning.CleanupOperatorEmailChanges(context.Background())
	require.NoError(t, err)

	calls := f.tokens.recorded()
	assert.Less(t,
		indexOfCall(calls, "RevokeExpiredOperatorEmailChanges"),
		indexOfCall(calls, "DeleteStaleOperatorEmailChanges"),
		"expired links are spent first, so the operator can request a new one")
	assert.True(t, f.tokens.emailChanges[expired.ID].Used)
}

func TestCleanupOperatorEmailChangesReportsAFailedRevocation(t *testing.T) {
	t.Parallel()

	f := newOperatorProvisioningFixture(t)
	f.tokens.revokeExpiredErr = errFakeStore

	_, err := f.provisioning.CleanupOperatorEmailChanges(context.Background())
	require.ErrorIs(t, err, errFakeStore)
	assert.NotContains(t, f.tokens.recorded(), "DeleteStaleOperatorEmailChanges")
}

// indexOfCall reports where a statement appears in the recorded order, or a
// number past the end when it never ran.
func indexOfCall(calls []string, call string) int {
	for index, recorded := range calls {
		if recorded == call {
			return index
		}
	}
	return len(calls) + 1
}
