package migrations

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestStudentConsentChangesAreTenantScopedAndAppendOnly(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	var enabled, forced bool
	require.NoError(t, db.NewRaw(`
		SELECT relrowsecurity, relforcerowsecurity
		FROM pg_class
		WHERE oid = 'audit.student_consent_changes'::regclass
	`).Scan(ctx, &enabled, &forced))
	assert.True(t, enabled)
	assert.True(t, forced)

	for _, privilege := range []string{"SELECT", "INSERT"} {
		assert.True(t, tenantHasPrivilege(t, db, "audit.student_consent_changes", privilege))
	}
	for _, privilege := range []string{"UPDATE", "DELETE", "TRUNCATE"} {
		assert.False(t, tenantHasPrivilege(t, db, "audit.student_consent_changes", privilege))
	}

	var studentFKIndexed bool
	require.NoError(t, db.NewRaw(`
		SELECT EXISTS (
			SELECT 1
			FROM pg_indexes
			WHERE schemaname = 'audit'
			  AND tablename = 'student_consent_changes'
			  AND indexname = 'idx_student_consent_changes_student_fk'
		)
	`).Scan(ctx, &studentFKIndexed))
	assert.True(t, studentFKIndexed)

	tenantA, _ := testpkg.CreateTestTenant(t, db)
	tenantB, _ := testpkg.CreateTestTenant(t, db)
	studentA := testpkg.CreateTestStudentForTenant(t, db, tenantA, "Consent", "A", "1a")
	studentB := testpkg.CreateTestStudentForTenant(t, db, tenantB, "Consent", "B", "1b")
	_, err := db.ExecContext(ctx, `
		INSERT INTO audit.student_consent_changes
			(tenant_id, student_id, consent_key, action, source)
		VALUES (?, ?, 'photo', 'granted', 'tenant_portal'),
		       (?, ?, 'photo', 'withdrawn', 'parent_portal')
	`, tenantA, studentA.ID, tenantB, studentB.ID)
	require.NoError(t, err)
	assertTenantRowsIsolated(t, db, "audit.student_consent_changes", tenantA, `
		INSERT INTO audit.student_consent_changes
			(tenant_id, student_id, consent_key, action, source)
		VALUES (?, ?, 'photo', 'withdrawn', 'parent_portal')
	`, tenantB, studentB.ID)
}
