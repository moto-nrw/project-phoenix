package postgres_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/domain"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestHolidayClosureRetentionKeepsRecoveryAndAcceptedKeys(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	store, ctx := testStore(t, db)
	organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: "Closure retention", Slug: fmt.Sprintf("closure-retention-%d", testpkg.Tenant(t)),
	}).ID
	ended := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	for _, mail := range []struct{ state, key, failure string }{
		{"sent", "sent", ""}, {"dead_letter", "dead_letter", "mailbox unavailable"},
		{"dead_letter", "unknown", deliveryContract.DispatchOutcomeUnknown}, {"cancelled", "cancelled", ""},
		{"cancelled", "suppressed", deliveryContract.MailHeldByList},
	} {
		_, err := db.ExecContext(ctx, `INSERT INTO platform.email_outbox (organization_id, kind, status, idempotency_key,
   recipient, payload, sent_at, dead_letter_at, cancelled_at, last_error)
   VALUES (?, 'holiday_closure_notice', ?, ?, '{"address":"family@example.org"}',
   '{"registration_ids":[],"recipient_email":"family@example.org"}', ?, ?, ?, ?)`,
			organization, mail.state, "closure-retention:"+mail.key, ended, ended, ended, mail.failure)
		require.NoError(t, err)
	}
	_, err := db.ExecContext(ctx, `INSERT INTO platform.email_delivery_events (outbox_id, provider, provider_event_id, state, occurred_at, diagnostic)
  SELECT id, 'lettermint', 'closure-retention-sent', 'delivered', ?, 'provider diagnostic'
  FROM platform.email_outbox WHERE organization_id = ? AND status = 'sent'`, ended, organization)
	require.NoError(t, err)
	require.NoError(t, tenant.WithAdminTx(ctx, db, func(ctx context.Context, _ bun.Tx) error {
		_, err := store.DeleteExpiredSystemEmails(ctx, ended.Add(31*24*time.Hour), 50)
		return err
	}))
	var states []string
	require.NoError(t, db.NewRaw(`SELECT status FROM platform.email_outbox WHERE organization_id = ? ORDER BY status`, organization).Scan(ctx, &states))
	require.Equal(t, []string{"cancelled", "dead_letter", "dead_letter", "sent"}, states)
	var payload, recipient string
	require.NoError(t, db.NewRaw(`SELECT payload::text, recipient::text FROM platform.email_outbox WHERE organization_id = ? AND status = 'sent'`, organization).Scan(ctx, &payload, &recipient))
	assert.JSONEq(t, `{"registration_ids":[]}`, payload, "successful mail keeps only the erasure links and delivery key")
	assert.JSONEq(t, `{}`, recipient)
	var events int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM platform.email_delivery_events AS event
  JOIN platform.email_outbox AS mail ON mail.id = event.outbox_id WHERE mail.organization_id = ?`, organization).Scan(ctx, &events))
	assert.Zero(t, events, "accepted mail's provider events expire with its content")

	raw := postgres.New(postgres.RequireTenantDatabase, postgres.RequireAdminDatabase)
	require.NoError(t, testpkg.WithinTraeger(ctx, organization, func(ctx context.Context) error {
		retried, err := raw.RetryFailedTraegerEmails(ctx, organization, []string{"closure-retention:dead_letter", "closure-retention:sent", "closure-retention:unknown", "closure-retention:suppressed"})
		require.NoError(t, err)
		assert.EqualValues(t, 2, retried, "known failures and suppression cancellations become due; accepted and unknown mail stay")
		return nil
	}))
	require.NoError(t, db.NewRaw(`SELECT payload::text FROM platform.email_outbox WHERE organization_id = ? AND idempotency_key = 'closure-retention:dead_letter'`, organization).Scan(ctx, &payload))
	assert.Contains(t, payload, "family@example.org", "recovery retains the original content")
}

func TestRecoverableCancellationKeepsHolidayClosureOrder(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	store, ctx := testStore(t, db)
	raw := postgres.New(postgres.RequireTenantDatabase, postgres.RequireAdminDatabase)
	for _, reason := range []string{deliveryContract.MailHeldByList, deliveryContract.MailHeldByStop, "guardian access revoked"} {
		t.Run(reason, func(t *testing.T) {
			testpkg.OwnTenant(t)
			organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: "Notice ordering", Slug: fmt.Sprintf("notice-order-%d", testpkg.Tenant(t))}).ID
			var first, second int64
			require.NoError(t, db.NewRaw(`INSERT INTO platform.email_outbox (organization_id, kind, idempotency_key, recipient, payload, status, last_error)
    VALUES (?, 'holiday_closure_notice', 'closure', '{"address":"family@example.org"}', '{}', 'cancelled', ?) RETURNING id`, organization, reason).Scan(ctx, &first))
			require.NoError(t, db.NewRaw(`INSERT INTO platform.email_outbox (organization_id, kind, idempotency_key, recipient, payload)
    VALUES (?, 'holiday_closure_notice', 'reopening', '{"address":"family@example.org"}', '{}') RETURNING id`, organization).Scan(ctx, &second))
			now := time.Now()
			claimed, err := store.Claim(ctx, domain.TransportEmail, 10, now, now.Add(time.Minute))
			require.NoError(t, err)
			if reason == "guardian access revoked" {
				require.Len(t, claimed, 1, "a permanent cancellation releases its correction")
				assert.Equal(t, second, claimed[0].ID)
				require.NoError(t, testpkg.WithinTraeger(ctx, organization, func(ctx context.Context) error {
					retried, err := raw.RetryFailedTraegerEmails(ctx, organization, []string{"reopening"})
					require.NoError(t, err)
					assert.Zero(t, retried, "a permanent cancellation must never be requeued")
					return nil
				}))
				_, err = store.FinalizeSent(ctx, domain.TransportEmail, second, *claimed[0].LeaseToken, json.RawMessage(`{}`), now)
				require.NoError(t, err)
				return
			}
			assert.Empty(t, claimed, "a retryable cancellation must not release its correction")
			require.NoError(t, testpkg.WithinTraeger(ctx, organization, func(ctx context.Context) error {
				states, err := raw.TraegerEmailStatusesByKey(ctx, organization, []string{"reopening"})
				require.NoError(t, err)
				require.Len(t, states, 1)
				assert.Equal(t, "dead_letter", states[0].Status, "the successor exposes the recoverable block")
				retried, err := raw.RetryFailedTraegerEmails(ctx, organization, []string{"reopening"})
				require.NoError(t, err)
				assert.EqualValues(t, 1, retried, "retrying the correction also recovers its cancelled predecessor")
				return nil
			}))
			now = time.Now()
			claimed, err = store.Claim(ctx, domain.TransportEmail, 10, now, now.Add(time.Minute))
			require.NoError(t, err)
			require.Len(t, claimed, 1)
			assert.Equal(t, first, claimed[0].ID)
			sent, err := store.FinalizeSent(ctx, domain.TransportEmail, first, *claimed[0].LeaseToken, json.RawMessage(`{}`), now)
			require.NoError(t, err)
			require.True(t, sent)
			claimed, err = store.Claim(ctx, domain.TransportEmail, 10, now, now.Add(time.Minute))
			require.NoError(t, err)
			require.Len(t, claimed, 1)
			assert.Equal(t, second, claimed[0].ID)
			_, err = store.FinalizeSent(ctx, domain.TransportEmail, second, *claimed[0].LeaseToken, json.RawMessage(`{}`), now)
			require.NoError(t, err)
		})
	}
}

// A second retry can see a dead letter while the first retry is still
// committing. Its stale candidate must not reset the newer transport state.
func TestConcurrentTraegerRetryPreservesNewerMailState(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	_, ctx := testStore(t, db)
	raw := postgres.New(postgres.RequireTenantDatabase, postgres.RequireAdminDatabase)
	organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: "Concurrent retry", Slug: fmt.Sprintf("concurrent-retry-%d", testpkg.Tenant(t)),
	}).ID
	for _, state := range []string{"pending", "claimed", "sent", "dead_letter"} {
		t.Run(state, func(t *testing.T) {
			var id int64
			require.NoError(t, db.NewRaw(`INSERT INTO platform.email_outbox
    (organization_id, kind, idempotency_key, recipient, payload, status, last_error)
    VALUES (?, 'holiday_closure_notice', ?, jsonb_build_object('address', ?), '{}', 'dead_letter', 'mailbox unavailable')
    RETURNING id`, organization, state, state+"@example.org").Scan(ctx, &id))
			first, err := db.BeginTx(ctx, nil)
			require.NoError(t, err)
			defer func() { _ = first.Rollback() }()
			_, err = first.ExecContext(ctx, `UPDATE platform.email_outbox SET status=?, last_error=?,
    lease_token=CASE WHEN ?='claimed' THEN 'newer-lease' END,
    lease_expires_at=CASE WHEN ?='claimed' THEN NOW()+interval '1 minute' END WHERE id=?`, state, deliveryContract.DispatchOutcomeUnknown, state, state, id)
			require.NoError(t, err)
			pid := make(chan int, 1)
			type outcome struct {
				retried int64
				err     error
			}
			finished := make(chan outcome, 1)
			retryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			go func() {
				var retried int64
				err := testpkg.WithinTraeger(retryCtx, organization, func(ctx context.Context) error {
					tx, _ := tenant.TransactionFromContext(ctx)
					var backendPID int
					if err := tx.(bun.Tx).NewRaw(`SELECT pg_backend_pid()`).Scan(ctx, &backendPID); err != nil {
						return err
					}
					pid <- backendPID
					var err error
					retried, err = raw.RetryFailedTraegerEmails(ctx, organization, []string{state})
					return err
				})
				finished <- outcome{retried, err}
			}()
			var backendPID int
			select {
			case backendPID = <-pid:
			case <-retryCtx.Done():
				t.Fatal(retryCtx.Err())
			}
			require.Eventually(t, func() bool {
				var waiting bool
				err := db.NewRaw(`SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE pid=? AND wait_event_type='Lock')`, backendPID).Scan(ctx, &waiting)
				return err == nil && waiting
			}, 5*time.Second, 10*time.Millisecond, "the second retry must select the old dead letter before the first commits")
			require.NoError(t, first.Commit())
			result := <-finished
			require.NoError(t, result.err)
			assert.Zero(t, result.retried, "a stale retry must not release mail already retried, claimed, sent or of unknown outcome")
			var storedState, lease string
			require.NoError(t, db.NewRaw(`SELECT status, COALESCE(lease_token, '') FROM platform.email_outbox WHERE id=?`, id).Scan(ctx, &storedState, &lease))
			assert.Equal(t, state, storedState)
			if state == "claimed" {
				assert.Equal(t, "newer-lease", lease)
			} else {
				assert.Empty(t, lease)
			}
		})
	}
}
