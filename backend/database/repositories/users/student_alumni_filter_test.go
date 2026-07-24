package users_test

import (
	"fmt"
	"testing"

	"github.com/gofrs/uuid"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Alumni (graduated students, soft-deactivated by a grade transition) must be
// invisible to the staff-facing read paths: group student lists, group counts,
// and the distinct school-class list. Their rows stay in the database so a
// transition revert can restore them.

func assignGroup(t *testing.T, db *bun.DB, studentID, groupID int64) {
	t.Helper()
	_, err := db.NewUpdate().
		TableExpr(`users.students`).
		Set("group_id = ?", groupID).
		Where("id = ?", studentID).
		Exec(t.Context())
	require.NoError(t, err)
}

func TestStudentRepository_AlumniExcludedFromGroupReads(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repos := repositories.NewFactory(db)
	ctx := testpkg.TenantContext(1)

	suffix := uuid.Must(uuid.NewV4()).String()[:8]
	group := testpkg.CreateTestEducationGroup(t, db, fmt.Sprintf("AlumniGroup-%s", suffix))

	activeStudent := testpkg.CreateTestStudent(t, db, "GroupActive", "Kid", fmt.Sprintf("1ga-%s", suffix))
	alumnusStudent := testpkg.CreateTestStudent(t, db, "GroupAlumnus", "Kid", fmt.Sprintf("1ga-%s", suffix))
	defer testpkg.CleanupActivityFixtures(t, db, activeStudent.ID, alumnusStudent.ID)
	defer func() {
		_, _ = db.NewDelete().
			TableExpr(`education.groups`).
			Where("id = ?", group.ID).
			Exec(t.Context())
	}()

	assignGroup(t, db, activeStudent.ID, group.ID)
	assignGroup(t, db, alumnusStudent.ID, group.ID)
	setLifecycle(t, db, alumnusStudent.ID, users.StudentStatusAlumnus, nil, nil)

	t.Run("FindByGroupID excludes alumni", func(t *testing.T) {
		students, err := repos.Student.FindByGroupID(ctx, group.ID)
		require.NoError(t, err)
		ids := make([]int64, 0, len(students))
		for _, s := range students {
			ids = append(ids, s.ID)
		}
		assert.Contains(t, ids, activeStudent.ID)
		assert.NotContains(t, ids, alumnusStudent.ID)
	})

	t.Run("FindByGroupIDs excludes alumni", func(t *testing.T) {
		students, err := repos.Student.FindByGroupIDs(ctx, []int64{group.ID})
		require.NoError(t, err)
		ids := make([]int64, 0, len(students))
		for _, s := range students {
			ids = append(ids, s.ID)
		}
		assert.Contains(t, ids, activeStudent.ID)
		assert.NotContains(t, ids, alumnusStudent.ID)
	})

	t.Run("CountByGroupIDs excludes alumni", func(t *testing.T) {
		counts, err := repos.Student.CountByGroupIDs(ctx, []int64{group.ID})
		require.NoError(t, err)
		assert.Equal(t, 1, counts[group.ID])
	})
}

// TestStudentRepository_AlumniExcludedFromGroupInfoReads covers the #405 P1 gap:
// the shared newStudentWithGroupQuery (FindAllWithGroups /
// FindByTeacherIDWithGroups) backs the IoT student roster
// (GET /api/iot/students) and the calendar student picker, and must hide
// graduated students exactly like the other group reads.
func TestStudentRepository_AlumniExcludedFromGroupInfoReads(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repos := repositories.NewFactory(db)
	ctx := testpkg.TenantContext(1)

	suffix := uuid.Must(uuid.NewV4()).String()[:8]
	group := testpkg.CreateTestEducationGroup(t, db, fmt.Sprintf("AlumniInfoGroup-%s", suffix))
	teacher := testpkg.CreateTestTeacher(t, db, "Roster", "Teacher")
	gt := testpkg.CreateTestGroupTeacher(t, db, group.ID, teacher.ID)

	activeStudent := testpkg.CreateTestStudent(t, db, "InfoActive", "Kid", fmt.Sprintf("1gi-%s", suffix))
	alumnusStudent := testpkg.CreateTestStudent(t, db, "InfoAlumnus", "Kid", fmt.Sprintf("1gi-%s", suffix))
	defer func() {
		cleanupStudentRecords(t, db, activeStudent.ID, alumnusStudent.ID)
		_, _ = db.NewDelete().
			TableExpr("education.group_teacher").
			Where("id = ?", gt.ID).
			Exec(ctx)
		cleanupEducationData(t, db, []int64{group.ID}, []int64{teacher.ID})
	}()

	assignGroup(t, db, activeStudent.ID, group.ID)
	assignGroup(t, db, alumnusStudent.ID, group.ID)
	setLifecycle(t, db, alumnusStudent.ID, users.StudentStatusAlumnus, nil, nil)

	t.Run("FindAllWithGroups excludes alumni", func(t *testing.T) {
		infos, err := repos.Student.FindAllWithGroups(ctx)
		require.NoError(t, err)
		ids := make([]int64, 0, len(infos))
		for _, info := range infos {
			ids = append(ids, info.ID)
		}
		assert.Contains(t, ids, activeStudent.ID)
		assert.NotContains(t, ids, alumnusStudent.ID)
	})

	t.Run("FindByTeacherIDWithGroups excludes alumni", func(t *testing.T) {
		infos, err := repos.Student.FindByTeacherIDWithGroups(ctx, teacher.ID)
		require.NoError(t, err)
		ids := make([]int64, 0, len(infos))
		for _, info := range infos {
			ids = append(ids, info.ID)
		}
		assert.Contains(t, ids, activeStudent.ID)
		assert.NotContains(t, ids, alumnusStudent.ID)
	})
}

func TestStudentRepository_AlumniExcludedFromSchoolClasses(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repos := repositories.NewFactory(db)
	ctx := testpkg.TenantContext(1)

	suffix := uuid.Must(uuid.NewV4()).String()[:8]
	alumniOnlyClass := fmt.Sprintf("4gone-%s", suffix)
	mixedClass := fmt.Sprintf("2mixed-%s", suffix)

	alumnusOnly := testpkg.CreateTestStudent(t, db, "ClassGone", "Kid", alumniOnlyClass)
	activeMixed := testpkg.CreateTestStudent(t, db, "ClassMixedActive", "Kid", mixedClass)
	alumnusMixed := testpkg.CreateTestStudent(t, db, "ClassMixedAlum", "Kid", mixedClass)
	defer testpkg.CleanupActivityFixtures(t, db, alumnusOnly.ID, activeMixed.ID, alumnusMixed.ID)

	setLifecycle(t, db, alumnusOnly.ID, users.StudentStatusAlumnus, nil, nil)
	setLifecycle(t, db, alumnusMixed.ID, users.StudentStatusAlumnus, nil, nil)

	classes, err := repos.Student.ListSchoolClasses(ctx)
	require.NoError(t, err)
	assert.NotContains(t, classes, alumniOnlyClass, "class with only alumni must disappear")
	assert.Contains(t, classes, mixedClass, "class with remaining active students must stay")
}
