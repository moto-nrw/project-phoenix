package migrations

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/uptrace/bun"
)

func TestFamilyProtectionEventsAreTenantScopedAndAppendOnly(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	var enabled, forced bool
	require.NoError(t, db.NewRaw(`
		SELECT relrowsecurity, relforcerowsecurity
		FROM pg_class
		WHERE oid = 'users.student_family_protection_events'::regclass
	`).Scan(ctx, &enabled, &forced))
	assert.True(t, enabled)
	assert.True(t, forced)

	for _, privilege := range []string{"SELECT", "INSERT"} {
		assert.True(t, tenantHasPrivilege(t, db, "users.student_family_protection_events", privilege))
	}
	for _, privilege := range []string{"UPDATE", "DELETE", "TRUNCATE"} {
		assert.False(t, tenantHasPrivilege(t, db, "users.student_family_protection_events", privilege))
	}

	tenantA, _ := testpkg.CreateTestTenant(t, db)
	tenantB, _ := testpkg.CreateTestTenant(t, db)
	studentA := testpkg.CreateTestStudentForTenant(t, db, tenantA, "RLS", "A", "1a")
	studentB := testpkg.CreateTestStudentForTenant(t, db, tenantB, "RLS", "B", "1b")
	account := testpkg.CreateTestAccount(t, db, "family-protection-rls@example.com")
	_, err := db.ExecContext(ctx, `
		INSERT INTO users.student_family_protection_events
			(tenant_id, student_id, enabled, reason, actor_account_id)
		VALUES (?, ?, TRUE, 'A', ?), (?, ?, TRUE, 'B', ?)
	`, tenantA, studentA.ID, account.ID, tenantB, studentB.ID, account.ID)
	require.NoError(t, err)
	assertTenantRowsIsolated(t, db, "users.student_family_protection_events", tenantA, `
		INSERT INTO users.student_family_protection_events
			(tenant_id, student_id, enabled, reason, actor_account_id)
		VALUES (?, ?, TRUE, 'cross tenant', ?)
	`, tenantB, studentB.ID, account.ID)
}

func assertTenantRowsIsolated(
	t *testing.T, db *bun.DB, relation string, tenantID int64, crossTenantInsert string, args ...any,
) {
	t.Helper()
	tx, err := db.BeginTx(t.Context(), nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })
	_, err = tx.ExecContext(t.Context(), `SET LOCAL ROLE phoenix_tenant`)
	require.NoError(t, err)
	_, err = tx.ExecContext(t.Context(), `SELECT set_config('app.current_tenant_id', ?, true)`, strconv.FormatInt(tenantID, 10))
	require.NoError(t, err)
	var count int
	require.NoError(t, tx.NewRaw(fmt.Sprintf(`SELECT count(*) FROM %s`, relation)).Scan(t.Context(), &count))
	assert.Equal(t, 1, count, "tenant role must see only its own row")
	_, err = tx.ExecContext(t.Context(), crossTenantInsert, args...)
	require.Error(t, err, "tenant role must reject a cross-tenant insert")
}
