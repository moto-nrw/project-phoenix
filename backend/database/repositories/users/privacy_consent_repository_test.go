package users_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// ============================================================================
// Setup Helpers
// ============================================================================

// ============================================================================
// CRUD Tests
// ============================================================================

func TestPrivacyConsentRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).PrivacyConsent
	ctx := testpkg.TenantContext(1)

	t.Run("creates consent with valid data", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Consent", "Create", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID, student.PersonID)

		now := time.Now()
		consent := &users.PrivacyConsent{
			StudentID:         student.ID,
			PolicyVersion:     "v1.0",
			Accepted:          true,
			AcceptedAt:        &now,
			DataRetentionDays: 30,
		}

		err := repo.Create(ctx, consent)
		require.NoError(t, err)
		assert.NotZero(t, consent.ID)

		testpkg.CleanupTableRecords(t, db, "users.privacy_consents", consent.ID)
	})

	t.Run("creates consent with expiry date", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Consent", "Expiry", "1b")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID, student.PersonID)

		now := time.Now()
		expiresAt := now.AddDate(1, 0, 0)

		consent := &users.PrivacyConsent{
			StudentID:         student.ID,
			PolicyVersion:     "v1.0",
			Accepted:          true,
			AcceptedAt:        &now,
			ExpiresAt:         &expiresAt,
			DataRetentionDays: 30,
		}

		err := repo.Create(ctx, consent)
		require.NoError(t, err)
		assert.NotZero(t, consent.ID)
		assert.NotNil(t, consent.ExpiresAt)

		testpkg.CleanupTableRecords(t, db, "users.privacy_consents", consent.ID)
	})

	t.Run("fails with nil consent", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("fails without student ID", func(t *testing.T) {
		consent := &users.PrivacyConsent{
			PolicyVersion:     "v1.0",
			DataRetentionDays: 30,
		}

		err := repo.Create(ctx, consent)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "student ID")
	})

	t.Run("fails without policy version", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Consent", "NoPol", "1c")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID, student.PersonID)

		consent := &users.PrivacyConsent{
			StudentID:         student.ID,
			DataRetentionDays: 30,
		}

		err := repo.Create(ctx, consent)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "policy version")
	})

	t.Run("fails with invalid data retention days", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Consent", "BadDays", "1d")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID, student.PersonID)

		consent := &users.PrivacyConsent{
			StudentID:         student.ID,
			PolicyVersion:     "v1.0",
			DataRetentionDays: 100, // Must be 1-31
		}

		err := repo.Create(ctx, consent)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "data retention")
	})
}

func TestPrivacyConsentRepository_FindByID(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).PrivacyConsent
	ctx := testpkg.TenantContext(1)

	t.Run("finds existing consent", func(t *testing.T) {
		consent := testpkg.CreateTestPrivacyConsent(t, db, "FindByID")
		defer testpkg.CleanupTableRecords(t, db, "users.privacy_consents", consent.ID)
		defer testpkg.CleanupActivityFixtures(t, db, consent.StudentID, consent.Student.PersonID)

		found, err := repo.FindByID(ctx, consent.ID)
		require.NoError(t, err)
		assert.Equal(t, consent.ID, found.ID)
		assert.Equal(t, consent.StudentID, found.StudentID)
	})

	t.Run("returns error for non-existent consent", func(t *testing.T) {
		_, err := repo.FindByID(ctx, int64(999999))
		require.Error(t, err)
	})
}

func TestPrivacyConsentRepository_FindByStudentID(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).PrivacyConsent
	ctx := testpkg.TenantContext(1)

	t.Run("finds consents by student ID", func(t *testing.T) {
		consent := testpkg.CreateTestPrivacyConsent(t, db, "ByStudent")
		defer testpkg.CleanupTableRecords(t, db, "users.privacy_consents", consent.ID)
		defer testpkg.CleanupActivityFixtures(t, db, consent.StudentID, consent.Student.PersonID)

		found, err := repo.FindByStudentID(ctx, consent.StudentID)
		require.NoError(t, err)
		assert.NotEmpty(t, found)

		var foundConsent bool
		for _, c := range found {
			if c.ID == consent.ID {
				foundConsent = true
				break
			}
		}
		assert.True(t, foundConsent)
	})

	t.Run("returns empty for non-existent student", func(t *testing.T) {
		found, err := repo.FindByStudentID(ctx, int64(999999))
		require.NoError(t, err)
		assert.Empty(t, found)
	})
}

func TestPrivacyConsentRepository_Update(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).PrivacyConsent
	ctx := testpkg.TenantContext(1)

	t.Run("updates consent", func(t *testing.T) {
		consent := testpkg.CreateTestPrivacyConsent(t, db, "Update")
		defer testpkg.CleanupTableRecords(t, db, "users.privacy_consents", consent.ID)
		defer testpkg.CleanupActivityFixtures(t, db, consent.StudentID, consent.Student.PersonID)

		consent.RenewalRequired = true
		consent.DataRetentionDays = 15

		err := repo.Update(ctx, consent)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, consent.ID)
		require.NoError(t, err)
		assert.True(t, found.RenewalRequired)
		assert.Equal(t, 15, found.DataRetentionDays)
	})

	t.Run("fails with nil consent", func(t *testing.T) {
		err := repo.Update(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
}

// ============================================================================
// Consent Management Tests
// ============================================================================

func TestPrivacyConsentRepository_Accept(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).PrivacyConsent
	ctx := testpkg.TenantContext(1)

	t.Run("accepts consent", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Accept", "Consent", "2a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID, student.PersonID)

		consent := &users.PrivacyConsent{
			StudentID:         student.ID,
			PolicyVersion:     "v1.0",
			Accepted:          false,
			DataRetentionDays: 30,
		}
		err := repo.Create(ctx, consent)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "users.privacy_consents", consent.ID)

		acceptedAt := time.Now()
		err = repo.Accept(ctx, consent.ID, acceptedAt)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, consent.ID)
		require.NoError(t, err)
		assert.True(t, found.Accepted)
		assert.NotNil(t, found.AcceptedAt)
	})
}

func TestPrivacyConsentRepository_Revoke(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).PrivacyConsent
	ctx := testpkg.TenantContext(1)

	t.Run("revokes consent", func(t *testing.T) {
		consent := testpkg.CreateTestPrivacyConsent(t, db, "Revoke")
		defer testpkg.CleanupTableRecords(t, db, "users.privacy_consents", consent.ID)
		defer testpkg.CleanupActivityFixtures(t, db, consent.StudentID, consent.Student.PersonID)

		err := repo.Revoke(ctx, consent.ID)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, consent.ID)
		require.NoError(t, err)
		assert.False(t, found.Accepted)
	})
}

// ============================================================================
// Query Tests
// ============================================================================

func TestPrivacyConsentRepository_FindActiveByStudentID(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).PrivacyConsent
	ctx := testpkg.TenantContext(1)

	t.Run("finds active consents for student", func(t *testing.T) {
		consent := testpkg.CreateTestPrivacyConsent(t, db, "Active")
		defer testpkg.CleanupTableRecords(t, db, "users.privacy_consents", consent.ID)
		defer testpkg.CleanupActivityFixtures(t, db, consent.StudentID, consent.Student.PersonID)

		found, err := repo.FindActiveByStudentID(ctx, consent.StudentID)
		require.NoError(t, err)

		var foundConsent bool
		for _, c := range found {
			if c.ID == consent.ID {
				foundConsent = true
				break
			}
		}
		assert.True(t, foundConsent)
	})

	// NOTE: Database has check constraint preventing expired dates at insert time,
	// so we can only test by setting expiry date after creation via SetExpiryDate
}

// ============================================================================
// List and Filter Tests
// ============================================================================

func TestPrivacyConsentRepository_List(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).PrivacyConsent
	ctx := testpkg.TenantContext(1)

	t.Run("lists with accepted filter", func(t *testing.T) {
		consent := testpkg.CreateTestPrivacyConsent(t, db, "FilterAccepted")
		defer testpkg.CleanupTableRecords(t, db, "users.privacy_consents", consent.ID)
		defer testpkg.CleanupActivityFixtures(t, db, consent.StudentID, consent.Student.PersonID)

		filters := map[string]interface{}{
			"accepted": true,
		}

		found, err := repo.List(ctx, filters)
		require.NoError(t, err)

		// All returned consents should be accepted
		for _, c := range found {
			assert.True(t, c.Accepted)
		}
	})

	t.Run("lists with policy_version filter", func(t *testing.T) {
		consent := testpkg.CreateTestPrivacyConsent(t, db, "FilterVersion")
		defer testpkg.CleanupTableRecords(t, db, "users.privacy_consents", consent.ID)
		defer testpkg.CleanupActivityFixtures(t, db, consent.StudentID, consent.Student.PersonID)

		filters := map[string]interface{}{
			"policy_version": "v1.0",
		}

		found, err := repo.List(ctx, filters)
		require.NoError(t, err)

		// All returned consents should have v1.0 policy
		for _, c := range found {
			assert.Equal(t, "v1.0", c.PolicyVersion)
		}
	})
}

// ============================================================================
// ListAcceptedRetentionSettings Tests
// ============================================================================

// insertConsentForTenant inserts a privacy consent row with an explicit
// accepted flag and retention window under the supplied tenant.
func insertConsentForTenant(t *testing.T, db *bun.DB, tenantID, studentID int64, policyVersion string, accepted bool, retentionDays int) {
	t.Helper()
	_, err := db.NewRaw(`
		INSERT INTO users.privacy_consents (student_id, policy_version, accepted, renewal_required, data_retention_days, tenant_id, created_at, updated_at)
		VALUES (?, ?, ?, false, ?, ?, NOW(), NOW())
	`, studentID, policyVersion, accepted, retentionDays, tenantID).Exec(testpkg.TenantContext(tenantID))
	require.NoError(t, err)
}

func TestPrivacyConsentRepository_ListAcceptedRetentionSettings(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).PrivacyConsent

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
		testpkg.CleanupTenantTestData(t, db, tenantID, otherTenantID)
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
	insertConsentForTenant(t, db, tenantID, studentA.ID, "v1.0", true, 7)
	insertConsentForTenant(t, db, tenantID, studentA.ID, "v2.0", true, 7)
	insertConsentForTenant(t, db, tenantID, studentB.ID, "v1.0", true, 30)
	// Not accepted — excluded.
	insertConsentForTenant(t, db, tenantID, studentC.ID, "v1.0", false, 14)
	// Other tenant — excluded.
	insertConsentForTenant(t, db, otherTenantID, foreign.ID, "v1.0", true, 7)

	t.Run("returns distinct accepted pairs ordered by student", func(t *testing.T) {
		settings, err := repo.ListAcceptedRetentionSettings(ctx)
		require.NoError(t, err)
		assert.Equal(t, []users.StudentRetentionSetting{
			{StudentID: studentA.ID, DataRetentionDays: 7},
			{StudentID: studentB.ID, DataRetentionDays: 30},
		}, settings)
	})
}
