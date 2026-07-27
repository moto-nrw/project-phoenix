package schedule

// Mock-based unit tests for the StaffShiftService (Dienstplan, #1376 core
// slice). int64 literals are fake in-memory IDs, not DB rows.

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
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
	findByOriginShiftIDFunc     func(ctx context.Context, originShiftID int64) ([]*scheduleModels.StaffShift, error)
	listFunc                    func(ctx context.Context, filters map[string]any) ([]*scheduleModels.StaffShift, error)
	updateColumnsFunc           func(ctx context.Context, shift *scheduleModels.StaffShift, columns ...string) (int64, error)
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

// FindByStaffIDsAndDateRange satisfies the batched interface method (#1417).
func (m *shiftMockRepo) FindByStaffIDsAndDateRange(context.Context, []int64, timezone.Date, timezone.Date) (map[int64][]*scheduleModels.StaffShift, error) {
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

func (m *shiftMockRepo) FindByOriginShiftID(ctx context.Context, originShiftID int64) ([]*scheduleModels.StaffShift, error) {
	if m.findByOriginShiftIDFunc != nil {
		return m.findByOriginShiftIDFunc(ctx, originShiftID)
	}
	return nil, nil
}

func (m *shiftMockRepo) FindByStaffIDsAndDates(_ context.Context, _ []int64, _ []timezone.Date) ([]*scheduleModels.StaffShift, error) {
	return nil, nil
}

// List and UpdateColumns satisfy the generic methods surfaced for the #1843
// sick cascade; the func fields follow the mock convention (nil = zero value).
func (m *shiftMockRepo) List(ctx context.Context, filters map[string]any) ([]*scheduleModels.StaffShift, error) {
	if m.listFunc != nil {
		return m.listFunc(ctx, filters)
	}
	return nil, nil
}

func (m *shiftMockRepo) UpdateColumns(ctx context.Context, shift *scheduleModels.StaffShift, columns ...string) (int64, error) {
	if m.updateColumnsFunc != nil {
		return m.updateColumnsFunc(ctx, shift, columns...)
	}
	return 0, nil
}

func (m *shiftMockRepo) FindUsedCalendarWeeks(_ context.Context, _, _ timezone.Date) ([]timezone.Date, error) {
	return nil, nil
}

func (m *shiftMockRepo) DeleteUpcomingByStaffID(context.Context, int64, timezone.Date) (int64, error) {
	return 0, nil
}

func (m *shiftMockRepo) BulkCreate(context.Context, []*scheduleModels.StaffShift) error {
	return nil
}

func (m *shiftMockRepo) DeleteNonDetachedBySeriesFrom(context.Context, int64, timezone.Date) (int64, error) {
	return 0, nil
}

func (m *shiftMockRepo) RepointDetachedSeriesFrom(context.Context, int64, int64, timezone.Date) (int64, error) {
	return 0, nil
}

func (m *shiftMockRepo) FindDetachedBySeriesFrom(context.Context, int64, timezone.Date) ([]*scheduleModels.StaffShift, error) {
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
func (m *shiftMockStaffRepo) FindByIDForUpdate(ctx context.Context, id int64) (*usersModels.Staff, error) {
	return m.FindByID(ctx, id)
}
func (m *shiftMockStaffRepo) FindByPersonID(context.Context, int64) (*usersModels.Staff, error) {
	return nil, errors.New("not implemented")
}
func (m *shiftMockStaffRepo) Update(context.Context, *usersModels.Staff) error { return nil }
func (m *shiftMockStaffRepo) Delete(context.Context, interface{}) error        { return nil }
func (m *shiftMockStaffRepo) List(context.Context, map[string]interface{}) ([]*usersModels.Staff, error) {
	return nil, nil
}
func (*shiftMockStaffRepo) FindReachableCalendarStaffIDs(context.Context, []int64) (map[int64]bool, error) {
	return map[int64]bool{}, nil
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
func (m *shiftMockStaffRepo) ListStaffWithPermission(context.Context, string) ([]*usersModels.StaffWithRoleInfo, error) {
	return nil, nil
}

func (m *shiftMockStaffRepo) GetStaffContactInfo(context.Context, int64) (*usersModels.StaffWithRoleInfo, error) {
	return nil, nil
}

func (m *shiftMockStaffRepo) ListStaffByRoles(context.Context, []string) ([]*usersModels.StaffWithRoleInfo, error) {
	return nil, nil
}

// stubShiftTypeService resolves shift types from an in-memory map so the shift
// service's active-flag enforcement can be exercised without a database. An
// unknown id maps to ErrShiftTypeNotFound, mirroring the real GetShiftType.
type stubShiftTypeService struct {
	types map[int64]*scheduleModels.ShiftType
	// listErr, when set, is returned by ListShiftTypes so attachShiftTypes'
	// error propagation can be exercised (#1844).
	listErr error
	// listCalls counts ListShiftTypes invocations so a test can assert the
	// resolve is skipped entirely when no shift carries a type.
	listCalls int
}

func (s *stubShiftTypeService) GetShiftType(_ context.Context, id int64) (*scheduleModels.ShiftType, error) {
	if t, ok := s.types[id]; ok {
		return t, nil
	}
	return nil, ErrShiftTypeNotFound
}

func (s *stubShiftTypeService) ListShiftTypes(context.Context) ([]*scheduleModels.ShiftType, error) {
	s.listCalls++
	if s.listErr != nil {
		return nil, s.listErr
	}
	out := make([]*scheduleModels.ShiftType, 0, len(s.types))
	for id, t := range s.types {
		t.ID = id // keep ID consistent with the map key so byID lookups resolve
		out = append(out, t)
	}
	return out, nil
}

func (s *stubShiftTypeService) CreateShiftType(_ context.Context, t *scheduleModels.ShiftType) (*scheduleModels.ShiftType, error) {
	return t, nil
}

func (s *stubShiftTypeService) UpdateShiftType(_ context.Context, t *scheduleModels.ShiftType) (*scheduleModels.ShiftType, error) {
	return t, nil
}

func (s *stubShiftTypeService) DeleteShiftType(context.Context, int64) error { return nil }

func (s *stubShiftTypeService) CreateDefaultShiftTypes(context.Context) ([]*scheduleModels.ShiftType, error) {
	return nil, nil
}

// ============================================================================
// Helpers
// ============================================================================

func shiftServiceFixture() (StaffShiftService, *shiftMockRepo, *shiftMockStaffRepo) {
	repo := &shiftMockRepo{}
	staffRepo := &shiftMockStaffRepo{}
	return NewStaffShiftService(repo, staffRepo, nil, nil, nil), repo, staffRepo
}

// shiftServiceWithTypes wires a stub ShiftTypeService so tests can exercise the
// active-flag enforcement on assigned shift types.
func shiftServiceWithTypes(types map[int64]*scheduleModels.ShiftType) (StaffShiftService, *shiftMockRepo, *shiftMockStaffRepo) {
	repo := &shiftMockRepo{}
	staffRepo := &shiftMockStaffRepo{}
	svc := NewStaffShiftService(repo, staffRepo, &stubShiftTypeService{types: types}, nil, nil)
	return svc, repo, staffRepo
}

func int64Ptr(v int64) *int64 { return &v }

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

func TestShiftService_CreateRejectsNilStaff(t *testing.T) {
	svc, _, staffRepo := shiftServiceFixture()
	staffRepo.findByIDFunc = func(_ context.Context, _ interface{}) (*usersModels.Staff, error) {
		return nil, nil
	}

	_, err := svc.CreateShift(context.Background(), validShift(999))
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrShiftInvalid)
	assert.Contains(t, err.Error(), "staff member not found")
}

func TestShiftService_CreatePropagatesLockError(t *testing.T) {
	repo := &shiftMockRepo{}
	staffRepo := &shiftMockStaffRepo{}
	svc := NewStaffShiftService(repo, staffRepo, nil, &bun.DB{}, nil)

	_, err := svc.CreateShift(context.Background(), validShift(7))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tenant id is required")
}

func TestShiftService_CreatePropagatesOverlapLookupError(t *testing.T) {
	svc, repo, _ := shiftServiceFixture()
	repo.findByStaffAndDateRangeFunc = func(_ context.Context, _ int64, _, _ timezone.Date) ([]*scheduleModels.StaffShift, error) {
		return nil, errors.New("read failed")
	}

	_, err := svc.CreateShift(context.Background(), validShift(7))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read failed")
}

func TestShiftService_CreatePropagatesCreateError(t *testing.T) {
	svc, repo, _ := shiftServiceFixture()
	repo.createFunc = func(_ context.Context, _ *scheduleModels.StaffShift) error {
		return errors.New("insert failed")
	}

	_, err := svc.CreateShift(context.Background(), validShift(7))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "insert failed")
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

func TestShiftService_UpdateRejectsConcurrentSameStaffMove(t *testing.T) {
	svc, repo, _ := shiftServiceFixture()

	existing := validShift(7)
	existing.ID = 5
	moved := *existing
	moved.Date = moved.Date.AddDays(1)
	moved.StartTime = wall(9, 0)
	moved.EndTime = wall(17, 0)

	reads := 0
	repo.findByIDFunc = func(_ context.Context, _ any) (*scheduleModels.StaffShift, error) {
		reads++
		if reads == 1 {
			return existing, nil
		}
		return &moved, nil
	}
	updates := 0
	repo.updateFunc = func(_ context.Context, _ *scheduleModels.StaffShift) error {
		updates++
		return nil
	}

	staleEdit := validShift(7)
	staleEdit.ID = existing.ID
	staleEdit.Notes = "stale edit"
	_, err := svc.UpdateShift(context.Background(), staleEdit)

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrShiftConflict)
	assert.Zero(t, updates, "a stale edit must not overwrite the moved slot")
}

func TestShiftService_UpdateClearsSickProvenanceWhenAdminKeepsCancellation(t *testing.T) {
	svc, repo, _ := shiftServiceFixture()
	absenceID := int64(91)
	existingReason := "Krankheit"
	existing := validShift(7)
	existing.ID = 5
	existing.Cancelled = true
	existing.ChangeReason = &existingReason
	existing.SickAbsenceID = &absenceID
	repo.findByIDFunc = func(_ context.Context, _ any) (*scheduleModels.StaffShift, error) {
		return existing, nil
	}

	manualReason := "Manuell abgesagt"
	update := validShift(7)
	update.ID = existing.ID
	update.Cancelled = true
	update.ChangeReason = &manualReason

	saved, err := svc.UpdateShift(context.Background(), update)

	require.NoError(t, err)
	assert.True(t, saved.Cancelled)
	assert.Nil(t, saved.SickAbsenceID)
	require.NotNil(t, saved.ChangeReason)
	assert.Equal(t, manualReason, *saved.ChangeReason)
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

func TestShiftService_UpdateCanPreserveExistingNotes(t *testing.T) {
	svc, repo, _ := shiftServiceFixture()

	existing := validShift(7)
	existing.ID = 5
	existing.Notes = "Existing note"
	repo.findByIDFunc = func(_ context.Context, _ any) (*scheduleModels.StaffShift, error) {
		return existing, nil
	}

	var persisted *scheduleModels.StaffShift
	repo.updateFunc = func(_ context.Context, shift *scheduleModels.StaffShift) error {
		persisted = shift
		return nil
	}

	update := validShift(7)
	update.ID = 5
	update.Notes = ""

	saved, err := svc.UpdateShiftWithOptions(context.Background(), update, StaffShiftUpdateOptions{PreserveExistingNotes: true})
	require.NoError(t, err)
	require.NotNil(t, persisted)
	assert.Equal(t, "Existing note", persisted.Notes)
	assert.Equal(t, "Existing note", saved.Notes)
}

func TestShiftService_UpdateExplicitNotesReplaceExistingNotes(t *testing.T) {
	svc, repo, _ := shiftServiceFixture()

	existing := validShift(7)
	existing.ID = 5
	existing.Notes = "Existing note"
	repo.findByIDFunc = func(_ context.Context, _ any) (*scheduleModels.StaffShift, error) {
		return existing, nil
	}

	update := validShift(7)
	update.ID = 5
	update.Notes = "Replacement"

	saved, err := svc.UpdateShiftWithOptions(context.Background(), update, StaffShiftUpdateOptions{})
	require.NoError(t, err)
	assert.Equal(t, "Replacement", saved.Notes)
}

func TestShiftService_UpdateExplicitEmptyNotesClearsExistingNotes(t *testing.T) {
	svc, repo, _ := shiftServiceFixture()

	existing := validShift(7)
	existing.ID = 5
	existing.Notes = "Existing note"
	repo.findByIDFunc = func(_ context.Context, _ any) (*scheduleModels.StaffShift, error) {
		return existing, nil
	}

	update := validShift(7)
	update.ID = 5
	update.Notes = ""

	saved, err := svc.UpdateShiftWithOptions(context.Background(), update, StaffShiftUpdateOptions{})
	require.NoError(t, err)
	assert.Empty(t, saved.Notes)
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

func TestShiftService_UpdateRejectsMissingID(t *testing.T) {
	svc, _, _ := shiftServiceFixture()

	_, err := svc.UpdateShift(context.Background(), validShift(7))
	assert.ErrorIs(t, err, ErrShiftNotFound)
}

func TestShiftService_UpdateRejectsNilExistingShift(t *testing.T) {
	svc, repo, _ := shiftServiceFixture()
	repo.findByIDFunc = func(_ context.Context, _ any) (*scheduleModels.StaffShift, error) {
		return nil, nil
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

func TestShiftService_UpdateRejectsInvalidMergedShift(t *testing.T) {
	svc, repo, _ := shiftServiceFixture()
	existing := validShift(7)
	existing.ID = 5
	repo.findByIDFunc = func(_ context.Context, _ any) (*scheduleModels.StaffShift, error) {
		return existing, nil
	}

	update := validShift(7)
	update.ID = 5
	update.EndTime = wall(7, 0)

	_, err := svc.UpdateShift(context.Background(), update)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrShiftInvalid)
}

func TestShiftService_UpdatePropagatesOverlap(t *testing.T) {
	svc, repo, _ := shiftServiceFixture()
	existing := validShift(7)
	existing.ID = 5
	conflicting := validShift(7)
	conflicting.ID = 6
	conflicting.StartTime = wall(15, 0)
	conflicting.EndTime = wall(18, 0)
	repo.findByIDFunc = func(_ context.Context, _ any) (*scheduleModels.StaffShift, error) {
		return existing, nil
	}
	repo.findByStaffAndDateRangeFunc = func(_ context.Context, _ int64, _, _ timezone.Date) ([]*scheduleModels.StaffShift, error) {
		return []*scheduleModels.StaffShift{existing, conflicting}, nil
	}

	update := validShift(7)
	update.ID = 5
	update.StartTime = wall(14, 0)
	update.EndTime = wall(17, 0)

	_, err := svc.UpdateShift(context.Background(), update)
	assert.ErrorIs(t, err, ErrShiftOverlap)
}

func TestShiftService_UpdatePropagatesLockError(t *testing.T) {
	repo := &shiftMockRepo{}
	staffRepo := &shiftMockStaffRepo{}
	svc := NewStaffShiftService(repo, staffRepo, nil, &bun.DB{}, nil)
	existing := validShift(7)
	existing.ID = 5
	repo.findByIDFunc = func(_ context.Context, _ any) (*scheduleModels.StaffShift, error) {
		return existing, nil
	}

	update := validShift(7)
	update.ID = 5

	_, err := svc.UpdateShift(context.Background(), update)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tenant id is required")
}

func TestShiftService_UpdatePropagatesUpdateError(t *testing.T) {
	svc, repo, _ := shiftServiceFixture()
	existing := validShift(7)
	existing.ID = 5
	repo.findByIDFunc = func(_ context.Context, _ any) (*scheduleModels.StaffShift, error) {
		return existing, nil
	}
	repo.updateFunc = func(_ context.Context, _ *scheduleModels.StaffShift) error {
		return errors.New("update failed")
	}

	update := validShift(7)
	update.ID = 5

	_, err := svc.UpdateShift(context.Background(), update)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "update failed")
}

func TestShiftService_DeleteNotFound(t *testing.T) {
	svc, repo, _ := shiftServiceFixture()
	repo.findByIDFunc = func(_ context.Context, _ any) (*scheduleModels.StaffShift, error) {
		return nil, sql.ErrNoRows
	}

	err := svc.DeleteShift(context.Background(), 12345)
	assert.ErrorIs(t, err, ErrShiftNotFound)
}

func TestShiftService_DeleteRejectsMissingID(t *testing.T) {
	svc, _, _ := shiftServiceFixture()

	err := svc.DeleteShift(context.Background(), 0)
	assert.ErrorIs(t, err, ErrShiftNotFound)
}

func TestShiftService_DeleteRejectsNilExistingShift(t *testing.T) {
	svc, repo, _ := shiftServiceFixture()
	repo.findByIDFunc = func(_ context.Context, _ any) (*scheduleModels.StaffShift, error) {
		return nil, nil
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

func TestShiftService_DeletePropagatesDeleteError(t *testing.T) {
	svc, repo, _ := shiftServiceFixture()
	existing := validShift(7)
	existing.ID = 5
	repo.findByIDFunc = func(_ context.Context, _ any) (*scheduleModels.StaffShift, error) {
		return existing, nil
	}
	repo.deleteFunc = func(_ context.Context, _ any) error {
		return errors.New("delete failed")
	}

	err := svc.DeleteShift(context.Background(), 5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "delete failed")
}

func TestShiftService_DeleteSuccess(t *testing.T) {
	svc, repo, _ := shiftServiceFixture()
	existing := validShift(7)
	existing.ID = 5
	repo.findByIDFunc = func(_ context.Context, _ any) (*scheduleModels.StaffShift, error) {
		return existing, nil
	}

	require.NoError(t, svc.DeleteShift(context.Background(), 5))
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

func TestShiftService_ListRejectsMissingRangeDates(t *testing.T) {
	svc, _, _ := shiftServiceFixture()

	_, err := svc.ListShifts(context.Background(), timezone.Date{}, timezone.NewDate(2026, time.July, 6))
	assert.ErrorIs(t, err, ErrShiftInvalid)

	_, err = svc.ListShifts(context.Background(), timezone.NewDate(2026, time.July, 6), timezone.Date{})
	assert.ErrorIs(t, err, ErrShiftInvalid)
}

func TestShiftService_ListDelegatesToRepository(t *testing.T) {
	svc, repo, _ := shiftServiceFixture()
	start := timezone.NewDate(2026, time.July, 6)
	end := start.AddDays(4)
	expected := []*scheduleModels.StaffShift{validShift(7)}
	repo.findByDateRangeFunc = func(_ context.Context, gotStart, gotEnd timezone.Date) ([]*scheduleModels.StaffShift, error) {
		assert.Equal(t, start, gotStart)
		assert.Equal(t, end, gotEnd)
		return expected, nil
	}

	shifts, err := svc.ListShifts(context.Background(), start, end)
	require.NoError(t, err)
	assert.Same(t, expected[0], shifts[0])
}

func TestShiftService_ListShiftsForStaffDelegatesToRepository(t *testing.T) {
	svc, repo, _ := shiftServiceFixture()
	start := timezone.NewDate(2026, time.July, 6)
	expected := []*scheduleModels.StaffShift{validShift(7)}
	repo.findByStaffAndDateRangeFunc = func(_ context.Context, staffID int64, gotStart, gotEnd timezone.Date) ([]*scheduleModels.StaffShift, error) {
		assert.Equal(t, int64(7), staffID)
		assert.Equal(t, start, gotStart)
		assert.Equal(t, start, gotEnd)
		return expected, nil
	}

	shifts, err := svc.ListShiftsForStaff(context.Background(), 7, start, start)
	require.NoError(t, err)
	assert.Same(t, expected[0], shifts[0])
}

func TestShiftService_ListShiftsForStaffRequiresStaffID(t *testing.T) {
	svc, _, _ := shiftServiceFixture()

	start := timezone.NewDate(2026, time.July, 6)
	_, err := svc.ListShiftsForStaff(context.Background(), 0, start, start)
	assert.ErrorIs(t, err, ErrShiftInvalid)
}

func TestShiftService_ListShiftsForStaffRejectsInvalidRange(t *testing.T) {
	svc, _, _ := shiftServiceFixture()

	start := timezone.NewDate(2026, time.July, 6)
	_, err := svc.ListShiftsForStaff(context.Background(), 7, start, start.AddDays(-1))
	assert.ErrorIs(t, err, ErrShiftInvalid)
}

func TestStaffShiftServiceDefaultsLogger(t *testing.T) {
	repo := &shiftMockRepo{}
	staffRepo := &shiftMockStaffRepo{}
	svc := NewStaffShiftService(repo, staffRepo, nil, nil, nil).(*staffShiftService)
	assert.Same(t, slog.Default(), svc.getLogger())
}

func TestStaffShiftServiceUsesInjectedLogger(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	svc := NewStaffShiftService(&shiftMockRepo{}, &shiftMockStaffRepo{}, nil, nil, logger).(*staffShiftService)
	assert.Same(t, logger, svc.getLogger())
}

// ============================================================================
// Shift-type active-flag enforcement (#1836)
// ============================================================================

func TestShiftService_CreateRejectsInactiveShiftType(t *testing.T) {
	svc, _, _ := shiftServiceWithTypes(map[int64]*scheduleModels.ShiftType{
		4: {IsActive: false},
	})

	shift := validShift(7)
	shift.ShiftTypeID = int64Ptr(4)

	_, err := svc.CreateShift(context.Background(), shift)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrShiftTypeInactive)
}

func TestShiftService_CreateAllowsActiveShiftType(t *testing.T) {
	svc, _, _ := shiftServiceWithTypes(map[int64]*scheduleModels.ShiftType{
		4: {IsActive: true},
	})

	shift := validShift(7)
	shift.ShiftTypeID = int64Ptr(4)

	_, err := svc.CreateShift(context.Background(), shift)
	require.NoError(t, err)
}

func TestShiftService_CreateRejectsUnknownShiftType(t *testing.T) {
	svc, _, _ := shiftServiceWithTypes(map[int64]*scheduleModels.ShiftType{})

	shift := validShift(7)
	shift.ShiftTypeID = int64Ptr(4)

	_, err := svc.CreateShift(context.Background(), shift)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrShiftTypeNotFound)
}

// An untyped shift never touches the shift-type service, so a nil dependency is
// fine and the create succeeds.
func TestShiftService_CreateWithoutTypeSkipsActiveCheck(t *testing.T) {
	svc, _, _ := shiftServiceFixture()

	_, err := svc.CreateShift(context.Background(), validShift(7))
	require.NoError(t, err)
}

func TestShiftService_UpdateKeepsAlreadyAttachedInactiveType(t *testing.T) {
	svc, repo, _ := shiftServiceWithTypes(map[int64]*scheduleModels.ShiftType{
		4: {IsActive: false},
	})

	existing := validShift(7)
	existing.ID = 5
	existing.ShiftTypeID = int64Ptr(4)
	repo.findByIDFunc = func(_ context.Context, _ any) (*scheduleModels.StaffShift, error) {
		return existing, nil
	}

	update := validShift(7)
	update.ID = 5
	update.ShiftTypeID = int64Ptr(4) // re-sends the same, now-inactive type

	_, err := svc.UpdateShift(context.Background(), update)
	require.NoError(t, err, "keeping a shift's already-attached inactive type must stay allowed")
}

func TestShiftService_UpdateRejectsSwitchToDifferentInactiveType(t *testing.T) {
	svc, repo, _ := shiftServiceWithTypes(map[int64]*scheduleModels.ShiftType{
		4: {IsActive: false},
		9: {IsActive: false},
	})

	existing := validShift(7)
	existing.ID = 5
	existing.ShiftTypeID = int64Ptr(4)
	repo.findByIDFunc = func(_ context.Context, _ any) (*scheduleModels.StaffShift, error) {
		return existing, nil
	}

	update := validShift(7)
	update.ID = 5
	update.ShiftTypeID = int64Ptr(9) // switches to a *different* inactive type

	_, err := svc.UpdateShift(context.Background(), update)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrShiftTypeInactive)
}

func TestShiftService_UpdateAllowsSwitchToActiveType(t *testing.T) {
	svc, repo, _ := shiftServiceWithTypes(map[int64]*scheduleModels.ShiftType{
		4: {IsActive: false},
		9: {IsActive: true},
	})

	existing := validShift(7)
	existing.ID = 5
	existing.ShiftTypeID = int64Ptr(4)
	repo.findByIDFunc = func(_ context.Context, _ any) (*scheduleModels.StaffShift, error) {
		return existing, nil
	}

	update := validShift(7)
	update.ID = 5
	update.ShiftTypeID = int64Ptr(9)

	_, err := svc.UpdateShift(context.Background(), update)
	require.NoError(t, err)
}

// ============================================================================
// Schichtart enrichment on reads — attachShiftTypes (#1844)
// ============================================================================

// shiftServiceWithStub wires a stub ShiftTypeService the caller controls so the
// read-time Schichtart enrichment (name + color) can be exercised end to end.
func shiftServiceWithStub(stub *stubShiftTypeService) (StaffShiftService, *shiftMockRepo) {
	repo := &shiftMockRepo{}
	svc := NewStaffShiftService(repo, &shiftMockStaffRepo{}, stub, nil, nil)
	return svc, repo
}

func TestShiftService_ListAttachesShiftTypes(t *testing.T) {
	stub := &stubShiftTypeService{types: map[int64]*scheduleModels.ShiftType{
		4: {Name: "Betreuung", Color: "#83CD2D", IsActive: true},
	}}
	svc, repo := shiftServiceWithStub(stub)

	typed := validShift(7)
	typed.ID = 1
	typed.ShiftTypeID = int64Ptr(4)
	untyped := validShift(8)
	untyped.ID = 2
	repo.findByDateRangeFunc = func(_ context.Context, _, _ timezone.Date) ([]*scheduleModels.StaffShift, error) {
		return []*scheduleModels.StaffShift{typed, untyped}, nil
	}

	start := timezone.NewDate(2026, time.July, 6)
	shifts, err := svc.ListShifts(context.Background(), start, start.AddDays(4))
	require.NoError(t, err)
	require.Len(t, shifts, 2)
	require.NotNil(t, shifts[0].ShiftType)
	assert.Equal(t, "Betreuung", shifts[0].ShiftType.Name)
	assert.Equal(t, "#83CD2D", shifts[0].ShiftType.Color)
	assert.Nil(t, shifts[1].ShiftType, "untyped shift keeps a nil Schichtart")
}

func TestShiftService_ListForStaffAttachesShiftTypes(t *testing.T) {
	stub := &stubShiftTypeService{types: map[int64]*scheduleModels.ShiftType{
		4: {Name: "Frühdienst", Color: "#5080D8", IsActive: true},
	}}
	svc, repo := shiftServiceWithStub(stub)

	typed := validShift(7)
	typed.ID = 1
	typed.ShiftTypeID = int64Ptr(4)
	repo.findByStaffAndDateRangeFunc = func(_ context.Context, _ int64, _, _ timezone.Date) ([]*scheduleModels.StaffShift, error) {
		return []*scheduleModels.StaffShift{typed}, nil
	}

	start := timezone.NewDate(2026, time.July, 6)
	shifts, err := svc.ListShiftsForStaff(context.Background(), 7, start, start)
	require.NoError(t, err)
	require.Len(t, shifts, 1)
	require.NotNil(t, shifts[0].ShiftType)
	assert.Equal(t, "Frühdienst", shifts[0].ShiftType.Name)
}

// When no shift in the range carries a type, the (potentially expensive)
// ListShiftTypes call is skipped entirely — proven by a stub whose list call
// would error if reached.
func TestShiftService_ListSkipsShiftTypeResolveWithoutTypedShifts(t *testing.T) {
	stub := &stubShiftTypeService{listErr: errors.New("must not be called")}
	svc, repo := shiftServiceWithStub(stub)

	untyped := validShift(7)
	untyped.ID = 1
	repo.findByDateRangeFunc = func(_ context.Context, _, _ timezone.Date) ([]*scheduleModels.StaffShift, error) {
		return []*scheduleModels.StaffShift{untyped}, nil
	}

	start := timezone.NewDate(2026, time.July, 6)
	shifts, err := svc.ListShifts(context.Background(), start, start.AddDays(4))
	require.NoError(t, err)
	require.Len(t, shifts, 1)
	assert.Zero(t, stub.listCalls, "no typed shift means no shift-type resolve")
}

func TestShiftService_ListPropagatesShiftTypeResolveError(t *testing.T) {
	stub := &stubShiftTypeService{
		types:   map[int64]*scheduleModels.ShiftType{4: {IsActive: true}},
		listErr: errors.New("shift types unavailable"),
	}
	svc, repo := shiftServiceWithStub(stub)

	typed := validShift(7)
	typed.ID = 1
	typed.ShiftTypeID = int64Ptr(4)
	repo.findByDateRangeFunc = func(_ context.Context, _, _ timezone.Date) ([]*scheduleModels.StaffShift, error) {
		return []*scheduleModels.StaffShift{typed}, nil
	}

	start := timezone.NewDate(2026, time.July, 6)
	_, err := svc.ListShifts(context.Background(), start, start.AddDays(4))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to resolve shift types")
	assert.Contains(t, err.Error(), "shift types unavailable")
}
