package compose_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPresence_ListAcceptedRetentionSettings(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	// insertConsentForTenant inserts a privacy consent row with an explicit
	// accepted flag and retention window under the supplied tenant.
	insertConsentForTenant := func(t *testing.T, tenantID, studentID int64, policyVersion string, accepted bool, retentionDays int) {
		t.Helper()
		_, err := db.NewRaw(`
		INSERT INTO users.privacy_consents (student_id, policy_version, accepted, renewal_required, data_retention_days, tenant_id, created_at, updated_at)
		VALUES (?, ?, ?, false, ?, ?, NOW(), NOW())
	`, studentID, policyVersion, accepted, retentionDays, tenantID).Exec(testpkg.TenantContext(tenantID))
		require.NoError(t, err)
	}

	repo, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
	require.NoError(t, err)

	tenantID := testpkg.UniqueTestTenantID(t)
	otherTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	testpkg.EnsureTestTenant(t, db, otherTenantID)
	t.Cleanup(func() {
		// Consents reference students, so they must go before
		// CleanupTenantTestData deletes the student rows.
		for _, tid := range []int64{tenantID, otherTenantID} {
			_, _ = db.NewDelete().
				Table("users.privacy_consents").
				Where("tenant_id = ?", tid).
				Exec(testpkg.TenantContext(tid))
		}
	})

	ctx := testpkg.TenantContext(tenantID)

	t.Run("empty when tenant has no accepted consents", func(t *testing.T) {
		settings, err := repo.ListAcceptedRetentionSettings(ctx)
		require.NoError(t, err)
		assert.Empty(t, settings)
	})

	studentA := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Retention", "Alpha", "1a")
	studentB := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Retention", "Beta", "1b")
	studentC := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Retention", "Gamma", "1c")
	foreign := testpkg.CreateTestStudentForTenant(t, db, otherTenantID, "Retention", "Foreign", "1d")

	// Two accepted consents with the same retention collapse via DISTINCT.
	insertConsentForTenant(t, tenantID, studentA.ID, "v1.0", true, 7)
	insertConsentForTenant(t, tenantID, studentA.ID, "v2.0", true, 7)
	insertConsentForTenant(t, tenantID, studentB.ID, "v1.0", true, 30)
	// Not accepted — excluded.
	insertConsentForTenant(t, tenantID, studentC.ID, "v1.0", false, 14)
	// Other tenant — excluded.
	insertConsentForTenant(t, otherTenantID, foreign.ID, "v1.0", true, 7)

	t.Run("returns distinct accepted pairs ordered by student", func(t *testing.T) {
		settings, err := repo.ListAcceptedRetentionSettings(ctx)
		require.NoError(t, err)
		assert.Equal(t, []studentpresence.StudentRetentionSetting{
			{StudentID: studentA.ID, DataRetentionDays: 7},
			{StudentID: studentB.ID, DataRetentionDays: 30},
		}, settings)
	})
}
