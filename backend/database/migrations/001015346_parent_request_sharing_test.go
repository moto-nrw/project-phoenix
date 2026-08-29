package migrations

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestParentRequestShareEventsAreTenantScopedAndAppendOnly(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	var enabled, forced bool
	require.NoError(t, db.NewRaw(`
		SELECT relrowsecurity, relforcerowsecurity
		FROM pg_class
		WHERE oid = 'users.parent_request_share_events'::regclass
	`).Scan(ctx, &enabled, &forced))
	assert.True(t, enabled)
	assert.True(t, forced)

	for _, privilege := range []string{"SELECT", "INSERT"} {
		assert.True(t, tenantHasPrivilege(t, db, "users.parent_request_share_events", privilege))
	}
	for _, privilege := range []string{"UPDATE", "DELETE", "TRUNCATE"} {
		assert.False(t, tenantHasPrivilege(t, db, "users.parent_request_share_events", privilege))
	}

	tenantA, _ := testpkg.CreateTestTenant(t, db)
	tenantB, _ := testpkg.CreateTestTenant(t, db)
	studentA := testpkg.CreateTestStudentForTenant(t, db, tenantA, "Share", "A", "1a")
	studentB := testpkg.CreateTestStudentForTenant(t, db, tenantB, "Share", "B", "1b")
	author := testpkg.CreateTestAccount(t, db, "request-sharing-author@example.com")
	recipient := testpkg.CreateTestAccount(t, db, "request-sharing-recipient@example.com")
	_, err := db.ExecContext(ctx, `
		INSERT INTO users.parent_request_share_events
			(tenant_id, student_id, request_type, request_id, author_account_id, recipient_account_ids)
		VALUES (?, ?, 'excused', 1, ?, ARRAY[?]::BIGINT[]),
		       (?, ?, 'excused', 2, ?, ARRAY[?]::BIGINT[])
	`, tenantA, studentA.ID, author.ID, recipient.ID, tenantB, studentB.ID, author.ID, recipient.ID)
	require.NoError(t, err)
	assertTenantRowsIsolated(t, db, "users.parent_request_share_events", tenantA, `
		INSERT INTO users.parent_request_share_events
			(tenant_id, student_id, request_type, request_id, author_account_id, recipient_account_ids)
		VALUES (?, ?, 'excused', 3, ?, ARRAY[?]::BIGINT[])
	`, tenantB, studentB.ID, author.ID, recipient.ID)
}
