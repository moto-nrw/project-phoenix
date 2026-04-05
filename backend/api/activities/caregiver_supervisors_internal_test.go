package activities

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func setupActivitiesResource(t *testing.T) (*bun.DB, *services.Factory, *Resource) {
	t.Helper()

	db, svc := testutil.SetupAPITest(t)
	resource := NewResource(svc.Activities, svc.Schedule, svc.Users, svc.UserContext, db)
	return db, svc, resource
}

func assignUserRoleToAccount(t *testing.T, db *bun.DB, accountID int64) {
	t.Helper()

	role := testpkg.GetOrCreateTestRole(t, db, "user")
	accountRole := &authModels.AccountRole{
		AccountID: accountID,
		RoleID:    role.ID,
	}
	accountRole.SetTenantID(1)

	err := db.NewInsert().
		Model(accountRole).
		ModelTableExpr(`auth.account_roles`).
		Scan(context.Background())
	require.NoError(t, err)
}

func TestFetchAllSupervisors_UsesCaregiverPool(t *testing.T) {
	db, _, resource := setupActivitiesResource(t)
	defer func() { _ = db.Close() }()

	activeTeacher, activeAccount := testpkg.CreateTestTeacherWithAccount(t, db, "Active", "Caregiver")
	passiveTeacher, passiveAccount := testpkg.CreateTestTeacherWithAccount(t, db, "Passive", "Teacher")
	defer testpkg.CleanupTeacherFixtures(t, db, activeTeacher.ID)
	defer testpkg.CleanupTeacherFixtures(t, db, passiveTeacher.ID)
	defer testpkg.CleanupAuthFixtures(t, db, activeAccount.ID)
	defer testpkg.CleanupAuthFixtures(t, db, passiveAccount.ID)

	assignUserRoleToAccount(t, db, activeAccount.ID)

	ctx := tenant.WithTenantID(context.Background(), 1)
	supervisors, err := resource.fetchAllSupervisors(ctx)
	require.NoError(t, err)

	require.Len(t, supervisors, 1)
	assert.Equal(t, activeTeacher.Staff.ID, supervisors[0].StaffID)
	assert.Equal(t, activeTeacher.ID, supervisors[0].ID)
	assert.Equal(t, "Active", supervisors[0].FirstName)
}

func TestFetchSupervisorsBySpecialization_FiltersToActiveCaregivers(t *testing.T) {
	db, _, resource := setupActivitiesResource(t)
	defer func() { _ = db.Close() }()

	activeTeacher, activeAccount := testpkg.CreateTestTeacherWithAccount(t, db, "Filtered", "Caregiver")
	activeTeacher.Specialization = "Sport"
	_, err := db.NewUpdate().
		Model(activeTeacher).
		ModelTableExpr(`users.teachers`).
		Set("specialization = ?", "Sport").
		Where("id = ?", activeTeacher.ID).
		Exec(context.Background())
	require.NoError(t, err)

	inactiveTeacher, inactiveAccount := testpkg.CreateTestTeacherWithAccount(t, db, "Filtered", "Ignored")
	inactiveTeacher.Specialization = "Sport"
	_, err = db.NewUpdate().
		Model(inactiveTeacher).
		ModelTableExpr(`users.teachers`).
		Set("specialization = ?", "Sport").
		Where("id = ?", inactiveTeacher.ID).
		Exec(context.Background())
	require.NoError(t, err)

	defer testpkg.CleanupTeacherFixtures(t, db, activeTeacher.ID)
	defer testpkg.CleanupTeacherFixtures(t, db, inactiveTeacher.ID)
	defer testpkg.CleanupAuthFixtures(t, db, activeAccount.ID)
	defer testpkg.CleanupAuthFixtures(t, db, inactiveAccount.ID)

	assignUserRoleToAccount(t, db, activeAccount.ID)

	ctx := tenant.WithTenantID(context.Background(), 1)
	supervisors, err := resource.fetchSupervisorsBySpecialization(ctx, "Sport")
	require.NoError(t, err)

	require.Len(t, supervisors, 1)
	assert.Equal(t, activeTeacher.ID, supervisors[0].ID)
	assert.Equal(t, activeTeacher.StaffID, supervisors[0].StaffID)
	assert.Equal(t, "Filtered", supervisors[0].FirstName)
}

func TestFetchAllSupervisors_ReturnsDirectoryError(t *testing.T) {
	resource := &Resource{
		UserService: nil,
	}

	_, err := resource.fetchAllSupervisors(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "person service does not implement caregiver directory")
}

func TestFetchSupervisorsBySpecialization_ReturnsDirectoryError(t *testing.T) {
	resource := &Resource{
		UserService: nil,
	}

	_, err := resource.fetchSupervisorsBySpecialization(context.Background(), "Sport")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "person service does not implement caregiver directory")
}
