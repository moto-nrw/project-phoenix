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
	t.Parallel()

	svc, repo, _ := shiftServiceFixture()

	existing := validShift(7) // 08:00–16:00
	existing.ID = 1
	repo.findByStaffAndDateRangeFunc = func(_ context.Context, _ int64, _, _ scheduleModels.Date) ([]*scheduleModels.StaffShift, error) {
		return []*scheduleModels.StaffShift{existing}, nil
	}

	gap := validShift(7) // same window as existing
	gap.Cancelled = true

	_, err := svc.CreateShift(context.Background(), gap)
	require.NoError(t, err)
}

// A cancelled existing shift frees its window: a real shift may reuse it.
func TestShiftService_OverlapIgnoresCancelledExisting(t *testing.T) {
	t.Parallel()

	svc, repo, _ := shiftServiceFixture()

	cancelled := validShift(7) // 08:00–16:00, absent
	cancelled.ID = 1
	cancelled.Cancelled = true
	repo.findByStaffAndDateRangeFunc = func(_ context.Context, _ int64, _, _ scheduleModels.Date) ([]*scheduleModels.StaffShift, error) {
		return []*scheduleModels.StaffShift{cancelled}, nil
	}

	reused := validShift(7) // overlaps the cancelled window exactly

	_, err := svc.CreateShift(context.Background(), reused)
	require.NoError(t, err)
}

func TestShiftService_CreateReplacementRejectsMissingOrigin(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	svc, repo, _ := shiftServiceFixture()
	repo.findByIDFunc = func(_ context.Context, _ any) (*scheduleModels.StaffShift, error) {
		origin := validShift(7)
		origin.ID = 5
		origin.Date = scheduleModels.NewDate(2026, 7, 7) // one day off the replacement
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
	t.Parallel()

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
	t.Parallel()

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

// A replacement must fall entirely within the origin's window: it covers part
// of the origin's gap, so a wider window (06:00-18:00 over an 08:00-16:00
// origin) would inflate the covering employee's planned minutes and auto-
// checkout beyond the actual gap (#1841).
func TestShiftService_CreateReplacementRejectsWindowOutsideOrigin(t *testing.T) {
	t.Parallel()

	svc, repo, _ := shiftServiceFixture()
	repo.findByIDFunc = func(_ context.Context, _ any) (*scheduleModels.StaffShift, error) {
		origin := validShift(7) // same date, cancelled, 08:00–16:00
		origin.ID = 5
		origin.Cancelled = true
		return origin, nil
	}

	replacement := validShift(8)
	replacement.OriginShiftID = int64Ptr(5)
	replacement.StartTime = wall(6, 0) // starts before the origin
	replacement.EndTime = wall(18, 0)  // ends after the origin

	_, err := svc.CreateShift(context.Background(), replacement)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrShiftInvalid)
	assert.Contains(t, err.Error(), "within the shift it covers")
}

// A replacement whose window sits exactly inside the origin's gap is accepted:
// containment is inclusive of shared boundaries.
func TestShiftService_CreateReplacementAcceptsWindowInsideOrigin(t *testing.T) {
	t.Parallel()

	svc, repo, _ := shiftServiceFixture()
	created := false
	repo.findByIDFunc = func(_ context.Context, _ any) (*scheduleModels.StaffShift, error) {
		origin := validShift(7) // 08:00–16:00, cancelled
		origin.ID = 5
		origin.Cancelled = true
		return origin, nil
	}
	repo.createFunc = func(_ context.Context, _ *scheduleModels.StaffShift) error {
		created = true
		return nil
	}

	replacement := validShift(8)
	replacement.OriginShiftID = int64Ptr(5)
	replacement.StartTime = wall(8, 0) // shares the origin's start boundary
	replacement.EndTime = wall(12, 0)  // ends inside the origin

	_, err := svc.CreateShift(context.Background(), replacement)
	require.NoError(t, err)
	assert.True(t, created)
}

// A plain edit that moves a replacement to a different day than its origin is
// rejected: the create-time invariant (a cover shares its origin's cancelled
// same-day gap) must hold after an update too (#1841).
func TestShiftService_UpdateReplacementRejectsCrossDateMove(t *testing.T) {
	t.Parallel()

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
	edit.Date = scheduleModels.NewDate(2026, 7, 7) // moved a day off its origin

	_, err := svc.UpdateShift(context.Background(), edit)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrShiftInvalid)
	assert.Contains(t, err.Error(), "same date")
}

// A same-day edit that resizes a replacement past its origin's window is
// rejected: the create-time containment invariant (a cover stays within the
// cancelled origin's gap) must survive a plain window edit, not only a date
// move. Without re-validating on a window change a 08:00-18:00 cover could
// attach to an 08:00-16:00 origin and inflate planned minutes / auto-checkout
// beyond the actual gap (#1841).
func TestShiftService_UpdateReplacementRejectsWindowOutsideOrigin(t *testing.T) {
	t.Parallel()

	svc, repo, _ := shiftServiceFixture()

	existing := validShift(8) // replacement on 2026-07-06, 08:00–16:00, inside origin
	existing.ID = 3
	existing.OriginShiftID = int64Ptr(5)
	origin := validShift(7) // origin on 2026-07-06, cancelled, 08:00–16:00
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
	edit.EndTime = wall(18, 0) // same day, but now extends past the origin's 16:00 end

	_, err := svc.UpdateShift(context.Background(), edit)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrShiftInvalid)
	assert.Contains(t, err.Error(), "within the shift it covers")
}

// A same-day edit that shrinks a cancelled ORIGIN around an existing
// replacement is rejected: the ordinary update path must not strand a cover
// outside the gap it fills. Rebuilding covers around a new window is the atomic
// cancellation flow's job, so a plain resize that leaves a cover hanging out is
// refused (#1841).
func TestShiftService_UpdateOriginRejectsResizeStrandingCover(t *testing.T) {
	t.Parallel()

	svc, repo, _ := shiftServiceFixture()

	origin := validShift(7) // origin on 2026-07-06, cancelled, 08:00–16:00
	origin.ID = 5
	origin.Cancelled = true
	cover := validShift(8) // replacement 10:00–16:00, inside the origin
	cover.ID = 3
	cover.StartTime = wall(10, 0)
	cover.OriginShiftID = int64Ptr(5)
	repo.findByIDFunc = func(_ context.Context, _ any) (*scheduleModels.StaffShift, error) {
		return origin, nil
	}
	repo.findByOriginShiftIDFunc = func(_ context.Context, _ int64) ([]*scheduleModels.StaffShift, error) {
		return []*scheduleModels.StaffShift{cover}, nil
	}

	edit := validShift(7)
	edit.ID = 5
	edit.EndTime = wall(12, 0) // shrinks the origin so the 10:00–16:00 cover no longer fits

	_, err := svc.UpdateShift(context.Background(), edit)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrShiftInvalid)
	assert.Contains(t, err.Error(), "no longer fits")
}

// Resizing a covered origin so its replacements still fit is allowed: the
// containment guard only blocks edits that would strand a cover, never a widen
// that keeps every cover inside the gap (#1841).
func TestShiftService_UpdateOriginResizeKeepingCoversSucceeds(t *testing.T) {
	t.Parallel()

	svc, repo, _ := shiftServiceFixture()

	origin := validShift(7) // origin on 2026-07-06, cancelled, 08:00–16:00
	origin.ID = 5
	origin.Cancelled = true
	cover := validShift(8) // replacement 10:00–16:00, inside the origin
	cover.ID = 3
	cover.StartTime = wall(10, 0)
	cover.OriginShiftID = int64Ptr(5)
	repo.findByIDFunc = func(_ context.Context, _ any) (*scheduleModels.StaffShift, error) {
		return origin, nil
	}
	repo.findByOriginShiftIDFunc = func(_ context.Context, _ int64) ([]*scheduleModels.StaffShift, error) {
		return []*scheduleModels.StaffShift{cover}, nil
	}
	repo.findByStaffAndDateRangeFunc = func(_ context.Context, _ int64, _, _ scheduleModels.Date) ([]*scheduleModels.StaffShift, error) {
		return nil, nil
	}
	var saved *scheduleModels.StaffShift
	repo.updateFunc = func(_ context.Context, s *scheduleModels.StaffShift) error {
		saved = s
		return nil
	}

	edit := validShift(7)
	edit.ID = 5
	edit.StartTime = wall(6, 0) // widens the origin; the 10:00–16:00 cover still fits
	edit.EndTime = wall(18, 0)

	_, err := svc.UpdateShift(context.Background(), edit)
	require.NoError(t, err)
	require.NotNil(t, saved)
	assert.Equal(t, wall(6, 0).Hour(), saved.StartTime.Hour())
}

// A plain edit never re-points the cover link: it stays whatever it was at
// creation, even when the request omits it.
func TestShiftService_UpdateKeepsOriginShiftID(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	edit.Date = scheduleModels.NewDate(2026, 7, 7) // moved a day off its covers

	_, err := svc.UpdateShift(context.Background(), edit)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrShiftInvalid)
	assert.Contains(t, err.Error(), "another date")
}

// An origin with no covers may still change date on a plain edit: the guard only
// fires when replacements actually reference the row.
func TestShiftService_UpdateOriginAllowsDateMoveWithoutCovers(t *testing.T) {
	t.Parallel()

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
	edit.Date = scheduleModels.NewDate(2026, 7, 8)

	_, err := svc.UpdateShift(context.Background(), edit)
	require.NoError(t, err)
	require.NotNil(t, saved)
	assert.Equal(t, scheduleModels.Date(timezone.NewDate(2026, 7, 8)), saved.Date)
}

// Marking an existing shift cancelled records the absence and skips overlap.
func TestShiftService_UpdateCanCancelShift(t *testing.T) {
	t.Parallel()

	svc, repo, _ := shiftServiceFixture()

	existing := validShift(8)
	existing.ID = 3
	repo.findByIDFunc = func(_ context.Context, _ any) (*scheduleModels.StaffShift, error) {
		return existing, nil
	}
	overlapChecked := false
	repo.findByStaffAndDateRangeFunc = func(_ context.Context, _ int64, _, _ scheduleModels.Date) ([]*scheduleModels.StaffShift, error) {
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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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

// Re-saving a cancelled origin rebuilds its cover set. An unchanged cover (same
// staff + window) keeps its OWN stored change reason instead of being stamped
// with the origin's reason, while a genuinely new cover takes the origin's
// reason (#1841).
func TestShiftService_ApplyCancellation_PreservesPerCoverChangeReason(t *testing.T) {
	t.Parallel()

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
	coverReason := "individueller grund"
	existingCover := validShift(8)
	existingCover.ID = 11
	existingCover.StartTime = wall(8, 0)
	existingCover.EndTime = wall(12, 0)
	existingCover.OriginShiftID = int64Ptr(5)
	existingCover.ChangeReason = &coverReason
	repo.findByOriginShiftIDFunc = func(_ context.Context, _ int64) ([]*scheduleModels.StaffShift, error) {
		return []*scheduleModels.StaffShift{existingCover}, nil
	}
	var created []*scheduleModels.StaffShift
	repo.createFunc = func(_ context.Context, s *scheduleModels.StaffShift) error {
		created = append(created, s)
		return nil
	}

	originReason := "origin grund"
	_, err := svc.ApplyCancellation(context.Background(), CancelShiftInput{
		ShiftID:      5,
		Cancelled:    true,
		ChangeReason: &originReason,
		ActorStaffID: 1,
		Replacements: []ShiftReplacementInput{
			// Unchanged cover — same staff + window as existingCover.
			{StaffID: 8, StartTime: wall(8, 0), EndTime: wall(12, 0)},
			// Brand-new cover.
			{StaffID: 9, StartTime: wall(12, 0), EndTime: wall(16, 0)},
		},
	})
	require.NoError(t, err)
	require.Len(t, created, 2)

	require.NotNil(t, created[0].ChangeReason)
	assert.Equal(t, "individueller grund", *created[0].ChangeReason,
		"an unchanged cover must keep its own reason, not the origin's")
	require.NotNil(t, created[1].ChangeReason)
	assert.Equal(t, "origin grund", *created[1].ChangeReason,
		"a genuinely new cover takes the origin's reason")
}

// The inactive-type grandfather is scoped to the staff member whose current
// cover carries the type. Transferring that type to a different person on the
// rebuild is rejected just like a brand-new cover would be (#1841).
func TestShiftService_ApplyCancellation_RejectsInactiveTypeTransfer(t *testing.T) {
	t.Parallel()

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
	// Staff 8 currently holds the inactive type.
	existingCover := validShift(8)
	existingCover.ID = 11
	existingCover.OriginShiftID = int64Ptr(5)
	existingCover.ShiftTypeID = int64Ptr(4)
	repo.findByOriginShiftIDFunc = func(_ context.Context, _ int64) ([]*scheduleModels.StaffShift, error) {
		return []*scheduleModels.StaffShift{existingCover}, nil
	}

	_, err := svc.ApplyCancellation(context.Background(), CancelShiftInput{
		ShiftID:      5,
		Cancelled:    true,
		ActorStaffID: 1,
		Replacements: []ShiftReplacementInput{
			// Different staff (9) tries to inherit staff 8's inactive type.
			{StaffID: 9, StartTime: wall(8, 0), EndTime: wall(16, 0), ShiftTypeID: int64Ptr(4)},
		},
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrShiftTypeInactive)
}

// existing is read before the advisory lock. If a concurrent edit commits while
// this request waits for the lock, the cancellation must build from the freshly
// re-read (locked) origin — preserving the newer window — not the stale pre-lock
// snapshot (#1841).
func TestShiftService_ApplyCancellation_ReloadsOriginAfterLock(t *testing.T) {
	t.Parallel()

	svc, repo, _ := shiftServiceFixture()

	stale := validShift(7) // pre-lock snapshot: 08:00–16:00
	stale.ID = 5
	fresh := validShift(7)
	fresh.ID = 5
	fresh.StartTime = wall(9, 0) // a concurrent edit shortened the window
	fresh.EndTime = wall(15, 0)

	calls := 0
	repo.findByIDFunc = func(_ context.Context, _ any) (*scheduleModels.StaffShift, error) {
		calls++
		if calls == 1 {
			return stale, nil // the read used only to compute the lock set
		}
		return fresh, nil // re-read under the lock sees the committed edit
	}
	var saved *scheduleModels.StaffShift
	repo.updateFunc = func(_ context.Context, s *scheduleModels.StaffShift) error {
		saved = s
		return nil
	}

	_, err := svc.ApplyCancellation(context.Background(), CancelShiftInput{
		ShiftID:      5,
		Cancelled:    true, // ApplyOriginEdits false: window is preserved, so it must be the FRESH one
		ActorStaffID: 1,
	})
	require.NoError(t, err)
	require.NotNil(t, saved)
	assert.Equal(t, wall(9, 0), saved.StartTime, "cancellation must preserve the re-read window, not the stale one")
	assert.Equal(t, wall(15, 0), saved.EndTime)
}

// A cover this cancellation drops (its staff absent from the new replacement set)
// must still be locked, so a concurrent edit/delete of it — which locks only that
// cover's staff — serializes against the delete-and-rebuild instead of racing it
// (#1841). The advisory lock is a no-op without a DB, so the lock set is observed
// directly via the test seam.
func TestShiftService_ApplyCancellation_LocksDroppedCoverStaff(t *testing.T) {
	t.Parallel()

	svc, repo, _ := shiftServiceFixture()

	origin := validShift(7) // origin owned by staff 7
	origin.ID = 5
	repo.findByIDFunc = func(_ context.Context, _ any) (*scheduleModels.StaffShift, error) {
		return origin, nil
	}
	repo.updateFunc = func(_ context.Context, s *scheduleModels.StaffShift) error {
		origin.Cancelled = s.Cancelled // the flip is visible to the cover creates
		return nil
	}
	// The origin currently has one cover owned by staff 3, whom the new replacement
	// set (staff 8) drops entirely.
	droppedCover := validShift(3)
	droppedCover.ID = 11
	droppedCover.OriginShiftID = int64Ptr(5)
	repo.findByOriginShiftIDFunc = func(_ context.Context, _ int64) ([]*scheduleModels.StaffShift, error) {
		return []*scheduleModels.StaffShift{droppedCover}, nil
	}
	repo.createFunc = func(_ context.Context, _ *scheduleModels.StaffShift) error { return nil }

	var lockSets [][]int64
	svc.(*staffShiftService).lockObserver = func(ids []int64) {
		lockSets = append(lockSets, append([]int64(nil), ids...))
	}

	_, err := svc.ApplyCancellation(context.Background(), CancelShiftInput{
		ShiftID:      5,
		Cancelled:    true,
		ActorStaffID: 1,
		Replacements: []ShiftReplacementInput{
			{StaffID: 8, StartTime: wall(9, 0), EndTime: wall(12, 0)},
		},
	})
	require.NoError(t, err)
	require.NotEmpty(t, lockSets)
	// The top-level cancellation lock set is the first acquisition; it must cover the
	// origin's staff (7), the requested replacement's staff (8), AND the dropped
	// cover's staff (3), taken in a single stable sorted order.
	assert.Equal(t, []int64{3, 7, 8}, lockSets[0],
		"the dropped cover's staff (3) must be folded into the cancellation lock set")
}

func TestShiftService_ApplyCancellation_RejectsCoverMovedAfterDiscovery(t *testing.T) {
	t.Parallel()

	svc, repo, _ := shiftServiceFixture()

	origin := validShift(7)
	origin.ID = 5
	repo.findByIDFunc = func(_ context.Context, _ any) (*scheduleModels.StaffShift, error) {
		return origin, nil
	}
	staleCover := validShift(3)
	staleCover.ID = 11
	staleCover.OriginShiftID = int64Ptr(origin.ID)
	movedCover := *staleCover
	movedCover.StaffID = 4
	coverReads := 0
	repo.findByOriginShiftIDFunc = func(_ context.Context, _ int64) ([]*scheduleModels.StaffShift, error) {
		coverReads++
		if coverReads == 1 {
			return []*scheduleModels.StaffShift{staleCover}, nil
		}
		return []*scheduleModels.StaffShift{&movedCover}, nil
	}
	deletes := 0
	repo.deleteFunc = func(_ context.Context, _ any) error {
		deletes++
		return nil
	}

	_, err := svc.ApplyCancellation(context.Background(), CancelShiftInput{
		ShiftID:      origin.ID,
		Cancelled:    true,
		ActorStaffID: 1,
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrShiftConflict)
	assert.Zero(t, deletes, "stale lock ownership must be rejected before mutating covers")
}
