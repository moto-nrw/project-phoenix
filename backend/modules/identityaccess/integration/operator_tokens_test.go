package integration

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"
)

func newOperatorTokens(t *testing.T, db *bun.DB) *identityaccess.Module {
	t.Helper()
	module, err := identityCompose.New(identityCompose.Dependencies{DB: db, Observe: func(identityCompose.Observation) {}})
	require.NoError(t, err)
	return module
}

func inviteeEmail() string {
	return fmt.Sprintf("invitee-%s@test.local", uuid.Must(uuid.NewV4()).String())
}

func newInvitation(createdBy int64, email string) identityaccess.OperatorInvitation {
	displayName := "Neue Operatorin"
	return identityaccess.OperatorInvitation{
		Email: email, Token: uuid.Must(uuid.NewV4()).String(), ExpiresAt: time.Now().Add(48 * time.Hour),
		CreatedBy: createdBy, DisplayName: &displayName,
	}
}

func newEmailChange(operatorID int64) identityaccess.OperatorEmailChange {
	return identityaccess.OperatorEmailChange{
		OperatorID: operatorID, NewEmail: inviteeEmail(), Token: uuid.Must(uuid.NewV4()).String(), Expiry: time.Now().Add(30 * time.Minute),
	}
}

// assertUniqueViolation proves the store error still carries the Postgres
// unique violation the retained flows map to their conflict answers.
func assertUniqueViolation(t *testing.T, err error, msg string) {
	t.Helper()
	var pgErr pgdriver.Error
	require.True(t, errors.As(err, &pgErr), "%s: %v", msg, err)
	assert.Equal(t, "23505", pgErr.Field('C'), msg)
}

// insertExpiredInvitation stores a link the capability refuses to create,
// the way an invitation looks once its lifetime has passed.
func insertExpiredInvitation(t *testing.T, db *bun.DB, createdBy int64, email string, createdAt time.Time) int64 {
	t.Helper()
	var id int64
	require.NoError(t, db.NewRaw(`INSERT INTO platform.operator_invitation_tokens (email, token, expires_at, created_by, created_at)
		VALUES (?, ?, ?, ?, ?) RETURNING id`, email, uuid.Must(uuid.NewV4()).String(), time.Now().Add(-time.Minute), createdBy, createdAt).
		Scan(context.Background(), &id))
	return id
}

func insertEmailChange(t *testing.T, db *bun.DB, operatorID int64, expiry time.Time, used bool, createdAt time.Time) int64 {
	t.Helper()
	var id int64
	require.NoError(t, db.NewRaw(`INSERT INTO platform.operator_email_change_tokens (operator_id, new_email, token, expiry, used, created_at)
		VALUES (?, ?, ?, ?, ?, ?) RETURNING id`, operatorID, inviteeEmail(), uuid.Must(uuid.NewV4()).String(), expiry, used, createdAt).
		Scan(context.Background(), &id))
	return id
}

// An invitation is stored only while it can still be redeemed, is
// redeemable exactly once, and is never revived after it expired.
func TestOperatorInvitationIsRedeemableOnce(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	tokens := newOperatorTokens(t, db)
	ctx := testpkg.Ctx(t)
	inviter := testpkg.CreateTestOperator(t, db)
	before := time.Now().Add(-time.Second)

	expired := newInvitation(inviter.ID, inviteeEmail())
	expired.ExpiresAt = time.Now().Add(-time.Second)
	_, err := tokens.CreateOperatorInvitation(ctx, expired)
	require.EqualError(t, err, "identity access: create operator invitation: token has already expired",
		"an invitation must never be mailed with a dead link")
	missing := newInvitation(0, inviteeEmail())
	_, err = tokens.CreateOperatorInvitation(ctx, missing)
	require.EqualError(t, err, "identity access: create operator invitation: created_by operator ID is required")

	email := inviteeEmail()
	invitation, err := tokens.CreateOperatorInvitation(ctx, newInvitation(inviter.ID, email))
	require.NoError(t, err)
	require.NotZero(t, invitation.ID)
	require.NotZero(t, invitation.CreatedAt)
	require.NotNil(t, invitation.DisplayName)
	assert.Equal(t, "Neue Operatorin", *invitation.DisplayName)
	assert.Zero(t, invitation.Delivery.RetryCount)

	_, err = tokens.CreateOperatorInvitation(ctx, newInvitation(inviter.ID, email))
	require.Error(t, err)
	assertUniqueViolation(t, err, "one pending invitation per address stays a unique violation the flow maps")

	found, err := tokens.FindRedeemableOperatorInvitation(ctx, invitation.Token)
	require.NoError(t, err)
	assert.Equal(t, invitation.ID, found.ID)
	_, err = tokens.FindRedeemableOperatorInvitation(ctx, "")
	require.ErrorIs(t, err, identityaccess.ErrOperatorInvitationNotFound)
	_, err = tokens.FindRedeemableOperatorInvitation(ctx, uuid.Must(uuid.NewV4()).String())
	require.ErrorIs(t, err, identityaccess.ErrOperatorInvitationNotFound)

	redeemed, err := tokens.RedeemOperatorInvitation(ctx, invitation.Token)
	require.NoError(t, err)
	assert.Equal(t, email, redeemed.Email)
	require.NotNil(t, redeemed.UsedAt)
	_, err = tokens.RedeemOperatorInvitation(ctx, invitation.Token)
	require.ErrorIs(t, err, identityaccess.ErrOperatorInvitationNotFound, "the loser of two redemptions must not create a second operator")
	_, err = tokens.FindRedeemableOperatorInvitation(ctx, invitation.Token)
	require.ErrorIs(t, err, identityaccess.ErrOperatorInvitationNotFound)
	spent, err := tokens.FindOperatorInvitation(ctx, invitation.ID)
	require.NoError(t, err, "a spent invitation is still found by id")
	require.NotNil(t, spent.UsedAt)
	_, err = tokens.FindOperatorInvitation(ctx, invitation.ID+1_000_000)
	require.ErrorIs(t, err, identityaccess.ErrOperatorInvitationNotFound)
	require.ErrorIs(t, tokens.RevokeOperatorInvitation(ctx, invitation.ID), identityaccess.ErrOperatorInvitationNotFound)
	require.ErrorIs(t, tokens.ExtendOperatorInvitation(ctx, invitation.ID, time.Now().Add(time.Hour)), identityaccess.ErrOperatorInvitationNotFound,
		"a spent invitation is not revived")

	expiredID := insertExpiredInvitation(t, db, inviter.ID, inviteeEmail(), time.Now())
	expiredRow, err := tokens.FindOperatorInvitation(ctx, expiredID)
	require.NoError(t, err)
	_, err = tokens.RedeemOperatorInvitation(ctx, expiredRow.Token)
	require.ErrorIs(t, err, identityaccess.ErrOperatorInvitationNotFound, "an expired invitation is not redeemable")
	require.ErrorIs(t, tokens.ExtendOperatorInvitation(ctx, expiredID, time.Now().Add(time.Hour)), identityaccess.ErrOperatorInvitationNotFound,
		"a resend must not revive an expired invitation")

	pending, err := tokens.CreateOperatorInvitation(ctx, newInvitation(inviter.ID, inviteeEmail()))
	require.NoError(t, err)
	require.ErrorIs(t, tokens.ExtendOperatorInvitation(ctx, pending.ID, time.Now().Add(-time.Minute)), identityaccess.ErrOperatorInvitationNotFound,
		"an extension into the past would hand out a dead link")
	extendedTo := time.Now().Add(72 * time.Hour).Truncate(time.Microsecond)
	require.NoError(t, tokens.ExtendOperatorInvitation(ctx, pending.ID, extendedTo))
	reloaded, err := tokens.FindOperatorInvitation(ctx, pending.ID)
	require.NoError(t, err)
	assert.True(t, extendedTo.Equal(reloaded.ExpiresAt))

	listed, err := tokens.ListRedeemableOperatorInvitations(ctx)
	require.NoError(t, err)
	listedIDs := make([]int64, 0, len(listed))
	for _, entry := range listed {
		listedIDs = append(listedIDs, entry.ID)
	}
	assert.Contains(t, listedIDs, pending.ID)
	assert.NotContains(t, listedIDs, invitation.ID, "spent invitations are not pending")
	assert.NotContains(t, listedIDs, expiredID, "expired invitations are not pending")

	count, err := tokens.CountOperatorInvitationsCreatedAfter(ctx, inviter.ID, before)
	require.NoError(t, err)
	assert.Equal(t, 3, count, "the rate limit counts every invitation sent, spent or not")

	revoked, err := tokens.RevokeOperatorInvitationsForEmail(ctx, pending.Email)
	require.NoError(t, err)
	assert.Equal(t, 1, revoked)
	revoked, err = tokens.RevokeOperatorInvitationsForEmail(ctx, pending.Email)
	require.NoError(t, err)
	assert.Zero(t, revoked)

	single, err := tokens.CreateOperatorInvitation(ctx, newInvitation(inviter.ID, inviteeEmail()))
	require.NoError(t, err)
	require.NoError(t, tokens.RevokeOperatorInvitation(ctx, single.ID))
	require.ErrorIs(t, tokens.RevokeOperatorInvitation(ctx, single.ID), identityaccess.ErrOperatorInvitationNotFound)

	deleted, err := tokens.DeleteExpiredOperatorInvitations(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, deleted, 1)
	_, err = tokens.FindOperatorInvitation(ctx, expiredID)
	require.ErrorIs(t, err, identityaccess.ErrOperatorInvitationNotFound)
	_, err = tokens.FindOperatorInvitation(ctx, single.ID)
	require.NoError(t, err, "cleanup keeps unexpired rows")
}

func TestOperatorTokenDeliveryIsRecordedAndBounded(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	tokens := newOperatorTokens(t, db)
	ctx := testpkg.Ctx(t)
	operator := testpkg.CreateTestOperator(t, db)

	invitation, err := tokens.CreateOperatorInvitation(ctx, newInvitation(operator.ID, inviteeEmail()))
	require.NoError(t, err)
	failure := strings.Repeat("ü", 1500)
	require.NoError(t, tokens.RecordOperatorInvitationDelivery(ctx, invitation.ID, identityaccess.TokenDelivery{Error: &failure, RetryCount: 3}))
	reloaded, err := tokens.FindOperatorInvitation(ctx, invitation.ID)
	require.NoError(t, err)
	require.NotNil(t, reloaded.Delivery.Error)
	assert.Equal(t, strings.Repeat("ü", 1024), *reloaded.Delivery.Error, "the stored error is cut at 1024 runes")
	assert.Equal(t, 3, reloaded.Delivery.RetryCount)
	assert.Nil(t, reloaded.Delivery.SentAt)
	assert.Equal(t, strings.Repeat("ü", 1500), failure, "the caller's value is not modified")

	sentAt := time.Now().Truncate(time.Microsecond)
	require.NoError(t, tokens.RecordOperatorInvitationDelivery(ctx, invitation.ID, identityaccess.TokenDelivery{SentAt: &sentAt, RetryCount: 4}))
	reloaded, err = tokens.FindOperatorInvitation(ctx, invitation.ID)
	require.NoError(t, err)
	require.NotNil(t, reloaded.Delivery.SentAt)
	assert.True(t, sentAt.Equal(*reloaded.Delivery.SentAt))
	assert.Nil(t, reloaded.Delivery.Error, "a successful delivery clears the previous error")
	assert.Equal(t, 4, reloaded.Delivery.RetryCount)
	require.NoError(t, tokens.RecordOperatorInvitationDelivery(ctx, invitation.ID+1_000_000, identityaccess.TokenDelivery{}),
		"a delivery callback for a row cleaned up meanwhile is a no-op")

	change, err := tokens.CreateOperatorEmailChange(ctx, newEmailChange(operator.ID))
	require.NoError(t, err)
	require.NoError(t, tokens.RecordOperatorEmailChangeDelivery(ctx, change.ID, identityaccess.TokenDelivery{Error: &failure, RetryCount: 2}))
	var stored struct {
		EmailError      string `bun:"email_error"`
		EmailRetryCount int    `bun:"email_retry_count"`
	}
	require.NoError(t, db.NewRaw(`SELECT email_error, email_retry_count FROM platform.operator_email_change_tokens WHERE id = ?`, change.ID).
		Scan(ctx, &stored))
	assert.Equal(t, strings.Repeat("ü", 1024), stored.EmailError)
	assert.Equal(t, 2, stored.EmailRetryCount)
}

func TestOperatorEmailChangeIsRedeemableOnce(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	tokens := newOperatorTokens(t, db)
	ctx := testpkg.Ctx(t)
	operator := testpkg.CreateTestOperator(t, db)
	other := testpkg.CreateTestOperator(t, db)
	before := time.Now().Add(-time.Second)

	expired := newEmailChange(operator.ID)
	expired.Expiry = time.Now().Add(-time.Second)
	_, err := tokens.CreateOperatorEmailChange(ctx, expired)
	require.EqualError(t, err, "identity access: create operator email change: token has already expired")
	used := newEmailChange(operator.ID)
	used.Used = true
	_, err = tokens.CreateOperatorEmailChange(ctx, used)
	require.EqualError(t, err, "identity access: create operator email change: token has already been used")

	change, err := tokens.CreateOperatorEmailChange(ctx, newEmailChange(operator.ID))
	require.NoError(t, err)
	require.NotZero(t, change.ID)
	assert.False(t, change.Used)

	_, err = tokens.CreateOperatorEmailChange(ctx, newEmailChange(operator.ID))
	require.ErrorIs(t, err, identityaccess.ErrOperatorEmailChangeActive,
		"one active link per operator is refused, and the SQLSTATE stays inside the adapter")

	foreign, err := tokens.CreateOperatorEmailChange(ctx, newEmailChange(other.ID))
	require.NoError(t, err)

	require.NoError(t, tokens.RevokeOperatorEmailChanges(ctx, operator.ID))
	_, err = tokens.RedeemOperatorEmailChange(ctx, change.Token)
	require.ErrorIs(t, err, identityaccess.ErrOperatorEmailChangeNotFound, "a revoked link is not redeemable")

	replacement, err := tokens.CreateOperatorEmailChange(ctx, newEmailChange(operator.ID))
	require.NoError(t, err, "revocation frees the active slot")
	redeemed, err := tokens.RedeemOperatorEmailChange(ctx, replacement.Token)
	require.NoError(t, err)
	assert.Equal(t, replacement.NewEmail, redeemed.NewEmail)
	assert.Equal(t, operator.ID, redeemed.OperatorID)
	assert.True(t, redeemed.Used)
	_, err = tokens.RedeemOperatorEmailChange(ctx, replacement.Token)
	require.ErrorIs(t, err, identityaccess.ErrOperatorEmailChangeNotFound, "the loser of two confirmations must not change the address again")
	_, err = tokens.RedeemOperatorEmailChange(ctx, "")
	require.ErrorIs(t, err, identityaccess.ErrOperatorEmailChangeNotFound)

	_, err = tokens.RedeemOperatorEmailChange(ctx, foreign.Token)
	require.NoError(t, err, "revoking one operator's links leaves other operators alone")

	count, err := tokens.CountOperatorEmailChangesCreatedAfter(ctx, operator.ID, before)
	require.NoError(t, err)
	assert.Equal(t, 2, count, "the rate limit counts every link sent, spent or not")
}

// The cleanup frees the active slot of an expired link at once but keeps
// every row the one-hour rate-limit window still counts.
func TestOperatorEmailChangeCleanupKeepsTheRateLimitWindow(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	tokens := newOperatorTokens(t, db)
	ctx := testpkg.Ctx(t)
	operator := testpkg.CreateTestOperator(t, db)
	now := time.Now()

	expiredRecent := insertEmailChange(t, db, operator.ID, now.Add(-time.Minute), false, now.Add(-10*time.Minute))
	usedOld := insertEmailChange(t, db, operator.ID, now.Add(time.Hour), true, now.Add(-2*time.Hour))
	expiredOld := insertEmailChange(t, db, operator.ID, now.Add(-90*time.Minute), true, now.Add(-2*time.Hour))
	activeOld := testpkg.CreateTestOperator(t, db)
	activeOldID := insertEmailChange(t, db, activeOld.ID, now.Add(time.Hour), false, now.Add(-2*time.Hour))

	revoked, err := tokens.RevokeExpiredOperatorEmailChanges(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, revoked)
	_, err = tokens.CreateOperatorEmailChange(ctx, newEmailChange(operator.ID))
	require.NoError(t, err, "the expired link no longer blocks a new request")

	deleted, err := tokens.DeleteStaleOperatorEmailChanges(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, deleted)

	var remaining []int64
	require.NoError(t, db.NewRaw(`SELECT id FROM platform.operator_email_change_tokens WHERE id IN (?) ORDER BY id`,
		bun.List([]int64{expiredRecent, usedOld, expiredOld, activeOldID})).Scan(ctx, &remaining))
	assert.Equal(t, []int64{expiredRecent, activeOldID}, remaining,
		"rows inside the rate-limit window and old links still redeemable stay")
	count, err := tokens.CountOperatorEmailChangesCreatedAfter(ctx, operator.ID, now.Add(-time.Hour))
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

// The operator link tables carry no tenant column: a tenant context must
// neither hide nor reveal rows.
func TestOperatorTokensAreVisibleFromEveryTenantContext(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	tokens := newOperatorTokens(t, db)
	operator := testpkg.CreateTestOperator(t, db)
	ctx := testpkg.Ctx(t)
	invitation, err := tokens.CreateOperatorInvitation(ctx, newInvitation(operator.ID, inviteeEmail()))
	require.NoError(t, err)
	first := testpkg.Tenant(t)
	second, _ := testpkg.CreateTestTenant(t, db)
	require.NotEqual(t, first, second)

	for name, tenantID := range map[string]int64{"own tenant": first, "foreign tenant": second} {
		t.Run(name, func(t *testing.T) {
			tenantCtx := testpkg.TenantContext(tenantID)
			found, err := tokens.FindRedeemableOperatorInvitation(tenantCtx, invitation.Token)
			require.NoError(t, err)
			assert.Equal(t, invitation.ID, found.ID)
			change, err := tokens.CreateOperatorEmailChange(tenantCtx, newEmailChange(operator.ID))
			require.NoError(t, err)
			redeemed, err := tokens.RedeemOperatorEmailChange(tenantCtx, change.Token)
			require.NoError(t, err)
			assert.Equal(t, change.ID, redeemed.ID)
		})
	}
}

// Inviting spends the address's previous invitation and stores the new one
// in one administrative transaction. A failure after either write rolls both
// back, so the invitee keeps the last usable link, and a retry succeeds.
func TestOperatorInviteRollsBackAfterEachWrite(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	tokens := newOperatorTokens(t, db)
	inviter := testpkg.CreateTestOperator(t, db)
	plain := testpkg.Ctx(t)
	ctx := testpkg.WithTenantRuntime(t, plain, db)
	email := inviteeEmail()
	previous, err := tokens.CreateOperatorInvitation(plain, newInvitation(inviter.ID, email))
	require.NoError(t, err)
	next := newInvitation(inviter.ID, email)

	writes := []func(context.Context) error{
		func(txCtx context.Context) error {
			_, err := tokens.RevokeOperatorInvitationsForEmail(txCtx, email)
			return err
		},
		func(txCtx context.Context) error {
			_, err := tokens.CreateOperatorInvitation(txCtx, next)
			return err
		},
	}
	for failAfter := range writes {
		err := testpkg.WithAdminTx(t, ctx, db, func(txCtx context.Context, _ bun.Tx) error {
			for index, write := range writes[:failAfter+1] {
				if err := write(txCtx); err != nil {
					t.Fatalf("write %d: %v", index, err)
				}
			}
			return errInjected
		})
		require.ErrorIs(t, err, errInjected)
		_, err = tokens.FindRedeemableOperatorInvitation(plain, previous.Token)
		require.NoError(t, err, "the previous invitation must stay redeemable after a failure at write %d", failAfter)
		_, err = tokens.FindRedeemableOperatorInvitation(plain, next.Token)
		require.ErrorIs(t, err, identityaccess.ErrOperatorInvitationNotFound, "the new invitation must be rolled back")
	}

	err = testpkg.WithAdminTx(t, ctx, db, func(txCtx context.Context, _ bun.Tx) error {
		for _, write := range writes {
			if err := write(txCtx); err != nil {
				return err
			}
		}
		return nil
	})
	require.NoError(t, err, "the retry starts from a clean state")
	_, err = tokens.FindRedeemableOperatorInvitation(plain, previous.Token)
	require.ErrorIs(t, err, identityaccess.ErrOperatorInvitationNotFound)
	_, err = tokens.FindRedeemableOperatorInvitation(plain, next.Token)
	require.NoError(t, err)
}

// Accepting an invitation spends it and creates the operator in one
// administrative transaction; confirming an e-mail change spends the link
// and updates the operator the same way. A failure after either write keeps
// the link redeemable and the operator unchanged, and a retry succeeds.
func TestOperatorLinkRedemptionRollsBackAfterEachWrite(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := newOperatorTokens(t, db)
	plain := testpkg.Ctx(t)
	ctx := testpkg.WithTenantRuntime(t, plain, db)
	inviter := testpkg.CreateTestOperator(t, db)
	operator := testpkg.CreateTestOperator(t, db)

	email := inviteeEmail()
	invitation, err := module.CreateOperatorInvitation(plain, newInvitation(inviter.ID, email))
	require.NoError(t, err)
	change, err := module.CreateOperatorEmailChange(plain, newEmailChange(operator.ID))
	require.NoError(t, err)

	flows := map[string]struct {
		writes          []func(context.Context) error
		assertUntouched func(t *testing.T)
		assertApplied   func(t *testing.T)
	}{
		"accept invitation": {
			writes: []func(context.Context) error{
				func(txCtx context.Context) error {
					_, err := module.RedeemOperatorInvitation(txCtx, invitation.Token)
					return err
				},
				func(txCtx context.Context) error {
					_, err := module.CreateOperator(txCtx, identityaccess.Operator{Email: email, DisplayName: "Neu", PasswordHash: "hash", Active: true})
					return err
				},
			},
			assertUntouched: func(t *testing.T) {
				t.Helper()
				_, err := module.FindRedeemableOperatorInvitation(plain, invitation.Token)
				require.NoError(t, err, "the redemption must be rolled back")
				_, err = module.FindOperatorByEmail(plain, email)
				require.ErrorIs(t, err, identityaccess.ErrOperatorNotFound, "the operator must be rolled back")
			},
			assertApplied: func(t *testing.T) {
				t.Helper()
				_, err := module.FindRedeemableOperatorInvitation(plain, invitation.Token)
				require.ErrorIs(t, err, identityaccess.ErrOperatorInvitationNotFound)
				created, err := module.FindOperatorByEmail(plain, email)
				require.NoError(t, err)
				testpkg.OwnTestOperator(t, db, created.ID)
			},
		},
		"confirm email change": {
			writes: []func(context.Context) error{
				func(txCtx context.Context) error {
					_, err := module.RedeemOperatorEmailChange(txCtx, change.Token)
					return err
				},
				func(txCtx context.Context) error {
					current, err := module.FindOperator(txCtx, operator.ID)
					if err != nil {
						return err
					}
					current.Email = change.NewEmail
					_, err = module.UpdateOperator(txCtx, current)
					return err
				},
			},
			assertUntouched: func(t *testing.T) {
				t.Helper()
				reloaded, err := module.FindOperator(plain, operator.ID)
				require.NoError(t, err)
				assert.Equal(t, operator.Email, reloaded.Email, "the address update must be rolled back")
				_, err = module.CreateOperatorEmailChange(plain, newEmailChange(operator.ID))
				require.Error(t, err, "the link must still hold the operator's active slot")
			},
			assertApplied: func(t *testing.T) {
				t.Helper()
				reloaded, err := module.FindOperator(plain, operator.ID)
				require.NoError(t, err)
				assert.Equal(t, change.NewEmail, reloaded.Email)
				_, err = module.RedeemOperatorEmailChange(plain, change.Token)
				require.ErrorIs(t, err, identityaccess.ErrOperatorEmailChangeNotFound)
			},
		},
	}
	for name, flow := range flows {
		t.Run(name, func(t *testing.T) {
			for failAfter := range flow.writes {
				err := testpkg.WithAdminTx(t, ctx, db, func(txCtx context.Context, _ bun.Tx) error {
					for index, write := range flow.writes[:failAfter+1] {
						if err := write(txCtx); err != nil {
							t.Fatalf("write %d: %v", index, err)
						}
					}
					return errInjected
				})
				require.ErrorIs(t, err, errInjected)
				flow.assertUntouched(t)
			}
			err := testpkg.WithAdminTx(t, ctx, db, func(txCtx context.Context, _ bun.Tx) error {
				for _, write := range flow.writes {
					if err := write(txCtx); err != nil {
						return err
					}
				}
				return nil
			})
			require.NoError(t, err, "the retry starts from a clean state")
			flow.assertApplied(t)
		})
	}
}

func TestOperatorTokenObservationsUseStableOperations(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	operator := testpkg.CreateTestOperator(t, db)
	var observations []identityCompose.Observation
	tokens, err := identityCompose.New(identityCompose.Dependencies{DB: db, Observe: func(observation identityCompose.Observation) {
		observations = append(observations, observation)
	}})
	require.NoError(t, err)
	ctx := testpkg.Ctx(t)

	invitation, err := tokens.CreateOperatorInvitation(ctx, newInvitation(operator.ID, inviteeEmail()))
	require.NoError(t, err)
	_, err = tokens.RedeemOperatorInvitation(ctx, invitation.Token)
	require.NoError(t, err)
	_, err = tokens.RedeemOperatorInvitation(ctx, invitation.Token)
	require.Error(t, err)
	change, err := tokens.CreateOperatorEmailChange(ctx, newEmailChange(operator.ID))
	require.NoError(t, err)
	_, err = tokens.CreateOperatorEmailChange(ctx, newEmailChange(operator.ID))
	require.Error(t, err)
	_, err = tokens.RedeemOperatorEmailChange(ctx, change.Token)
	require.NoError(t, err)

	expected := []struct {
		operation string
		rows      int64
		code      string
	}{
		{"create_operator_invitation", 1, "none"},
		{"redeem_operator_invitation", 1, "none"},
		{"redeem_operator_invitation", 0, "not_found"},
		{"create_operator_email_change", 1, "none"},
		{"create_operator_email_change", 0, "internal"},
		{"redeem_operator_email_change", 1, "none"},
	}
	require.Len(t, observations, len(expected))
	for index, want := range expected {
		observation := observations[index]
		assert.Equal(t, want.operation, observation.Operation)
		assert.Equal(t, want.rows, observation.Stats.Rows, want.operation)
		assert.Equal(t, want.code, identityaccess.ErrorCode(observation.Err), want.operation)
		assert.EqualValues(t, 1, observation.Stats.Queries, want.operation)
	}
}
