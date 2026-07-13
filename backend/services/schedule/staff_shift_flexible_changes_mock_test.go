package schedule

// Unit tests for the flexible daily-change behaviour on staff shifts (#1841):
// a cancelled shift drops out of overlap, planned minutes, and coverage; a
// replacement points at a real same-day origin; the cover link is immutable on
// a plain edit. int64 literals are fake in-memory IDs, not DB rows.

import (
	"context"
	"database/sql"
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A brand-new shift marked cancelled is a deliberately-open gap; it never
// collides with an existing shift because it does not take place.
func TestShiftService_CreateCancelledSkipsOverlap(t *testing.T) {
	svc, repo, _ := shiftServiceFixture()

	existing := validShift(7) // 08:00–16:00
	existing.ID = 1
	repo.findByStaffAndDateRangeFunc = func(_ context.Context, _ int64, _, _ timezone.Date) ([]*scheduleModels.StaffShift, error) {
		return []*scheduleModels.StaffShift{existing}, nil
	}

	gap := validShift(7) // same window as existing
	gap.Cancelled = true

	_, err := svc.CreateShift(context.Background(), gap)
	require.NoError(t, err)
}

// A cancelled existing shift frees its window: a real shift may reuse it.
func TestShiftService_OverlapIgnoresCancelledExisting(t *testing.T) {
	svc, repo, _ := shiftServiceFixture()

	cancelled := validShift(7) // 08:00–16:00, absent
	cancelled.ID = 1
	cancelled.Cancelled = true
	repo.findByStaffAndDateRangeFunc = func(_ context.Context, _ int64, _, _ timezone.Date) ([]*scheduleModels.StaffShift, error) {
		return []*scheduleModels.StaffShift{cancelled}, nil
	}

	reused := validShift(7) // overlaps the cancelled window exactly

	_, err := svc.CreateShift(context.Background(), reused)
	require.NoError(t, err)
}

func TestShiftService_CreateReplacementRejectsMissingOrigin(t *testing.T) {
	svc, repo, _ := shiftServiceFixture()
	repo.findByIDFunc = func(_ context.Context, _ any) (*scheduleModels.StaffShift, error) {
		return nil, sql.ErrNoRows
	}

	replacement := validShift(8)
	replacement.OriginShiftID = int64Ptr(999)

	_, err := svc.CreateShift(context.Background(), replacement)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrShiftInvalid)
	assert.Contains(t, err.Error(), "origin shift not found")
}

func TestShiftService_CreateReplacementRejectsCrossDateOrigin(t *testing.T) {
	svc, repo, _ := shiftServiceFixture()
	repo.findByIDFunc = func(_ context.Context, _ any) (*scheduleModels.StaffShift, error) {
		origin := validShift(7)
		origin.ID = 5
		origin.Date = timezone.NewDate(2026, 7, 7) // one day off the replacement
		return origin, nil
	}

	replacement := validShift(8) // date 2026-07-06
	replacement.OriginShiftID = int64Ptr(5)

	_, err := svc.CreateShift(context.Background(), replacement)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrShiftInvalid)
	assert.Contains(t, err.Error(), "same date")
}

func TestShiftService_CreateReplacementAcceptsSameDayCancelledOrigin(t *testing.T) {
	svc, repo, _ := shiftServiceFixture()
	created := false
	repo.findByIDFunc = func(_ context.Context, _ any) (*scheduleModels.StaffShift, error) {
		origin := validShift(7) // same date as the replacement
		origin.ID = 5
		origin.Cancelled = true // a replacement only covers a gap the cancellation opened
		return origin, nil
	}
	repo.createFunc = func(_ context.Context, s *scheduleModels.StaffShift) error {
		created = true
		require.NotNil(t, s.OriginShiftID)
		assert.Equal(t, int64(5), *s.OriginShiftID)
		return nil
	}

	replacement := validShift(8)
	replacement.OriginShiftID = int64Ptr(5)

	_, err := svc.CreateShift(context.Background(), replacement)
	require.NoError(t, err)
	assert.True(t, created)
}

// A replacement may only cover a cancelled origin: an active origin still
// contributes its own planned minutes, so covering it would double-count.
func TestShiftService_CreateReplacementRejectsActiveOrigin(t *testing.T) {
	svc, repo, _ := shiftServiceFixture()
	repo.findByIDFunc = func(_ context.Context, _ any) (*scheduleModels.StaffShift, error) {
		origin := validShift(7) // same date, but NOT cancelled
		origin.ID = 5
		return origin, nil
	}

	replacement := validShift(8)
	replacement.OriginShiftID = int64Ptr(5)

	_, err := svc.CreateShift(context.Background(), replacement)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrShiftInvalid)
	assert.Contains(t, err.Error(), "cancelled shift")
}

// A plain edit that moves a replacement to a different day than its origin is
// rejected: the create-time invariant (a cover shares its origin's cancelled
// same-day gap) must hold after an update too (#1841).
func TestShiftService_UpdateReplacementRejectsCrossDateMove(t *testing.T) {
	svc, repo, _ := shiftServiceFixture()

	existing := validShift(8) // replacement on 2026-07-06
	existing.ID = 3
	existing.OriginShiftID = int64Ptr(5)
	origin := validShift(7) // origin on 2026-07-06, cancelled
	origin.ID = 5
	origin.Cancelled = true
	repo.findByIDFunc = func(_ context.Context, id any) (*scheduleModels.StaffShift, error) {
		if id == int64(5) {
			return origin, nil
		}
		return existing, nil
	}

	edit := validShift(8)
	edit.ID = 3
	edit.Date = timezone.NewDate(2026, 7, 7) // moved a day off its origin

	_, err := svc.UpdateShift(context.Background(), edit)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrShiftInvalid)
	assert.Contains(t, err.Error(), "same date")
}

// A plain edit never re-points the cover link: it stays whatever it was at
// creation, even when the request omits it.
func TestShiftService_UpdateKeepsOriginShiftID(t *testing.T) {
	svc, repo, _ := shiftServiceFixture()

	existing := validShift(8)
	existing.ID = 3
	existing.OriginShiftID = int64Ptr(5)
	repo.findByIDFunc = func(_ context.Context, _ any) (*scheduleModels.StaffShift, error) {
		return existing, nil
	}
	var saved *scheduleModels.StaffShift
	repo.updateFunc = func(_ context.Context, s *scheduleModels.StaffShift) error {
		saved = s
		return nil
	}

	edit := validShift(8)
	edit.ID = 3
	edit.OriginShiftID = nil // request drops it

	_, err := svc.UpdateShift(context.Background(), edit)
	require.NoError(t, err)
	require.NotNil(t, saved.OriginShiftID)
	assert.Equal(t, int64(5), *saved.OriginShiftID)
}

// A person cannot be entered as the replacement for their own cancelled shift:
// that marks them absent (the cancelled origin) and present (an active cover)
// at once, and the cancelled origin is invisible to overlap checks (#1841).
func TestShiftService_CreateReplacementRejectsSelfReplacement(t *testing.T) {
	svc, repo, _ := shiftServiceFixture()
	repo.findByIDFunc = func(_ context.Context, _ any) (*scheduleModels.StaffShift, error) {
		origin := validShift(7) // same staff member as the replacement below
		origin.ID = 5
		origin.Cancelled = true
		return origin, nil
	}

	replacement := validShift(7) // staff 7 covers staff 7's own cancelled shift
	replacement.OriginShiftID = int64Ptr(5)

	_, err := svc.CreateShift(context.Background(), replacement)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrShiftInvalid)
	assert.Contains(t, err.Error(), "its own replacement")
}

// Moving a covered ORIGIN to another date on a plain edit is rejected: its
// covers would be stranded on the old date while still pointing at it (#1841).
func TestShiftService_UpdateOriginRejectsDateMoveWithCovers(t *testing.T) {
	svc, repo, _ := shiftServiceFixture()

	existing := validShift(7) // origin on 2026-07-06, cancelled, not a replacement
	existing.ID = 5
	existing.Cancelled = true
	repo.findByIDFunc = func(_ context.Context, _ any) (*scheduleModels.StaffShift, error) {
		return existing, nil
	}
	cover := validShift(8)
	cover.ID = 11
	cover.OriginShiftID = int64Ptr(5)
	repo.findByOriginShiftIDFunc = func(_ context.Context, originID int64) ([]*scheduleModels.StaffShift, error) {
		assert.Equal(t, int64(5), originID)
		return []*scheduleModels.StaffShift{cover}, nil
	}

	edit := validShift(7)
	edit.ID = 5
	edit.Cancelled = true
	edit.Date = timezone.NewDate(2026, 7, 7) // moved a day off its covers

	_, err := svc.UpdateShift(context.Background(), edit)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrShiftInvalid)
	assert.Contains(t, err.Error(), "another date")
}

// An origin with no covers may still change date on a plain edit: the guard only
// fires when replacements actually reference the row.
func TestShiftService_UpdateOriginAllowsDateMoveWithoutCovers(t *testing.T) {
	svc, repo, _ := shiftServiceFixture()

	existing := validShift(7)
	existing.ID = 5
	repo.findByIDFunc = func(_ context.Context, _ any) (*scheduleModels.StaffShift, error) {
		return existing, nil
	}
	var saved *scheduleModels.StaffShift
	repo.updateFunc = func(_ context.Context, s *scheduleModels.StaffShift) error {
		saved = s
		return nil
	}

	edit := validShift(7)
	edit.ID = 5
	edit.Date = timezone.NewDate(2026, 7, 8)

	_, err := svc.UpdateShift(context.Background(), edit)
	require.NoError(t, err)
	require.NotNil(t, saved)
	assert.Equal(t, timezone.NewDate(2026, 7, 8), saved.Date)
}

// Marking an existing shift cancelled records the absence and skips overlap.
func TestShiftService_UpdateCanCancelShift(t *testing.T) {
	svc, repo, _ := shiftServiceFixture()

	existing := validShift(8)
	existing.ID = 3
	repo.findByIDFunc = func(_ context.Context, _ any) (*scheduleModels.StaffShift, error) {
		return existing, nil
	}
	overlapChecked := false
	repo.findByStaffAndDateRangeFunc = func(_ context.Context, _ int64, _, _ timezone.Date) ([]*scheduleModels.StaffShift, error) {
		overlapChecked = true
		return nil, nil
	}
	var saved *scheduleModels.StaffShift
	repo.updateFunc = func(_ context.Context, s *scheduleModels.StaffShift) error {
		saved = s
		return nil
	}

	edit := validShift(8)
	edit.ID = 3
	edit.Cancelled = true

	_, err := svc.UpdateShift(context.Background(), edit)
	require.NoError(t, err)
	assert.True(t, saved.Cancelled)
	assert.False(t, overlapChecked, "a cancelled shift skips the overlap lookup")
}

// An omitted change_reason preserves the stored one; an explicit value replaces it.
func TestShiftService_UpdatePreservesChangeReasonWhenOmitted(t *testing.T) {
	svc, repo, _ := shiftServiceFixture()

	existingReason := "krank"
	existing := validShift(8)
	existing.ID = 3
	existing.ChangeReason = &existingReason
	repo.findByIDFunc = func(_ context.Context, _ any) (*scheduleModels.StaffShift, error) {
		return existing, nil
	}
	var saved *scheduleModels.StaffShift
	repo.updateFunc = func(_ context.Context, s *scheduleModels.StaffShift) error {
		saved = s
		return nil
	}

	edit := validShift(8)
	edit.ID = 3
	edit.ChangeReason = nil

	_, err := svc.UpdateShiftWithOptions(context.Background(), edit, StaffShiftUpdateOptions{
		PreserveExistingChangeReason: true,
	})
	require.NoError(t, err)
	require.NotNil(t, saved.ChangeReason)
	assert.Equal(t, "krank", *saved.ChangeReason)
}

// Cancelling a shift with replacements flips the origin and creates each cover
// pointing at the (now cancelled) origin, in one call.
func TestShiftService_ApplyCancellation_CancelsAndCreatesReplacements(t *testing.T) {
	svc, repo, _ := shiftServiceFixture()

	origin := validShift(7)
	origin.ID = 5
	repo.findByIDFunc = func(_ context.Context, _ any) (*scheduleModels.StaffShift, error) {
		return origin, nil
	}
	repo.updateFunc = func(_ context.Context, s *scheduleModels.StaffShift) error {
		origin.Cancelled = s.Cancelled // the same-tx flip is visible to the cover creates
		return nil
	}
	var created []*scheduleModels.StaffShift
	repo.createFunc = func(_ context.Context, s *scheduleModels.StaffShift) error {
		created = append(created, s)
		return nil
	}

	reason := "krank"
	result, err := svc.ApplyCancellation(context.Background(), CancelShiftInput{
		ShiftID:      5,
		Cancelled:    true,
		ChangeReason: &reason,
		ActorStaffID: 1,
		Replacements: []ShiftReplacementInput{
			{StaffID: 8, StartTime: wall(8, 0), EndTime: wall(12, 0)},
			{StaffID: 9, StartTime: wall(12, 0), EndTime: wall(16, 0)},
		},
	})
	require.NoError(t, err)
	assert.True(t, result.Shift.Cancelled)
	require.Len(t, created, 2)
	require.Len(t, result.Replacements, 2)
	for _, cover := range created {
		require.NotNil(t, cover.OriginShiftID)
		assert.Equal(t, int64(5), *cover.OriginShiftID)
		require.NotNil(t, cover.ChangeReason)
		assert.Equal(t, "krank", *cover.ChangeReason)
	}
}

// When the caller supplies the origin's own edited window/type (ApplyOriginEdits),
// the cancellation applies them instead of preserving the stored values, so a
// time change made in the same save is not silently dropped (#1841).
func TestShiftService_ApplyCancellation_AppliesOriginEdits(t *testing.T) {
	svc, repo, _ := shiftServiceFixture()

	origin := validShift(7) // stored 08:00–16:00
	origin.ID = 5
	repo.findByIDFunc = func(_ context.Context, _ any) (*scheduleModels.StaffShift, error) {
		return origin, nil
	}
	var saved *scheduleModels.StaffShift
	repo.updateFunc = func(_ context.Context, s *scheduleModels.StaffShift) error {
		saved = s
		origin.Cancelled = s.Cancelled
		return nil
	}

	newType := int64(42)
	_, err := svc.ApplyCancellation(context.Background(), CancelShiftInput{
		ShiftID:          5,
		Cancelled:        true,
		ApplyOriginEdits: true,
		StartTime:        wall(9, 0),
		EndTime:          wall(14, 0),
		BreakMinutes:     15,
		ShiftTypeID:      &newType,
		ActorStaffID:     1,
	})
	require.NoError(t, err)
	require.NotNil(t, saved)
	assert.Equal(t, wall(9, 0), saved.StartTime)
	assert.Equal(t, wall(14, 0), saved.EndTime)
	assert.Equal(t, 15, saved.BreakMinutes)
	require.NotNil(t, saved.ShiftTypeID)
	assert.Equal(t, newType, *saved.ShiftTypeID)
}

// Without ApplyOriginEdits the stored window/type is preserved: a caller that
// only flips the flag must not zero out the origin's times.
func TestShiftService_ApplyCancellation_PreservesWindowWhenNotEditing(t *testing.T) {
	svc, repo, _ := shiftServiceFixture()

	origin := validShift(7) // stored 08:00–16:00
	origin.ID = 5
	repo.findByIDFunc = func(_ context.Context, _ any) (*scheduleModels.StaffShift, error) {
		return origin, nil
	}
	var saved *scheduleModels.StaffShift
	repo.updateFunc = func(_ context.Context, s *scheduleModels.StaffShift) error {
		saved = s
		origin.Cancelled = s.Cancelled
		return nil
	}

	_, err := svc.ApplyCancellation(context.Background(), CancelShiftInput{
		ShiftID:      5,
		Cancelled:    true,
		ActorStaffID: 1,
	})
	require.NoError(t, err)
	require.NotNil(t, saved)
	assert.Equal(t, wall(8, 0), saved.StartTime)
	assert.Equal(t, wall(16, 0), saved.EndTime)
}

// When rebuilding an existing cover set, a shift type that was deactivated after
// the cover was created is grandfathered in: re-saving the cancelled origin
// re-sends that type and must not be rejected, mirroring the ordinary-edit rule
// that lets a shift keep its already-attached inactive type (#1841).
func TestShiftService_ApplyCancellation_PreservesInactiveCoverType(t *testing.T) {
	svc, repo, _ := shiftServiceWithTypes(map[int64]*scheduleModels.ShiftType{
		4: {IsActive: false}, // deactivated after the cover was first created
	})

	origin := validShift(7)
	origin.ID = 5
	origin.Cancelled = true
	repo.findByIDFunc = func(_ context.Context, _ any) (*scheduleModels.StaffShift, error) {
		return origin, nil
	}
	repo.updateFunc = func(_ context.Context, s *scheduleModels.StaffShift) error {
		origin.Cancelled = s.Cancelled
		return nil
	}
	existingCover := validShift(8)
	existingCover.ID = 11
	existingCover.OriginShiftID = int64Ptr(5)
	existingCover.ShiftTypeID = int64Ptr(4) // the now-inactive type
	repo.findByOriginShiftIDFunc = func(_ context.Context, _ int64) ([]*scheduleModels.StaffShift, error) {
		return []*scheduleModels.StaffShift{existingCover}, nil
	}
	var created []*scheduleModels.StaffShift
	repo.createFunc = func(_ context.Context, s *scheduleModels.StaffShift) error {
		created = append(created, s)
		return nil
	}

	_, err := svc.ApplyCancellation(context.Background(), CancelShiftInput{
		ShiftID:      5,
		Cancelled:    true,
		ActorStaffID: 1,
		Replacements: []ShiftReplacementInput{
			{StaffID: 8, StartTime: wall(8, 0), EndTime: wall(16, 0), ShiftTypeID: int64Ptr(4)},
		},
	})
	require.NoError(t, err, "an existing cover's since-deactivated type must survive a rebuild")
	require.Len(t, created, 1)
	require.NotNil(t, created[0].ShiftTypeID)
	assert.Equal(t, int64(4), *created[0].ShiftTypeID)
}

// A brand-new cover (not in the existing set) with an inactive type is still
// rejected — only types a current cover already carries are grandfathered.
func TestShiftService_ApplyCancellation_RejectsNewInactiveCoverType(t *testing.T) {
	svc, repo, _ := shiftServiceWithTypes(map[int64]*scheduleModels.ShiftType{
		4: {IsActive: false},
	})

	origin := validShift(7)
	origin.ID = 5
	origin.Cancelled = true
	repo.findByIDFunc = func(_ context.Context, _ any) (*scheduleModels.StaffShift, error) {
		return origin, nil
	}
	repo.updateFunc = func(_ context.Context, s *scheduleModels.StaffShift) error {
		origin.Cancelled = s.Cancelled
		return nil
	}
	// No existing covers -> nothing grandfathered.
	repo.findByOriginShiftIDFunc = func(_ context.Context, _ int64) ([]*scheduleModels.StaffShift, error) {
		return nil, nil
	}

	_, err := svc.ApplyCancellation(context.Background(), CancelShiftInput{
		ShiftID:      5,
		Cancelled:    true,
		ActorStaffID: 1,
		Replacements: []ShiftReplacementInput{
			{StaffID: 8, StartTime: wall(8, 0), EndTime: wall(16, 0), ShiftTypeID: int64Ptr(4)},
		},
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrShiftTypeInactive)
}

// Reactivating a cancelled shift removes every existing replacement and creates
// none, so the plan never counts both the restored origin and its old covers.
func TestShiftService_ApplyCancellation_ReactivationRemovesReplacements(t *testing.T) {
	svc, repo, _ := shiftServiceFixture()

	origin := validShift(7)
	origin.ID = 5
	origin.Cancelled = true
	repo.findByIDFunc = func(_ context.Context, _ any) (*scheduleModels.StaffShift, error) {
		return origin, nil
	}
	repo.updateFunc = func(_ context.Context, s *scheduleModels.StaffShift) error {
		origin.Cancelled = s.Cancelled
		return nil
	}
	cover1 := validShift(8)
	cover1.ID = 11
	cover1.OriginShiftID = int64Ptr(5)
	cover2 := validShift(9)
	cover2.ID = 12
	cover2.OriginShiftID = int64Ptr(5)
	repo.findByOriginShiftIDFunc = func(_ context.Context, _ int64) ([]*scheduleModels.StaffShift, error) {
		return []*scheduleModels.StaffShift{cover1, cover2}, nil
	}
	var deleted []any
	repo.deleteFunc = func(_ context.Context, id any) error {
		deleted = append(deleted, id)
		return nil
	}
	createCalled := false
	repo.createFunc = func(_ context.Context, _ *scheduleModels.StaffShift) error {
		createCalled = true
		return nil
	}

	result, err := svc.ApplyCancellation(context.Background(), CancelShiftInput{
		ShiftID:      5,
		Cancelled:    false,
		ActorStaffID: 1,
	})
	require.NoError(t, err)
	assert.False(t, result.Shift.Cancelled)
	assert.ElementsMatch(t, []any{int64(11), int64(12)}, deleted)
	assert.False(t, createCalled, "reactivation must not create any replacement")
}

// Replacements only make sense while cancelling; supplying them on a
// reactivation is rejected before any write happens.
func TestShiftService_ApplyCancellation_RejectsReplacementsWhenNotCancelled(t *testing.T) {
	svc, _, _ := shiftServiceFixture()

	_, err := svc.ApplyCancellation(context.Background(), CancelShiftInput{
		ShiftID:      5,
		Cancelled:    false,
		Replacements: []ShiftReplacementInput{{StaffID: 8, StartTime: wall(8, 0), EndTime: wall(12, 0)}},
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrShiftInvalid)
}

// A replacement shift covers a gap; it is not itself a cancellable gap.
func TestShiftService_ApplyCancellation_RejectsCancellingAReplacement(t *testing.T) {
	svc, repo, _ := shiftServiceFixture()
	existing := validShift(8)
	existing.ID = 11
	existing.OriginShiftID = int64Ptr(5)
	repo.findByIDFunc = func(_ context.Context, _ any) (*scheduleModels.StaffShift, error) {
		return existing, nil
	}

	_, err := svc.ApplyCancellation(context.Background(), CancelShiftInput{
		ShiftID:      11,
		Cancelled:    true,
		ActorStaffID: 1,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrShiftInvalid)
}

// Cancelled shifts contribute zero planned minutes so the weekly Sollzeit delta
// reflects the real gap.
func TestPlannedShiftMinutes_ExcludesCancelled(t *testing.T) {
	date := timezone.NewDate(2026, 7, 6)
	worked := testShift(t, 7, date, "08:00", "16:00") // 480 net minutes
	absent := testShift(t, 7, date, "08:00", "16:00")
	absent.Cancelled = true

	planned := plannedShiftMinutes([]*scheduleModels.StaffShift{worked, absent})

	total := 0
	for _, minutes := range planned {
		total += minutes
	}
	assert.Equal(t, 480, total)
}

// A cancelled shift covers nothing, so a timetable assignment inside its window
// reads as fully uncovered.
func TestUncoveredShiftIntervals_CancelledShiftCoversNothing(t *testing.T) {
	date := timezone.NewDate(2026, 7, 6)
	shift := testShift(t, 7, date, "08:00", "16:00")
	shift.Cancelled = true

	start := testClock(t, "09:00")
	end := testClock(t, "12:00")
	gaps := uncoveredShiftIntervals(start, end, []*scheduleModels.StaffShift{shift})

	assert.Equal(t, [][2]string{{"09:00", "12:00"}}, formattedGaps(gaps))

	// Sanity: the same shift uncancelled fully covers the window.
	shift.Cancelled = false
	assert.Empty(t, formattedGaps(uncoveredShiftIntervals(start, end, []*scheduleModels.StaffShift{shift})))
}
