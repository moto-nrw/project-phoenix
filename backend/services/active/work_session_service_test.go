package active

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/models/base"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
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
	getByStaffAndDateFunc   func(ctx context.Context, staffID int64, date timezone.Date) (*activeModels.WorkSession, error)
	getCurrentByStaffIDFunc func(ctx context.Context, staffID int64) (*activeModels.WorkSession, error)
	getHistoryByStaffIDFunc func(ctx context.Context, staffID int64, from, to timezone.Date) ([]*activeModels.WorkSession, error)
	getOpenSessionsFunc     func(ctx context.Context, beforeDate timezone.Date) ([]*activeModels.WorkSession, error)
	getTodayPresenceMapFunc func(ctx context.Context) (map[int64]string, error)
	closeSessionFunc        func(ctx context.Context, id int64, checkOutTime time.Time, autoCheckedOut bool) (bool, error)
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

func (m *wsMockWorkSessionRepository) GetByStaffAndDate(ctx context.Context, staffID int64, date timezone.Date) (*activeModels.WorkSession, error) {
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

func (m *wsMockWorkSessionRepository) GetHistoryByStaffID(ctx context.Context, staffID int64, from, to timezone.Date) ([]*activeModels.WorkSession, error) {
	if m.getHistoryByStaffIDFunc != nil {
		return m.getHistoryByStaffIDFunc(ctx, staffID, from, to)
	}
	return nil, nil
}

func (m *wsMockWorkSessionRepository) GetOpenSessions(ctx context.Context, beforeDate timezone.Date) ([]*activeModels.WorkSession, error) {
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

func (m *wsMockWorkSessionRepository) CloseSession(ctx context.Context, id int64, checkOutTime time.Time, autoCheckedOut bool) (bool, error) {
	if m.closeSessionFunc != nil {
		return m.closeSessionFunc(ctx, id, checkOutTime, autoCheckedOut)
	}
	return true, nil
}

func (m *wsMockWorkSessionRepository) UpdateBreakMinutes(ctx context.Context, id int64, breakMinutes int) error {
	if m.updateBreakMinutesFunc != nil {
		return m.updateBreakMinutesFunc(ctx, id, breakMinutes)
	}
	return nil
}

type wsMockStaffWorkScheduleRepository struct {
	getCurrentByStaffIDFunc func(ctx context.Context, staffID int64) ([]*configModels.StaffWorkSchedule, error)
	getByStaffIDAndDateFunc func(ctx context.Context, staffID int64, date timezone.Date) ([]*configModels.StaffWorkSchedule, error)
	replaceScheduleFunc     func(ctx context.Context, staffID int64, entries []*configModels.StaffWorkSchedule) error
}

func (m *wsMockStaffWorkScheduleRepository) GetCurrentByStaffID(ctx context.Context, staffID int64) ([]*configModels.StaffWorkSchedule, error) {
	if m.getCurrentByStaffIDFunc != nil {
		return m.getCurrentByStaffIDFunc(ctx, staffID)
	}
	return nil, nil
}

func (m *wsMockStaffWorkScheduleRepository) GetByStaffIDAndDate(ctx context.Context, staffID int64, date timezone.Date) ([]*configModels.StaffWorkSchedule, error) {
	if m.getByStaffIDAndDateFunc != nil {
		return m.getByStaffIDAndDateFunc(ctx, staffID, date)
	}
	return nil, nil
}

func (m *wsMockStaffWorkScheduleRepository) ReplaceSchedule(ctx context.Context, staffID int64, entries []*configModels.StaffWorkSchedule) error {
	if m.replaceScheduleFunc != nil {
		return m.replaceScheduleFunc(ctx, staffID, entries)
	}
	return nil
}

type wsMockWorkTimeModelRepository struct {
	findByIDFunc func(ctx context.Context, id int64) (*configModels.WorkTimeModel, error)
}

func (m *wsMockWorkTimeModelRepository) List(ctx context.Context) ([]*configModels.WorkTimeModel, error) {
	return nil, nil
}

func (m *wsMockWorkTimeModelRepository) FindByID(ctx context.Context, id int64) (*configModels.WorkTimeModel, error) {
	if m.findByIDFunc != nil {
		return m.findByIDFunc(ctx, id)
	}
	return nil, sql.ErrNoRows
}

func (m *wsMockWorkTimeModelRepository) Create(ctx context.Context, model *configModels.WorkTimeModel, entries []*configModels.WorkTimeModelEntry) error {
	return nil
}

func (m *wsMockWorkTimeModelRepository) Update(ctx context.Context, model *configModels.WorkTimeModel, entries []*configModels.WorkTimeModelEntry) error {
	return nil
}

func (m *wsMockWorkTimeModelRepository) RefreshAssignedStaffSchedules(ctx context.Context, modelID int64) error {
	return nil
}

func (m *wsMockWorkTimeModelRepository) Delete(ctx context.Context, id int64) error {
	return nil
}

type wsMockStaffRepository struct {
	findByIDFunc func(ctx context.Context, id interface{}) (*userModels.Staff, error)
}

func (m *wsMockStaffRepository) Create(ctx context.Context, staff *userModels.Staff) error {
	return nil
}

func (m *wsMockStaffRepository) FindByID(ctx context.Context, id interface{}) (*userModels.Staff, error) {
	if m.findByIDFunc != nil {
		return m.findByIDFunc(ctx, id)
	}
	return nil, sql.ErrNoRows
}

func (m *wsMockStaffRepository) FindByPersonID(ctx context.Context, personID int64) (*userModels.Staff, error) {
	return nil, sql.ErrNoRows
}

func (m *wsMockStaffRepository) Update(ctx context.Context, staff *userModels.Staff) error {
	return nil
}

func (m *wsMockStaffRepository) Delete(ctx context.Context, id interface{}) error {
	return nil
}

func (m *wsMockStaffRepository) List(ctx context.Context, filters map[string]interface{}) ([]*userModels.Staff, error) {
	return nil, nil
}

func (m *wsMockStaffRepository) ListAllWithPerson(ctx context.Context) ([]*userModels.Staff, error) {
	return nil, nil
}

func (m *wsMockStaffRepository) ClearWorkTimeModel(context.Context, int64) error { return nil }

func (m *wsMockStaffRepository) UpdateNotes(ctx context.Context, id int64, notes string) error {
	return nil
}

func (m *wsMockStaffRepository) FindWithPerson(ctx context.Context, id int64) (*userModels.Staff, error) {
	return nil, sql.ErrNoRows
}

func (m *wsMockStaffRepository) FindByIDs(ctx context.Context, ids []int64) (map[int64]*userModels.Staff, error) {
	return nil, nil
}

func (m *wsMockStaffRepository) FindWithPersonByIDs(ctx context.Context, ids []int64) (map[int64]*userModels.Staff, error) {
	return nil, nil
}

func (m *wsMockStaffRepository) ListStaffByRoles(ctx context.Context, roles []string) ([]*userModels.StaffWithRoleInfo, error) {
	return nil, nil
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
	getExpiredBreaksFunc     func(ctx context.Context, before time.Time) ([]*activeModels.WorkSessionBreak, error)
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

func (m *wsMockWorkSessionBreakRepository) GetExpiredBreaks(ctx context.Context, before time.Time) ([]*activeModels.WorkSessionBreak, error) {
	if m.getExpiredBreaksFunc != nil {
		return m.getExpiredBreaksFunc(ctx, before)
	}
	return nil, nil
}

// ============================================================================
// Mock for WorkSessionEditRepository (prefixed with ws)
// ============================================================================

type wsMockWorkSessionEditRepository struct {
	createBatchFunc             func(ctx context.Context, edits []*auditModels.WorkSessionEdit) error
	getBySessionIDFunc          func(ctx context.Context, sessionID int64) ([]*auditModels.WorkSessionEdit, error)
	countBySessionIDFunc        func(ctx context.Context, sessionID int64) (int, error)
	countBySessionIDsFunc       func(ctx context.Context, sessionIDs []int64) (map[int64]int, error)
	countManualBySessionIDsFunc func(ctx context.Context, sessionIDs []int64) (map[int64]int, error)
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

func (m *wsMockWorkSessionEditRepository) CountManualBySessionIDs(ctx context.Context, sessionIDs []int64) (map[int64]int, error) {
	if m.countManualBySessionIDsFunc != nil {
		return m.countManualBySessionIDsFunc(ctx, sessionIDs)
	}
	// Existing tests stub countBySessionIDsFunc to mean "edits by a person";
	// fall back so their semantics are unchanged.
	return m.CountBySessionIDs(ctx, sessionIDs)
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
	getByStaffAndDateRangeFunc func(ctx context.Context, staffID int64, from, to timezone.Date) ([]*activeModels.StaffAbsence, error)
	getByStaffAndDateFunc      func(ctx context.Context, staffID int64, date timezone.Date) (*activeModels.StaffAbsence, error)
	getByDateRangeFunc         func(ctx context.Context, from, to timezone.Date) ([]*activeModels.StaffAbsence, error)
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

func (m *wsMockStaffAbsenceRepository) GetByStaffAndDateRange(ctx context.Context, staffID int64, from, to timezone.Date) ([]*activeModels.StaffAbsence, error) {
	if m.getByStaffAndDateRangeFunc != nil {
		return m.getByStaffAndDateRangeFunc(ctx, staffID, from, to)
	}
	return nil, nil
}

func (m *wsMockStaffAbsenceRepository) GetByStaffAndDate(ctx context.Context, staffID int64, date timezone.Date) (*activeModels.StaffAbsence, error) {
	if m.getByStaffAndDateFunc != nil {
		return m.getByStaffAndDateFunc(ctx, staffID, date)
	}
	return nil, nil
}

func (m *wsMockStaffAbsenceRepository) GetByDateRange(ctx context.Context, from, to timezone.Date) ([]*activeModels.StaffAbsence, error) {
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

// ListByStaffAndStatuses + ListByStatus are part of the StaffAbsenceRepository
// interface added in the Tranche 4 vacation-workflow spike. No-op defaults so
// work-session tests still satisfy the interface.
func (m *wsMockStaffAbsenceRepository) ListByStaffAndStatuses(_ context.Context, _ int64, _ []string) ([]*activeModels.StaffAbsence, error) {
	return nil, nil
}

func (m *wsMockStaffAbsenceRepository) ListByStatus(_ context.Context, _ string) ([]*activeModels.StaffAbsence, error) {
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

func (m *wsMockGroupSupervisorRepository) CreateBulk(ctx context.Context, supervisors []*activeModels.GroupSupervisor) error {
	return nil
}

func (m *wsMockGroupSupervisorRepository) EndSupervisionsByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64) (int64, error) {
	return 0, nil
}

func (m *wsMockGroupSupervisorRepository) EndByActiveGroupAndStaffID(ctx context.Context, activeGroupID, staffID int64) (int, error) {
	return 0, nil
}

func (m *wsMockGroupSupervisorRepository) FindAllActive(ctx context.Context) ([]*activeModels.GroupSupervisor, error) {
	return nil, nil
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

	sessionRepo.getByStaffAndDateFunc = func(_ context.Context, _ int64, _ timezone.Date) (*activeModels.WorkSession, error) {
		return nil, sql.ErrNoRows
	}

	sessionRepo.createFunc = func(_ context.Context, entity *activeModels.WorkSession) error {
		entity.ID = 10
		return nil
	}

	session, err := svc.CheckIn(ctx, staffID, activeModels.WorkSessionStatusPresent, activeModels.WorkSessionSourceApp)
	require.NoError(t, err)
	require.NotNil(t, session)
	assert.Equal(t, staffID, session.StaffID)
	assert.Equal(t, activeModels.WorkSessionStatusPresent, session.Status)
	assert.Equal(t, activeModels.WorkSessionSourceApp, session.Source)
	assert.Nil(t, session.CheckOutTime)
}

func TestWSCheckIn_RejectsEmptyStatus(t *testing.T) {
	// Issue #1368: staff must explicitly choose Vor Ort vs Homeoffice; the
	// service no longer silently defaults an empty status to "present".
	svc, sessionRepo, _, _, _ := wsCreateTestService()
	ctx := context.Background()

	sessionRepo.createFunc = func(_ context.Context, _ *activeModels.WorkSession) error {
		t.Fatal("createFunc should not be called when status is empty")
		return nil
	}

	session, err := svc.CheckIn(ctx, 100, "", activeModels.WorkSessionSourceApp)
	require.Error(t, err)
	assert.Nil(t, session)
	assert.Contains(t, err.Error(), "status must be")
}

func TestWSCheckIn_AlreadyCheckedIn(t *testing.T) {
	svc, sessionRepo, _, _, _ := wsCreateTestService()

	sessionRepo.getByStaffAndDateFunc = func(_ context.Context, _ int64, _ timezone.Date) (*activeModels.WorkSession, error) {
		return &activeModels.WorkSession{
			Model:       base.Model{ID: 1},
			StaffID:     100,
			CheckInTime: time.Now().Add(-2 * time.Hour),
		}, nil
	}

	session, err := svc.CheckIn(context.Background(), 100, activeModels.WorkSessionStatusPresent, activeModels.WorkSessionSourceApp)
	require.Error(t, err)
	assert.Nil(t, session)
	assert.Contains(t, err.Error(), "already checked in")
}

func TestWSCheckIn_ReopenCheckedOutSession(t *testing.T) {
	svc, sessionRepo, _, _, _ := wsCreateTestService()
	checkOut := time.Now().Add(-1 * time.Hour)

	sessionRepo.getByStaffAndDateFunc = func(_ context.Context, _ int64, _ timezone.Date) (*activeModels.WorkSession, error) {
		return &activeModels.WorkSession{
			Model:          base.Model{ID: 1},
			StaffID:        100,
			CheckInTime:    time.Now().Add(-4 * time.Hour),
			CheckOutTime:   &checkOut,
			AutoCheckedOut: true,
			Status:         activeModels.WorkSessionStatusPresent,
			Date:           timezone.TodayDate(),
			CreatedBy:      100,
		}, nil
	}

	sessionRepo.updateFunc = func(_ context.Context, entity *activeModels.WorkSession) error {
		assert.Nil(t, entity.CheckOutTime)
		assert.False(t, entity.AutoCheckedOut)
		return nil
	}

	session, err := svc.CheckIn(context.Background(), 100, activeModels.WorkSessionStatusPresent, activeModels.WorkSessionSourceApp)
	require.NoError(t, err)
	require.NotNil(t, session)
	assert.Nil(t, session.CheckOutTime)
}

// TestWSCheckIn_ReopenPreservesOriginalSourceAndStatus locks in the
// audit-trail behaviour for Issue #1368: when the staff member reopens an
// NFC-originated session via the App with the SAME status, both Source and
// Status are preserved. Reopen never overwrites either field — Source has
// no audit-edit channel, and Status changes must go through UpdateSession
// (which gates on a notes reason).
func TestWSCheckIn_ReopenPreservesOriginalSourceAndStatus(t *testing.T) {
	svc, sessionRepo, _, _, _ := wsCreateTestService()
	checkOut := time.Now().Add(-1 * time.Hour)

	sessionRepo.getByStaffAndDateFunc = func(_ context.Context, _ int64, _ timezone.Date) (*activeModels.WorkSession, error) {
		return &activeModels.WorkSession{
			Model:          base.Model{ID: 1},
			StaffID:        100,
			CheckInTime:    time.Now().Add(-4 * time.Hour),
			CheckOutTime:   &checkOut,
			AutoCheckedOut: true,
			Status:         activeModels.WorkSessionStatusPresent,
			Source:         activeModels.WorkSessionSourceNFC,
			Date:           timezone.TodayDate(),
			CreatedBy:      100,
		}, nil
	}

	var capturedSource, capturedStatus string
	sessionRepo.updateFunc = func(_ context.Context, entity *activeModels.WorkSession) error {
		capturedSource = entity.Source
		capturedStatus = entity.Status
		return nil
	}

	// Reopen via App with the SAME status — both Source and Status survive.
	session, err := svc.CheckIn(context.Background(), 100,
		activeModels.WorkSessionStatusPresent, activeModels.WorkSessionSourceApp)
	require.NoError(t, err)
	require.NotNil(t, session)
	assert.Equal(t, activeModels.WorkSessionSourceNFC, capturedSource,
		"reopen must preserve the originating Source")
	assert.Equal(t, activeModels.WorkSessionStatusPresent, capturedStatus,
		"reopen must preserve the originating Status")
}

// TestWSCheckIn_ReopenStatusMismatchReturnsTypedConflict guards the audit
// trail: a reopen that would silently flip Vor Ort ↔ Homeoffice is rejected
// before the DB write. The frontend uses the typed code to branch into the
// "change status with reason" flow (UpdateSession with notes).
func TestWSCheckIn_ReopenStatusMismatchReturnsTypedConflict(t *testing.T) {
	svc, sessionRepo, _, _, _ := wsCreateTestService()
	checkOut := time.Now().Add(-1 * time.Hour)

	existing := &activeModels.WorkSession{
		Model:          base.Model{ID: 42},
		StaffID:        100,
		CheckInTime:    time.Now().Add(-4 * time.Hour),
		CheckOutTime:   &checkOut,
		AutoCheckedOut: true,
		Status:         activeModels.WorkSessionStatusPresent,
		Source:         activeModels.WorkSessionSourceNFC,
		Date:           timezone.TodayDate(),
		CreatedBy:      100,
	}
	sessionRepo.getByStaffAndDateFunc = func(_ context.Context, _ int64, _ timezone.Date) (*activeModels.WorkSession, error) {
		return existing, nil
	}
	sessionRepo.updateFunc = func(_ context.Context, _ *activeModels.WorkSession) error {
		t.Fatal("repo.Update must not be called on a status-mismatch reopen")
		return nil
	}

	session, err := svc.CheckIn(context.Background(), 100,
		activeModels.WorkSessionStatusHomeOffice, activeModels.WorkSessionSourceApp)
	require.Error(t, err)
	assert.Nil(t, session)

	var conflict *ReopenStatusConflictError
	require.ErrorAs(t, err, &conflict, "must surface as the typed conflict")
	assert.Equal(t, int64(42), conflict.SessionID)
	assert.Equal(t, activeModels.WorkSessionStatusPresent, conflict.ExistingStatus)
	assert.Equal(t, activeModels.WorkSessionStatusHomeOffice, conflict.RequestedStatus)
}

func TestWSCheckIn_InvalidStatus(t *testing.T) {
	svc, _, _, _, _ := wsCreateTestService()

	session, err := svc.CheckIn(context.Background(), 100, "invalid_status", activeModels.WorkSessionSourceApp)
	require.Error(t, err)
	assert.Nil(t, session)
	assert.Contains(t, err.Error(), "status must be")
}

// TestWSCheckIn_InvalidSource guards the second validation step in CheckIn:
// a bogus channel must be rejected at the service boundary before any DB
// write, so the only values that ever reach active.work_sessions.source are
// 'app' or 'nfc' (matching the CHECK constraint in migration 1.15.54).
// The error string is also part of the HTTP-boundary contract — the
// classifier in api/time-tracking/errors.go keys on the "source must be"
// prefix to produce 400 instead of 500.
func TestWSCheckIn_InvalidSource(t *testing.T) {
	svc, _, _, _, _ := wsCreateTestService()

	session, err := svc.CheckIn(context.Background(), 100,
		activeModels.WorkSessionStatusPresent, "bogus")
	require.Error(t, err)
	assert.Nil(t, session)
	assert.Contains(t, err.Error(), "source must be",
		"classifyServiceError matches this prefix to map the error to HTTP 400")
}

// TestWSCheckIn_RejectsUnknownSource locks in the write/read asymmetry on
// the 'unknown' sentinel: legacy rows on disk may carry it (migration 1.15.54
// backfills NULL → 'unknown'), but the service must never produce a new row
// with source='unknown'. Without this gate, a careless caller could erase
// the audit signal "this stamp's channel was actually never recorded" by
// re-writing a fresh row with the same sentinel.
func TestWSCheckIn_RejectsUnknownSource(t *testing.T) {
	svc, _, _, _, _ := wsCreateTestService()

	session, err := svc.CheckIn(context.Background(), 100,
		activeModels.WorkSessionStatusPresent, activeModels.WorkSessionSourceUnknown)
	require.Error(t, err)
	assert.Nil(t, session)
	assert.Contains(t, err.Error(), "source must be",
		"'unknown' is a read-only sentinel for legacy rows and must not be writable")
}

// TestReopenStatusConflictError_Error covers the typed error's stringification.
// Production callers use errors.As to branch on the type, which does not
// invoke Error(), but the message still appears in slog warnings/info logs
// when the error bubbles up — locking the string keeps log greps stable.
func TestReopenStatusConflictError_Error(t *testing.T) {
	err := &ReopenStatusConflictError{
		SessionID:       7,
		ExistingStatus:  activeModels.WorkSessionStatusPresent,
		RequestedStatus: activeModels.WorkSessionStatusHomeOffice,
	}
	assert.Equal(t, "reopen status conflict", err.Error())
}

// TestWSReopenThenUpdateSession_EmitsFieldStatusEdit exercises the realistic
// frontend sequence end-to-end: a checked-out 'present' session is reopened
// at the existing status (no conflict), then the requested status flip is
// routed through UpdateSession with a notes reason. The combined behaviour
// the audit trail relies on is:
//
//  1. The reopen does not create an audit edit (it preserves Status).
//  2. The follow-up UpdateSession produces a FieldStatus edit whose old/new
//     values match the chosen flip and whose Reason carries the user's text.
//
// This complements the unit tests that cover each step in isolation
// (TestWSCheckIn_ReopenPreservesOriginalSourceAndStatus and
// TestWSUpdateSession_StatusChange) by locking in the cross-call behaviour.
func TestWSReopenThenUpdateSession_EmitsFieldStatusEdit(t *testing.T) {
	svc, sessionRepo, _, auditRepo, _ := wsCreateTestService()
	ctx := context.Background()
	staffID := int64(100)
	sessionID := int64(701)
	checkOut := time.Now().Add(-1 * time.Hour)

	current := &activeModels.WorkSession{
		Model:          base.Model{ID: sessionID},
		StaffID:        staffID,
		CheckInTime:    time.Now().Add(-4 * time.Hour),
		CheckOutTime:   &checkOut,
		AutoCheckedOut: true,
		Status:         activeModels.WorkSessionStatusPresent,
		Source:         activeModels.WorkSessionSourceApp,
		Date:           timezone.TodayDate(),
		CreatedBy:      staffID,
	}

	sessionRepo.getByStaffAndDateFunc = func(_ context.Context, _ int64, _ timezone.Date) (*activeModels.WorkSession, error) {
		return current, nil
	}
	sessionRepo.findByIDFunc = func(_ context.Context, _ any) (*activeModels.WorkSession, error) {
		return current, nil
	}
	sessionRepo.updateFunc = func(_ context.Context, entity *activeModels.WorkSession) error {
		current = entity
		return nil
	}

	var auditCalls int
	var capturedEdits []*auditModels.WorkSessionEdit
	auditRepo.createBatchFunc = func(_ context.Context, edits []*auditModels.WorkSessionEdit) error {
		auditCalls++
		capturedEdits = append(capturedEdits, edits...)
		return nil
	}

	// Step 1: reopen at existing status — no audit edit.
	reopened, err := svc.CheckIn(ctx, staffID,
		activeModels.WorkSessionStatusPresent, activeModels.WorkSessionSourceApp)
	require.NoError(t, err)
	require.NotNil(t, reopened)
	require.Nil(t, reopened.CheckOutTime, "reopen must clear CheckOutTime")
	assert.Equal(t, 0, auditCalls, "reopen alone must not emit audit edits")

	// Step 2: route the actual status flip through UpdateSession with reason.
	newStatus := activeModels.WorkSessionStatusHomeOffice
	reason := "Mittags ins Homeoffice gewechselt"
	updated, err := svc.UpdateSession(ctx, staffID, sessionID, SessionUpdateRequest{
		Status: &newStatus,
		Notes:  &reason,
	})
	require.NoError(t, err)
	require.NotNil(t, updated)

	require.GreaterOrEqual(t, auditCalls, 1, "UpdateSession must emit an audit batch")
	var statusEdit *auditModels.WorkSessionEdit
	for _, e := range capturedEdits {
		if e.FieldName == auditModels.FieldStatus {
			statusEdit = e
			break
		}
	}
	require.NotNil(t, statusEdit, "audit batch must contain a FieldStatus edit")
	require.NotNil(t, statusEdit.OldValue)
	require.NotNil(t, statusEdit.NewValue)
	assert.Equal(t, activeModels.WorkSessionStatusPresent, *statusEdit.OldValue,
		"FieldStatus old value reflects the pre-update status")
	assert.Equal(t, activeModels.WorkSessionStatusHomeOffice, *statusEdit.NewValue,
		"FieldStatus new value reflects the requested flip")
	require.NotNil(t, statusEdit.Notes,
		"FieldStatus edit must carry the user-supplied reason in Notes")
	assert.Equal(t, reason, *statusEdit.Notes)
}

func TestWSCheckIn_RepoError(t *testing.T) {
	svc, sessionRepo, _, _, _ := wsCreateTestService()

	sessionRepo.getByStaffAndDateFunc = func(_ context.Context, _ int64, _ timezone.Date) (*activeModels.WorkSession, error) {
		return nil, errors.New("database error")
	}

	session, err := svc.CheckIn(context.Background(), 100, activeModels.WorkSessionStatusPresent, activeModels.WorkSessionSourceApp)
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

	sessionRepo.closeSessionFunc = func(_ context.Context, _ int64, _ time.Time, autoCheckedOut bool) (bool, error) {
		assert.False(t, autoCheckedOut)
		return true, nil
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

	sessionRepo.closeSessionFunc = func(_ context.Context, _ int64, _ time.Time, _ bool) (bool, error) {
		return true, nil
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

	brk, err := svc.StartBreak(context.Background(), staffID, nil)
	require.NoError(t, err)
	require.NotNil(t, brk)
	assert.Equal(t, int64(50), brk.SessionID)
}

func TestWSStartBreak_NoActiveSession(t *testing.T) {
	svc, sessionRepo, _, _, _ := wsCreateTestService()

	sessionRepo.getCurrentByStaffIDFunc = func(_ context.Context, _ int64) (*activeModels.WorkSession, error) {
		return nil, sql.ErrNoRows
	}

	brk, err := svc.StartBreak(context.Background(), 100, nil)
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

	brk, err := svc.StartBreak(context.Background(), 100, nil)
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
	from := timezone.TodayDate().AddDays(-7)
	to := timezone.TodayDate()

	checkIn := time.Now().Add(-8 * time.Hour)
	checkOut := time.Now().Add(-2 * time.Hour)
	sessionRepo.getHistoryByStaffIDFunc = func(_ context.Context, _ int64, _, _ timezone.Date) ([]*activeModels.WorkSession, error) {
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

	auditRepo.countManualBySessionIDsFunc = func(_ context.Context, _ []int64) (map[int64]int, error) {
		return map[int64]int{1: 1}, nil
	}
	auditRepo.countBySessionIDsFunc = func(_ context.Context, _ []int64) (map[int64]int, error) {
		return map[int64]int{1: 2}, nil
	}

	breakRepo.getBySessionIDFunc = func(_ context.Context, _ int64) ([]*activeModels.WorkSessionBreak, error) {
		return []*activeModels.WorkSessionBreak{}, nil
	}

	historyResp, err := svc.GetHistory(context.Background(), staffID, from, to)
	require.NoError(t, err)
	require.Len(t, historyResp.Sessions, 1)
	assert.Equal(t, 1, historyResp.Sessions[0].EditCount)
	assert.Equal(t, 2, historyResp.Sessions[0].AuditCount)
	require.Len(t, historyResp.WeeklySummaries, 1)
}

func TestWSGetHistory_UsesRotationWeekTargets(t *testing.T) {
	svc, sessionRepo, breakRepo, auditRepo, _ := wsCreateTestService()
	staffID := int64(100)
	wednesdayWeekZero := time.Date(2026, 6, 3, 8, 0, 0, 0, time.UTC)
	mondayWeekOne := time.Date(2026, 6, 8, 8, 0, 0, 0, time.UTC)
	checkOutWeekZero := wednesdayWeekZero.Add(20 * time.Hour)
	checkOutWeekOne := mondayWeekOne.Add(30 * time.Hour)

	svc.scheduleRepo = &wsMockStaffWorkScheduleRepository{
		getByStaffIDAndDateFunc: func(_ context.Context, _ int64, _ timezone.Date) ([]*configModels.StaffWorkSchedule, error) {
			validFrom := timezone.NewDate(2026, 6, 3)
			return []*configModels.StaffWorkSchedule{
				{
					StaffID:        staffID,
					WeekIndex:      0,
					RotationLength: 2,
					DayOfWeek:      configModels.DayMonday,
					TargetMinutes:  20 * 60,
					ValidFrom:      validFrom,
				},
				{
					StaffID:        staffID,
					WeekIndex:      1,
					RotationLength: 2,
					DayOfWeek:      configModels.DayMonday,
					TargetMinutes:  30 * 60,
					ValidFrom:      validFrom,
				},
			}, nil
		},
	}
	sessionRepo.getHistoryByStaffIDFunc = func(_ context.Context, _ int64, _, _ timezone.Date) ([]*activeModels.WorkSession, error) {
		return []*activeModels.WorkSession{
			{
				Model:          base.Model{ID: 1},
				StaffID:        staffID,
				Date:           timezone.DateFromTime(wednesdayWeekZero),
				CheckInTime:    wednesdayWeekZero,
				CheckOutTime:   &checkOutWeekZero,
				BreakMinutes:   0,
				Status:         activeModels.WorkSessionStatusPresent,
				Source:         activeModels.WorkSessionSourceApp,
				CreatedBy:      staffID,
				AutoCheckedOut: false,
			},
			{
				Model:          base.Model{ID: 2},
				StaffID:        staffID,
				Date:           timezone.DateFromTime(mondayWeekOne),
				CheckInTime:    mondayWeekOne,
				CheckOutTime:   &checkOutWeekOne,
				BreakMinutes:   0,
				Status:         activeModels.WorkSessionStatusPresent,
				Source:         activeModels.WorkSessionSourceApp,
				CreatedBy:      staffID,
				AutoCheckedOut: false,
			},
		}, nil
	}
	auditRepo.countBySessionIDsFunc = func(_ context.Context, _ []int64) (map[int64]int, error) {
		return map[int64]int{}, nil
	}
	breakRepo.getBySessionIDFunc = func(_ context.Context, _ int64) ([]*activeModels.WorkSessionBreak, error) {
		return []*activeModels.WorkSessionBreak{}, nil
	}

	historyResp, err := svc.GetHistory(context.Background(), staffID, timezone.DateFromTime(wednesdayWeekZero), timezone.DateFromTime(mondayWeekOne))
	require.NoError(t, err)
	require.Len(t, historyResp.WeeklySummaries, 2)
	require.NotNil(t, historyResp.WeeklySummaries[0].TargetMinutes)
	require.NotNil(t, historyResp.WeeklySummaries[1].TargetMinutes)
	assert.Equal(t, 20*60, *historyResp.WeeklySummaries[0].TargetMinutes)
	assert.Equal(t, 0, *historyResp.WeeklySummaries[0].DeltaMinutes)
	assert.Equal(t, 30*60, *historyResp.WeeklySummaries[1].TargetMinutes)
	assert.Equal(t, 0, *historyResp.WeeklySummaries[1].DeltaMinutes)
}

func TestWSGetHistory_UsesDateValidCustomScheduleTargets(t *testing.T) {
	svc, sessionRepo, breakRepo, auditRepo, _ := wsCreateTestService()
	staffID := int64(100)
	oldWeek := time.Date(2026, 6, 3, 8, 0, 0, 0, time.UTC)
	newWeek := time.Date(2026, 6, 10, 8, 0, 0, 0, time.UTC)
	checkOutOldWeek := oldWeek.Add(20 * time.Hour)
	checkOutNewWeek := newWeek.Add(30 * time.Hour)
	changeDate := timezone.NewDate(2026, 6, 8)

	svc.scheduleRepo = &wsMockStaffWorkScheduleRepository{
		getCurrentByStaffIDFunc: func(_ context.Context, _ int64) ([]*configModels.StaffWorkSchedule, error) {
			t.Fatal("history targets must use date-valid schedule rows")
			return nil, nil
		},
		getByStaffIDAndDateFunc: func(_ context.Context, _ int64, date timezone.Date) ([]*configModels.StaffWorkSchedule, error) {
			if date.Before(changeDate) {
				return []*configModels.StaffWorkSchedule{
					{
						StaffID:        staffID,
						WeekIndex:      0,
						RotationLength: 1,
						DayOfWeek:      configModels.DayWednesday,
						TargetMinutes:  20 * 60,
						ValidFrom:      timezone.NewDate(2026, 1, 1),
					},
				}, nil
			}
			return []*configModels.StaffWorkSchedule{
				{
					StaffID:        staffID,
					WeekIndex:      0,
					RotationLength: 1,
					DayOfWeek:      configModels.DayWednesday,
					TargetMinutes:  30 * 60,
					ValidFrom:      changeDate,
				},
			}, nil
		},
	}
	sessionRepo.getHistoryByStaffIDFunc = func(_ context.Context, _ int64, _, _ timezone.Date) ([]*activeModels.WorkSession, error) {
		return []*activeModels.WorkSession{
			{
				Model:        base.Model{ID: 2401},
				StaffID:      staffID,
				Date:         timezone.DateFromTime(oldWeek),
				CheckInTime:  oldWeek,
				CheckOutTime: &checkOutOldWeek,
				Status:       activeModels.WorkSessionStatusPresent,
				Source:       activeModels.WorkSessionSourceApp,
				CreatedBy:    staffID,
			},
			{
				Model:        base.Model{ID: 2402},
				StaffID:      staffID,
				Date:         timezone.DateFromTime(newWeek),
				CheckInTime:  newWeek,
				CheckOutTime: &checkOutNewWeek,
				Status:       activeModels.WorkSessionStatusPresent,
				Source:       activeModels.WorkSessionSourceApp,
				CreatedBy:    staffID,
			},
		}, nil
	}
	auditRepo.countBySessionIDsFunc = func(_ context.Context, _ []int64) (map[int64]int, error) {
		return map[int64]int{}, nil
	}
	breakRepo.getBySessionIDFunc = func(_ context.Context, _ int64) ([]*activeModels.WorkSessionBreak, error) {
		return []*activeModels.WorkSessionBreak{}, nil
	}

	historyResp, err := svc.GetHistory(context.Background(), staffID, timezone.DateFromTime(oldWeek), timezone.DateFromTime(newWeek))

	require.NoError(t, err)
	require.Len(t, historyResp.WeeklySummaries, 2)
	require.NotNil(t, historyResp.WeeklySummaries[0].TargetMinutes)
	require.NotNil(t, historyResp.WeeklySummaries[1].TargetMinutes)
	assert.Equal(t, 20*60, *historyResp.WeeklySummaries[0].TargetMinutes)
	assert.Equal(t, 0, *historyResp.WeeklySummaries[0].DeltaMinutes)
	assert.Equal(t, 30*60, *historyResp.WeeklySummaries[1].TargetMinutes)
	assert.Equal(t, 0, *historyResp.WeeklySummaries[1].DeltaMinutes)
}

func TestWSGetHistory_UsesTemplateScheduleSnapshotTargets(t *testing.T) {
	svc, sessionRepo, breakRepo, auditRepo, _ := wsCreateTestService()
	staffID := int64(100)
	modelID := int64(2300)
	anchor := timezone.NewDate(2026, 6, 1)
	weekZero := time.Date(2026, 6, 3, 8, 0, 0, 0, time.UTC)
	weekOne := time.Date(2026, 6, 8, 8, 0, 0, 0, time.UTC)
	checkOutWeekZero := weekZero.Add(25 * time.Hour)
	checkOutWeekOne := weekOne.Add(35 * time.Hour)

	svc.staffRepo = &wsMockStaffRepository{
		findByIDFunc: func(_ context.Context, _ interface{}) (*userModels.Staff, error) {
			return &userModels.Staff{
				Model:              base.Model{ID: staffID},
				WorkTimeModelID:    &modelID,
				RotationAnchorDate: &anchor,
			}, nil
		},
	}
	svc.workModelRepo = &wsMockWorkTimeModelRepository{
		findByIDFunc: func(_ context.Context, id int64) (*configModels.WorkTimeModel, error) {
			t.Fatalf("template-assigned staff with schedule snapshot must not read current model %d", id)
			return nil, sql.ErrNoRows
		},
	}
	svc.scheduleRepo = &wsMockStaffWorkScheduleRepository{
		getByStaffIDAndDateFunc: func(_ context.Context, _ int64, _ timezone.Date) ([]*configModels.StaffWorkSchedule, error) {
			return []*configModels.StaffWorkSchedule{
				{
					WeekIndex:      0,
					RotationLength: 2,
					DayOfWeek:      configModels.DayMonday,
					TargetMinutes:  25 * 60,
					ValidFrom:      anchor,
				},
				{
					WeekIndex:      1,
					RotationLength: 2,
					DayOfWeek:      configModels.DayMonday,
					TargetMinutes:  35 * 60,
					ValidFrom:      anchor,
				},
			}, nil
		},
	}
	sessionRepo.getHistoryByStaffIDFunc = func(_ context.Context, _ int64, _, _ timezone.Date) ([]*activeModels.WorkSession, error) {
		return []*activeModels.WorkSession{
			{
				Model:        base.Model{ID: 2301},
				StaffID:      staffID,
				Date:         timezone.DateFromTime(weekZero),
				CheckInTime:  weekZero,
				CheckOutTime: &checkOutWeekZero,
				Status:       activeModels.WorkSessionStatusPresent,
				Source:       activeModels.WorkSessionSourceApp,
				CreatedBy:    staffID,
			},
			{
				Model:        base.Model{ID: 2302},
				StaffID:      staffID,
				Date:         timezone.DateFromTime(weekOne),
				CheckInTime:  weekOne,
				CheckOutTime: &checkOutWeekOne,
				Status:       activeModels.WorkSessionStatusPresent,
				Source:       activeModels.WorkSessionSourceApp,
				CreatedBy:    staffID,
			},
		}, nil
	}
	auditRepo.countBySessionIDsFunc = func(_ context.Context, _ []int64) (map[int64]int, error) {
		return map[int64]int{}, nil
	}
	breakRepo.getBySessionIDFunc = func(_ context.Context, _ int64) ([]*activeModels.WorkSessionBreak, error) {
		return []*activeModels.WorkSessionBreak{}, nil
	}

	historyResp, err := svc.GetHistory(context.Background(), staffID, timezone.DateFromTime(weekZero), timezone.DateFromTime(weekOne))
	require.NoError(t, err)
	require.Len(t, historyResp.WeeklySummaries, 2)
	require.NotNil(t, historyResp.WeeklySummaries[0].TargetMinutes)
	require.NotNil(t, historyResp.WeeklySummaries[1].TargetMinutes)
	assert.Equal(t, 25*60, *historyResp.WeeklySummaries[0].TargetMinutes)
	assert.Equal(t, 0, *historyResp.WeeklySummaries[0].DeltaMinutes)
	assert.Equal(t, 35*60, *historyResp.WeeklySummaries[1].TargetMinutes)
	assert.Equal(t, 0, *historyResp.WeeklySummaries[1].DeltaMinutes)
}

func TestWSGetHistory_RepoError(t *testing.T) {
	svc, sessionRepo, _, _, _ := wsCreateTestService()

	sessionRepo.getHistoryByStaffIDFunc = func(_ context.Context, _ int64, _, _ timezone.Date) ([]*activeModels.WorkSession, error) {
		return nil, errors.New("database error")
	}

	historyResp, err := svc.GetHistory(context.Background(), 100, timezone.TodayDate(), timezone.TodayDate())
	require.Error(t, err)
	assert.Nil(t, historyResp)
}

// ============================================================================
// GetSessionBreaks Tests
// ============================================================================

func TestWSGetSessionBreaks_Success(t *testing.T) {
	svc, sessionRepo, breakRepo, _, _ := wsCreateTestService()
	staffID := int64(100)
	sessionID := int64(502)

	// Mock FindByID to return session owned by staffID
	sessionRepo.findByIDFunc = func(_ context.Context, _ any) (*activeModels.WorkSession, error) {
		return &activeModels.WorkSession{
			Model:   base.Model{ID: sessionID},
			StaffID: staffID,
		}, nil
	}

	breakRepo.getBySessionIDFunc = func(_ context.Context, _ int64) ([]*activeModels.WorkSessionBreak, error) {
		return []*activeModels.WorkSessionBreak{
			{Model: base.Model{ID: 1}},
			{Model: base.Model{ID: 2}},
		}, nil
	}

	breaks, err := svc.GetSessionBreaks(context.Background(), staffID, sessionID)
	require.NoError(t, err)
	assert.Len(t, breaks, 2)
}

// ============================================================================
// GetSessionEdits Tests
// ============================================================================

func TestWSGetSessionEdits_Success(t *testing.T) {
	svc, sessionRepo, _, auditRepo, _ := wsCreateTestService()
	staffID := int64(100)
	sessionID := int64(503)

	// Mock FindByID to return session owned by staffID
	sessionRepo.findByIDFunc = func(_ context.Context, _ any) (*activeModels.WorkSession, error) {
		return &activeModels.WorkSession{
			Model:   base.Model{ID: sessionID},
			StaffID: staffID,
		}, nil
	}

	auditRepo.getBySessionIDFunc = func(_ context.Context, _ int64) ([]*auditModels.WorkSessionEdit, error) {
		return []*auditModels.WorkSessionEdit{
			{SessionID: 1, FieldName: "check_in_time"},
		}, nil
	}

	edits, err := svc.GetSessionEdits(context.Background(), staffID, sessionID)
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
	yesterday := timezone.TodayDate().AddDays(-1)

	sessionRepo.getOpenSessionsFunc = func(_ context.Context, _ timezone.Date) ([]*activeModels.WorkSession, error) {
		return []*activeModels.WorkSession{
			{Model: base.Model{ID: 1}, Date: yesterday},
			{Model: base.Model{ID: 2}, Date: yesterday},
		}, nil
	}

	sessionRepo.closeSessionFunc = func(_ context.Context, _ int64, _ time.Time, autoCheckedOut bool) (bool, error) {
		assert.True(t, autoCheckedOut)
		return true, nil
	}

	count, err := svc.CleanupOpenSessions(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestWSCleanupOpenSessions_NoOpenSessions(t *testing.T) {
	svc, sessionRepo, _, _, _ := wsCreateTestService()

	sessionRepo.getOpenSessionsFunc = func(_ context.Context, _ timezone.Date) ([]*activeModels.WorkSession, error) {
		return nil, nil
	}

	count, err := svc.CleanupOpenSessions(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestWSCleanupOpenSessions_CheckOutTimeIsBerlinEndOfDay(t *testing.T) {
	svc, sessionRepo, _, _, _ := wsCreateTestService()

	// Session date stored as a calendar day
	sessionDate := timezone.NewDate(2026, 3, 26)

	sessionRepo.getOpenSessionsFunc = func(_ context.Context, _ timezone.Date) ([]*activeModels.WorkSession, error) {
		return []*activeModels.WorkSession{
			{Model: base.Model{ID: 100}, Date: sessionDate},
		}, nil
	}

	var capturedCheckOutTime time.Time
	sessionRepo.closeSessionFunc = func(_ context.Context, _ int64, checkOutTime time.Time, _ bool) (bool, error) {
		capturedCheckOutTime = checkOutTime
		return true, nil
	}

	count, err := svc.CleanupOpenSessions(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// The check-out time must be 23:59:59 Europe/Berlin, NOT 23:59:59 UTC.
	// In CET (UTC+1), 23:59:59 Berlin = 22:59:59 UTC.
	// In CEST (UTC+2), 23:59:59 Berlin = 21:59:59 UTC.
	checkOutInBerlin := capturedCheckOutTime.In(timezone.Berlin)

	assert.Equal(t, 23, checkOutInBerlin.Hour(), "hour should be 23 in Berlin time")
	assert.Equal(t, 59, checkOutInBerlin.Minute(), "minute should be 59")
	assert.Equal(t, 59, checkOutInBerlin.Second(), "second should be 59")
	assert.Equal(t, 2026, checkOutInBerlin.Year())
	assert.Equal(t, time.March, checkOutInBerlin.Month())
	assert.Equal(t, 26, checkOutInBerlin.Day(), "date should still be March 26, not March 27")

	// Verify it is NOT 23:59:59 UTC (the old buggy behavior)
	assert.NotEqual(t, 23, capturedCheckOutTime.UTC().Hour(),
		"check-out should NOT be 23:59:59 UTC — that would be 00:59:59 CET the next day")
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

	session, err := svc.EnsureCheckedIn(context.Background(), staffID, activeModels.WorkSessionSourceNFC)
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

	sessionRepo.getByStaffAndDateFunc = func(_ context.Context, _ int64, _ timezone.Date) (*activeModels.WorkSession, error) {
		return &activeModels.WorkSession{
			Model:        base.Model{ID: 1},
			StaffID:      staffID,
			CheckOutTime: &checkOut,
		}, nil
	}

	session, err := svc.EnsureCheckedIn(context.Background(), staffID, activeModels.WorkSessionSourceNFC)
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
	sessionRepo.getByStaffAndDateFunc = func(_ context.Context, _ int64, _ timezone.Date) (*activeModels.WorkSession, error) {
		callCount++
		if callCount == 1 {
			// First call from EnsureCheckedIn
			return nil, sql.ErrNoRows
		}
		// Second call from CheckIn
		return nil, sql.ErrNoRows
	}

	var capturedSource string
	sessionRepo.createFunc = func(_ context.Context, entity *activeModels.WorkSession) error {
		entity.ID = 10
		capturedSource = entity.Source
		return nil
	}

	session, err := svc.EnsureCheckedIn(context.Background(), staffID, activeModels.WorkSessionSourceNFC)
	require.NoError(t, err)
	require.NotNil(t, session)
	assert.Equal(t, activeModels.WorkSessionSourceNFC, capturedSource,
		"EnsureCheckedIn must forward the caller-supplied source to CheckIn")
}

// TestWSEnsureCheckedIn_ForwardsAppSource verifies that EnsureCheckedIn does
// not hard-code 'nfc' — non-NFC callers (web triggers, future schedulers)
// must be able to record their channel faithfully.
func TestWSEnsureCheckedIn_ForwardsAppSource(t *testing.T) {
	svc, sessionRepo, _, _, _ := wsCreateTestService()
	staffID := int64(100)

	sessionRepo.getCurrentByStaffIDFunc = func(_ context.Context, _ int64) (*activeModels.WorkSession, error) {
		return nil, sql.ErrNoRows
	}
	sessionRepo.getByStaffAndDateFunc = func(_ context.Context, _ int64, _ timezone.Date) (*activeModels.WorkSession, error) {
		return nil, sql.ErrNoRows
	}

	var capturedSource string
	sessionRepo.createFunc = func(_ context.Context, entity *activeModels.WorkSession) error {
		entity.ID = 11
		capturedSource = entity.Source
		return nil
	}

	_, err := svc.EnsureCheckedIn(context.Background(), staffID, activeModels.WorkSessionSourceApp)
	require.NoError(t, err)
	assert.Equal(t, activeModels.WorkSessionSourceApp, capturedSource)
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
			Date:        timezone.TodayDate(),
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
			Date:        timezone.TodayDate(),
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
			Date:        timezone.TodayDate(),
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
			Date:        timezone.TodayDate(),
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

// Generic query helper stubs (interface additions for the issue #585
// cleanup refactor) — unused by these tests.
func (m *wsMockWorkSessionRepository) CountWithOptions(context.Context, *base.QueryOptions) (int, error) {
	return 0, nil
}

func (m *wsMockWorkSessionRepository) OldestBefore(context.Context, string, *timezone.Date) (*timezone.Date, error) {
	return nil, nil
}

func (m *wsMockWorkSessionRepository) DeleteOlderThan(context.Context, string, timezone.Date) (int64, error) {
	return 0, nil
}

func (m *wsMockStaffAbsenceRepository) CountWithOptions(context.Context, *base.QueryOptions) (int, error) {
	return 0, nil
}

func (m *wsMockStaffAbsenceRepository) OldestBefore(context.Context, string, *timezone.Date) (*timezone.Date, error) {
	return nil, nil
}

func (m *wsMockStaffAbsenceRepository) DeleteOlderThan(context.Context, string, timezone.Date) (int64, error) {
	return 0, nil
}

func (m *wsMockStaffAbsenceRepository) DeleteNonHistoricalByStaffID(context.Context, int64, timezone.Date) (int64, error) {
	return 0, nil
}

func (m *wsMockGroupSupervisorRepository) ListActiveSupervisionBlockers(context.Context, int64, int64) ([]userModels.BlockerSupervision, error) {
	return nil, nil
}

func (m *wsMockGroupSupervisorRepository) FindStaleOpen(context.Context, timezone.Date) ([]*activeModels.GroupSupervisor, error) {
	return nil, nil
}

func (m *wsMockGroupSupervisorRepository) UpdateColumns(context.Context, *activeModels.GroupSupervisor, ...string) (int64, error) {
	return 0, nil
}
