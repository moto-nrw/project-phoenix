package audit

import (
	"testing"
	"time"

	audit "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// CRUD Tests
// ============================================================================

func TestDataDeletionRepository_Create(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := NewDataDeletionRepository(NewRuntime(db, auditTestTenantID))
	ctx := testpkg.Ctx(t)

	// Create a test student for FK reference
	student := testpkg.CreateTestStudent(t, db, "Deletion", "Student", "1a")

	t.Run("creates data deletion with valid data", func(t *testing.T) {
		deletion := audit.NewDataDeletion(
			student.ID,
			audit.DeletionTypeVisitRetention,
			10,
			"system",
		)

		err := testpkg.TenantCommand1(t, ctx, db, repo.Create, deletion)
		require.NoError(t, err)
		assert.NotZero(t, deletion.ID)

	})

	t.Run("creates manual deletion record", func(t *testing.T) {
		studentID := student.ID
		deletion := &audit.DataDeletion{
			StudentID:      &studentID,
			DeletionType:   audit.DeletionTypeManual,
			RecordsDeleted: 5,
			DeletionReason: "Parent requested data deletion",
			DeletedBy:      "admin@example.com",
			DeletedAt:      time.Now(),
		}

		err := testpkg.TenantCommand1(t, ctx, db, repo.Create, deletion)
		require.NoError(t, err)
		assert.NotZero(t, deletion.ID)

	})

	t.Run("creates GDPR request deletion", func(t *testing.T) {
		deletion := audit.NewDataDeletion(
			student.ID,
			audit.DeletionTypeGDPRRequest,
			25,
			"admin@example.com",
		)
		deletion.DeletionReason = "GDPR Article 17 request"
		deletion.SetMetadata("request_id", "GDPR-2024-001")

		err := testpkg.TenantCommand1(t, ctx, db, repo.Create, deletion)
		require.NoError(t, err)
		assert.NotZero(t, deletion.ID)

	})

	t.Run("create with nil deletion should fail", func(t *testing.T) {
		err := testpkg.TenantCommand1(t, ctx, db, repo.Create, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
}

func TestDataDeletionRepository_FindByID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := NewDataDeletionRepository(NewRuntime(db, auditTestTenantID))
	ctx := testpkg.Ctx(t)

	student := testpkg.CreateTestStudent(t, db, "Find", "Student", "2a")

	t.Run("finds existing deletion", func(t *testing.T) {
		deletion := audit.NewDataDeletion(
			student.ID,
			audit.DeletionTypeVisitRetention,
			15,
			"system",
		)
		err := testpkg.TenantCommand1(t, ctx, db, repo.Create, deletion)
		require.NoError(t, err)

		found, err := testpkg.TenantQuery1(t, ctx, db, repo.FindByID, any(deletion.ID))
		require.NoError(t, err)
		assert.Equal(t, deletion.ID, found.ID)
		require.NotNil(t, found.StudentID)
		assert.Equal(t, student.ID, *found.StudentID)
	})

	t.Run("returns error for non-existent deletion", func(t *testing.T) {
		_, err := testpkg.TenantQuery1(t, ctx, db, repo.FindByID, any(int64(999999)))
		require.Error(t, err)
	})
}

// ============================================================================
// Query Tests
// ============================================================================

func TestDataDeletionRepository_FindByStudentID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := NewDataDeletionRepository(NewRuntime(db, auditTestTenantID))
	ctx := testpkg.Ctx(t)

	student1 := testpkg.CreateTestStudent(t, db, "Student", "One", "3a")
	student2 := testpkg.CreateTestStudent(t, db, "Student", "Two", "3b")

	t.Run("finds deletions by student ID", func(t *testing.T) {
		deletion1 := audit.NewDataDeletion(student1.ID, audit.DeletionTypeVisitRetention, 5, "system")
		deletion2 := audit.NewDataDeletion(student1.ID, audit.DeletionTypeManual, 3, "admin")
		deletion3 := audit.NewDataDeletion(student2.ID, audit.DeletionTypeGDPRRequest, 10, "admin")

		err := testpkg.TenantCommand1(t, ctx, db, repo.Create, deletion1)
		require.NoError(t, err)
		err = testpkg.TenantCommand1(t, ctx, db, repo.Create, deletion2)
		require.NoError(t, err)
		err = testpkg.TenantCommand1(t, ctx, db, repo.Create, deletion3)
		require.NoError(t, err)

		deletions, err := testpkg.TenantQuery1(t, ctx, db, repo.FindByStudentID, student1.ID)
		require.NoError(t, err)
		assert.Len(t, deletions, 2)

		for _, d := range deletions {
			require.NotNil(t, d.StudentID)
			assert.Equal(t, student1.ID, *d.StudentID)
		}
	})
}

func TestDataDeletionRepository_FindByDateRange(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := NewDataDeletionRepository(NewRuntime(db, auditTestTenantID))
	ctx := testpkg.Ctx(t)

	student := testpkg.CreateTestStudent(t, db, "Range", "Student", "4a")

	t.Run("finds deletions in date range", func(t *testing.T) {
		now := time.Now()
		weekAgo := now.AddDate(0, 0, -7)

		deletion := audit.NewDataDeletion(student.ID, audit.DeletionTypeVisitRetention, 5, "system")
		err := testpkg.TenantCommand1(t, ctx, db, repo.Create, deletion)
		require.NoError(t, err)

		deletions, err := testpkg.TenantQuery2(t, ctx, db, repo.FindByDateRange, weekAgo, now.Add(time.Hour))
		require.NoError(t, err)

		var found bool
		for _, d := range deletions {
			if d.ID == deletion.ID {
				found = true
				break
			}
		}
		assert.True(t, found)
	})
}

func TestDataDeletionRepository_FindByType(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := NewDataDeletionRepository(NewRuntime(db, auditTestTenantID))
	ctx := testpkg.Ctx(t)

	student := testpkg.CreateTestStudent(t, db, "Type", "Student", "5a")

	t.Run("finds deletions by type", func(t *testing.T) {
		visitRetention := audit.NewDataDeletion(student.ID, audit.DeletionTypeVisitRetention, 5, "system")
		manual := audit.NewDataDeletion(student.ID, audit.DeletionTypeManual, 3, "admin")

		err := testpkg.TenantCommand1(t, ctx, db, repo.Create, visitRetention)
		require.NoError(t, err)
		err = testpkg.TenantCommand1(t, ctx, db, repo.Create, manual)
		require.NoError(t, err)

		deletions, err := testpkg.TenantQuery1(t, ctx, db, repo.FindByType, audit.DeletionTypeVisitRetention)
		require.NoError(t, err)

		for _, d := range deletions {
			assert.Equal(t, audit.DeletionTypeVisitRetention, d.DeletionType)
		}

		var found bool
		for _, d := range deletions {
			if d.ID == visitRetention.ID {
				found = true
				break
			}
		}
		assert.True(t, found)
	})
}

func TestDataDeletionRepository_List(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := NewDataDeletionRepository(NewRuntime(db, auditTestTenantID))
	ctx := testpkg.Ctx(t)

	student := testpkg.CreateTestStudent(t, db, "List", "Student", "8a")

	t.Run("lists all deletions", func(t *testing.T) {
		deletion := audit.NewDataDeletion(student.ID, audit.DeletionTypeVisitRetention, 5, "system")
		err := testpkg.TenantCommand1(t, ctx, db, repo.Create, deletion)
		require.NoError(t, err)

		deletions, err := testpkg.TenantQuery1(t, ctx, db, repo.List, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, deletions)
	})

	t.Run("lists with filters", func(t *testing.T) {
		deletion := audit.NewDataDeletion(student.ID, audit.DeletionTypeManual, 8, "admin@test.com")
		err := testpkg.TenantCommand1(t, ctx, db, repo.Create, deletion)
		require.NoError(t, err)

		filters := map[string]interface{}{
			"deletion_type": audit.DeletionTypeManual,
		}
		deletions, err := testpkg.TenantQuery1(t, ctx, db, repo.List, filters)
		require.NoError(t, err)

		for _, d := range deletions {
			assert.Equal(t, audit.DeletionTypeManual, d.DeletionType)
		}
	})
}

func TestTenantIsolation_DataDeletionVisibility(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repo := NewDataDeletionRepository(NewRuntime(db, auditTestTenantID))
	tenantA, ctxA := testpkg.Tenant(t), testpkg.Ctx(t)
	scopeB := testpkg.NewTenantScope(t, db)
	tenantB, ctxB := scopeB.TenantID, scopeB.Context()

	// Data deletions require students (which require persons).
	sA := testpkg.CreateTestStudentForTenant(t, db, tenantA, "AuditA", "Student", "3a")
	sB := testpkg.CreateTestStudentForTenant(t, db, tenantB, "AuditB", "Student", "3a")

	ddA := testpkg.CreateTestDataDeletionForTenant(t, db, tenantA, sA.ID)
	ddB := testpkg.CreateTestDataDeletionForTenant(t, db, tenantB, sB.ID)

	// --- Tenant A ---
	deletions, err := testpkg.TenantQuery1(t, ctxA, db, repo.List, nil)
	require.NoError(t, err)

	for _, d := range deletions {
		assert.Equal(t, tenantA, d.TenantID,
			"cross-tenant leak: tenant B data deletion visible to tenant A (List)")
	}

	// DataDeletionRepository uses base.Repository — FindByID returns error on not-found
	_, err = testpkg.TenantQuery1(t, ctxA, db, repo.FindByID, any(ddB.ID))
	assert.Error(t, err,
		"cross-tenant FindByID should fail: tenant A must not see tenant B data deletion %d", ddB.ID)

	// --- Tenant B ---
	deletions, err = testpkg.TenantQuery1(t, ctxB, db, repo.List, nil)
	require.NoError(t, err)

	for _, d := range deletions {
		assert.Equal(t, tenantB, d.TenantID,
			"cross-tenant leak: tenant A data deletion visible to tenant B (List)")
	}

	_, err = testpkg.TenantQuery1(t, ctxB, db, repo.FindByID, any(ddA.ID))
	assert.Error(t, err,
		"cross-tenant FindByID should fail: tenant B must not see tenant A data deletion %d", ddA.ID)
}
