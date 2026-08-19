package education_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	activeRepo "github.com/moto-nrw/project-phoenix/database/repositories/active"
	educationRepo "github.com/moto-nrw/project-phoenix/database/repositories/education"
	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	educationService "github.com/moto-nrw/project-phoenix/services/education"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingOfferingResyncer captures every offering-source resync the grade
// transition triggers (#2137 review): promotions rewrite school classes, so
// apply AND revert must re-reconcile the Jahrgang-filtered sourced rosters.
type recordingOfferingResyncer struct {
	calls []timezone.Date
}

func (r *recordingOfferingResyncer) ResyncOfferingSourcedTemplates(_ context.Context, effectiveFrom timezone.Date) error {
	r.calls = append(r.calls, effectiveFrom)
	return nil
}

// orderRecordingReconciler and orderRecordingResyncer append to one shared log
// so the test below can pin the archive/resync ordering (#2147 review round
// 16): the offering-source resync deletes sourced still-planned rows of the
// graduates WITHOUT archiving, so the archive pass must run first on apply —
// and on revert the archive replay must run first, so the resync finds the
// replayed rows (room, note, non-booking markers intact) and retains them
// instead of recreating plain expected rows.
type orderRecordingReconciler struct {
	log *[]string
}

func (r *orderRecordingReconciler) RemoveStudentsFromFutureRosters(_ context.Context, _ int64, _ []int64) error {
	*r.log = append(*r.log, "archive")
	return nil
}

func (r *orderRecordingReconciler) RestoreStudentsToFutureRosters(_ context.Context, _ int64, _ []int64, _ *int64) error {
	*r.log = append(*r.log, "restore")
	return nil
}

func (r *orderRecordingReconciler) CurrentRosterBaseline(_ context.Context) (int64, error) {
	*r.log = append(*r.log, "baseline")
	return 0, nil
}

type orderRecordingResyncer struct {
	log *[]string
}

func (r *orderRecordingResyncer) ResyncOfferingSourcedTemplates(_ context.Context, _ timezone.Date) error {
	*r.log = append(*r.log, "resync")
	return nil
}

func TestGradeTransitionService_ApplyAndRevert_ArchiveBracketsOfferingResync(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	log := make([]string, 0, 5)
	service := educationService.NewGradeTransitionService(educationService.GradeTransitionServiceDependencies{
		TransitionRepo:   educationRepo.NewGradeTransitionRepository(db),
		StudentRepo:      usersRepo.NewStudentRepository(db),
		PersonRepo:       usersRepo.NewPersonRepository(db),
		VisitRepo:        activeRepo.NewVisitRepository(db),
		AttendanceRepo:   activeRepo.NewAttendanceRepository(db),
		RosterReconciler: &orderRecordingReconciler{log: &log},
		DB:               db,
	})
	service.SetOfferingSourceResyncer(&orderRecordingResyncer{log: &log})

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 15*time.Second)
	defer cancel()

	account := testpkg.CreateTestAccount(t, db, "transition-resync-order@test.local")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	suffix := uuid.Must(uuid.NewV4()).String()[:8]
	gradClass := fmt.Sprintf("4order-%s", suffix)

	student := testpkg.CreateTestStudent(t, db, "Order", "Child", gradClass)
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)

	transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
	testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, gradClass, nil)
	defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

	_, err := service.Apply(ctx, transition.ID, account.ID)
	require.NoError(t, err)
	assert.Equal(t, []string{"archive", "resync", "baseline"}, log,
		"apply must archive the graduates' planned rows BEFORE the offering-source resync deletes them unarchived")

	log = log[:0]
	_, err = service.Revert(ctx, transition.ID, account.ID)
	require.NoError(t, err)
	assert.Equal(t, []string{"restore", "resync"}, log,
		"revert must replay the archived rows BEFORE the resync, so it retains them instead of recreating plain expected rows")
}

func TestGradeTransitionService_ApplyAndRevert_ResyncOfferingSourcedRosters(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := educationService.NewGradeTransitionService(educationService.GradeTransitionServiceDependencies{
		TransitionRepo: educationRepo.NewGradeTransitionRepository(db),
		StudentRepo:    usersRepo.NewStudentRepository(db),
		PersonRepo:     usersRepo.NewPersonRepository(db),
		VisitRepo:      activeRepo.NewVisitRepository(db),
		AttendanceRepo: activeRepo.NewAttendanceRepository(db),
		DB:             db,
	})
	resyncer := &recordingOfferingResyncer{}
	service.SetOfferingSourceResyncer(resyncer)

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 15*time.Second)
	defer cancel()

	account := testpkg.CreateTestAccount(t, db, "transition-offering-resync@test.local")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	suffix := uuid.Must(uuid.NewV4()).String()[:8]
	fromClass := fmt.Sprintf("2resync-%s", suffix)
	toClass := fmt.Sprintf("3resync-%s", suffix)

	student := testpkg.CreateTestStudent(t, db, "Resync", "Child", fromClass)
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)

	transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
	testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, fromClass, &toClass)
	defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

	_, err := service.Apply(ctx, transition.ID, account.ID)
	require.NoError(t, err)
	require.Len(t, resyncer.calls, 1,
		"apply must resync offering-sourced rosters after rewriting school classes")
	assert.Equal(t, timezone.TodayDate(), resyncer.calls[0])

	_, err = service.Revert(ctx, transition.ID, account.ID)
	require.NoError(t, err)
	require.Len(t, resyncer.calls, 2,
		"revert must resync in the opposite direction")
	assert.Equal(t, timezone.TodayDate(), resyncer.calls[1])
}
