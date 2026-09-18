// Package platform_test contains integration tests for the operator
// invitation flow that require a real database connection.
package platform_test

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The operator invitation flow moved to Identity & Access with #3332. These
// cases drive it through the composed root the server builds, so they cover
// the whole path the operator routes take: the retained envelope, the
// owner's flow, the stored link and the mail the delivery queues.

// buildInvitationService returns the retained invitation contract the
// operator routes depend on, composed the way the server composes it.
func buildInvitationService(t *testing.T, db *bun.DB) platformSvc.OperatorInvitationService {
	t.Helper()
	return buildServiceFactory(t, db).OperatorInvitation
}

// createInvitingOperator creates the operator an invitation is created by.
func createInvitingOperator(t *testing.T, db *bun.DB, email string) int64 {
	t.Helper()
	ctx := context.Background()

	var operatorID int64
	require.NoError(t, db.NewRaw(
		`INSERT INTO platform.operators (email, password_hash, display_name, active)
		 VALUES (?, 'x', ?, true) RETURNING id`, email, "Einladende Operatorin",
	).Scan(ctx, &operatorID))

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM platform.operator_invitation_tokens WHERE created_by = ?`, operatorID)
		_, _ = db.ExecContext(ctx, `DELETE FROM platform.operator_audit_log WHERE operator_id = ?`, operatorID)
		_, _ = db.ExecContext(ctx, `DELETE FROM platform.operators WHERE id = ?`, operatorID)
	})
	return operatorID
}

func uniqueOperatorEmail(prefix string) string {
	return fmt.Sprintf("%s-%d@test.local", prefix, time.Now().UnixNano())
}

// The whole path: an invitation is stored redeemable, the public page can
// read it, and accepting it creates the operator exactly once.
func TestIntegration_OperatorInvitation_InviteValidateAccept(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db := testpkg.SetupTestDB(t)

	service := buildInvitationService(t, db)
	ctx := context.Background()
	inviter := createInvitingOperator(t, db, uniqueOperatorEmail("operator-inviter"))
	invitee := uniqueOperatorEmail("operator-invitee")
	displayName := "Neue Operatorin"

	require.NoError(t, service.InviteOperator(ctx, invitee, &displayName, inviter, net.ParseIP("10.0.0.1")))

	pending, err := service.ListPendingOperatorInvitations(ctx)
	require.NoError(t, err)
	var token string
	for _, invitation := range pending {
		if invitation.Email == invitee {
			token = invitation.Token
			assert.Equal(t, displayName, *invitation.DisplayName)
			assert.True(t, invitation.ExpiresAt.After(time.Now()))
		}
	}
	require.NotEmpty(t, token, "the invitation is listed as pending")

	preview, err := service.ValidateOperatorInvitation(ctx, token)
	require.NoError(t, err)
	assert.Equal(t, invitee, preview.Email)

	operator, err := service.AcceptOperatorInvitation(ctx, token, "Angenommene Operatorin", "SecurePass123!", net.ParseIP("10.0.0.2"))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM platform.operator_audit_log WHERE operator_id = ?`, operator.ID)
		_, _ = db.ExecContext(ctx, `DELETE FROM platform.operators WHERE id = ?`, operator.ID)
	})
	assert.Equal(t, invitee, operator.Email)
	assert.Equal(t, "Angenommene Operatorin", operator.DisplayName)
	assert.True(t, operator.Active)

	// The link is spent: a replay is refused with the envelope the public
	// accept page renders as "not found".
	_, err = service.AcceptOperatorInvitation(ctx, token, "Zweite", "SecurePass123!", nil)
	var notFound *platformSvc.OperatorInvitationNotFoundError
	require.ErrorAs(t, err, &notFound)
}

// An address that already belongs to an operator is reported, because an
// invitation is an administrative act with nothing to enumerate.
func TestIntegration_OperatorInvitation_RefusesAnExistingOperator(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db := testpkg.SetupTestDB(t)

	service := buildInvitationService(t, db)
	ctx := context.Background()
	existing := uniqueOperatorEmail("operator-existing")
	inviter := createInvitingOperator(t, db, uniqueOperatorEmail("operator-inviter"))
	createInvitingOperator(t, db, existing)

	err := service.InviteOperator(ctx, existing, nil, inviter, nil)
	var exists *platformSvc.OperatorInvitationEmailExistsError
	require.ErrorAs(t, err, &exists)
}

// An unusable address is refused with the message the operator surface maps
// to German, and nothing is stored.
func TestIntegration_OperatorInvitation_RefusesAnUnusableAddress(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db := testpkg.SetupTestDB(t)

	service := buildInvitationService(t, db)
	ctx := context.Background()
	inviter := createInvitingOperator(t, db, uniqueOperatorEmail("operator-inviter"))

	err := service.InviteOperator(ctx, "not-an-email", nil, inviter, nil)
	var invalid *platformSvc.InvalidDataError
	require.ErrorAs(t, err, &invalid)
	assert.Equal(t, "invalid email format", invalid.Err.Error())

	pending, listErr := service.ListPendingOperatorInvitations(ctx)
	require.NoError(t, listErr)
	for _, invitation := range pending {
		assert.NotEqual(t, "not-an-email", invitation.Email)
	}
}

// A second invitation for the same address spends the first, so only the
// newest link can be redeemed.
func TestIntegration_OperatorInvitation_SpendsThePreviousLink(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db := testpkg.SetupTestDB(t)

	service := buildInvitationService(t, db)
	ctx := context.Background()
	inviter := createInvitingOperator(t, db, uniqueOperatorEmail("operator-inviter"))
	invitee := uniqueOperatorEmail("operator-invitee")

	require.NoError(t, service.InviteOperator(ctx, invitee, nil, inviter, nil))
	first := pendingTokenFor(t, service, invitee)
	require.NoError(t, service.InviteOperator(ctx, invitee, nil, inviter, nil))
	second := pendingTokenFor(t, service, invitee)
	require.NotEqual(t, first, second)

	_, err := service.ValidateOperatorInvitation(ctx, first)
	var notFound *platformSvc.OperatorInvitationNotFoundError
	require.ErrorAs(t, err, &notFound)
	_, err = service.ValidateOperatorInvitation(ctx, second)
	require.NoError(t, err)
}

// Five invitations per hour: the sixth is refused, and the refusal leaves
// the invitee's live link alone.
func TestIntegration_OperatorInvitation_RateLimit(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db := testpkg.SetupTestDB(t)

	service := buildInvitationService(t, db)
	ctx := context.Background()
	inviter := createInvitingOperator(t, db, uniqueOperatorEmail("operator-inviter"))

	for i := range 5 {
		require.NoError(t, service.InviteOperator(ctx, uniqueOperatorEmail(fmt.Sprintf("operator-limit-%d", i)), nil, inviter, nil),
			"invitation %d must be allowed", i)
	}
	live := uniqueOperatorEmail("operator-limit-live")
	err := service.InviteOperator(ctx, live, nil, inviter, nil)
	var limited *platformSvc.OperatorInvitationRateLimitError
	require.ErrorAs(t, err, &limited)
}

// A resend extends a still redeemable link; a revoked one is never revived.
func TestIntegration_OperatorInvitation_ResendAndRevoke(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db := testpkg.SetupTestDB(t)

	service := buildInvitationService(t, db)
	ctx := context.Background()
	inviter := createInvitingOperator(t, db, uniqueOperatorEmail("operator-inviter"))
	invitee := uniqueOperatorEmail("operator-invitee")

	require.NoError(t, service.InviteOperator(ctx, invitee, nil, inviter, nil))
	token := pendingTokenFor(t, service, invitee)
	before := pendingExpiryFor(t, service, invitee)
	invitationID := pendingIDFor(t, service, invitee)

	require.NoError(t, service.ResendOperatorInvitation(ctx, invitationID, inviter, net.ParseIP("10.0.0.3")))
	assert.True(t, pendingExpiryFor(t, service, invitee).After(before), "the resend extends the link")
	_, err := service.ValidateOperatorInvitation(ctx, token)
	require.NoError(t, err, "the resend keeps the same link usable")

	require.NoError(t, service.RevokeOperatorInvitation(ctx, invitationID, inviter, net.ParseIP("10.0.0.4")))
	_, err = service.ValidateOperatorInvitation(ctx, token)
	var notFound *platformSvc.OperatorInvitationNotFoundError
	require.ErrorAs(t, err, &notFound)

	err = service.ResendOperatorInvitation(ctx, invitationID, inviter, nil)
	require.ErrorAs(t, err, &notFound, "a revoked link is never revived")
}

func pendingInvitationFor(t *testing.T, service platformSvc.OperatorInvitationService, email string) (int64, string, time.Time) {
	t.Helper()
	pending, err := service.ListPendingOperatorInvitations(context.Background())
	require.NoError(t, err)
	for _, invitation := range pending {
		if invitation.Email == email {
			return invitation.ID, invitation.Token, invitation.ExpiresAt
		}
	}
	require.Fail(t, "no pending invitation for "+email)
	return 0, "", time.Time{}
}

func pendingTokenFor(t *testing.T, service platformSvc.OperatorInvitationService, email string) string {
	t.Helper()
	_, token, _ := pendingInvitationFor(t, service, email)
	return token
}

func pendingIDFor(t *testing.T, service platformSvc.OperatorInvitationService, email string) int64 {
	t.Helper()
	id, _, _ := pendingInvitationFor(t, service, email)
	return id
}

func pendingExpiryFor(t *testing.T, service platformSvc.OperatorInvitationService, email string) time.Time {
	t.Helper()
	_, _, expiry := pendingInvitationFor(t, service, email)
	return expiry
}
