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
	service, db, cleanup := setupGradeTransitionServiceTest(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(testpkg.TenantContext(1), 15*time.Second)
	defer cancel()

	account := testpkg.CreateTestAccount(t, db, "transition-classteacher@test.local")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	suffix := uuid.Must(uuid.NewV4()).String()[:8]
	class1 := fmt.Sprintf("1a-%s", suffix)
	class2 := fmt.Sprintf("2a-%s", suffix)
	class3 := fmt.Sprintf("3a-%s", suffix)

	// One child per class so the transition has a locked cohort to move.
	student1 := testpkg.CreateTestStudent(t, db, "Remap", "Child1", class1)
	student2 := testpkg.CreateTestStudent(t, db, "Remap", "Child2", class2)
	student3 := testpkg.CreateTestStudent(t, db, "Remap", "Child3", class3)
	defer testpkg.CleanupActivityFixtures(t, db, student1.ID, student2.ID, student3.ID)

	// The teacher holds the chain's start AND middle class — the rename
	// 1a→2a while 2a→3a must not trip the unique index. A second teacher
	// holds the graduating class.
	chainTeacher := testpkg.CreateTestStaff(t, db, "Remap", "ChainTeacher")
	graduateTeacher := testpkg.CreateTestStaff(t, db, "Remap", "GraduateTeacher")
	defer testpkg.CleanupStaffFixtures(t, db, chainTeacher.ID, graduateTeacher.ID)
	defer func() {
		tenantCtx := testpkg.TenantContext(1)
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
	defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

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

// The revert replays the recorded ledger, not a reverse rename — a
// pre-existing assignment of a rename target that the apply never touched
// must survive the revert untouched, and a merge (teacher held both the from-
// and the to-class) must round-trip without losing the surviving assignment.
func TestGradeTransitionService_Revert_TouchesOnlyLedgeredClassTeachers(t *testing.T) {
	service, db, cleanup := setupGradeTransitionServiceTest(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(testpkg.TenantContext(1), 15*time.Second)
	defer cancel()

	account := testpkg.CreateTestAccount(t, db, "transition-ledger@test.local")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	suffix := uuid.Must(uuid.NewV4()).String()[:8]
	class1 := fmt.Sprintf("1a-%s", suffix)
	class2 := fmt.Sprintf("2a-%s", suffix)

	// Only 1a → 2a is mapped; 2a itself is NOT a from-class.
	student := testpkg.CreateTestStudent(t, db, "Ledger", "Child", class1)
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)

	// mergeTeacher holds BOTH classes (apply drops 1a, keeps 2a);
	// bystander holds only the pre-existing 2a the apply never touches.
	mergeTeacher := testpkg.CreateTestStaff(t, db, "Ledger", "MergeTeacher")
	bystander := testpkg.CreateTestStaff(t, db, "Ledger", "Bystander")
	defer testpkg.CleanupStaffFixtures(t, db, mergeTeacher.ID, bystander.ID)
	defer func() {
		tenantCtx := testpkg.TenantContext(1)
		_, _ = db.NewDelete().TableExpr("education.class_teachers").
			Where("staff_id IN (?)", bun.List([]int64{mergeTeacher.ID, bystander.ID})).
			Exec(tenantCtx)
	}()

	testpkg.CreateTestClassTeacher(t, db, mergeTeacher.ID, class1)
	testpkg.CreateTestClassTeacher(t, db, mergeTeacher.ID, class2)
	testpkg.CreateTestClassTeacher(t, db, bystander.ID, class2)

	transition := testpkg.CreateTestGradeTransition(t, db, "2026-2027", account.ID)
	testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, class1, testpkg.StrPtr(class2))
	defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

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
