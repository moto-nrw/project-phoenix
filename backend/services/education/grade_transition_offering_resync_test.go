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

func TestGradeTransitionService_ApplyAndRevert_ResyncOfferingSourcedRosters(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

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

	ctx, cancel := context.WithTimeout(testpkg.TenantContext(1), 15*time.Second)
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
