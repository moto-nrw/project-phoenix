package compose

import (
	"context"
	"errors"
	"fmt"
	"time"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/application/timetracking"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The row adapters in this file serve the time-tracking row contracts
// (work sessions, breaks, balance adjustments, vacation openings and quotas)
// from the Workforce capability, like shift_rows.go does for the planning
// rows (#3413). They perform no persistence of their own.

// --- work sessions ---

type workSessionRepository struct{ workforce workforce.Capability }

// newWorkSessionRows serves timetracking.WorkSessionRepository from
// the Workforce capability.
func newWorkSessionRows(capability workforce.Capability) timetracking.WorkSessionRepository {
	if capability == nil {
		panic("work session repository adapter: Workforce capability is required")
	}
	return workSessionRepository{workforce: capability}
}

func (r workSessionRepository) Create(ctx context.Context, entity *timetracking.WorkSession) error {
	if entity == nil {
		return errors.New("WorkSession cannot be nil or zero value")
	}
	if err := entity.Validate(); err != nil {
		return err
	}
	created, err := r.workforce.CreateWorkSession(ctx, workSessionToCapability(entity))
	if err != nil {
		return workSessionWriteError("create", err)
	}
	applyWorkSessionToLegacy(entity, created)
	return nil
}

func (r workSessionRepository) FindByID(ctx context.Context, id any) (*timetracking.WorkSession, error) {
	sessionID, err := legacyID(id)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "find by id", Err: err}
	}
	value, err := r.workforce.FindWorkSession(ctx, sessionID)
	if err != nil {
		return nil, rowReadError("find by id", err, workforce.ErrWorkSessionNotFound)
	}
	return workSessionToLegacy(value), nil
}

func (r workSessionRepository) Update(ctx context.Context, entity *timetracking.WorkSession) error {
	if entity == nil {
		return errors.New("WorkSession cannot be nil or zero value")
	}
	if err := entity.Validate(); err != nil {
		return err
	}
	updated, err := r.workforce.UpdateWorkSession(ctx, workSessionToCapability(entity))
	if err != nil {
		if errors.Is(err, workforce.ErrWorkSessionNotFound) {
			return rowsAffectedError("update WorkSession")
		}
		return workSessionWriteError("update", err)
	}
	applyWorkSessionToLegacy(entity, updated)
	return nil
}

func (r workSessionRepository) Delete(ctx context.Context, id any) error {
	sessionID, err := legacyID(id)
	if err != nil {
		return &modelBase.DatabaseError{Op: "delete", Err: err}
	}
	if err := r.workforce.DeleteWorkSession(ctx, sessionID); err != nil {
		return workSessionWriteError("delete", err)
	}
	return nil
}

func (r workSessionRepository) ListWithFilter(ctx context.Context, filter workforce.WorkSessionFilter) ([]*timetracking.WorkSession, error) {
	return r.list(ctx, "list with options", filter)
}

func (r workSessionRepository) CountWithFilter(ctx context.Context, filter workforce.WorkSessionFilter) (int, error) {
	count, err := r.workforce.CountWorkSessions(ctx, filter)
	if err != nil {
		return 0, rowReadError("count with options", err, nil)
	}
	return count, nil
}

func (r workSessionRepository) OldestBefore(ctx context.Context, column string, cutoff *calendar.Date) (*calendar.Date, error) {
	before := ""
	if cutoff != nil {
		before = cutoff.String()
	}
	oldest, err := r.workforce.OldestWorkSessionDate(ctx, column, before)
	if err != nil {
		return nil, rowReadError("oldest before", err, nil)
	}
	if oldest == "" {
		return nil, nil
	}
	date, err := calendar.ParseDate(oldest)
	if err != nil {
		return nil, err
	}
	return &date, nil
}

func (r workSessionRepository) DeleteOlderThan(ctx context.Context, column string, cutoff calendar.Date) (int64, error) {
	deleted, err := r.workforce.DeleteWorkSessionsOlderThan(ctx, column, cutoff.String())
	if err != nil {
		return 0, workSessionWriteError("delete older than", err)
	}
	return deleted, nil
}

func (r workSessionRepository) LockStaffBalanceWrites(ctx context.Context, staffID int64) error {
	return r.workforce.LockStaffBalanceWrites(ctx, staffID)
}

func (r workSessionRepository) GetCurrentByStaffID(ctx context.Context, staffID int64) (*timetracking.WorkSession, error) {
	value, err := r.workforce.TodayOpenWorkSession(ctx, staffID)
	if err != nil {
		return nil, rowReadError("get current by staff ID", err, workforce.ErrWorkSessionNotFound)
	}
	return workSessionToLegacy(value), nil
}

func (r workSessionRepository) GetLatestOpenByStaffID(ctx context.Context, staffID int64) (*timetracking.WorkSession, error) {
	value, err := r.workforce.LatestOpenWorkSession(ctx, staffID)
	if err != nil {
		return nil, rowReadError("get latest open by staff ID", err, workforce.ErrWorkSessionNotFound)
	}
	return workSessionToLegacy(value), nil
}

func (r workSessionRepository) GetOpenByStaffAndDate(ctx context.Context, staffID int64, date calendar.Date) (*timetracking.WorkSession, error) {
	value, err := r.workforce.OpenWorkSessionOn(ctx, staffID, date.String())
	if err != nil {
		return nil, rowReadError("get current by staff ID", err, workforce.ErrWorkSessionNotFound)
	}
	return workSessionToLegacy(value), nil
}

func (r workSessionRepository) GetOpenByStaffAndDateForUpdate(ctx context.Context, staffID int64, date calendar.Date) (*timetracking.WorkSession, error) {
	value, err := r.workforce.LockOpenWorkSessionOn(ctx, staffID, date.String())
	if err != nil {
		return nil, rowReadError("get current by staff ID", err, workforce.ErrWorkSessionNotFound)
	}
	return workSessionToLegacy(value), nil
}

func (r workSessionRepository) LockOpenByIDForUpdate(ctx context.Context, id int64) (*timetracking.WorkSession, error) {
	value, err := r.workforce.LockOpenWorkSession(ctx, id)
	if err != nil {
		return nil, rowReadError("lock open session by ID", err, workforce.ErrWorkSessionNotFound)
	}
	return workSessionToLegacy(value), nil
}

func (r workSessionRepository) ListOverlappingByStaffID(ctx context.Context, staffID int64, from time.Time, to *time.Time) ([]*timetracking.WorkSession, error) {
	values, err := r.workforce.ListOverlappingWorkSessions(ctx, []int64{staffID}, from, to)
	if err != nil {
		return nil, rowReadError("list overlapping by staff ID", err, nil)
	}
	return workSessionsToLegacy(values), nil
}

func (r workSessionRepository) ListOverlappingByStaffIDs(ctx context.Context, staffIDs []int64, from time.Time, to *time.Time) (map[int64][]*timetracking.WorkSession, error) {
	result := make(map[int64][]*timetracking.WorkSession, len(staffIDs))
	if len(staffIDs) == 0 {
		return result, nil
	}
	values, err := r.workforce.ListOverlappingWorkSessions(ctx, staffIDs, from, to)
	if err != nil {
		return nil, rowReadError("list overlapping by staff IDs", err, nil)
	}
	for _, value := range values {
		result[value.StaffID] = append(result[value.StaffID], workSessionToLegacy(value))
	}
	return result, nil
}

func (r workSessionRepository) GetHistoryByStaffID(ctx context.Context, staffID int64, from, to calendar.Date) ([]*timetracking.WorkSession, error) {
	return r.list(ctx, "get history by staff ID", workforce.WorkSessionFilter{
		StaffID: staffID, DateFrom: from.String(), DateTo: to.String(),
		Order: workSessionOrder(workforce.WorkSessionOrderDate, workforce.WorkSessionOrderCheckInTime),
	})
}

func (r workSessionRepository) GetHistoryByStaffIDs(ctx context.Context, staffIDs []int64, from, to calendar.Date) (map[int64][]*timetracking.WorkSession, error) {
	result := make(map[int64][]*timetracking.WorkSession, len(staffIDs))
	if len(staffIDs) == 0 {
		return result, nil
	}
	sessions, err := r.list(ctx, "get history by staff IDs", workforce.WorkSessionFilter{
		StaffIDs: staffIDs, DateFrom: from.String(), DateTo: to.String(),
		Order: workSessionOrder(workforce.WorkSessionOrderStaffID, workforce.WorkSessionOrderDate, workforce.WorkSessionOrderCheckInTime),
	})
	if err != nil {
		return nil, err
	}
	for _, session := range sessions {
		result[session.StaffID] = append(result[session.StaffID], session)
	}
	return result, nil
}

func (r workSessionRepository) GetOpenSessions(ctx context.Context, beforeDate calendar.Date) ([]*timetracking.WorkSession, error) {
	open := true
	return r.list(ctx, "get open sessions", workforce.WorkSessionFilter{DateBefore: beforeDate.String(), Open: &open})
}

func (r workSessionRepository) GetTodayPresenceMap(ctx context.Context) (map[int64]string, error) {
	presence, err := r.workforce.WorkPresenceMap(ctx)
	if err != nil {
		return nil, rowReadError("get today presence map", err, nil)
	}
	return presence, nil
}

func (r workSessionRepository) CloseSession(ctx context.Context, id int64, checkOutTime time.Time, autoCheckedOut bool) (bool, error) {
	closed, err := r.workforce.CloseWorkSession(ctx, id, checkOutTime, autoCheckedOut)
	if err != nil {
		return false, workSessionWriteError("close session", err)
	}
	return closed, nil
}

func (r workSessionRepository) UpdateBreakMinutes(ctx context.Context, id int64, breakMinutes int) error {
	affected, err := r.workforce.SetWorkSessionBreakMinutes(ctx, id, breakMinutes)
	if err != nil {
		return workSessionWriteError("update columns WorkSession", err)
	}
	if affected != 1 {
		return &modelBase.DatabaseError{Op: "update break minutes", Err: fmt.Errorf("expected 1 rows affected, got %d", affected)}
	}
	return nil
}

func (r workSessionRepository) list(ctx context.Context, op string, filter workforce.WorkSessionFilter) ([]*timetracking.WorkSession, error) {
	values, err := r.workforce.ListWorkSessions(ctx, filter)
	if err != nil {
		return nil, rowReadError(op, err, nil)
	}
	return workSessionsToLegacy(values), nil
}

// --- work session breaks ---

type workSessionBreakRepository struct{ workforce workforce.Capability }

// newWorkSessionBreakRows serves timetracking.WorkSessionBreakRepository
// from the Workforce capability.
func newWorkSessionBreakRows(capability workforce.Capability) timetracking.WorkSessionBreakRepository {
	if capability == nil {
		panic("work session break repository adapter: Workforce capability is required")
	}
	return workSessionBreakRepository{workforce: capability}
}

func (r workSessionBreakRepository) Create(ctx context.Context, entity *timetracking.WorkSessionBreak) error {
	if entity == nil {
		return errors.New("WorkSessionBreak cannot be nil or zero value")
	}
	if err := entity.Validate(); err != nil {
		return err
	}
	created, err := r.workforce.CreateWorkSessionBreak(ctx, workSessionBreakToCapability(entity))
	if err != nil {
		return workSessionWriteError("create", err)
	}
	applyWorkSessionBreakToLegacy(entity, created)
	return nil
}

func (r workSessionBreakRepository) FindByID(ctx context.Context, id any) (*timetracking.WorkSessionBreak, error) {
	breakID, err := legacyID(id)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "find by id", Err: err}
	}
	value, err := r.workforce.FindWorkSessionBreak(ctx, breakID)
	if err != nil {
		return nil, rowReadError("find by id", err, workforce.ErrWorkSessionBreakNotFound)
	}
	return workSessionBreakToLegacy(value), nil
}

func (r workSessionBreakRepository) Update(ctx context.Context, entity *timetracking.WorkSessionBreak) error {
	if entity == nil {
		return errors.New("WorkSessionBreak cannot be nil or zero value")
	}
	if err := entity.Validate(); err != nil {
		return err
	}
	updated, err := r.workforce.UpdateWorkSessionBreak(ctx, workSessionBreakToCapability(entity))
	if err != nil {
		if errors.Is(err, workforce.ErrWorkSessionBreakNotFound) {
			return rowsAffectedError("update WorkSessionBreak")
		}
		return workSessionWriteError("update", err)
	}
	applyWorkSessionBreakToLegacy(entity, updated)
	return nil
}

func (r workSessionBreakRepository) Delete(ctx context.Context, id any) error {
	breakID, err := legacyID(id)
	if err != nil {
		return &modelBase.DatabaseError{Op: "delete", Err: err}
	}
	if err := r.workforce.DeleteWorkSessionBreak(ctx, breakID); err != nil {
		return workSessionWriteError("delete", err)
	}
	return nil
}

func (r workSessionBreakRepository) GetBySessionID(ctx context.Context, sessionID int64) ([]*timetracking.WorkSessionBreak, error) {
	values, err := r.workforce.ListWorkSessionBreaks(ctx, workforce.WorkSessionBreakFilter{SessionID: sessionID})
	if err != nil {
		return nil, rowReadError("get breaks by session ID", err, nil)
	}
	return workSessionBreaksToLegacy(values), nil
}

func (r workSessionBreakRepository) GetBySessionIDs(ctx context.Context, sessionIDs []int64) (map[int64][]*timetracking.WorkSessionBreak, error) {
	result := make(map[int64][]*timetracking.WorkSessionBreak, len(sessionIDs))
	if len(sessionIDs) == 0 {
		return result, nil
	}
	values, err := r.workforce.ListWorkSessionBreaks(ctx, workforce.WorkSessionBreakFilter{SessionIDs: sessionIDs})
	if err != nil {
		return nil, rowReadError("get breaks by session IDs", err, nil)
	}
	for _, value := range values {
		result[value.SessionID] = append(result[value.SessionID], workSessionBreakToLegacy(value))
	}
	return result, nil
}

func (r workSessionBreakRepository) GetActiveBySessionID(ctx context.Context, sessionID int64) (*timetracking.WorkSessionBreak, error) {
	active := true
	values, err := r.workforce.ListWorkSessionBreaks(ctx, workforce.WorkSessionBreakFilter{SessionID: sessionID, Active: &active, Limit: 1})
	if err != nil {
		return nil, rowReadError("get active break by session ID", err, nil)
	}
	if len(values) == 0 {
		return nil, nil
	}
	return workSessionBreakToLegacy(values[0]), nil
}

func (r workSessionBreakRepository) EndBreak(ctx context.Context, id int64, endedAt time.Time, durationMinutes int) error {
	affected, err := r.workforce.EndWorkSessionBreak(ctx, id, endedAt, durationMinutes)
	if err != nil {
		return workSessionWriteError("end break", err)
	}
	if affected != 1 {
		return &modelBase.DatabaseError{Op: "end break", Err: fmt.Errorf("expected 1 rows affected, got %d", affected)}
	}
	return nil
}

func (r workSessionBreakRepository) UpdateDuration(ctx context.Context, id int64, durationMinutes int, endedAt time.Time) error {
	if _, err := r.workforce.SetWorkSessionBreakDuration(ctx, id, durationMinutes, endedAt); err != nil {
		return workSessionWriteError("update columns WorkSessionBreak", err)
	}
	return nil
}

func (r workSessionBreakRepository) GetExpiredBreaks(ctx context.Context, before time.Time) ([]*timetracking.WorkSessionBreak, error) {
	values, err := r.workforce.ExpiredWorkSessionBreaks(ctx, before)
	if err != nil {
		return nil, rowReadError("get expired breaks", err, nil)
	}
	return workSessionBreaksToLegacy(values), nil
}

// --- staff balance adjustments ---

type staffBalanceAdjustmentRepository struct{ workforce workforce.Capability }

// newBalanceAdjustmentRows serves
// timetracking.StaffBalanceAdjustmentRepository from the Workforce capability.
func newBalanceAdjustmentRows(capability workforce.Capability) timetracking.StaffBalanceAdjustmentRepository {
	if capability == nil {
		panic("staff balance adjustment repository adapter: Workforce capability is required")
	}
	return staffBalanceAdjustmentRepository{workforce: capability}
}

func (r staffBalanceAdjustmentRepository) Create(ctx context.Context, entity *timetracking.StaffBalanceAdjustment) error {
	if entity == nil {
		return errors.New("StaffBalanceAdjustment cannot be nil or zero value")
	}
	if err := entity.Validate(); err != nil {
		return err
	}
	created, err := r.workforce.CreateStaffBalanceAdjustment(ctx, adjustmentToCapability(entity))
	if err != nil {
		return workSessionWriteError("create", err)
	}
	applyAdjustmentToLegacy(entity, created)
	return nil
}

func (r staffBalanceAdjustmentRepository) FindByID(ctx context.Context, id any) (*timetracking.StaffBalanceAdjustment, error) {
	adjustmentID, err := legacyID(id)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "find by id", Err: err}
	}
	value, err := r.workforce.FindStaffBalanceAdjustment(ctx, adjustmentID)
	if err != nil {
		return nil, rowReadError("find by id", err, workforce.ErrStaffBalanceAdjustmentNotFound)
	}
	return adjustmentToLegacy(value), nil
}

func (r staffBalanceAdjustmentRepository) Update(ctx context.Context, entity *timetracking.StaffBalanceAdjustment) error {
	if entity == nil {
		return errors.New("StaffBalanceAdjustment cannot be nil or zero value")
	}
	if err := entity.Validate(); err != nil {
		return err
	}
	updated, err := r.workforce.UpdateStaffBalanceAdjustment(ctx, adjustmentToCapability(entity))
	if err != nil {
		if errors.Is(err, workforce.ErrStaffBalanceAdjustmentNotFound) {
			return rowsAffectedError("update StaffBalanceAdjustment")
		}
		return workSessionWriteError("update", err)
	}
	applyAdjustmentToLegacy(entity, updated)
	return nil
}

func (r staffBalanceAdjustmentRepository) Delete(ctx context.Context, id any) error {
	adjustmentID, err := legacyID(id)
	if err != nil {
		return &modelBase.DatabaseError{Op: "delete", Err: err}
	}
	if err := r.workforce.DeleteStaffBalanceAdjustment(ctx, adjustmentID); err != nil {
		return workSessionWriteError("delete", err)
	}
	return nil
}

func (r staffBalanceAdjustmentRepository) ListWithFilter(ctx context.Context, filter workforce.StaffBalanceAdjustmentFilter) ([]*timetracking.StaffBalanceAdjustment, error) {
	return r.list(ctx, "list with options", filter)
}

func (r staffBalanceAdjustmentRepository) list(ctx context.Context, op string, filter workforce.StaffBalanceAdjustmentFilter) ([]*timetracking.StaffBalanceAdjustment, error) {
	values, err := r.workforce.ListStaffBalanceAdjustments(ctx, filter)
	if err != nil {
		return nil, rowReadError(op, err, nil)
	}
	return adjustmentsToLegacy(values), nil
}

func (r staffBalanceAdjustmentRepository) LockStaffBalanceWrites(ctx context.Context, staffID int64) error {
	return r.workforce.LockStaffBalanceWrites(ctx, staffID)
}

func (r staffBalanceAdjustmentRepository) GetByStaffAndDateRange(ctx context.Context, staffID int64, from, to calendar.Date) ([]*timetracking.StaffBalanceAdjustment, error) {
	return r.list(ctx, "get balance adjustments by staff+range", workforce.StaffBalanceAdjustmentFilter{
		StaffID: staffID, EffectiveFrom: from.String(), EffectiveTo: to.String(),
	})
}

func (r staffBalanceAdjustmentRepository) GetByStaffIDsAndDateRange(ctx context.Context, staffIDs []int64, from, to calendar.Date) (map[int64][]*timetracking.StaffBalanceAdjustment, error) {
	result := make(map[int64][]*timetracking.StaffBalanceAdjustment, len(staffIDs))
	if len(staffIDs) == 0 {
		return result, nil
	}
	values, err := r.workforce.ListStaffBalanceAdjustments(ctx, workforce.StaffBalanceAdjustmentFilter{
		StaffIDs: staffIDs, EffectiveFrom: from.String(), EffectiveTo: to.String(),
	})
	if err != nil {
		return nil, rowReadError("get balance adjustments by staff IDs+range", err, nil)
	}
	for _, value := range values {
		result[value.StaffID] = append(result[value.StaffID], adjustmentToLegacy(value))
	}
	return result, nil
}

// --- staff vacation openings ---

type staffVacationOpeningRepository struct{ workforce workforce.Capability }

// newVacationOpeningRows serves
// timetracking.StaffVacationOpeningRepository from the Workforce capability.
func newVacationOpeningRows(capability workforce.Capability) timetracking.StaffVacationOpeningRepository {
	if capability == nil {
		panic("staff vacation opening repository adapter: Workforce capability is required")
	}
	return staffVacationOpeningRepository{workforce: capability}
}

func (r staffVacationOpeningRepository) Create(ctx context.Context, entity *timetracking.StaffVacationOpening) error {
	if entity == nil {
		return errors.New("StaffVacationOpening cannot be nil or zero value")
	}
	if err := entity.Validate(); err != nil {
		return err
	}
	created, err := r.workforce.CreateStaffVacationOpening(ctx, openingToCapability(entity))
	if err != nil {
		return workSessionWriteError("create", err)
	}
	applyOpeningToLegacy(entity, created)
	return nil
}

func (r staffVacationOpeningRepository) FindByID(ctx context.Context, id any) (*timetracking.StaffVacationOpening, error) {
	openingID, err := legacyID(id)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "find by id", Err: err}
	}
	value, err := r.workforce.FindStaffVacationOpening(ctx, openingID)
	if err != nil {
		return nil, rowReadError("find by id", err, workforce.ErrStaffVacationOpeningNotFound)
	}
	return openingToLegacy(value), nil
}

func (r staffVacationOpeningRepository) Update(ctx context.Context, entity *timetracking.StaffVacationOpening) error {
	if entity == nil {
		return errors.New("StaffVacationOpening cannot be nil or zero value")
	}
	if err := entity.Validate(); err != nil {
		return err
	}
	updated, err := r.workforce.UpdateStaffVacationOpening(ctx, openingToCapability(entity))
	if err != nil {
		if errors.Is(err, workforce.ErrStaffVacationOpeningNotFound) {
			return rowsAffectedError("update StaffVacationOpening")
		}
		return workSessionWriteError("update", err)
	}
	applyOpeningToLegacy(entity, updated)
	return nil
}

func (r staffVacationOpeningRepository) Delete(ctx context.Context, id any) error {
	openingID, err := legacyID(id)
	if err != nil {
		return &modelBase.DatabaseError{Op: "delete", Err: err}
	}
	if err := r.workforce.DeleteStaffVacationOpening(ctx, openingID); err != nil {
		return workSessionWriteError("delete", err)
	}
	return nil
}

func (r staffVacationOpeningRepository) GetByStaffAndYear(ctx context.Context, staffID int64, year int) (*timetracking.StaffVacationOpening, error) {
	values, err := r.workforce.ListStaffVacationOpenings(ctx, workforce.StaffVacationFilter{StaffID: staffID, Year: year, Limit: 1})
	if err != nil {
		return nil, rowReadError("get vacation opening by staff+year", err, nil)
	}
	if len(values) == 0 {
		return nil, nil
	}
	return openingToLegacy(values[0]), nil
}

func (r staffVacationOpeningRepository) GetByStaffIDsAndYear(ctx context.Context, staffIDs []int64, year int) (map[int64]*timetracking.StaffVacationOpening, error) {
	result := make(map[int64]*timetracking.StaffVacationOpening, len(staffIDs))
	if len(staffIDs) == 0 {
		return result, nil
	}
	values, err := r.workforce.ListStaffVacationOpenings(ctx, workforce.StaffVacationFilter{StaffIDs: staffIDs, Year: year})
	if err != nil {
		return nil, rowReadError("get vacation openings by staff IDs+year", err, nil)
	}
	for _, value := range values {
		result[value.StaffID] = openingToLegacy(value)
	}
	return result, nil
}

// --- staff vacation quota ---

type staffVacationQuotaRepository struct{ workforce workforce.Capability }

// newVacationQuotaRows serves
// timetracking.StaffVacationQuotaRepository from the Workforce capability.
func newVacationQuotaRows(capability workforce.Capability) timetracking.StaffVacationQuotaRepository {
	if capability == nil {
		panic("staff vacation quota repository adapter: Workforce capability is required")
	}
	return staffVacationQuotaRepository{workforce: capability}
}

func (r staffVacationQuotaRepository) Create(ctx context.Context, entity *timetracking.StaffVacationQuota) error {
	if entity == nil {
		return errors.New("StaffVacationQuota cannot be nil or zero value")
	}
	if err := entity.Validate(); err != nil {
		return err
	}
	created, err := r.workforce.CreateStaffVacationQuota(ctx, quotaToCapability(entity))
	if err != nil {
		return workSessionWriteError("create", err)
	}
	applyQuotaToLegacy(entity, created)
	return nil
}

func (r staffVacationQuotaRepository) FindByID(ctx context.Context, id any) (*timetracking.StaffVacationQuota, error) {
	quotaID, err := legacyID(id)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "find by id", Err: err}
	}
	value, err := r.workforce.FindStaffVacationQuota(ctx, quotaID)
	if err != nil {
		return nil, rowReadError("find by id", err, workforce.ErrStaffVacationQuotaNotFound)
	}
	return quotaToLegacy(value), nil
}

func (r staffVacationQuotaRepository) Update(ctx context.Context, entity *timetracking.StaffVacationQuota) error {
	if entity == nil {
		return errors.New("StaffVacationQuota cannot be nil or zero value")
	}
	if err := entity.Validate(); err != nil {
		return err
	}
	updated, err := r.workforce.UpdateStaffVacationQuota(ctx, quotaToCapability(entity))
	if err != nil {
		if errors.Is(err, workforce.ErrStaffVacationQuotaNotFound) {
			return rowsAffectedError("update StaffVacationQuota")
		}
		return workSessionWriteError("update", err)
	}
	applyQuotaToLegacy(entity, updated)
	return nil
}

func (r staffVacationQuotaRepository) Delete(ctx context.Context, id any) error {
	quotaID, err := legacyID(id)
	if err != nil {
		return &modelBase.DatabaseError{Op: "delete", Err: err}
	}
	if err := r.workforce.DeleteStaffVacationQuota(ctx, quotaID); err != nil {
		return workSessionWriteError("delete", err)
	}
	return nil
}

func (r staffVacationQuotaRepository) GetByStaffAndYear(ctx context.Context, staffID int64, year int) (*timetracking.StaffVacationQuota, error) {
	values, err := r.workforce.ListStaffVacationQuotas(ctx, workforce.StaffVacationFilter{StaffID: staffID, Year: year, Limit: 1})
	if err != nil {
		return nil, rowReadError("get vacation quota by staff+year", err, nil)
	}
	if len(values) == 0 {
		return nil, nil
	}
	return quotaToLegacy(values[0]), nil
}

func (r staffVacationQuotaRepository) GetByStaffIDsAndYear(ctx context.Context, staffIDs []int64, year int) (map[int64]*timetracking.StaffVacationQuota, error) {
	result := make(map[int64]*timetracking.StaffVacationQuota, len(staffIDs))
	if len(staffIDs) == 0 {
		return result, nil
	}
	values, err := r.workforce.ListStaffVacationQuotas(ctx, workforce.StaffVacationFilter{StaffIDs: staffIDs, Year: year})
	if err != nil {
		return nil, rowReadError("get vacation quotas by staff IDs+year", err, nil)
	}
	for _, value := range values {
		result[value.StaffID] = quotaToLegacy(value)
	}
	return result, nil
}

func (r staffVacationQuotaRepository) Upsert(ctx context.Context, entity *timetracking.StaffVacationQuota) error {
	if entity == nil {
		return fmt.Errorf("quota cannot be nil")
	}
	if err := entity.Validate(); err != nil {
		return err
	}
	if err := r.workforce.UpsertStaffVacationQuota(ctx, quotaToCapability(entity)); err != nil {
		return workSessionWriteError("upsert vacation quota", err)
	}
	return nil
}

// --- shared helpers ---

func workSessionOrder(fields ...workforce.WorkSessionOrderField) []workforce.WorkSessionOrder {
	order := make([]workforce.WorkSessionOrder, 0, len(fields))
	for _, field := range fields {
		order = append(order, workforce.WorkSessionOrder{Field: field})
	}
	return order
}

// workSessionWriteError keeps the legacy write error shape: validation
// failures surface bare, everything else is a DatabaseError whose chain still
// reaches the driver error of a rejected duplicate.
func workSessionWriteError(op string, err error) error {
	if errors.Is(err, workforce.ErrInvalidWorkSession) {
		return errors.New(err.Error())
	}
	return &modelBase.DatabaseError{Op: op, Err: err}
}

// --- mapping ---

func workSessionToLegacy(value workforce.WorkSession) *timetracking.WorkSession {
	entity := &timetracking.WorkSession{}
	applyWorkSessionToLegacy(entity, value)
	return entity
}

func applyWorkSessionToLegacy(entity *timetracking.WorkSession, value workforce.WorkSession) {
	entity.ID = value.ID
	entity.CreatedAt = value.CreatedAt
	entity.UpdatedAt = value.UpdatedAt
	entity.TenantID = value.TenantID
	entity.StaffID = value.StaffID
	entity.Date = calendar.Date(value.Date)
	entity.Status = value.Status
	entity.Source = value.Source
	entity.CheckInTime = value.CheckInTime
	entity.CheckOutTime = value.CheckOutTime
	entity.ReopenedAt = value.ReopenedAt
	entity.BreakMinutes = value.BreakMinutes
	entity.Notes = value.Notes
	entity.AutoCheckedOut = value.AutoCheckedOut
	entity.CreatedBy = value.CreatedBy
	entity.UpdatedBy = value.UpdatedBy
}

func workSessionsToLegacy(values []workforce.WorkSession) []*timetracking.WorkSession {
	// Empty, not nil: callers serialize the result straight to JSON.
	result := make([]*timetracking.WorkSession, 0, len(values))
	for _, value := range values {
		result = append(result, workSessionToLegacy(value))
	}
	return result
}

func workSessionBreakToLegacy(value workforce.WorkSessionBreak) *timetracking.WorkSessionBreak {
	entity := &timetracking.WorkSessionBreak{}
	applyWorkSessionBreakToLegacy(entity, value)
	return entity
}

func applyWorkSessionBreakToLegacy(entity *timetracking.WorkSessionBreak, value workforce.WorkSessionBreak) {
	entity.ID = value.ID
	entity.CreatedAt = value.CreatedAt
	entity.UpdatedAt = value.UpdatedAt
	entity.TenantID = value.TenantID
	entity.SessionID = value.SessionID
	entity.StartedAt = value.StartedAt
	entity.EndedAt = value.EndedAt
	entity.DurationMinutes = value.DurationMinutes
	entity.PlannedEndTime = value.PlannedEndTime
}

func workSessionBreaksToLegacy(values []workforce.WorkSessionBreak) []*timetracking.WorkSessionBreak {
	result := make([]*timetracking.WorkSessionBreak, 0, len(values))
	for _, value := range values {
		result = append(result, workSessionBreakToLegacy(value))
	}
	return result
}

func adjustmentToCapability(entity *timetracking.StaffBalanceAdjustment) workforce.StaffBalanceAdjustment {
	return workforce.StaffBalanceAdjustment{
		ID: entity.ID, TenantID: entity.TenantID, StaffID: entity.StaffID, Type: entity.Type, MinutesDelta: entity.MinutesDelta,
		EffectiveDate: entity.EffectiveDate.String(), Note: entity.Note, DecidedBy: entity.DecidedBy, DecidedAt: entity.DecidedAt,
		CreatedAt: entity.CreatedAt, UpdatedAt: entity.UpdatedAt,
	}
}

func adjustmentToLegacy(value workforce.StaffBalanceAdjustment) *timetracking.StaffBalanceAdjustment {
	entity := &timetracking.StaffBalanceAdjustment{}
	applyAdjustmentToLegacy(entity, value)
	return entity
}

func applyAdjustmentToLegacy(entity *timetracking.StaffBalanceAdjustment, value workforce.StaffBalanceAdjustment) {
	entity.ID = value.ID
	entity.CreatedAt = value.CreatedAt
	entity.UpdatedAt = value.UpdatedAt
	entity.TenantID = value.TenantID
	entity.StaffID = value.StaffID
	entity.Type = value.Type
	entity.MinutesDelta = value.MinutesDelta
	entity.EffectiveDate = calendar.Date(value.EffectiveDate)
	entity.Note = value.Note
	entity.DecidedBy = value.DecidedBy
	entity.DecidedAt = value.DecidedAt
}

func adjustmentsToLegacy(values []workforce.StaffBalanceAdjustment) []*timetracking.StaffBalanceAdjustment {
	result := make([]*timetracking.StaffBalanceAdjustment, 0, len(values))
	for _, value := range values {
		result = append(result, adjustmentToLegacy(value))
	}
	return result
}

func openingToCapability(entity *timetracking.StaffVacationOpening) workforce.StaffVacationOpening {
	return workforce.StaffVacationOpening{
		ID: entity.ID, TenantID: entity.TenantID, StaffID: entity.StaffID, Year: entity.Year, EffectiveDate: entity.EffectiveDate.String(),
		TakenBeforeDays: entity.TakenBeforeDays, EnteredRemainingDays: entity.EnteredRemainingDays, Note: entity.Note,
		DecidedBy: entity.DecidedBy, DecidedAt: entity.DecidedAt, CreatedAt: entity.CreatedAt, UpdatedAt: entity.UpdatedAt,
	}
}

func openingToLegacy(value workforce.StaffVacationOpening) *timetracking.StaffVacationOpening {
	entity := &timetracking.StaffVacationOpening{}
	applyOpeningToLegacy(entity, value)
	return entity
}

func applyOpeningToLegacy(entity *timetracking.StaffVacationOpening, value workforce.StaffVacationOpening) {
	entity.ID = value.ID
	entity.CreatedAt = value.CreatedAt
	entity.UpdatedAt = value.UpdatedAt
	entity.TenantID = value.TenantID
	entity.StaffID = value.StaffID
	entity.Year = value.Year
	entity.EffectiveDate = calendar.Date(value.EffectiveDate)
	entity.TakenBeforeDays = value.TakenBeforeDays
	entity.EnteredRemainingDays = value.EnteredRemainingDays
	entity.Note = value.Note
	entity.DecidedBy = value.DecidedBy
	entity.DecidedAt = value.DecidedAt
}

func quotaToCapability(entity *timetracking.StaffVacationQuota) workforce.StaffVacationQuota {
	return workforce.StaffVacationQuota{
		ID: entity.ID, TenantID: entity.TenantID, StaffID: entity.StaffID, Year: entity.Year,
		EntitledDays: entity.EntitledDays, CarryoverDays: entity.CarryoverDays, CreatedAt: entity.CreatedAt, UpdatedAt: entity.UpdatedAt,
		ChangeReason: entity.ChangeReason, ChangedBy: entity.ChangedBy,
	}
}

func quotaToLegacy(value workforce.StaffVacationQuota) *timetracking.StaffVacationQuota {
	entity := &timetracking.StaffVacationQuota{}
	applyQuotaToLegacy(entity, value)
	return entity
}

func applyQuotaToLegacy(entity *timetracking.StaffVacationQuota, value workforce.StaffVacationQuota) {
	entity.ID = value.ID
	entity.CreatedAt = value.CreatedAt
	entity.UpdatedAt = value.UpdatedAt
	entity.TenantID = value.TenantID
	entity.StaffID = value.StaffID
	entity.Year = value.Year
	entity.EntitledDays = value.EntitledDays
	entity.CarryoverDays = value.CarryoverDays
}
