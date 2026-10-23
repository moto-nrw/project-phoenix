package contracttest_test

import (
	"testing"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	"github.com/moto-nrw/project-phoenix/api"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
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

func TestExternalCaregiverRecordRepository_Create(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db)).ExternalCaregiverRecords
	ctx := testpkg.Ctx(t)

	t.Run("creates record with valid data", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Extern", "Create")

		record := &peopleValues.ExternalCaregiverRecord{
			StaffID:           staff.ID,
			ActivityExpertise: "Music",
			Organization:      "Test Org",
		}

		err := testpkg.TenantCommand1(t, ctx, db, repo.Create, record)
		require.NoError(t, err)
		assert.NotZero(t, record.ID)

	})

	t.Run("creates record with contact info", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Extern", "Contact")

		record := &peopleValues.ExternalCaregiverRecord{
			StaffID:           staff.ID,
			ActivityExpertise: "Art",
			ContactEmail:      "extern@test.local",
			ContactPhone:      "+49123456789",
		}

		err := testpkg.TenantCommand1(t, ctx, db, repo.Create, record)
		require.NoError(t, err)
		assert.NotZero(t, record.ID)

	})

	t.Run("creates record with date range", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Extern", "Dates")

		startDate := calendar.TodayDate()
		endDate := startDate.AddDays(90)

		record := &peopleValues.ExternalCaregiverRecord{
			StaffID:           staff.ID,
			ActivityExpertise: "Dance",
			StartDate:         &startDate,
			EndDate:           &endDate,
		}

		err := testpkg.TenantCommand1(t, ctx, db, repo.Create, record)
		require.NoError(t, err)
		assert.NotZero(t, record.ID)
		assert.NotNil(t, record.StartDate)
		assert.NotNil(t, record.EndDate)

	})

	t.Run("fails with nil record", func(t *testing.T) {
		err := testpkg.TenantCommand1(t, ctx, db, repo.Create, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("fails without staff ID", func(t *testing.T) {
		record := &peopleValues.ExternalCaregiverRecord{
			ActivityExpertise: "Music",
		}

		err := testpkg.TenantCommand1(t, ctx, db, repo.Create, record)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "staff ID")
	})

	t.Run("fails without activity expertise", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Extern", "NoExp")

		record := &peopleValues.ExternalCaregiverRecord{
			StaffID: staff.ID,
		}

		err := testpkg.TenantCommand1(t, ctx, db, repo.Create, record)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "activity expertise")
	})

	t.Run("fails with invalid email", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Extern", "BadEmail")

		record := &peopleValues.ExternalCaregiverRecord{
			StaffID:           staff.ID,
			ActivityExpertise: "Music",
			ContactEmail:      "not-an-email",
		}

		err := testpkg.TenantCommand1(t, ctx, db, repo.Create, record)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "email")
	})
}

func TestExternalCaregiverRecordRepository_FindByID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db)).ExternalCaregiverRecords
	ctx := testpkg.Ctx(t)

	t.Run("finds existing record", func(t *testing.T) {
		record := testpkg.CreateTestExternalCaregiverRecord(t, db, "FindByID")

		found, err := testpkg.TenantQuery1(t, ctx, db, repo.FindByID, any(record.ID))
		require.NoError(t, err)
		assert.Equal(t, record.ID, found.ID)
		assert.Equal(t, record.ActivityExpertise, found.ActivityExpertise)
	})

	t.Run("returns error for non-existent record", func(t *testing.T) {
		_, err := testpkg.TenantQuery1(t, ctx, db, repo.FindByID, any(int64(999999)))
		require.Error(t, err)
	})
}

func TestExternalCaregiverRecordRepository_FindByStaffID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db)).ExternalCaregiverRecords
	ctx := testpkg.Ctx(t)

	t.Run("finds record by staff ID", func(t *testing.T) {
		record := testpkg.CreateTestExternalCaregiverRecord(t, db, "FindByStaff")

		found, err := testpkg.TenantQuery1(t, ctx, db, repo.FindByStaffID, record.StaffID)
		require.NoError(t, err)
		assert.Equal(t, record.ID, found.ID)
		assert.Equal(t, record.StaffID, found.StaffID)
	})

	t.Run("returns error for non-existent staff ID", func(t *testing.T) {
		_, err := testpkg.TenantQuery1(t, ctx, db, repo.FindByStaffID, int64(999999))
		require.Error(t, err)
	})
}

func TestExternalCaregiverRecordRepository_Update(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db)).ExternalCaregiverRecords
	ctx := testpkg.Ctx(t)

	t.Run("updates record", func(t *testing.T) {
		recordFixture := testpkg.CreateTestExternalCaregiverRecord(t, db, "Update")
		record, err := testpkg.TenantQuery1(t, ctx, db, repo.FindByID, any(recordFixture.ID))
		require.NoError(t, err)

		record.ActivityExpertise = "Updated Expertise"
		record.Organization = "Updated Org"

		err = testpkg.TenantCommand1(t, ctx, db, repo.Update, record)
		require.NoError(t, err)

		found, err := testpkg.TenantQuery1(t, ctx, db, repo.FindByID, any(record.ID))
		require.NoError(t, err)
		assert.Equal(t, "Updated Expertise", found.ActivityExpertise)
		assert.Equal(t, "Updated Org", found.Organization)
	})

	t.Run("fails with nil record", func(t *testing.T) {
		err := testpkg.TenantCommand1(t, ctx, db, repo.Update, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
}

func TestExternalCaregiverRecordRepository_Delete(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db)).ExternalCaregiverRecords
	ctx := testpkg.Ctx(t)

	t.Run("deletes existing record", func(t *testing.T) {
		record := testpkg.CreateTestExternalCaregiverRecord(t, db, "Delete")

		err := testpkg.TenantCommand1(t, ctx, db, repo.Delete, any(record.ID))
		require.NoError(t, err)

		_, err = testpkg.TenantQuery1(t, ctx, db, repo.FindByID, any(record.ID))
		require.Error(t, err)
	})

	// NOTE: Base repository Delete returns nil for non-existent records
}

// ============================================================================
// Query Tests
// ============================================================================

// ============================================================================
// List and Filter Tests
// ============================================================================

func TestExternalCaregiverRecordRepository_List(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db)).ExternalCaregiverRecords
	ctx := testpkg.Ctx(t)

	t.Run("lists records with organization filter", func(t *testing.T) {
		recordFixture := testpkg.CreateTestExternalCaregiverRecord(t, db, "ListOrg")
		record, err := testpkg.TenantQuery1(t, ctx, db, repo.FindByID, any(recordFixture.ID))
		require.NoError(t, err)

		record.Organization = "ListTestOrg"
		err = testpkg.TenantCommand1(t, ctx, db, repo.Update, record)
		require.NoError(t, err)

		found, err := testpkg.TenantQuery1(t, ctx, db, repo.List, map[string]interface{}{
			"organization_like": "ListTest",
		})
		require.NoError(t, err)
		assert.NotEmpty(t, found)
	})

	t.Run("lists records with expertise filter", func(t *testing.T) {
		recordFixture := testpkg.CreateTestExternalCaregiverRecord(t, db, "ListExp")
		record, err := testpkg.TenantQuery1(t, ctx, db, repo.FindByID, any(recordFixture.ID))
		require.NoError(t, err)

		record.ActivityExpertise = "ListTestExpertise"
		err = testpkg.TenantCommand1(t, ctx, db, repo.Update, record)
		require.NoError(t, err)

		found, err := testpkg.TenantQuery1(t, ctx, db, repo.List, map[string]interface{}{
			"expertise_like": "ListTest",
		})
		require.NoError(t, err)
		assert.NotEmpty(t, found)
	})
}
