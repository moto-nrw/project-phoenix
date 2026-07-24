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

// TestGradeTransitionService_Revert_EnforcesReverseOrder covers the P1 fix:
// only the most recently applied transition may be reverted. Reverting an older
// one out of order would replay its history over the classes a newer transition
// has since written, so the server must reject it (409 → ErrNotLatestApplied)
// until the newer one is reverted first.
func TestGradeTransitionService_Revert_EnforcesReverseOrder(t *testing.T) {
	service, db, cleanup := setupGradeTransitionServiceTest(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(testpkg.TenantContext(1), 10*time.Second)
	defer cancel()

	account := testpkg.CreateTestAccount(t, db, "transition-reverse-order@test.local")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	suffix := uuid.Must(uuid.NewV4()).String()[:8]
	classA1 := fmt.Sprintf("1order-%s", suffix)
	classA2 := fmt.Sprintf("2order-%s", suffix)
	classB1 := fmt.Sprintf("3order-%s", suffix)
	classB2 := fmt.Sprintf("4order-%s", suffix)

	studentA := testpkg.CreateTestStudent(t, db, "Order", "A", classA1)
	studentB := testpkg.CreateTestStudent(t, db, "Order", "B", classB1)
	defer testpkg.CleanupActivityFixtures(t, db, studentA.ID, studentB.ID)

	// Apply the OLDER transition first, then a NEWER one on a different class.
	older := testpkg.CreateTestGradeTransition(t, db, "2024-2025", account.ID)
	testpkg.CreateTestGradeTransitionMapping(t, db, older.ID, classA1, testpkg.StrPtr(classA2))
	defer testpkg.CleanupGradeTransitionFixtures(t, db, older.ID)
	_, err := service.Apply(ctx, older.ID, account.ID)
	require.NoError(t, err)

	newer := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
	testpkg.CreateTestGradeTransitionMapping(t, db, newer.ID, classB1, testpkg.StrPtr(classB2))
	defer testpkg.CleanupGradeTransitionFixtures(t, db, newer.ID)
	_, err = service.Apply(ctx, newer.ID, account.ID)
	require.NoError(t, err)

	// Reverting the older one while the newer is still applied is refused.
	_, err = service.Revert(ctx, older.ID, account.ID)
	require.ErrorIs(t, err, educationService.ErrNotLatestApplied)

	// Reverting the latest works, and then the older one becomes revertable.
	_, err = service.Revert(ctx, newer.ID, account.ID)
	require.NoError(t, err)
	_, err = service.Revert(ctx, older.ID, account.ID)
	require.NoError(t, err)
}

// TestGradeTransitionService_Revert_PreservesLaterClassEdit covers the P2 fix:
// a revert must not clobber a class a child was moved into after the transition.
// A student promoted 1a -> 2a and then manually moved to 2b must stay in 2b when
// the transition is reverted, because their current class no longer matches the
// class the transition assigned.
func TestGradeTransitionService_Revert_PreservesLaterClassEdit(t *testing.T) {
	service, db, cleanup := setupGradeTransitionServiceTest(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(testpkg.TenantContext(1), 10*time.Second)
	defer cancel()

	account := testpkg.CreateTestAccount(t, db, "transition-preserve-edit@test.local")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	suffix := uuid.Must(uuid.NewV4()).String()[:8]
	fromClass := fmt.Sprintf("1edit-%s", suffix)
	toClass := fmt.Sprintf("2edit-%s", suffix)
	movedClass := fmt.Sprintf("2moved-%s", suffix)

	student := testpkg.CreateTestStudent(t, db, "Moved", "Child", fromClass)
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)

	transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
	testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, fromClass, testpkg.StrPtr(toClass))
	defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

	_, err := service.Apply(ctx, transition.ID, account.ID)
	require.NoError(t, err)

	// Admin manually moves the child to another class after the transition.
	_, err = db.NewUpdate().
		TableExpr(`users.students`).
		Set("school_class = ?", movedClass).
		Where("id = ?", student.ID).
		Exec(ctx)
	require.NoError(t, err)

	result, err := service.Revert(ctx, transition.ID, account.ID)
	require.NoError(t, err)

	// The manual correction survives — revert must NOT force the child back to
	// fromClass — and the skip is surfaced as a warning.
	var classAfterRevert string
	require.NoError(t, db.NewSelect().TableExpr(`users.students`).Column("school_class").
		Where("id = ?", student.ID).Scan(ctx, &classAfterRevert))
	assert.Equal(t, movedClass, classAfterRevert,
		"a since-moved child must keep the newer class, not be clobbered by the revert")
	assert.Equal(t, 0, result.StudentsPromoted, "the moved child is not counted as reverted")
	require.NotEmpty(t, result.Warnings)
	var warned bool
	for _, w := range result.Warnings {
		if strings.Contains(w, "class changed") {
			warned = true
		}
	}
	assert.True(t, warned, "expected a warning that a promoted student could not be reverted")
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
