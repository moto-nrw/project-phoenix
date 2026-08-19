package active_test

import (
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	activeRepo "github.com/moto-nrw/project-phoenix/database/repositories/active"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/moto-nrw/project-phoenix/services/active"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func createActiveServiceWithCrossTenant(t *testing.T, db *bun.DB) active.Service {
	t.Helper()
	repoFactory := repositories.NewFactory(db)
	serviceFactory, err := services.NewFactory(repoFactory, db, slog.Default())
	require.NoError(t, err)
	return serviceFactory.Active
}

func TestCrossTenantStudents(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	// Ensure two tenants exist
	tenantA := int64(42)
	tenantB := int64(43)
	testpkg.CleanupTenantTestData(t, db, tenantA, tenantB)
	t.Cleanup(func() { testpkg.CleanupTenantTestData(t, db, tenantA, tenantB) })
	testpkg.EnsureTestTenant(t, db, tenantA)
	testpkg.EnsureTestTenant(t, db, tenantB)

	service := createActiveServiceWithCrossTenant(t, db)

	t.Run("no cross-tenant visits returns empty slice", func(t *testing.T) {
		ctx := testpkg.TenantContext(tenantA)

		students, err := service.GetCrossTenantStudents(ctx, tenantA)
		require.NoError(t, err)
		assert.Empty(t, students)
	})

	t.Run("same-tenant students are not returned", func(t *testing.T) {
		ctx := testpkg.TenantContext(tenantA)

		// Create a student belonging to tenant A
		studentA := testpkg.CreateTestStudentForTenant(t, db, tenantA, "Local", "StudentA", "1a")

		// Create an active group on tenant A and an active visit for the local student
		activeGroupA := testpkg.CreateTestActiveGroupForTenant(t, db, tenantA)
		testpkg.CreateTestVisitForTenant(t, db, tenantA, studentA.ID, activeGroupA.ID, time.Now(), nil)

		students, err := service.GetCrossTenantStudents(ctx, tenantA)
		require.NoError(t, err)
		assert.Empty(t, students, "same-tenant student should not appear in cross-tenant results")
	})

	t.Run("cross-tenant student with active visit is returned", func(t *testing.T) {
		ctx := testpkg.TenantContext(tenantA)

		// Create a student belonging to tenant B
		studentB := testpkg.CreateTestStudentForTenant(t, db, tenantB, "Visitor", "StudentB", "2b")

		// Create an active group on tenant A and create a visit for the foreign student
		// The visit's tenant_id is the hosting tenant (A), but the student's tenant_id is B
		activeGroupA2 := testpkg.CreateTestActiveGroupForTenant(t, db, tenantA)
		testpkg.CreateTestVisitForTenant(t, db, tenantA, studentB.ID, activeGroupA2.ID, time.Now(), nil)

		students, err := service.GetCrossTenantStudents(ctx, tenantA)
		require.NoError(t, err)
		require.Len(t, students, 1, "cross-tenant student should be returned")

		assert.Equal(t, studentB.ID, students[0].StudentID)
		assert.Equal(t, "Visitor", students[0].FirstName)
		assert.Equal(t, "StudentB", students[0].LastName)
		assert.NotEmpty(t, students[0].HomeTenant, "home_tenant slug should be populated")
	})

	t.Run("ended visits are not returned", func(t *testing.T) {
		// Use a different pair of tenants to avoid pollution
		tenantC := int64(44)
		tenantD := int64(45)
		testpkg.CleanupTenantTestData(t, db, tenantC, tenantD)
		t.Cleanup(func() { testpkg.CleanupTenantTestData(t, db, tenantC, tenantD) })
		testpkg.EnsureTestTenant(t, db, tenantC)
		testpkg.EnsureTestTenant(t, db, tenantD)

		ctx := testpkg.TenantContext(tenantC)

		studentD := testpkg.CreateTestStudentForTenant(t, db, tenantD, "Past", "Visitor", "3c")
		activeGroupC := testpkg.CreateTestActiveGroupForTenant(t, db, tenantC)

		exitTime := time.Now()
		testpkg.CreateTestVisitForTenant(t, db, tenantC, studentD.ID, activeGroupC.ID, time.Now().Add(-time.Hour), &exitTime)

		students, err := service.GetCrossTenantStudents(ctx, tenantC)
		require.NoError(t, err)
		assert.Empty(t, students, "ended visits should not appear in results")
	})

	t.Run("only CrossTenantStudent fields are populated", func(t *testing.T) {
		// Reuse existing cross-tenant data from tenant A
		ctx := testpkg.TenantContext(tenantA)

		students, err := service.GetCrossTenantStudents(ctx, tenantA)
		require.NoError(t, err)

		for _, s := range students {
			assert.NotZero(t, s.StudentID, "student_id must be set")
			assert.NotEmpty(t, s.FirstName, "first_name must be set")
			assert.NotEmpty(t, s.LastName, "last_name must be set")
			assert.NotEmpty(t, s.HomeTenant, "home_tenant must be set")
			// GroupName may be empty if student has no group assigned — that's OK
		}
	})
}

// TestCrossTenantRepository_Direct tests the repository directly to ensure
// the SQL query works correctly.
func TestCrossTenantRepository_Direct(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	tenantHost := int64(50)
	tenantVisitor := int64(51)
	testpkg.CleanupTenantTestData(t, db, tenantHost, tenantVisitor)
	t.Cleanup(func() { testpkg.CleanupTenantTestData(t, db, tenantHost, tenantVisitor) })
	testpkg.EnsureTestTenant(t, db, tenantHost)
	testpkg.EnsureTestTenant(t, db, tenantVisitor)

	repo := activeRepo.NewCrossTenantRepository(db)

	t.Run("returns empty for no visits", func(t *testing.T) {
		results, err := repo.FindCrossTenantStudents(testpkg.TenantContext(tenantHost), tenantHost)
		require.NoError(t, err)
		assert.Empty(t, results)
	})

	t.Run("finds cross-tenant visitor", func(t *testing.T) {
		student := testpkg.CreateTestStudentForTenant(t, db, tenantVisitor, "RepoTest", "Visitor", "4d")
		activeGroup := testpkg.CreateTestActiveGroupForTenant(t, db, tenantHost)
		testpkg.CreateTestVisitForTenant(t, db, tenantHost, student.ID, activeGroup.ID, time.Now(), nil)

		results, err := repo.FindCrossTenantStudents(testpkg.TenantContext(tenantHost), tenantHost)
		require.NoError(t, err)
		require.NotEmpty(t, results)

		found := false
		for _, r := range results {
			if r.StudentID == student.ID {
				found = true
				assert.Equal(t, "RepoTest", r.FirstName)
				assert.Equal(t, "Visitor", r.LastName)
			}
		}
		assert.True(t, found, "expected to find cross-tenant student in results")
	})
}
