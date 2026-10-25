package api

import (
	"context"
	"errors"
	"log/slog"
	"regexp"
	"strconv"
	"testing"
	"time"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"

	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func mailTransactionFixture(t *testing.T) (*bun.DB, context.Context, owners.OutboxEnqueuer) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.WithTenantRuntime(t, testpkg.Ctx(t), db)
	runtime, err := deliveryCompose.NewModule(deliveryCompose.ModuleDependencies{
		DB: db, People: owners.EnrollmentFlowGuardianDirectory{}, Observe: func(deliveryContract.Observation) {},
	})
	require.NoError(t, err)
	return db, ctx, owners.OutboxEnqueuer{Runtime: runtime, Outbox: deliveryCompose.NewEmailOutbox(deliveryCompose.NewDurableEmails(runtime.Module))}
}

func assertTransactionalMails(t *testing.T, db *bun.DB, ctx context.Context, admin bool, address string, kinds []string, trigger func(context.Context) error) {
	t.Helper()
	matches := `kind = ? AND ((payload->'Message'->'To'->>'Address') = ? OR (payload->'Message'->'Content'->>'Email') = ?)`
	t.Cleanup(func() {
		_, err := db.NewDelete().TableExpr("platform.email_outbox").Where(matches, owners.TransactionalMailKind, address, address).Exec(context.Background())
		require.NoError(t, err)
	})
	counts := func(queryCtx context.Context, source bun.IDB, expected int) {
		t.Helper()
		for _, kind := range kinds {
			n, err := source.NewSelect().TableExpr("platform.email_outbox").Where(matches, owners.TransactionalMailKind, address, address).Where("payload->>'Type' = ?", kind).Count(queryCtx)
			require.NoError(t, err)
			assert.EqualValues(t, expected, n, kind)
		}
	}
	transaction := func(commit bool) error {
		body := func(txCtx context.Context, tx bun.Tx) error {
			if err := trigger(txCtx); err != nil {
				return err
			}
			counts(txCtx, tx, 1)
			if !commit {
				return errors.New("rollback triggering change")
			}
			return nil
		}
		if admin {
			return tenant.WithAdminTx(ctx, db, body)
		}
		return tenant.WithTenantTx(ctx, db, testpkg.Tenant(t), body)
	}
	require.ErrorContains(t, transaction(false), "rollback triggering change")
	counts(ctx, db, 0)
	require.NoError(t, transaction(true))
	counts(ctx, db, 1)
}

func TestPasswordResetOutboxIsAtomicWithToken(t *testing.T) {
	t.Parallel()
	db, ctx, outbox := mailTransactionFixture(t)
	module, err := NewAuthTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), WithAuthTestTransactionalOutbox(outbox), WithAuthTestPasswordResetRateLimit(false))
	require.NoError(t, err)
	account := testpkg.CreateTestAccount(t, db, "atomic-password-reset")
	assertTransactionalMails(t, db, ctx, true, account.Email, []string{"password_reset"}, func(txCtx context.Context) error {
		link, err := module.Auth.InitiatePasswordReset(txCtx, account.Email, identityaccess.PasswordResetScopeStaff)
		if err == nil {
			require.NotNil(t, link)
		}
		return err
	})
}

func TestStaffAndSchoolInvitationOutboxIsAtomicWithToken(t *testing.T) {
	t.Parallel()
	db, ctx, outbox := mailTransactionFixture(t)
	module, err := NewAuthTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), WithAuthTestTransactionalOutbox(outbox))
	require.NoError(t, err)
	creator := testpkg.CreateTestAccount(t, db, "atomic-invitation-creator")
	role := testpkg.CreateTestRole(t, db, "atomic-invited-staff")
	address := "atomic-invitation-" + strconv.FormatInt(testpkg.UniqueSuffix(), 10) + "@example.test"
	assertTransactionalMails(t, db, ctx, false, address, []string{"invitation"}, func(txCtx context.Context) error {
		_, err := module.Auth.CreateSchoolInvitation(txCtx, identityaccess.SchoolInvitationRequest{
			Email: address, TenantID: testpkg.Tenant(t), RoleID: role.ID, CreatedBy: creator.ID,
			ActorPermissions: []string{"users:manage"},
		})
		return err
	})
}

func TestOperatorInvitationOutboxIsAtomicWithToken(t *testing.T) {
	t.Parallel()
	db, ctx, _ := mailTransactionFixture(t)
	module, err := NewTestIdentityAccess(testpkg.NewDatabase(t, db), slog.Default())
	require.NoError(t, err)
	inviter := testpkg.CreateTestOperator(t, db)
	address := "atomic-operator-" + strconv.FormatInt(testpkg.UniqueSuffix(), 10) + "@example.test"
	assertTransactionalMails(t, db, ctx, true, address, []string{"operator_invitation"}, func(txCtx context.Context) error {
		return module.InviteOperator(txCtx, identityaccess.OperatorInvitationRequest{Email: address, CreatedBy: inviter.ID})
	})
}

func TestDemoAccessAndLeadOutboxAreAtomicWithAccess(t *testing.T) {
	t.Parallel()
	db, ctx, outbox := mailTransactionFixture(t)
	var slug string
	require.NoError(t, db.NewRaw(`SELECT slug FROM platform.schools WHERE id = ?`, testpkg.Tenant(t)).Scan(ctx, &slug))
	module, err := NewAuthTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), WithAuthTestTransactionalOutbox(outbox), WithStandingDemoSchool(slug))
	require.NoError(t, err)
	address := "atomic-demo-" + strconv.FormatInt(testpkg.UniqueSuffix(), 10) + "@example.test"
	t.Cleanup(func() {
		_, err := db.NewRaw(`DELETE FROM auth.demo_accesses WHERE email = ?`, address).Exec(context.Background())
		require.NoError(t, err)
	})
	assertTransactionalMails(t, db, ctx, true, address, []string{"demo_access", "demo_lead"}, func(txCtx context.Context) error {
		return module.DemoAccess.RequestDemoAccess(txCtx, identityaccess.DemoAccessRequest{Email: address, FirstName: "Test", LastName: "Parent", SchoolName: "Test School", EntryURLPrefix: "http://localhost/demo#token=", ClientIP: "127.0.0.1"})
	})
}

// Global reset/operator mail must never invent a school or leak through RLS.
func TestSystemEmailOutboxScopeIdempotencyAndRetention(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db, ctx, outbox := mailTransactionFixture(t)
	key := "system-mail-" + strconv.FormatInt(testpkg.UniqueSuffix(), 10)
	intent := deliveryContract.EmailIntent{Template: "test_system_mail", Recipient: deliveryContract.EmailRecipient{Address: key + "@example.test"}, Payload: []byte(`{"test":true}`), IdempotencyKey: key, DeliverAfter: time.Now().Add(time.Hour)}
	require.Error(t, tenant.WithTenantTx(ctx, db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		_, err := outbox.Runtime.Module.EnqueueSystemEmail(txCtx, intent)
		return err
	}))
	var first, second deliveryContract.Enqueued
	require.NoError(t, tenant.WithAdminTx(ctx, db, func(txCtx context.Context, _ bun.Tx) error {
		var err error
		first, err = outbox.Runtime.Module.EnqueueSystemEmail(txCtx, intent)
		if err != nil {
			return err
		}
		second, err = outbox.Runtime.Module.EnqueueSystemEmail(txCtx, intent)
		return err
	}))
	require.Equal(t, first.ID, second.ID)
	require.True(t, second.Duplicate)
	t.Cleanup(func() {
		_, err := db.NewRaw(`DELETE FROM platform.email_outbox WHERE idempotency_key = ?`, key).Exec(context.Background())
		require.NoError(t, err)
	})
	require.NoError(t, tenant.WithTenantTx(ctx, db, testpkg.Tenant(t), func(txCtx context.Context, tx bun.Tx) error {
		var bypass bool
		require.NoError(t, tx.NewRaw(`SELECT rolsuper OR rolbypassrls FROM pg_roles WHERE rolname = current_user`).Scan(txCtx, &bypass))
		require.False(t, bypass)
		count, err := tx.NewSelect().TableExpr("platform.email_outbox").Where("id = ?", first.ID).Count(txCtx)
		require.NoError(t, err)
		require.Zero(t, count)
		return nil
	}))
	// Pending entries remain durable even after the terminal retention cutoff.
	_, err := db.NewRaw(`UPDATE platform.email_outbox SET created_at = ?, updated_at = ? WHERE id = ?`, time.Now().Add(-31*24*time.Hour), time.Now().Add(-31*24*time.Hour), first.ID).Exec(ctx)
	require.NoError(t, err)
	runtime, err := deliveryCompose.NewModule(deliveryCompose.ModuleDependencies{DB: db, People: owners.EnrollmentFlowGuardianDirectory{}, Observe: func(deliveryContract.Observation) {}})
	require.NoError(t, err)
	worker, err := deliveryCompose.NewWorker(runtime, owners.ReminderPushProvider{})
	require.NoError(t, err)
	_, err = worker.RunOnce(ctx, 2, 3)
	require.NoError(t, err)
	count, err := db.NewSelect().TableExpr("platform.email_outbox").Where("id = ? AND tenant_id IS NULL", first.ID).Count(ctx)
	require.NoError(t, err)
	require.EqualValues(t, 1, count)
	_, err = db.NewRaw(`UPDATE platform.email_outbox SET status = 'sent', sent_at = ? WHERE id = ?`, time.Now().Add(-31*24*time.Hour), first.ID).Exec(ctx)
	require.NoError(t, err)
	_, err = worker.RunOnce(ctx, 2, 3)
	require.NoError(t, err)
	count, err = db.NewSelect().TableExpr("platform.email_outbox").Where("id = ?", first.ID).Count(ctx)
	require.NoError(t, err)
	require.Zero(t, count)
}

func TestInvitationResendMailOutboxIsAtomic(t *testing.T) {
	t.Parallel()
	for _, operator := range []bool{false, true} {
		t.Run(strconv.FormatBool(operator), func(t *testing.T) {
			testpkg.OwnTenant(t)
			db, ctx, outbox := mailTransactionFixture(t)
			address := "atomic-resend-" + strconv.FormatInt(testpkg.UniqueSuffix(), 10) + "@example.test"
			kind := "invitation"
			var trigger func(context.Context) error
			if operator {
				module, err := NewTestIdentityAccess(testpkg.NewDatabase(t, db), slog.Default())
				require.NoError(t, err)
				inviter := testpkg.CreateTestOperator(t, db)
				require.NoError(t, module.InviteOperator(ctx, identityaccess.OperatorInvitationRequest{Email: address, CreatedBy: inviter.ID}))
				var id int64
				require.NoError(t, db.NewRaw(`SELECT id FROM platform.operator_invitation_tokens WHERE email = ?`, address).Scan(ctx, &id))
				trigger = func(txCtx context.Context) error {
					return module.ResendOperatorInvitationByActor(txCtx, id, inviter.ID, "127.0.0.1")
				}
				kind = "operator_invitation"
			} else {
				module, err := NewAuthTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), WithAuthTestTransactionalOutbox(outbox))
				require.NoError(t, err)
				creator := testpkg.CreateTestAccount(t, db, "atomic-resend-creator")
				role := testpkg.CreateTestRole(t, db, "atomic-resend-staff")
				invitation, err := module.Auth.CreateSchoolInvitation(ctx, identityaccess.SchoolInvitationRequest{Email: address, TenantID: testpkg.Tenant(t), RoleID: role.ID, CreatedBy: creator.ID, ActorPermissions: []string{"users:manage"}})
				require.NoError(t, err)
				trigger = func(txCtx context.Context) error {
					return module.Auth.ResendSchoolInvitation(txCtx, invitation.ID, creator.ID)
				}
			}
			_, err := db.NewRaw(`DELETE FROM platform.email_outbox WHERE kind = ? AND payload->'Message'->'To'->>'Address' = ?`, owners.TransactionalMailKind, address).Exec(ctx)
			require.NoError(t, err)
			assertTransactionalMails(t, db, ctx, operator, address, []string{kind}, trigger)
		})
	}
}

func TestOperatorEmailChangeMailOutboxIsAtomic(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"operator_email_change_verify", "operator_email_change_requested", "operator_email_change_confirmed"} {
		t.Run(kind, func(t *testing.T) {
			testpkg.OwnTenant(t)
			db, ctx, _ := mailTransactionFixture(t)
			module, err := NewTestIdentityAccess(testpkg.NewDatabase(t, db), slog.Default())
			require.NoError(t, err)
			oldAddress := "atomic-old-" + strconv.FormatInt(testpkg.UniqueSuffix(), 10) + "@example.test"
			newAddress := "atomic-new-" + strconv.FormatInt(testpkg.UniqueSuffix(), 10) + "@example.test"
			password := "Fixture-operator-password-4087!"
			operator := testpkg.CreateTestOperatorWithPassword(t, db, oldAddress, password)
			t.Cleanup(func() {
				_, err := db.NewRaw(`DELETE FROM platform.email_outbox WHERE kind = ? AND payload->'Message'->'To'->>'Address' IN (?, ?)`, owners.TransactionalMailKind, oldAddress, newAddress).Exec(context.Background())
				require.NoError(t, err)
			})
			initiate := func(txCtx context.Context) error {
				return module.InitiateOperatorEmailChange(txCtx, identityaccess.OperatorEmailChangeRequest{OperatorID: operator.ID, NewEmail: newAddress, CurrentPassword: password, IPAddress: "127.0.0.1"})
			}
			address, trigger := oldAddress, initiate
			if kind == "operator_email_change_verify" {
				address = newAddress
			}
			if kind == "operator_email_change_confirmed" {
				require.NoError(t, initiate(ctx))
				// The row stores only the hash of the link (#4902); the
				// confirmation follows the link the verification mail carries.
				var mail string
				require.NoError(t, db.NewRaw(`SELECT payload::text FROM platform.email_outbox WHERE kind = ? AND payload->'Message'->'To'->>'Address' = ? ORDER BY id DESC LIMIT 1`,
					owners.TransactionalMailKind, newAddress).Scan(ctx, &mail))
				match := emailChangeLinkToken.FindStringSubmatch(mail)
				require.Len(t, match, 2, "the verification mail carries a link")
				token := match[1]
				trigger = func(txCtx context.Context) error {
					_, err := module.ConfirmOperatorEmailChange(txCtx, token, "127.0.0.1")
					return err
				}
			}
			assertTransactionalMails(t, db, ctx, true, address, []string{kind}, trigger)
		})
	}
}

var emailChangeLinkToken = regexp.MustCompile(`email-confirm\?token=([A-Za-z0-9_-]+)`)
