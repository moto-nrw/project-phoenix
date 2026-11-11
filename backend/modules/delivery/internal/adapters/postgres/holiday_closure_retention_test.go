package postgres_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/adapters/postgres"
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
	require.Equal(t, []string{"dead_letter", "dead_letter", "sent"}, states)
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
		retried, err := raw.RetryFailedTraegerEmails(ctx, organization, []string{"closure-retention:dead_letter", "closure-retention:sent", "closure-retention:unknown"})
		require.NoError(t, err)
		assert.EqualValues(t, 1, retried, "only the failed original mail becomes due again")
		return nil
	}))
	require.NoError(t, db.NewRaw(`SELECT payload::text FROM platform.email_outbox WHERE organization_id = ? AND status = 'pending'`, organization).Scan(ctx, &payload))
	assert.Contains(t, payload, "family@example.org", "recovery retains the original content")
}
