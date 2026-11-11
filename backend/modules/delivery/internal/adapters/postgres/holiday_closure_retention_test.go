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
