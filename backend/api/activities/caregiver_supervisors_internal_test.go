package activities

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func setupActivitiesInternalRoute(t *testing.T) (*bun.DB, *Resource) {
	t.Helper()

	db, svc := testutil.SetupActivitiesModule(t)
	resource := NewResource(svc.Activities, svc.Schedule, svc.Users, svc.UserContext, db)
	return db, resource
}

func TestFetchAllSupervisors_IncludesLegacyTeachers(t *testing.T) {
	t.Parallel()
	db, resource := setupActivitiesInternalRoute(t)

	activeTeacher, _ := testpkg.CreateTestTeacherWithAccount(t, db, "Active", "Caregiver")
	legacyTeacher, _ := testpkg.CreateTestTeacherWithAccount(t, db, "Legacy", "Teacher")

	ctx := testpkg.Ctx(t)
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
	t.Parallel()
	db, resource := setupActivitiesInternalRoute(t)

	activeTeacher, _ := testpkg.CreateTestTeacherWithAccount(t, db, "Filtered", "Caregiver")
	_, err := db.NewUpdate().
		Model(activeTeacher).
		ModelTableExpr(`users.teachers AS "teacher"`).
		Set("specialization = ?", "Sport").
		Where("id = ?", activeTeacher.ID).
		Exec(context.Background())
	require.NoError(t, err)

	legacyTeacher, _ := testpkg.CreateTestTeacherWithAccount(t, db, "Legacy", "Included")
	_, err = db.NewUpdate().
		Model(legacyTeacher).
		ModelTableExpr(`users.teachers AS "teacher"`).
		Set("specialization = ?", "Sport").
		Where("id = ?", legacyTeacher.ID).
		Exec(context.Background())
	require.NoError(t, err)

	ctx := testpkg.Ctx(t)
	supervisors, err := resource.fetchSupervisorsBySpecialization(ctx, "Sport")
	require.NoError(t, err)

	supervisorNames := make(map[int64]string, len(supervisors))
	for _, supervisor := range supervisors {
		supervisorNames[supervisor.StaffID] = supervisor.FirstName
	}

	assert.Equal(t, "Filtered", supervisorNames[activeTeacher.StaffID])
	assert.Equal(t, "Legacy", supervisorNames[legacyTeacher.StaffID])
}
