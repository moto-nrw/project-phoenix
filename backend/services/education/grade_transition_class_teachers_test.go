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
	assert.Empty(t, staffClassNames(t, db, ctx, graduateTeacher.ID),
		"graduated assignments have no history and stay gone after revert")
}
