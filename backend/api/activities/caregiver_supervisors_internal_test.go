package activities

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
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

func TestFetchAllSupervisors_IncludesLegacyTeachers(t *testing.T) {
	db, _, resource := setupActivitiesResource(t)

	activeTeacher, activeAccount := testpkg.CreateTestTeacherWithAccount(t, db, "Active", "Caregiver")
	legacyTeacher, legacyAccount := testpkg.CreateTestTeacherWithAccount(t, db, "Legacy", "Teacher")
	defer testpkg.CleanupTeacherFixtures(t, db, activeTeacher.ID)
	defer testpkg.CleanupTeacherFixtures(t, db, legacyTeacher.ID)
	defer testpkg.CleanupAuthFixtures(t, db, activeAccount.ID)
	defer testpkg.CleanupAuthFixtures(t, db, legacyAccount.ID)

	ctx := tenant.WithTenantID(context.Background(), 1)
	supervisors, err := resource.fetchAllSupervisors(ctx)
	require.NoError(t, err)

	supervisorNames := make(map[int64]string, len(supervisors))
	for _, supervisor := range supervisors {
		supervisorNames[supervisor.StaffID] = supervisor.FirstName
	}

	assert.Equal(t, "Active", supervisorNames[activeTeacher.Staff.ID])
	assert.Equal(t, "Legacy", supervisorNames[legacyTeacher.Staff.ID])
}

func TestFetchSupervisorsBySpecialization_IncludesLegacyTeachers(t *testing.T) {
	db, _, resource := setupActivitiesResource(t)

	activeTeacher, activeAccount := testpkg.CreateTestTeacherWithAccount(t, db, "Filtered", "Caregiver")
	_, err := db.NewUpdate().
		Model(activeTeacher).
		ModelTableExpr(`users.teachers AS "teacher"`).
		Set("specialization = ?", "Sport").
		Where("id = ?", activeTeacher.ID).
		Exec(context.Background())
	require.NoError(t, err)

	legacyTeacher, legacyAccount := testpkg.CreateTestTeacherWithAccount(t, db, "Legacy", "Included")
	_, err = db.NewUpdate().
		Model(legacyTeacher).
		ModelTableExpr(`users.teachers AS "teacher"`).
		Set("specialization = ?", "Sport").
		Where("id = ?", legacyTeacher.ID).
		Exec(context.Background())
	require.NoError(t, err)

	defer testpkg.CleanupTeacherFixtures(t, db, activeTeacher.ID)
	defer testpkg.CleanupTeacherFixtures(t, db, legacyTeacher.ID)
	defer testpkg.CleanupAuthFixtures(t, db, activeAccount.ID)
	defer testpkg.CleanupAuthFixtures(t, db, legacyAccount.ID)

	ctx := tenant.WithTenantID(context.Background(), 1)
	supervisors, err := resource.fetchSupervisorsBySpecialization(ctx, "Sport")
	require.NoError(t, err)

	supervisorNames := make(map[int64]string, len(supervisors))
	for _, supervisor := range supervisors {
		supervisorNames[supervisor.StaffID] = supervisor.FirstName
	}

	assert.Equal(t, "Filtered", supervisorNames[activeTeacher.StaffID])
	assert.Equal(t, "Legacy", supervisorNames[legacyTeacher.StaffID])
}
