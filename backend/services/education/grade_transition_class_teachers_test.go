package education_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// staffClassNames reads the current class assignments of one staff member
// straight from the table, in normalized order.
func staffClassNames(t *testing.T, db *bun.DB, ctx context.Context, staffID int64) []string {
	t.Helper()
	var classes []string
	err := db.NewSelect().
		TableExpr("education.class_teachers").
		Column("school_class").
		Where("staff_id = ?", staffID).
		OrderExpr("LOWER(BTRIM(school_class)) ASC").
		Scan(ctx, &classes)
	require.NoError(t, err)
	return classes
}

// The Klassenlehrer assignments must follow the school-year rollover (#1772):
// promoted classes carry their teachers along — including chain renames that
// would collide with the unique index if applied one by one — and graduating
// classes lose theirs. Revert carries the promotions back.
func TestGradeTransitionService_Apply_RemapsClassTeachers(t *testing.T) {
	t.Parallel()

	service, db, cleanup := setupGradeTransitionServiceTest(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 15*time.Second)
	defer cancel()

	account := testpkg.CreateTestAccount(t, db, "transition-classteacher@test.local")

	suffix := uuid.Must(uuid.NewV4()).String()[:8]
	class1 := fmt.Sprintf("1a-%s", suffix)
	class2 := fmt.Sprintf("2a-%s", suffix)
	class3 := fmt.Sprintf("3a-%s", suffix)

	// One child per class so the transition has a locked cohort to move.
	testpkg.CreateTestStudent(t, db, "Remap", "Child1", class1)
	testpkg.CreateTestStudent(t, db, "Remap", "Child2", class2)
	testpkg.CreateTestStudent(t, db, "Remap", "Child3", class3)

	// The teacher holds the chain's start AND middle class — the rename
	// 1a→2a while 2a→3a must not trip the unique index. A second teacher
	// holds the graduating class.
	chainTeacher := testpkg.CreateTestStaff(t, db, "Remap", "ChainTeacher")
	graduateTeacher := testpkg.CreateTestStaff(t, db, "Remap", "GraduateTeacher")
	defer func() {
		tenantCtx := testpkg.Ctx(t)
		_, _ = db.NewDelete().TableExpr("education.class_teachers").
			Where("staff_id IN (?)", bun.List([]int64{chainTeacher.ID, graduateTeacher.ID})).
			Exec(tenantCtx)
	}()

	testpkg.CreateTestClassTeacher(t, db, chainTeacher.ID, class1)
	testpkg.CreateTestClassTeacher(t, db, chainTeacher.ID, class2)
	testpkg.CreateTestClassTeacher(t, db, graduateTeacher.ID, class3)

	transition := testpkg.CreateTestGradeTransition(t, db, "2026-2027", account.ID)
	testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, class1, testpkg.StrPtr(class2))
	testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, class2, testpkg.StrPtr(class3))
	testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, class3, nil) // graduates

	_, err := service.Apply(ctx, transition.ID, account.ID)
	require.NoError(t, err)

	assert.Equal(t, []string{class2, class3}, staffClassNames(t, db, ctx, chainTeacher.ID),
		"chain rename must carry both assignments forward")
	assert.Empty(t, staffClassNames(t, db, ctx, graduateTeacher.ID),
		"graduating class must lose its teacher assignment")

	_, err = service.Revert(ctx, transition.ID, account.ID)
	require.NoError(t, err)

	assert.Equal(t, []string{class1, class2}, staffClassNames(t, db, ctx, chainTeacher.ID),
		"revert must carry the promoted assignments back")
	assert.Equal(t, []string{class3}, staffClassNames(t, db, ctx, graduateTeacher.ID),
		"revert must restore the graduated class assignment from the ledger, like it reactivates the students")
}

// Teacher rows move under exactly the same condition as the students: an
// EXACT (trimmed) from-class match. A teacher assigned "1A" must stay put
// when the mapping renames "1a" — the "1A" children do not move either, and
// remapping the teacher would take away sight of their own class.
func TestGradeTransitionService_Apply_MatchesClassTeachersExactlyLikeStudents(t *testing.T) {
	t.Parallel()

	service, db, cleanup := setupGradeTransitionServiceTest(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 15*time.Second)
	defer cancel()

	account := testpkg.CreateTestAccount(t, db, "transition-exactmatch@test.local")

	suffix := uuid.Must(uuid.NewV4()).String()[:8]
	classLower := fmt.Sprintf("1a-%s", suffix)
	classUpper := fmt.Sprintf("1A-%s", suffix)
	classTarget := fmt.Sprintf("2a-%s", suffix)

	// The moving cohort sits in the lower-case class; the teacher holds the
	// case-variant the mapping does NOT name.
	testpkg.CreateTestStudent(t, db, "Exact", "Child", classLower)

	teacher := testpkg.CreateTestStaff(t, db, "Exact", "CaseTeacher")
	defer func() {
		tenantCtx := testpkg.Ctx(t)
		_, _ = db.NewDelete().TableExpr("education.class_teachers").
			Where("staff_id = ?", teacher.ID).Exec(tenantCtx)
	}()
	testpkg.CreateTestClassTeacher(t, db, teacher.ID, classUpper)

	transition := testpkg.CreateTestGradeTransition(t, db, "2026-2027", account.ID)
	testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, classLower, testpkg.StrPtr(classTarget))

	_, err := service.Apply(ctx, transition.ID, account.ID)
	require.NoError(t, err)

	assert.Equal(t, []string{classUpper}, staffClassNames(t, db, ctx, teacher.ID),
		"a case-variant assignment must stay put, exactly like its students do")
}

// A staff member offboarded between apply and revert must not get assignments
// resurrected: offboarding cleared their class rows, and the soft-deleted
// staff row would satisfy the FK even though the normal write path 404s on it.
func TestGradeTransitionService_Revert_SkipsOffboardedStaff(t *testing.T) {
	t.Parallel()

	service, db, cleanup := setupGradeTransitionServiceTest(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 15*time.Second)
	defer cancel()

	account := testpkg.CreateTestAccount(t, db, "transition-offboard@test.local")

	suffix := uuid.Must(uuid.NewV4()).String()[:8]
	class1 := fmt.Sprintf("1a-%s", suffix)
	class2 := fmt.Sprintf("2a-%s", suffix)

	testpkg.CreateTestStudent(t, db, "Offboard", "Child", class1)

	teacher := testpkg.CreateTestStaff(t, db, "Offboard", "Teacher")
	defer func() {
		tenantCtx := testpkg.Ctx(t)
		_, _ = db.NewDelete().TableExpr("education.class_teachers").
			Where("staff_id = ?", teacher.ID).Exec(tenantCtx)
	}()
	testpkg.CreateTestClassTeacher(t, db, teacher.ID, class1)

	transition := testpkg.CreateTestGradeTransition(t, db, "2026-2027", account.ID)
	testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, class1, testpkg.StrPtr(class2))

	_, err := service.Apply(ctx, transition.ID, account.ID)
	require.NoError(t, err)
	require.Equal(t, []string{class2}, staffClassNames(t, db, ctx, teacher.ID))

	// Offboarding: assignments cleared, staff row soft-deleted.
	_, err = db.NewDelete().TableExpr("education.class_teachers").
		Where("staff_id = ?", teacher.ID).Exec(ctx)
	require.NoError(t, err)
	_, err = db.NewUpdate().TableExpr("users.staff").
		Set("deleted_at = NOW()").
		Where("id = ?", teacher.ID).Exec(ctx)
	require.NoError(t, err)

	_, err = service.Revert(ctx, transition.ID, account.ID)
	require.NoError(t, err)

	assert.Empty(t, staffClassNames(t, db, ctx, teacher.ID),
		"revert must not resurrect assignments for an offboarded staff member")
}

// An assignment the admin deliberately removed AFTER the apply must stay
// removed on revert: the renamed row's disappearance means the admin took the
// class away, and restoring the pre-apply name would silently re-grant
// student-data scope.
func TestGradeTransitionService_Revert_RespectsAdminRemovalAfterApply(t *testing.T) {
	t.Parallel()

	service, db, cleanup := setupGradeTransitionServiceTest(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 15*time.Second)
	defer cancel()

	account := testpkg.CreateTestAccount(t, db, "transition-adminremoval@test.local")

	suffix := uuid.Must(uuid.NewV4()).String()[:8]
	class1 := fmt.Sprintf("1a-%s", suffix)
	class2 := fmt.Sprintf("2a-%s", suffix)
	class3 := fmt.Sprintf("3b-%s", suffix)

	testpkg.CreateTestStudent(t, db, "AdminRemoval", "Child", class1)

	teacher := testpkg.CreateTestStaff(t, db, "AdminRemoval", "Teacher")
	defer func() {
		tenantCtx := testpkg.Ctx(t)
		_, _ = db.NewDelete().TableExpr("education.class_teachers").
			Where("staff_id = ?", teacher.ID).Exec(tenantCtx)
	}()
	testpkg.CreateTestClassTeacher(t, db, teacher.ID, class1)

	transition := testpkg.CreateTestGradeTransition(t, db, "2026-2027", account.ID)
	testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, class1, testpkg.StrPtr(class2))

	_, err := service.Apply(ctx, transition.ID, account.ID)
	require.NoError(t, err)
	require.Equal(t, []string{class2}, staffClassNames(t, db, ctx, teacher.ID))

	// The admin re-scopes the teacher after the apply: 2a removed, 3b set.
	_, err = db.NewDelete().TableExpr("education.class_teachers").
		Where("staff_id = ?", teacher.ID).Exec(ctx)
	require.NoError(t, err)
	testpkg.CreateTestClassTeacher(t, db, teacher.ID, class3)

	_, err = service.Revert(ctx, transition.ID, account.ID)
	require.NoError(t, err)

	assert.Equal(t, []string{class3}, staffClassNames(t, db, ctx, teacher.ID),
		"revert must not restore a class the admin deliberately took away after the apply")
}

// setupChainTeacherTransition builds the dual-role rename chain: one teacher
// holds 1a AND 2a, the mappings rename 1a→2a and 2a→3a, so after the apply
// the middle class name (2a) is both a created target (the renamed 1a) and a
// restore source (the pre-apply 2a). Returns the teacher, the three class
// names, and a fixture cleanup the caller MUST defer AFTER the setup cleanup
// (defers run LIFO, and the fixture deletes need the still-open DB).
func setupChainTeacherTransition(t *testing.T, db *bun.DB) (teacherID, transitionID, accountID int64, class1, class2, class3 string, cleanup func()) {
	t.Helper()

	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("transition-chain-%s@test.local", uuid.Must(uuid.NewV4()).String()[:8]))

	suffix := uuid.Must(uuid.NewV4()).String()[:8]
	class1 = fmt.Sprintf("1a-%s", suffix)
	class2 = fmt.Sprintf("2a-%s", suffix)
	class3 = fmt.Sprintf("3a-%s", suffix)

	testpkg.CreateTestStudent(t, db, "Chain", "Child1", class1)
	testpkg.CreateTestStudent(t, db, "Chain", "Child2", class2)

	teacher := testpkg.CreateTestStaff(t, db, "Chain", "DualRoleTeacher")
	testpkg.CreateTestClassTeacher(t, db, teacher.ID, class1)
	testpkg.CreateTestClassTeacher(t, db, teacher.ID, class2)

	transition := testpkg.CreateTestGradeTransition(t, db, "2026-2027", account.ID)
	testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, class1, testpkg.StrPtr(class2))
	testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, class2, testpkg.StrPtr(class3))

	cleanup = func() {
		tenantCtx := testpkg.Ctx(t)
		_, _ = db.NewDelete().TableExpr("education.class_teachers").
			Where("staff_id = ?", teacher.ID).Exec(tenantCtx)
	}

	return teacher.ID, transition.ID, account.ID, class1, class2, class3, cleanup
}

// deleteClassTeacherRow removes one specific assignment, the way an admin
// edit between apply and revert would.
func deleteClassTeacherRow(t *testing.T, db *bun.DB, ctx context.Context, staffID int64, class string) {
	t.Helper()
	res, err := db.NewDelete().TableExpr("education.class_teachers").
		Where("staff_id = ?", staffID).
		Where("LOWER(BTRIM(school_class)) = LOWER(BTRIM(?))", class).
		Exec(ctx)
	require.NoError(t, err)
	affected, err := res.RowsAffected()
	require.NoError(t, err)
	require.EqualValues(t, 1, affected)
}

// Dual-role chain, admin removes the MIDDLE class after the apply: deleting
// the renamed "2a" row must suppress BOTH ledger restores that involve that
// name — the paired "1a" (its renamed row is gone) AND the removed "2a"
// itself (re-creating the name the admin just deleted would re-grant scope
// over any student still carrying it, e.g. one enrolled after the cohort
// lock). The teacher deliberately ends with nothing; erring on the side of
// less scope beats silently re-granting a removed class, and the admin can
// re-assign.
func TestGradeTransitionService_Revert_ChainAdminRemovedMiddleClass(t *testing.T) {
	t.Parallel()

	service, db, cleanup := setupGradeTransitionServiceTest(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 15*time.Second)
	defer cancel()

	teacherID, transitionID, accountID, _, class2, class3, fixtureCleanup := setupChainTeacherTransition(t, db)
	defer fixtureCleanup()

	_, err := service.Apply(ctx, transitionID, accountID)
	require.NoError(t, err)
	require.Equal(t, []string{class2, class3}, staffClassNames(t, db, ctx, teacherID))

	deleteClassTeacherRow(t, db, ctx, teacherID, class2)

	_, err = service.Revert(ctx, transitionID, accountID)
	require.NoError(t, err)

	assert.Empty(t, staffClassNames(t, db, ctx, teacherID),
		"removing the dual-role middle class must suppress both restores that involve its name")
}

// Dual-role chain, admin removes the END class after the apply: the kept
// "2a" row is the promoted 1a cohort and rolls back to its pre-apply name
// ("1a") like in a clean revert, while the pre-apply "2a" stays gone — its
// renamed row ("3a") is exactly what the admin deleted, and the students the
// ledger replay renames back from 3a to 2a are the ones the admin revoked.
// The class NAMES the admin saw ("kept 2a") shift because every class name
// rolls back with the students; the SCOPE the admin curated is preserved.
func TestGradeTransitionService_Revert_ChainAdminRemovedEndClass(t *testing.T) {
	t.Parallel()

	service, db, cleanup := setupGradeTransitionServiceTest(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 15*time.Second)
	defer cancel()

	teacherID, transitionID, accountID, class1, class2, class3, fixtureCleanup := setupChainTeacherTransition(t, db)
	defer fixtureCleanup()

	_, err := service.Apply(ctx, transitionID, accountID)
	require.NoError(t, err)
	require.Equal(t, []string{class2, class3}, staffClassNames(t, db, ctx, teacherID))

	deleteClassTeacherRow(t, db, ctx, teacherID, class3)

	_, err = service.Revert(ctx, transitionID, accountID)
	require.NoError(t, err)

	assert.Equal(t, []string{class1}, staffClassNames(t, db, ctx, teacherID),
		"the kept cohort returns under its pre-apply name, the admin-removed cohort stays gone")
}

// The revert replays the recorded ledger, not a reverse rename — a
// pre-existing assignment of a rename target that the apply never touched
// must survive the revert untouched, and a merge (teacher held both the from-
// and the to-class) must round-trip without losing the surviving assignment.
func TestGradeTransitionService_Revert_TouchesOnlyLedgeredClassTeachers(t *testing.T) {
	t.Parallel()

	service, db, cleanup := setupGradeTransitionServiceTest(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 15*time.Second)
	defer cancel()

	account := testpkg.CreateTestAccount(t, db, "transition-ledger@test.local")

	suffix := uuid.Must(uuid.NewV4()).String()[:8]
	class1 := fmt.Sprintf("1a-%s", suffix)
	class2 := fmt.Sprintf("2a-%s", suffix)

	// Only 1a → 2a is mapped; 2a itself is NOT a from-class.
	testpkg.CreateTestStudent(t, db, "Ledger", "Child", class1)

	// mergeTeacher holds BOTH classes (apply drops 1a, keeps 2a);
	// bystander holds only the pre-existing 2a the apply never touches.
	mergeTeacher := testpkg.CreateTestStaff(t, db, "Ledger", "MergeTeacher")
	bystander := testpkg.CreateTestStaff(t, db, "Ledger", "Bystander")
	defer func() {
		tenantCtx := testpkg.Ctx(t)
		_, _ = db.NewDelete().TableExpr("education.class_teachers").
			Where("staff_id IN (?)", bun.List([]int64{mergeTeacher.ID, bystander.ID})).
			Exec(tenantCtx)
	}()

	testpkg.CreateTestClassTeacher(t, db, mergeTeacher.ID, class1)
	testpkg.CreateTestClassTeacher(t, db, mergeTeacher.ID, class2)
	testpkg.CreateTestClassTeacher(t, db, bystander.ID, class2)

	transition := testpkg.CreateTestGradeTransition(t, db, "2026-2027", account.ID)
	testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, class1, testpkg.StrPtr(class2))

	_, err := service.Apply(ctx, transition.ID, account.ID)
	require.NoError(t, err)

	assert.Equal(t, []string{class2}, staffClassNames(t, db, ctx, mergeTeacher.ID),
		"merge: the 1a assignment folds into the already-held 2a")
	assert.Equal(t, []string{class2}, staffClassNames(t, db, ctx, bystander.ID),
		"apply must not touch the bystander's pre-existing 2a")

	_, err = service.Revert(ctx, transition.ID, account.ID)
	require.NoError(t, err)

	assert.Equal(t, []string{class1, class2}, staffClassNames(t, db, ctx, mergeTeacher.ID),
		"round trip must restore 1a without losing the surviving 2a")
	assert.Equal(t, []string{class2}, staffClassNames(t, db, ctx, bystander.ID),
		"revert must not rewrite an assignment the apply never touched")
}
