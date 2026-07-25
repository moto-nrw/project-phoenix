package migrations

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// phoenix_tenant may only READ delivery events. Every write path (webhook
// ingest, retention sweep) runs as phoenix_admin, so a tenant-role INSERT or
// UPDATE would mean a request handler could forge or rewrite a provider's
// verdict about an email.
func TestEmailDeliveryEvents_TenantRoleIsReadOnly(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()

	var canSelect bool
	require.NoError(t, db.NewRaw(`
		SELECT has_table_privilege('phoenix_tenant', 'platform.email_delivery_events', 'SELECT')
	`).Scan(ctx, &canSelect))
	assert.True(t, canSelect, "tenant requests must be able to read delivery status")

	for _, privilege := range []string{"INSERT", "UPDATE", "DELETE", "TRUNCATE"} {
		var granted bool
		require.NoError(t, db.NewRaw(`
			SELECT has_table_privilege('phoenix_tenant', 'platform.email_delivery_events', ?)
		`, privilege).Scan(ctx, &granted))
		assert.False(t, granted,
			"phoenix_tenant must not hold %s on platform.email_delivery_events", privilege)
	}
}

// RLS must be enabled AND forced: without FORCE, the table owner bypasses the
// policy, and several maintenance paths connect as the owner.
func TestEmailDeliveryEvents_RLSIsForced(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	var rlsEnabled, rlsForced bool
	require.NoError(t, db.NewRaw(`
		SELECT relrowsecurity, relforcerowsecurity
		FROM pg_class
		WHERE oid = 'platform.email_delivery_events'::regclass
	`).Scan(context.Background(), &rlsEnabled, &rlsForced))

	assert.True(t, rlsEnabled, "row level security must be enabled")
	assert.True(t, rlsForced, "row level security must be forced")
}

// The unique index is the wire-level replay guard, and it must be GLOBAL: the
// webhook is unauthenticated, so a per-tenant index would let a replayed event
// be applied a second time under a different tenant.
func TestEmailDeliveryEvents_ProviderEventIndexIsGlobal(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	var definition string
	require.NoError(t, db.NewRaw(`
		SELECT indexdef FROM pg_indexes
		WHERE schemaname = 'platform'
		  AND indexname = 'uq_email_delivery_events_provider_event'
	`).Scan(context.Background(), &definition))

	assert.Contains(t, definition, "UNIQUE")
	assert.Contains(t, definition, "provider")
	assert.Contains(t, definition, "provider_event_id")
	assert.NotContains(t, definition, "tenant_id",
		"the replay guard must not be scoped per tenant")
}

// A delivery status outside the vocabulary must be impossible to store: the
// UI branches on these values, and an unknown one would render as nothing.
func TestEmailOutbox_DeliveryStatusCheckConstraint(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	var exists bool
	require.NoError(t, db.NewRaw(`
		SELECT EXISTS (
			SELECT 1 FROM pg_constraint
			WHERE conrelid = 'platform.email_outbox'::regclass
			  AND conname = 'email_outbox_delivery_status_check'
		)
	`).Scan(context.Background(), &exists))

	assert.True(t, exists, "delivery_status must be constrained to the known vocabulary")
}

// Correlation depends on message_id being unambiguous — a duplicate would make
// a provider event resolvable to two different outbox rows.
func TestEmailOutbox_MessageIDIndexesAreUnique(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	for _, name := range []string{
		"uq_email_outbox_message_id",
		"uq_email_outbox_provider_message_id",
	} {
		var definition string
		require.NoError(t, db.NewRaw(`
			SELECT indexdef FROM pg_indexes
			WHERE schemaname = 'platform' AND indexname = ?
		`, name).Scan(context.Background(), &definition))
		assert.Contains(t, definition, "UNIQUE", "%s must be unique", name)
	}
}
