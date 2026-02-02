package audit_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/audit"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// CreateBatch Tests
// ============================================================================

func TestWorkSessionEditRepository_CreateBatch(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).WorkSessionEdit
	sessionRepo := repositories.NewFactory(db).WorkSession
	ctx := context.Background()

	staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
	defer testpkg.CleanupActivityFixtures(t, db, 0, staff.ID)

	t.Run("creates multiple edit records", func(t *testing.T) {
		// Create a work session
		today := timezone.DateOfUTC(time.Now())
		session := &active.WorkSession{
			StaffID:     staff.ID,
			Date:        today,
			Status:      active.WorkSessionStatusPresent,
			CheckInTime: time.Now(),
			CreatedBy:   staff.ID,
		}
		err := sessionRepo.Create(ctx, session)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "active.work_sessions", session.ID)

		oldValue := "08:00"
		newValue := "09:00"
		edits := []*audit.WorkSessionEdit{
			{
				SessionID: session.ID,
				StaffID:   staff.ID,
				EditedBy:  staff.ID,
				FieldName: audit.FieldCheckInTime,
				OldValue:  &oldValue,
				NewValue:  &newValue,
			},
		}

		err = repo.CreateBatch(ctx, edits)
		require.NoError(t, err)
		assert.NotZero(t, edits[0].ID)

		testpkg.CleanupTableRecords(t, db, "audit.work_session_edits", edits[0].ID)
	})

	t.Run("handles empty batch", func(t *testing.T) {
		err := repo.CreateBatch(ctx, []*audit.WorkSessionEdit{})
		require.NoError(t, err) // Should not error on empty batch
	})

	t.Run("fails with nil edit in batch", func(t *testing.T) {
		edits := []*audit.WorkSessionEdit{nil}
		err := repo.CreateBatch(ctx, edits)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("validates field names", func(t *testing.T) {
		today := timezone.DateOfUTC(time.Now())
		session := &active.WorkSession{
			StaffID:     staff.ID,
			Date:        today,
			Status:      active.WorkSessionStatusPresent,
			CheckInTime: time.Now(),
			CreatedBy:   staff.ID,
		}
		err := sessionRepo.Create(ctx, session)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "active.work_sessions", session.ID)

		edits := []*audit.WorkSessionEdit{
			{
				SessionID: session.ID,
				StaffID:   staff.ID,
				EditedBy:  staff.ID,
				FieldName: "invalid_field",
			},
		}

		err = repo.CreateBatch(ctx, edits)
		assert.Error(t, err)
	})

	t.Run("validates missing session ID", func(t *testing.T) {
		edits := []*audit.WorkSessionEdit{
			{
				SessionID: 0, // Invalid
				StaffID:   staff.ID,
				EditedBy:  staff.ID,
				FieldName: audit.FieldCheckInTime,
			},
		}

		err := repo.CreateBatch(ctx, edits)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "session ID is required")
	})

	t.Run("validates missing staff ID", func(t *testing.T) {
		edits := []*audit.WorkSessionEdit{
			{
				SessionID: 1,
				StaffID:   0, // Invalid
				EditedBy:  staff.ID,
				FieldName: audit.FieldCheckInTime,
			},
		}

		err := repo.CreateBatch(ctx, edits)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "staff ID is required")
	})

	t.Run("validates missing edited_by", func(t *testing.T) {
		edits := []*audit.WorkSessionEdit{
			{
				SessionID: 1,
				StaffID:   staff.ID,
				EditedBy:  0, // Invalid
				FieldName: audit.FieldCheckInTime,
			},
		}

		err := repo.CreateBatch(ctx, edits)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "edited by is required")
	})

	t.Run("creates batch with multiple fields", func(t *testing.T) {
		today := timezone.DateOfUTC(time.Now())
		session := &active.WorkSession{
			StaffID:     staff.ID,
			Date:        today,
			Status:      active.WorkSessionStatusPresent,
			CheckInTime: time.Now(),
			CreatedBy:   staff.ID,
		}
		err := sessionRepo.Create(ctx, session)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "active.work_sessions", session.ID)

		oldCheckIn := "08:00"
		newCheckIn := "09:00"
		oldBreak := "30"
		newBreak := "45"

		edits := []*audit.WorkSessionEdit{
			{
				SessionID: session.ID,
				StaffID:   staff.ID,
				EditedBy:  staff.ID,
				FieldName: audit.FieldCheckInTime,
				OldValue:  &oldCheckIn,
				NewValue:  &newCheckIn,
			},
			{
				SessionID: session.ID,
				StaffID:   staff.ID,
				EditedBy:  staff.ID,
				FieldName: audit.FieldBreakMinutes,
				OldValue:  &oldBreak,
				NewValue:  &newBreak,
			},
		}

		err = repo.CreateBatch(ctx, edits)
		require.NoError(t, err)
		assert.NotZero(t, edits[0].ID)
		assert.NotZero(t, edits[1].ID)

		defer func() {
			testpkg.CleanupTableRecords(t, db, "audit.work_session_edits", edits[0].ID)
			testpkg.CleanupTableRecords(t, db, "audit.work_session_edits", edits[1].ID)
		}()
	})
}

// ============================================================================
// GetBySessionID Tests
// ============================================================================

func TestWorkSessionEditRepository_GetBySessionID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).WorkSessionEdit
	sessionRepo := repositories.NewFactory(db).WorkSession
	ctx := context.Background()

	staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
	defer testpkg.CleanupActivityFixtures(t, db, 0, staff.ID)

	t.Run("returns edit records for session", func(t *testing.T) {
		// Create a work session
		today := timezone.DateOfUTC(time.Now())
		session := &active.WorkSession{
			StaffID:     staff.ID,
			Date:        today,
			Status:      active.WorkSessionStatusPresent,
			CheckInTime: time.Now(),
			CreatedBy:   staff.ID,
		}
		err := sessionRepo.Create(ctx, session)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "active.work_sessions", session.ID)

		oldValue := "08:00"
		newValue := "09:00"
		edit := &audit.WorkSessionEdit{
			SessionID: session.ID,
			StaffID:   staff.ID,
			EditedBy:  staff.ID,
			FieldName: audit.FieldCheckInTime,
			OldValue:  &oldValue,
			NewValue:  &newValue,
		}

		err = repo.CreateBatch(ctx, []*audit.WorkSessionEdit{edit})
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "audit.work_session_edits", edit.ID)

		edits, err := repo.GetBySessionID(ctx, session.ID)
		require.NoError(t, err)
		assert.NotEmpty(t, edits)

		var found bool
		for _, e := range edits {
			if e.ID == edit.ID {
				found = true
				assert.Equal(t, audit.FieldCheckInTime, e.FieldName)
				break
			}
		}
		assert.True(t, found)
	})

	t.Run("returns empty for session with no edits", func(t *testing.T) {
		today := timezone.DateOfUTC(time.Now())
		session := &active.WorkSession{
			StaffID:     staff.ID,
			Date:        today,
			Status:      active.WorkSessionStatusPresent,
			CheckInTime: time.Now(),
			CreatedBy:   staff.ID,
		}
		err := sessionRepo.Create(ctx, session)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "active.work_sessions", session.ID)

		edits, err := repo.GetBySessionID(ctx, session.ID)
		require.NoError(t, err)
		assert.Empty(t, edits)
	})

	t.Run("orders edits by creation time descending", func(t *testing.T) {
		today := timezone.DateOfUTC(time.Now())
		session := &active.WorkSession{
			StaffID:     staff.ID,
			Date:        today,
			Status:      active.WorkSessionStatusPresent,
			CheckInTime: time.Now(),
			CreatedBy:   staff.ID,
		}
		err := sessionRepo.Create(ctx, session)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "active.work_sessions", session.ID)

		oldValue1 := "08:00"
		newValue1 := "09:00"
		oldValue2 := "09:00"
		newValue2 := "10:00"

		edit1 := &audit.WorkSessionEdit{
			SessionID: session.ID,
			StaffID:   staff.ID,
			EditedBy:  staff.ID,
			FieldName: audit.FieldCheckInTime,
			OldValue:  &oldValue1,
			NewValue:  &newValue1,
		}

		// Create first edit
		err = repo.CreateBatch(ctx, []*audit.WorkSessionEdit{edit1})
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "audit.work_session_edits", edit1.ID)

		// Wait a bit to ensure different timestamps
		time.Sleep(10 * time.Millisecond)

		edit2 := &audit.WorkSessionEdit{
			SessionID: session.ID,
			StaffID:   staff.ID,
			EditedBy:  staff.ID,
			FieldName: audit.FieldCheckInTime,
			OldValue:  &oldValue2,
			NewValue:  &newValue2,
		}

		// Create second edit
		err = repo.CreateBatch(ctx, []*audit.WorkSessionEdit{edit2})
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "audit.work_session_edits", edit2.ID)

		edits, err := repo.GetBySessionID(ctx, session.ID)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(edits), 2)

		// Most recent should be first
		if len(edits) >= 2 {
			assert.True(t, edits[0].CreatedAt.After(edits[1].CreatedAt) || edits[0].CreatedAt.Equal(edits[1].CreatedAt))
		}
	})
}

// ============================================================================
// CountBySessionID Tests
// ============================================================================

func TestWorkSessionEditRepository_CountBySessionID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).WorkSessionEdit
	sessionRepo := repositories.NewFactory(db).WorkSession
	ctx := context.Background()

	staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
	defer testpkg.CleanupActivityFixtures(t, db, 0, staff.ID)

	t.Run("returns count of edits for session", func(t *testing.T) {
		// Create a work session
		today := timezone.DateOfUTC(time.Now())
		session := &active.WorkSession{
			StaffID:     staff.ID,
			Date:        today,
			Status:      active.WorkSessionStatusPresent,
			CheckInTime: time.Now(),
			CreatedBy:   staff.ID,
		}
		err := sessionRepo.Create(ctx, session)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "active.work_sessions", session.ID)

		oldValue := "30"
		newValue := "45"
		edits := []*audit.WorkSessionEdit{
			{
				SessionID: session.ID,
				StaffID:   staff.ID,
				EditedBy:  staff.ID,
				FieldName: audit.FieldBreakMinutes,
				OldValue:  &oldValue,
				NewValue:  &newValue,
			},
		}

		err = repo.CreateBatch(ctx, edits)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "audit.work_session_edits", edits[0].ID)

		count, err := repo.CountBySessionID(ctx, session.ID)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, count, 1)
	})

	t.Run("returns zero for session with no edits", func(t *testing.T) {
		today := timezone.DateOfUTC(time.Now())
		session := &active.WorkSession{
			StaffID:     staff.ID,
			Date:        today,
			Status:      active.WorkSessionStatusPresent,
			CheckInTime: time.Now(),
			CreatedBy:   staff.ID,
		}
		err := sessionRepo.Create(ctx, session)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "active.work_sessions", session.ID)

		count, err := repo.CountBySessionID(ctx, session.ID)
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})
}

// ============================================================================
// CountBySessionIDs Tests
// ============================================================================

func TestWorkSessionEditRepository_CountBySessionIDs(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).WorkSessionEdit
	sessionRepo := repositories.NewFactory(db).WorkSession
	ctx := context.Background()

	staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
	defer testpkg.CleanupActivityFixtures(t, db, 0, staff.ID)

	t.Run("returns counts for multiple sessions", func(t *testing.T) {
		// Create two work sessions
		today := timezone.DateOfUTC(time.Now())
		session1 := &active.WorkSession{
			StaffID:     staff.ID,
			Date:        today,
			Status:      active.WorkSessionStatusPresent,
			CheckInTime: time.Now(),
			CreatedBy:   staff.ID,
		}
		session2 := &active.WorkSession{
			StaffID:     staff.ID,
			Date:        today.AddDate(0, 0, 1),
			Status:      active.WorkSessionStatusPresent,
			CheckInTime: time.Now(),
			CreatedBy:   staff.ID,
		}

		err := sessionRepo.Create(ctx, session1)
		require.NoError(t, err)
		err = sessionRepo.Create(ctx, session2)
		require.NoError(t, err)
		defer func() {
			testpkg.CleanupTableRecords(t, db, "active.work_sessions", session1.ID)
			testpkg.CleanupTableRecords(t, db, "active.work_sessions", session2.ID)
		}()

		oldValue := "08:00"
		newValue := "09:00"

		// Create edits for first session
		edit1 := &audit.WorkSessionEdit{
			SessionID: session1.ID,
			StaffID:   staff.ID,
			EditedBy:  staff.ID,
			FieldName: audit.FieldCheckInTime,
			OldValue:  &oldValue,
			NewValue:  &newValue,
		}

		// Create edits for second session
		edit2 := &audit.WorkSessionEdit{
			SessionID: session2.ID,
			StaffID:   staff.ID,
			EditedBy:  staff.ID,
			FieldName: audit.FieldCheckInTime,
			OldValue:  &oldValue,
			NewValue:  &newValue,
		}

		err = repo.CreateBatch(ctx, []*audit.WorkSessionEdit{edit1, edit2})
		require.NoError(t, err)
		defer func() {
			testpkg.CleanupTableRecords(t, db, "audit.work_session_edits", edit1.ID)
			testpkg.CleanupTableRecords(t, db, "audit.work_session_edits", edit2.ID)
		}()

		counts, err := repo.CountBySessionIDs(ctx, []int64{session1.ID, session2.ID})
		require.NoError(t, err)
		assert.GreaterOrEqual(t, counts[session1.ID], 1)
		assert.GreaterOrEqual(t, counts[session2.ID], 1)
	})

	t.Run("returns empty map for empty input", func(t *testing.T) {
		counts, err := repo.CountBySessionIDs(ctx, []int64{})
		require.NoError(t, err)
		assert.Empty(t, counts)
	})

	t.Run("returns zero counts for sessions with no edits", func(t *testing.T) {
		today := timezone.DateOfUTC(time.Now())
		session := &active.WorkSession{
			StaffID:     staff.ID,
			Date:        today,
			Status:      active.WorkSessionStatusPresent,
			CheckInTime: time.Now(),
			CreatedBy:   staff.ID,
		}
		err := sessionRepo.Create(ctx, session)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "active.work_sessions", session.ID)

		counts, err := repo.CountBySessionIDs(ctx, []int64{session.ID})
		require.NoError(t, err)
		// Session with no edits should not be in the map (or have zero count)
		_, exists := counts[session.ID]
		assert.False(t, exists, "session with no edits should not be in counts map")
	})
}
