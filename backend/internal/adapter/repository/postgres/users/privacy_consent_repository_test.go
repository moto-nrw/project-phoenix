package users_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/adapter/repository/postgres"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Setup Helpers
// ============================================================================

// ============================================================================
// CRUD Tests
// ============================================================================

func TestPrivacyConsentRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).PrivacyConsent
	ctx := context.Background()

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
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).PrivacyConsent
	ctx := context.Background()

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
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).PrivacyConsent
	ctx := context.Background()

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

func TestPrivacyConsentRepository_FindByStudentIDAndPolicyVersion(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).PrivacyConsent
	ctx := context.Background()

	t.Run("finds consent by student ID and policy version", func(t *testing.T) {
		consent := testpkg.CreateTestPrivacyConsent(t, db, "ByVersion")
		defer testpkg.CleanupTableRecords(t, db, "users.privacy_consents", consent.ID)
		defer testpkg.CleanupActivityFixtures(t, db, consent.StudentID, consent.Student.PersonID)

		found, err := repo.FindByStudentIDAndPolicyVersion(ctx, consent.StudentID, "v1.0")
		require.NoError(t, err)
		assert.Equal(t, consent.ID, found.ID)
	})

	t.Run("returns error for non-existent combination", func(t *testing.T) {
		_, err := repo.FindByStudentIDAndPolicyVersion(ctx, int64(999999), "v99.0")
		require.Error(t, err)
	})
}

func TestPrivacyConsentRepository_Update(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).PrivacyConsent
	ctx := context.Background()

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
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).PrivacyConsent
	ctx := context.Background()

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
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).PrivacyConsent
	ctx := context.Background()

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

func TestPrivacyConsentRepository_SetExpiryDate(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).PrivacyConsent
	ctx := context.Background()

	t.Run("sets expiry date", func(t *testing.T) {
		consent := testpkg.CreateTestPrivacyConsent(t, db, "Expiry")
		defer testpkg.CleanupTableRecords(t, db, "users.privacy_consents", consent.ID)
		defer testpkg.CleanupActivityFixtures(t, db, consent.StudentID, consent.Student.PersonID)

		newExpiry := time.Now().AddDate(0, 6, 0)
		err := repo.SetExpiryDate(ctx, consent.ID, newExpiry)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, consent.ID)
		require.NoError(t, err)
		assert.NotNil(t, found.ExpiresAt)
	})
}

func TestPrivacyConsentRepository_SetRenewalRequired(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).PrivacyConsent
	ctx := context.Background()

	t.Run("sets renewal required to true", func(t *testing.T) {
		consent := testpkg.CreateTestPrivacyConsent(t, db, "Renewal")
		defer testpkg.CleanupTableRecords(t, db, "users.privacy_consents", consent.ID)
		defer testpkg.CleanupActivityFixtures(t, db, consent.StudentID, consent.Student.PersonID)

		err := repo.SetRenewalRequired(ctx, consent.ID, true)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, consent.ID)
		require.NoError(t, err)
		assert.True(t, found.RenewalRequired)
	})

	t.Run("sets renewal required to false", func(t *testing.T) {
		consent := testpkg.CreateTestPrivacyConsent(t, db, "NoRenewal")
		defer testpkg.CleanupTableRecords(t, db, "users.privacy_consents", consent.ID)
		defer testpkg.CleanupActivityFixtures(t, db, consent.StudentID, consent.Student.PersonID)

		// First set to true
		err := repo.SetRenewalRequired(ctx, consent.ID, true)
		require.NoError(t, err)

		// Then set to false
		err = repo.SetRenewalRequired(ctx, consent.ID, false)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, consent.ID)
		require.NoError(t, err)
		assert.False(t, found.RenewalRequired)
	})
}

func TestPrivacyConsentRepository_UpdateDetails(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).PrivacyConsent
	ctx := context.Background()

	t.Run("updates details with valid JSON", func(t *testing.T) {
		consent := testpkg.CreateTestPrivacyConsent(t, db, "Details")
		defer testpkg.CleanupTableRecords(t, db, "users.privacy_consents", consent.ID)
		defer testpkg.CleanupActivityFixtures(t, db, consent.StudentID, consent.Student.PersonID)

		details := `{"agreed_to_photos": true, "agreed_to_videos": false}`
		err := repo.UpdateDetails(ctx, consent.ID, details)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, consent.ID)
		require.NoError(t, err)
		assert.NotNil(t, found.Details)
	})

	t.Run("fails with invalid JSON", func(t *testing.T) {
		consent := testpkg.CreateTestPrivacyConsent(t, db, "BadDetails")
		defer testpkg.CleanupTableRecords(t, db, "users.privacy_consents", consent.ID)
		defer testpkg.CleanupActivityFixtures(t, db, consent.StudentID, consent.Student.PersonID)

		err := repo.UpdateDetails(ctx, consent.ID, "not-valid-json")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid details JSON")
	})
}

// ============================================================================
// Query Tests
// ============================================================================

func TestPrivacyConsentRepository_FindActiveByStudentID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).PrivacyConsent
	ctx := context.Background()

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

func TestPrivacyConsentRepository_FindExpired(t *testing.T) {
	// NOTE: Database has check constraint preventing expired dates at insert time.
	// FindExpired would work for consents that expired after creation.
	// We test that the query runs without error.

	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).PrivacyConsent
	ctx := context.Background()

	t.Run("runs query successfully", func(t *testing.T) {
		_, err := repo.FindExpired(ctx)
		require.NoError(t, err)
	})
}

func TestPrivacyConsentRepository_FindNeedingRenewal(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).PrivacyConsent
	ctx := context.Background()

	t.Run("finds consents needing renewal", func(t *testing.T) {
		consent := testpkg.CreateTestPrivacyConsent(t, db, "NeedRenewal")
		defer testpkg.CleanupTableRecords(t, db, "users.privacy_consents", consent.ID)
		defer testpkg.CleanupActivityFixtures(t, db, consent.StudentID, consent.Student.PersonID)

		// Set renewal required
		err := repo.SetRenewalRequired(ctx, consent.ID, true)
		require.NoError(t, err)

		found, err := repo.FindNeedingRenewal(ctx)
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
}

// ============================================================================
// List and Filter Tests
// ============================================================================

func TestPrivacyConsentRepository_List(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).PrivacyConsent
	ctx := context.Background()

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
