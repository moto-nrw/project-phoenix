package active

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/models/base"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Mocks for WorkSessionRepository (prefixed with ws)
// ============================================================================

type wsMockWorkSessionRepository struct {
	lockBalanceWritesFunc   func(ctx context.Context, staffID int64) error
	createFunc              func(ctx context.Context, entity *activeModels.WorkSession) error
	findByIDFunc            func(ctx context.Context, id any) (*activeModels.WorkSession, error)
	updateFunc              func(ctx context.Context, entity *activeModels.WorkSession) error
	deleteFunc              func(ctx context.Context, id any) error
	listFunc                func(ctx context.Context, options *base.QueryOptions) ([]*activeModels.WorkSession, error)
	listByStaffAndDateFunc  func(ctx context.Context, staffID int64, date timezone.Date) ([]*activeModels.WorkSession, error)
	getCurrentByStaffIDFunc func(ctx context.Context, staffID int64) (*activeModels.WorkSession, error)
	// Only set by tests that need the two lookups to differ, i.e. an open
	// block that lives on a day other than the one being read. Unset, the
	// day-independent lookup answers like the today-scoped one.
	getLatestOpenByStaffIDFunc func(ctx context.Context, staffID int64) (*activeModels.WorkSession, error)
	getOpenByStaffAndDateFunc  func(ctx context.Context, staffID int64, date timezone.Date) (*activeModels.WorkSession, error)
	// Same idea for the locking read: set it when the test asserts WHICH day
	// the lock was taken on, otherwise getCurrentForUpdateFunc suffices.
	getOpenByStaffAndDateForUpdateFunc func(ctx context.Context, staffID int64, date timezone.Date) (*activeModels.WorkSession, error)
	getCurrentForUpdateFunc            func(ctx context.Context, staffID int64) (*activeModels.WorkSession, error)
	lockOpenByIDFunc                   func(ctx context.Context, id int64) (*activeModels.WorkSession, error)
	getHistoryByStaffIDFunc            func(ctx context.Context, staffID int64, from, to timezone.Date) ([]*activeModels.WorkSession, error)
	listOverlappingByStaffIDFunc       func(ctx context.Context, staffID int64, from time.Time, to *time.Time) ([]*activeModels.WorkSession, error)
	getHistoryByStaffIDsFunc           func(ctx context.Context, staffIDs []int64, from, to timezone.Date) (map[int64][]*activeModels.WorkSession, error)
	getOpenSessionsFunc                func(ctx context.Context, beforeDate timezone.Date) ([]*activeModels.WorkSession, error)
	getTodayPresenceMapFunc            func(ctx context.Context) (map[int64]string, error)
	closeSessionFunc                   func(ctx context.Context, id int64, checkOutTime time.Time, autoCheckedOut bool) (bool, error)
	updateBreakMinutesFunc             func(ctx context.Context, id int64, breakMinutes int) error
}

func (m *wsMockWorkSessionRepository) LockStaffBalanceWrites(ctx context.Context, staffID int64) error {
	if m.lockBalanceWritesFunc != nil {
		return m.lockBalanceWritesFunc(ctx, staffID)
	}
	return nil
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
	if m.listByStaffAndDateFunc != nil {
		return m.listByStaffAndDateFunc(ctx, 0, timezone.TodayDate())
	}
	return nil, nil
}

func (m *wsMockWorkSessionRepository) GetCurrentByStaffID(ctx context.Context, staffID int64) (*activeModels.WorkSession, error) {
	if m.getCurrentByStaffIDFunc != nil {
		return m.getCurrentByStaffIDFunc(ctx, staffID)
	}
	return nil, sql.ErrNoRows
}

func (m *wsMockWorkSessionRepository) GetOpenByStaffAndDate(ctx context.Context, staffID int64, date timezone.Date) (*activeModels.WorkSession, error) {
	if m.getOpenByStaffAndDateFunc != nil {
		return m.getOpenByStaffAndDateFunc(ctx, staffID, date)
	}
	return m.GetCurrentByStaffID(ctx, staffID)
}

func (m *wsMockWorkSessionRepository) GetLatestOpenByStaffID(ctx context.Context, staffID int64) (*activeModels.WorkSession, error) {
	if m.getLatestOpenByStaffIDFunc != nil {
		return m.getLatestOpenByStaffIDFunc(ctx, staffID)
	}
	return m.GetCurrentByStaffID(ctx, staffID)
}

func (m *wsMockWorkSessionRepository) GetOpenByStaffAndDateForUpdate(ctx context.Context, staffID int64, date timezone.Date) (*activeModels.WorkSession, error) {
	if m.getOpenByStaffAndDateForUpdateFunc != nil {
		return m.getOpenByStaffAndDateForUpdateFunc(ctx, staffID, date)
	}
	if m.getCurrentForUpdateFunc != nil {
		return m.getCurrentForUpdateFunc(ctx, staffID)
	}
	return m.GetCurrentByStaffID(ctx, staffID)
}

func (m *wsMockWorkSessionRepository) LockOpenByIDForUpdate(ctx context.Context, id int64) (*activeModels.WorkSession, error) {
	if m.lockOpenByIDFunc != nil {
		return m.lockOpenByIDFunc(ctx, id)
	}
	if m.findByIDFunc != nil {
		session, err := m.findByIDFunc(ctx, id)
		if err != nil {
			return nil, err
		}
		if session.CheckOutTime != nil {
			return nil, sql.ErrNoRows
		}
		return session, nil
	}
	return nil, sql.ErrNoRows
}

func (m *wsMockWorkSessionRepository) GetHistoryByStaffID(ctx context.Context, staffID int64, from, to timezone.Date) ([]*activeModels.WorkSession, error) {
	if m.getHistoryByStaffIDFunc != nil {
		return m.getHistoryByStaffIDFunc(ctx, staffID, from, to)
	}
	return nil, nil
}

func (m *wsMockWorkSessionRepository) ListOverlappingByStaffID(ctx context.Context, staffID int64, from time.Time, to *time.Time) ([]*activeModels.WorkSession, error) {
	if m.listOverlappingByStaffIDFunc != nil {
		return m.listOverlappingByStaffIDFunc(ctx, staffID, from, to)
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
	getCurrentByStaffIDFunc        func(ctx context.Context, staffID int64) ([]*configModels.StaffWorkSchedule, error)
	getByStaffIDAndDateFunc        func(ctx context.Context, staffID int64, date timezone.Date) ([]*configModels.StaffWorkSchedule, error)
	replaceScheduleFunc            func(ctx context.Context, staffID int64, entries []*configModels.StaffWorkSchedule, anchor timezone.Date) error
	findByStaffIDsValidInRangeFunc func(ctx context.Context, staffIDs []int64, from, to timezone.Date) ([]*configModels.StaffWorkSchedule, error)
	hasScheduleHistoryFunc         func(ctx context.Context, staffID int64) (bool, error)
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

func (m *wsMockStaffWorkScheduleRepository) ReplaceSchedule(ctx context.Context, staffID int64, entries []*configModels.StaffWorkSchedule, anchor timezone.Date) error {
	if m.replaceScheduleFunc != nil {
		return m.replaceScheduleFunc(ctx, staffID, entries, anchor)
	}
	return nil
}

func (m *wsMockStaffWorkScheduleRepository) FindByStaffIDsValidInRange(ctx context.Context, staffIDs []int64, from, to timezone.Date) ([]*configModels.StaffWorkSchedule, error) {
	if m.findByStaffIDsValidInRangeFunc != nil {
		return m.findByStaffIDsValidInRangeFunc(ctx, staffIDs, from, to)
	}
	return nil, nil
}

func (m *wsMockStaffWorkScheduleRepository) HasScheduleHistory(ctx context.Context, staffID int64) (bool, error) {
	if m.hasScheduleHistoryFunc != nil {
		return m.hasScheduleHistoryFunc(ctx, staffID)
	}
	return false, nil
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

func (m *wsMockWorkTimeModelRepository) FindByIDs(context.Context, []int64) ([]*configModels.WorkTimeModel, error) {
	return nil, nil
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

type wsMockSettingsResolver struct {
	resolveBoolFunc func(ctx context.Context, key string) (bool, error)
	resolveIntFunc  func(ctx context.Context, key string) (int, error)
}

func (m *wsMockSettingsResolver) ResolveBool(ctx context.Context, key string) (bool, error) {
	if m.resolveBoolFunc != nil {
		return m.resolveBoolFunc(ctx, key)
	}
	return false, nil
}

func (m *wsMockSettingsResolver) ResolveInt(ctx context.Context, key string) (int, error) {
	if m.resolveIntFunc != nil {
		return m.resolveIntFunc(ctx, key)
	}
	return 0, nil
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
	createFunc                    func(ctx context.Context, entity *activeModels.StaffAbsence) error
	findByIDFunc                  func(ctx context.Context, id any) (*activeModels.StaffAbsence, error)
	updateFunc                    func(ctx context.Context, entity *activeModels.StaffAbsence) error
	deleteFunc                    func(ctx context.Context, id any) error
	listFunc                      func(ctx context.Context, options *base.QueryOptions) ([]*activeModels.StaffAbsence, error)
	getByStaffAndDateRangeFunc    func(ctx context.Context, staffID int64, from, to timezone.Date) ([]*activeModels.StaffAbsence, error)
	getByStaffIDsAndDateRangeFunc func(ctx context.Context, staffIDs []int64, from, to timezone.Date) (map[int64][]*activeModels.StaffAbsence, error)
	getByStaffAndDateFunc         func(ctx context.Context, staffID int64, date timezone.Date) (*activeModels.StaffAbsence, error)
	getByDateRangeFunc            func(ctx context.Context, from, to timezone.Date) ([]*activeModels.StaffAbsence, error)
	getAbsenceMapForDateFunc      func(ctx context.Context, date timezone.Date) (map[int64]string, error)
}

func (m *wsMockStaffAbsenceRepository) LockStaffAbsenceWrites(context.Context, int64) error {
	return nil
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

func (m *wsMockStaffAbsenceRepository) GetAbsenceMapForDate(ctx context.Context, date timezone.Date) (map[int64]string, error) {
	if m.getAbsenceMapForDateFunc != nil {
		return m.getAbsenceMapForDateFunc(ctx, date)
	}
	return nil, nil
}

// ListByStaffAndStatuses + ListByStatuses are part of the StaffAbsenceRepository
// interface added in the Tranche 4 vacation-workflow spike. No-op defaults so
// work-session tests still satisfy the interface.
func (m *wsMockStaffAbsenceRepository) ListByStaffAndStatuses(_ context.Context, _ int64, _ []string) ([]*activeModels.StaffAbsence, error) {
	return nil, nil
}

func (m *wsMockStaffAbsenceRepository) ListByStatuses(_ context.Context, _ []string) ([]*activeModels.StaffAbsence, error) {
	return nil, nil
}

func (m *wsMockStaffAbsenceRepository) ListRequests(_ context.Context, _ activeModels.AbsenceRequestFilter) ([]*activeModels.AbsenceRequestRow, error) {
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

func (m *wsMockGroupSupervisorRepository) ListActiveSupervisedRooms(ctx context.Context) ([]activeModels.StaffRoomSupervision, error) {
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

func TestWorkSessionService_CheckInLocksBalanceBeforeLookupAndWrite(t *testing.T) {
	service, sessionRepo, _, _, _ := wsCreateTestService()
	events := []string{}
	staffID := int64(71)
	sessionRepo.lockBalanceWritesFunc = func(_ context.Context, gotStaffID int64) error {
		assert.Equal(t, staffID, gotStaffID)
		events = append(events, "lock")
		return nil
	}
	sessionRepo.listByStaffAndDateFunc = func(context.Context, int64, timezone.Date) ([]*activeModels.WorkSession, error) {
		events = append(events, "lookup")
		return nil, nil
	}
	sessionRepo.createFunc = func(context.Context, *activeModels.WorkSession) error {
		events = append(events, "create")
		return nil
	}

	_, err := service.CheckIn(
		context.Background(),
		staffID,
		activeModels.WorkSessionStatusPresent,
		activeModels.WorkSessionSourceApp,
		"",
	)

	require.NoError(t, err)
	assert.Equal(t, []string{"lock", "lookup", "create"}, events)
}

func TestWSCheckIn_Success(t *testing.T) {
	svc, sessionRepo, _, _, _ := wsCreateTestService()
	ctx := context.Background()
	staffID := int64(100)

	sessionRepo.listByStaffAndDateFunc = func(_ context.Context, _ int64, _ timezone.Date) ([]*activeModels.WorkSession, error) {
		return nil, nil
	}

	sessionRepo.createFunc = func(_ context.Context, entity *activeModels.WorkSession) error {
		entity.ID = 10
		return nil
	}

	session, err := svc.CheckIn(ctx, staffID, activeModels.WorkSessionStatusPresent, activeModels.WorkSessionSourceApp, "")
	require.NoError(t, err)
	require.NotNil(t, session)
	assert.Equal(t, staffID, session.StaffID)
	assert.Equal(t, activeModels.WorkSessionStatusPresent, session.Status)
	assert.Equal(t, activeModels.WorkSessionSourceApp, session.Source)
	assert.Nil(t, session.CheckOutTime)
}

func TestWSCheckInBroadcastsTimeTrackingChangeAfterCommit(t *testing.T) {
	svc, sessionRepo, _, _, _ := wsCreateTestService()
	broadcaster := testpkg.NewRecordingBroadcaster()
	svc.SetBroadcaster(broadcaster)
	ctx, commit := tenant.WithAfterCommitHooksForTest(
		tenant.WithTenantID(context.Background(), 42),
	)

	sessionRepo.listByStaffAndDateFunc = func(_ context.Context, _ int64, _ timezone.Date) ([]*activeModels.WorkSession, error) {
		return nil, nil
	}

	_, err := svc.CheckIn(
		ctx,
		100,
		activeModels.WorkSessionStatusPresent,
		activeModels.WorkSessionSourceApp,
		"",
	)
	require.NoError(t, err)
	assert.Empty(t, broadcaster.Events(),
		"the invalidation must not precede the surrounding transaction commit")

	commit()

	events := broadcaster.EventsOfType(realtime.EventStaffTimeTrackingChanged)
	require.Len(t, events, 1)
	calls := broadcaster.CallsByMethod("tenant")
	require.Len(t, calls, 1)
	assert.Equal(t, int64(42), calls[0].TenantID)
}

func TestWSCheckIn_PlannedStartEnforcement(t *testing.T) {
	startAt := func(t *testing.T, hhmm string) *time.Time {
		t.Helper()
		parsed, err := time.Parse("15:04", hhmm)
		require.NoError(t, err)
		wallClock := timezone.WallClock(parsed)
		return &wallClock
	}

	tests := []struct {
		name          string
		now           time.Time
		rows          []*configModels.StaffWorkSchedule
		wantErr       bool
		wantCreate    bool
		wantPlannedAt string
	}{
		{
			name: "before planned start rejects",
			now:  time.Date(2026, time.July, 6, 8, 59, 0, 0, timezone.Berlin),
			rows: []*configModels.StaffWorkSchedule{{
				WeekIndex:      0,
				RotationLength: 1,
				DayOfWeek:      configModels.DayMonday,
				TargetMinutes:  480,
				StartTime:      startAt(t, "09:00"),
				ValidFrom:      timezone.DateFromTime(time.Date(2026, time.July, 6, 0, 0, 0, 0, timezone.Berlin)),
			}},
			wantErr:       true,
			wantPlannedAt: "09:00",
		},
		{
			name: "exact planned start allows",
			now:  time.Date(2026, time.July, 6, 9, 0, 0, 0, timezone.Berlin),
			rows: []*configModels.StaffWorkSchedule{{
				WeekIndex:      0,
				RotationLength: 1,
				DayOfWeek:      configModels.DayMonday,
				TargetMinutes:  480,
				StartTime:      startAt(t, "09:00"),
				ValidFrom:      timezone.DateFromTime(time.Date(2026, time.July, 6, 0, 0, 0, 0, timezone.Berlin)),
			}},
			wantCreate: true,
		},
		{
			name: "after planned start allows",
			now:  time.Date(2026, time.July, 6, 9, 1, 0, 0, timezone.Berlin),
			rows: []*configModels.StaffWorkSchedule{{
				WeekIndex:      0,
				RotationLength: 1,
				DayOfWeek:      configModels.DayMonday,
				TargetMinutes:  480,
				StartTime:      startAt(t, "09:00"),
				ValidFrom:      timezone.DateFromTime(time.Date(2026, time.July, 6, 0, 0, 0, 0, timezone.Berlin)),
			}},
			wantCreate: true,
		},
		{
			name:       "no schedule keeps existing behavior",
			now:        time.Date(2026, time.July, 6, 8, 30, 0, 0, timezone.Berlin),
			rows:       nil,
			wantCreate: true,
		},
		{
			name: "schedule without start time keeps existing behavior",
			now:  time.Date(2026, time.July, 6, 8, 30, 0, 0, timezone.Berlin),
			rows: []*configModels.StaffWorkSchedule{{
				WeekIndex:      0,
				RotationLength: 1,
				DayOfWeek:      configModels.DayMonday,
				TargetMinutes:  480,
				ValidFrom:      timezone.DateFromTime(time.Date(2026, time.July, 6, 0, 0, 0, 0, timezone.Berlin)),
			}},
			wantCreate: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, sessionRepo, _, _, _ := wsCreateTestService()
			svc.nowFunc = func() time.Time { return tt.now }
			svc.settings = &wsMockSettingsResolver{resolveBoolFunc: func(_ context.Context, key string) (bool, error) {
				assert.Equal(t, configModels.KeyTimeTrackingEnforcePlannedStart, key)
				return true, nil
			}}
			svc.scheduleRepo = &wsMockStaffWorkScheduleRepository{
				getByStaffIDAndDateFunc: func(_ context.Context, _ int64, _ timezone.Date) ([]*configModels.StaffWorkSchedule, error) {
					return tt.rows, nil
				},
			}

			sessionRepo.listByStaffAndDateFunc = func(_ context.Context, _ int64, _ timezone.Date) ([]*activeModels.WorkSession, error) {
				return nil, nil
			}
			created := false
			sessionRepo.createFunc = func(_ context.Context, entity *activeModels.WorkSession) error {
				created = true
				entity.ID = 10
				return nil
			}

			session, err := svc.CheckIn(context.Background(), 100, activeModels.WorkSessionStatusPresent, activeModels.WorkSessionSourceApp, "")
			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, session)
				var plannedErr *PlannedStartNotReachedError
				require.ErrorAs(t, err, &plannedErr)
				assert.Equal(t, tt.wantPlannedAt, plannedErr.PlannedStartTime)
				assert.False(t, created, "early check-in must not create a work session")
				return
			}

			require.NoError(t, err)
			require.NotNil(t, session)
			assert.Equal(t, tt.wantCreate, created)
		})
	}
}

func TestWSCreateSessionAsAdmin_IgnoresPlannedStartEnforcement(t *testing.T) {
	svc, sessionRepo, _, auditRepo, _ := wsCreateTestService()
	ctx := context.Background()
	editorStaffID := int64(10)
	targetStaffID := int64(100)
	checkIn := time.Date(2026, time.July, 6, 8, 0, 0, 0, timezone.Berlin)
	checkOut := time.Date(2026, time.July, 6, 10, 0, 0, 0, timezone.Berlin)

	svc.settings = &wsMockSettingsResolver{resolveBoolFunc: func(context.Context, string) (bool, error) {
		t.Fatal("admin-created sessions must not resolve planned-start enforcement")
		return true, nil
	}}

	var created *activeModels.WorkSession
	sessionRepo.createFunc = func(_ context.Context, entity *activeModels.WorkSession) error {
		entity.ID = 55
		created = entity
		return nil
	}

	var capturedEdits []*auditModels.WorkSessionEdit
	auditRepo.createBatchFunc = func(_ context.Context, edits []*auditModels.WorkSessionEdit) error {
		capturedEdits = edits
		return nil
	}

	session, err := svc.CreateSessionAsAdmin(ctx, editorStaffID, targetStaffID, AdminCreateSessionRequest{
		Date:         checkIn,
		CheckInTime:  checkIn,
		CheckOutTime: checkOut,
		Status:       activeModels.WorkSessionStatusPresent,
		Notes:        "Früherer Start durch Admin genehmigt",
	})

	require.NoError(t, err)
	require.NotNil(t, session)
	require.NotNil(t, created)
	assert.Equal(t, checkIn, created.CheckInTime)
	require.NotNil(t, created.CheckOutTime)
	assert.Equal(t, checkOut, *created.CheckOutTime)
	assert.Equal(t, editorStaffID, created.CreatedBy)
	require.NotEmpty(t, capturedEdits, "admin-created sessions must keep audit trail")
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

	session, err := svc.CheckIn(ctx, 100, "", activeModels.WorkSessionSourceApp, "")
	require.Error(t, err)
	assert.Nil(t, session)
	assert.Contains(t, err.Error(), "status must be")
}

func TestWSCheckIn_AlreadyCheckedIn(t *testing.T) {
	svc, sessionRepo, _, _, _ := wsCreateTestService()

	open := &activeModels.WorkSession{
		Model:       base.Model{ID: 1},
		StaffID:     100,
		CheckInTime: time.Now().Add(-2 * time.Hour),
	}
	// The open block is visible in both reads the service does — the day list
	// and the "is anything running" lookup — like it would be in the database.
	sessionRepo.getCurrentByStaffIDFunc = func(_ context.Context, _ int64) (*activeModels.WorkSession, error) {
		return open, nil
	}
	sessionRepo.listByStaffAndDateFunc = func(_ context.Context, _ int64, _ timezone.Date) ([]*activeModels.WorkSession, error) {
		return []*activeModels.WorkSession{open}, nil
	}

	session, err := svc.CheckIn(context.Background(), 100, activeModels.WorkSessionStatusPresent, activeModels.WorkSessionSourceApp, "")
	require.Error(t, err)
	assert.Nil(t, session)
	assert.Contains(t, err.Error(), "already checked in")
}

// A block that was opened before Berlin midnight is still running after it.
// Opening a second one there would leave two open blocks: the checkout closes
// exactly one, so the running block and the day totals would drift apart. The
// guard therefore looks for an open block on ANY day, not just the stamp's.
func TestWSCheckIn_OpenBlockOfAnEarlierDayBlocksNewOne(t *testing.T) {
	yesterday := timezone.NewDate(2026, 7, 21)
	stampedAt := time.Date(2026, time.July, 22, 6, 0, 0, 0, timezone.Berlin)

	svc, sessionRepo, _, _, _ := wsCreateTestService()
	svc.nowFunc = func() time.Time { return stampedAt }

	sessionRepo.getLatestOpenByStaffIDFunc = func(_ context.Context, _ int64) (*activeModels.WorkSession, error) {
		return &activeModels.WorkSession{
			Model:       base.Model{ID: 77},
			StaffID:     100,
			Date:        yesterday,
			Status:      activeModels.WorkSessionStatusPresent,
			Source:      activeModels.WorkSessionSourceNFC,
			CheckInTime: time.Date(2026, time.July, 21, 22, 0, 0, 0, timezone.Berlin),
		}, nil
	}
	sessionRepo.createFunc = func(_ context.Context, _ *activeModels.WorkSession) error {
		t.Fatal("a second block must not be created while another one is still open")
		return nil
	}

	session, err := svc.CheckIn(context.Background(), 100, activeModels.WorkSessionStatusPresent, activeModels.WorkSessionSourceApp, "")
	require.Error(t, err)
	assert.Nil(t, session)
	assert.Contains(t, err.Error(), "already checked in")
}

// The counterpart of the guard above: the check-out has to reach the block
// that is actually running, otherwise a block that crossed midnight could
// neither be closed nor followed by a new one.
func TestWSCheckOut_ClosesBlockOpenedOnAnEarlierDay(t *testing.T) {
	yesterday := timezone.NewDate(2026, 7, 21)
	svc, sessionRepo, breakRepo, _, supervisorRepo := wsCreateTestService()
	svc.nowFunc = func() time.Time { return time.Date(2026, time.July, 22, 6, 0, 0, 0, timezone.Berlin) }

	open := &activeModels.WorkSession{
		Model:       base.Model{ID: 77},
		StaffID:     100,
		Date:        yesterday,
		Status:      activeModels.WorkSessionStatusPresent,
		Source:      activeModels.WorkSessionSourceNFC,
		CheckInTime: time.Date(2026, time.July, 21, 22, 0, 0, 0, timezone.Berlin),
	}
	sessionRepo.getLatestOpenByStaffIDFunc = func(_ context.Context, _ int64) (*activeModels.WorkSession, error) {
		return open, nil
	}
	var lookedUpDays []timezone.Date
	sessionRepo.getOpenByStaffAndDateFunc = func(_ context.Context, _ int64, day timezone.Date) (*activeModels.WorkSession, error) {
		lookedUpDays = append(lookedUpDays, day)
		if day == yesterday {
			return open, nil
		}
		return nil, sql.ErrNoRows
	}
	breakRepo.getActiveBySessionIDFunc = func(_ context.Context, _ int64) (*activeModels.WorkSessionBreak, error) {
		return nil, nil
	}
	var closedID int64
	sessionRepo.closeSessionFunc = func(_ context.Context, id int64, _ time.Time, _ bool) (bool, error) {
		closedID = id
		return true, nil
	}
	supervisorRepo.endAllActiveByStaffIDFunc = func(_ context.Context, _ int64) (int, error) {
		return 0, nil
	}
	checkOut := time.Date(2026, time.July, 22, 6, 0, 0, 0, timezone.Berlin)
	sessionRepo.findByIDFunc = func(_ context.Context, id any) (*activeModels.WorkSession, error) {
		return &activeModels.WorkSession{
			Model:        base.Model{ID: id.(int64)},
			StaffID:      100,
			Date:         yesterday,
			CheckInTime:  open.CheckInTime,
			CheckOutTime: &checkOut,
		}, nil
	}

	session, err := svc.CheckOut(context.Background(), 100, "")
	require.NoError(t, err)
	require.NotNil(t, session)
	assert.Equal(t, open.ID, closedID, "the running block is the one that gets closed")
	assert.Equal(t, []timezone.Date{yesterday}, lookedUpDays, "resolved on the block's own day, not on today")
}

// TestWSCheckIn_SecondBlockAfterCheckout locks in the #2402 semantics: a
// check-in after a same-day checkout creates a NEW work block instead of
// reopening the closed one. The first block keeps its check-out as a real
// interval boundary, and the gap between the blocks is not work time.
func TestWSCheckIn_SecondBlockAfterCheckout(t *testing.T) {
	svc, sessionRepo, _, _, _ := wsCreateTestService()
	checkIn := time.Now().Add(-4 * time.Hour)
	checkOut := time.Now().Add(-1 * time.Hour)

	sessionRepo.listByStaffAndDateFunc = func(_ context.Context, _ int64, _ timezone.Date) ([]*activeModels.WorkSession, error) {
		return []*activeModels.WorkSession{{
			Model:          base.Model{ID: 1},
			StaffID:        100,
			CheckInTime:    checkIn,
			CheckOutTime:   &checkOut,
			AutoCheckedOut: true,
			Status:         activeModels.WorkSessionStatusPresent,
			Date:           timezone.TodayDate(),
			CreatedBy:      100,
		}}, nil
	}

	sessionRepo.updateFunc = func(_ context.Context, _ *activeModels.WorkSession) error {
		t.Fatal("a second check-in must not reopen (update) the closed block")
		return nil
	}
	var created *activeModels.WorkSession
	sessionRepo.createFunc = func(_ context.Context, entity *activeModels.WorkSession) error {
		entity.ID = 2
		created = entity
		return nil
	}

	session, err := svc.CheckIn(context.Background(), 100, activeModels.WorkSessionStatusPresent, activeModels.WorkSessionSourceApp, "")
	require.NoError(t, err)
	require.NotNil(t, session)
	require.NotNil(t, created, "second check-in must create a new block")
	assert.Equal(t, created.ID, session.ID)
	assert.Nil(t, session.CheckOutTime)
	assert.True(t, session.CheckInTime.After(checkOut),
		"the new block starts at the stamp, not at the first block's check-in")
	assert.Nil(t, session.ReopenedAt)
}

// TestWSCheckIn_SecondBlockWithDifferentStatus is the core of #2402: a
// Homeoffice morning followed by an OGS afternoon. The second check-in with a
// DIFFERENT status simply creates a new block carrying that status — no
// conflict, no reason, no audit edit, and the first block's status is
// untouched.
func TestWSCheckIn_SecondBlockWithDifferentStatus(t *testing.T) {
	svc, sessionRepo, _, auditRepo, _ := wsCreateTestService()
	checkOut := time.Now().Add(-90 * time.Minute)

	firstBlock := &activeModels.WorkSession{
		Model:        base.Model{ID: 42},
		StaffID:      100,
		CheckInTime:  time.Now().Add(-5 * time.Hour),
		CheckOutTime: &checkOut,
		Status:       activeModels.WorkSessionStatusHomeOffice,
		Source:       activeModels.WorkSessionSourceApp,
		Date:         timezone.TodayDate(),
		CreatedBy:    100,
	}
	sessionRepo.listByStaffAndDateFunc = func(_ context.Context, _ int64, _ timezone.Date) ([]*activeModels.WorkSession, error) {
		return []*activeModels.WorkSession{firstBlock}, nil
	}
	sessionRepo.updateFunc = func(_ context.Context, _ *activeModels.WorkSession) error {
		t.Fatal("the first block must stay untouched")
		return nil
	}
	auditRepo.createBatchFunc = func(_ context.Context, _ []*auditModels.WorkSessionEdit) error {
		t.Fatal("a new block with its own status needs no audit edit")
		return nil
	}
	var created *activeModels.WorkSession
	sessionRepo.createFunc = func(_ context.Context, entity *activeModels.WorkSession) error {
		entity.ID = 43
		created = entity
		return nil
	}

	session, err := svc.CheckIn(context.Background(), 100,
		activeModels.WorkSessionStatusPresent, activeModels.WorkSessionSourceNFC, "")
	require.NoError(t, err)
	require.NotNil(t, session)
	require.NotNil(t, created)
	assert.Equal(t, activeModels.WorkSessionStatusPresent, created.Status,
		"the new block carries the requested status")
	assert.Equal(t, activeModels.WorkSessionSourceNFC, created.Source,
		"the new block carries its own channel")
	assert.Equal(t, activeModels.WorkSessionStatusHomeOffice, firstBlock.Status,
		"the first block keeps its status")
	require.NotNil(t, firstBlock.CheckOutTime, "the first block stays closed")
}

func TestWSCheckIn_InvalidStatus(t *testing.T) {
	svc, _, _, _, _ := wsCreateTestService()

	session, err := svc.CheckIn(context.Background(), 100, "invalid_status", activeModels.WorkSessionSourceApp, "")
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
		activeModels.WorkSessionStatusPresent, "bogus", "")
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
		activeModels.WorkSessionStatusPresent, activeModels.WorkSessionSourceUnknown, "")
	require.Error(t, err)
	assert.Nil(t, session)
	assert.Contains(t, err.Error(), "source must be",
		"'unknown' is a read-only sentinel for legacy rows and must not be writable")
}

func TestWSCheckIn_RepoError(t *testing.T) {
	svc, sessionRepo, _, _, _ := wsCreateTestService()

	sessionRepo.listByStaffAndDateFunc = func(_ context.Context, _ int64, _ timezone.Date) ([]*activeModels.WorkSession, error) {
		return nil, errors.New("database error")
	}

	session, err := svc.CheckIn(context.Background(), 100, activeModels.WorkSessionStatusPresent, activeModels.WorkSessionSourceApp, "")
	require.Error(t, err)
	assert.Nil(t, session)
	assert.Contains(t, err.Error(), "failed to check existing sessions")
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

	session, err := svc.CheckOut(ctx, staffID, "")
	require.NoError(t, err)
	require.NotNil(t, session)
	assert.NotNil(t, session.CheckOutTime)
}

func TestWSCheckOut_NoActiveSession(t *testing.T) {
	svc, sessionRepo, _, _, _ := wsCreateTestService()

	sessionRepo.getCurrentByStaffIDFunc = func(_ context.Context, _ int64) (*activeModels.WorkSession, error) {
		return nil, sql.ErrNoRows
	}

	session, err := svc.CheckOut(context.Background(), 100, "")
	require.Error(t, err)
	assert.Nil(t, session)
	assert.Contains(t, err.Error(), "no active session found")
}

func TestWSCheckOut_NilSession(t *testing.T) {
	svc, sessionRepo, _, _, _ := wsCreateTestService()

	sessionRepo.getCurrentByStaffIDFunc = func(_ context.Context, _ int64) (*activeModels.WorkSession, error) {
		return nil, nil
	}

	session, err := svc.CheckOut(context.Background(), 100, "")
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

	session, err := svc.CheckOut(context.Background(), staffID, "")
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

func TestWSStartBreak_LocksCurrentSessionBeforeCreate(t *testing.T) {
	svc, sessionRepo, breakRepo, _, _ := wsCreateTestService()
	staffID := int64(100)

	sessionRepo.getCurrentByStaffIDFunc = func(_ context.Context, _ int64) (*activeModels.WorkSession, error) {
		t.Fatal("StartBreak must lock the active session before creating a break")
		return nil, nil
	}
	// Resolving which day the running block is filed on is a separate,
	// deliberately unlocked lookup; the row the break hangs on still comes
	// from the FOR UPDATE read below.
	sessionRepo.getLatestOpenByStaffIDFunc = func(_ context.Context, _ int64) (*activeModels.WorkSession, error) {
		return &activeModels.WorkSession{
			Model:       base.Model{ID: 50},
			StaffID:     staffID,
			Date:        timezone.TodayDate(),
			CheckInTime: time.Now().Add(-2 * time.Hour),
		}, nil
	}
	locked := false
	sessionRepo.getCurrentForUpdateFunc = func(_ context.Context, _ int64) (*activeModels.WorkSession, error) {
		locked = true
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
		require.True(t, locked, "session lock must happen before break insert")
		entity.ID = 10
		return nil
	}

	brk, err := svc.StartBreak(context.Background(), staffID, nil)
	require.NoError(t, err)
	require.NotNil(t, brk)
	assert.Equal(t, int64(50), brk.SessionID)
}

func TestWSStartBreak_CustomDurationSetsPlannedEnd(t *testing.T) {
	svc, sessionRepo, breakRepo, _, _ := wsCreateTestService()
	staffID := int64(100)
	durationMinutes := 90

	sessionRepo.getCurrentForUpdateFunc = func(_ context.Context, _ int64) (*activeModels.WorkSession, error) {
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
		require.NotNil(t, entity.PlannedEndTime)
		plannedDuration := entity.PlannedEndTime.Sub(entity.StartedAt)
		assert.InDelta(t, durationMinutes, plannedDuration.Minutes(), 0.1)
		entity.ID = 10
		return nil
	}

	brk, err := svc.StartBreak(context.Background(), staffID, &durationMinutes)
	require.NoError(t, err)
	require.NotNil(t, brk)
	assert.Equal(t, int64(50), brk.SessionID)
	assert.NotNil(t, brk.PlannedEndTime)
}

func TestWSStartBreak_RejectsCustomDurationAboveLimit(t *testing.T) {
	svc, sessionRepo, breakRepo, _, _ := wsCreateTestService()
	durationMinutes := 241

	sessionRepo.getCurrentForUpdateFunc = func(_ context.Context, _ int64) (*activeModels.WorkSession, error) {
		return &activeModels.WorkSession{
			Model:       base.Model{ID: 50},
			StaffID:     100,
			CheckInTime: time.Now().Add(-2 * time.Hour),
		}, nil
	}

	breakRepo.getActiveBySessionIDFunc = func(_ context.Context, _ int64) (*activeModels.WorkSessionBreak, error) {
		return nil, nil
	}

	brk, err := svc.StartBreak(context.Background(), 100, &durationMinutes)
	require.Error(t, err)
	assert.Nil(t, brk)
	assert.Contains(t, err.Error(), "planned_duration_minutes must be between 1 and 240")
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

// A block that crossed Berlin midnight keeps its own (yesterday's) date. The
// break actions must follow it there — resolving "today" from the clock would
// report "no active session found" to somebody who is demonstrably clocked in.
func TestWSStartBreak_FollowsBlockOpenedOnAnEarlierDay(t *testing.T) {
	yesterday := timezone.NewDate(2026, 7, 21)
	svc, sessionRepo, breakRepo, _, _ := wsCreateTestService()
	svc.nowFunc = func() time.Time { return time.Date(2026, time.July, 22, 1, 0, 0, 0, timezone.Berlin) }

	sessionRepo.getCurrentByStaffIDFunc = func(_ context.Context, _ int64) (*activeModels.WorkSession, error) {
		return nil, sql.ErrNoRows // nothing is filed on today
	}
	sessionRepo.getLatestOpenByStaffIDFunc = func(_ context.Context, _ int64) (*activeModels.WorkSession, error) {
		return &activeModels.WorkSession{
			Model:       base.Model{ID: 77},
			StaffID:     100,
			Date:        yesterday,
			CheckInTime: time.Date(2026, time.July, 21, 22, 0, 0, 0, timezone.Berlin),
		}, nil
	}
	var lockedDay timezone.Date
	sessionRepo.getOpenByStaffAndDateForUpdateFunc = func(_ context.Context, _ int64, day timezone.Date) (*activeModels.WorkSession, error) {
		lockedDay = day
		return &activeModels.WorkSession{
			Model:       base.Model{ID: 77},
			StaffID:     100,
			Date:        yesterday,
			CheckInTime: time.Date(2026, time.July, 21, 22, 0, 0, 0, timezone.Berlin),
		}, nil
	}
	breakRepo.getActiveBySessionIDFunc = func(_ context.Context, _ int64) (*activeModels.WorkSessionBreak, error) {
		return nil, nil
	}
	breakRepo.createFunc = func(_ context.Context, entity *activeModels.WorkSessionBreak) error {
		entity.ID = 10
		return nil
	}

	brk, err := svc.StartBreak(context.Background(), 100, nil)
	require.NoError(t, err)
	require.NotNil(t, brk)
	assert.Equal(t, int64(77), brk.SessionID)
	assert.Equal(t, yesterday, lockedDay)
}

func TestWSEndBreak_FollowsBlockOpenedOnAnEarlierDay(t *testing.T) {
	yesterday := timezone.NewDate(2026, 7, 21)
	svc, sessionRepo, breakRepo, _, _ := wsCreateTestService()
	svc.nowFunc = func() time.Time { return time.Date(2026, time.July, 22, 1, 0, 0, 0, timezone.Berlin) }

	open := &activeModels.WorkSession{
		Model:       base.Model{ID: 77},
		StaffID:     100,
		Date:        yesterday,
		CheckInTime: time.Date(2026, time.July, 21, 22, 0, 0, 0, timezone.Berlin),
	}
	sessionRepo.getCurrentByStaffIDFunc = func(_ context.Context, _ int64) (*activeModels.WorkSession, error) {
		return nil, sql.ErrNoRows // nothing is filed on today
	}
	sessionRepo.getLatestOpenByStaffIDFunc = func(_ context.Context, _ int64) (*activeModels.WorkSession, error) {
		return open, nil
	}
	var lookedUpDay timezone.Date
	sessionRepo.getOpenByStaffAndDateFunc = func(_ context.Context, _ int64, day timezone.Date) (*activeModels.WorkSession, error) {
		lookedUpDay = day
		return open, nil
	}
	breakRepo.getActiveBySessionIDFunc = func(_ context.Context, _ int64) (*activeModels.WorkSessionBreak, error) {
		return &activeModels.WorkSessionBreak{
			Model:     base.Model{ID: 1},
			SessionID: 77,
			StartedAt: time.Date(2026, time.July, 22, 0, 30, 0, 0, timezone.Berlin),
		}, nil
	}
	breakRepo.endBreakFunc = func(_ context.Context, _ int64, _ time.Time, _ int) error { return nil }
	breakRepo.getBySessionIDFunc = func(_ context.Context, _ int64) ([]*activeModels.WorkSessionBreak, error) {
		return []*activeModels.WorkSessionBreak{}, nil
	}
	sessionRepo.updateBreakMinutesFunc = func(_ context.Context, _ int64, _ int) error { return nil }
	sessionRepo.findByIDFunc = func(_ context.Context, id any) (*activeModels.WorkSession, error) {
		return &activeModels.WorkSession{Model: base.Model{ID: id.(int64)}, StaffID: 100, Date: yesterday}, nil
	}

	session, err := svc.EndBreak(context.Background(), 100)
	require.NoError(t, err)
	require.NotNil(t, session)
	assert.Equal(t, yesterday, lookedUpDay)
}

func TestWSAutoEndExpiredBreaks_UsesPlannedEndAndRecalculatesBreakMinutes(t *testing.T) {
	svc, sessionRepo, breakRepo, _, _ := wsCreateTestService()
	startedAt := time.Now().Add(-2 * time.Hour)
	plannedEnd := startedAt.Add(90 * time.Minute)
	sessionID := int64(50)
	breakID := int64(10)
	endedBreak := &activeModels.WorkSessionBreak{
		Model:           base.Model{ID: breakID},
		SessionID:       sessionID,
		StartedAt:       startedAt,
		EndedAt:         &plannedEnd,
		DurationMinutes: 90,
	}

	breakRepo.getExpiredBreaksFunc = func(_ context.Context, before time.Time) ([]*activeModels.WorkSessionBreak, error) {
		assert.True(t, before.After(plannedEnd) || before.Equal(plannedEnd))
		return []*activeModels.WorkSessionBreak{
			{
				Model:          base.Model{ID: breakID},
				SessionID:      sessionID,
				StartedAt:      startedAt,
				PlannedEndTime: &plannedEnd,
			},
		}, nil
	}
	sessionRepo.findByIDFunc = func(_ context.Context, id any) (*activeModels.WorkSession, error) {
		assert.Equal(t, sessionID, id)
		return &activeModels.WorkSession{StaffID: 100}, nil
	}
	breakRepo.endBreakFunc = func(_ context.Context, id int64, endedAt time.Time, durationMinutes int) error {
		assert.Equal(t, breakID, id)
		assert.True(t, plannedEnd.Equal(endedAt))
		assert.Equal(t, 90, durationMinutes)
		return nil
	}
	breakRepo.getBySessionIDFunc = func(_ context.Context, id int64) ([]*activeModels.WorkSessionBreak, error) {
		assert.Equal(t, sessionID, id)
		return []*activeModels.WorkSessionBreak{endedBreak}, nil
	}
	sessionRepo.updateBreakMinutesFunc = func(_ context.Context, id int64, breakMinutes int) error {
		assert.Equal(t, sessionID, id)
		assert.Equal(t, 90, breakMinutes)
		return nil
	}

	count, err := svc.AutoEndExpiredBreaks(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestWSLockStaffBalanceWritesOrdered_SortsAndDeduplicates(t *testing.T) {
	svc, sessionRepo, _, _, _ := wsCreateTestService()
	var locked []int64
	sessionRepo.lockBalanceWritesFunc = func(_ context.Context, staffID int64) error {
		locked = append(locked, staffID)
		return nil
	}

	err := svc.lockStaffBalanceWritesOrdered(context.Background(), []int64{30, 10, 30, 20, 10})

	require.NoError(t, err)
	assert.Equal(t, []int64{10, 20, 30}, locked)
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

// WorkSession.BreakMinutes caches ENDED breaks only, so an open break is
// invisible to netMinutes and the day row would keep counting break time as
// worked time — climbing while the Monatskarte and the week KPI, which both
// deduct the running break server-side, stand still (#1842).
func TestWSGetHistory_DeductsRunningBreakFromNetMinutes(t *testing.T) {
	svc, sessionRepo, breakRepo, auditRepo, _ := wsCreateTestService()
	staffID := int64(100)
	// Open session, checked in 4h ago: 30 min of ended breaks (in the cache)
	// plus a break that started 20 min ago and is still running.
	checkIn := time.Now().Add(-4 * time.Hour)
	breakStart := time.Now().Add(-20 * time.Minute)
	endedBreakEnd := time.Now().Add(-2 * time.Hour)

	sessionRepo.getHistoryByStaffIDFunc = func(_ context.Context, _ int64, _, _ timezone.Date) ([]*activeModels.WorkSession, error) {
		return []*activeModels.WorkSession{
			{
				Model:        base.Model{ID: 1},
				StaffID:      staffID,
				Date:         timezone.TodayDate(),
				CheckInTime:  checkIn,
				BreakMinutes: 30,
			},
		}, nil
	}
	auditRepo.countManualBySessionIDsFunc = func(_ context.Context, _ []int64) (map[int64]int, error) {
		return map[int64]int{}, nil
	}
	auditRepo.countBySessionIDsFunc = func(_ context.Context, _ []int64) (map[int64]int, error) {
		return map[int64]int{}, nil
	}
	breakRepo.getBySessionIDFunc = func(_ context.Context, _ int64) ([]*activeModels.WorkSessionBreak, error) {
		return []*activeModels.WorkSessionBreak{
			{Model: base.Model{ID: 1}, SessionID: 1, StartedAt: time.Now().Add(-150 * time.Minute), EndedAt: &endedBreakEnd, DurationMinutes: 30},
			{Model: base.Model{ID: 2}, SessionID: 1, StartedAt: breakStart},
		}, nil
	}

	historyResp, err := svc.GetHistory(context.Background(), staffID, timezone.TodayDate(), timezone.TodayDate())
	require.NoError(t, err)
	require.Len(t, historyResp.Sessions, 1)

	// 240 gross − 30 ended − 20 running = 190.
	assert.InDelta(t, 190, historyResp.Sessions[0].NetMinutes, 1,
		"the running break must be deducted, exactly as the Monatskarte does")
	assert.InDelta(t, 190, historyResp.WeeklySummaries[0].TotalNetMinutes, 1,
		"the weekly summary aggregates the corrected value")
	// The reader must be able to add the row up: the Ist above already stopped
	// growing, so reporting the raw ENDED-breaks cache (30) as the pause would
	// print "Pause 0:30" against 20 minutes of deducted time and break
	// gross = net + Pause on screen (#1842).
	assert.InDelta(t, 50, historyResp.Sessions[0].BreakMinutes, 1,
		"the displayed pause must include the running break")
}

// The pause total is what the day row prints, so it has to survive JSON: the
// response embeds *WorkSession, whose own break_minutes tag would win if the
// shadowing field were ever removed — and the row would silently fall back to
// the ENDED-breaks cache while NetMinutes stayed corrected (#1842).
func TestWSGetHistory_SerializesRunningBreakInBreakMinutes(t *testing.T) {
	svc, sessionRepo, breakRepo, auditRepo, _ := wsCreateTestService()
	staffID := int64(100)
	checkIn := time.Now().Add(-2 * time.Hour)

	sessionRepo.getHistoryByStaffIDFunc = func(_ context.Context, _ int64, _, _ timezone.Date) ([]*activeModels.WorkSession, error) {
		return []*activeModels.WorkSession{
			{Model: base.Model{ID: 1}, StaffID: staffID, Date: timezone.TodayDate(), CheckInTime: checkIn},
		}, nil
	}
	auditRepo.countManualBySessionIDsFunc = func(_ context.Context, _ []int64) (map[int64]int, error) {
		return map[int64]int{}, nil
	}
	auditRepo.countBySessionIDsFunc = func(_ context.Context, _ []int64) (map[int64]int, error) {
		return map[int64]int{}, nil
	}
	breakRepo.getBySessionIDFunc = func(_ context.Context, _ int64) ([]*activeModels.WorkSessionBreak, error) {
		return []*activeModels.WorkSessionBreak{
			{Model: base.Model{ID: 1}, SessionID: 1, StartedAt: time.Now().Add(-15 * time.Minute)},
		}, nil
	}

	historyResp, err := svc.GetHistory(context.Background(), staffID, timezone.TodayDate(), timezone.TodayDate())
	require.NoError(t, err)
	require.Len(t, historyResp.Sessions, 1)

	encoded, err := json.Marshal(historyResp.Sessions[0])
	require.NoError(t, err)
	var wire struct {
		BreakMinutes int `json:"break_minutes"`
		NetMinutes   int `json:"net_minutes"`
	}
	require.NoError(t, json.Unmarshal(encoded, &wire))
	assert.InDelta(t, 15, wire.BreakMinutes, 1, "break_minutes must carry the running break")
	assert.InDelta(t, 105, wire.NetMinutes, 1)
}

// A checked-out session is final: its breaks are all ended and folded into the
// cache, so nothing may be deducted twice.
func TestWSGetHistory_ClosedSessionKeepsCachedBreaks(t *testing.T) {
	svc, sessionRepo, breakRepo, auditRepo, _ := wsCreateTestService()
	staffID := int64(100)
	checkIn := time.Now().Add(-8 * time.Hour)
	checkOut := time.Now().Add(-2 * time.Hour)
	breakEnd := time.Now().Add(-5 * time.Hour)

	sessionRepo.getHistoryByStaffIDFunc = func(_ context.Context, _ int64, _, _ timezone.Date) ([]*activeModels.WorkSession, error) {
		return []*activeModels.WorkSession{
			{
				Model:        base.Model{ID: 1},
				StaffID:      staffID,
				Date:         timezone.TodayDate(),
				CheckInTime:  checkIn,
				CheckOutTime: &checkOut,
				BreakMinutes: 30,
			},
		}, nil
	}
	auditRepo.countManualBySessionIDsFunc = func(_ context.Context, _ []int64) (map[int64]int, error) {
		return map[int64]int{}, nil
	}
	auditRepo.countBySessionIDsFunc = func(_ context.Context, _ []int64) (map[int64]int, error) {
		return map[int64]int{}, nil
	}
	breakRepo.getBySessionIDFunc = func(_ context.Context, _ int64) ([]*activeModels.WorkSessionBreak, error) {
		return []*activeModels.WorkSessionBreak{
			{Model: base.Model{ID: 1}, SessionID: 1, StartedAt: time.Now().Add(-330 * time.Minute), EndedAt: &breakEnd, DurationMinutes: 30},
		}, nil
	}

	historyResp, err := svc.GetHistory(context.Background(), staffID, timezone.TodayDate(), timezone.TodayDate())
	require.NoError(t, err)
	require.Len(t, historyResp.Sessions, 1)
	// 360 gross − 30 cached = 330, unchanged.
	assert.InDelta(t, 330, historyResp.Sessions[0].NetMinutes, 1)
}

func TestWSGetHistory_UsesRotationWeekTargets(t *testing.T) {
	svc, sessionRepo, breakRepo, auditRepo, _ := wsCreateTestService()
	staffID := int64(100)
	wednesdayWeekZero := time.Date(2026, 6, 3, 8, 0, 0, 0, time.UTC)
	mondayWeekOne := time.Date(2026, 6, 8, 8, 0, 0, 0, time.UTC)
	checkOutWeekZero := wednesdayWeekZero.Add(20 * time.Hour)
	checkOutWeekOne := mondayWeekOne.Add(30 * time.Hour)

	svc.scheduleRepo = &wsMockStaffWorkScheduleRepository{
		findByStaffIDsValidInRangeFunc: func(_ context.Context, _ []int64, _, _ timezone.Date) ([]*configModels.StaffWorkSchedule, error) {
			// The shared resolver applies validity per day (the old per-day
			// mock ignored its date argument), so the window must cover the
			// full first week; the Monday anchor stays in the same ISO week.
			validFrom := timezone.NewDate(2026, 6, 1)
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
		findByStaffIDsValidInRangeFunc: func(_ context.Context, _ []int64, _, _ timezone.Date) ([]*configModels.StaffWorkSchedule, error) {
			// The old per-day mock branched on its date argument; the batched
			// read returns both generations and the shared resolver applies
			// the validity windows per day (valid_until exclusive).
			return []*configModels.StaffWorkSchedule{
				{
					StaffID:        staffID,
					WeekIndex:      0,
					RotationLength: 1,
					DayOfWeek:      configModels.DayWednesday,
					TargetMinutes:  20 * 60,
					ValidFrom:      timezone.NewDate(2026, 1, 1),
					ValidUntil:     &changeDate,
				},
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

	svc.staffRepo = &testpkg.StaffRepoMock{
		FindByIDFn: func(_ context.Context, _ any) (*userModels.Staff, error) {
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
		findByStaffIDsValidInRangeFunc: func(_ context.Context, _ []int64, _, _ timezone.Date) ([]*configModels.StaffWorkSchedule, error) {
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
	staleDay := timezone.TodayDate().AddDays(-2)

	sessionRepo.getOpenSessionsFunc = func(_ context.Context, _ timezone.Date) ([]*activeModels.WorkSession, error) {
		return []*activeModels.WorkSession{
			{Model: base.Model{ID: 1}, Date: staleDay},
			{Model: base.Model{ID: 2}, Date: staleDay},
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

func TestWSCleanupOpenSessions_KeepsYesterdayOpenForNightBlocks(t *testing.T) {
	svc, sessionRepo, _, _, _ := wsCreateTestService()
	today := timezone.NewDate(2026, 8, 18)
	svc.nowFunc = func() time.Time { return time.Date(2026, time.August, 18, 8, 0, 0, 0, timezone.Berlin) }

	var beforeDate timezone.Date
	sessionRepo.getOpenSessionsFunc = func(_ context.Context, before timezone.Date) ([]*activeModels.WorkSession, error) {
		beforeDate = before
		return nil, nil
	}

	count, err := svc.CleanupOpenSessions(context.Background())
	require.NoError(t, err)
	assert.Zero(t, count)
	assert.Equal(t, today.AddDays(-1), beforeDate)
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

// Starting a supervision while a block from an earlier day is still running
// must return that block. Looking only at today would let the auto-stamp run
// into the check-in guard and fail the supervision start.
func TestWSEnsureCheckedIn_ReturnsBlockOpenedOnAnEarlierDay(t *testing.T) {
	svc, sessionRepo, _, _, _ := wsCreateTestService()
	staffID := int64(100)
	yesterday := timezone.NewDate(2026, 7, 21)
	svc.nowFunc = func() time.Time { return time.Date(2026, time.July, 22, 6, 0, 0, 0, timezone.Berlin) }

	sessionRepo.getLatestOpenByStaffIDFunc = func(_ context.Context, _ int64) (*activeModels.WorkSession, error) {
		return &activeModels.WorkSession{
			Model:       base.Model{ID: 77},
			StaffID:     staffID,
			Date:        yesterday,
			CheckInTime: time.Date(2026, time.July, 21, 22, 0, 0, 0, timezone.Berlin),
		}, nil
	}
	sessionRepo.createFunc = func(_ context.Context, _ *activeModels.WorkSession) error {
		t.Fatal("the running block must be reused, not doubled")
		return nil
	}

	session, err := svc.EnsureCheckedIn(context.Background(), staffID, activeModels.WorkSessionSourceNFC)
	require.NoError(t, err)
	require.NotNil(t, session)
	assert.Equal(t, int64(77), session.ID)
}

func TestWSEnsureCheckedIn_AlreadyCheckedOutToday(t *testing.T) {
	svc, sessionRepo, _, _, _ := wsCreateTestService()
	staffID := int64(100)
	checkOut := time.Now().Add(-1 * time.Hour)

	sessionRepo.getCurrentByStaffIDFunc = func(_ context.Context, _ int64) (*activeModels.WorkSession, error) {
		return nil, sql.ErrNoRows
	}

	sessionRepo.listByStaffAndDateFunc = func(_ context.Context, _ int64, _ timezone.Date) ([]*activeModels.WorkSession, error) {
		return []*activeModels.WorkSession{{
			Model:        base.Model{ID: 1},
			StaffID:      staffID,
			CheckOutTime: &checkOut,
		}}, nil
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

	sessionRepo.listByStaffAndDateFunc = func(_ context.Context, _ int64, _ timezone.Date) ([]*activeModels.WorkSession, error) {
		return nil, nil
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
	sessionRepo.listByStaffAndDateFunc = func(_ context.Context, _ int64, _ timezone.Date) ([]*activeModels.WorkSession, error) {
		return nil, nil
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
	broadcaster := testpkg.NewRecordingBroadcaster()
	svc.SetBroadcaster(broadcaster)
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

	session, err := svc.UpdateSession(
		tenant.WithTenantID(context.Background(), 43),
		staffID,
		sessionID,
		updates,
	)
	require.NoError(t, err)
	require.NotNil(t, session)
	require.Len(t, broadcaster.EventsOfType(realtime.EventStaffTimeTrackingChanged), 1)
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
		require.NotNil(t, edits[0].OldValue)
		require.NotNil(t, edits[0].NewValue)
		assert.Equal(t, "30", *edits[0].OldValue)
		assert.Equal(t, "45", *edits[0].NewValue)
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

func (m *wsMockStaffAbsenceRepository) ListNonHistoricalByStaffID(context.Context, int64, timezone.Date) ([]*activeModels.StaffAbsence, error) {
	return nil, nil
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

// ============================================================================
// F9 Deviation-Reason Gate Tests (CheckIn/CheckOut vs. planned shifts)
// ============================================================================

// wsDeviationSettings enables operations.time_tracking_require_deviation_reason
// and answers the tolerance lookup; every other bool setting (e.g. the
// planned-start enforcement consulted on the same code path) stays off.
func wsDeviationSettings(toleranceMinutes int) *wsMockSettingsResolver {
	return &wsMockSettingsResolver{
		resolveBoolFunc: func(_ context.Context, key string) (bool, error) {
			return key == configModels.KeyTimeTrackingRequireDeviationReason, nil
		},
		resolveIntFunc: func(_ context.Context, key string) (int, error) {
			return toleranceMinutes, nil
		},
	}
}

func TestWSCheckOut_DeviationGate(t *testing.T) {
	day := timezone.NewDate(2026, 7, 6) // Monday
	shift := shiftFor(100, day, 8, 16)  // planned 08:00-16:00

	tests := []struct {
		name         string
		now          time.Time
		shifts       []*scheduleModels.StaffShift
		settings     *wsMockSettingsResolver
		reason       string
		wantErr      bool
		wantMinutes  int
		wantAudit    bool
		wantAuditOld string
		wantAuditNew string
	}{
		{
			name:     "within tolerance saves without reason",
			now:      time.Date(2026, time.July, 6, 16, 10, 0, 0, timezone.Berlin),
			shifts:   []*scheduleModels.StaffShift{shift},
			settings: wsDeviationSettings(15),
		},
		{
			name:        "outside tolerance without reason rejects",
			now:         time.Date(2026, time.July, 6, 16, 30, 0, 0, timezone.Berlin),
			shifts:      []*scheduleModels.StaffShift{shift},
			settings:    wsDeviationSettings(15),
			wantErr:     true,
			wantMinutes: 30,
		},
		{
			name:         "outside tolerance with reason saves and audits",
			now:          time.Date(2026, time.July, 6, 16, 30, 0, 0, timezone.Berlin),
			shifts:       []*scheduleModels.StaffShift{shift},
			settings:     wsDeviationSettings(15),
			reason:       "Elterngespräch lief länger",
			wantAudit:    true,
			wantAuditOld: "16:00",
			wantAuditNew: "16:30",
		},
		{
			name:     "no shift that day skips the gate",
			now:      time.Date(2026, time.July, 6, 22, 0, 0, 0, timezone.Berlin),
			shifts:   nil,
			settings: wsDeviationSettings(15),
		},
		{
			// Leaving early is missing time, visible in the saldo — F9 only
			// gates "später gehen".
			name:     "early leave is not gated",
			now:      time.Date(2026, time.July, 6, 15, 0, 0, 0, timezone.Berlin),
			shifts:   []*scheduleModels.StaffShift{shift},
			settings: wsDeviationSettings(15),
		},
		{
			name:   "latest shift end is the reference",
			now:    time.Date(2026, time.July, 6, 16, 30, 0, 0, timezone.Berlin),
			shifts: []*scheduleModels.StaffShift{shiftFor(100, day, 8, 12), shift},
			// 16:30 vs latest end 16:00 = 30 min > 15 → reject without reason
			settings:    wsDeviationSettings(15),
			wantErr:     true,
			wantMinutes: 30,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, sessionRepo, breakRepo, auditRepo, supervisorRepo := wsCreateTestService()
			svc.nowFunc = func() time.Time { return tt.now }
			svc.settings = tt.settings
			shiftRepo := &wsMockStaffShiftRepository{}
			shiftRepo.findByStaffIDsAndDateFunc = func(_ context.Context, staffIDs []int64, date timezone.Date) ([]*scheduleModels.StaffShift, error) {
				assert.Equal(t, []int64{100}, staffIDs)
				assert.Equal(t, day, date)
				return tt.shifts, nil
			}
			svc.SetStaffShiftRepo(shiftRepo)

			sessionRepo.getCurrentByStaffIDFunc = func(_ context.Context, _ int64) (*activeModels.WorkSession, error) {
				return &activeModels.WorkSession{
					Model:       base.Model{ID: 1},
					StaffID:     100,
					Date:        day,
					CheckInTime: time.Date(2026, time.July, 6, 8, 0, 0, 0, timezone.Berlin),
				}, nil
			}
			breakRepo.getActiveBySessionIDFunc = func(_ context.Context, _ int64) (*activeModels.WorkSessionBreak, error) {
				return nil, nil
			}
			closed := false
			sessionRepo.closeSessionFunc = func(_ context.Context, _ int64, _ time.Time, _ bool) (bool, error) {
				closed = true
				return true, nil
			}
			supervisorRepo.endAllActiveByStaffIDFunc = func(_ context.Context, _ int64) (int, error) {
				return 0, nil
			}
			sessionRepo.findByIDFunc = func(_ context.Context, id any) (*activeModels.WorkSession, error) {
				checkOut := tt.now
				return &activeModels.WorkSession{
					Model:        base.Model{ID: id.(int64)},
					StaffID:      100,
					Date:         day,
					CheckInTime:  time.Date(2026, time.July, 6, 8, 0, 0, 0, timezone.Berlin),
					CheckOutTime: &checkOut,
				}, nil
			}
			var capturedEdits []*auditModels.WorkSessionEdit
			auditRepo.createBatchFunc = func(_ context.Context, edits []*auditModels.WorkSessionEdit) error {
				capturedEdits = edits
				return nil
			}

			session, err := svc.CheckOut(context.Background(), 100, tt.reason)

			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, session)
				var devErr *DeviationReasonRequiredError
				require.ErrorAs(t, err, &devErr)
				assert.Equal(t, "check_out", devErr.Action)
				assert.Equal(t, "16:00", devErr.PlannedTime)
				assert.Equal(t, "16:30", devErr.ActualTime)
				assert.Equal(t, tt.wantMinutes, devErr.DeviationMinutes)
				assert.False(t, closed, "a rejected check-out must not close the session")
				return
			}

			require.NoError(t, err)
			require.NotNil(t, session)
			assert.True(t, closed)
			if tt.wantAudit {
				require.Len(t, capturedEdits, 1)
				edit := capturedEdits[0]
				assert.Equal(t, auditModels.FieldDeviationReason, edit.FieldName)
				require.NotNil(t, edit.OldValue)
				assert.Equal(t, tt.wantAuditOld, *edit.OldValue)
				require.NotNil(t, edit.NewValue)
				assert.Equal(t, tt.wantAuditNew, *edit.NewValue)
				require.NotNil(t, edit.Notes)
				assert.Equal(t, tt.reason, *edit.Notes)
				assert.Equal(t, int64(100), edit.EditedBy)
			} else {
				assert.Empty(t, capturedEdits, "no deviation, no audit edit")
			}
		})
	}
}

func TestWSCheckOut_DeviationGateOffSkipsShiftLookup(t *testing.T) {
	svc, sessionRepo, breakRepo, _, supervisorRepo := wsCreateTestService()
	svc.settings = &wsMockSettingsResolver{} // setting resolves to false
	lookedUp := false
	shiftRepo := &wsMockStaffShiftRepository{}
	shiftRepo.findByStaffIDsAndDateFunc = func(_ context.Context, _ []int64, _ timezone.Date) ([]*scheduleModels.StaffShift, error) {
		lookedUp = true
		return nil, nil
	}
	svc.SetStaffShiftRepo(shiftRepo)

	sessionRepo.getCurrentByStaffIDFunc = func(_ context.Context, _ int64) (*activeModels.WorkSession, error) {
		return &activeModels.WorkSession{
			Model:       base.Model{ID: 1},
			StaffID:     100,
			Date:        timezone.TodayDate(),
			CheckInTime: time.Now().Add(-4 * time.Hour),
		}, nil
	}
	breakRepo.getActiveBySessionIDFunc = func(_ context.Context, _ int64) (*activeModels.WorkSessionBreak, error) {
		return nil, nil
	}
	sessionRepo.closeSessionFunc = func(_ context.Context, _ int64, _ time.Time, _ bool) (bool, error) {
		return true, nil
	}
	supervisorRepo.endAllActiveByStaffIDFunc = func(_ context.Context, _ int64) (int, error) {
		return 0, nil
	}
	sessionRepo.findByIDFunc = func(_ context.Context, id any) (*activeModels.WorkSession, error) {
		checkOut := time.Now()
		return &activeModels.WorkSession{
			Model:        base.Model{ID: id.(int64)},
			StaffID:      100,
			CheckOutTime: &checkOut,
		}, nil
	}

	session, err := svc.CheckOut(context.Background(), 100, "")
	require.NoError(t, err)
	require.NotNil(t, session)
	assert.False(t, lookedUp, "disabled setting must not load shifts")
}

func TestWSCheckIn_DeviationGate(t *testing.T) {
	day := timezone.NewDate(2026, 7, 6) // Monday
	shift := shiftFor(100, day, 8, 16)

	tests := []struct {
		name        string
		now         time.Time
		reason      string
		extraShift  bool
		wantErr     bool
		wantMinutes int
		wantAudit   bool
	}{
		{
			name:        "early beyond tolerance without reason rejects",
			now:         time.Date(2026, time.July, 6, 7, 30, 0, 0, timezone.Berlin),
			wantErr:     true,
			wantMinutes: 30,
		},
		{
			name:      "early beyond tolerance with reason saves and audits",
			now:       time.Date(2026, time.July, 6, 7, 30, 0, 0, timezone.Berlin),
			reason:    "Frühdienst übernommen",
			wantAudit: true,
		},
		{
			name: "early within tolerance saves without reason",
			now:  time.Date(2026, time.July, 6, 7, 50, 0, 0, timezone.Berlin),
		},
		{
			name: "late arrival is not gated",
			now:  time.Date(2026, time.July, 6, 8, 30, 0, 0, timezone.Berlin),
		},
		{
			// With a second, later shift the EARLIEST start stays the
			// check-in reference.
			name:        "earliest shift start is the reference",
			now:         time.Date(2026, time.July, 6, 7, 30, 0, 0, timezone.Berlin),
			extraShift:  true,
			wantErr:     true,
			wantMinutes: 30,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, sessionRepo, _, auditRepo, _ := wsCreateTestService()
			svc.nowFunc = func() time.Time { return tt.now }
			svc.settings = wsDeviationSettings(15)
			shiftRepo := &wsMockStaffShiftRepository{}
			shiftRepo.findByStaffIDsAndDateFunc = func(_ context.Context, _ []int64, _ timezone.Date) ([]*scheduleModels.StaffShift, error) {
				shifts := []*scheduleModels.StaffShift{shift}
				if tt.extraShift {
					shifts = append(shifts, shiftFor(100, day, 13, 16))
				}
				return shifts, nil
			}
			svc.SetStaffShiftRepo(shiftRepo)

			sessionRepo.listByStaffAndDateFunc = func(_ context.Context, _ int64, _ timezone.Date) ([]*activeModels.WorkSession, error) {
				return nil, nil
			}
			created := false
			sessionRepo.createFunc = func(_ context.Context, entity *activeModels.WorkSession) error {
				created = true
				entity.ID = 10
				return nil
			}
			var capturedEdits []*auditModels.WorkSessionEdit
			auditRepo.createBatchFunc = func(_ context.Context, edits []*auditModels.WorkSessionEdit) error {
				capturedEdits = edits
				return nil
			}

			session, err := svc.CheckIn(context.Background(), 100, activeModels.WorkSessionStatusPresent, activeModels.WorkSessionSourceApp, tt.reason)

			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, session)
				var devErr *DeviationReasonRequiredError
				require.ErrorAs(t, err, &devErr)
				assert.Equal(t, "check_in", devErr.Action)
				assert.Equal(t, "08:00", devErr.PlannedTime)
				assert.Equal(t, "07:30", devErr.ActualTime)
				assert.Equal(t, tt.wantMinutes, devErr.DeviationMinutes)
				assert.False(t, created, "a rejected check-in must not create a session")
				return
			}

			require.NoError(t, err)
			require.NotNil(t, session)
			assert.True(t, created)
			if tt.wantAudit {
				require.Len(t, capturedEdits, 1)
				assert.Equal(t, auditModels.FieldDeviationReason, capturedEdits[0].FieldName)
				require.NotNil(t, capturedEdits[0].Notes)
				assert.Equal(t, tt.reason, *capturedEdits[0].Notes)
			} else {
				assert.Empty(t, capturedEdits)
			}
		})
	}
}

func TestWSCheckIn_SecondBlockSkipsDeviationGate(t *testing.T) {
	// Starting a second block after a checkout resumes an already-started
	// work day, not a new arrival: even far outside the tolerance window no
	// reason is demanded (same exemption the pre-#2402 reopen path had).
	now := time.Date(2026, time.July, 6, 16, 30, 0, 0, timezone.Berlin)
	day := timezone.NewDate(2026, 7, 6)

	svc, sessionRepo, _, _, _ := wsCreateTestService()
	svc.nowFunc = func() time.Time { return now }
	svc.settings = wsDeviationSettings(15)
	shiftRepo := &wsMockStaffShiftRepository{}
	shiftRepo.findByStaffIDsAndDateFunc = func(_ context.Context, _ []int64, _ timezone.Date) ([]*scheduleModels.StaffShift, error) {
		t.Fatal("a second block must not consult the deviation gate")
		return nil, nil
	}
	svc.SetStaffShiftRepo(shiftRepo)

	checkOut := time.Date(2026, time.July, 6, 12, 0, 0, 0, timezone.Berlin)
	sessionRepo.listByStaffAndDateFunc = func(_ context.Context, _ int64, _ timezone.Date) ([]*activeModels.WorkSession, error) {
		return []*activeModels.WorkSession{{
			Model:        base.Model{ID: 7},
			StaffID:      100,
			Date:         day,
			CreatedBy:    100,
			Status:       activeModels.WorkSessionStatusPresent,
			Source:       activeModels.WorkSessionSourceApp,
			CheckInTime:  time.Date(2026, time.July, 6, 8, 0, 0, 0, timezone.Berlin),
			CheckOutTime: &checkOut,
		}}, nil
	}
	sessionRepo.updateFunc = func(_ context.Context, _ *activeModels.WorkSession) error {
		t.Fatal("a second block must be created, never reopened")
		return nil
	}
	var created *activeModels.WorkSession
	sessionRepo.createFunc = func(_ context.Context, entity *activeModels.WorkSession) error {
		created = entity
		entity.ID = 8
		return nil
	}

	session, err := svc.CheckIn(context.Background(), 100, activeModels.WorkSessionStatusPresent, activeModels.WorkSessionSourceApp, "")
	require.NoError(t, err)
	require.NotNil(t, session)
	require.NotNil(t, created, "second check-in must create a new block")
	assert.Nil(t, session.CheckOutTime)
}

// A kiosk resolves its calendar day before it stamps. When the request crosses
// Berlin midnight on the way in, the day it pinned selects the row to reopen —
// but a session created fresh belongs to the day of its own check_in_time.
// Storing 21.07 next to a 22.07 stamp misfiles the session in the daily
// history, in shift and deviation lookups, and in every total keyed on the date
// column.
func TestWSCheckInOn_NewSessionUsesTheStampDay(t *testing.T) {
	pinnedDay := timezone.NewDate(2026, 7, 21)
	stampedAt := time.Date(2026, time.July, 22, 0, 0, 1, 0, timezone.Berlin)

	svc, sessionRepo, _, _, _ := wsCreateTestService()
	svc.nowFunc = func() time.Time { return stampedAt }

	var listCalls int
	sessionRepo.listFunc = func(_ context.Context, _ *base.QueryOptions) ([]*activeModels.WorkSession, error) {
		listCalls++
		return nil, nil
	}
	var created *activeModels.WorkSession
	sessionRepo.createFunc = func(_ context.Context, entity *activeModels.WorkSession) error {
		created = entity
		entity.ID = 5
		return nil
	}

	session, err := svc.CheckInOn(context.Background(), 100, pinnedDay, activeModels.WorkSessionStatusPresent, activeModels.WorkSessionSourceNFC, "")
	require.NoError(t, err)
	require.NotNil(t, session)

	assert.Equal(t, 1, listCalls, "the new block checks its stamp day for existing blocks")
	require.NotNil(t, created)
	assert.Equal(t, timezone.DateFromTime(stampedAt), created.Date)
	assert.Equal(t, timezone.DateFromTime(created.CheckInTime), created.Date)
}

// The pinned day may be stale by the time the stamp is written. A session that
// is still running on it is a night shift and stays the caller's business, but
// a closed row belongs to a day this arrival is no longer part of: reopening it
// would move yesterday's check-in and delete yesterday's checkout. The stamp
// must open its own day instead.
func TestWSCheckInOn_StalePinnedDayDoesNotReopenYesterday(t *testing.T) {
	pinnedDay := timezone.NewDate(2026, 7, 21)
	stampedAt := time.Date(2026, time.July, 22, 0, 0, 1, 0, timezone.Berlin)
	stampDay := timezone.DateFromTime(stampedAt)

	svc, sessionRepo, _, _, _ := wsCreateTestService()
	svc.nowFunc = func() time.Time { return stampedAt }

	checkOut := time.Date(2026, time.July, 21, 16, 0, 0, 0, timezone.Berlin)
	yesterday := &activeModels.WorkSession{
		Model:        base.Model{ID: 9},
		StaffID:      100,
		Date:         pinnedDay,
		CreatedBy:    100,
		Status:       activeModels.WorkSessionStatusPresent,
		Source:       activeModels.WorkSessionSourceNFC,
		CheckInTime:  time.Date(2026, time.July, 21, 8, 0, 0, 0, timezone.Berlin),
		CheckOutTime: &checkOut,
	}

	sessionRepo.listFunc = func(_ context.Context, _ *base.QueryOptions) ([]*activeModels.WorkSession, error) {
		return []*activeModels.WorkSession{yesterday}, nil
	}
	sessionRepo.updateFunc = func(_ context.Context, _ *activeModels.WorkSession) error {
		t.Fatal("a closed session of a past day must never be reopened")
		return nil
	}
	var created *activeModels.WorkSession
	sessionRepo.createFunc = func(_ context.Context, entity *activeModels.WorkSession) error {
		created = entity
		entity.ID = 11
		return nil
	}

	session, err := svc.CheckInOn(context.Background(), 100, pinnedDay, activeModels.WorkSessionStatusPresent, activeModels.WorkSessionSourceNFC, "")
	require.NoError(t, err)
	require.NotNil(t, session)

	require.NotNil(t, created)
	assert.Equal(t, stampDay, created.Date)
	assert.NotNil(t, yesterday.CheckOutTime, "yesterday's departure stays recorded")
}

func TestWSEnsureCheckedIn_SkipsDeviationGate(t *testing.T) {
	// The supervision auto-stamp has no way to collect a reason; an early
	// start must still produce a work session.
	now := time.Date(2026, time.July, 6, 7, 0, 0, 0, timezone.Berlin)

	svc, sessionRepo, _, _, _ := wsCreateTestService()
	svc.nowFunc = func() time.Time { return now }
	svc.settings = wsDeviationSettings(15)
	shiftRepo := &wsMockStaffShiftRepository{}
	shiftRepo.findByStaffIDsAndDateFunc = func(_ context.Context, _ []int64, _ timezone.Date) ([]*scheduleModels.StaffShift, error) {
		t.Fatal("EnsureCheckedIn must not consult the deviation gate")
		return nil, nil
	}
	svc.SetStaffShiftRepo(shiftRepo)

	sessionRepo.getCurrentByStaffIDFunc = func(_ context.Context, _ int64) (*activeModels.WorkSession, error) {
		return nil, sql.ErrNoRows
	}
	sessionRepo.listByStaffAndDateFunc = func(_ context.Context, _ int64, _ timezone.Date) ([]*activeModels.WorkSession, error) {
		return nil, nil
	}
	created := false
	sessionRepo.createFunc = func(_ context.Context, entity *activeModels.WorkSession) error {
		created = true
		entity.ID = 11
		return nil
	}

	session, err := svc.EnsureCheckedIn(context.Background(), 100, activeModels.WorkSessionSourceNFC)
	require.NoError(t, err)
	require.NotNil(t, session)
	assert.True(t, created)
}

// ============================================================================
// F8 Self-Edit Notes Gate Tests (UpdateSession time fields)
// ============================================================================

func TestWSUpdateSession_TimeChangeNotesGate(t *testing.T) {
	staffID := int64(100)
	sessionID := int64(100)
	oldCheckIn := time.Date(2026, time.July, 6, 8, 0, 0, 0, timezone.Berlin)
	newCheckIn := time.Date(2026, time.July, 6, 7, 0, 0, 0, timezone.Berlin)
	newBreak := 45

	tests := []struct {
		name      string
		settingOn bool
		updates   SessionUpdateRequest
		wantErr   bool
	}{
		{
			name:      "check-in change without notes rejects when setting on",
			settingOn: true,
			updates:   SessionUpdateRequest{CheckInTime: &newCheckIn},
			wantErr:   true,
		},
		{
			name:      "check-in change with notes passes",
			settingOn: true,
			updates:   SessionUpdateRequest{CheckInTime: &newCheckIn, Notes: wsStrPtr("Bus verpasst")},
		},
		{
			name:    "check-in change without notes passes when setting off",
			updates: SessionUpdateRequest{CheckInTime: &newCheckIn},
		},
		{
			name:      "unchanged resend without notes never trips the gate",
			settingOn: true,
			updates:   SessionUpdateRequest{CheckInTime: &oldCheckIn},
		},
		{
			name:      "break-minutes change without notes rejects when setting on",
			settingOn: true,
			updates:   SessionUpdateRequest{BreakMinutes: &newBreak},
			wantErr:   true,
		},
		{
			name:      "per-break duration edit without notes rejects when setting on",
			settingOn: true,
			updates:   SessionUpdateRequest{Breaks: []BreakDurationUpdate{{ID: 5, DurationMinutes: 20}}},
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, sessionRepo, breakRepo, auditRepo, _ := wsCreateTestService()
			if tt.settingOn {
				svc.settings = wsDeviationSettings(15)
			}

			checkOut := time.Date(2026, time.July, 6, 16, 0, 0, 0, timezone.Berlin)
			sessionRepo.findByIDFunc = func(_ context.Context, _ any) (*activeModels.WorkSession, error) {
				return &activeModels.WorkSession{
					Model:        base.Model{ID: sessionID},
					StaffID:      staffID,
					CheckInTime:  oldCheckIn,
					CheckOutTime: &checkOut,
					BreakMinutes: 30,
					Status:       activeModels.WorkSessionStatusPresent,
					Date:         timezone.NewDate(2026, 7, 6),
					CreatedBy:    staffID,
				}, nil
			}
			sessionRepo.updateFunc = func(_ context.Context, _ *activeModels.WorkSession) error { return nil }
			breakRepo.getBySessionIDFunc = func(_ context.Context, _ int64) ([]*activeModels.WorkSessionBreak, error) {
				ended := checkOut
				return []*activeModels.WorkSessionBreak{{
					Model:           base.Model{ID: 5},
					SessionID:       sessionID,
					StartedAt:       oldCheckIn.Add(4 * time.Hour),
					EndedAt:         &ended,
					DurationMinutes: 30,
				}}, nil
			}
			sessionRepo.updateBreakMinutesFunc = func(_ context.Context, _ int64, _ int) error { return nil }
			breakRepo.updateFunc = func(_ context.Context, _ *activeModels.WorkSessionBreak) error { return nil }
			auditRepo.createBatchFunc = func(_ context.Context, _ []*auditModels.WorkSessionEdit) error { return nil }

			session, err := svc.UpdateSession(context.Background(), staffID, sessionID, tt.updates)

			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, session)
				assert.Contains(t, err.Error(), "notes required when changing recorded times")
				return
			}
			require.NoError(t, err)
			require.NotNil(t, session)
		})
	}
}

// ============================================================================
// ApplyCustomScheduleRows anchor persistence (#1842)
// ============================================================================

func TestWSUpdateScheduleBroadcastsTimeTrackingChangeAfterCommit(t *testing.T) {
	svc, _, _, _, _ := wsCreateTestService()
	broadcaster := testpkg.NewRecordingBroadcaster()
	svc.SetBroadcaster(broadcaster)
	svc.scheduleRepo = &wsMockStaffWorkScheduleRepository{}
	svc.staffRepo = &testpkg.StaffRepoMock{
		UpdateFn: func(_ context.Context, _ *userModels.Staff) error { return nil },
	}
	ctx, commit := tenant.WithAfterCommitHooksForTest(
		tenant.WithTenantID(context.Background(), 42),
	)

	err := svc.UpdateSchedule(ctx, &userModels.Staff{Model: base.Model{ID: 100}}, ScheduleUpdateInput{
		Mode:           "custom",
		RotationLength: 1,
		Entries: []ScheduleEntry{{
			WeekIndex:     0,
			DayOfWeek:     configModels.DayMonday,
			TargetMinutes: 480,
		}},
	})
	require.NoError(t, err)
	assert.Empty(t, broadcaster.Events())

	commit()

	events := broadcaster.EventsOfType(realtime.EventStaffTimeTrackingChanged)
	require.Len(t, events, 1)
	calls := broadcaster.CallsByMethod("tenant")
	require.Len(t, calls, 1)
	assert.Equal(t, int64(42), calls[0].TenantID)
}

// A staff member saving a multi-week custom schedule with no anchor anywhere
// must still get one stamped onto the rows. Left NULL, the rows fall back to
// the staff-level anchor at read time — and a later template assignment writes
// exactly that, re-paritying these historical A/B weeks and moving the carry.
func TestWSApplyCustomScheduleRows_StampsAnchorForFirstRotation(t *testing.T) {
	svc, _, _, _, _ := wsCreateTestService()
	staff := &userModels.Staff{Model: base.Model{ID: 100}}

	var written timezone.Date
	svc.scheduleRepo = &wsMockStaffWorkScheduleRepository{
		replaceScheduleFunc: func(_ context.Context, _ int64, _ []*configModels.StaffWorkSchedule, anchor timezone.Date) error {
			written = anchor
			return nil
		},
	}
	svc.staffRepo = &testpkg.StaffRepoMock{
		UpdateFn: func(_ context.Context, _ *userModels.Staff) error { return nil },
	}

	entries := []*configModels.StaffWorkSchedule{
		{StaffID: staff.ID, WeekIndex: 0, RotationLength: 2, DayOfWeek: configModels.DayMonday, TargetMinutes: 480},
		{StaffID: staff.ID, WeekIndex: 1, RotationLength: 2, DayOfWeek: configModels.DayMonday, TargetMinutes: 240},
	}
	require.NoError(t, svc.ApplyCustomScheduleRows(context.Background(), staff, entries, timezone.Date{}))

	assert.Equal(t, timezone.TodayDate(), written, "rotational rows must carry the version's own anchor")
	require.NotNil(t, staff.RotationAnchorDate)
	assert.Equal(t, timezone.TodayDate(), *staff.RotationAnchorDate)
}

// A single-week schedule has no A/B parity, so it keeps a NULL anchor.
func TestWSApplyCustomScheduleRows_SingleWeekKeepsAnchorUnset(t *testing.T) {
	svc, _, _, _, _ := wsCreateTestService()
	staff := &userModels.Staff{Model: base.Model{ID: 100}}

	var written timezone.Date
	svc.scheduleRepo = &wsMockStaffWorkScheduleRepository{
		replaceScheduleFunc: func(_ context.Context, _ int64, _ []*configModels.StaffWorkSchedule, anchor timezone.Date) error {
			written = anchor
			return nil
		},
	}
	svc.staffRepo = &testpkg.StaffRepoMock{
		UpdateFn: func(_ context.Context, _ *userModels.Staff) error { return nil },
	}

	entries := []*configModels.StaffWorkSchedule{
		{StaffID: staff.ID, WeekIndex: 0, RotationLength: 1, DayOfWeek: configModels.DayMonday, TargetMinutes: 480},
	}
	require.NoError(t, svc.ApplyCustomScheduleRows(context.Background(), staff, entries, timezone.Date{}))

	assert.True(t, written.IsZero(), "single-week rows have no parity to anchor")
	assert.Nil(t, staff.RotationAnchorDate)
}

// An existing staff anchor still wins over today: the rows must be stamped
// with the anchor the schedule was actually being planned against.
func TestWSApplyCustomScheduleRows_ExistingStaffAnchorWins(t *testing.T) {
	svc, _, _, _, _ := wsCreateTestService()
	existing := timezone.NewDate(2026, 6, 1)
	staff := &userModels.Staff{Model: base.Model{ID: 100}, RotationAnchorDate: &existing}

	var written timezone.Date
	svc.scheduleRepo = &wsMockStaffWorkScheduleRepository{
		replaceScheduleFunc: func(_ context.Context, _ int64, _ []*configModels.StaffWorkSchedule, anchor timezone.Date) error {
			written = anchor
			return nil
		},
	}
	svc.staffRepo = &testpkg.StaffRepoMock{
		UpdateFn: func(_ context.Context, _ *userModels.Staff) error { return nil },
	}

	entries := []*configModels.StaffWorkSchedule{
		{StaffID: staff.ID, WeekIndex: 0, RotationLength: 2, DayOfWeek: configModels.DayMonday, TargetMinutes: 480},
	}
	require.NoError(t, svc.ApplyCustomScheduleRows(context.Background(), staff, entries, timezone.Date{}))

	assert.Equal(t, existing, written)
	require.NotNil(t, staff.RotationAnchorDate)
	assert.Equal(t, existing, *staff.RotationAnchorDate)
}

// recalcBreakMinutes must cache ENDED breaks only. A still-running break is
// live data that every reader re-derives against its own clock (netMinutes +
// runningBreakMinutes in the month service, the live session card). Folding
// its elapsed time into the cache made the Monatskarte subtract the same break
// twice — once from the cache, once live — which under-reported Ist and Saldo
// until the break was closed. Reachable whenever a recalc runs while a break
// is open: editing an ended break on a session whose second break still runs.
func TestWSRecalcBreakMinutes_ExcludesRunningBreak(t *testing.T) {
	svc, sessionRepo, breakRepo, _, _ := wsCreateTestService()
	ctx := context.Background()
	sessionID := int64(42)
	endedAt := time.Now().Add(-60 * time.Minute)

	breakRepo.getBySessionIDFunc = func(_ context.Context, _ int64) ([]*activeModels.WorkSessionBreak, error) {
		return []*activeModels.WorkSessionBreak{
			{
				Model:           base.Model{ID: 1},
				SessionID:       sessionID,
				StartedAt:       endedAt.Add(-30 * time.Minute),
				EndedAt:         &endedAt,
				DurationMinutes: 30,
			},
			{
				Model:     base.Model{ID: 2},
				SessionID: sessionID,
				StartedAt: time.Now().Add(-20 * time.Minute), // still running
			},
		}, nil
	}

	var cached int
	sessionRepo.updateBreakMinutesFunc = func(_ context.Context, _ int64, breakMinutes int) error {
		cached = breakMinutes
		return nil
	}

	require.NoError(t, svc.recalcBreakMinutes(ctx, sessionID))
	assert.Equal(t, 30, cached,
		"only the ended break belongs in the cache; the running break's 20 minutes would be double-counted by readers")
}

// GetHistoryByStaffIDs satisfies the batched interface method (#1417); this mock
// exercises the single-staff path only.
func (m *wsMockWorkSessionRepository) GetHistoryByStaffIDs(ctx context.Context, staffIDs []int64, from, to timezone.Date) (map[int64][]*activeModels.WorkSession, error) {
	if m.getHistoryByStaffIDsFunc != nil {
		return m.getHistoryByStaffIDsFunc(ctx, staffIDs, from, to)
	}
	return nil, nil
}

// GetActiveBySessionIDs satisfies the batched interface method (#1417); this mock
// exercises the single-staff path only.
func (m *wsMockWorkSessionBreakRepository) GetActiveBySessionIDs(context.Context, []int64) (map[int64]*activeModels.WorkSessionBreak, error) {
	return nil, nil
}

// GetByStaffIDsAndDateRange satisfies the batched interface method (#1417); this mock
// exercises the single-staff path only.
func (m *wsMockStaffAbsenceRepository) GetByStaffIDsAndDateRange(ctx context.Context, staffIDs []int64, from, to timezone.Date) (map[int64][]*activeModels.StaffAbsence, error) {
	if m.getByStaffIDsAndDateRangeFunc != nil {
		return m.getByStaffIDsAndDateRangeFunc(ctx, staffIDs, from, to)
	}
	return nil, nil
}

// FindStaffIDsWithScheduleHistory satisfies the batched interface method (#1417); this mock
// exercises the single-staff path only.
func (m *wsMockStaffWorkScheduleRepository) FindStaffIDsWithScheduleHistory(context.Context, []int64) (map[int64]bool, error) {
	return nil, nil
}

// ============================================================================
// Block Overlap Guard (#2402)
// ============================================================================

// wsClosedBlock builds a closed block on `day` between the given Berlin wall
// clock hours, for overlap-guard tests.
func wsClosedBlock(id int64, day timezone.Date, fromHour, toHour int) *activeModels.WorkSession {
	checkIn := time.Date(day.Year, time.Month(day.Month), day.Day, fromHour, 0, 0, 0, timezone.Berlin)
	checkOut := time.Date(day.Year, time.Month(day.Month), day.Day, toHour, 0, 0, 0, timezone.Berlin)
	return &activeModels.WorkSession{
		Model:        base.Model{ID: id},
		StaffID:      100,
		Date:         day,
		Status:       activeModels.WorkSessionStatusPresent,
		Source:       activeModels.WorkSessionSourceApp,
		CheckInTime:  checkIn,
		CheckOutTime: &checkOut,
		CreatedBy:    100,
	}
}

// wsOverlapping reproduces the SQL predicate of ListOverlappingByStaffID so a
// mocked repository answers the overlap query the way PostgreSQL does: a
// sibling intersects [from, to) unless it ends before it starts or starts
// after it ends. A nil `to` means the candidate runs open-ended.
func wsOverlapping(sessions []*activeModels.WorkSession, from time.Time, to *time.Time) []*activeModels.WorkSession {
	var out []*activeModels.WorkSession
	for _, s := range sessions {
		if s.CheckOutTime != nil && !s.CheckOutTime.After(from) {
			continue
		}
		if to != nil && !to.After(s.CheckInTime) {
			continue
		}
		out = append(out, s)
	}
	return out
}

func TestWSCreateSessionAsAdmin_RejectsOverlappingBlock(t *testing.T) {
	svc, sessionRepo, _, _, _ := wsCreateTestService()
	day := timezone.NewDate(2026, 8, 17)

	sessionRepo.listOverlappingByStaffIDFunc = func(_ context.Context, _ int64, _ time.Time, _ *time.Time) ([]*activeModels.WorkSession, error) {
		return []*activeModels.WorkSession{wsClosedBlock(1, day, 8, 12)}, nil
	}
	sessionRepo.createFunc = func(_ context.Context, _ *activeModels.WorkSession) error {
		t.Fatal("an overlapping Nachtrag must not be created")
		return nil
	}

	_, err := svc.CreateSessionAsAdmin(context.Background(), 10, 100, AdminCreateSessionRequest{
		Date:         time.Date(2026, time.August, 17, 0, 0, 0, 0, timezone.Berlin),
		CheckInTime:  time.Date(2026, time.August, 17, 11, 0, 0, 0, timezone.Berlin),
		CheckOutTime: time.Date(2026, time.August, 17, 14, 0, 0, 0, timezone.Berlin),
		Status:       activeModels.WorkSessionStatusPresent,
		Notes:        "Nachtrag",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "work session overlaps an existing block")
}

func TestWSCreateSessionAsAdmin_AllowsTouchingBlock(t *testing.T) {
	svc, sessionRepo, _, auditRepo, _ := wsCreateTestService()
	day := timezone.NewDate(2026, 8, 17)

	// The repository answers the overlap question itself, so a touching block
	// (08:00–12:00 vs a 12:00 start) is simply not part of the result.
	sessionRepo.listOverlappingByStaffIDFunc = func(_ context.Context, _ int64, from time.Time, to *time.Time) ([]*activeModels.WorkSession, error) {
		return wsOverlapping([]*activeModels.WorkSession{wsClosedBlock(1, day, 8, 12)}, from, to), nil
	}
	created := false
	sessionRepo.createFunc = func(_ context.Context, entity *activeModels.WorkSession) error {
		created = true
		entity.ID = 2
		return nil
	}
	auditRepo.createBatchFunc = func(_ context.Context, _ []*auditModels.WorkSessionEdit) error {
		return nil
	}

	// 12:00–16:00 touches the 08:00–12:00 block exactly — allowed.
	_, err := svc.CreateSessionAsAdmin(context.Background(), 10, 100, AdminCreateSessionRequest{
		Date:         time.Date(2026, time.August, 17, 0, 0, 0, 0, timezone.Berlin),
		CheckInTime:  time.Date(2026, time.August, 17, 12, 0, 0, 0, timezone.Berlin),
		CheckOutTime: time.Date(2026, time.August, 17, 16, 0, 0, 0, timezone.Berlin),
		Status:       activeModels.WorkSessionStatusPresent,
		Notes:        "Nachtrag",
	})
	require.NoError(t, err)
	assert.True(t, created)
}

func TestWSCheckIn_RejectsOverlapWithClosedFutureBlock(t *testing.T) {
	// A closed block can reach past "now" (an admin Nachtrag for the
	// afternoon, an edited checkout in the future). A check-in inside that
	// interval must be rejected, or the overlap double-counts in every sum
	// built from the day's rows.
	now := time.Date(2026, time.August, 17, 12, 0, 0, 0, timezone.Berlin)
	day := timezone.NewDate(2026, 8, 17)

	svc, sessionRepo, _, _, _ := wsCreateTestService()
	svc.nowFunc = func() time.Time { return now }

	sessionRepo.listOverlappingByStaffIDFunc = func(_ context.Context, _ int64, from time.Time, to *time.Time) ([]*activeModels.WorkSession, error) {
		return wsOverlapping([]*activeModels.WorkSession{wsClosedBlock(1, day, 8, 16)}, from, to), nil
	}
	sessionRepo.createFunc = func(_ context.Context, _ *activeModels.WorkSession) error {
		t.Fatal("a check-in inside a closed block must not be created")
		return nil
	}

	_, err := svc.CheckIn(context.Background(), 100, activeModels.WorkSessionStatusPresent, activeModels.WorkSessionSourceApp, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "work session overlaps an existing block")
}

// A block is filed on the day of its check-in, so a night block dated
// yesterday runs into this morning. The lookup asks for the candidate's own
// interval, not for a date window, so the night block is part of the answer.
func TestWSCheckIn_RejectsOverlapWithNightBlockOfTheDayBefore(t *testing.T) {
	now := time.Date(2026, time.August, 18, 1, 0, 0, 0, timezone.Berlin)
	yesterday := timezone.NewDate(2026, 8, 17)

	svc, sessionRepo, _, _, _ := wsCreateTestService()
	svc.nowFunc = func() time.Time { return now }

	nightEnd := time.Date(2026, time.August, 18, 2, 0, 0, 0, timezone.Berlin)
	night := wsClosedBlock(1, yesterday, 22, 23)
	night.CheckOutTime = &nightEnd

	var askedFrom time.Time
	var askedTo *time.Time
	sessionRepo.listOverlappingByStaffIDFunc = func(_ context.Context, _ int64, from time.Time, to *time.Time) ([]*activeModels.WorkSession, error) {
		askedFrom, askedTo = from, to
		return wsOverlapping([]*activeModels.WorkSession{night}, from, to), nil
	}
	sessionRepo.createFunc = func(_ context.Context, _ *activeModels.WorkSession) error {
		t.Fatal("a check-in inside yesterday's night block must not be created")
		return nil
	}

	_, err := svc.CheckIn(context.Background(), 100, activeModels.WorkSessionStatusPresent, activeModels.WorkSessionSourceApp, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "work session overlaps an existing block")
	assert.True(t, askedFrom.Equal(now), "the check-in stamp is the lower bound")
	assert.Nil(t, askedTo, "a fresh check-in has no end yet")
}

// A block that started days earlier still overlaps: an auto-checkout that
// never ran leaves an open block hanging, and a date window sized "one day
// back" would not see it. The lookup is bounded by the candidate's interval,
// not by a fixed number of days.
func TestWSCreateSessionAsAdmin_RejectsOverlapWithBlockFromDaysBefore(t *testing.T) {
	svc, sessionRepo, _, _, _ := wsCreateTestService()

	// Opened three days before the Nachtrag and never closed.
	stale := wsClosedBlock(1, timezone.NewDate(2026, 8, 14), 8, 12)
	stale.CheckOutTime = nil

	sessionRepo.listOverlappingByStaffIDFunc = func(_ context.Context, _ int64, from time.Time, to *time.Time) ([]*activeModels.WorkSession, error) {
		return wsOverlapping([]*activeModels.WorkSession{stale}, from, to), nil
	}
	sessionRepo.createFunc = func(_ context.Context, _ *activeModels.WorkSession) error {
		t.Fatal("a Nachtrag inside a still-open block must not be created")
		return nil
	}

	_, err := svc.CreateSessionAsAdmin(context.Background(), 10, 100, AdminCreateSessionRequest{
		Date:         time.Date(2026, time.August, 17, 0, 0, 0, 0, timezone.Berlin),
		CheckInTime:  time.Date(2026, time.August, 17, 9, 0, 0, 0, timezone.Berlin),
		CheckOutTime: time.Date(2026, time.August, 17, 12, 0, 0, 0, timezone.Berlin),
		Status:       activeModels.WorkSessionStatusPresent,
		Notes:        "Nachtrag",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "work session overlaps an existing block")
}

// The mirrored case: a Nachtrag that itself ends after midnight is compared
// against the following day's blocks too — its own end is the upper bound.
func TestWSCreateSessionAsAdmin_OverlapCoversTheEndDay(t *testing.T) {
	svc, sessionRepo, _, _, _ := wsCreateTestService()
	tomorrow := timezone.NewDate(2026, 8, 18)

	var askedFrom time.Time
	var askedTo *time.Time
	sessionRepo.listOverlappingByStaffIDFunc = func(_ context.Context, _ int64, from time.Time, to *time.Time) ([]*activeModels.WorkSession, error) {
		askedFrom, askedTo = from, to
		return wsOverlapping([]*activeModels.WorkSession{wsClosedBlock(1, tomorrow, 1, 6)}, from, to), nil
	}
	sessionRepo.createFunc = func(_ context.Context, _ *activeModels.WorkSession) error {
		t.Fatal("a Nachtrag running into an existing early block must not be created")
		return nil
	}

	checkIn := time.Date(2026, time.August, 17, 22, 0, 0, 0, timezone.Berlin)
	checkOut := time.Date(2026, time.August, 18, 3, 0, 0, 0, timezone.Berlin)
	_, err := svc.CreateSessionAsAdmin(context.Background(), 10, 100, AdminCreateSessionRequest{
		Date:         time.Date(2026, time.August, 17, 0, 0, 0, 0, timezone.Berlin),
		CheckInTime:  checkIn,
		CheckOutTime: checkOut,
		Status:       activeModels.WorkSessionStatusPresent,
		Notes:        "Nachtschicht",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "work session overlaps an existing block")
	assert.True(t, askedFrom.Equal(checkIn))
	require.NotNil(t, askedTo)
	assert.True(t, askedTo.Equal(checkOut), "the lookup reaches to the block's own end")
}

func TestWSUpdateSession_RejectsOverlapWithSiblingBlock(t *testing.T) {
	svc, sessionRepo, _, _, _ := wsCreateTestService()
	day := timezone.NewDate(2026, 8, 17)

	edited := wsClosedBlock(2, day, 13, 16)
	sibling := wsClosedBlock(1, day, 8, 12)

	sessionRepo.findByIDFunc = func(_ context.Context, _ any) (*activeModels.WorkSession, error) {
		return edited, nil
	}
	sessionRepo.listOverlappingByStaffIDFunc = func(_ context.Context, _ int64, from time.Time, to *time.Time) ([]*activeModels.WorkSession, error) {
		return wsOverlapping([]*activeModels.WorkSession{sibling, edited}, from, to), nil
	}
	sessionRepo.updateFunc = func(_ context.Context, _ *activeModels.WorkSession) error {
		t.Fatal("an update that overlaps a sibling block must not be persisted")
		return nil
	}

	// Pull the second block's check-in back into the first block.
	newCheckIn := time.Date(2026, time.August, 17, 11, 0, 0, 0, timezone.Berlin)
	_, err := svc.UpdateSession(context.Background(), 100, 2, SessionUpdateRequest{
		CheckInTime: &newCheckIn,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "work session overlaps an existing block")
}

func TestWSUpdateSession_RejectsOverlapBeforeWritingBreaks(t *testing.T) {
	svc, sessionRepo, breakRepo, _, _ := wsCreateTestService()
	day := timezone.NewDate(2026, 8, 17)
	edited := wsClosedBlock(2, day, 13, 16)
	sibling := wsClosedBlock(1, day, 8, 12)

	sessionRepo.findByIDFunc = func(_ context.Context, _ any) (*activeModels.WorkSession, error) { return edited, nil }
	sessionRepo.listOverlappingByStaffIDFunc = func(_ context.Context, _ int64, from time.Time, to *time.Time) ([]*activeModels.WorkSession, error) {
		return wsOverlapping([]*activeModels.WorkSession{sibling, edited}, from, to), nil
	}
	breakRepo.updateDurationFunc = func(context.Context, int64, int, time.Time) error {
		t.Fatal("a rejected overlap must not update a break")
		return nil
	}

	newCheckIn := time.Date(2026, time.August, 17, 11, 0, 0, 0, timezone.Berlin)
	_, err := svc.UpdateSession(context.Background(), 100, edited.ID, SessionUpdateRequest{
		CheckInTime: &newCheckIn,
		Breaks:      []BreakDurationUpdate{{ID: 1, DurationMinutes: 45}},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "work session overlaps an existing block")
}

// The frontend picks the block to edit by id (#2402). A JSON number would be
// parsed as a float64-backed JS number and rounded past 2^53 before the client
// could stringify it, so the wire has to carry the id quoted.
func TestWSSessionResponse_SerializesIDAsString(t *testing.T) {
	day := timezone.NewDate(2026, 8, 17)
	// Beyond Number.MAX_SAFE_INTEGER (9007199254740991): a numeric wire value
	// would come back as ...992 in the browser.
	session := wsClosedBlock(9007199254740993, day, 8, 12)

	raw, err := json.Marshal(&SessionResponse{WorkSession: session, NetMinutes: 240})
	require.NoError(t, err)

	assert.Contains(t, string(raw), `"id":"9007199254740993"`)

	var wire struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(raw, &wire))
	assert.Equal(t, "9007199254740993", wire.ID)
}
