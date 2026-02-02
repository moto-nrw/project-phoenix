package active

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Mocks for WorkSessionRepository (prefixed with ws)
// ============================================================================

type wsMockWorkSessionRepository struct {
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

func (m *wsMockWorkSessionRepository) Create(ctx context.Context, entity *activeModels.WorkSession) error {
	if m.createFunc != nil {
		return m.createFunc(ctx, entity)
	}
	return nil
}

func (m *wsMockWorkSessionRepository) FindByID(ctx context.Context, id any) (*activeModels.WorkSession, error) {
	if m.findByIDFunc != nil {
		return m.findByIDFunc(ctx, id)
	}
	return nil, sql.ErrNoRows
}

func (m *wsMockWorkSessionRepository) Update(ctx context.Context, entity *activeModels.WorkSession) error {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, entity)
	}
	return nil
}

func (m *wsMockWorkSessionRepository) Delete(ctx context.Context, id any) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, id)
	}
	return nil
}

func (m *wsMockWorkSessionRepository) List(ctx context.Context, options *base.QueryOptions) ([]*activeModels.WorkSession, error) {
	if m.listFunc != nil {
		return m.listFunc(ctx, options)
	}
	return nil, nil
}

func (m *wsMockWorkSessionRepository) GetByStaffAndDate(ctx context.Context, staffID int64, date time.Time) (*activeModels.WorkSession, error) {
	if m.getByStaffAndDateFunc != nil {
		return m.getByStaffAndDateFunc(ctx, staffID, date)
	}
	return nil, sql.ErrNoRows
}

func (m *wsMockWorkSessionRepository) GetCurrentByStaffID(ctx context.Context, staffID int64) (*activeModels.WorkSession, error) {
	if m.getCurrentByStaffIDFunc != nil {
		return m.getCurrentByStaffIDFunc(ctx, staffID)
	}
	return nil, sql.ErrNoRows
}

func (m *wsMockWorkSessionRepository) GetHistoryByStaffID(ctx context.Context, staffID int64, from, to time.Time) ([]*activeModels.WorkSession, error) {
	if m.getHistoryByStaffIDFunc != nil {
		return m.getHistoryByStaffIDFunc(ctx, staffID, from, to)
	}
	return nil, nil
}

func (m *wsMockWorkSessionRepository) GetOpenSessions(ctx context.Context, beforeDate time.Time) ([]*activeModels.WorkSession, error) {
	if m.getOpenSessionsFunc != nil {
		return m.getOpenSessionsFunc(ctx, beforeDate)
	}
	return nil, nil
}

func (m *wsMockWorkSessionRepository) GetTodayPresenceMap(ctx context.Context) (map[int64]string, error) {
	if m.getTodayPresenceMapFunc != nil {
		return m.getTodayPresenceMapFunc(ctx)
	}
	return nil, nil
}

func (m *wsMockWorkSessionRepository) CloseSession(ctx context.Context, id int64, checkOutTime time.Time, autoCheckedOut bool) error {
	if m.closeSessionFunc != nil {
		return m.closeSessionFunc(ctx, id, checkOutTime, autoCheckedOut)
	}
	return nil
}

func (m *wsMockWorkSessionRepository) UpdateBreakMinutes(ctx context.Context, id int64, breakMinutes int) error {
	if m.updateBreakMinutesFunc != nil {
		return m.updateBreakMinutesFunc(ctx, id, breakMinutes)
	}
	return nil
}

// ============================================================================
// Mock for WorkSessionBreakRepository (prefixed with ws)
// ============================================================================

type wsMockWorkSessionBreakRepository struct {
	createFunc               func(ctx context.Context, entity *activeModels.WorkSessionBreak) error
	findByIDFunc             func(ctx context.Context, id any) (*activeModels.WorkSessionBreak, error)
	updateFunc               func(ctx context.Context, entity *activeModels.WorkSessionBreak) error
	deleteFunc               func(ctx context.Context, id any) error
	listFunc                 func(ctx context.Context, options *base.QueryOptions) ([]*activeModels.WorkSessionBreak, error)
	getBySessionIDFunc       func(ctx context.Context, sessionID int64) ([]*activeModels.WorkSessionBreak, error)
	getActiveBySessionIDFunc func(ctx context.Context, sessionID int64) (*activeModels.WorkSessionBreak, error)
	endBreakFunc             func(ctx context.Context, id int64, endedAt time.Time, durationMinutes int) error
	updateDurationFunc       func(ctx context.Context, id int64, durationMinutes int, endedAt time.Time) error
}

func (m *wsMockWorkSessionBreakRepository) Create(ctx context.Context, entity *activeModels.WorkSessionBreak) error {
	if m.createFunc != nil {
		return m.createFunc(ctx, entity)
	}
	return nil
}

func (m *wsMockWorkSessionBreakRepository) FindByID(ctx context.Context, id any) (*activeModels.WorkSessionBreak, error) {
	if m.findByIDFunc != nil {
		return m.findByIDFunc(ctx, id)
	}
	return nil, sql.ErrNoRows
}

func (m *wsMockWorkSessionBreakRepository) Update(ctx context.Context, entity *activeModels.WorkSessionBreak) error {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, entity)
	}
	return nil
}

func (m *wsMockWorkSessionBreakRepository) Delete(ctx context.Context, id any) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, id)
	}
	return nil
}

func (m *wsMockWorkSessionBreakRepository) List(ctx context.Context, options *base.QueryOptions) ([]*activeModels.WorkSessionBreak, error) {
	if m.listFunc != nil {
		return m.listFunc(ctx, options)
	}
	return nil, nil
}

func (m *wsMockWorkSessionBreakRepository) GetBySessionID(ctx context.Context, sessionID int64) ([]*activeModels.WorkSessionBreak, error) {
	if m.getBySessionIDFunc != nil {
		return m.getBySessionIDFunc(ctx, sessionID)
	}
	return nil, nil
}

func (m *wsMockWorkSessionBreakRepository) GetActiveBySessionID(ctx context.Context, sessionID int64) (*activeModels.WorkSessionBreak, error) {
	if m.getActiveBySessionIDFunc != nil {
		return m.getActiveBySessionIDFunc(ctx, sessionID)
	}
	return nil, nil
}

func (m *wsMockWorkSessionBreakRepository) EndBreak(ctx context.Context, id int64, endedAt time.Time, durationMinutes int) error {
	if m.endBreakFunc != nil {
		return m.endBreakFunc(ctx, id, endedAt, durationMinutes)
	}
	return nil
}

func (m *wsMockWorkSessionBreakRepository) UpdateDuration(ctx context.Context, id int64, durationMinutes int, endedAt time.Time) error {
	if m.updateDurationFunc != nil {
		return m.updateDurationFunc(ctx, id, durationMinutes, endedAt)
	}
	return nil
}

// ============================================================================
// Mock for WorkSessionEditRepository (prefixed with ws)
// ============================================================================

type wsMockWorkSessionEditRepository struct {
	createBatchFunc       func(ctx context.Context, edits []*auditModels.WorkSessionEdit) error
	getBySessionIDFunc    func(ctx context.Context, sessionID int64) ([]*auditModels.WorkSessionEdit, error)
	countBySessionIDFunc  func(ctx context.Context, sessionID int64) (int, error)
	countBySessionIDsFunc func(ctx context.Context, sessionIDs []int64) (map[int64]int, error)
}

func (m *wsMockWorkSessionEditRepository) CreateBatch(ctx context.Context, edits []*auditModels.WorkSessionEdit) error {
	if m.createBatchFunc != nil {
		return m.createBatchFunc(ctx, edits)
	}
	return nil
}

func (m *wsMockWorkSessionEditRepository) GetBySessionID(ctx context.Context, sessionID int64) ([]*auditModels.WorkSessionEdit, error) {
	if m.getBySessionIDFunc != nil {
		return m.getBySessionIDFunc(ctx, sessionID)
	}
	return nil, nil
}

func (m *wsMockWorkSessionEditRepository) CountBySessionID(ctx context.Context, sessionID int64) (int, error) {
	if m.countBySessionIDFunc != nil {
		return m.countBySessionIDFunc(ctx, sessionID)
	}
	return 0, nil
}

func (m *wsMockWorkSessionEditRepository) CountBySessionIDs(ctx context.Context, sessionIDs []int64) (map[int64]int, error) {
	if m.countBySessionIDsFunc != nil {
		return m.countBySessionIDsFunc(ctx, sessionIDs)
	}
	return map[int64]int{}, nil
}

// ============================================================================
// Mock for StaffAbsenceRepository (prefixed with ws)
// ============================================================================

type wsMockStaffAbsenceRepository struct {
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

func (m *wsMockStaffAbsenceRepository) Create(ctx context.Context, entity *activeModels.StaffAbsence) error {
	if m.createFunc != nil {
		return m.createFunc(ctx, entity)
	}
	return nil
}

func (m *wsMockStaffAbsenceRepository) FindByID(ctx context.Context, id any) (*activeModels.StaffAbsence, error) {
	if m.findByIDFunc != nil {
		return m.findByIDFunc(ctx, id)
	}
	return nil, nil
}

func (m *wsMockStaffAbsenceRepository) Update(ctx context.Context, entity *activeModels.StaffAbsence) error {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, entity)
	}
	return nil
}

func (m *wsMockStaffAbsenceRepository) Delete(ctx context.Context, id any) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, id)
	}
	return nil
}

func (m *wsMockStaffAbsenceRepository) List(ctx context.Context, options *base.QueryOptions) ([]*activeModels.StaffAbsence, error) {
	if m.listFunc != nil {
		return m.listFunc(ctx, options)
	}
	return nil, nil
}

func (m *wsMockStaffAbsenceRepository) GetByStaffAndDateRange(ctx context.Context, staffID int64, from, to time.Time) ([]*activeModels.StaffAbsence, error) {
	if m.getByStaffAndDateRangeFunc != nil {
		return m.getByStaffAndDateRangeFunc(ctx, staffID, from, to)
	}
	return nil, nil
}

func (m *wsMockStaffAbsenceRepository) GetByStaffAndDate(ctx context.Context, staffID int64, date time.Time) (*activeModels.StaffAbsence, error) {
	if m.getByStaffAndDateFunc != nil {
		return m.getByStaffAndDateFunc(ctx, staffID, date)
	}
	return nil, nil
}

func (m *wsMockStaffAbsenceRepository) GetByDateRange(ctx context.Context, from, to time.Time) ([]*activeModels.StaffAbsence, error) {
	if m.getByDateRangeFunc != nil {
		return m.getByDateRangeFunc(ctx, from, to)
	}
	return nil, nil
}

func (m *wsMockStaffAbsenceRepository) GetTodayAbsenceMap(ctx context.Context) (map[int64]string, error) {
	if m.getTodayAbsenceMapFunc != nil {
		return m.getTodayAbsenceMapFunc(ctx)
	}
	return nil, nil
}

// ============================================================================
// Mock for GroupSupervisorRepository (prefixed with ws)
// ============================================================================

type wsMockGroupSupervisorRepository struct {
	createFunc                          func(ctx context.Context, entity *activeModels.GroupSupervisor) error
	findByIDFunc                        func(ctx context.Context, id any) (*activeModels.GroupSupervisor, error)
	updateFunc                          func(ctx context.Context, entity *activeModels.GroupSupervisor) error
	deleteFunc                          func(ctx context.Context, id any) error
	listFunc                            func(ctx context.Context, options *base.QueryOptions) ([]*activeModels.GroupSupervisor, error)
	findByStaffIDFunc                   func(ctx context.Context, staffID int64) ([]*activeModels.GroupSupervisor, error)
	findActiveByStaffIDFunc             func(ctx context.Context, staffID int64) ([]*activeModels.GroupSupervisor, error)
	findByActiveGroupIDFunc             func(ctx context.Context, activeGroupID int64, activeOnly bool) ([]*activeModels.GroupSupervisor, error)
	findByActiveGroupIDsFunc            func(ctx context.Context, activeGroupIDs []int64, activeOnly bool) ([]*activeModels.GroupSupervisor, error)
	endSupervisionFunc                  func(ctx context.Context, id int64) error
	getStaffIDsWithSupervisionTodayFunc func(ctx context.Context) ([]int64, error)
	endAllActiveByStaffIDFunc           func(ctx context.Context, staffID int64) (int, error)
}

func (m *wsMockGroupSupervisorRepository) Create(ctx context.Context, entity *activeModels.GroupSupervisor) error {
	if m.createFunc != nil {
		return m.createFunc(ctx, entity)
	}
	return nil
}

func (m *wsMockGroupSupervisorRepository) FindByID(ctx context.Context, id any) (*activeModels.GroupSupervisor, error) {
	if m.findByIDFunc != nil {
		return m.findByIDFunc(ctx, id)
	}
	return nil, nil
}

func (m *wsMockGroupSupervisorRepository) Update(ctx context.Context, entity *activeModels.GroupSupervisor) error {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, entity)
	}
	return nil
}

func (m *wsMockGroupSupervisorRepository) Delete(ctx context.Context, id any) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, id)
	}
	return nil
}

func (m *wsMockGroupSupervisorRepository) List(ctx context.Context, options *base.QueryOptions) ([]*activeModels.GroupSupervisor, error) {
	if m.listFunc != nil {
		return m.listFunc(ctx, options)
	}
	return nil, nil
}

func (m *wsMockGroupSupervisorRepository) FindByStaffID(ctx context.Context, staffID int64) ([]*activeModels.GroupSupervisor, error) {
	if m.findByStaffIDFunc != nil {
		return m.findByStaffIDFunc(ctx, staffID)
	}
	return nil, nil
}

func (m *wsMockGroupSupervisorRepository) FindActiveByStaffID(ctx context.Context, staffID int64) ([]*activeModels.GroupSupervisor, error) {
	if m.findActiveByStaffIDFunc != nil {
		return m.findActiveByStaffIDFunc(ctx, staffID)
	}
	return nil, nil
}

func (m *wsMockGroupSupervisorRepository) FindByActiveGroupID(ctx context.Context, activeGroupID int64, activeOnly bool) ([]*activeModels.GroupSupervisor, error) {
	if m.findByActiveGroupIDFunc != nil {
		return m.findByActiveGroupIDFunc(ctx, activeGroupID, activeOnly)
	}
	return nil, nil
}

func (m *wsMockGroupSupervisorRepository) FindByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64, activeOnly bool) ([]*activeModels.GroupSupervisor, error) {
	if m.findByActiveGroupIDsFunc != nil {
		return m.findByActiveGroupIDsFunc(ctx, activeGroupIDs, activeOnly)
	}
	return nil, nil
}

func (m *wsMockGroupSupervisorRepository) EndSupervision(ctx context.Context, id int64) error {
	if m.endSupervisionFunc != nil {
		return m.endSupervisionFunc(ctx, id)
	}
	return nil
}

func (m *wsMockGroupSupervisorRepository) GetStaffIDsWithSupervisionToday(ctx context.Context) ([]int64, error) {
	if m.getStaffIDsWithSupervisionTodayFunc != nil {
		return m.getStaffIDsWithSupervisionTodayFunc(ctx)
	}
	return nil, nil
}

func (m *wsMockGroupSupervisorRepository) EndAllActiveByStaffID(ctx context.Context, staffID int64) (int, error) {
	if m.endAllActiveByStaffIDFunc != nil {
		return m.endAllActiveByStaffIDFunc(ctx, staffID)
	}
	return 0, nil
}

// ============================================================================
// Helper to create test service
// ============================================================================

func wsCreateTestService() (*workSessionService, *wsMockWorkSessionRepository, *wsMockWorkSessionBreakRepository, *wsMockWorkSessionEditRepository, *wsMockGroupSupervisorRepository) {
	sessionRepo := &wsMockWorkSessionRepository{}
	breakRepo := &wsMockWorkSessionBreakRepository{}
	auditRepo := &wsMockWorkSessionEditRepository{}
	absenceRepo := &wsMockStaffAbsenceRepository{}
	supervisorRepo := &wsMockGroupSupervisorRepository{}

	service := &workSessionService{
		repo:           sessionRepo,
		breakRepo:      breakRepo,
		auditRepo:      auditRepo,
		absenceRepo:    absenceRepo,
		supervisorRepo: supervisorRepo,
	}

	return service, sessionRepo, breakRepo, auditRepo, supervisorRepo
}

// ============================================================================
// CheckIn Tests
// ============================================================================

func TestWSCheckIn_Success(t *testing.T) {
	svc, sessionRepo, _, _, _ := wsCreateTestService()
	ctx := context.Background()
	staffID := int64(100)

	sessionRepo.getByStaffAndDateFunc = func(_ context.Context, _ int64, _ time.Time) (*activeModels.WorkSession, error) {
		return nil, sql.ErrNoRows
	}

	sessionRepo.createFunc = func(_ context.Context, entity *activeModels.WorkSession) error {
		entity.ID = 10
		return nil
	}

	session, err := svc.CheckIn(ctx, staffID, activeModels.WorkSessionStatusPresent)
	require.NoError(t, err)
	require.NotNil(t, session)
	assert.Equal(t, staffID, session.StaffID)
	assert.Equal(t, activeModels.WorkSessionStatusPresent, session.Status)
	assert.Nil(t, session.CheckOutTime)
}

func TestWSCheckIn_DefaultStatus(t *testing.T) {
	svc, sessionRepo, _, _, _ := wsCreateTestService()
	ctx := context.Background()

	sessionRepo.getByStaffAndDateFunc = func(_ context.Context, _ int64, _ time.Time) (*activeModels.WorkSession, error) {
		return nil, sql.ErrNoRows
	}
	sessionRepo.createFunc = func(_ context.Context, entity *activeModels.WorkSession) error {
		assert.Equal(t, activeModels.WorkSessionStatusPresent, entity.Status)
		entity.ID = 10
		return nil
	}

	session, err := svc.CheckIn(ctx, 100, "")
	require.NoError(t, err)
	assert.Equal(t, activeModels.WorkSessionStatusPresent, session.Status)
}

func TestWSCheckIn_AlreadyCheckedIn(t *testing.T) {
	svc, sessionRepo, _, _, _ := wsCreateTestService()

	sessionRepo.getByStaffAndDateFunc = func(_ context.Context, _ int64, _ time.Time) (*activeModels.WorkSession, error) {
		return &activeModels.WorkSession{
			Model:       base.Model{ID: 1},
			StaffID:     100,
			CheckInTime: time.Now().Add(-2 * time.Hour),
		}, nil
	}

	session, err := svc.CheckIn(context.Background(), 100, activeModels.WorkSessionStatusPresent)
	require.Error(t, err)
	assert.Nil(t, session)
	assert.Contains(t, err.Error(), "already checked in")
}

func TestWSCheckIn_ReopenCheckedOutSession(t *testing.T) {
	svc, sessionRepo, _, _, _ := wsCreateTestService()
	checkOut := time.Now().Add(-1 * time.Hour)

	sessionRepo.getByStaffAndDateFunc = func(_ context.Context, _ int64, _ time.Time) (*activeModels.WorkSession, error) {
		return &activeModels.WorkSession{
			Model:          base.Model{ID: 1},
			StaffID:        100,
			CheckInTime:    time.Now().Add(-4 * time.Hour),
			CheckOutTime:   &checkOut,
			AutoCheckedOut: true,
			Status:         activeModels.WorkSessionStatusPresent,
			Date:           time.Now().Truncate(24 * time.Hour),
			CreatedBy:      100,
		}, nil
	}

	sessionRepo.updateFunc = func(_ context.Context, entity *activeModels.WorkSession) error {
		assert.Nil(t, entity.CheckOutTime)
		assert.False(t, entity.AutoCheckedOut)
		return nil
	}

	session, err := svc.CheckIn(context.Background(), 100, activeModels.WorkSessionStatusPresent)
	require.NoError(t, err)
	require.NotNil(t, session)
	assert.Nil(t, session.CheckOutTime)
}

func TestWSCheckIn_InvalidStatus(t *testing.T) {
	svc, _, _, _, _ := wsCreateTestService()

	session, err := svc.CheckIn(context.Background(), 100, "invalid_status")
	require.Error(t, err)
	assert.Nil(t, session)
	assert.Contains(t, err.Error(), "status must be")
}

func TestWSCheckIn_RepoError(t *testing.T) {
	svc, sessionRepo, _, _, _ := wsCreateTestService()

	sessionRepo.getByStaffAndDateFunc = func(_ context.Context, _ int64, _ time.Time) (*activeModels.WorkSession, error) {
		return nil, errors.New("database error")
	}

	session, err := svc.CheckIn(context.Background(), 100, activeModels.WorkSessionStatusPresent)
	require.Error(t, err)
	assert.Nil(t, session)
	assert.Contains(t, err.Error(), "failed to check existing session")
}

// ============================================================================
// CheckOut Tests
// ============================================================================

func TestWSCheckOut_Success(t *testing.T) {
	svc, sessionRepo, breakRepo, _, supervisorRepo := wsCreateTestService()
	ctx := context.Background()
	staffID := int64(100)

	sessionRepo.getCurrentByStaffIDFunc = func(_ context.Context, _ int64) (*activeModels.WorkSession, error) {
		return &activeModels.WorkSession{
			Model:       base.Model{ID: 1},
			StaffID:     staffID,
			CheckInTime: time.Now().Add(-4 * time.Hour),
		}, nil
	}

	breakRepo.getActiveBySessionIDFunc = func(_ context.Context, _ int64) (*activeModels.WorkSessionBreak, error) {
		return nil, nil
	}

	sessionRepo.closeSessionFunc = func(_ context.Context, _ int64, _ time.Time, autoCheckedOut bool) error {
		assert.False(t, autoCheckedOut)
		return nil
	}

	supervisorRepo.endAllActiveByStaffIDFunc = func(_ context.Context, _ int64) (int, error) {
		return 1, nil
	}

	checkOut := time.Now()
	sessionRepo.findByIDFunc = func(_ context.Context, id any) (*activeModels.WorkSession, error) {
		return &activeModels.WorkSession{
			Model:        base.Model{ID: id.(int64)},
			StaffID:      staffID,
			CheckInTime:  time.Now().Add(-4 * time.Hour),
			CheckOutTime: &checkOut,
		}, nil
	}

	session, err := svc.CheckOut(ctx, staffID)
	require.NoError(t, err)
	require.NotNil(t, session)
	assert.NotNil(t, session.CheckOutTime)
}

func TestWSCheckOut_NoActiveSession(t *testing.T) {
	svc, sessionRepo, _, _, _ := wsCreateTestService()

	sessionRepo.getCurrentByStaffIDFunc = func(_ context.Context, _ int64) (*activeModels.WorkSession, error) {
		return nil, sql.ErrNoRows
	}

	session, err := svc.CheckOut(context.Background(), 100)
	require.Error(t, err)
	assert.Nil(t, session)
	assert.Contains(t, err.Error(), "no active session found")
}

func TestWSCheckOut_NilSession(t *testing.T) {
	svc, sessionRepo, _, _, _ := wsCreateTestService()

	sessionRepo.getCurrentByStaffIDFunc = func(_ context.Context, _ int64) (*activeModels.WorkSession, error) {
		return nil, nil
	}

	session, err := svc.CheckOut(context.Background(), 100)
	require.Error(t, err)
	assert.Nil(t, session)
	assert.Contains(t, err.Error(), "no active session found")
}

func TestWSCheckOut_WithActiveBreak(t *testing.T) {
	svc, sessionRepo, breakRepo, _, supervisorRepo := wsCreateTestService()
	staffID := int64(100)

	sessionRepo.getCurrentByStaffIDFunc = func(_ context.Context, _ int64) (*activeModels.WorkSession, error) {
		return &activeModels.WorkSession{
			Model:       base.Model{ID: 1},
			StaffID:     staffID,
			CheckInTime: time.Now().Add(-4 * time.Hour),
		}, nil
	}

	breakRepo.getActiveBySessionIDFunc = func(_ context.Context, _ int64) (*activeModels.WorkSessionBreak, error) {
		return &activeModels.WorkSessionBreak{
			Model:     base.Model{ID: 1},
			SessionID: 1,
			StartedAt: time.Now().Add(-30 * time.Minute),
		}, nil
	}

	breakRepo.endBreakFunc = func(_ context.Context, _ int64, _ time.Time, _ int) error {
		return nil
	}

	breakRepo.getBySessionIDFunc = func(_ context.Context, _ int64) ([]*activeModels.WorkSessionBreak, error) {
		return []*activeModels.WorkSessionBreak{}, nil
	}

	sessionRepo.updateBreakMinutesFunc = func(_ context.Context, _ int64, _ int) error {
		return nil
	}

	sessionRepo.closeSessionFunc = func(_ context.Context, _ int64, _ time.Time, _ bool) error {
		return nil
	}

	supervisorRepo.endAllActiveByStaffIDFunc = func(_ context.Context, _ int64) (int, error) {
		return 0, nil
	}

	checkOut := time.Now()
	sessionRepo.findByIDFunc = func(_ context.Context, id any) (*activeModels.WorkSession, error) {
		return &activeModels.WorkSession{
			Model:        base.Model{ID: id.(int64)},
			StaffID:      staffID,
			CheckInTime:  time.Now().Add(-4 * time.Hour),
			CheckOutTime: &checkOut,
		}, nil
	}

	session, err := svc.CheckOut(context.Background(), staffID)
	require.NoError(t, err)
	require.NotNil(t, session)
	assert.NotNil(t, session.CheckOutTime)
}

// ============================================================================
// StartBreak Tests
// ============================================================================

func TestWSStartBreak_Success(t *testing.T) {
	svc, sessionRepo, breakRepo, _, _ := wsCreateTestService()
	staffID := int64(100)

	sessionRepo.getCurrentByStaffIDFunc = func(_ context.Context, _ int64) (*activeModels.WorkSession, error) {
		return &activeModels.WorkSession{
			Model:       base.Model{ID: 50},
			StaffID:     staffID,
			CheckInTime: time.Now().Add(-2 * time.Hour),
		}, nil
	}

	breakRepo.getActiveBySessionIDFunc = func(_ context.Context, _ int64) (*activeModels.WorkSessionBreak, error) {
		return nil, nil
	}

	breakRepo.createFunc = func(_ context.Context, entity *activeModels.WorkSessionBreak) error {
		entity.ID = 10
		return nil
	}

	brk, err := svc.StartBreak(context.Background(), staffID)
	require.NoError(t, err)
	require.NotNil(t, brk)
	assert.Equal(t, int64(50), brk.SessionID)
}

func TestWSStartBreak_NoActiveSession(t *testing.T) {
	svc, sessionRepo, _, _, _ := wsCreateTestService()

	sessionRepo.getCurrentByStaffIDFunc = func(_ context.Context, _ int64) (*activeModels.WorkSession, error) {
		return nil, sql.ErrNoRows
	}

	brk, err := svc.StartBreak(context.Background(), 100)
	require.Error(t, err)
	assert.Nil(t, brk)
	assert.Contains(t, err.Error(), "no active session found")
}

func TestWSStartBreak_AlreadyOnBreak(t *testing.T) {
	svc, sessionRepo, breakRepo, _, _ := wsCreateTestService()

	sessionRepo.getCurrentByStaffIDFunc = func(_ context.Context, _ int64) (*activeModels.WorkSession, error) {
		return &activeModels.WorkSession{
			Model:       base.Model{ID: 1},
			StaffID:     100,
			CheckInTime: time.Now().Add(-2 * time.Hour),
		}, nil
	}

	breakRepo.getActiveBySessionIDFunc = func(_ context.Context, _ int64) (*activeModels.WorkSessionBreak, error) {
		return &activeModels.WorkSessionBreak{
			Model:     base.Model{ID: 1},
			SessionID: 1,
			StartedAt: time.Now().Add(-15 * time.Minute),
		}, nil
	}

	brk, err := svc.StartBreak(context.Background(), 100)
	require.Error(t, err)
	assert.Nil(t, brk)
	assert.Contains(t, err.Error(), "break already active")
}

// ============================================================================
// EndBreak Tests
// ============================================================================

func TestWSEndBreak_Success(t *testing.T) {
	svc, sessionRepo, breakRepo, _, _ := wsCreateTestService()
	staffID := int64(100)

	sessionRepo.getCurrentByStaffIDFunc = func(_ context.Context, _ int64) (*activeModels.WorkSession, error) {
		return &activeModels.WorkSession{
			Model:       base.Model{ID: 1},
			StaffID:     staffID,
			CheckInTime: time.Now().Add(-2 * time.Hour),
		}, nil
	}

	breakRepo.getActiveBySessionIDFunc = func(_ context.Context, _ int64) (*activeModels.WorkSessionBreak, error) {
		return &activeModels.WorkSessionBreak{
			Model:     base.Model{ID: 1},
			SessionID: 1,
			StartedAt: time.Now().Add(-30 * time.Minute),
		}, nil
	}

	breakRepo.endBreakFunc = func(_ context.Context, _ int64, _ time.Time, _ int) error {
		return nil
	}

	breakRepo.getBySessionIDFunc = func(_ context.Context, _ int64) ([]*activeModels.WorkSessionBreak, error) {
		return []*activeModels.WorkSessionBreak{}, nil
	}

	sessionRepo.updateBreakMinutesFunc = func(_ context.Context, _ int64, _ int) error {
		return nil
	}

	sessionRepo.findByIDFunc = func(_ context.Context, id any) (*activeModels.WorkSession, error) {
		return &activeModels.WorkSession{
			Model:        base.Model{ID: id.(int64)},
			StaffID:      staffID,
			BreakMinutes: 30,
		}, nil
	}

	session, err := svc.EndBreak(context.Background(), staffID)
	require.NoError(t, err)
	require.NotNil(t, session)
}

func TestWSEndBreak_NoActiveBreak(t *testing.T) {
	svc, sessionRepo, breakRepo, _, _ := wsCreateTestService()

	sessionRepo.getCurrentByStaffIDFunc = func(_ context.Context, _ int64) (*activeModels.WorkSession, error) {
		return &activeModels.WorkSession{
			Model:       base.Model{ID: 1},
			StaffID:     100,
			CheckInTime: time.Now().Add(-2 * time.Hour),
		}, nil
	}

	breakRepo.getActiveBySessionIDFunc = func(_ context.Context, _ int64) (*activeModels.WorkSessionBreak, error) {
		return nil, nil
	}

	session, err := svc.EndBreak(context.Background(), 100)
	require.Error(t, err)
	assert.Nil(t, session)
	assert.Contains(t, err.Error(), "no active break found")
}

func TestWSEndBreak_NoActiveSession(t *testing.T) {
	svc, sessionRepo, _, _, _ := wsCreateTestService()

	sessionRepo.getCurrentByStaffIDFunc = func(_ context.Context, _ int64) (*activeModels.WorkSession, error) {
		return nil, sql.ErrNoRows
	}

	session, err := svc.EndBreak(context.Background(), 100)
	require.Error(t, err)
	assert.Nil(t, session)
	assert.Contains(t, err.Error(), "no active session found")
}

// ============================================================================
// GetCurrentSession Tests
// ============================================================================

func TestWSGetCurrentSession_Found(t *testing.T) {
	svc, sessionRepo, _, _, _ := wsCreateTestService()
	staffID := int64(100)

	sessionRepo.getCurrentByStaffIDFunc = func(_ context.Context, _ int64) (*activeModels.WorkSession, error) {
		return &activeModels.WorkSession{
			Model:   base.Model{ID: 1},
			StaffID: staffID,
		}, nil
	}

	session, err := svc.GetCurrentSession(context.Background(), staffID)
	require.NoError(t, err)
	require.NotNil(t, session)
	assert.Equal(t, staffID, session.StaffID)
}

func TestWSGetCurrentSession_NotFound(t *testing.T) {
	svc, sessionRepo, _, _, _ := wsCreateTestService()

	sessionRepo.getCurrentByStaffIDFunc = func(_ context.Context, _ int64) (*activeModels.WorkSession, error) {
		return nil, sql.ErrNoRows
	}

	session, err := svc.GetCurrentSession(context.Background(), 100)
	require.NoError(t, err)
	assert.Nil(t, session)
}

// ============================================================================
// GetHistory Tests
// ============================================================================

func TestWSGetHistory_Success(t *testing.T) {
	svc, sessionRepo, breakRepo, auditRepo, _ := wsCreateTestService()
	staffID := int64(100)
	from := time.Now().AddDate(0, 0, -7)
	to := time.Now()

	checkIn := time.Now().Add(-8 * time.Hour)
	checkOut := time.Now().Add(-2 * time.Hour)
	sessionRepo.getHistoryByStaffIDFunc = func(_ context.Context, _ int64, _, _ time.Time) ([]*activeModels.WorkSession, error) {
		return []*activeModels.WorkSession{
			{
				Model:        base.Model{ID: 1},
				StaffID:      staffID,
				CheckInTime:  checkIn,
				CheckOutTime: &checkOut,
				BreakMinutes: 30,
			},
		}, nil
	}

	auditRepo.countBySessionIDsFunc = func(_ context.Context, _ []int64) (map[int64]int, error) {
		return map[int64]int{1: 2}, nil
	}

	breakRepo.getBySessionIDFunc = func(_ context.Context, _ int64) ([]*activeModels.WorkSessionBreak, error) {
		return []*activeModels.WorkSessionBreak{}, nil
	}

	responses, err := svc.GetHistory(context.Background(), staffID, from, to)
	require.NoError(t, err)
	require.Len(t, responses, 1)
	assert.Equal(t, 2, responses[0].EditCount)
}

func TestWSGetHistory_RepoError(t *testing.T) {
	svc, sessionRepo, _, _, _ := wsCreateTestService()

	sessionRepo.getHistoryByStaffIDFunc = func(_ context.Context, _ int64, _, _ time.Time) ([]*activeModels.WorkSession, error) {
		return nil, errors.New("database error")
	}

	responses, err := svc.GetHistory(context.Background(), 100, time.Now(), time.Now())
	require.Error(t, err)
	assert.Nil(t, responses)
}

// ============================================================================
// GetSessionBreaks Tests
// ============================================================================

func TestWSGetSessionBreaks_Success(t *testing.T) {
	svc, _, breakRepo, _, _ := wsCreateTestService()

	breakRepo.getBySessionIDFunc = func(_ context.Context, _ int64) ([]*activeModels.WorkSessionBreak, error) {
		return []*activeModels.WorkSessionBreak{
			{Model: base.Model{ID: 1}},
			{Model: base.Model{ID: 2}},
		}, nil
	}

	breaks, err := svc.GetSessionBreaks(context.Background(), 1)
	require.NoError(t, err)
	assert.Len(t, breaks, 2)
}

// ============================================================================
// GetSessionEdits Tests
// ============================================================================

func TestWSGetSessionEdits_Success(t *testing.T) {
	svc, _, _, auditRepo, _ := wsCreateTestService()

	auditRepo.getBySessionIDFunc = func(_ context.Context, _ int64) ([]*auditModels.WorkSessionEdit, error) {
		return []*auditModels.WorkSessionEdit{
			{SessionID: 1, FieldName: "check_in_time"},
		}, nil
	}

	edits, err := svc.GetSessionEdits(context.Background(), 1)
	require.NoError(t, err)
	assert.Len(t, edits, 1)
}

// ============================================================================
// GetTodayPresenceMap Tests
// ============================================================================

func TestWSGetTodayPresenceMap_Success(t *testing.T) {
	svc, sessionRepo, _, _, _ := wsCreateTestService()

	sessionRepo.getTodayPresenceMapFunc = func(_ context.Context) (map[int64]string, error) {
		return map[int64]string{
			1: activeModels.WorkSessionStatusPresent,
			2: activeModels.WorkSessionStatusHomeOffice,
		}, nil
	}

	presenceMap, err := svc.GetTodayPresenceMap(context.Background())
	require.NoError(t, err)
	assert.Len(t, presenceMap, 2)
	assert.Equal(t, activeModels.WorkSessionStatusPresent, presenceMap[1])
}

// ============================================================================
// CleanupOpenSessions Tests
// ============================================================================

func TestWSCleanupOpenSessions_Success(t *testing.T) {
	svc, sessionRepo, _, _, _ := wsCreateTestService()
	yesterday := time.Now().AddDate(0, 0, -1).Truncate(24 * time.Hour)

	sessionRepo.getOpenSessionsFunc = func(_ context.Context, _ time.Time) ([]*activeModels.WorkSession, error) {
		return []*activeModels.WorkSession{
			{Model: base.Model{ID: 1}, Date: yesterday},
			{Model: base.Model{ID: 2}, Date: yesterday},
		}, nil
	}

	sessionRepo.closeSessionFunc = func(_ context.Context, _ int64, _ time.Time, autoCheckedOut bool) error {
		assert.True(t, autoCheckedOut)
		return nil
	}

	count, err := svc.CleanupOpenSessions(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestWSCleanupOpenSessions_NoOpenSessions(t *testing.T) {
	svc, sessionRepo, _, _, _ := wsCreateTestService()

	sessionRepo.getOpenSessionsFunc = func(_ context.Context, _ time.Time) ([]*activeModels.WorkSession, error) {
		return nil, nil
	}

	count, err := svc.CleanupOpenSessions(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

// ============================================================================
// EnsureCheckedIn Tests
// ============================================================================

func TestWSEnsureCheckedIn_AlreadyActive(t *testing.T) {
	svc, sessionRepo, _, _, _ := wsCreateTestService()
	staffID := int64(100)

	sessionRepo.getCurrentByStaffIDFunc = func(_ context.Context, _ int64) (*activeModels.WorkSession, error) {
		return &activeModels.WorkSession{
			Model:       base.Model{ID: 1},
			StaffID:     staffID,
			CheckInTime: time.Now().Add(-2 * time.Hour),
		}, nil
	}

	session, err := svc.EnsureCheckedIn(context.Background(), staffID)
	require.NoError(t, err)
	require.NotNil(t, session)
	assert.Equal(t, staffID, session.StaffID)
}

func TestWSEnsureCheckedIn_AlreadyCheckedOutToday(t *testing.T) {
	svc, sessionRepo, _, _, _ := wsCreateTestService()
	staffID := int64(100)
	checkOut := time.Now().Add(-1 * time.Hour)

	sessionRepo.getCurrentByStaffIDFunc = func(_ context.Context, _ int64) (*activeModels.WorkSession, error) {
		return nil, sql.ErrNoRows
	}

	sessionRepo.getByStaffAndDateFunc = func(_ context.Context, _ int64, _ time.Time) (*activeModels.WorkSession, error) {
		return &activeModels.WorkSession{
			Model:        base.Model{ID: 1},
			StaffID:      staffID,
			CheckOutTime: &checkOut,
		}, nil
	}

	session, err := svc.EnsureCheckedIn(context.Background(), staffID)
	require.NoError(t, err)
	assert.Nil(t, session) // Should return nil when already checked out today
}

func TestWSEnsureCheckedIn_CreatesNew(t *testing.T) {
	svc, sessionRepo, _, _, _ := wsCreateTestService()
	staffID := int64(100)

	sessionRepo.getCurrentByStaffIDFunc = func(_ context.Context, _ int64) (*activeModels.WorkSession, error) {
		return nil, sql.ErrNoRows
	}

	callCount := 0
	sessionRepo.getByStaffAndDateFunc = func(_ context.Context, _ int64, _ time.Time) (*activeModels.WorkSession, error) {
		callCount++
		if callCount == 1 {
			// First call from EnsureCheckedIn
			return nil, sql.ErrNoRows
		}
		// Second call from CheckIn
		return nil, sql.ErrNoRows
	}

	sessionRepo.createFunc = func(_ context.Context, entity *activeModels.WorkSession) error {
		entity.ID = 10
		return nil
	}

	session, err := svc.EnsureCheckedIn(context.Background(), staffID)
	require.NoError(t, err)
	require.NotNil(t, session)
}

// ============================================================================
// UpdateSession Tests
// ============================================================================

func TestWSUpdateSession_CheckInTimeChange(t *testing.T) {
	svc, sessionRepo, _, auditRepo, _ := wsCreateTestService()
	staffID := int64(100)
	sessionID := int64(100)

	oldCheckIn := time.Now().Add(-8 * time.Hour)
	newCheckIn := time.Now().Add(-7 * time.Hour)

	sessionRepo.findByIDFunc = func(_ context.Context, _ any) (*activeModels.WorkSession, error) {
		return &activeModels.WorkSession{
			Model:       base.Model{ID: sessionID},
			StaffID:     staffID,
			CheckInTime: oldCheckIn,
			Status:      activeModels.WorkSessionStatusPresent,
			Date:        time.Now().Truncate(24 * time.Hour),
			CreatedBy:   staffID,
		}, nil
	}

	sessionRepo.updateFunc = func(_ context.Context, _ *activeModels.WorkSession) error {
		return nil
	}

	auditRepo.createBatchFunc = func(_ context.Context, edits []*auditModels.WorkSessionEdit) error {
		assert.Len(t, edits, 1)
		assert.Equal(t, auditModels.FieldCheckInTime, edits[0].FieldName)
		return nil
	}

	updates := SessionUpdateRequest{
		CheckInTime: &newCheckIn,
	}

	session, err := svc.UpdateSession(context.Background(), staffID, sessionID, updates)
	require.NoError(t, err)
	require.NotNil(t, session)
}

func TestWSUpdateSession_OwnershipFails(t *testing.T) {
	svc, sessionRepo, _, _, _ := wsCreateTestService()

	sessionRepo.findByIDFunc = func(_ context.Context, _ any) (*activeModels.WorkSession, error) {
		return &activeModels.WorkSession{
			Model:   base.Model{ID: 1},
			StaffID: 200, // Different staff
		}, nil
	}

	updates := SessionUpdateRequest{
		Notes: wsStrPtr("test"),
	}

	session, err := svc.UpdateSession(context.Background(), 100, 1, updates)
	require.Error(t, err)
	assert.Nil(t, session)
	assert.Contains(t, err.Error(), "can only update own sessions")
}

func TestWSUpdateSession_NotFound(t *testing.T) {
	svc, sessionRepo, _, _, _ := wsCreateTestService()

	sessionRepo.findByIDFunc = func(_ context.Context, _ any) (*activeModels.WorkSession, error) {
		return nil, sql.ErrNoRows
	}

	session, err := svc.UpdateSession(context.Background(), 100, 999, SessionUpdateRequest{})
	require.Error(t, err)
	assert.Nil(t, session)
	assert.Contains(t, err.Error(), "session not found")
}

func TestWSUpdateSession_BreakDurationUpdate(t *testing.T) {
	svc, sessionRepo, breakRepo, auditRepo, _ := wsCreateTestService()
	staffID := int64(100)
	sessionID := int64(100)

	endedAt := time.Now().Add(-1 * time.Hour)
	sessionRepo.findByIDFunc = func(_ context.Context, _ any) (*activeModels.WorkSession, error) {
		return &activeModels.WorkSession{
			Model:       base.Model{ID: sessionID},
			StaffID:     staffID,
			CheckInTime: time.Now().Add(-8 * time.Hour),
			Status:      activeModels.WorkSessionStatusPresent,
			Date:        time.Now().Truncate(24 * time.Hour),
			CreatedBy:   staffID,
		}, nil
	}

	breakRepo.getBySessionIDFunc = func(_ context.Context, _ int64) ([]*activeModels.WorkSessionBreak, error) {
		return []*activeModels.WorkSessionBreak{
			{
				Model:           base.Model{ID: 1},
				SessionID:       sessionID,
				StartedAt:       time.Now().Add(-2 * time.Hour),
				EndedAt:         &endedAt,
				DurationMinutes: 30,
			},
		}, nil
	}

	breakRepo.updateDurationFunc = func(_ context.Context, _ int64, dur int, _ time.Time) error {
		assert.Equal(t, 45, dur)
		return nil
	}

	sessionRepo.updateBreakMinutesFunc = func(_ context.Context, _ int64, _ int) error {
		return nil
	}

	sessionRepo.updateFunc = func(_ context.Context, _ *activeModels.WorkSession) error {
		return nil
	}

	auditRepo.createBatchFunc = func(_ context.Context, edits []*auditModels.WorkSessionEdit) error {
		assert.Len(t, edits, 1)
		assert.Equal(t, auditModels.FieldBreakDuration, edits[0].FieldName)
		return nil
	}

	updates := SessionUpdateRequest{
		Breaks: []BreakDurationUpdate{
			{ID: 1, DurationMinutes: 45},
		},
	}

	session, err := svc.UpdateSession(context.Background(), staffID, sessionID, updates)
	require.NoError(t, err)
	require.NotNil(t, session)
}

func TestWSUpdateSession_BreakNotBelongsToSession(t *testing.T) {
	svc, sessionRepo, breakRepo, _, _ := wsCreateTestService()
	staffID := int64(100)
	sessionID := int64(100)

	sessionRepo.findByIDFunc = func(_ context.Context, _ any) (*activeModels.WorkSession, error) {
		return &activeModels.WorkSession{
			Model:       base.Model{ID: sessionID},
			StaffID:     staffID,
			CheckInTime: time.Now().Add(-8 * time.Hour),
			Status:      activeModels.WorkSessionStatusPresent,
			Date:        time.Now().Truncate(24 * time.Hour),
			CreatedBy:   staffID,
		}, nil
	}

	breakRepo.getBySessionIDFunc = func(_ context.Context, _ int64) ([]*activeModels.WorkSessionBreak, error) {
		return []*activeModels.WorkSessionBreak{}, nil // No breaks for this session
	}

	updates := SessionUpdateRequest{
		Breaks: []BreakDurationUpdate{
			{ID: 999, DurationMinutes: 45}, // Break doesn't belong to session
		},
	}

	session, err := svc.UpdateSession(context.Background(), staffID, sessionID, updates)
	require.Error(t, err)
	assert.Nil(t, session)
	assert.Contains(t, err.Error(), "does not belong to this session")
}

func TestWSUpdateSession_CannotEditActiveBreak(t *testing.T) {
	svc, sessionRepo, breakRepo, _, _ := wsCreateTestService()
	staffID := int64(100)
	sessionID := int64(100)

	sessionRepo.findByIDFunc = func(_ context.Context, _ any) (*activeModels.WorkSession, error) {
		return &activeModels.WorkSession{
			Model:       base.Model{ID: sessionID},
			StaffID:     staffID,
			CheckInTime: time.Now().Add(-8 * time.Hour),
			Status:      activeModels.WorkSessionStatusPresent,
			Date:        time.Now().Truncate(24 * time.Hour),
			CreatedBy:   staffID,
		}, nil
	}

	breakRepo.getBySessionIDFunc = func(_ context.Context, _ int64) ([]*activeModels.WorkSessionBreak, error) {
		return []*activeModels.WorkSessionBreak{
			{
				Model:     base.Model{ID: 1},
				SessionID: sessionID,
				StartedAt: time.Now().Add(-30 * time.Minute),
				EndedAt:   nil, // Active break
			},
		}, nil
	}

	updates := SessionUpdateRequest{
		Breaks: []BreakDurationUpdate{
			{ID: 1, DurationMinutes: 45},
		},
	}

	session, err := svc.UpdateSession(context.Background(), staffID, sessionID, updates)
	require.Error(t, err)
	assert.Nil(t, session)
	assert.Contains(t, err.Error(), "cannot edit duration of an active break")
}

// Helper for string pointers
func wsStrPtr(s string) *string {
	return &s
}
