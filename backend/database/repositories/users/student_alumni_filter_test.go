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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repos := repositories.NewFactory(db)
	ctx := testpkg.Ctx(t)

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repos := repositories.NewFactory(db)
	ctx := testpkg.Ctx(t)

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repos := repositories.NewFactory(db)
	ctx := testpkg.Ctx(t)

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

// Import duplicate detection must not see alumni either. Before graduation
// became a soft delete the row was gone, so a graduate could not collide with a
// new child; with the row retained, an unfiltered lookup rejects that import as
// already_exists (#405 review).
func TestStudentRepository_FindByNameAndClassExcludesAlumni(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repos := repositories.NewFactory(db)
	ctx := testpkg.Ctx(t)

	suffix := uuid.Must(uuid.NewV4()).String()[:8]
	class := fmt.Sprintf("1na-%s", suffix)
	firstName := fmt.Sprintf("Namensvetter%s", suffix)

	graduate := testpkg.CreateTestStudent(t, db, firstName, "Kid", class)
	defer testpkg.CleanupActivityFixtures(t, db, graduate.ID)
	setLifecycle(t, db, graduate.ID, users.StudentStatusAlumnus, nil, nil)

	t.Run("graduate does not block a same-name import", func(t *testing.T) {
		students, err := repos.Student.FindByNameAndClass(ctx, firstName, "Kid", class)
		require.NoError(t, err)
		assert.Empty(t, students, "alumnus must not surface as an import duplicate")
	})

	t.Run("an active namesake is still detected", func(t *testing.T) {
		active := testpkg.CreateTestStudent(t, db, firstName, "Kid", class)
		defer testpkg.CleanupActivityFixtures(t, db, active.ID)

		students, err := repos.Student.FindByNameAndClass(ctx, firstName, "Kid", class)
		require.NoError(t, err)
		require.Len(t, students, 1)
		assert.Equal(t, active.ID, students[0].ID)
	})
}
