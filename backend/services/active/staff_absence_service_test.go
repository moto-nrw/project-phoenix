package active

import (
	"context"
	"errors"
	"testing"
	"time"

	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Mocks for StaffAbsenceRepository (prefixed with abs)
// ============================================================================

type absStaffAbsenceRepoMock struct {
	createFunc                 func(ctx context.Context, entity *activeModels.StaffAbsence) error
	findByIDFunc               func(ctx context.Context, id any) (*activeModels.StaffAbsence, error)
	updateFunc                 func(ctx context.Context, entity *activeModels.StaffAbsence) error
	deleteFunc                 func(ctx context.Context, id any) error
	listFunc                   func(ctx context.Context, options *base.QueryOptions) ([]*activeModels.StaffAbsence, error)
	getByStaffAndDateRangeFunc func(ctx context.Context, staffID int64, from, to time.Time) ([]*activeModels.StaffAbsence, error)
	getByStaffAndDateFunc      func(ctx context.Context, staffID int64, date time.Time) (*activeModels.StaffAbsence, error)
	getByDateRangeFunc         func(ctx context.Context, from, to time.Time) ([]*activeModels.StaffAbsence, error)
	getTodayAbsenceMapFunc     func(ctx context.Context) (map[int64]string, error)
}

func (m *absStaffAbsenceRepoMock) Create(ctx context.Context, entity *activeModels.StaffAbsence) error {
	if m.createFunc != nil {
		return m.createFunc(ctx, entity)
	}
	return nil
}

func (m *absStaffAbsenceRepoMock) FindByID(ctx context.Context, id any) (*activeModels.StaffAbsence, error) {
	if m.findByIDFunc != nil {
		return m.findByIDFunc(ctx, id)
	}
	return nil, errors.New("not found")
}

func (m *absStaffAbsenceRepoMock) Update(ctx context.Context, entity *activeModels.StaffAbsence) error {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, entity)
	}
	return nil
}

func (m *absStaffAbsenceRepoMock) Delete(ctx context.Context, id any) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, id)
	}
	return nil
}

func (m *absStaffAbsenceRepoMock) List(ctx context.Context, options *base.QueryOptions) ([]*activeModels.StaffAbsence, error) {
	if m.listFunc != nil {
		return m.listFunc(ctx, options)
	}
	return nil, nil
}

func (m *absStaffAbsenceRepoMock) GetByStaffAndDateRange(ctx context.Context, staffID int64, from, to time.Time) ([]*activeModels.StaffAbsence, error) {
	if m.getByStaffAndDateRangeFunc != nil {
		return m.getByStaffAndDateRangeFunc(ctx, staffID, from, to)
	}
	return nil, nil
}

func (m *absStaffAbsenceRepoMock) GetByStaffAndDate(ctx context.Context, staffID int64, date time.Time) (*activeModels.StaffAbsence, error) {
	if m.getByStaffAndDateFunc != nil {
		return m.getByStaffAndDateFunc(ctx, staffID, date)
	}
	return nil, nil
}

func (m *absStaffAbsenceRepoMock) GetByDateRange(ctx context.Context, from, to time.Time) ([]*activeModels.StaffAbsence, error) {
	if m.getByDateRangeFunc != nil {
		return m.getByDateRangeFunc(ctx, from, to)
	}
	return nil, nil
}

func (m *absStaffAbsenceRepoMock) GetTodayAbsenceMap(ctx context.Context) (map[int64]string, error) {
	if m.getTodayAbsenceMapFunc != nil {
		return m.getTodayAbsenceMapFunc(ctx)
	}
	return nil, nil
}

// ============================================================================
// Mock for WorkSessionRepository (prefixed with abs)
// ============================================================================

type absWorkSessionRepoMock struct {
	createFunc              func(ctx context.Context, entity *activeModels.WorkSession) error
	findByIDFunc            func(ctx context.Context, id any) (*activeModels.WorkSession, error)
	updateFunc              func(ctx context.Context, entity *activeModels.WorkSession) error
	deleteFunc              func(ctx context.Context, id any) error
	listFunc                func(ctx context.Context, options *base.QueryOptions) ([]*activeModels.WorkSession, error)
	getByStaffAndDateFunc   func(ctx context.Context, staffID int64, date time.Time) (*activeModels.WorkSession, error)
	getCurrentByStaffIDFunc func(ctx context.Context, staffID int64) (*activeModels.WorkSession, error)
	getHistoryByStaffIDFunc func(ctx context.Context, staffID int64, from, to time.Time) ([]*activeModels.WorkSession, error)
	getOpenSessionsFunc     func(ctx context.Context, beforeDate time.Time) ([]*activeModels.WorkSession, error)
	getTodayPresenceMapFunc func(ctx context.Context) (map[int64]string, error)
	closeSessionFunc        func(ctx context.Context, id int64, checkOutTime time.Time, autoCheckedOut bool) error
	updateBreakMinutesFunc  func(ctx context.Context, id int64, breakMinutes int) error
}

func (m *absWorkSessionRepoMock) Create(ctx context.Context, entity *activeModels.WorkSession) error {
	if m.createFunc != nil {
		return m.createFunc(ctx, entity)
	}
	return nil
}

func (m *absWorkSessionRepoMock) FindByID(ctx context.Context, id any) (*activeModels.WorkSession, error) {
	if m.findByIDFunc != nil {
		return m.findByIDFunc(ctx, id)
	}
	return nil, errors.New("not found")
}

func (m *absWorkSessionRepoMock) Update(ctx context.Context, entity *activeModels.WorkSession) error {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, entity)
	}
	return nil
}

func (m *absWorkSessionRepoMock) Delete(ctx context.Context, id any) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, id)
	}
	return nil
}

func (m *absWorkSessionRepoMock) List(ctx context.Context, options *base.QueryOptions) ([]*activeModels.WorkSession, error) {
	if m.listFunc != nil {
		return m.listFunc(ctx, options)
	}
	return nil, nil
}

func (m *absWorkSessionRepoMock) GetByStaffAndDate(ctx context.Context, staffID int64, date time.Time) (*activeModels.WorkSession, error) {
	if m.getByStaffAndDateFunc != nil {
		return m.getByStaffAndDateFunc(ctx, staffID, date)
	}
	return nil, nil
}

func (m *absWorkSessionRepoMock) GetCurrentByStaffID(ctx context.Context, staffID int64) (*activeModels.WorkSession, error) {
	if m.getCurrentByStaffIDFunc != nil {
		return m.getCurrentByStaffIDFunc(ctx, staffID)
	}
	return nil, nil
}

func (m *absWorkSessionRepoMock) GetHistoryByStaffID(ctx context.Context, staffID int64, from, to time.Time) ([]*activeModels.WorkSession, error) {
	if m.getHistoryByStaffIDFunc != nil {
		return m.getHistoryByStaffIDFunc(ctx, staffID, from, to)
	}
	return nil, nil
}

func (m *absWorkSessionRepoMock) GetOpenSessions(ctx context.Context, beforeDate time.Time) ([]*activeModels.WorkSession, error) {
	if m.getOpenSessionsFunc != nil {
		return m.getOpenSessionsFunc(ctx, beforeDate)
	}
	return nil, nil
}

func (m *absWorkSessionRepoMock) GetTodayPresenceMap(ctx context.Context) (map[int64]string, error) {
	if m.getTodayPresenceMapFunc != nil {
		return m.getTodayPresenceMapFunc(ctx)
	}
	return nil, nil
}

func (m *absWorkSessionRepoMock) CloseSession(ctx context.Context, id int64, checkOutTime time.Time, autoCheckedOut bool) error {
	if m.closeSessionFunc != nil {
		return m.closeSessionFunc(ctx, id, checkOutTime, autoCheckedOut)
	}
	return nil
}

func (m *absWorkSessionRepoMock) UpdateBreakMinutes(ctx context.Context, id int64, breakMinutes int) error {
	if m.updateBreakMinutesFunc != nil {
		return m.updateBreakMinutesFunc(ctx, id, breakMinutes)
	}
	return nil
}

// ============================================================================
// Helper Functions
// ============================================================================

func absSetupService() (*staffAbsenceService, *absStaffAbsenceRepoMock, *absWorkSessionRepoMock) {
	absRepo := &absStaffAbsenceRepoMock{}
	workRepo := &absWorkSessionRepoMock{}
	svc := &staffAbsenceService{
		absenceRepo:     absRepo,
		workSessionRepo: workRepo,
	}
	return svc, absRepo, workRepo
}

// ============================================================================
// CreateAbsence Tests
// ============================================================================

func TestAbsCreateAbsence_Success(t *testing.T) {
	svc, absRepo, workRepo := absSetupService()
	ctx := context.Background()
	staffID := int64(100)

	absRepo.getByStaffAndDateRangeFunc = func(_ context.Context, _ int64, _, _ time.Time) ([]*activeModels.StaffAbsence, error) {
		return nil, nil
	}

	workRepo.getHistoryByStaffIDFunc = func(_ context.Context, _ int64, _, _ time.Time) ([]*activeModels.WorkSession, error) {
		return nil, nil
	}

	absRepo.createFunc = func(_ context.Context, entity *activeModels.StaffAbsence) error {
		assert.Equal(t, staffID, entity.StaffID)
		assert.Equal(t, activeModels.AbsenceTypeSick, entity.AbsenceType)
		assert.Equal(t, activeModels.AbsenceStatusReported, entity.Status)
		entity.ID = 100
		return nil
	}

	req := CreateAbsenceRequest{
		AbsenceType: activeModels.AbsenceTypeSick,
		DateStart:   "2026-02-10",
		DateEnd:     "2026-02-12",
		Note:        "Flu",
	}

	result, err := svc.CreateAbsence(ctx, staffID, req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, int64(100), result.ID)
	assert.Equal(t, 3, result.DurationDays) // 10, 11, 12 = 3 days
}

func TestAbsCreateAbsence_InvalidDateStart(t *testing.T) {
	svc, _, _ := absSetupService()

	req := CreateAbsenceRequest{
		AbsenceType: activeModels.AbsenceTypeSick,
		DateStart:   "not-a-date",
		DateEnd:     "2026-02-12",
	}

	result, err := svc.CreateAbsence(context.Background(), 1, req)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "invalid date_start format")
}

func TestAbsCreateAbsence_InvalidDateEnd(t *testing.T) {
	svc, _, _ := absSetupService()

	req := CreateAbsenceRequest{
		AbsenceType: activeModels.AbsenceTypeSick,
		DateStart:   "2026-02-10",
		DateEnd:     "invalid",
	}

	result, err := svc.CreateAbsence(context.Background(), 1, req)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "invalid date_end format")
}

func TestAbsCreateAbsence_OverlapDifferentType(t *testing.T) {
	svc, absRepo, _ := absSetupService()
	ctx := context.Background()

	absRepo.getByStaffAndDateRangeFunc = func(_ context.Context, _ int64, _, _ time.Time) ([]*activeModels.StaffAbsence, error) {
		return []*activeModels.StaffAbsence{
			{
				Model:       base.Model{ID: 50},
				AbsenceType: activeModels.AbsenceTypeVacation,
				DateStart:   time.Date(2026, 2, 10, 0, 0, 0, 0, time.UTC),
				DateEnd:     time.Date(2026, 2, 12, 0, 0, 0, 0, time.UTC),
			},
		}, nil
	}

	req := CreateAbsenceRequest{
		AbsenceType: activeModels.AbsenceTypeSick,
		DateStart:   "2026-02-11",
		DateEnd:     "2026-02-13",
	}

	result, err := svc.CreateAbsence(ctx, 1, req)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "overlaps with existing")
}

func TestAbsCreateAbsence_MergeSameType(t *testing.T) {
	svc, absRepo, _ := absSetupService()
	ctx := context.Background()
	staffID := int64(100)

	existing := &activeModels.StaffAbsence{
		Model:       base.Model{ID: 60},
		StaffID:     staffID,
		AbsenceType: activeModels.AbsenceTypeSick,
		DateStart:   time.Date(2026, 2, 8, 0, 0, 0, 0, time.UTC),
		DateEnd:     time.Date(2026, 2, 10, 0, 0, 0, 0, time.UTC),
		Status:      activeModels.AbsenceStatusReported,
	}

	absRepo.getByStaffAndDateRangeFunc = func(_ context.Context, _ int64, _, _ time.Time) ([]*activeModels.StaffAbsence, error) {
		return []*activeModels.StaffAbsence{existing}, nil
	}

	var updatedAbsence *activeModels.StaffAbsence
	absRepo.updateFunc = func(_ context.Context, entity *activeModels.StaffAbsence) error {
		updatedAbsence = entity
		return nil
	}

	req := CreateAbsenceRequest{
		AbsenceType: activeModels.AbsenceTypeSick,
		DateStart:   "2026-02-10",
		DateEnd:     "2026-02-12",
	}

	result, err := svc.CreateAbsence(ctx, staffID, req)
	require.NoError(t, err)
	require.NotNil(t, result)
	// Merged range: 2026-02-08 to 2026-02-12
	assert.Equal(t, "2026-02-08", updatedAbsence.DateStart.Format("2006-01-02"))
	assert.Equal(t, "2026-02-12", updatedAbsence.DateEnd.Format("2006-01-02"))
}

func TestAbsCreateAbsence_MergeMultipleSameType(t *testing.T) {
	svc, absRepo, _ := absSetupService()
	ctx := context.Background()
	staffID := int64(100)

	absRepo.getByStaffAndDateRangeFunc = func(_ context.Context, _ int64, _, _ time.Time) ([]*activeModels.StaffAbsence, error) {
		return []*activeModels.StaffAbsence{
			{
				Model:       base.Model{ID: 60},
				StaffID:     staffID,
				AbsenceType: activeModels.AbsenceTypeSick,
				DateStart:   time.Date(2026, 2, 5, 0, 0, 0, 0, time.UTC),
				DateEnd:     time.Date(2026, 2, 8, 0, 0, 0, 0, time.UTC),
			},
			{
				Model:       base.Model{ID: 61},
				StaffID:     staffID,
				AbsenceType: activeModels.AbsenceTypeSick,
				DateStart:   time.Date(2026, 2, 12, 0, 0, 0, 0, time.UTC),
				DateEnd:     time.Date(2026, 2, 14, 0, 0, 0, 0, time.UTC),
			},
		}, nil
	}

	absRepo.updateFunc = func(_ context.Context, entity *activeModels.StaffAbsence) error {
		return nil
	}

	var deletedIDs []int64
	absRepo.deleteFunc = func(_ context.Context, id any) error {
		deletedIDs = append(deletedIDs, id.(int64))
		return nil
	}

	req := CreateAbsenceRequest{
		AbsenceType: activeModels.AbsenceTypeSick,
		DateStart:   "2026-02-07",
		DateEnd:     "2026-02-13",
	}

	result, err := svc.CreateAbsence(ctx, staffID, req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Contains(t, deletedIDs, int64(61))
}

func TestAbsCreateAbsence_RepoCheckError(t *testing.T) {
	svc, absRepo, _ := absSetupService()

	absRepo.getByStaffAndDateRangeFunc = func(_ context.Context, _ int64, _, _ time.Time) ([]*activeModels.StaffAbsence, error) {
		return nil, errors.New("db error")
	}

	req := CreateAbsenceRequest{
		AbsenceType: activeModels.AbsenceTypeSick,
		DateStart:   "2026-02-10",
		DateEnd:     "2026-02-12",
	}

	result, err := svc.CreateAbsence(context.Background(), 1, req)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to check existing absences")
}

func TestAbsCreateAbsence_RepoCreateError(t *testing.T) {
	svc, absRepo, workRepo := absSetupService()

	absRepo.getByStaffAndDateRangeFunc = func(_ context.Context, _ int64, _, _ time.Time) ([]*activeModels.StaffAbsence, error) {
		return nil, nil
	}
	workRepo.getHistoryByStaffIDFunc = func(_ context.Context, _ int64, _, _ time.Time) ([]*activeModels.WorkSession, error) {
		return nil, nil
	}
	absRepo.createFunc = func(_ context.Context, _ *activeModels.StaffAbsence) error {
		return errors.New("insert failed")
	}

	req := CreateAbsenceRequest{
		AbsenceType: activeModels.AbsenceTypeSick,
		DateStart:   "2026-02-10",
		DateEnd:     "2026-02-12",
	}

	result, err := svc.CreateAbsence(context.Background(), 1, req)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to create absence")
}

// ============================================================================
// UpdateAbsence Tests
// ============================================================================

func TestAbsUpdateAbsence_Success(t *testing.T) {
	svc, absRepo, _ := absSetupService()
	ctx := context.Background()
	staffID := int64(100)
	absenceID := int64(100)

	existing := &activeModels.StaffAbsence{
		Model:       base.Model{ID: absenceID},
		StaffID:     staffID,
		AbsenceType: activeModels.AbsenceTypeSick,
		DateStart:   time.Date(2026, 2, 10, 0, 0, 0, 0, time.UTC),
		DateEnd:     time.Date(2026, 2, 12, 0, 0, 0, 0, time.UTC),
		HalfDay:     false,
		Note:        "Flu",
		Status:      activeModels.AbsenceStatusReported,
		CreatedBy:   staffID,
	}

	absRepo.findByIDFunc = func(_ context.Context, _ any) (*activeModels.StaffAbsence, error) {
		return existing, nil
	}

	absRepo.getByStaffAndDateRangeFunc = func(_ context.Context, _ int64, _, _ time.Time) ([]*activeModels.StaffAbsence, error) {
		return []*activeModels.StaffAbsence{existing}, nil // Only itself
	}

	absRepo.updateFunc = func(_ context.Context, entity *activeModels.StaffAbsence) error {
		assert.Equal(t, "Updated note", entity.Note)
		assert.Equal(t, activeModels.AbsenceTypeVacation, entity.AbsenceType)
		return nil
	}

	newType := activeModels.AbsenceTypeVacation
	newNote := "Updated note"
	req := UpdateAbsenceRequest{
		AbsenceType: &newType,
		Note:        &newNote,
	}

	result, err := svc.UpdateAbsence(ctx, staffID, absenceID, req)
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestAbsUpdateAbsence_NotFound(t *testing.T) {
	svc, absRepo, _ := absSetupService()

	absRepo.findByIDFunc = func(_ context.Context, _ any) (*activeModels.StaffAbsence, error) {
		return nil, errors.New("not found")
	}

	result, err := svc.UpdateAbsence(context.Background(), 1, 999, UpdateAbsenceRequest{})
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "absence not found")
}

func TestAbsUpdateAbsence_OwnershipFails(t *testing.T) {
	svc, absRepo, _ := absSetupService()

	absRepo.findByIDFunc = func(_ context.Context, _ any) (*activeModels.StaffAbsence, error) {
		return &activeModels.StaffAbsence{
			Model:   base.Model{ID: 100},
			StaffID: 999, // Different staff
		}, nil
	}

	result, err := svc.UpdateAbsence(context.Background(), 1, 100, UpdateAbsenceRequest{})
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "can only update own absences")
}

func TestAbsUpdateAbsence_InvalidDateStart(t *testing.T) {
	svc, absRepo, _ := absSetupService()
	staffID := int64(100)

	absRepo.findByIDFunc = func(_ context.Context, _ any) (*activeModels.StaffAbsence, error) {
		return &activeModels.StaffAbsence{
			Model:     base.Model{ID: 100},
			StaffID:   staffID,
			DateStart: time.Date(2026, 2, 10, 0, 0, 0, 0, time.UTC),
			DateEnd:   time.Date(2026, 2, 12, 0, 0, 0, 0, time.UTC),
		}, nil
	}

	bad := "not-a-date"
	req := UpdateAbsenceRequest{DateStart: &bad}

	result, err := svc.UpdateAbsence(context.Background(), staffID, 100, req)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "invalid date_start format")
}

func TestAbsUpdateAbsence_InvalidDateEnd(t *testing.T) {
	svc, absRepo, _ := absSetupService()
	staffID := int64(100)

	absRepo.findByIDFunc = func(_ context.Context, _ any) (*activeModels.StaffAbsence, error) {
		return &activeModels.StaffAbsence{
			Model:     base.Model{ID: 100},
			StaffID:   staffID,
			DateStart: time.Date(2026, 2, 10, 0, 0, 0, 0, time.UTC),
			DateEnd:   time.Date(2026, 2, 12, 0, 0, 0, 0, time.UTC),
		}, nil
	}

	bad := "not-a-date"
	req := UpdateAbsenceRequest{DateEnd: &bad}

	result, err := svc.UpdateAbsence(context.Background(), staffID, 100, req)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "invalid date_end format")
}

func TestAbsUpdateAbsence_OverlapAfterUpdate(t *testing.T) {
	svc, absRepo, _ := absSetupService()
	staffID := int64(100)
	absenceID := int64(100)

	existing := &activeModels.StaffAbsence{
		Model:       base.Model{ID: absenceID},
		StaffID:     staffID,
		AbsenceType: activeModels.AbsenceTypeSick,
		DateStart:   time.Date(2026, 2, 10, 0, 0, 0, 0, time.UTC),
		DateEnd:     time.Date(2026, 2, 12, 0, 0, 0, 0, time.UTC),
	}

	absRepo.findByIDFunc = func(_ context.Context, _ any) (*activeModels.StaffAbsence, error) {
		return existing, nil
	}

	absRepo.getByStaffAndDateRangeFunc = func(_ context.Context, _ int64, _, _ time.Time) ([]*activeModels.StaffAbsence, error) {
		return []*activeModels.StaffAbsence{
			existing,
			{
				Model:     base.Model{ID: 200},
				StaffID:   staffID,
				DateStart: time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC),
				DateEnd:   time.Date(2026, 2, 17, 0, 0, 0, 0, time.UTC),
			},
		}, nil
	}

	newEnd := "2026-02-16"
	req := UpdateAbsenceRequest{DateEnd: &newEnd}

	result, err := svc.UpdateAbsence(context.Background(), staffID, absenceID, req)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "overlap")
}

// ============================================================================
// DeleteAbsence Tests
// ============================================================================

func TestAbsDeleteAbsence_Success(t *testing.T) {
	svc, absRepo, _ := absSetupService()
	staffID := int64(100)
	absenceID := int64(100)

	absRepo.findByIDFunc = func(_ context.Context, _ any) (*activeModels.StaffAbsence, error) {
		return &activeModels.StaffAbsence{
			Model:   base.Model{ID: absenceID},
			StaffID: staffID,
		}, nil
	}

	var deletedID int64
	absRepo.deleteFunc = func(_ context.Context, id any) error {
		deletedID = id.(int64)
		return nil
	}

	err := svc.DeleteAbsence(context.Background(), staffID, absenceID)
	require.NoError(t, err)
	assert.Equal(t, absenceID, deletedID)
}

func TestAbsDeleteAbsence_NotFound(t *testing.T) {
	svc, absRepo, _ := absSetupService()

	absRepo.findByIDFunc = func(_ context.Context, _ any) (*activeModels.StaffAbsence, error) {
		return nil, errors.New("not found")
	}

	err := svc.DeleteAbsence(context.Background(), 1, 999)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "absence not found")
}

func TestAbsDeleteAbsence_OwnershipFails(t *testing.T) {
	svc, absRepo, _ := absSetupService()

	absRepo.findByIDFunc = func(_ context.Context, _ any) (*activeModels.StaffAbsence, error) {
		return &activeModels.StaffAbsence{
			Model:   base.Model{ID: 100},
			StaffID: 999,
		}, nil
	}

	err := svc.DeleteAbsence(context.Background(), 1, 100)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "can only delete own absences")
}

func TestAbsDeleteAbsence_RepoError(t *testing.T) {
	svc, absRepo, _ := absSetupService()
	staffID := int64(100)

	absRepo.findByIDFunc = func(_ context.Context, _ any) (*activeModels.StaffAbsence, error) {
		return &activeModels.StaffAbsence{
			Model:   base.Model{ID: 100},
			StaffID: staffID,
		}, nil
	}

	absRepo.deleteFunc = func(_ context.Context, _ any) error {
		return errors.New("database error")
	}

	err := svc.DeleteAbsence(context.Background(), staffID, 100)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete absence")
}

// ============================================================================
// GetAbsencesForRange Tests
// ============================================================================

func TestAbsGetAbsencesForRange_Success(t *testing.T) {
	svc, absRepo, _ := absSetupService()
	ctx := context.Background()
	staffID := int64(100)
	from := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 2, 28, 0, 0, 0, 0, time.UTC)

	absRepo.getByStaffAndDateRangeFunc = func(_ context.Context, _ int64, _, _ time.Time) ([]*activeModels.StaffAbsence, error) {
		return []*activeModels.StaffAbsence{
			{
				Model:     base.Model{ID: 1},
				DateStart: time.Date(2026, 2, 10, 0, 0, 0, 0, time.UTC),
				DateEnd:   time.Date(2026, 2, 12, 0, 0, 0, 0, time.UTC),
			},
			{
				Model:     base.Model{ID: 2},
				DateStart: time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC),
				DateEnd:   time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC),
			},
		}, nil
	}

	results, err := svc.GetAbsencesForRange(ctx, staffID, from, to)
	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.Equal(t, 3, results[0].DurationDays)
	assert.Equal(t, 1, results[1].DurationDays)
}

func TestAbsGetAbsencesForRange_Empty(t *testing.T) {
	svc, absRepo, _ := absSetupService()

	absRepo.getByStaffAndDateRangeFunc = func(_ context.Context, _ int64, _, _ time.Time) ([]*activeModels.StaffAbsence, error) {
		return nil, nil
	}

	from := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 2, 28, 0, 0, 0, 0, time.UTC)

	results, err := svc.GetAbsencesForRange(context.Background(), 1, from, to)
	require.NoError(t, err)
	assert.Len(t, results, 0)
}

func TestAbsGetAbsencesForRange_RepoError(t *testing.T) {
	svc, absRepo, _ := absSetupService()

	absRepo.getByStaffAndDateRangeFunc = func(_ context.Context, _ int64, _, _ time.Time) ([]*activeModels.StaffAbsence, error) {
		return nil, errors.New("database error")
	}

	from := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 2, 28, 0, 0, 0, 0, time.UTC)

	results, err := svc.GetAbsencesForRange(context.Background(), 1, from, to)
	require.Error(t, err)
	assert.Nil(t, results)
	assert.Contains(t, err.Error(), "failed to get absences")
}

// ============================================================================
// HasAbsenceOnDate Tests
// ============================================================================

func TestAbsHasAbsenceOnDate_Found(t *testing.T) {
	svc, absRepo, _ := absSetupService()
	staffID := int64(100)
	date := time.Date(2026, 2, 11, 0, 0, 0, 0, time.UTC)

	absRepo.getByStaffAndDateFunc = func(_ context.Context, _ int64, _ time.Time) (*activeModels.StaffAbsence, error) {
		return &activeModels.StaffAbsence{
			Model:       base.Model{ID: 100},
			StaffID:     staffID,
			AbsenceType: activeModels.AbsenceTypeSick,
		}, nil
	}

	has, absence, err := svc.HasAbsenceOnDate(context.Background(), staffID, date)
	require.NoError(t, err)
	assert.True(t, has)
	require.NotNil(t, absence)
	assert.Equal(t, int64(100), absence.ID)
}

func TestAbsHasAbsenceOnDate_NotFound(t *testing.T) {
	svc, absRepo, _ := absSetupService()

	absRepo.getByStaffAndDateFunc = func(_ context.Context, _ int64, _ time.Time) (*activeModels.StaffAbsence, error) {
		return nil, nil
	}

	has, absence, err := svc.HasAbsenceOnDate(context.Background(), 1, time.Now())
	require.NoError(t, err)
	assert.False(t, has)
	assert.Nil(t, absence)
}

func TestAbsHasAbsenceOnDate_RepoError(t *testing.T) {
	svc, absRepo, _ := absSetupService()

	absRepo.getByStaffAndDateFunc = func(_ context.Context, _ int64, _ time.Time) (*activeModels.StaffAbsence, error) {
		return nil, errors.New("database error")
	}

	has, absence, err := svc.HasAbsenceOnDate(context.Background(), 1, time.Now())
	require.Error(t, err)
	assert.False(t, has)
	assert.Nil(t, absence)
	assert.Contains(t, err.Error(), "failed to check absence")
}
