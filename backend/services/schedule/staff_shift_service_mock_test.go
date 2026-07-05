package schedule

// Mock-based unit tests for the StaffShiftService (Dienstplan, #1376 core
// slice). int64 literals are fake in-memory IDs, not DB rows.

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Mocks
// ============================================================================

type shiftMockRepo struct {
	createFunc                  func(ctx context.Context, shift *scheduleModels.StaffShift) error
	findByIDFunc                func(ctx context.Context, id any) (*scheduleModels.StaffShift, error)
	updateFunc                  func(ctx context.Context, shift *scheduleModels.StaffShift) error
	deleteFunc                  func(ctx context.Context, id any) error
	findByDateRangeFunc         func(ctx context.Context, start, end timezone.Date) ([]*scheduleModels.StaffShift, error)
	findByStaffAndDateRangeFunc func(ctx context.Context, staffID int64, start, end timezone.Date) ([]*scheduleModels.StaffShift, error)
}

func (m *shiftMockRepo) Create(ctx context.Context, shift *scheduleModels.StaffShift) error {
	if m.createFunc != nil {
		return m.createFunc(ctx, shift)
	}
	return nil
}

func (m *shiftMockRepo) FindByID(ctx context.Context, id any) (*scheduleModels.StaffShift, error) {
	if m.findByIDFunc != nil {
		return m.findByIDFunc(ctx, id)
	}
	return nil, errors.New("not found")
}

func (m *shiftMockRepo) Update(ctx context.Context, shift *scheduleModels.StaffShift) error {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, shift)
	}
	return nil
}

func (m *shiftMockRepo) Delete(ctx context.Context, id any) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, id)
	}
	return nil
}

func (m *shiftMockRepo) FindByDateRange(ctx context.Context, start, end timezone.Date) ([]*scheduleModels.StaffShift, error) {
	if m.findByDateRangeFunc != nil {
		return m.findByDateRangeFunc(ctx, start, end)
	}
	return nil, nil
}

func (m *shiftMockRepo) FindByStaffAndDateRange(ctx context.Context, staffID int64, start, end timezone.Date) ([]*scheduleModels.StaffShift, error) {
	if m.findByStaffAndDateRangeFunc != nil {
		return m.findByStaffAndDateRangeFunc(ctx, staffID, start, end)
	}
	return nil, nil
}

func (m *shiftMockRepo) FindByStaffIDsAndDate(_ context.Context, _ []int64, _ timezone.Date) ([]*scheduleModels.StaffShift, error) {
	return nil, nil
}

type shiftMockStaffRepo struct {
	findByIDFunc func(ctx context.Context, id interface{}) (*usersModels.Staff, error)
}

func (m *shiftMockStaffRepo) FindByID(ctx context.Context, id interface{}) (*usersModels.Staff, error) {
	if m.findByIDFunc != nil {
		return m.findByIDFunc(ctx, id)
	}
	return &usersModels.Staff{}, nil
}

// The remaining StaffRepository methods are unused by the shift service.
func (m *shiftMockStaffRepo) Create(context.Context, *usersModels.Staff) error { return nil }
func (m *shiftMockStaffRepo) FindByPersonID(context.Context, int64) (*usersModels.Staff, error) {
	return nil, errors.New("not implemented")
}
func (m *shiftMockStaffRepo) Update(context.Context, *usersModels.Staff) error { return nil }
func (m *shiftMockStaffRepo) Delete(context.Context, interface{}) error        { return nil }
func (m *shiftMockStaffRepo) List(context.Context, map[string]interface{}) ([]*usersModels.Staff, error) {
	return nil, nil
}
func (m *shiftMockStaffRepo) ListAllWithPerson(context.Context) ([]*usersModels.Staff, error) {
	return nil, nil
}
func (m *shiftMockStaffRepo) UpdateNotes(context.Context, int64, string) error { return nil }
func (m *shiftMockStaffRepo) ClearWorkTimeModel(context.Context, int64) error  { return nil }
func (m *shiftMockStaffRepo) FindWithPerson(context.Context, int64) (*usersModels.Staff, error) {
	return nil, errors.New("not implemented")
}
func (m *shiftMockStaffRepo) FindByIDs(context.Context, []int64) (map[int64]*usersModels.Staff, error) {
	return nil, nil
}
func (m *shiftMockStaffRepo) FindWithPersonByIDs(context.Context, []int64) (map[int64]*usersModels.Staff, error) {
	return nil, nil
}
func (m *shiftMockStaffRepo) ListStaffByRoles(context.Context, []string) ([]*usersModels.StaffWithRoleInfo, error) {
	return nil, nil
}

// ============================================================================
// Helpers
// ============================================================================

func shiftServiceFixture() (StaffShiftService, *shiftMockRepo, *shiftMockStaffRepo) {
	repo := &shiftMockRepo{}
	staffRepo := &shiftMockStaffRepo{}
	return NewStaffShiftService(repo, staffRepo, nil, nil), repo, staffRepo
}

func wall(hour, minute int) time.Time {
	return time.Date(1, 1, 1, hour, minute, 0, 0, time.UTC)
}

func validShift(staffID int64) *scheduleModels.StaffShift {
	return &scheduleModels.StaffShift{
		StaffID:   staffID,
		Date:      timezone.NewDate(2026, time.July, 6),
		StartTime: wall(8, 0),
		EndTime:   wall(16, 0),
		CreatedBy: 1,
	}
}

// ============================================================================
// Tests
// ============================================================================

func TestShiftService_CreateSuccess(t *testing.T) {
	svc, repo, _ := shiftServiceFixture()
	created := false
	repo.createFunc = func(_ context.Context, _ *scheduleModels.StaffShift) error {
		created = true
		return nil
	}

	saved, err := svc.CreateShift(context.Background(), validShift(7))
	require.NoError(t, err)
	require.NotNil(t, saved)
	assert.True(t, created)
}

func TestShiftService_CreateRejectsInvalid(t *testing.T) {
	svc, _, _ := shiftServiceFixture()

	shift := validShift(7)
	shift.EndTime = wall(7, 0) // ends before it starts

	_, err := svc.CreateShift(context.Background(), shift)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrShiftInvalid)
}

func TestShiftService_CreateRejectsUnknownStaff(t *testing.T) {
	svc, _, staffRepo := shiftServiceFixture()
	staffRepo.findByIDFunc = func(_ context.Context, _ interface{}) (*usersModels.Staff, error) {
		return nil, sql.ErrNoRows
	}

	_, err := svc.CreateShift(context.Background(), validShift(999))
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrShiftInvalid)
}

func TestShiftService_CreatePropagatesStaffLookupError(t *testing.T) {
	svc, _, staffRepo := shiftServiceFixture()
	staffRepo.findByIDFunc = func(_ context.Context, _ interface{}) (*usersModels.Staff, error) {
		return nil, errors.New("connection reset")
	}

	_, err := svc.CreateShift(context.Background(), validShift(999))
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrShiftInvalid)
	assert.Contains(t, err.Error(), "connection reset")
}

func TestShiftService_CreateRejectsOverlap(t *testing.T) {
	svc, repo, _ := shiftServiceFixture()

	existing := validShift(7) // 08:00–16:00
	existing.ID = 1
	repo.findByStaffAndDateRangeFunc = func(_ context.Context, _ int64, _, _ timezone.Date) ([]*scheduleModels.StaffShift, error) {
		return []*scheduleModels.StaffShift{existing}, nil
	}

	overlapping := validShift(7)
	overlapping.StartTime = wall(15, 0)
	overlapping.EndTime = wall(18, 0)

	_, err := svc.CreateShift(context.Background(), overlapping)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrShiftOverlap)
}

func TestShiftService_CreateAllowsTouchingShifts(t *testing.T) {
	svc, repo, _ := shiftServiceFixture()

	existing := validShift(7) // 08:00–16:00
	existing.ID = 1
	repo.findByStaffAndDateRangeFunc = func(_ context.Context, _ int64, _, _ timezone.Date) ([]*scheduleModels.StaffShift, error) {
		return []*scheduleModels.StaffShift{existing}, nil
	}

	adjacent := validShift(7)
	adjacent.StartTime = wall(16, 0) // starts exactly at the other's end
	adjacent.EndTime = wall(18, 0)

	_, err := svc.CreateShift(context.Background(), adjacent)
	require.NoError(t, err)
}

func TestShiftService_UpdateExcludesSelfFromOverlap(t *testing.T) {
	svc, repo, _ := shiftServiceFixture()

	existing := validShift(7)
	existing.ID = 5
	repo.findByIDFunc = func(_ context.Context, _ any) (*scheduleModels.StaffShift, error) {
		return existing, nil
	}
	repo.findByStaffAndDateRangeFunc = func(_ context.Context, _ int64, _, _ timezone.Date) ([]*scheduleModels.StaffShift, error) {
		return []*scheduleModels.StaffShift{existing}, nil
	}

	update := validShift(7)
	update.ID = 5
	update.EndTime = wall(15, 0)

	saved, err := svc.UpdateShift(context.Background(), update)
	require.NoError(t, err)
	assert.Equal(t, wall(15, 0), saved.EndTime)
}

func TestShiftService_UpdateKeepsStaffAssignment(t *testing.T) {
	svc, repo, _ := shiftServiceFixture()

	existing := validShift(7)
	existing.ID = 5
	existing.CreatedBy = 3
	repo.findByIDFunc = func(_ context.Context, _ any) (*scheduleModels.StaffShift, error) {
		return existing, nil
	}

	update := validShift(99) // attempts to move the shift to another staff member
	update.ID = 5

	saved, err := svc.UpdateShift(context.Background(), update)
	require.NoError(t, err)
	assert.Equal(t, int64(7), saved.StaffID, "staff assignment must be immutable on update")
	assert.Equal(t, int64(3), saved.CreatedBy, "original creator must be preserved")
}

func TestShiftService_UpdatePreservesCreatedAt(t *testing.T) {
	svc, repo, _ := shiftServiceFixture()

	existing := validShift(7)
	existing.ID = 5
	existing.CreatedAt = time.Date(2026, time.June, 1, 10, 0, 0, 0, time.UTC)
	repo.findByIDFunc = func(_ context.Context, _ any) (*scheduleModels.StaffShift, error) {
		return existing, nil
	}

	update := validShift(7) // request-built model: zero CreatedAt
	update.ID = 5

	saved, err := svc.UpdateShift(context.Background(), update)
	require.NoError(t, err)
	assert.Equal(t, existing.CreatedAt, saved.CreatedAt,
		"zero CreatedAt would be written as DEFAULT and reset created_at to now()")
}

func TestShiftService_UpdateNotFound(t *testing.T) {
	svc, repo, _ := shiftServiceFixture()
	repo.findByIDFunc = func(_ context.Context, _ any) (*scheduleModels.StaffShift, error) {
		return nil, sql.ErrNoRows
	}

	update := validShift(7)
	update.ID = 12345

	_, err := svc.UpdateShift(context.Background(), update)
	assert.ErrorIs(t, err, ErrShiftNotFound)
}

func TestShiftService_UpdatePropagatesFindError(t *testing.T) {
	svc, repo, _ := shiftServiceFixture()
	repo.findByIDFunc = func(_ context.Context, _ any) (*scheduleModels.StaffShift, error) {
		return nil, errors.New("database timeout")
	}

	update := validShift(7)
	update.ID = 12345

	_, err := svc.UpdateShift(context.Background(), update)
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrShiftNotFound)
	assert.Contains(t, err.Error(), "database timeout")
}

func TestShiftService_DeleteNotFound(t *testing.T) {
	svc, repo, _ := shiftServiceFixture()
	repo.findByIDFunc = func(_ context.Context, _ any) (*scheduleModels.StaffShift, error) {
		return nil, sql.ErrNoRows
	}

	err := svc.DeleteShift(context.Background(), 12345)
	assert.ErrorIs(t, err, ErrShiftNotFound)
}

func TestShiftService_DeletePropagatesFindError(t *testing.T) {
	svc, repo, _ := shiftServiceFixture()
	repo.findByIDFunc = func(_ context.Context, _ any) (*scheduleModels.StaffShift, error) {
		return nil, errors.New("database timeout")
	}

	err := svc.DeleteShift(context.Background(), 12345)
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrShiftNotFound)
	assert.Contains(t, err.Error(), "database timeout")
}

func TestShiftService_ListRejectsTooLargeRange(t *testing.T) {
	svc, _, _ := shiftServiceFixture()

	start := timezone.NewDate(2026, time.January, 1)
	end := start.AddDays(maxShiftRangeDays + 1)

	_, err := svc.ListShifts(context.Background(), start, end)
	assert.ErrorIs(t, err, ErrShiftRangeTooLarge)
}

func TestShiftService_ListRejectsInvertedRange(t *testing.T) {
	svc, _, _ := shiftServiceFixture()

	start := timezone.NewDate(2026, time.July, 6)
	_, err := svc.ListShifts(context.Background(), start, start.AddDays(-1))
	assert.ErrorIs(t, err, ErrShiftInvalid)
}

func TestShiftService_ListShiftsForStaffRequiresStaffID(t *testing.T) {
	svc, _, _ := shiftServiceFixture()

	start := timezone.NewDate(2026, time.July, 6)
	_, err := svc.ListShiftsForStaff(context.Background(), 0, start, start)
	assert.ErrorIs(t, err, ErrShiftInvalid)
}
