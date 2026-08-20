package active

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/email"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Mocks for StaffAbsenceRepository (prefixed with abs)
// ============================================================================

type absStaffAbsenceRepoMock struct {
	lockStaffAbsenceWritesFunc func(ctx context.Context, staffID int64) error
	createFunc                 func(ctx context.Context, entity *activeModels.StaffAbsence) error
	findByIDFunc               func(ctx context.Context, id any) (*activeModels.StaffAbsence, error)
	updateFunc                 func(ctx context.Context, entity *activeModels.StaffAbsence) error
	deleteFunc                 func(ctx context.Context, id any) error
	listFunc                   func(ctx context.Context, options *base.QueryOptions) ([]*activeModels.StaffAbsence, error)
	getByStaffAndDateRangeFunc func(ctx context.Context, staffID int64, from, to timezone.Date) ([]*activeModels.StaffAbsence, error)
	getByStaffAndDateFunc      func(ctx context.Context, staffID int64, date timezone.Date) (*activeModels.StaffAbsence, error)
	getByDateRangeFunc         func(ctx context.Context, from, to timezone.Date) ([]*activeModels.StaffAbsence, error)
	getAbsenceMapForDateFunc   func(ctx context.Context, date timezone.Date) (map[int64]string, error)
	listRequestsFunc           func(ctx context.Context, filter activeModels.AbsenceRequestFilter) ([]*activeModels.AbsenceRequestRow, error)
	listByStatusesFunc         func(ctx context.Context, statuses []string) ([]*activeModels.StaffAbsence, error)
}

func (m *absStaffAbsenceRepoMock) LockStaffAbsenceWrites(ctx context.Context, staffID int64) error {
	if m.lockStaffAbsenceWritesFunc != nil {
		return m.lockStaffAbsenceWritesFunc(ctx, staffID)
	}
	return nil
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

// GetByStaffIDsAndDateRange satisfies the batched interface method (#1417);
// these tests exercise the single-staff path only.
func (m *absStaffAbsenceRepoMock) GetByStaffIDsAndDateRange(context.Context, []int64, timezone.Date, timezone.Date) (map[int64][]*activeModels.StaffAbsence, error) {
	return nil, nil
}

func (m *absStaffAbsenceRepoMock) GetByStaffAndDateRange(ctx context.Context, staffID int64, from, to timezone.Date) ([]*activeModels.StaffAbsence, error) {
	if m.getByStaffAndDateRangeFunc != nil {
		return m.getByStaffAndDateRangeFunc(ctx, staffID, from, to)
	}
	return nil, nil
}

func (m *absStaffAbsenceRepoMock) GetByStaffAndDate(ctx context.Context, staffID int64, date timezone.Date) (*activeModels.StaffAbsence, error) {
	if m.getByStaffAndDateFunc != nil {
		return m.getByStaffAndDateFunc(ctx, staffID, date)
	}
	return nil, nil
}

func (m *absStaffAbsenceRepoMock) GetByDateRange(ctx context.Context, from, to timezone.Date) ([]*activeModels.StaffAbsence, error) {
	if m.getByDateRangeFunc != nil {
		return m.getByDateRangeFunc(ctx, from, to)
	}
	return nil, nil
}

func (m *absStaffAbsenceRepoMock) GetAbsenceMapForDate(ctx context.Context, date timezone.Date) (map[int64]string, error) {
	if m.getAbsenceMapForDateFunc != nil {
		return m.getAbsenceMapForDateFunc(ctx, date)
	}
	return nil, nil
}

// ListByStaffAndStatuses + ListByStatuses are part of the StaffAbsenceRepository
// interface added in the Tranche 4 vacation-workflow spike. No-op defaults so
// tests that don't exercise the vacation inbox still satisfy the interface.
func (m *absStaffAbsenceRepoMock) ListByStaffAndStatuses(_ context.Context, _ int64, _ []string) ([]*activeModels.StaffAbsence, error) {
	return nil, nil
}

func (m *absStaffAbsenceRepoMock) ListRequests(ctx context.Context, filter activeModels.AbsenceRequestFilter) ([]*activeModels.AbsenceRequestRow, error) {
	if m.listRequestsFunc != nil {
		return m.listRequestsFunc(ctx, filter)
	}
	return nil, nil
}

func (m *absStaffAbsenceRepoMock) ListByStatuses(ctx context.Context, statuses []string) ([]*activeModels.StaffAbsence, error) {
	if m.listByStatusesFunc != nil {
		return m.listByStatusesFunc(ctx, statuses)
	}
	return nil, nil
}

type absVacationQuotaRepoMock struct {
	getByStaffAndYearFunc func(ctx context.Context, staffID int64, year int) (*activeModels.StaffVacationQuota, error)
	upsertFunc            func(ctx context.Context, quota *activeModels.StaffVacationQuota) error
}

func (m *absVacationQuotaRepoMock) Create(context.Context, *activeModels.StaffVacationQuota) error {
	return nil
}

func (m *absVacationQuotaRepoMock) FindByID(context.Context, any) (*activeModels.StaffVacationQuota, error) {
	return nil, errors.New("not found")
}

func (m *absVacationQuotaRepoMock) Update(context.Context, *activeModels.StaffVacationQuota) error {
	return nil
}

func (m *absVacationQuotaRepoMock) Delete(context.Context, any) error {
	return nil
}

func (m *absVacationQuotaRepoMock) List(context.Context, *base.QueryOptions) ([]*activeModels.StaffVacationQuota, error) {
	return nil, nil
}

// GetByStaffIDsAndYear satisfies the batched interface method (#1417).
func (m *absVacationQuotaRepoMock) GetByStaffIDsAndYear(context.Context, []int64, int) (map[int64]*activeModels.StaffVacationQuota, error) {
	return nil, nil
}

func (m *absVacationQuotaRepoMock) GetByStaffAndYear(ctx context.Context, staffID int64, year int) (*activeModels.StaffVacationQuota, error) {
	if m.getByStaffAndYearFunc != nil {
		return m.getByStaffAndYearFunc(ctx, staffID, year)
	}
	return nil, nil
}

func (m *absVacationQuotaRepoMock) Upsert(ctx context.Context, quota *activeModels.StaffVacationQuota) error {
	if m.upsertFunc != nil {
		return m.upsertFunc(ctx, quota)
	}
	return nil
}

type absStaffAbsenceAuditRepoMock struct {
	createFunc func(ctx context.Context, audit *activeModels.StaffAbsenceAudit) error
}

func (m *absStaffAbsenceAuditRepoMock) Create(ctx context.Context, audit *activeModels.StaffAbsenceAudit) error {
	if m.createFunc != nil {
		return m.createFunc(ctx, audit)
	}
	return nil
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
	getByStaffAndDateFunc   func(ctx context.Context, staffID int64, date timezone.Date) (*activeModels.WorkSession, error)
	getCurrentByStaffIDFunc func(ctx context.Context, staffID int64) (*activeModels.WorkSession, error)
	getHistoryByStaffIDFunc func(ctx context.Context, staffID int64, from, to timezone.Date) ([]*activeModels.WorkSession, error)
	getOpenSessionsFunc     func(ctx context.Context, beforeDate timezone.Date) ([]*activeModels.WorkSession, error)
	getTodayPresenceMapFunc func(ctx context.Context) (map[int64]string, error)
	closeSessionFunc        func(ctx context.Context, id int64, checkOutTime time.Time, autoCheckedOut bool) (bool, error)
	updateBreakMinutesFunc  func(ctx context.Context, id int64, breakMinutes int) error
}

func (m *absWorkSessionRepoMock) LockStaffBalanceWrites(context.Context, int64) error {
	return nil
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

func (m *absWorkSessionRepoMock) ListByStaffAndDate(ctx context.Context, staffID int64, date timezone.Date) ([]*activeModels.WorkSession, error) {
	if m.getByStaffAndDateFunc != nil {
		session, err := m.getByStaffAndDateFunc(ctx, staffID, date)
		if err != nil || session == nil {
			return nil, err
		}
		return []*activeModels.WorkSession{session}, nil
	}
	return nil, nil
}

func (m *absWorkSessionRepoMock) GetCurrentByStaffID(ctx context.Context, staffID int64) (*activeModels.WorkSession, error) {
	if m.getCurrentByStaffIDFunc != nil {
		return m.getCurrentByStaffIDFunc(ctx, staffID)
	}
	return nil, nil
}

func (m *absWorkSessionRepoMock) GetOpenByStaffAndDate(ctx context.Context, staffID int64, _ timezone.Date) (*activeModels.WorkSession, error) {
	return m.GetCurrentByStaffID(ctx, staffID)
}

func (m *absWorkSessionRepoMock) GetLatestOpenByStaffID(ctx context.Context, staffID int64) (*activeModels.WorkSession, error) {
	return m.GetCurrentByStaffID(ctx, staffID)
}

func (m *absWorkSessionRepoMock) GetOpenByStaffAndDateForUpdate(ctx context.Context, staffID int64, _ timezone.Date) (*activeModels.WorkSession, error) {
	return m.GetCurrentByStaffID(ctx, staffID)
}

func (m *absWorkSessionRepoMock) LockOpenByIDForUpdate(ctx context.Context, id int64) (*activeModels.WorkSession, error) {
	session, err := m.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if session.CheckOutTime != nil {
		return nil, errors.New("not found")
	}
	return session, nil
}

// GetHistoryByStaffIDs satisfies the batched interface method (#1417).
func (m *absWorkSessionRepoMock) GetHistoryByStaffIDs(context.Context, []int64, timezone.Date, timezone.Date) (map[int64][]*activeModels.WorkSession, error) {
	return nil, nil
}

func (m *absWorkSessionRepoMock) GetHistoryByStaffID(ctx context.Context, staffID int64, from, to timezone.Date) ([]*activeModels.WorkSession, error) {
	if m.getHistoryByStaffIDFunc != nil {
		return m.getHistoryByStaffIDFunc(ctx, staffID, from, to)
	}
	return nil, nil
}

// ListOverlappingByStaffID satisfies the interface; the absence service never
// creates work blocks, so nothing here overlaps anything.
func (m *absWorkSessionRepoMock) ListOverlappingByStaffID(context.Context, int64, time.Time, *time.Time) ([]*activeModels.WorkSession, error) {
	return nil, nil
}

func (m *absWorkSessionRepoMock) GetOpenSessions(ctx context.Context, beforeDate timezone.Date) ([]*activeModels.WorkSession, error) {
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

func (m *absWorkSessionRepoMock) CloseSession(ctx context.Context, id int64, checkOutTime time.Time, autoCheckedOut bool) (bool, error) {
	if m.closeSessionFunc != nil {
		return m.closeSessionFunc(ctx, id, checkOutTime, autoCheckedOut)
	}
	return true, nil
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
	quotaRepo := &absVacationQuotaRepoMock{}
	auditRepo := &absStaffAbsenceAuditRepoMock{}
	svc := &staffAbsenceService{
		absenceRepo:     absRepo,
		workSessionRepo: workRepo,
		quotaRepo:       quotaRepo,
		auditRepo:       auditRepo,
		// Deletes require the tombstone writer (#1417); assertions stay on
		// the absence repo.
		deletionRepo: noopDeletionAuditRepo{},
	}
	return svc, absRepo, workRepo
}

func TestValidateVacationOpeningAbsencesBefore_DoesNotLockPreview(t *testing.T) {
	svc, absRepo, _ := absSetupService()
	lockCalls := 0
	absRepo.lockStaffAbsenceWritesFunc = func(context.Context, int64) error {
		lockCalls++
		return nil
	}

	err := svc.ValidateVacationOpeningAbsencesBefore(
		context.Background(),
		42,
		timezone.NewDate(2026, time.March, 1),
	)

	require.NoError(t, err)
	assert.Zero(t, lockCalls)
}

// ============================================================================
// CreateAbsence Tests
// ============================================================================

func TestAbsCreateAbsence_Success(t *testing.T) {
	svc, absRepo, workRepo := absSetupService()
	ctx := context.Background()
	staffID := int64(100)

	absRepo.getByStaffAndDateRangeFunc = func(_ context.Context, _ int64, _, _ timezone.Date) ([]*activeModels.StaffAbsence, error) {
		return nil, nil
	}

	workRepo.getHistoryByStaffIDFunc = func(_ context.Context, _ int64, _, _ timezone.Date) ([]*activeModels.WorkSession, error) {
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

func TestAbsCreateAbsence_BlocksVacationType(t *testing.T) {
	svc, absRepo, _ := absSetupService()
	absRepo.createFunc = func(_ context.Context, _ *activeModels.StaffAbsence) error {
		t.Fatal("vacation must not be created through generic absence create")
		return nil
	}

	result, err := svc.CreateAbsence(context.Background(), int64(100), CreateAbsenceRequest{
		AbsenceType: activeModels.AbsenceTypeVacation,
		DateStart:   "2026-02-10",
		DateEnd:     "2026-02-12",
	})

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "vacation flow")
}

func TestAbsCompTimeRequiresManagerControl(t *testing.T) {
	const (
		staffID   = int64(100)
		managerID = int64(200)
	)
	req := CreateAbsenceRequest{
		AbsenceType: activeModels.AbsenceTypeCompTime,
		DateStart:   "2026-07-20",
		DateEnd:     "2026-07-20",
		HalfDay:     true,
		Note:        "Freizeitausgleich",
	}

	t.Run("own create rejected", func(t *testing.T) {
		svc, absRepo, _ := absSetupService()
		absRepo.createFunc = func(context.Context, *activeModels.StaffAbsence) error {
			t.Fatal("self-service comp time must not be persisted")
			return nil
		}

		result, err := svc.CreateOwnAbsence(context.Background(), staffID, nil, req)

		require.ErrorIs(t, err, ErrManagerControlledAbsence)
		assert.Nil(t, result)
	})

	t.Run("manager create allowed", func(t *testing.T) {
		svc, absRepo, _ := absSetupService()
		absRepo.getByStaffAndDateRangeFunc = func(context.Context, int64, timezone.Date, timezone.Date) ([]*activeModels.StaffAbsence, error) {
			return nil, nil
		}
		absRepo.createFunc = func(_ context.Context, absence *activeModels.StaffAbsence) error {
			assert.Equal(t, activeModels.AbsenceTypeCompTime, absence.AbsenceType)
			assert.Equal(t, activeModels.AbsenceStatusReported, absence.Status)
			assert.Equal(t, managerID, absence.CreatedBy)
			assert.True(t, absence.HalfDay)
			absence.ID = 77
			return nil
		}

		result, err := svc.CreateAbsenceFor(context.Background(), staffID, managerID, nil, req)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, int64(77), result.ID)
	})

	t.Run("own update of manager-created comp time rejected", func(t *testing.T) {
		svc, absRepo, _ := absSetupService()
		absRepo.findByIDFunc = func(context.Context, any) (*activeModels.StaffAbsence, error) {
			return &activeModels.StaffAbsence{
				StaffID:     staffID,
				AbsenceType: activeModels.AbsenceTypeCompTime,
			}, nil
		}
		absRepo.updateFunc = func(context.Context, *activeModels.StaffAbsence) error {
			t.Fatal("self-service comp time must not be updated")
			return nil
		}
		note := "Geändert"

		result, err := svc.UpdateAbsence(context.Background(), staffID, nil, 77, UpdateAbsenceRequest{Note: &note})

		require.ErrorIs(t, err, ErrManagerControlledAbsence)
		assert.Nil(t, result)
	})

	t.Run("own conversion into comp time rejected", func(t *testing.T) {
		svc, absRepo, _ := absSetupService()
		absRepo.findByIDFunc = func(context.Context, any) (*activeModels.StaffAbsence, error) {
			return &activeModels.StaffAbsence{
				StaffID:     staffID,
				AbsenceType: activeModels.AbsenceTypeOther,
			}, nil
		}
		compTime := activeModels.AbsenceTypeCompTime

		result, err := svc.UpdateAbsence(context.Background(), staffID, nil, 77, UpdateAbsenceRequest{AbsenceType: &compTime})

		require.ErrorIs(t, err, ErrManagerControlledAbsence)
		assert.Nil(t, result)
	})

	t.Run("own delete rejected and manager delete allowed", func(t *testing.T) {
		svc, absRepo, _ := absSetupService()
		absence := &activeModels.StaffAbsence{
			StaffID:     staffID,
			AbsenceType: activeModels.AbsenceTypeCompTime,
		}
		absence.ID = 77
		absRepo.findByIDFunc = func(context.Context, any) (*activeModels.StaffAbsence, error) {
			return absence, nil
		}
		deleteCalls := 0
		absRepo.deleteFunc = func(context.Context, any) error {
			deleteCalls++
			return nil
		}

		err := svc.DeleteOwnAbsence(context.Background(), staffID, nil, absence.ID)
		require.ErrorIs(t, err, ErrManagerControlledAbsence)
		assert.Zero(t, deleteCalls)

		err = svc.DeleteAbsenceFor(context.Background(), staffID, managerID, nil, absence.ID)
		require.NoError(t, err)
		assert.Equal(t, 1, deleteCalls)
	})
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

func TestAbsCreateAbsence_ReversedDateRange(t *testing.T) {
	svc, _, _ := absSetupService()

	req := CreateAbsenceRequest{
		AbsenceType: activeModels.AbsenceTypeCompTime,
		DateStart:   "2026-02-12",
		DateEnd:     "2026-02-10",
	}

	result, err := svc.CreateAbsenceFor(context.Background(), 1, 2, nil, req)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "invalid date range")
}

func TestAbsCreateAbsence_OverlapDifferentType(t *testing.T) {
	svc, absRepo, _ := absSetupService()
	ctx := context.Background()

	absRepo.getByStaffAndDateRangeFunc = func(_ context.Context, _ int64, _, _ timezone.Date) ([]*activeModels.StaffAbsence, error) {
		return []*activeModels.StaffAbsence{
			{
				Model:       base.Model{ID: 50},
				AbsenceType: activeModels.AbsenceTypeVacation,
				DateStart:   timezone.NewDate(2026, 2, 10),
				DateEnd:     timezone.NewDate(2026, 2, 12),
				Status:      activeModels.AbsenceStatusApproved,
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
		DateStart:   timezone.NewDate(2026, 2, 8),
		DateEnd:     timezone.NewDate(2026, 2, 10),
		Status:      activeModels.AbsenceStatusReported,
	}

	absRepo.getByStaffAndDateRangeFunc = func(_ context.Context, _ int64, _, _ timezone.Date) ([]*activeModels.StaffAbsence, error) {
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

	absRepo.getByStaffAndDateRangeFunc = func(_ context.Context, _ int64, _, _ timezone.Date) ([]*activeModels.StaffAbsence, error) {
		return []*activeModels.StaffAbsence{
			{
				Model:       base.Model{ID: 60},
				StaffID:     staffID,
				AbsenceType: activeModels.AbsenceTypeSick,
				DateStart:   timezone.NewDate(2026, 2, 5),
				DateEnd:     timezone.NewDate(2026, 2, 8),
				Status:      activeModels.AbsenceStatusReported,
			},
			{
				Model:       base.Model{ID: 61},
				StaffID:     staffID,
				AbsenceType: activeModels.AbsenceTypeSick,
				DateStart:   timezone.NewDate(2026, 2, 12),
				DateEnd:     timezone.NewDate(2026, 2, 14),
				Status:      activeModels.AbsenceStatusReported,
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

	absRepo.getByStaffAndDateRangeFunc = func(_ context.Context, _ int64, _, _ timezone.Date) ([]*activeModels.StaffAbsence, error) {
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

	absRepo.getByStaffAndDateRangeFunc = func(_ context.Context, _ int64, _, _ timezone.Date) ([]*activeModels.StaffAbsence, error) {
		return nil, nil
	}
	workRepo.getHistoryByStaffIDFunc = func(_ context.Context, _ int64, _, _ timezone.Date) ([]*activeModels.WorkSession, error) {
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
		DateStart:   timezone.NewDate(2026, 2, 10),
		DateEnd:     timezone.NewDate(2026, 2, 12),
		HalfDay:     false,
		Note:        "Flu",
		Status:      activeModels.AbsenceStatusReported,
		CreatedBy:   staffID,
	}

	absRepo.findByIDFunc = func(_ context.Context, _ any) (*activeModels.StaffAbsence, error) {
		return existing, nil
	}

	absRepo.getByStaffAndDateRangeFunc = func(_ context.Context, _ int64, _, _ timezone.Date) ([]*activeModels.StaffAbsence, error) {
		return []*activeModels.StaffAbsence{existing}, nil // Only itself
	}

	absRepo.updateFunc = func(_ context.Context, entity *activeModels.StaffAbsence) error {
		assert.Equal(t, "Updated note", entity.Note)
		assert.Equal(t, activeModels.AbsenceTypeSick, entity.AbsenceType)
		return nil
	}

	newNote := "Updated note"
	req := UpdateAbsenceRequest{
		Note: &newNote,
	}

	result, err := svc.UpdateAbsence(ctx, staffID, nil, absenceID, req)
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestAbsUpdateAbsence_BlocksVacationType(t *testing.T) {
	svc, absRepo, _ := absSetupService()
	staffID := int64(100)
	absenceID := int64(100)

	absRepo.findByIDFunc = func(_ context.Context, _ any) (*activeModels.StaffAbsence, error) {
		return &activeModels.StaffAbsence{
			Model:       base.Model{ID: absenceID},
			StaffID:     staffID,
			AbsenceType: activeModels.AbsenceTypeSick,
			Status:      activeModels.AbsenceStatusReported,
		}, nil
	}
	absRepo.updateFunc = func(_ context.Context, _ *activeModels.StaffAbsence) error {
		t.Fatal("generic absence update must not convert rows to vacation")
		return nil
	}

	newType := activeModels.AbsenceTypeVacation
	result, err := svc.UpdateAbsence(context.Background(), staffID, nil, absenceID, UpdateAbsenceRequest{AbsenceType: &newType})

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "vacation flow")
}

func TestAbsUpdateAbsence_NotFound(t *testing.T) {
	svc, absRepo, _ := absSetupService()

	absRepo.findByIDFunc = func(_ context.Context, _ any) (*activeModels.StaffAbsence, error) {
		return nil, errors.New("not found")
	}

	result, err := svc.UpdateAbsence(context.Background(), 1, nil, 999, UpdateAbsenceRequest{})
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

	result, err := svc.UpdateAbsence(context.Background(), 1, nil, 100, UpdateAbsenceRequest{})
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
			DateStart: timezone.NewDate(2026, 2, 10),
			DateEnd:   timezone.NewDate(2026, 2, 12),
		}, nil
	}

	bad := "not-a-date"
	req := UpdateAbsenceRequest{DateStart: &bad}

	result, err := svc.UpdateAbsence(context.Background(), staffID, nil, 100, req)
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
			DateStart: timezone.NewDate(2026, 2, 10),
			DateEnd:   timezone.NewDate(2026, 2, 12),
		}, nil
	}

	bad := "not-a-date"
	req := UpdateAbsenceRequest{DateEnd: &bad}

	result, err := svc.UpdateAbsence(context.Background(), staffID, nil, 100, req)
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
		DateStart:   timezone.NewDate(2026, 2, 10),
		DateEnd:     timezone.NewDate(2026, 2, 12),
		Status:      activeModels.AbsenceStatusReported,
	}

	absRepo.findByIDFunc = func(_ context.Context, _ any) (*activeModels.StaffAbsence, error) {
		return existing, nil
	}

	absRepo.getByStaffAndDateRangeFunc = func(_ context.Context, _ int64, _, _ timezone.Date) ([]*activeModels.StaffAbsence, error) {
		return []*activeModels.StaffAbsence{
			existing,
			{
				Model:     base.Model{ID: 200},
				StaffID:   staffID,
				DateStart: timezone.NewDate(2026, 2, 15),
				DateEnd:   timezone.NewDate(2026, 2, 17),
				Status:    activeModels.AbsenceStatusReported,
			},
		}, nil
	}

	newEnd := "2026-02-16"
	req := UpdateAbsenceRequest{DateEnd: &newEnd}

	result, err := svc.UpdateAbsence(context.Background(), staffID, nil, absenceID, req)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "overlap")
}

func TestAbsUpdateAbsence_BlocksVacationWorkflowRows(t *testing.T) {
	svc, absRepo, _ := absSetupService()
	staffID := int64(100)
	absenceID := int64(100)

	for _, status := range []string{
		activeModels.AbsenceStatusRequested,
		activeModels.AbsenceStatusApproved,
		activeModels.AbsenceStatusDeclined,
		activeModels.AbsenceStatusCanceled,
	} {
		t.Run(status, func(t *testing.T) {
			absRepo.findByIDFunc = func(_ context.Context, _ any) (*activeModels.StaffAbsence, error) {
				return &activeModels.StaffAbsence{
					Model:       base.Model{ID: absenceID},
					StaffID:     staffID,
					AbsenceType: activeModels.AbsenceTypeVacation,
					Status:      status,
				}, nil
			}
			absRepo.updateFunc = func(_ context.Context, _ *activeModels.StaffAbsence) error {
				t.Fatal("workflow vacation must not be updated through generic absence update")
				return nil
			}

			note := "changed"
			result, err := svc.UpdateAbsence(context.Background(), staffID, nil, absenceID, UpdateAbsenceRequest{Note: &note})

			require.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), "vacation flow")
		})
	}
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

func TestAbsDeleteAbsence_BlocksVacationWorkflowRows(t *testing.T) {
	svc, absRepo, _ := absSetupService()
	staffID := int64(100)
	absenceID := int64(100)

	for _, status := range []string{
		activeModels.AbsenceStatusRequested,
		activeModels.AbsenceStatusApproved,
		activeModels.AbsenceStatusDeclined,
		activeModels.AbsenceStatusCanceled,
	} {
		t.Run(status, func(t *testing.T) {
			absRepo.findByIDFunc = func(_ context.Context, _ any) (*activeModels.StaffAbsence, error) {
				return &activeModels.StaffAbsence{
					Model:       base.Model{ID: absenceID},
					StaffID:     staffID,
					AbsenceType: activeModels.AbsenceTypeVacation,
					Status:      status,
				}, nil
			}
			absRepo.deleteFunc = func(_ context.Context, _ any) error {
				t.Fatal("workflow vacation must not be deleted through generic absence delete")
				return nil
			}

			err := svc.DeleteAbsence(context.Background(), staffID, absenceID)

			require.Error(t, err)
			assert.Contains(t, err.Error(), "vacation flow")
		})
	}
}

// ============================================================================
// GetAbsencesForRange Tests
// ============================================================================

func TestAbsGetAbsencesForRange_Success(t *testing.T) {
	svc, absRepo, _ := absSetupService()
	ctx := context.Background()
	staffID := int64(100)
	from := timezone.NewDate(2026, 2, 1)
	to := timezone.NewDate(2026, 2, 28)

	absRepo.getByStaffAndDateRangeFunc = func(_ context.Context, _ int64, _, _ timezone.Date) ([]*activeModels.StaffAbsence, error) {
		return []*activeModels.StaffAbsence{
			{
				Model:     base.Model{ID: 1},
				DateStart: timezone.NewDate(2026, 2, 10),
				DateEnd:   timezone.NewDate(2026, 2, 12),
			},
			{
				Model:     base.Model{ID: 2},
				DateStart: timezone.NewDate(2026, 2, 15),
				DateEnd:   timezone.NewDate(2026, 2, 15),
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

	absRepo.getByStaffAndDateRangeFunc = func(_ context.Context, _ int64, _, _ timezone.Date) ([]*activeModels.StaffAbsence, error) {
		return nil, nil
	}

	from := timezone.NewDate(2026, 2, 1)
	to := timezone.NewDate(2026, 2, 28)

	results, err := svc.GetAbsencesForRange(context.Background(), 1, from, to)
	require.NoError(t, err)
	assert.Len(t, results, 0)
}

func TestAbsGetAbsencesForRange_RepoError(t *testing.T) {
	svc, absRepo, _ := absSetupService()

	absRepo.getByStaffAndDateRangeFunc = func(_ context.Context, _ int64, _, _ timezone.Date) ([]*activeModels.StaffAbsence, error) {
		return nil, errors.New("database error")
	}

	from := timezone.NewDate(2026, 2, 1)
	to := timezone.NewDate(2026, 2, 28)

	results, err := svc.GetAbsencesForRange(context.Background(), 1, from, to)
	require.Error(t, err)
	assert.Nil(t, results)
	assert.Contains(t, err.Error(), "failed to get absences")
}

func TestAbsListAbsences_ByStatus(t *testing.T) {
	svc, absRepo, _ := absSetupService()
	staffID := int64(100)

	absRepo.listFunc = func(_ context.Context, options *base.QueryOptions) ([]*activeModels.StaffAbsence, error) {
		filteredStaffID, ok := options.Filter.Get("staff_id")
		require.True(t, ok)
		assert.Equal(t, staffID, filteredStaffID)
		status, ok := options.Filter.Get("status")
		require.True(t, ok)
		assert.Equal(t, activeModels.AbsenceStatusQuestion, status)
		require.NotNil(t, options.Sorting)
		require.Len(t, options.Sorting.Fields, 1)
		assert.Equal(t, "requested_at", options.Sorting.Fields[0].Field)
		assert.Equal(t, base.SortDesc, options.Sorting.Fields[0].Direction)

		return []*activeModels.StaffAbsence{{
			Model:     base.Model{ID: 17},
			StaffID:   staffID,
			Status:    activeModels.AbsenceStatusQuestion,
			DateStart: timezone.NewDate(2027, 7, 10),
			DateEnd:   timezone.NewDate(2027, 7, 11),
		}}, nil
	}

	results, err := svc.ListAbsences(context.Background(), staffID, StaffAbsenceListFilter{
		Status: activeModels.AbsenceStatusQuestion,
	})

	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, int64(17), results[0].ID)
	assert.Equal(t, 2, results[0].DurationDays)
}

func TestAbsListAbsences_RejectsInvalidFilters(t *testing.T) {
	svc, _, _ := absSetupService()
	from := timezone.NewDate(2026, 1, 1)

	tests := []struct {
		name   string
		filter StaffAbsenceListFilter
		errMsg string
	}{
		{
			name:   "empty",
			filter: StaffAbsenceListFilter{},
			errMsg: "absence list filter is required",
		},
		{
			name:   "incomplete date range",
			filter: StaffAbsenceListFilter{From: &from},
			errMsg: "from and to must be provided together",
		},
		{
			name:   "unknown status",
			filter: StaffAbsenceListFilter{Status: "unknown"},
			errMsg: "invalid absence status",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			results, err := svc.ListAbsences(context.Background(), 100, tc.filter)
			require.Error(t, err)
			assert.Nil(t, results)
			assert.EqualError(t, err, tc.errMsg)
		})
	}
}

// ============================================================================
// HasAbsenceOnDate Tests
// ============================================================================

func TestAbsHasAbsenceOnDate_Found(t *testing.T) {
	svc, absRepo, _ := absSetupService()
	staffID := int64(100)
	date := timezone.NewDate(2026, 2, 11)

	absRepo.getByStaffAndDateFunc = func(_ context.Context, _ int64, _ timezone.Date) (*activeModels.StaffAbsence, error) {
		return &activeModels.StaffAbsence{
			Model:       base.Model{ID: 100},
			StaffID:     staffID,
			AbsenceType: activeModels.AbsenceTypeSick,
			Status:      activeModels.AbsenceStatusReported,
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

	absRepo.getByStaffAndDateFunc = func(_ context.Context, _ int64, _ timezone.Date) (*activeModels.StaffAbsence, error) {
		return nil, nil
	}

	has, absence, err := svc.HasAbsenceOnDate(context.Background(), 1, timezone.TodayDate())
	require.NoError(t, err)
	assert.False(t, has)
	assert.Nil(t, absence)
}

func TestAbsHasAbsenceOnDate_RepoError(t *testing.T) {
	svc, absRepo, _ := absSetupService()

	absRepo.getByStaffAndDateFunc = func(_ context.Context, _ int64, _ timezone.Date) (*activeModels.StaffAbsence, error) {
		return nil, errors.New("database error")
	}

	has, absence, err := svc.HasAbsenceOnDate(context.Background(), 1, timezone.TodayDate())
	require.Error(t, err)
	assert.False(t, has)
	assert.Nil(t, absence)
	assert.Contains(t, err.Error(), "failed to check absence")
}

func TestAbsHasAbsenceOnDate_IgnoresPendingAndCanceled(t *testing.T) {
	svc, absRepo, _ := absSetupService()
	staffID := int64(100)
	date := timezone.NewDate(2026, 2, 11)

	for _, status := range []string{
		activeModels.AbsenceStatusRequested,
		activeModels.AbsenceStatusDeclined,
		activeModels.AbsenceStatusCanceled,
	} {
		t.Run(status, func(t *testing.T) {
			absRepo.getByStaffAndDateFunc = func(_ context.Context, _ int64, _ timezone.Date) (*activeModels.StaffAbsence, error) {
				return &activeModels.StaffAbsence{
					Model:       base.Model{ID: 100},
					StaffID:     staffID,
					AbsenceType: activeModels.AbsenceTypeVacation,
					Status:      status,
				}, nil
			}

			has, absence, err := svc.HasAbsenceOnDate(context.Background(), staffID, date)
			require.NoError(t, err)
			assert.False(t, has)
			assert.Nil(t, absence)
		})
	}
}

func TestAbsCountWorkingDays_HalfDayOnWeekendDoesNotGoNegative(t *testing.T) {
	saturday := timezone.NewDate(2027, 2, 13)
	sunday := timezone.NewDate(2027, 2, 14)

	assert.Equal(t, 0.0, countWorkingDays(saturday, sunday, true, true))
}

func TestAbsRequestVacation_RejectsWeekendOnlyHalfDayRange(t *testing.T) {
	svc, absRepo, _ := absSetupService()
	staffID := int64(100)

	absRepo.getByStaffAndDateRangeFunc = func(_ context.Context, _ int64, _, _ timezone.Date) ([]*activeModels.StaffAbsence, error) {
		return nil, nil
	}

	result, err := svc.RequestVacation(context.Background(), staffID, RequestVacationRequest{
		DateStart:    "2027-02-13",
		DateEnd:      "2027-02-14",
		StartHalfDay: true,
		EndHalfDay:   true,
		Note:         "weekend",
	})

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "no working days")
}

func TestAbsRequestVacation_IgnoresDeclinedOverlap(t *testing.T) {
	svc, absRepo, _ := absSetupService()
	staffID := int64(100)

	absRepo.getByStaffAndDateRangeFunc = func(_ context.Context, _ int64, _, _ timezone.Date) ([]*activeModels.StaffAbsence, error) {
		return []*activeModels.StaffAbsence{
			{
				Model:       base.Model{ID: 100},
				StaffID:     staffID,
				AbsenceType: activeModels.AbsenceTypeVacation,
				DateStart:   timezone.NewDate(2027, 2, 15),
				DateEnd:     timezone.NewDate(2027, 2, 16),
				Status:      activeModels.AbsenceStatusDeclined,
			},
		}, nil
	}
	absRepo.createFunc = func(_ context.Context, entity *activeModels.StaffAbsence) error {
		require.NotNil(t, entity.WorkingDays)
		assert.Equal(t, 2.0, *entity.WorkingDays)
		entity.ID = 101
		return nil
	}

	result, err := svc.RequestVacation(context.Background(), staffID, RequestVacationRequest{
		DateStart: "2027-02-15",
		DateEnd:   "2027-02-16",
		Note:      "retry",
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, int64(101), result.ID)
}

func TestAbsApproveAbsence_WritesAudit(t *testing.T) {
	absRepo := &absStaffAbsenceRepoMock{}
	auditRepo := &absStaffAbsenceAuditRepoMock{}
	broadcaster := testpkg.NewRecordingBroadcaster()
	svc := &staffAbsenceService{
		absenceRepo: absRepo,
		auditRepo:   auditRepo,
		broadcaster: broadcaster,
	}
	absenceID := int64(1000)
	actorAccountID := int64(2000)
	decidedByStaffID := int64(3000)
	ctx, commit := tenant.WithAfterCommitHooksForTest(
		tenant.WithTenantID(context.Background(), 42),
	)

	absRepo.findByIDFunc = func(_ context.Context, id any) (*activeModels.StaffAbsence, error) {
		assert.Equal(t, absenceID, id)
		return &activeModels.StaffAbsence{
			Model:       base.Model{ID: absenceID},
			StaffID:     int64(4000),
			AbsenceType: activeModels.AbsenceTypeVacation,
			DateStart:   timezone.NewDate(2027, 2, 15),
			DateEnd:     timezone.NewDate(2027, 2, 16),
			Status:      activeModels.AbsenceStatusRequested,
		}, nil
	}
	absRepo.updateFunc = func(_ context.Context, entity *activeModels.StaffAbsence) error {
		assert.Equal(t, activeModels.AbsenceStatusApproved, entity.Status)
		require.NotNil(t, entity.ApprovedBy)
		assert.Equal(t, decidedByStaffID, *entity.ApprovedBy)
		return nil
	}
	auditRepo.createFunc = func(_ context.Context, audit *activeModels.StaffAbsenceAudit) error {
		assert.Equal(t, absenceID, audit.AbsenceID)
		require.NotNil(t, audit.FromStatus)
		assert.Equal(t, activeModels.AbsenceStatusRequested, *audit.FromStatus)
		assert.Equal(t, activeModels.AbsenceStatusApproved, audit.ToStatus)
		assert.Equal(t, actorAccountID, audit.ActorID)
		assert.Equal(t, "ok", audit.Note)
		return nil
	}

	result, err := svc.ApproveAbsence(ctx, absenceID, actorAccountID, decidedByStaffID, "ok")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, activeModels.AbsenceStatusApproved, result.Status)
	assert.Empty(t, broadcaster.Events())

	commit()

	events := broadcaster.EventsOfType(realtime.EventStaffTimeTrackingChanged)
	require.Len(t, events, 1)
	calls := broadcaster.CallsByMethod("tenant")
	require.Len(t, calls, 1)
	assert.Equal(t, int64(42), calls[0].TenantID)
}

func TestAbsDenyAbsence_WritesAudit(t *testing.T) {
	absRepo := &absStaffAbsenceRepoMock{}
	auditRepo := &absStaffAbsenceAuditRepoMock{}
	svc := &staffAbsenceService{
		absenceRepo: absRepo,
		auditRepo:   auditRepo,
	}
	absenceID := int64(1001)
	actorAccountID := int64(2001)
	decidedByStaffID := int64(3001)

	absRepo.findByIDFunc = func(_ context.Context, _ any) (*activeModels.StaffAbsence, error) {
		return &activeModels.StaffAbsence{
			Model:       base.Model{ID: absenceID},
			StaffID:     int64(4001),
			AbsenceType: activeModels.AbsenceTypeVacation,
			DateStart:   timezone.NewDate(2027, 3, 1),
			DateEnd:     timezone.NewDate(2027, 3, 2),
			Status:      activeModels.AbsenceStatusRequested,
		}, nil
	}
	absRepo.updateFunc = func(_ context.Context, entity *activeModels.StaffAbsence) error {
		assert.Equal(t, activeModels.AbsenceStatusDeclined, entity.Status)
		require.NotNil(t, entity.ApprovedBy)
		assert.Equal(t, decidedByStaffID, *entity.ApprovedBy)
		return nil
	}
	auditRepo.createFunc = func(_ context.Context, audit *activeModels.StaffAbsenceAudit) error {
		require.NotNil(t, audit.FromStatus)
		assert.Equal(t, activeModels.AbsenceStatusRequested, *audit.FromStatus)
		assert.Equal(t, activeModels.AbsenceStatusDeclined, audit.ToStatus)
		assert.Equal(t, actorAccountID, audit.ActorID)
		assert.Equal(t, "no", audit.Note)
		return nil
	}

	result, err := svc.DenyAbsence(context.Background(), absenceID, actorAccountID, decidedByStaffID, "no")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, activeModels.AbsenceStatusDeclined, result.Status)
}

func TestAbsCancelAbsence_WritesAudit(t *testing.T) {
	absRepo := &absStaffAbsenceRepoMock{}
	auditRepo := &absStaffAbsenceAuditRepoMock{}
	svc := &staffAbsenceService{
		absenceRepo: absRepo,
		auditRepo:   auditRepo,
	}
	staffID := int64(4002)
	absenceID := int64(1002)
	actorAccountID := int64(2002)

	absRepo.findByIDFunc = func(_ context.Context, _ any) (*activeModels.StaffAbsence, error) {
		return &activeModels.StaffAbsence{
			Model:       base.Model{ID: absenceID},
			StaffID:     staffID,
			AbsenceType: activeModels.AbsenceTypeVacation,
			DateStart:   timezone.NewDate(2027, 4, 1),
			DateEnd:     timezone.NewDate(2027, 4, 2),
			Status:      activeModels.AbsenceStatusApproved,
		}, nil
	}
	absRepo.updateFunc = func(_ context.Context, entity *activeModels.StaffAbsence) error {
		assert.Equal(t, activeModels.AbsenceStatusCanceled, entity.Status)
		return nil
	}
	auditRepo.createFunc = func(_ context.Context, audit *activeModels.StaffAbsenceAudit) error {
		require.NotNil(t, audit.FromStatus)
		assert.Equal(t, activeModels.AbsenceStatusApproved, *audit.FromStatus)
		assert.Equal(t, activeModels.AbsenceStatusCanceled, audit.ToStatus)
		assert.Equal(t, actorAccountID, audit.ActorID)
		assert.Empty(t, audit.Note)
		return nil
	}

	err := svc.CancelAbsence(context.Background(), staffID, actorAccountID, absenceID)

	require.NoError(t, err)
}

func TestAbsVacationCutoffUsesBerlinCalendarDay(t *testing.T) {
	now := time.Date(2026, 6, 7, 0, 30, 0, 0, timezone.Berlin)
	yesterday := timezone.NewDate(2026, 6, 6)
	today := timezone.NewDate(2026, 6, 7)

	assert.True(t, isBeforeLocalToday(yesterday, now))
	assert.False(t, isBeforeLocalToday(today, now))
}

func TestAbsGetVacationQuotaSummary_SplitsCrossYearVacation(t *testing.T) {
	absRepo := &absStaffAbsenceRepoMock{}
	quotaRepo := &absVacationQuotaRepoMock{}
	svc := &staffAbsenceService{
		absenceRepo: absRepo,
		quotaRepo:   quotaRepo,
	}
	staffID := int64(100)
	workingDays := 5.0

	quotaRepo.getByStaffAndYearFunc = func(_ context.Context, _ int64, _ int) (*activeModels.StaffVacationQuota, error) {
		return &activeModels.StaffVacationQuota{
			StaffID:       staffID,
			Year:          2026,
			EntitledDays:  30,
			CarryoverDays: 0,
		}, nil
	}
	absRepo.getByStaffAndDateRangeFunc = func(_ context.Context, _ int64, _, _ timezone.Date) ([]*activeModels.StaffAbsence, error) {
		return []*activeModels.StaffAbsence{
			{
				Model:       base.Model{ID: 100},
				StaffID:     staffID,
				AbsenceType: activeModels.AbsenceTypeVacation,
				DateStart:   timezone.NewDate(2025, 12, 29),
				DateEnd:     timezone.NewDate(2026, 1, 2),
				Status:      activeModels.AbsenceStatusApproved,
				WorkingDays: &workingDays,
			},
		}, nil
	}

	summary, err := svc.GetVacationQuotaSummary(context.Background(), staffID, 2026)
	require.NoError(t, err)
	require.NotNil(t, summary)
	assert.Equal(t, 2.0, summary.TakenDays)
	assert.Equal(t, 28.0, summary.RemainingDays)
}

func TestAbsUpsertVacationQuota_RejectsInvalidPrecision(t *testing.T) {
	quotaRepo := &absVacationQuotaRepoMock{}
	upsertCalled := false
	quotaRepo.upsertFunc = func(context.Context, *activeModels.StaffVacationQuota) error {
		upsertCalled = true
		return nil
	}
	svc := &staffAbsenceService{quotaRepo: quotaRepo}

	err := svc.UpsertVacationQuota(context.Background(), 42, 2026, 30.25, 0)

	require.ErrorIs(t, err, ErrVacationQuotaInvalid)
	assert.Contains(t, err.Error(), "at most one decimal place")
	assert.False(t, upsertCalled)
}

// Generic query helper stubs (interface additions for the issue #585
// cleanup refactor) — unused by these tests.
func (m *absStaffAbsenceRepoMock) CountWithOptions(context.Context, *base.QueryOptions) (int, error) {
	return 0, nil
}

func (m *absStaffAbsenceRepoMock) OldestBefore(context.Context, string, *timezone.Date) (*timezone.Date, error) {
	return nil, nil
}

func (m *absStaffAbsenceRepoMock) DeleteOlderThan(context.Context, string, timezone.Date) (int64, error) {
	return 0, nil
}

func (m *absStaffAbsenceRepoMock) DeleteNonHistoricalByStaffID(context.Context, int64, timezone.Date) (int64, error) {
	return 0, nil
}

func (m *absStaffAbsenceRepoMock) ListNonHistoricalByStaffID(context.Context, int64, timezone.Date) ([]*activeModels.StaffAbsence, error) {
	return nil, nil
}

func (m *absWorkSessionRepoMock) CountWithOptions(context.Context, *base.QueryOptions) (int, error) {
	return 0, nil
}

func (m *absWorkSessionRepoMock) OldestBefore(context.Context, string, *timezone.Date) (*timezone.Date, error) {
	return nil, nil
}

func (m *absWorkSessionRepoMock) DeleteOlderThan(context.Context, string, timezone.Date) (int64, error) {
	return 0, nil
}

// ============================================================================
// #1843 sick cascade wiring (stub syncer — the real cascade is covered by the
// hermetic tests in services/schedule/shift_plan_sync_service_test.go)
// ============================================================================

type absShiftPlanSyncerMock struct {
	markCalls      []SickCascadeInput
	clearCalls     []SickCascadeInput
	reconcileCalls [][2]SickCascadeInput
	reassignCalls  [][2]int64
	markErr        error
	clearErr       error
	reconcileErr   error
	reassignErr    error
}

func (m *absShiftPlanSyncerMock) MarkSickForRange(_ context.Context, in SickCascadeInput) error {
	m.markCalls = append(m.markCalls, in)
	return m.markErr
}

func (m *absShiftPlanSyncerMock) ClearSickForRange(_ context.Context, in SickCascadeInput) error {
	m.clearCalls = append(m.clearCalls, in)
	return m.clearErr
}

func (m *absShiftPlanSyncerMock) ReconcileSickRange(_ context.Context, before, after SickCascadeInput) error {
	m.reconcileCalls = append(m.reconcileCalls, [2]SickCascadeInput{before, after})
	return m.reconcileErr
}

func absSetupServiceWithSyncer() (*staffAbsenceService, *absStaffAbsenceRepoMock, *absShiftPlanSyncerMock) {
	svc, absRepo, workRepo := absSetupService()
	workRepo.getHistoryByStaffIDFunc = func(_ context.Context, _ int64, _, _ timezone.Date) ([]*activeModels.WorkSession, error) {
		return nil, nil
	}
	syncer := &absShiftPlanSyncerMock{}
	svc.SetShiftPlanSyncer(syncer)
	return svc, absRepo, syncer
}

func TestAbsCreateAbsenceFor_SickCascadesWithActor(t *testing.T) {
	svc, absRepo, syncer := absSetupServiceWithSyncer()
	subjectID, creatorID, accountID := int64(100), int64(200), int64(77)

	absRepo.getByStaffAndDateRangeFunc = func(_ context.Context, _ int64, _, _ timezone.Date) ([]*activeModels.StaffAbsence, error) {
		return nil, nil
	}
	absRepo.createFunc = func(_ context.Context, entity *activeModels.StaffAbsence) error {
		assert.Equal(t, subjectID, entity.StaffID)
		assert.Equal(t, creatorID, entity.CreatedBy, "creator must be the admin, not the subject")
		entity.ID = 555
		return nil
	}

	_, err := svc.CreateAbsenceFor(context.Background(), subjectID, creatorID, &accountID, CreateAbsenceRequest{
		AbsenceType: activeModels.AbsenceTypeSick,
		DateStart:   "2026-02-10",
		DateEnd:     "2026-02-12",
	})
	require.NoError(t, err)
	require.Len(t, syncer.markCalls, 1)
	call := syncer.markCalls[0]
	assert.Equal(t, subjectID, call.SubjectStaffID)
	assert.Equal(t, int64(555), call.AbsenceID)
	assert.Equal(t, creatorID, call.ActorStaffID)
	require.NotNil(t, call.ActorAccountID)
	assert.Equal(t, accountID, *call.ActorAccountID)
	assert.Equal(t, "2026-02-10", call.DateStart.String())
	assert.Equal(t, "2026-02-12", call.DateEnd.String())
}

func TestAbsCreateAbsenceFor_HalfDayAndNonSickSkipCascade(t *testing.T) {
	for _, tc := range []struct {
		name string
		req  CreateAbsenceRequest
	}{
		{name: "half day sick", req: CreateAbsenceRequest{AbsenceType: activeModels.AbsenceTypeSick, DateStart: "2026-02-10", DateEnd: "2026-02-10", HalfDay: true}},
		{name: "training", req: CreateAbsenceRequest{AbsenceType: activeModels.AbsenceTypeTraining, DateStart: "2026-02-10", DateEnd: "2026-02-12"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, absRepo, syncer := absSetupServiceWithSyncer()
			absRepo.getByStaffAndDateRangeFunc = func(_ context.Context, _ int64, _, _ timezone.Date) ([]*activeModels.StaffAbsence, error) {
				return nil, nil
			}
			_, err := svc.CreateAbsenceFor(context.Background(), 100, 100, nil, tc.req)
			require.NoError(t, err)
			assert.Empty(t, syncer.markCalls, "cascade must not fire")
		})
	}
}

func TestAbsCreateAbsenceFor_RejectsMultiDayHalfDaySickReport(t *testing.T) {
	svc, absRepo, syncer := absSetupServiceWithSyncer()
	createCalled := false
	absRepo.createFunc = func(_ context.Context, _ *activeModels.StaffAbsence) error {
		createCalled = true
		return nil
	}

	_, err := svc.CreateAbsenceFor(context.Background(), 100, 100, nil, CreateAbsenceRequest{
		AbsenceType: activeModels.AbsenceTypeSick,
		DateStart:   "2026-02-10",
		DateEnd:     "2026-02-11",
		HalfDay:     true,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "half-day reports must cover exactly one date")
	assert.False(t, createCalled)
	assert.Empty(t, syncer.markCalls)
}

func TestAbsCreateAbsenceFor_RejectsMultiDayHalfDayCompTime(t *testing.T) {
	svc, absRepo, syncer := absSetupServiceWithSyncer()
	createCalled := false
	absRepo.createFunc = func(_ context.Context, _ *activeModels.StaffAbsence) error {
		createCalled = true
		return nil
	}

	_, err := svc.CreateAbsenceFor(context.Background(), 100, 100, nil, CreateAbsenceRequest{
		AbsenceType: activeModels.AbsenceTypeCompTime,
		DateStart:   "2026-02-10",
		DateEnd:     "2026-02-11",
		HalfDay:     true,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "half-day reports must cover exactly one date")
	assert.False(t, createCalled)
	assert.Empty(t, syncer.markCalls)
}

// A comp_time absence dated before the account start would show up in the
// history without ever reducing the Stundenkonto: balance aggregation begins
// at the anchor. The create must reject it, mirroring the balance-adjustment
// guard (#1420).
func TestAbsCreateAbsenceFor_RejectsCompTimeBeforeAccountStart(t *testing.T) {
	svc, absRepo, syncer := absSetupServiceWithSyncer()
	svc.settings = &wtmMockSettings{accountStart: "2026-08-01"}
	createCalled := false
	absRepo.createFunc = func(_ context.Context, _ *activeModels.StaffAbsence) error {
		createCalled = true
		return nil
	}

	_, err := svc.CreateAbsenceFor(context.Background(), 100, 200, nil, CreateAbsenceRequest{
		AbsenceType: activeModels.AbsenceTypeCompTime,
		DateStart:   "2026-07-31",
		DateEnd:     "2026-07-31",
		Note:        "Freizeitausgleich",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid comp_time absence")
	assert.Contains(t, err.Error(), "before the account start")
	assert.False(t, createCalled)
	assert.Empty(t, syncer.markCalls)
}

func TestAbsCreateAbsenceFor_AllowsCompTimeOnAccountStart(t *testing.T) {
	svc, absRepo, _ := absSetupServiceWithSyncer()
	svc.settings = &wtmMockSettings{accountStart: "2026-08-01"}
	absRepo.getByStaffAndDateRangeFunc = func(_ context.Context, _ int64, _, _ timezone.Date) ([]*activeModels.StaffAbsence, error) {
		return nil, nil
	}
	absRepo.createFunc = func(_ context.Context, absence *activeModels.StaffAbsence) error {
		absence.ID = 88
		return nil
	}

	result, err := svc.CreateAbsenceFor(context.Background(), 100, 200, nil, CreateAbsenceRequest{
		AbsenceType: activeModels.AbsenceTypeCompTime,
		DateStart:   "2026-08-01",
		DateEnd:     "2026-08-01",
		Note:        "Freizeitausgleich",
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, int64(88), result.ID)
}

// Non-comp_time absences may legitimately predate the account start (a sick
// day from before the Stundenkonto existed is still a fact worth recording).
func TestAbsCreateAbsenceFor_AllowsSickBeforeAccountStart(t *testing.T) {
	svc, absRepo, _ := absSetupServiceWithSyncer()
	svc.settings = &wtmMockSettings{accountStart: "2026-08-01"}
	absRepo.getByStaffAndDateRangeFunc = func(_ context.Context, _ int64, _, _ timezone.Date) ([]*activeModels.StaffAbsence, error) {
		return nil, nil
	}
	absRepo.createFunc = func(_ context.Context, absence *activeModels.StaffAbsence) error {
		absence.ID = 89
		return nil
	}

	result, err := svc.CreateAbsenceFor(context.Background(), 100, 200, nil, CreateAbsenceRequest{
		AbsenceType: activeModels.AbsenceTypeSick,
		DateStart:   "2026-07-31",
		DateEnd:     "2026-07-31",
		HalfDay:     true,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, int64(89), result.ID)
}

func TestAbsUpdateAbsence_RejectsMultiDayHalfDaySickReport(t *testing.T) {
	svc, absRepo, syncer := absSetupServiceWithSyncer()
	existing := &activeModels.StaffAbsence{
		StaffID:     100,
		AbsenceType: activeModels.AbsenceTypeSick,
		DateStart:   timezone.NewDate(2026, 2, 10),
		DateEnd:     timezone.NewDate(2026, 2, 10),
		HalfDay:     true,
		Status:      activeModels.AbsenceStatusReported,
		CreatedBy:   100,
	}
	existing.ID = 501
	absRepo.findByIDFunc = func(_ context.Context, _ any) (*activeModels.StaffAbsence, error) {
		return existing, nil
	}

	dateEnd := "2026-02-11"
	_, err := svc.UpdateAbsence(context.Background(), existing.StaffID, nil, existing.ID, UpdateAbsenceRequest{
		DateEnd: &dateEnd,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "half-day reports must cover exactly one date")
	assert.Empty(t, syncer.reconcileCalls)
}

func TestAbsCreateAbsenceFor_CascadeErrorAbortsCreate(t *testing.T) {
	svc, absRepo, syncer := absSetupServiceWithSyncer()
	syncer.markErr = errors.New("boom")
	absRepo.getByStaffAndDateRangeFunc = func(_ context.Context, _ int64, _, _ timezone.Date) ([]*activeModels.StaffAbsence, error) {
		return nil, nil
	}
	_, err := svc.CreateAbsenceFor(context.Background(), 100, 100, nil, CreateAbsenceRequest{
		AbsenceType: activeModels.AbsenceTypeSick,
		DateStart:   "2026-02-10",
		DateEnd:     "2026-02-10",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "plan cascade failed")
}

func TestAbsCreateAbsenceFor_MergeCascadesMergedRange(t *testing.T) {
	svc, absRepo, syncer := absSetupServiceWithSyncer()
	existing := &activeModels.StaffAbsence{
		StaffID:     100,
		AbsenceType: activeModels.AbsenceTypeSick,
		DateStart:   timezone.NewDate(2026, 2, 10),
		DateEnd:     timezone.NewDate(2026, 2, 12),
		Status:      activeModels.AbsenceStatusReported,
	}
	existing.ID = 42
	absRepo.getByStaffAndDateRangeFunc = func(_ context.Context, _ int64, _, _ timezone.Date) ([]*activeModels.StaffAbsence, error) {
		return []*activeModels.StaffAbsence{existing}, nil
	}

	_, err := svc.CreateAbsenceFor(context.Background(), 100, 100, nil, CreateAbsenceRequest{
		AbsenceType: activeModels.AbsenceTypeSick,
		DateStart:   "2026-02-12",
		DateEnd:     "2026-02-14",
	})
	require.NoError(t, err)
	require.Len(t, syncer.markCalls, 1)
	call := syncer.markCalls[0]
	assert.Equal(t, int64(42), call.AbsenceID, "cascade must target the merged primary")
	assert.Equal(t, "2026-02-10", call.DateStart.String(), "cascade must cover the merged range")
	assert.Equal(t, "2026-02-14", call.DateEnd.String())
}

func TestAbsDeleteAbsenceFor_SickReversesCascadeBeforeDelete(t *testing.T) {
	svc, absRepo, syncer := absSetupServiceWithSyncer()
	sick := &activeModels.StaffAbsence{
		StaffID:     100,
		AbsenceType: activeModels.AbsenceTypeSick,
		DateStart:   timezone.NewDate(2026, 2, 10),
		DateEnd:     timezone.NewDate(2026, 2, 11),
		Status:      activeModels.AbsenceStatusReported,
	}
	sick.ID = 42
	absRepo.findByIDFunc = func(_ context.Context, _ any) (*activeModels.StaffAbsence, error) {
		return sick, nil
	}
	deleted := false
	absRepo.deleteFunc = func(_ context.Context, _ any) error {
		deleted = true
		return nil
	}

	accountID := int64(77)
	require.NoError(t, svc.DeleteAbsenceFor(context.Background(), 100, 200, &accountID, 42))
	require.Len(t, syncer.clearCalls, 1)
	assert.Equal(t, int64(42), syncer.clearCalls[0].AbsenceID)
	assert.Equal(t, int64(200), syncer.clearCalls[0].ActorStaffID)
	assert.True(t, deleted)
}

func TestAbsDeleteAbsenceFor_ReversalErrorKeepsAbsence(t *testing.T) {
	svc, absRepo, syncer := absSetupServiceWithSyncer()
	syncer.clearErr = errors.New("boom")
	sick := &activeModels.StaffAbsence{
		StaffID:     100,
		AbsenceType: activeModels.AbsenceTypeSick,
		DateStart:   timezone.NewDate(2026, 2, 10),
		DateEnd:     timezone.NewDate(2026, 2, 11),
		Status:      activeModels.AbsenceStatusReported,
	}
	sick.ID = 42
	absRepo.findByIDFunc = func(_ context.Context, _ any) (*activeModels.StaffAbsence, error) {
		return sick, nil
	}
	deleted := false
	absRepo.deleteFunc = func(_ context.Context, _ any) error {
		deleted = true
		return nil
	}

	err := svc.DeleteAbsenceFor(context.Background(), 100, 100, nil, 42)
	require.Error(t, err)
	assert.False(t, deleted, "a failed reversal must abort the delete")
}

func TestAbsDeleteAbsence_ReversalRunsUnconditionally(t *testing.T) {
	// The reversal is keyed by provenance stamps, so it must run for EVERY
	// delete: a non-sick or half-day row that never cascaded is a no-op, but
	// a mutated row must still release whatever it stamped (#1843 review).
	svc, absRepo, syncer := absSetupServiceWithSyncer()
	other := &activeModels.StaffAbsence{
		StaffID:     100,
		AbsenceType: activeModels.AbsenceTypeOther,
		DateStart:   timezone.NewDate(2026, 2, 10),
		DateEnd:     timezone.NewDate(2026, 2, 10),
		Status:      activeModels.AbsenceStatusReported,
	}
	other.ID = 42
	absRepo.findByIDFunc = func(_ context.Context, _ any) (*activeModels.StaffAbsence, error) {
		return other, nil
	}

	require.NoError(t, svc.DeleteAbsence(context.Background(), 100, 42))
	require.Len(t, syncer.clearCalls, 1)
	assert.Equal(t, int64(42), syncer.clearCalls[0].AbsenceID)
}

func TestAbsUpdateAbsence_RejectsSickHalfDayFlip(t *testing.T) {
	svc, absRepo, _ := absSetupServiceWithSyncer()
	sick := &activeModels.StaffAbsence{
		StaffID:     100,
		AbsenceType: activeModels.AbsenceTypeSick,
		DateStart:   timezone.NewDate(2026, 2, 10),
		DateEnd:     timezone.NewDate(2026, 2, 10),
		Status:      activeModels.AbsenceStatusReported,
	}
	sick.ID = 42
	absRepo.findByIDFunc = func(_ context.Context, _ any) (*activeModels.StaffAbsence, error) {
		return sick, nil
	}
	half := true
	_, err := svc.UpdateAbsence(context.Background(), 100, nil, 42, UpdateAbsenceRequest{HalfDay: &half})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "half and full days")
}

func (m *absShiftPlanSyncerMock) ReassignSickStamps(_ context.Context, fromAbsenceID, toAbsenceID int64) error {
	m.reassignCalls = append(m.reassignCalls, [2]int64{fromAbsenceID, toAbsenceID})
	return m.reassignErr
}

func TestAbsUpdateAbsence_RejectsSickTypeChange(t *testing.T) {
	for _, tc := range []struct {
		name    string
		current string
		newType string
	}{
		{name: "sick to other", current: activeModels.AbsenceTypeSick, newType: activeModels.AbsenceTypeOther},
		{name: "other to sick", current: activeModels.AbsenceTypeOther, newType: activeModels.AbsenceTypeSick},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, absRepo, _ := absSetupServiceWithSyncer()
			existing := &activeModels.StaffAbsence{
				StaffID:     100,
				AbsenceType: tc.current,
				DateStart:   timezone.NewDate(2026, 2, 10),
				DateEnd:     timezone.NewDate(2026, 2, 11),
				Status:      activeModels.AbsenceStatusReported,
			}
			existing.ID = 42
			absRepo.findByIDFunc = func(_ context.Context, _ any) (*activeModels.StaffAbsence, error) {
				return existing, nil
			}
			_, err := svc.UpdateAbsence(context.Background(), 100, nil, 42, UpdateAbsenceRequest{
				AbsenceType: &tc.newType,
			})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid absence type change")
		})
	}
}

func TestAbsUpdateAbsence_ReconcilesSickDateDifference(t *testing.T) {
	svc, absRepo, syncer := absSetupServiceWithSyncer()
	existing := &activeModels.StaffAbsence{
		StaffID:     100,
		CreatedBy:   200,
		AbsenceType: activeModels.AbsenceTypeSick,
		DateStart:   timezone.NewDate(2026, 2, 10),
		DateEnd:     timezone.NewDate(2026, 2, 11),
		Status:      activeModels.AbsenceStatusReported,
	}
	existing.ID = 42
	absRepo.findByIDFunc = func(_ context.Context, _ any) (*activeModels.StaffAbsence, error) {
		return existing, nil
	}
	absRepo.getByStaffAndDateRangeFunc = func(_ context.Context, _ int64, _, _ timezone.Date) ([]*activeModels.StaffAbsence, error) {
		return []*activeModels.StaffAbsence{existing}, nil
	}

	newEnd := "2026-02-13"
	actorAccountID := int64(77)
	_, err := svc.UpdateAbsence(context.Background(), 100, &actorAccountID, 42, UpdateAbsenceRequest{DateEnd: &newEnd})
	require.NoError(t, err)
	require.Len(t, syncer.reconcileCalls, 1)
	assert.Equal(t, "2026-02-11", syncer.reconcileCalls[0][0].DateEnd.String())
	assert.Equal(t, "2026-02-13", syncer.reconcileCalls[0][1].DateEnd.String())
	assert.Equal(t, int64(42), syncer.reconcileCalls[0][1].AbsenceID)
	require.NotNil(t, syncer.reconcileCalls[0][0].ActorAccountID)
	assert.Equal(t, actorAccountID, *syncer.reconcileCalls[0][0].ActorAccountID)
	require.NotNil(t, syncer.reconcileCalls[0][1].ActorAccountID)
	assert.Equal(t, actorAccountID, *syncer.reconcileCalls[0][1].ActorAccountID)
}

func TestAbsUpdateAbsence_ReconcileFailurePropagates(t *testing.T) {
	svc, absRepo, syncer := absSetupServiceWithSyncer()
	syncer.reconcileErr = errors.New("plan write failed")
	existing := &activeModels.StaffAbsence{
		StaffID:     100,
		CreatedBy:   200,
		AbsenceType: activeModels.AbsenceTypeSick,
		DateStart:   timezone.NewDate(2026, 2, 10),
		DateEnd:     timezone.NewDate(2026, 2, 11),
		Status:      activeModels.AbsenceStatusReported,
	}
	existing.ID = 42
	absRepo.findByIDFunc = func(_ context.Context, _ any) (*activeModels.StaffAbsence, error) { return existing, nil }
	absRepo.getByStaffAndDateRangeFunc = func(_ context.Context, _ int64, _, _ timezone.Date) ([]*activeModels.StaffAbsence, error) {
		return []*activeModels.StaffAbsence{existing}, nil
	}
	newEnd := "2026-02-13"

	result, err := svc.UpdateAbsence(context.Background(), 100, nil, 42, UpdateAbsenceRequest{DateEnd: &newEnd})

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "plan cascade reconciliation failed")
}

func TestAbsCreateAbsenceFor_MergeReassignsSecondaryStamps(t *testing.T) {
	svc, absRepo, syncer := absSetupServiceWithSyncer()
	primaryRow := &activeModels.StaffAbsence{
		StaffID:     100,
		AbsenceType: activeModels.AbsenceTypeSick,
		DateStart:   timezone.NewDate(2026, 2, 10),
		DateEnd:     timezone.NewDate(2026, 2, 11),
		Status:      activeModels.AbsenceStatusReported,
	}
	primaryRow.ID = 42
	secondary := &activeModels.StaffAbsence{
		StaffID:     100,
		AbsenceType: activeModels.AbsenceTypeSick,
		DateStart:   timezone.NewDate(2026, 2, 13),
		DateEnd:     timezone.NewDate(2026, 2, 14),
		Status:      activeModels.AbsenceStatusReported,
	}
	secondary.ID = 43
	absRepo.getByStaffAndDateRangeFunc = func(_ context.Context, _ int64, _, _ timezone.Date) ([]*activeModels.StaffAbsence, error) {
		return []*activeModels.StaffAbsence{primaryRow, secondary}, nil
	}

	// New report bridges both existing sick absences into one range.
	_, err := svc.CreateAbsenceFor(context.Background(), 100, 100, nil, CreateAbsenceRequest{
		AbsenceType: activeModels.AbsenceTypeSick,
		DateStart:   "2026-02-11",
		DateEnd:     "2026-02-13",
	})
	require.NoError(t, err)
	require.Len(t, syncer.reassignCalls, 1, "the deleted secondary's stamps must move to the primary")
	assert.Equal(t, [2]int64{43, 42}, syncer.reassignCalls[0])
	require.Len(t, syncer.markCalls, 1)
	assert.Equal(t, "2026-02-10", syncer.markCalls[0].DateStart.String())
	assert.Equal(t, "2026-02-14", syncer.markCalls[0].DateEnd.String())
}

func TestAbsCreateAbsenceFor_RejectsMixedDurationSickMerge(t *testing.T) {
	svc, absRepo, syncer := absSetupServiceWithSyncer()
	halfDayExisting := &activeModels.StaffAbsence{
		StaffID:      100,
		AbsenceType:  activeModels.AbsenceTypeSick,
		DateStart:    timezone.NewDate(2026, 2, 10),
		DateEnd:      timezone.NewDate(2026, 2, 10),
		HalfDay:      true,
		StartHalfDay: true,
		EndHalfDay:   true,
		Status:       activeModels.AbsenceStatusReported,
	}
	halfDayExisting.ID = 42
	absRepo.getByStaffAndDateRangeFunc = func(_ context.Context, _ int64, _, _ timezone.Date) ([]*activeModels.StaffAbsence, error) {
		return []*activeModels.StaffAbsence{halfDayExisting}, nil
	}

	_, err := svc.CreateAbsenceFor(context.Background(), 100, 100, nil, CreateAbsenceRequest{
		AbsenceType: activeModels.AbsenceTypeSick,
		DateStart:   "2026-02-10",
		DateEnd:     "2026-02-12",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "half-day and full-day reports cannot be merged")
	assert.True(t, halfDayExisting.HalfDay, "a rejected merge must not mutate the stored report")
	assert.Empty(t, syncer.markCalls)
}

func TestAbsCreateAbsenceFor_RejectsHalfDayExtensionOfFullDaySickReport(t *testing.T) {
	svc, absRepo, syncer := absSetupServiceWithSyncer()
	fullDayExisting := &activeModels.StaffAbsence{
		StaffID:     100,
		AbsenceType: activeModels.AbsenceTypeSick,
		DateStart:   timezone.NewDate(2026, 2, 10),
		DateEnd:     timezone.NewDate(2026, 2, 10),
		Status:      activeModels.AbsenceStatusReported,
	}
	fullDayExisting.ID = 42
	absRepo.getByStaffAndDateRangeFunc = func(_ context.Context, _ int64, _, _ timezone.Date) ([]*activeModels.StaffAbsence, error) {
		return []*activeModels.StaffAbsence{fullDayExisting}, nil
	}

	_, err := svc.CreateAbsenceFor(context.Background(), 100, 100, nil, CreateAbsenceRequest{
		AbsenceType: activeModels.AbsenceTypeSick,
		DateStart:   "2026-02-10",
		DateEnd:     "2026-02-12",
		HalfDay:     true,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "half-day and full-day reports cannot be merged")
	assert.Equal(t, "2026-02-10", fullDayExisting.DateEnd.String(), "a rejected merge must not widen the stored report")
	assert.Empty(t, syncer.markCalls, "half-day dates must not inherit the full-day cascade")
}

// A full-day comp_time create overlapping a stored half-day comp_time must
// not merge: the merge keeps the primary's HalfDay flag, so it would extend a
// half-day row across multiple days and deduct the wrong Stundenkonto amount.
func TestAbsCreateAbsenceFor_RejectsMixedDurationCompTimeMerge(t *testing.T) {
	svc, absRepo, syncer := absSetupServiceWithSyncer()
	halfDayExisting := &activeModels.StaffAbsence{
		StaffID:     100,
		AbsenceType: activeModels.AbsenceTypeCompTime,
		DateStart:   timezone.NewDate(2026, 2, 10),
		DateEnd:     timezone.NewDate(2026, 2, 10),
		HalfDay:     true,
		Status:      activeModels.AbsenceStatusReported,
	}
	halfDayExisting.ID = 42
	absRepo.getByStaffAndDateRangeFunc = func(_ context.Context, _ int64, _, _ timezone.Date) ([]*activeModels.StaffAbsence, error) {
		return []*activeModels.StaffAbsence{halfDayExisting}, nil
	}

	_, err := svc.CreateAbsenceFor(context.Background(), 100, 100, nil, CreateAbsenceRequest{
		AbsenceType: activeModels.AbsenceTypeCompTime,
		DateStart:   "2026-02-10",
		DateEnd:     "2026-02-12",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid comp_time absence: half-day and full-day entries cannot be merged")
	assert.True(t, halfDayExisting.HalfDay, "a rejected merge must not mutate the stored absence")
	assert.Equal(t, "2026-02-10", halfDayExisting.DateEnd.String(), "a rejected merge must not widen the stored absence")
	assert.Empty(t, syncer.markCalls)
}

// The mirror case: a half-day comp_time create overlapping a stored full-day
// comp_time must not silently shrink the requested full day to a half day.
func TestAbsCreateAbsenceFor_RejectsHalfDayExtensionOfFullDayCompTime(t *testing.T) {
	svc, absRepo, syncer := absSetupServiceWithSyncer()
	fullDayExisting := &activeModels.StaffAbsence{
		StaffID:     100,
		AbsenceType: activeModels.AbsenceTypeCompTime,
		DateStart:   timezone.NewDate(2026, 2, 10),
		DateEnd:     timezone.NewDate(2026, 2, 10),
		Status:      activeModels.AbsenceStatusReported,
	}
	fullDayExisting.ID = 42
	absRepo.getByStaffAndDateRangeFunc = func(_ context.Context, _ int64, _, _ timezone.Date) ([]*activeModels.StaffAbsence, error) {
		return []*activeModels.StaffAbsence{fullDayExisting}, nil
	}

	_, err := svc.CreateAbsenceFor(context.Background(), 100, 100, nil, CreateAbsenceRequest{
		AbsenceType: activeModels.AbsenceTypeCompTime,
		DateStart:   "2026-02-10",
		DateEnd:     "2026-02-10",
		HalfDay:     true,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid comp_time absence: half-day and full-day entries cannot be merged")
	assert.False(t, fullDayExisting.HalfDay, "a rejected merge must not mutate the stored absence")
	assert.Empty(t, syncer.markCalls)
}

func TestAbsCreateAbsenceFor_RejectsOversizedSickRange(t *testing.T) {
	svc, absRepo, syncer := absSetupServiceWithSyncer()
	checkedOverlap := false
	absRepo.getByStaffAndDateRangeFunc = func(_ context.Context, _ int64, _, _ timezone.Date) ([]*activeModels.StaffAbsence, error) {
		checkedOverlap = true
		return nil, nil
	}

	_, err := svc.CreateAbsenceFor(context.Background(), 100, 100, nil, CreateAbsenceRequest{
		AbsenceType: activeModels.AbsenceTypeSick,
		DateStart:   "2026-01-01",
		DateEnd:     "2027-01-02",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "date range exceeds 366 days")
	assert.False(t, checkedOverlap, "invalid range must fail before database work")
	assert.Empty(t, syncer.markCalls)
}

func TestAbsUpdateAbsence_RejectsOversizedSickRange(t *testing.T) {
	svc, absRepo, syncer := absSetupServiceWithSyncer()
	existing := &activeModels.StaffAbsence{
		StaffID: 100, CreatedBy: 100,
		AbsenceType: activeModels.AbsenceTypeSick,
		DateStart:   timezone.NewDate(2026, 1, 1), DateEnd: timezone.NewDate(2026, 1, 2),
		Status: activeModels.AbsenceStatusReported,
	}
	existing.ID = 42
	absRepo.findByIDFunc = func(_ context.Context, _ any) (*activeModels.StaffAbsence, error) { return existing, nil }
	newEnd := "2027-01-02"

	_, err := svc.UpdateAbsence(context.Background(), 100, nil, 42, UpdateAbsenceRequest{DateEnd: &newEnd})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "date range exceeds 366 days")
	assert.Empty(t, syncer.reconcileCalls)
}

func TestAbsCreateAbsenceFor_MergeDeleteFailureAborts(t *testing.T) {
	svc, absRepo, syncer := absSetupServiceWithSyncer()
	primary := &activeModels.StaffAbsence{
		StaffID: 100, AbsenceType: activeModels.AbsenceTypeSick,
		DateStart: timezone.NewDate(2026, 2, 10), DateEnd: timezone.NewDate(2026, 2, 11),
		Status: activeModels.AbsenceStatusReported,
	}
	primary.ID = 42
	secondary := &activeModels.StaffAbsence{
		StaffID: 100, AbsenceType: activeModels.AbsenceTypeSick,
		DateStart: timezone.NewDate(2026, 2, 12), DateEnd: timezone.NewDate(2026, 2, 13),
		Status: activeModels.AbsenceStatusReported,
	}
	secondary.ID = 43
	absRepo.getByStaffAndDateRangeFunc = func(_ context.Context, _ int64, _, _ timezone.Date) ([]*activeModels.StaffAbsence, error) {
		return []*activeModels.StaffAbsence{primary, secondary}, nil
	}
	absRepo.deleteFunc = func(_ context.Context, id any) error {
		if id == secondary.ID {
			return errors.New("delete failed")
		}
		return nil
	}

	result, err := svc.CreateAbsenceFor(context.Background(), 100, 100, nil, CreateAbsenceRequest{
		AbsenceType: activeModels.AbsenceTypeSick,
		DateStart:   "2026-02-11",
		DateEnd:     "2026-02-12",
	})

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to delete merged absence")
	require.Len(t, syncer.reassignCalls, 1)
	assert.Empty(t, syncer.markCalls, "a failed merge must not continue into the cascade")
}

// ============================================================================
// Rückfrage (question) + resubmit workflow (#1419 4d)
// ============================================================================

func TestAbsQuestionAbsence_WritesAuditAndDecisionNote(t *testing.T) {
	absRepo := &absStaffAbsenceRepoMock{}
	auditRepo := &absStaffAbsenceAuditRepoMock{}
	svc := &staffAbsenceService{
		absenceRepo: absRepo,
		auditRepo:   auditRepo,
	}
	absenceID := int64(1100)
	actorAccountID := int64(2100)

	absRepo.findByIDFunc = func(_ context.Context, _ any) (*activeModels.StaffAbsence, error) {
		return &activeModels.StaffAbsence{
			Model:       base.Model{ID: absenceID},
			StaffID:     int64(4100),
			AbsenceType: activeModels.AbsenceTypeVacation,
			DateStart:   timezone.NewDate(2027, 5, 3),
			DateEnd:     timezone.NewDate(2027, 5, 4),
			Status:      activeModels.AbsenceStatusRequested,
		}, nil
	}
	absRepo.updateFunc = func(_ context.Context, entity *activeModels.StaffAbsence) error {
		assert.Equal(t, activeModels.AbsenceStatusQuestion, entity.Status)
		assert.Equal(t, "Wer übernimmt die Frühschicht?", entity.DecisionNote)
		assert.Nil(t, entity.ApprovedBy, "a question is not a decision")
		assert.Nil(t, entity.ApprovedAt, "a question is not a decision")
		return nil
	}
	auditRepo.createFunc = func(_ context.Context, audit *activeModels.StaffAbsenceAudit) error {
		require.NotNil(t, audit.FromStatus)
		assert.Equal(t, activeModels.AbsenceStatusRequested, *audit.FromStatus)
		assert.Equal(t, activeModels.AbsenceStatusQuestion, audit.ToStatus)
		assert.Equal(t, actorAccountID, audit.ActorID)
		assert.Equal(t, "Wer übernimmt die Frühschicht?", audit.Note)
		return nil
	}

	result, err := svc.QuestionAbsence(context.Background(), absenceID, actorAccountID, "Wer übernimmt die Frühschicht?")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, activeModels.AbsenceStatusQuestion, result.Status)
}

func TestAbsQuestionAbsence_RequiresNote(t *testing.T) {
	svc := &staffAbsenceService{absenceRepo: &absStaffAbsenceRepoMock{}}

	result, err := svc.QuestionAbsence(context.Background(), int64(1101), int64(2101), "")

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, "question note is required", err.Error())
}

func TestAbsQuestionAbsence_RejectsWhitespaceOnlyNote(t *testing.T) {
	svc := &staffAbsenceService{absenceRepo: &absStaffAbsenceRepoMock{}}

	result, err := svc.QuestionAbsence(context.Background(), int64(1101), int64(2101), " \t\n ")

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, "question note is required", err.Error())
}

func TestAbsQuestionAbsence_TrimsStoredAndAuditedNote(t *testing.T) {
	absRepo := &absStaffAbsenceRepoMock{}
	auditRepo := &absStaffAbsenceAuditRepoMock{}
	svc := &staffAbsenceService{absenceRepo: absRepo, auditRepo: auditRepo}
	absence := &activeModels.StaffAbsence{
		Model:   base.Model{ID: int64(1110)},
		StaffID: int64(4110),
		Status:  activeModels.AbsenceStatusRequested,
	}
	absRepo.findByIDFunc = func(context.Context, any) (*activeModels.StaffAbsence, error) {
		return absence, nil
	}
	absRepo.updateFunc = func(_ context.Context, entity *activeModels.StaffAbsence) error {
		assert.Equal(t, "Wer übernimmt?", entity.DecisionNote)
		return nil
	}
	auditRepo.createFunc = func(_ context.Context, audit *activeModels.StaffAbsenceAudit) error {
		assert.Equal(t, "Wer übernimmt?", audit.Note)
		return nil
	}

	_, err := svc.QuestionAbsence(context.Background(), absence.ID, int64(2110), "  Wer übernimmt? \n")

	require.NoError(t, err)
}

func TestAbsQuestionAbsence_OnlyFromRequested(t *testing.T) {
	absRepo := &absStaffAbsenceRepoMock{}
	svc := &staffAbsenceService{absenceRepo: absRepo, auditRepo: &absStaffAbsenceAuditRepoMock{}}
	absRepo.findByIDFunc = func(_ context.Context, _ any) (*activeModels.StaffAbsence, error) {
		return &activeModels.StaffAbsence{
			Model:  base.Model{ID: int64(1102)},
			Status: activeModels.AbsenceStatusApproved,
		}, nil
	}

	result, err := svc.QuestionAbsence(context.Background(), int64(1102), int64(2102), "note")

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, "only requested absences can be questioned", err.Error())
}

func TestAbsApproveAbsence_FromQuestion(t *testing.T) {
	absRepo := &absStaffAbsenceRepoMock{}
	auditRepo := &absStaffAbsenceAuditRepoMock{}
	svc := &staffAbsenceService{absenceRepo: absRepo, auditRepo: auditRepo}
	decidedByStaffID := int64(3103)

	absRepo.findByIDFunc = func(_ context.Context, _ any) (*activeModels.StaffAbsence, error) {
		return &activeModels.StaffAbsence{
			Model:       base.Model{ID: int64(1103)},
			StaffID:     int64(4103),
			AbsenceType: activeModels.AbsenceTypeVacation,
			DateStart:   timezone.NewDate(2027, 6, 1),
			DateEnd:     timezone.NewDate(2027, 6, 2),
			Status:      activeModels.AbsenceStatusQuestion,
		}, nil
	}
	absRepo.updateFunc = func(_ context.Context, entity *activeModels.StaffAbsence) error {
		assert.Equal(t, activeModels.AbsenceStatusApproved, entity.Status)
		return nil
	}
	auditRepo.createFunc = func(_ context.Context, audit *activeModels.StaffAbsenceAudit) error {
		require.NotNil(t, audit.FromStatus)
		assert.Equal(t, activeModels.AbsenceStatusQuestion, *audit.FromStatus)
		return nil
	}

	result, err := svc.ApproveAbsence(context.Background(), int64(1103), int64(2103), decidedByStaffID, "passt doch")

	require.NoError(t, err)
	assert.Equal(t, activeModels.AbsenceStatusApproved, result.Status)
}

func TestAbsDenyAbsence_FromQuestion(t *testing.T) {
	absRepo := &absStaffAbsenceRepoMock{}
	svc := &staffAbsenceService{absenceRepo: absRepo, auditRepo: &absStaffAbsenceAuditRepoMock{}}

	absRepo.findByIDFunc = func(_ context.Context, _ any) (*activeModels.StaffAbsence, error) {
		return &activeModels.StaffAbsence{
			Model:       base.Model{ID: int64(1104)},
			StaffID:     int64(4104),
			AbsenceType: activeModels.AbsenceTypeVacation,
			DateStart:   timezone.NewDate(2027, 6, 7),
			DateEnd:     timezone.NewDate(2027, 6, 8),
			Status:      activeModels.AbsenceStatusQuestion,
		}, nil
	}
	absRepo.updateFunc = func(_ context.Context, entity *activeModels.StaffAbsence) error {
		assert.Equal(t, activeModels.AbsenceStatusDeclined, entity.Status)
		return nil
	}

	result, err := svc.DenyAbsence(context.Background(), int64(1104), int64(2104), int64(3104), "keine Vertretung")

	require.NoError(t, err)
	assert.Equal(t, activeModels.AbsenceStatusDeclined, result.Status)
}

func TestAbsResubmitAbsence_RestampsAndAudits(t *testing.T) {
	absRepo := &absStaffAbsenceRepoMock{}
	auditRepo := &absStaffAbsenceAuditRepoMock{}
	svc := &staffAbsenceService{absenceRepo: absRepo, auditRepo: auditRepo}
	staffID := int64(4105)
	originalRequestedAt := time.Now().Add(-48 * time.Hour)

	absRepo.findByIDFunc = func(_ context.Context, _ any) (*activeModels.StaffAbsence, error) {
		return &activeModels.StaffAbsence{
			Model:        base.Model{ID: int64(1105)},
			StaffID:      staffID,
			AbsenceType:  activeModels.AbsenceTypeVacation,
			DateStart:    timezone.NewDate(2027, 7, 1),
			DateEnd:      timezone.NewDate(2027, 7, 2),
			Status:       activeModels.AbsenceStatusQuestion,
			Note:         "alte Notiz",
			DecisionNote: "Wer übernimmt die Frühschicht?",
			RequestedAt:  originalRequestedAt,
		}, nil
	}
	absRepo.updateFunc = func(_ context.Context, entity *activeModels.StaffAbsence) error {
		assert.Equal(t, activeModels.AbsenceStatusRequested, entity.Status)
		assert.Equal(t, "Vertretung ist geklärt", entity.Note)
		assert.Equal(t, "Wer übernimmt die Frühschicht?", entity.DecisionNote)
		assert.True(t, entity.RequestedAt.After(originalRequestedAt), "resubmit must re-stamp requested_at")
		return nil
	}
	auditRepo.createFunc = func(_ context.Context, audit *activeModels.StaffAbsenceAudit) error {
		require.NotNil(t, audit.FromStatus)
		assert.Equal(t, activeModels.AbsenceStatusQuestion, *audit.FromStatus)
		assert.Equal(t, activeModels.AbsenceStatusRequested, audit.ToStatus)
		assert.Equal(t, "Vertretung ist geklärt", audit.Note)
		return nil
	}

	result, err := svc.ResubmitAbsence(context.Background(), staffID, int64(2105), int64(1105), "Vertretung ist geklärt")

	require.NoError(t, err)
	assert.Equal(t, activeModels.AbsenceStatusRequested, result.Status)
}

func TestAbsResubmitAbsence_RejectsWhitespaceOnlyNote(t *testing.T) {
	svc := &staffAbsenceService{absenceRepo: &absStaffAbsenceRepoMock{}}

	result, err := svc.ResubmitAbsence(context.Background(), int64(4105), int64(2105), int64(1105), " \t\n ")

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, "resubmit note is required", err.Error())
}

func TestAbsResubmitAbsence_TrimsStoredAndAuditedNote(t *testing.T) {
	absRepo := &absStaffAbsenceRepoMock{}
	auditRepo := &absStaffAbsenceAuditRepoMock{}
	svc := &staffAbsenceService{absenceRepo: absRepo, auditRepo: auditRepo}
	absence := &activeModels.StaffAbsence{
		Model:   base.Model{ID: int64(1111)},
		StaffID: int64(4111),
		Status:  activeModels.AbsenceStatusQuestion,
	}
	absRepo.findByIDFunc = func(context.Context, any) (*activeModels.StaffAbsence, error) {
		return absence, nil
	}
	absRepo.updateFunc = func(_ context.Context, entity *activeModels.StaffAbsence) error {
		assert.Equal(t, "Vertretung geklärt", entity.Note)
		return nil
	}
	auditRepo.createFunc = func(_ context.Context, audit *activeModels.StaffAbsenceAudit) error {
		assert.Equal(t, "Vertretung geklärt", audit.Note)
		return nil
	}

	_, err := svc.ResubmitAbsence(
		context.Background(),
		absence.StaffID,
		int64(2111),
		absence.ID,
		"  Vertretung geklärt \n",
	)

	require.NoError(t, err)
}

func TestAbsResubmitAbsence_OwnershipGuard(t *testing.T) {
	absRepo := &absStaffAbsenceRepoMock{}
	svc := &staffAbsenceService{absenceRepo: absRepo}
	absRepo.findByIDFunc = func(_ context.Context, _ any) (*activeModels.StaffAbsence, error) {
		return &activeModels.StaffAbsence{
			Model:   base.Model{ID: int64(1106)},
			StaffID: int64(4106),
			Status:  activeModels.AbsenceStatusQuestion,
		}, nil
	}

	result, err := svc.ResubmitAbsence(context.Background(), int64(9999), int64(2106), int64(1106), "meins?")

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, "can only resubmit own absences", err.Error())
}

func TestAbsResubmitAbsence_OnlyFromQuestion(t *testing.T) {
	absRepo := &absStaffAbsenceRepoMock{}
	svc := &staffAbsenceService{absenceRepo: absRepo}
	absRepo.findByIDFunc = func(_ context.Context, _ any) (*activeModels.StaffAbsence, error) {
		return &activeModels.StaffAbsence{
			Model:   base.Model{ID: int64(1107)},
			StaffID: int64(4107),
			Status:  activeModels.AbsenceStatusRequested,
		}, nil
	}

	result, err := svc.ResubmitAbsence(context.Background(), int64(4107), int64(2107), int64(1107), "noch mal")

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, "only absences with a question can be resubmitted", err.Error())
}

func TestAbsListPendingRequests_IncludesQuestionRows(t *testing.T) {
	absRepo := &absStaffAbsenceRepoMock{}
	svc := &staffAbsenceService{absenceRepo: absRepo}
	absRepo.listByStatusesFunc = func(_ context.Context, statuses []string) ([]*activeModels.StaffAbsence, error) {
		assert.ElementsMatch(t, []string{
			activeModels.AbsenceStatusRequested,
			activeModels.AbsenceStatusQuestion,
		}, statuses)
		return []*activeModels.StaffAbsence{
			{Model: base.Model{ID: int64(1108)}, Status: activeModels.AbsenceStatusRequested, DateStart: timezone.NewDate(2027, 8, 2), DateEnd: timezone.NewDate(2027, 8, 2)},
			{Model: base.Model{ID: int64(1109)}, Status: activeModels.AbsenceStatusQuestion, DateStart: timezone.NewDate(2027, 8, 3), DateEnd: timezone.NewDate(2027, 8, 3)},
		}, nil
	}

	result, err := svc.ListPendingRequests(context.Background())

	require.NoError(t, err)
	require.Len(t, result, 2)
	assert.Equal(t, activeModels.AbsenceStatusQuestion, result[1].Status)
}

// ============================================================================
// Absence email notifications (#1419 4d)
// ============================================================================

type absSettingsMock struct{ enabled bool }

func (m absSettingsMock) ResolveBool(context.Context, string) (bool, error) {
	return m.enabled, nil
}

func newAbsEmailTestService(t *testing.T, absRepo *absStaffAbsenceRepoMock, enabled bool, staffRepo *testpkg.StaffRepoMock) (*staffAbsenceService, *testpkg.CapturingMailer) {
	t.Helper()
	mailer := testpkg.NewCapturingMailer()
	svc := &staffAbsenceService{
		absenceRepo: absRepo,
		auditRepo:   &absStaffAbsenceAuditRepoMock{},
	}
	svc.SetAbsenceEmailDeps(AbsenceEmailDeps{
		Settings:    absSettingsMock{enabled: enabled},
		Dispatcher:  email.NewDispatcher(mailer, slog.Default()),
		StaffRepo:   staffRepo,
		SchoolRepo:  absenceEmailSchoolFinderStub{school: &platformModels.School{Subdomain: "tenant"}},
		DefaultFrom: email.NewEmail("moto", "no-reply@moto.test"),
		FrontendURL: "http://localhost:3000",
	})
	return svc, mailer
}

func absEmailStaffRepoMock() *testpkg.StaffRepoMock {
	return &testpkg.StaffRepoMock{
		GetStaffContactInfoFn: func(_ context.Context, staffID int64) (*usersModels.StaffWithRoleInfo, error) {
			return &usersModels.StaffWithRoleInfo{
				StaffID:   staffID,
				FirstName: "Mila",
				LastName:  "Muster",
				Email:     "mila@example.test",
			}, nil
		},
		ListStaffWithPermissionFn: func(_ context.Context, permissionName string) ([]*usersModels.StaffWithRoleInfo, error) {
			return []*usersModels.StaffWithRoleInfo{
				{StaffID: int64(8100), FirstName: "Lena", LastName: "Leitung", Email: "lena@example.test"},
			}, nil
		},
	}
}

func TestAbsRequestVacation_SendsApproverEmail(t *testing.T) {
	absRepo := &absStaffAbsenceRepoMock{}
	absRepo.getByStaffAndDateRangeFunc = func(context.Context, int64, timezone.Date, timezone.Date) ([]*activeModels.StaffAbsence, error) {
		return nil, nil
	}
	absRepo.createFunc = func(_ context.Context, entity *activeModels.StaffAbsence) error {
		entity.ID = 1200
		return nil
	}
	svc, mailer := newAbsEmailTestService(t, absRepo, true, absEmailStaffRepoMock())

	ctx := tenant.WithTenantID(context.Background(), int64(7001))
	_, err := svc.RequestVacation(ctx, int64(4200), RequestVacationRequest{
		DateStart: "2027-07-01",
		DateEnd:   "2027-07-02",
		Note:      "Sommerurlaub",
	})
	require.NoError(t, err)

	require.True(t, mailer.WaitForMessages(1, 2*time.Second), "approver email should be dispatched")
	msgs := mailer.Messages()
	require.Len(t, msgs, 1)
	assert.Equal(t, "absence-request-received.html", msgs[0].Template)
	assert.Equal(t, "lena@example.test", msgs[0].To.Address)
	assert.Contains(t, msgs[0].Subject, "Mila Muster")
}

func TestAbsQuestionAbsence_SendsRequesterEmail(t *testing.T) {
	absRepo := &absStaffAbsenceRepoMock{}
	absRepo.findByIDFunc = func(_ context.Context, _ any) (*activeModels.StaffAbsence, error) {
		absence := &activeModels.StaffAbsence{
			Model:       base.Model{ID: int64(1201)},
			StaffID:     int64(4201),
			AbsenceType: activeModels.AbsenceTypeVacation,
			DateStart:   timezone.NewDate(2027, 7, 5),
			DateEnd:     timezone.NewDate(2027, 7, 6),
			Status:      activeModels.AbsenceStatusRequested,
		}
		absence.SetTenantID(int64(7001))
		return absence, nil
	}
	absRepo.updateFunc = func(context.Context, *activeModels.StaffAbsence) error { return nil }
	svc, mailer := newAbsEmailTestService(t, absRepo, true, absEmailStaffRepoMock())

	_, err := svc.QuestionAbsence(context.Background(), int64(1201), int64(2201), "Wie lange genau?")
	require.NoError(t, err)

	require.True(t, mailer.WaitForMessages(1, 2*time.Second), "requester email should be dispatched")
	msgs := mailer.Messages()
	require.Len(t, msgs, 1)
	assert.Equal(t, "absence-request-question.html", msgs[0].Template)
	assert.Equal(t, "mila@example.test", msgs[0].To.Address)
}

func TestAbsEmails_SettingDisabled_NoSend(t *testing.T) {
	absRepo := &absStaffAbsenceRepoMock{}
	absRepo.getByStaffAndDateRangeFunc = func(context.Context, int64, timezone.Date, timezone.Date) ([]*activeModels.StaffAbsence, error) {
		return nil, nil
	}
	absRepo.createFunc = func(_ context.Context, entity *activeModels.StaffAbsence) error {
		entity.ID = 1202
		return nil
	}
	svc, mailer := newAbsEmailTestService(t, absRepo, false, absEmailStaffRepoMock())

	ctx := tenant.WithTenantID(context.Background(), int64(7001))
	_, err := svc.RequestVacation(ctx, int64(4202), RequestVacationRequest{
		DateStart: "2027-07-08",
		DateEnd:   "2027-07-09",
	})
	require.NoError(t, err)

	assert.False(t, mailer.WaitForMessages(1, 300*time.Millisecond), "no email when setting is off")
	assert.Empty(t, mailer.Messages())
}

// ============================================================================
// comp_time overdraft guard (#1420 review)
// ============================================================================

// absMonthServiceMock stubs the WorkTimeMonthService reads the comp_time
// overdraft guard performs; every other method panics via the nil embed.
type absMonthServiceMock struct {
	WorkTimeMonthService
	capacity       int
	capacityDate   timezone.Date
	deduction      int
	deductions     []int
	deductionCalls int
}

func (m *absMonthServiceMock) GetCompTimeDeductionMinutes(context.Context, int64, timezone.Date, timezone.Date, bool) (int, error) {
	if m.deductionCalls < len(m.deductions) {
		deduction := m.deductions[m.deductionCalls]
		m.deductionCalls++
		return deduction, nil
	}
	return m.deduction, nil
}

func (m *absMonthServiceMock) GetBalanceReductionCapacity(_ context.Context, _ int64, effectiveDate timezone.Date) (int, error) {
	m.capacityDate = effectiveDate
	return m.capacity, nil
}

// A multi-day comp_time absence deducting more daily-target minutes than the
// Stundenkonto has accrued must be rejected — the same overdraft guard the
// payout/comp-time adjustments enforce.
func TestAbsCreateAbsenceFor_RejectsCompTimeAboveBalance(t *testing.T) {
	svc, absRepo, syncer := absSetupServiceWithSyncer()
	svc.settings = &wtmMockSettings{accountStart: "2026-06-01"}
	svc.monthService = &absMonthServiceMock{capacity: 480, deduction: 960}
	createCalled := false
	absRepo.createFunc = func(_ context.Context, _ *activeModels.StaffAbsence) error {
		createCalled = true
		return nil
	}

	start := timezone.TodayDate().AddDays(1)
	_, err := svc.CreateAbsenceFor(context.Background(), 100, 200, nil, CreateAbsenceRequest{
		AbsenceType: activeModels.AbsenceTypeCompTime,
		DateStart:   start.String(),
		DateEnd:     start.AddDays(1).String(),
		Note:        "Freizeitausgleich",
	})

	require.ErrorIs(t, err, ErrCompTimeExceedsBalance)
	assert.False(t, createCalled)
	assert.Empty(t, syncer.markCalls)
}

func TestAbsCreateAbsenceFor_AllowsCompTimeWithinBalance(t *testing.T) {
	svc, absRepo, _ := absSetupServiceWithSyncer()
	svc.settings = &wtmMockSettings{accountStart: "2026-06-01"}
	svc.monthService = &absMonthServiceMock{capacity: 960, deduction: 960}
	absRepo.createFunc = func(_ context.Context, absence *activeModels.StaffAbsence) error {
		absence.ID = 90
		return nil
	}

	start := timezone.TodayDate().AddDays(1)
	result, err := svc.CreateAbsenceFor(context.Background(), 100, 200, nil, CreateAbsenceRequest{
		AbsenceType: activeModels.AbsenceTypeCompTime,
		DateStart:   start.String(),
		DateEnd:     start.AddDays(1).String(),
		Note:        "Freizeitausgleich",
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, int64(90), result.ID)
}

func TestAbsCreateAbsenceFor_AllowsWorkedHalfCoveredByAccruedBalance(t *testing.T) {
	svc, absRepo, _ := absSetupServiceWithSyncer()
	svc.settings = &wtmMockSettings{accountStart: "2026-06-01"}
	// The current closing balance is zero after four accrued hours pay for the
	// four unworked target hours. The half-day deduction is already reflected
	// in that closing balance and must not be charged a second time.
	svc.monthService = &absMonthServiceMock{capacity: 0, deduction: 240}
	absRepo.createFunc = func(_ context.Context, absence *activeModels.StaffAbsence) error {
		absence.ID = 91
		return nil
	}

	today := timezone.TodayDate()
	result, err := svc.CreateAbsenceFor(context.Background(), 100, 200, nil, CreateAbsenceRequest{
		AbsenceType: activeModels.AbsenceTypeCompTime,
		DateStart:   today.String(),
		DateEnd:     today.String(),
		HalfDay:     true,
		Note:        "Halber Freizeitausgleich",
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, int64(91), result.ID)
}

func TestAbsCreateAbsenceFor_AllowsFullDayCoveredByAccruedBalance(t *testing.T) {
	svc, absRepo, _ := absSetupServiceWithSyncer()
	svc.settings = &wtmMockSettings{accountStart: "2026-06-01"}
	// The current closing balance is zero after one accrued day pays for the
	// unworked target. The full-day deduction is already reflected in that
	// closing balance and must not be charged a second time.
	svc.monthService = &absMonthServiceMock{capacity: 0, deduction: 480}
	absRepo.createFunc = func(_ context.Context, absence *activeModels.StaffAbsence) error {
		absence.ID = 92
		return nil
	}

	today := timezone.TodayDate()
	result, err := svc.CreateAbsenceFor(context.Background(), 100, 200, nil, CreateAbsenceRequest{
		AbsenceType: activeModels.AbsenceTypeCompTime,
		DateStart:   today.String(),
		DateEnd:     today.String(),
		Note:        "Ganzer Freizeitausgleich",
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, int64(92), result.ID)
}

func TestAbsCreateAbsenceFor_RejectsWorkedHalfWithExistingDeficit(t *testing.T) {
	svc, absRepo, _ := absSetupServiceWithSyncer()
	svc.settings = &wtmMockSettings{accountStart: "2026-06-01"}
	svc.monthService = &absMonthServiceMock{capacity: -1, deduction: 240}
	createCalled := false
	absRepo.createFunc = func(_ context.Context, _ *activeModels.StaffAbsence) error {
		createCalled = true
		return nil
	}

	today := timezone.TodayDate()
	_, err := svc.CreateAbsenceFor(context.Background(), 100, 200, nil, CreateAbsenceRequest{
		AbsenceType: activeModels.AbsenceTypeCompTime,
		DateStart:   today.String(),
		DateEnd:     today.String(),
		HalfDay:     true,
		Note:        "Halber Freizeitausgleich",
	})

	require.ErrorIs(t, err, ErrCompTimeExceedsBalance)
	assert.False(t, createCalled)
}

// The timeline capacity includes same-day and later ledger reductions. The
// shared staff lock serializes both writes, so the second debit must see the
// first and reject their combined overdraft.
func TestAbsCreateAbsenceFor_RejectsCompTimeCombinedWithSameDayAdjustment(t *testing.T) {
	svc, absRepo, _ := absSetupServiceWithSyncer()
	svc.settings = &wtmMockSettings{accountStart: "2026-06-01"}
	svc.monthService = &absMonthServiceMock{
		capacity:  360,
		deduction: 480,
	}
	createCalled := false
	absRepo.createFunc = func(_ context.Context, _ *activeModels.StaffAbsence) error {
		createCalled = true
		return nil
	}

	start := timezone.TodayDate().AddDays(1)
	_, err := svc.CreateAbsenceFor(context.Background(), 100, 200, nil, CreateAbsenceRequest{
		AbsenceType: activeModels.AbsenceTypeCompTime,
		DateStart:   start.String(),
		DateEnd:     start.String(),
		Note:        "Freizeitausgleich",
	})

	require.ErrorIs(t, err, ErrCompTimeExceedsBalance)
	assert.Contains(t, err.Error(), "only 360 minutes remain available")
	assert.False(t, createCalled)
}

// On the account-start day no credit exists to cover the target.
func TestAbsCreateAbsenceFor_RejectsCompTimeOnAccountStartWithoutAccrual(t *testing.T) {
	svc, _, _ := absSetupServiceWithSyncer()
	today := timezone.TodayDate()
	svc.settings = &wtmMockSettings{accountStart: today.String()}
	// The day's missing target has already lowered the closing balance to
	// -480. Adding that realized deduction back leaves no available credit.
	svc.monthService = &absMonthServiceMock{capacity: -480, deduction: 480}

	_, err := svc.CreateAbsenceFor(context.Background(), 100, 200, nil, CreateAbsenceRequest{
		AbsenceType: activeModels.AbsenceTypeCompTime,
		DateStart:   today.String(),
		DateEnd:     today.String(),
		Note:        "Freizeitausgleich",
	})

	require.ErrorIs(t, err, ErrCompTimeExceedsBalance)
	assert.Contains(t, err.Error(), "only 0 minutes remain available")
}

// Extending an existing comp_time absence via the merge path must consume
// capacity only for the newly added part; the existing row is already
// reserved by the timeline capacity calculation.
func TestAbsCreateAbsenceFor_RejectsCompTimeMergeAboveBalance(t *testing.T) {
	svc, absRepo, _ := absSetupServiceWithSyncer()
	svc.settings = &wtmMockSettings{accountStart: "2026-06-01"}
	svc.monthService = &absMonthServiceMock{
		capacity:   479,
		deductions: []int{960, 480},
	}
	start := timezone.TodayDate().AddDays(1)
	existing := &activeModels.StaffAbsence{
		StaffID:     100,
		AbsenceType: activeModels.AbsenceTypeCompTime,
		DateStart:   start,
		DateEnd:     start,
		Status:      activeModels.AbsenceStatusReported,
	}
	existing.ID = 700
	absRepo.getByStaffAndDateRangeFunc = func(_ context.Context, _ int64, _, _ timezone.Date) ([]*activeModels.StaffAbsence, error) {
		return []*activeModels.StaffAbsence{existing}, nil
	}
	updateCalled := false
	absRepo.updateFunc = func(_ context.Context, _ *activeModels.StaffAbsence) error {
		updateCalled = true
		return nil
	}

	_, err := svc.CreateAbsenceFor(context.Background(), 100, 200, nil, CreateAbsenceRequest{
		AbsenceType: activeModels.AbsenceTypeCompTime,
		DateStart:   start.AddDays(1).String(),
		DateEnd:     start.AddDays(1).String(),
		Note:        "Verlängerung",
	})

	require.ErrorIs(t, err, ErrCompTimeExceedsBalance)
	assert.False(t, updateCalled, "an overdrafting merge must not persist")
}

func TestAbsCreateAbsenceFor_RejectsCompTimeAgainstLaterLedgerCapacity(t *testing.T) {
	svc, absRepo, _ := absSetupServiceWithSyncer()
	svc.settings = &wtmMockSettings{accountStart: "2026-06-01"}
	monthService := &absMonthServiceMock{capacity: 300, deduction: 480}
	svc.monthService = monthService
	createCalled := false
	absRepo.createFunc = func(_ context.Context, _ *activeModels.StaffAbsence) error {
		createCalled = true
		return nil
	}

	start := timezone.TodayDate().AddDays(1)
	_, err := svc.CreateAbsenceFor(context.Background(), 100, 200, nil, CreateAbsenceRequest{
		AbsenceType: activeModels.AbsenceTypeCompTime,
		DateStart:   start.String(),
		DateEnd:     start.String(),
		Note:        "Vor späterer Auszahlung",
	})

	require.ErrorIs(t, err, ErrCompTimeExceedsBalance)
	assert.Equal(t, start, monthService.capacityDate)
	assert.False(t, createCalled)
}

// The timeline capacity has no defined result beyond the carry-chain horizon,
// so far-future comp_time must be a client error, not a 500.
func TestAbsCreateAbsenceFor_RejectsFarFutureCompTime(t *testing.T) {
	svc, absRepo, _ := absSetupServiceWithSyncer()
	svc.settings = &wtmMockSettings{accountStart: "2026-06-01"}
	svc.monthService = &absMonthServiceMock{capacity: 10000, deduction: 480}
	createCalled := false
	absRepo.createFunc = func(_ context.Context, _ *activeModels.StaffAbsence) error {
		createCalled = true
		return nil
	}

	start := timezone.TodayDate().AddDays(420)
	_, err := svc.CreateAbsenceFor(context.Background(), 100, 200, nil, CreateAbsenceRequest{
		AbsenceType: activeModels.AbsenceTypeCompTime,
		DateStart:   start.String(),
		DateEnd:     start.String(),
		Note:        "Freizeitausgleich",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid comp_time absence")
	assert.Contains(t, err.Error(), "months ahead")
	assert.False(t, createCalled)
}
