package api

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/delivery"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	"github.com/moto-nrw/project-phoenix/modules/delivery/compose/email"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type mfaOutboxTransport func(context.Context, email.Message) error

func (send mfaOutboxTransport) Send(message email.Message) error {
	return send(context.Background(), message)
}
func (send mfaOutboxTransport) SendContext(ctx context.Context, message email.Message) error {
	return send(ctx, message)
}

func TestMFACodeOutboxCommitsWithChallengeBeforeSynchronousDelivery(t *testing.T) {
	t.Parallel()
	for _, operator := range []bool{false, true} {
		t.Run(map[bool]string{false: "account", true: "operator"}[operator], func(t *testing.T) {
			testpkg.SetupIsolatedTestDB(t)
			db, ctx, outbox := mailTransactionFixture(t)
			var identityID int64
			kind := "mfa_email_code"
			challengeCount := func() int64 {
				var n int64
				var err error
				if operator {
					err = db.NewRaw(`SELECT count(*) FROM platform.operator_mfa_email_challenges WHERE operator_id = ?`, identityID).Scan(ctx, &n)
				} else {
					err = db.NewRaw(`SELECT count(*) FROM auth.mfa_email_challenges WHERE account_id = ?`, identityID).Scan(ctx, &n)
				}
				require.NoError(t, err)
				return n
			}
			calls := 0
			transport := mfaOutboxTransport(func(context.Context, email.Message) error {
				calls++
				assert.EqualValues(t, 1, challengeCount(), "the challenge is committed before SMTP")
				n, err := db.NewSelect().TableExpr("platform.email_outbox").Where("payload->>'Type' = ? AND related_entity_id = ?", kind, identityID).Count(ctx)
				require.NoError(t, err)
				assert.EqualValues(t, 1, n, "the entry is committed before SMTP")
				return nil
			})
			module, err := NewAuthTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), WithAuthTestMailer(transport), WithAuthTestTransactionalOutbox(outbox))
			require.NoError(t, err)
			if operator {
				identityID = testpkg.CreateTestOperator(t, db).ID
				kind = "operator_mfa_email_code"
			} else {
				identityID = testpkg.CreateTestAccount(t, db, "atomic-mfa-code").ID
			}
			t.Cleanup(func() {
				_, err := db.NewDelete().TableExpr("platform.email_outbox").Where("related_entity_type = ? AND related_entity_id = ?", kind, identityID).Exec(context.Background())
				require.NoError(t, err)
			})
			trigger := func() (string, error) {
				if operator {
					return module.OperatorMFA.StartOperatorMFAChallenge(ctx, identityID, net.ParseIP("203.0.113.77"))
				}
				return module.MFA.StartMFAChallenge(ctx, identityID, testpkg.Tenant(t), identityaccess.MFAChallengeScopeTenant, net.ParseIP("203.0.113.77"))
			}
			_, err = db.NewRaw(`ALTER TABLE platform.email_outbox ADD CONSTRAINT reject_mfa_test_enqueue CHECK (payload->>'Type' NOT IN ('mfa_email_code', 'operator_mfa_email_code'))`).Exec(ctx)
			require.NoError(t, err)
			token, err := trigger()
			require.Error(t, err)
			assert.Empty(t, token)
			assert.Zero(t, calls)
			assert.Zero(t, challengeCount(), "enqueue failure rolls the triggering challenge back")
			n, err := db.NewSelect().TableExpr("platform.email_outbox").Where("payload->>'Type' = ? AND related_entity_id = ?", kind, identityID).Count(ctx)
			require.NoError(t, err)
			assert.Zero(t, n)
			_, err = db.NewRaw(`ALTER TABLE platform.email_outbox DROP CONSTRAINT reject_mfa_test_enqueue`).Exec(ctx)
			require.NoError(t, err)
			token, err = trigger()
			require.NoError(t, err)
			assert.NotEmpty(t, token)
			assert.Equal(t, 1, calls)
			assert.EqualValues(t, 1, challengeCount())
			var state string
			require.NoError(t, db.NewRaw(`SELECT status FROM platform.email_outbox WHERE related_entity_type = ? AND related_entity_id = ?`, kind, identityID).Scan(ctx, &state))
			assert.Equal(t, "sent", state)
		})
	}
}

func mfaLeasedMailFixture(t *testing.T, send mfaOutboxTransport) (*bun.DB, context.Context, mfaMailer, *delivery.Worker) {
	t.Helper()
	testpkg.SetupIsolatedTestDB(t)
	db, ctx, _ := mailTransactionFixture(t)
	database := testpkg.NewDatabase(t, db)
	runtime, err := deliveryCompose.NewModule(deliveryCompose.ModuleDependencies{DB: db, People: enrollmentFlowGuardianDirectory{}, Observe: func(delivery.Observation) {}})
	require.NoError(t, err)
	provider := &deliveryCompose.MailPushProvider{Registry: deliveryCompose.NewEmailTemplateRegistry(map[string]deliveryCompose.MailRenderer{transactionalMailKind: transactionalMailRenderer{logger: slog.Default()}}), Mailer: send, Logger: slog.Default(), Tenants: deliveryCompose.NewTenantTransactions(db)}
	worker, err := deliveryCompose.NewWorker(runtime, provider)
	require.NoError(t, err)
	mail, err := newMFAMailer(database, runtime, send, email.Email{}, "", slog.Default(), []time.Duration{time.Millisecond})
	require.NoError(t, err)
	return db, ctx, mail, worker
}

func TestMFACodeLeasedFailureIsTerminalAndCrashIsRecovered(t *testing.T) {
	t.Parallel()
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "crash_before_claim", true: "transport_failure"}[fail], func(t *testing.T) {
			calls := 0
			refusal := errors.New("SMTP refused MFA code")
			db, ctx, mail, worker := mfaLeasedMailFixture(t, func(context.Context, email.Message) error {
				calls++
				if fail {
					return refusal
				}
				return nil
			})
			account := testpkg.CreateTestAccount(t, db, "leased-mfa-code")
			address := account.Email
			var send func(context.Context) error
			assertTransactionalMails(t, db, ctx, true, address, []string{"mfa_email_code"}, func(txCtx context.Context) error {
				var err error
				send, err = mail.EnqueueCode(txCtx, identityaccess.MFACodeMail{Recipient: address, ReferenceID: account.ID, Code: "123456", ExpiryMinutes: 5}, account.ID, time.Now().Add(5*time.Minute))
				return err
			})
			if fail {
				require.ErrorIs(t, send(ctx), refusal)
				assert.Equal(t, 3, calls, "preserve the existing interactive retry policy")
			}
			require.NoError(t, func() error { _, err := worker.RunOnce(ctx, 2, 6); return err }())
			var state string
			var attempts int
			require.NoError(t, db.NewRaw(`SELECT status, attempts FROM platform.email_outbox WHERE kind = ? AND related_entity_type = ? AND related_entity_id = ?`, transactionalMailKind, "mfa_email_code", account.ID).Scan(ctx, &state, &attempts))
			if fail {
				assert.Equal(t, "dead_letter", state)
				assert.Equal(t, 3, attempts)
				assert.Equal(t, 3, calls, "a reported failure is never retried by the worker")
			} else {
				assert.Equal(t, "sent", state)
				assert.Equal(t, 1, calls, "the worker recovers committed mail without a caller continuation")
			}
		})
	}
}

func TestTrustedDeviceNoticeOutboxIsAtomicWithDevice(t *testing.T) {
	t.Parallel()
	for _, operator := range []bool{false, true} {
		t.Run(map[bool]string{false: "account", true: "operator"}[operator], func(t *testing.T) {
			testpkg.SetupIsolatedTestDB(t)
			db, ctx, outbox := mailTransactionFixture(t)
			module, err := NewAuthTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), WithAuthTestTransactionalOutbox(outbox))
			require.NoError(t, err)
			var id int64
			kind := "mfa_trusted_device_added"
			if operator {
				id = testpkg.CreateTestOperator(t, db).ID
				kind = "operator_mfa_trusted_device_added"
			} else {
				id = testpkg.CreateTestAccount(t, db, "atomic-trusted-device").ID
			}
			t.Cleanup(func() {
				_, err := db.NewDelete().TableExpr("platform.email_outbox").Where("related_entity_type = ? AND related_entity_id = ?", kind, id).Exec(context.Background())
				require.NoError(t, err)
			})
			countDevices := func() int64 {
				var n int64
				var err error
				if operator {
					err = db.NewRaw(`SELECT count(*) FROM platform.operator_mfa_trusted_devices WHERE operator_id = ?`, id).Scan(ctx, &n)
				} else {
					err = db.NewRaw(`SELECT count(*) FROM auth.mfa_trusted_devices WHERE account_id = ?`, id).Scan(ctx, &n)
				}
				require.NoError(t, err)
				return n
			}
			trigger := func() (string, time.Time, error) {
				if operator {
					return module.OperatorMFA.IssueOperatorTrustedDevice(ctx, id, "Outbox test", net.ParseIP("203.0.113.78"))
				}
				return module.MFA.IssueTrustedDevice(ctx, id, testpkg.Tenant(t), "Outbox test", net.ParseIP("203.0.113.78"))
			}
			_, err = db.NewRaw(`ALTER TABLE platform.email_outbox ADD CONSTRAINT reject_device_notice_test CHECK (payload->>'Type' NOT IN ('mfa_trusted_device_added', 'operator_mfa_trusted_device_added'))`).Exec(ctx)
			require.NoError(t, err)
			cookie, _, err := trigger()
			require.Error(t, err)
			assert.Empty(t, cookie)
			assert.Zero(t, countDevices())
			n, err := db.NewSelect().TableExpr("platform.email_outbox").Where("related_entity_type = ? AND related_entity_id = ?", kind, id).Count(ctx)
			require.NoError(t, err)
			assert.Zero(t, n)
			_, err = db.NewRaw(`ALTER TABLE platform.email_outbox DROP CONSTRAINT reject_device_notice_test`).Exec(ctx)
			require.NoError(t, err)
			cookie, _, err = trigger()
			require.NoError(t, err)
			assert.NotEmpty(t, cookie)
			assert.EqualValues(t, 1, countDevices())
			n, err = db.NewSelect().TableExpr("platform.email_outbox").Where("related_entity_type = ? AND related_entity_id = ?", kind, id).Count(ctx)
			require.NoError(t, err)
			assert.EqualValues(t, 1, n)
		})
	}
}
