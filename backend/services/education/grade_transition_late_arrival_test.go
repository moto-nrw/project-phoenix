// Late-arrival guard for the apply cohort (#405 review).
//
// The row locks the apply takes bound what a concurrent writer can CHANGE, not
// what it can ADD. A child who lands in a mapped class after the cohort snapshot
// is never locked and never written, so without a guard the transition would be
// marked applied while that child stays behind in a class it just emptied — a
// half-done bulk change reported as a finished one, with no history row for the
// revert to undo.
package education_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	educationRepo "github.com/moto-nrw/project-phoenix/database/repositories/education"
	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/users"
	educationService "github.com/moto-nrw/project-phoenix/services/education"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// lateArrivalRepo is the real repository with one seam: after the apply took its
// cohort snapshot, it runs afterSnapshot — the deterministic stand-in for the
// concurrent commit that moves a child into a mapped class while the apply is
// locking rows.
type lateArrivalRepo struct {
	educationModels.GradeTransitionRepository
	calls         int
	afterSnapshot func()
}

func (r *lateArrivalRepo) GetStudentsByClasses(
	ctx context.Context, classes []string,
) ([]*educationModels.StudentClassInfo, error) {
	rows, err := r.GradeTransitionRepository.GetStudentsByClasses(ctx, classes)
	r.calls++
	if r.calls == 1 && r.afterSnapshot != nil {
		r.afterSnapshot()
	}
	return rows, err
}

func TestGradeTransitionService_Apply_RefusesChildAddedAfterCohortSnapshot(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 20*time.Second)
	defer cancel()

	account := testpkg.CreateTestAccount(t, db, "transition-late-arrival@test.local")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	suffix := uuid.Must(uuid.NewV4()).String()[:8]
	gradClass := fmt.Sprintf("4late-%s", suffix)

	present := testpkg.CreateTestStudent(t, db, "Present", "AtSnapshot", gradClass)
	defer testpkg.CleanupActivityFixtures(t, db, present.ID)

	var lateID int64
	repo := &lateArrivalRepo{
		GradeTransitionRepository: educationRepo.NewGradeTransitionRepository(db),
		afterSnapshot: func() {
			late := testpkg.CreateTestStudent(t, db, "Late", "Arrival", gradClass)
			lateID = late.ID
		},
	}
	defer func() {
		if lateID != 0 {
			testpkg.CleanupActivityFixtures(t, db, lateID)
		}
	}()

	service := educationService.NewGradeTransitionService(educationService.GradeTransitionServiceDependencies{
		TransitionRepo: repo,
		StudentRepo:    usersRepo.NewStudentRepository(db),
		PersonRepo:     usersRepo.NewPersonRepository(db),
		DB:             db,
	})

	transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
	testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, gradClass, nil) // graduate
	defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

	_, err := service.Apply(ctx, transition.ID, account.ID)
	require.ErrorIs(t, err, educationService.ErrPreviewStale,
		"a child added to a mapped class after the snapshot must abort the apply, not be skipped")

	// Nothing was written: neither child graduated and the transition is still a
	// draft the admin can re-preview and apply completely.
	assertStudentStatus(t, db, present.ID, users.StudentStatusActive)
	assertStudentStatus(t, db, lateID, users.StudentStatusActive)

	fresh, err := educationRepo.NewGradeTransitionRepository(db).FindByID(ctx, transition.ID)
	require.NoError(t, err)
	assert.Equal(t, educationModels.TransitionStatusDraft, fresh.Status)

	// The second apply sees the complete cohort (both children are in the
	// snapshot now) and succeeds — the guard delays the transition, it does not
	// block it.
	repo.afterSnapshot = nil
	result, err := service.Apply(ctx, transition.ID, account.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, result.StudentsGraduated)
	assertStudentStatus(t, db, present.ID, users.StudentStatusAlumnus)
	assertStudentStatus(t, db, lateID, users.StudentStatusAlumnus)
}

func assertStudentStatus(t *testing.T, db *bun.DB, studentID int64, want users.StudentStatus) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var status string
	require.NoError(t, db.NewSelect().
		TableExpr(`users.students`).
		Column("status").
		Where("id = ?", studentID).
		Scan(ctx, &status))
	assert.Equal(t, string(want), status)
}
