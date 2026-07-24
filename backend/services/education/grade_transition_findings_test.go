package education_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	activeRepo "github.com/moto-nrw/project-phoenix/database/repositories/active"
	educationRepo "github.com/moto-nrw/project-phoenix/database/repositories/education"
	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	"github.com/moto-nrw/project-phoenix/models/users"
	educationService "github.com/moto-nrw/project-phoenix/services/education"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// letterOnlySuffix returns a short digit-free token so a class name never
// accidentally matches the grade-number pattern (which any hex digit would).
func letterOnlySuffix(t *testing.T) string {
	t.Helper()
	raw := uuid.Must(uuid.NewV4()).String()
	letters := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' {
			return r
		}
		return -1
	}, raw)
	require.GreaterOrEqual(t, len(letters), 4)
	return letters[:6]
}

// TestGradeTransitionService_Revert_RestoresOriginalStatus covers the P1 fix:
// graduation includes every non-alumnus row, so a class may hold pending
// (future) enrollments. A revert must return each graduate to the status it
// held before the transition, not blanket-activate everyone.
func TestGradeTransitionService_Revert_RestoresOriginalStatus(t *testing.T) {
	service, db, cleanup := setupGradeTransitionServiceTest(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(testpkg.TenantContext(1), 10*time.Second)
	defer cancel()

	account := testpkg.CreateTestAccount(t, db, "transition-status-restore@test.local")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	suffix := uuid.Must(uuid.NewV4()).String()[:8]
	gradClass := fmt.Sprintf("4pending-%s", suffix)

	student := testpkg.CreateTestStudent(t, db, "Future", "Enrollment", gradClass)
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)

	// This child is a pending future enrollment, not an active one.
	_, err := db.NewUpdate().
		TableExpr(`users.students`).
		Set("status = ?", string(users.StudentStatusPending)).
		Where("id = ?", student.ID).
		Exec(ctx)
	require.NoError(t, err)

	transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
	testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, gradClass, nil) // graduate
	defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

	_, err = service.Apply(ctx, transition.ID, account.ID)
	require.NoError(t, err)

	// Graduated -> alumnus (soft-deleted).
	var status string
	require.NoError(t, db.NewSelect().TableExpr(`users.students`).Column("status").
		Where("id = ?", student.ID).Scan(ctx, &status))
	assert.Equal(t, string(users.StudentStatusAlumnus), status)

	_, err = service.Revert(ctx, transition.ID, account.ID)
	require.NoError(t, err)

	// Restored to PENDING, not active — a future enrollment must not be
	// silently activated by a revert.
	require.NoError(t, db.NewSelect().TableExpr(`users.students`).Column("status").
		Where("id = ?", student.ID).Scan(ctx, &status))
	assert.Equal(t, string(users.StudentStatusPending), status,
		"revert must restore the pre-transition status, not blanket-activate")
}

// TestGradeTransitionService_Apply_RejectsCheckedInGraduate covers the P1 fix:
// a graduating child with an open visit would become an alumnus the kiosk can
// no longer check out. The apply must be refused until they are checked out.
func TestGradeTransitionService_Apply_RejectsCheckedInGraduate(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer db.Close()

	service := educationService.NewGradeTransitionService(educationService.GradeTransitionServiceDependencies{
		TransitionRepo: educationRepo.NewGradeTransitionRepository(db),
		StudentRepo:    usersRepo.NewStudentRepository(db),
		PersonRepo:     usersRepo.NewPersonRepository(db),
		VisitRepo:      activeRepo.NewVisitRepository(db),
		AttendanceRepo: activeRepo.NewAttendanceRepository(db),
		DB:             db,
	})

	ctx, cancel := context.WithTimeout(testpkg.TenantContext(1), 10*time.Second)
	defer cancel()

	account := testpkg.CreateTestAccount(t, db, "transition-checked-in@test.local")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	suffix := uuid.Must(uuid.NewV4()).String()[:8]
	gradClass := fmt.Sprintf("4checkedin-%s", suffix)

	activityGroup := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("AG-%s", suffix))
	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("Room-%s", suffix))
	activeGroup := testpkg.CreateTestActiveGroup(t, db, activityGroup.ID, room.ID)
	student := testpkg.CreateTestStudent(t, db, "Checked", "In", gradClass)
	defer testpkg.CleanupActivityFixtures(t, db, student.ID, activityGroup.ID, room.ID)

	// Open visit (nil exit time) = currently checked into a room.
	testpkg.CreateTestVisit(t, db, student.ID, activeGroup.ID, time.Now().Add(-time.Hour), nil)

	transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
	testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, gradClass, nil) // graduate
	defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

	_, err := service.Apply(ctx, transition.ID, account.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "checked in")

	// Nothing changed — the child is still active (not stranded as alumnus).
	var status string
	require.NoError(t, db.NewSelect().TableExpr(`users.students`).Column("status").
		Where("id = ?", student.ID).Scan(ctx, &status))
	assert.Equal(t, string(users.StudentStatusActive), status)
}

// TestGradeTransitionService_SuggestMappings_MarksAmbiguous covers the P1 fix:
// a class name without a grade pattern must be flagged Ambiguous so the editor
// does not silently preselect Abgang for placeholder/free-form classes.
func TestGradeTransitionService_SuggestMappings_MarksAmbiguous(t *testing.T) {
	service, db, cleanup := setupGradeTransitionServiceTest(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(testpkg.TenantContext(1), 10*time.Second)
	defer cancel()

	ambiguousClass := "sonder" + letterOnlySuffix(t)
	numericSuffix := uuid.Must(uuid.NewV4()).String()[:8]
	numericClass := fmt.Sprintf("2num-%s", numericSuffix)

	ambiguousStudent := testpkg.CreateTestStudent(t, db, "Odd", "Class", ambiguousClass)
	numericStudent := testpkg.CreateTestStudent(t, db, "Normal", "Class", numericClass)
	defer testpkg.CleanupActivityFixtures(t, db, ambiguousStudent.ID, numericStudent.ID)

	suggestions, err := service.SuggestMappings(ctx)
	require.NoError(t, err)

	var foundAmbiguous, foundNumeric bool
	for _, s := range suggestions {
		switch s.FromClass {
		case ambiguousClass:
			foundAmbiguous = true
			assert.True(t, s.Ambiguous, "class without a grade pattern must be marked ambiguous")
			assert.True(t, s.IsGraduating, "ambiguous class keeps the legacy graduating hint")
		case numericClass:
			foundNumeric = true
			assert.False(t, s.Ambiguous, "a numeric class is a confident suggestion, not ambiguous")
		}
	}
	assert.True(t, foundAmbiguous, "expected a suggestion for %s", ambiguousClass)
	assert.True(t, foundNumeric, "expected a suggestion for %s", numericClass)
}
