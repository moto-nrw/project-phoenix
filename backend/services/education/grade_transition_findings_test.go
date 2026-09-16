package education_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	"github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/moto-nrw/project-phoenix/workflows/gradetransition"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// assertStudentStatus reads the lifecycle status a transition left behind.
//
// The late-arrival guard that used to live in this package
// (grade_transition_late_arrival_test.go) moved to
// workflows/gradetransition/apply_test.go as
// TestApplyRefusesChildAddedAfterCohortSnapshot: the only seam between the
// cohort snapshot and the re-read under the locks is the People Directory
// port, which a test in this package may not import.
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

// letterOnlySuffix returns a short digit-free token so a class name never
// accidentally matches the grade-number pattern (which any digit would).
//
// Hex digits are FOLDED onto letters rather than dropped: dropping them makes
// the token's length depend on how many letters the random UUID happened to
// contain, which can leave fewer characters than the slice below needs. Folding
// keeps the token digit-free, always at least 32 characters long, and as unique
// as the UUID it came from.
func letterOnlySuffix(t *testing.T) string {
	t.Helper()
	raw := uuid.Must(uuid.NewV4()).String()
	letters := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= '0' && r <= '9':
			return 'q' + (r - '0') // 0-9 -> q-z
		default:
			return -1 // the UUID's hyphens
		}
	}, raw)
	require.GreaterOrEqual(t, len(letters), 6)
	return letters[:6]
}

// TestGradeTransitionWorkflow_Revert_RestoresOriginalStatus covers the P1 fix:
// graduation includes every non-alumnus row, so a class may hold pending
// (future) enrollments. A revert must return each graduate to the status it
// held before the transition, not blanket-activate everyone.
func TestGradeTransitionWorkflow_Revert_RestoresOriginalStatus(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	f := newTransitionFixture(t, db)
	wf := f.workflow(t)

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 10*time.Second)
	defer cancel()

	suffix := uuid.Must(uuid.NewV4()).String()[:8]
	gradClass := fmt.Sprintf("4pending-%s", suffix)

	student := testpkg.CreateTestStudent(t, db, "Future", "Enrollment", gradClass)

	// This child is a pending future enrollment, not an active one.
	_, err := db.NewUpdate().
		TableExpr(`users.students`).
		Set("status = ?", string(users.StudentStatusPending)).
		Where("id = ?", student.ID).
		Exec(ctx)
	require.NoError(t, err)

	id := f.createDraft(t, ctx, "2025-2026", graduate(gradClass))

	_, err = wf.Apply(ctx, id, "")
	require.NoError(t, err)

	// Graduated -> alumnus (soft-deleted).
	var status string
	require.NoError(t, db.NewSelect().TableExpr(`users.students`).Column("status").
		Where("id = ?", student.ID).Scan(ctx, &status))
	assert.Equal(t, string(users.StudentStatusAlumnus), status)

	_, err = wf.Revert(ctx, id)
	require.NoError(t, err)

	// Restored to PENDING, not active — a future enrollment must not be
	// silently activated by a revert.
	require.NoError(t, db.NewSelect().TableExpr(`users.students`).Column("status").
		Where("id = ?", student.ID).Scan(ctx, &status))
	assert.Equal(t, string(users.StudentStatusPending), status,
		"revert must restore the pre-transition status, not blanket-activate")
}

// TestGradeTransitionWorkflow_Revert_EnforcesReverseOrder covers the P1 fix:
// only the most recently applied transition may be reverted. Reverting an older
// one out of order would replay its history over the classes a newer transition
// has since written, so the server must reject it (409 → ErrNotLatestApplied)
// until the newer one is reverted first.
func TestGradeTransitionWorkflow_Revert_EnforcesReverseOrder(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	f := newTransitionFixture(t, db)
	wf := f.workflow(t)

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 10*time.Second)
	defer cancel()

	suffix := uuid.Must(uuid.NewV4()).String()[:8]
	classA1 := fmt.Sprintf("1order-%s", suffix)
	classA2 := fmt.Sprintf("2order-%s", suffix)
	classB1 := fmt.Sprintf("3order-%s", suffix)
	classB2 := fmt.Sprintf("4order-%s", suffix)

	testpkg.CreateTestStudent(t, db, "Order", "A", classA1)
	testpkg.CreateTestStudent(t, db, "Order", "B", classB1)

	// Apply the OLDER transition first, then a NEWER one on a different class.
	older := f.createDraft(t, ctx, "2024-2025", promote(classA1, classA2))
	_, err := wf.Apply(ctx, older, "")
	require.NoError(t, err)

	newer := f.createDraft(t, ctx, "2025-2026", promote(classB1, classB2))
	_, err = wf.Apply(ctx, newer, "")
	require.NoError(t, err)

	// Reverting the older one while the newer is still applied is refused.
	_, err = wf.Revert(ctx, older)
	require.ErrorIs(t, err, gradetransition.ErrNotLatestApplied)

	// Reverting the latest works, and then the older one becomes revertable.
	_, err = wf.Revert(ctx, newer)
	require.NoError(t, err)
	_, err = wf.Revert(ctx, older)
	require.NoError(t, err)
}

// TestGradeTransitionWorkflow_Revert_PreservesLaterClassEdit covers the P2 fix:
// a revert must not clobber a class a child was moved into after the transition.
// A student promoted 1a -> 2a and then manually moved to 2b must stay in 2b when
// the transition is reverted, because their current class no longer matches the
// class the transition assigned.
func TestGradeTransitionWorkflow_Revert_PreservesLaterClassEdit(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	f := newTransitionFixture(t, db)
	wf := f.workflow(t)

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 10*time.Second)
	defer cancel()

	suffix := uuid.Must(uuid.NewV4()).String()[:8]
	fromClass := fmt.Sprintf("1edit-%s", suffix)
	toClass := fmt.Sprintf("2edit-%s", suffix)
	movedClass := fmt.Sprintf("2moved-%s", suffix)

	student := testpkg.CreateTestStudent(t, db, "Moved", "Child", fromClass)

	id := f.createDraft(t, ctx, "2025-2026", promote(fromClass, toClass))

	_, err := wf.Apply(ctx, id, "")
	require.NoError(t, err)

	// Admin manually moves the child to another class after the transition.
	_, err = db.NewUpdate().
		TableExpr(`users.students`).
		Set("school_class = ?", movedClass).
		Where("id = ?", student.ID).
		Exec(ctx)
	require.NoError(t, err)

	result, err := wf.Revert(ctx, id)
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

// TestGradeTransitionWorkflow_Apply_RejectsCheckedInGraduate covers the P1 fix:
// a graduating child with an open visit would become an alumnus the kiosk can
// no longer check out. The apply must be refused until they are checked out.
func TestGradeTransitionWorkflow_Apply_RejectsCheckedInGraduate(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	f := newTransitionFixture(t, db)
	wf := f.workflow(t)

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 10*time.Second)
	defer cancel()

	suffix := uuid.Must(uuid.NewV4()).String()[:8]
	gradClass := fmt.Sprintf("4checkedin-%s", suffix)

	activityGroup := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("AG-%s", suffix))
	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("Room-%s", suffix))
	activeGroup := testpkg.CreateTestActiveGroup(t, db, activityGroup.ID, room.ID)
	student := testpkg.CreateTestStudent(t, db, "Checked", "In", gradClass)

	// Open visit (nil exit time) = currently checked into a room.
	testpkg.CreateTestVisit(t, db, student.ID, activeGroup.ID, time.Now().Add(-time.Hour), nil)

	id := f.createDraft(t, ctx, "2025-2026", graduate(gradClass))

	_, err := wf.Apply(ctx, id, "")
	require.ErrorIs(t, err, gradetransition.ErrGraduatesCheckedIn)
	assert.Contains(t, err.Error(), "checked in")
	assert.Contains(t, err.Error(), "1 student(s) must be checked out first")

	// Nothing changed — the child is still active (not stranded as alumnus).
	var status string
	require.NoError(t, db.NewSelect().TableExpr(`users.students`).Column("status").
		Where("id = ?", student.ID).Scan(ctx, &status))
	assert.Equal(t, string(users.StudentStatusActive), status)
}

// TestGradeTransitionWorkflow_SuggestMappings_MarksAmbiguous covers the P1 fix:
// a class name without a grade pattern must be flagged Ambiguous so the editor
// does not silently preselect Abgang for placeholder/free-form classes.
func TestGradeTransitionWorkflow_SuggestMappings_MarksAmbiguous(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	f := newTransitionFixture(t, db)
	wf := f.workflow(t)

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 10*time.Second)
	defer cancel()

	// Both names are built from digit-free tokens so their shape is deterministic:
	// the ambiguous one carries no digit at all, and the numeric one is exactly
	// "<grade><letters>", which the production pattern always matches. A random
	// hex suffix would sometimes contain no digit after the hyphen and make the
	// "numeric" class ambiguous too.
	ambiguousClass := "sonder" + letterOnlySuffix(t)
	numericClass := "2num" + letterOnlySuffix(t)

	testpkg.CreateTestStudent(t, db, "Odd", "Class", ambiguousClass)
	testpkg.CreateTestStudent(t, db, "Normal", "Class", numericClass)

	suggestions, err := wf.SuggestMappings(ctx)
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

// TestGradeTransitionWorkflow_Apply_RejectsStalePreview covers the P1 fix:
// the admin's confirmation is bound to the preview they reviewed. If another
// admin moves a child into a graduating class after the preview was rendered,
// applying the confirmed preview would graduate a child nobody approved — so the
// apply must be refused (409 → ErrPreviewStale) until the preview is reloaded.
func TestGradeTransitionWorkflow_Apply_RejectsStalePreview(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	f := newTransitionFixture(t, db)
	wf := f.workflow(t)

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 15*time.Second)
	defer cancel()

	suffix := uuid.Must(uuid.NewV4()).String()[:8]
	gradClass := fmt.Sprintf("4stale-%s", suffix)
	otherClass := fmt.Sprintf("3stale-%s", suffix)

	reviewed := testpkg.CreateTestStudent(t, db, "Reviewed", "Child", gradClass)
	latecomer := testpkg.CreateTestStudent(t, db, "Late", "Child", otherClass)

	id := f.createDraft(t, ctx, "2025-2026", graduate(gradClass))

	preview, err := wf.Preview(ctx, id)
	require.NoError(t, err)
	require.NotEmpty(t, preview.Fingerprint, "preview must expose a fingerprint to confirm against")
	require.Equal(t, 1, preview.ToGraduate)

	// Another admin moves a second child into the graduating class.
	_, err = db.NewUpdate().
		TableExpr(`users.students`).
		Set("school_class = ?", gradClass).
		Where("id = ?", latecomer.ID).
		Exec(ctx)
	require.NoError(t, err)

	_, err = wf.Apply(ctx, id, preview.Fingerprint)
	require.ErrorIs(t, err, gradetransition.ErrPreviewStale)

	// Nothing was applied: both children keep their status and the transition
	// stays a draft.
	assertStudentStatus(t, db, reviewed.ID, users.StudentStatusActive)
	assertStudentStatus(t, db, latecomer.ID, users.StudentStatusActive)

	current, err := wf.FindTransition(ctx, id)
	require.NoError(t, err)
	assert.True(t, current.IsDraft(), "a refused apply leaves the transition a draft")

	// Reloading the preview shows the new reality and unblocks the apply.
	fresh, err := wf.Preview(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, 2, fresh.ToGraduate, "the reloaded preview includes the child that was moved in")
	assert.NotEqual(t, preview.Fingerprint, fresh.Fingerprint)

	_, err = wf.Apply(ctx, id, fresh.Fingerprint)
	require.NoError(t, err)
	assertStudentStatus(t, db, reviewed.ID, users.StudentStatusAlumnus)
	assertStudentStatus(t, db, latecomer.ID, users.StudentStatusAlumnus)
}
