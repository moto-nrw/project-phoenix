package active

import (
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tests for isNotFoundError helper
func TestIsNotFoundError(t *testing.T) {
	t.Parallel()

	t.Run("returns true for DatabaseError with repository not-found", func(t *testing.T) {
		err := &base.DatabaseError{
			Op:  "find by id",
			Err: base.ErrNotFound,
		}
		assert.True(t, base.IsNoRows(err))
	})

	t.Run("returns false for DatabaseError with other error", func(t *testing.T) {
		err := &base.DatabaseError{
			Op:  "find by id",
			Err: errors.New("connection refused"),
		}
		assert.False(t, base.IsNoRows(err))
	})

	t.Run("returns false for non-DatabaseError", func(t *testing.T) {
		err := errors.New("some error")
		assert.False(t, base.IsNoRows(err))
	})

	t.Run("returns false for nil error", func(t *testing.T) {
		assert.False(t, base.IsNoRows(nil))
	})

	t.Run("returns true for wrapped DatabaseError with repository not-found", func(t *testing.T) {
		dbErr := &base.DatabaseError{
			Op:  "find by id",
			Err: base.ErrNotFound,
		}
		// Wrap the error
		wrappedErr := errors.Join(errors.New("context"), dbErr)
		// Note: errors.As will find the DatabaseError in the chain
		assert.True(t, base.IsNoRows(wrappedErr))
	})
}

// Tests for isDuplicateActiveVisitViolation helper (Issue #844).
//
// This is the only test that exercises the partial unique index from
// migration 1.15.47 end-to-end: it inserts one open visit, then attempts
// a second open visit for the same (tenant_id, student_id) so PostgreSQL
// raises 23505 on uniq_active_visits_open_per_student. The captured error
// chain (*base.DatabaseError → bun → pgdriver.Error) is then fed into
// isDuplicateActiveVisitViolation to verify both:
//
//  1. The migration's index actually rejects the duplicate at the DB layer.
//  2. The detection function correctly identifies that error so
//     service.CreateVisit translates it to ErrStudentAlreadyActive (and
//     thus the IoT handler returns 409, not 500).
//
// Without this test, the unique-index → 23505 → ErrStudentAlreadyActive
// path has zero coverage: the existing TestActiveService_CreateVisit
// "returns ErrStudentAlreadyActive when student already has open visit"
// short-circuits inside the app-level ensureStudentHasNoActiveVisit
// check and never reaches visitRepo.Create.
func TestIsDuplicateActiveVisitViolation(t *testing.T) {
	t.Parallel()

	t.Run("returns false for nil error", func(t *testing.T) {
		assert.False(t, isDuplicateActiveVisitViolation(nil))
	})

	t.Run("returns false for unrelated error", func(t *testing.T) {
		assert.False(t, isDuplicateActiveVisitViolation(errors.New("connection refused")))
	})

	t.Run("returns false for DatabaseError without pgdriver error", func(t *testing.T) {
		err := &base.DatabaseError{Op: "create", Err: errors.New("not a pg error")}
		assert.False(t, isDuplicateActiveVisitViolation(err))
	})

	t.Run("returns true for real 23505 violation on uniq_active_visits_open_per_student", func(t *testing.T) {
		db := testpkg.SetupTestDB(t)

		ctx := testpkg.Ctx(t)
		repo := repositories.NewFactory(db).ActiveVisit

		// Hermetic fixtures.
		student := testpkg.CreateTestStudent(t, db, "DupViol", "Student", "1a")
		activity := testpkg.CreateTestActivityGroup(t, db, "DupViolActivity")
		room := testpkg.CreateTestRoom(t, db, "DupViolRoom")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)

		// First insert: succeeds and stays open (exit_time IS NULL).
		testpkg.CreateTestVisit(t, db, student.ID, activeGroup.ID, time.Now().Add(-1*time.Minute), nil)

		// Second insert: same (tenant_id, student_id, exit_time IS NULL)
		// — must be rejected by uniq_active_visits_open_per_student.
		duplicate := &activeModels.Visit{
			StudentID:     student.ID,
			ActiveGroupID: activeGroup.ID,
			EntryTime:     time.Now(),
		}
		err := repo.Create(ctx, duplicate)

		require.Error(t, err, "partial unique index from migration 1.15.47 must reject the second open visit")
		assert.Equal(t, int64(0), duplicate.ID, "rejected insert must not assign an ID")
		assert.True(t, isDuplicateActiveVisitViolation(err),
			"detection helper must recognise the 23505 on uniq_active_visits_open_per_student so service.CreateVisit returns ErrStudentAlreadyActive (Issue #844). got: %v", err)
	})
}
