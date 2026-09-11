package migrations

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestParentRequestEventsAreTenantScopedAndAppendOnly(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	var enabled, forced bool
	require.NoError(t, db.NewRaw(`
		SELECT relrowsecurity, relforcerowsecurity
		FROM pg_class
		WHERE oid = 'users.parent_request_events'::regclass
	`).Scan(ctx, &enabled, &forced))
	assert.True(t, enabled)
	assert.True(t, forced)

	for _, privilege := range []string{"SELECT", "INSERT"} {
		assert.True(t, tenantHasPrivilege(t, db, "users.parent_request_events", privilege))
	}
	// A history staff can rewrite is not a history.
	for _, privilege := range []string{"UPDATE", "DELETE", "TRUNCATE"} {
		assert.False(t, tenantHasPrivilege(t, db, "users.parent_request_events", privilege))
	}

	tenantA, _ := testpkg.CreateTestTenant(t, db)
	tenantB, _ := testpkg.CreateTestTenant(t, db)
	studentA := testpkg.CreateTestStudentForTenant(t, db, tenantA, "Event", "A", "1a")
	studentB := testpkg.CreateTestStudentForTenant(t, db, tenantB, "Event", "B", "1b")
	actor := testpkg.CreateTestAccount(t, db, "parent-request-event-actor@example.com")

	_, err := db.ExecContext(ctx, `
		INSERT INTO users.parent_request_events
			(tenant_id, student_id, request_type, request_id, event_type, actor_account_id, version)
		VALUES (?, ?, 'excused', 1, 'submitted', ?, 'v1'),
		       (?, ?, 'excused', 2, 'submitted', ?, 'v1')
	`, tenantA, studentA.ID, actor.ID, tenantB, studentB.ID, actor.ID)
	require.NoError(t, err)

	// An unknown event type must not reach the ledger: the recorder's constants
	// and this CHECK have to stay in step.
	_, err = db.ExecContext(ctx, `
		INSERT INTO users.parent_request_events
			(tenant_id, student_id, request_type, request_id, event_type)
		VALUES (?, ?, 'excused', 3, 'invented')
	`, tenantA, studentA.ID)
	require.Error(t, err)

	_, err = db.ExecContext(ctx, `
		INSERT INTO users.parent_request_events
			(tenant_id, student_id, request_type, request_id, event_type)
		VALUES (?, ?, 'not_a_request_type', 4, 'submitted')
	`, tenantA, studentA.ID)
	require.Error(t, err)

	assertTenantRowsIsolated(t, db, "users.parent_request_events", tenantA, `
		INSERT INTO users.parent_request_events
			(tenant_id, student_id, request_type, request_id, event_type, actor_account_id)
		VALUES (?, ?, 'excused', 5, 'decided', ?)
	`, tenantB, studentB.ID, actor.ID)
}
