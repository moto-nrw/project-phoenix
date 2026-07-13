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

func TestShiftService_CreateReplacementAcceptsSameDayOrigin(t *testing.T) {
	svc, repo, _ := shiftServiceFixture()
	created := false
	repo.findByIDFunc = func(_ context.Context, _ any) (*scheduleModels.StaffShift, error) {
		origin := validShift(7) // same date as the replacement
		origin.ID = 5
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
