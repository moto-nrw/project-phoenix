package migrations

import (
	"context"
	"testing"

	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEmailDeliveryEventGrants pins 1.15.758 (#5932): a school may read and
// expire its delivery data. Only the administrative worker writes it, because
// the school of an event comes from the mail it matches.
func TestEmailDeliveryEventGrants(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	for _, table := range []string{"platform.email_delivery_events", "platform.email_suppressions"} {
		privilege := func(action string) bool {
			var granted bool
			require.NoError(t, db.NewRaw(`SELECT has_table_privilege('phoenix_tenant', ?, ?)`, table, action).Scan(ctx, &granted))
			return granted
		}
		assert.True(t, privilege("SELECT"), table)
		assert.True(t, privilege("DELETE"), table)
		assert.False(t, privilege("INSERT"), table)
		assert.False(t, privilege("UPDATE"), table)

		var forced bool
		require.NoError(t, db.NewRaw(`SELECT relforcerowsecurity FROM pg_class WHERE oid = ?::regclass`, table).Scan(ctx, &forced))
		assert.True(t, forced, table)
	}
}

// TestEmailDeliveryEventsGoWithTheSchool erases a school that has a mail, an
// event and a suppressed address.
func TestEmailDeliveryEventsGoWithTheSchool(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	school := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, school)
	var mailID int64
	require.NoError(t, db.NewRaw(`INSERT INTO platform.email_outbox (tenant_id, kind, recipient, payload)
		VALUES (?, 'welcome', '{"address":"guardian@example.com"}', '{}') RETURNING id`, school).Scan(ctx, &mailID))
	_, err := db.ExecContext(ctx, `INSERT INTO platform.email_delivery_events (tenant_id, outbox_id, provider, provider_event_id, state, occurred_at)
		VALUES (?, ?, 'lettermint', 'e1', 'failed', NOW())`, school, mailID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO platform.email_suppressions (tenant_id, address, reason) VALUES (?, 'guardian@example.com', 'failed')`, school)
	require.NoError(t, err)

	require.Equal(t, "db_done", eraseSchoolAsCLI(t, db, school, beginErasure(t, db, school)).Status)

	count := func(table string) int {
		var rows int
		require.NoError(t, db.NewRaw(`SELECT count(*) FROM `+table+` WHERE tenant_id = ?`, school).Scan(ctx, &rows))
		return rows
	}
	assert.Zero(t, count("platform.email_delivery_events"))
	assert.Zero(t, count("platform.email_suppressions"))
}

// TestEmailDeliveryEventsMigrationRoundTrip rolls 1.15.758 back and forward:
// the down restores the 1.15.733 erasure, which still works.
func TestEmailDeliveryEventsMigrationRoundTrip(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	// The message attachments (1.15.936, #6685) extend the erasure functions
	// this test restores; they go first.
	// The pickup change notices (1.15.956, #6855) extend the erasure of 1.15.954; they go first.
	require.NoError(t, pickupChangeTeamNoticesDown(t.Context(), db))
	// The note notices (1.15.954, #6854) extend the erasure of 1.15.942; they go first.
	require.NoError(t, studentNoteTeamNoticesDown(t.Context(), db))
	require.NoError(t, messageAttachmentsDown(t.Context(), db))
	ctx := context.Background()
	exists := func(name string) bool {
		var found bool
		require.NoError(t, db.NewRaw(`SELECT to_regclass(?) IS NOT NULL`, name).Scan(ctx, &found))
		return found
	}

	// 1.15.942 (ADR 0196) extends the erasure functions of 1.15.811; it goes first.
	require.NoError(t, holidayClosureEventsDown(ctx, db))
	require.NoError(t, emailDeliveryEventsDown(ctx, db))
	assert.False(t, exists("platform.email_delivery_events"))
	assert.False(t, exists("platform.email_suppressions"))
	for _, entry := range readErasurePlan(t, db) {
		assert.NotContains(t, []string{"platform.email_delivery_events", "platform.email_suppressions"}, entry.Name)
	}
	rolledBack := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, rolledBack)
	require.Equal(t, "db_done", eraseSchoolAsCLI(t, db, rolledBack, beginErasure(t, db, rolledBack)).Status,
		"the restored erasure still works")

	require.NoError(t, emailDeliveryEventsUp(ctx, db))
	assert.True(t, exists("platform.email_delivery_events"))
	assert.True(t, exists("platform.email_suppressions"))
	names := map[string]bool{}
	for _, entry := range readErasurePlan(t, db) {
		names[entry.Name] = true
	}
	assert.True(t, names["platform.email_delivery_events"])
	assert.True(t, names["platform.email_suppressions"])
}
