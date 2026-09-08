package legacy

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

// The adapters in this file keep the retained models/active work-session
// contracts alive on top of the Workforce capability while their consumers
// migrate (#2690): work sessions, breaks, balance adjustments, vacation
// openings and quotas. They perform no persistence of their own.

// --- work sessions ---

type workSessionRepository struct{ workforce workforce.Capability }

// NewWorkSessionRepository serves activeModels.WorkSessionRepository from
// the Workforce capability.
func NewWorkSessionRepository(capability workforce.Capability) activeModels.WorkSessionRepository {
	if capability == nil {
		panic("work session repository adapter: Workforce capability is required")
	}
	return workSessionRepository{workforce: capability}
}

func (r workSessionRepository) Create(ctx context.Context, entity *activeModels.WorkSession) error {
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

func (r workSessionRepository) FindByID(ctx context.Context, id any) (*activeModels.WorkSession, error) {
	sessionID, err := legacyID(id)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "find by id", Err: err}
	}
	value, err := r.workforce.FindWorkSession(ctx, sessionID)
	if err != nil {
		return nil, workSessionReadError("find by id", err, workforce.ErrWorkSessionNotFound)
	}
	return workSessionToLegacy(value), nil
}

func (r workSessionRepository) Update(ctx context.Context, entity *activeModels.WorkSession) error {
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

func (r workSessionRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*activeModels.WorkSession, error) {
	filter, err := workSessionFilterFromOptions(options)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "list with options", Err: err}
	}
	return r.list(ctx, "list with options", filter)
}

func (r workSessionRepository) CountWithOptions(ctx context.Context, options *modelBase.QueryOptions) (int, error) {
	filter, err := workSessionFilterFromOptions(options)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "count with options", Err: err}
	}
	count, err := r.workforce.CountWorkSessions(ctx, filter)
	if err != nil {
		return 0, workSessionReadError("count with options", err, nil)
	}
	return count, nil
}

func (r workSessionRepository) OldestBefore(ctx context.Context, column string, cutoff *timezone.Date) (*timezone.Date, error) {
	before := ""
	if cutoff != nil {
		before = cutoff.String()
	}
	oldest, err := r.workforce.OldestWorkSessionDate(ctx, column, before)
	if err != nil {
		return nil, workSessionReadError("oldest before", err, nil)
	}
	if oldest == "" {
		return nil, nil
	}
	date, err := timezone.ParseDate(oldest)
	if err != nil {
		return nil, err
	}
	return &date, nil
}

func (r workSessionRepository) DeleteOlderThan(ctx context.Context, column string, cutoff timezone.Date) (int64, error) {
	deleted, err := r.workforce.DeleteWorkSessionsOlderThan(ctx, column, cutoff.String())
	if err != nil {
		return 0, workSessionWriteError("delete older than", err)
	}
	return deleted, nil
}

func (r workSessionRepository) LockStaffBalanceWrites(ctx context.Context, staffID int64) error {
	return r.workforce.LockStaffBalanceWrites(ctx, staffID)
}

func (r workSessionRepository) GetCurrentByStaffID(ctx context.Context, staffID int64) (*activeModels.WorkSession, error) {
	value, err := r.workforce.TodayOpenWorkSession(ctx, staffID)
	if err != nil {
		return nil, workSessionReadError("get current by staff ID", err, workforce.ErrWorkSessionNotFound)
	}
	return workSessionToLegacy(value), nil
}

func (r workSessionRepository) GetLatestOpenByStaffID(ctx context.Context, staffID int64) (*activeModels.WorkSession, error) {
	value, err := r.workforce.LatestOpenWorkSession(ctx, staffID)
	if err != nil {
		return nil, workSessionReadError("get latest open by staff ID", err, workforce.ErrWorkSessionNotFound)
	}
	return workSessionToLegacy(value), nil
}

func (r workSessionRepository) GetOpenByStaffAndDate(ctx context.Context, staffID int64, date timezone.Date) (*activeModels.WorkSession, error) {
	value, err := r.workforce.OpenWorkSessionOn(ctx, staffID, date.String())
	if err != nil {
		return nil, workSessionReadError("get current by staff ID", err, workforce.ErrWorkSessionNotFound)
	}
	return workSessionToLegacy(value), nil
}

func (r workSessionRepository) GetOpenByStaffAndDateForUpdate(ctx context.Context, staffID int64, date timezone.Date) (*activeModels.WorkSession, error) {
	value, err := r.workforce.LockOpenWorkSessionOn(ctx, staffID, date.String())
	if err != nil {
		return nil, workSessionReadError("get current by staff ID", err, workforce.ErrWorkSessionNotFound)
	}
	return workSessionToLegacy(value), nil
}

func (r workSessionRepository) LockOpenByIDForUpdate(ctx context.Context, id int64) (*activeModels.WorkSession, error) {
	value, err := r.workforce.LockOpenWorkSession(ctx, id)
	if err != nil {
		return nil, workSessionReadError("lock open session by ID", err, workforce.ErrWorkSessionNotFound)
	}
	return workSessionToLegacy(value), nil
}

func (r workSessionRepository) ListOverlappingByStaffID(ctx context.Context, staffID int64, from time.Time, to *time.Time) ([]*activeModels.WorkSession, error) {
	values, err := r.workforce.ListOverlappingWorkSessions(ctx, []int64{staffID}, from, to)
	if err != nil {
		return nil, workSessionReadError("list overlapping by staff ID", err, nil)
	}
	return workSessionsToLegacy(values), nil
}

func (r workSessionRepository) ListOverlappingByStaffIDs(ctx context.Context, staffIDs []int64, from time.Time, to *time.Time) (map[int64][]*activeModels.WorkSession, error) {
	result := make(map[int64][]*activeModels.WorkSession, len(staffIDs))
	if len(staffIDs) == 0 {
		return result, nil
	}
	values, err := r.workforce.ListOverlappingWorkSessions(ctx, staffIDs, from, to)
	if err != nil {
		return nil, workSessionReadError("list overlapping by staff IDs", err, nil)
	}
	for _, value := range values {
		result[value.StaffID] = append(result[value.StaffID], workSessionToLegacy(value))
	}
	return result, nil
}

func (r workSessionRepository) GetHistoryByStaffID(ctx context.Context, staffID int64, from, to timezone.Date) ([]*activeModels.WorkSession, error) {
	return r.list(ctx, "get history by staff ID", workforce.WorkSessionFilter{
		StaffID: staffID, DateFrom: from.String(), DateTo: to.String(),
		Order: workSessionOrder(workforce.WorkSessionOrderDate, workforce.WorkSessionOrderCheckInTime),
	})
}

func (r workSessionRepository) GetHistoryByStaffIDs(ctx context.Context, staffIDs []int64, from, to timezone.Date) (map[int64][]*activeModels.WorkSession, error) {
	result := make(map[int64][]*activeModels.WorkSession, len(staffIDs))
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

func (r workSessionRepository) GetOpenSessions(ctx context.Context, beforeDate timezone.Date) ([]*activeModels.WorkSession, error) {
	open := true
	return r.list(ctx, "get open sessions", workforce.WorkSessionFilter{DateBefore: beforeDate.String(), Open: &open})
}

func (r workSessionRepository) GetTodayPresenceMap(ctx context.Context) (map[int64]string, error) {
	presence, err := r.workforce.WorkPresenceMap(ctx)
	if err != nil {
		return nil, workSessionReadError("get today presence map", err, nil)
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

func (r workSessionRepository) list(ctx context.Context, op string, filter workforce.WorkSessionFilter) ([]*activeModels.WorkSession, error) {
	values, err := r.workforce.ListWorkSessions(ctx, filter)
	if err != nil {
		return nil, workSessionReadError(op, err, nil)
	}
	return workSessionsToLegacy(values), nil
}

// workSessionFilterFromOptions translates the generic query options the
// retained callers still build into the typed capability filter. Only the
// columns and operators those callers use are accepted.
func workSessionFilterFromOptions(options *modelBase.QueryOptions) (workforce.WorkSessionFilter, error) {
	filter := workforce.WorkSessionFilter{}
	if options == nil {
		return filter, nil
	}
	if options.Filter != nil {
		if len(options.Filter.OrFilters()) > 0 || len(options.Filter.AndFilters()) > 0 {
			return filter, errors.New("nested work session filters are not supported")
		}
		for _, condition := range options.Filter.Conditions() {
			switch {
			case condition.Field == "staff_id" && condition.Operator == modelBase.OpEqual:
				id, ok := conditionInt64(condition.Value)
				if !ok {
					return filter, errors.New("staff_id filter must be an integer")
				}
				filter.StaffID = id
			case condition.Field == "staff_id" && condition.Operator == modelBase.OpIn:
				ids, ok := conditionInt64List(condition.Value)
				if !ok {
					return filter, errors.New("staff_id filter must be a list of integers")
				}
				filter.StaffIDs = ids
			case condition.Field == "id" && condition.Operator == modelBase.OpIn:
				ids, ok := conditionInt64List(condition.Value)
				if !ok {
					return filter, errors.New("id filter must be a list of integers")
				}
				filter.IDs = ids
			case condition.Field == "date" && condition.Operator == modelBase.OpEqual:
				date, ok := conditionDate(condition.Value)
				if !ok {
					return filter, errors.New("date filter must be a calendar date")
				}
				filter.Date = date
			case condition.Field == "date" && condition.Operator == modelBase.OpLessThan:
				date, ok := conditionDate(condition.Value)
				if !ok {
					return filter, errors.New("date filter must be a calendar date")
				}
				filter.DateBefore = date
			default:
				return filter, fmt.Errorf("unsupported work session filter %s %s", condition.Field, condition.Operator)
			}
		}
	}
	if options.Sorting != nil {
		for _, field := range options.Sorting.Fields {
			switch field.Field {
			case "id", "staff_id", "date", "check_in_time":
				filter.Order = append(filter.Order, workforce.WorkSessionOrder{
					Field: workforce.WorkSessionOrderField(field.Field), Descending: field.Direction == modelBase.SortDesc,
				})
			default:
				return filter, fmt.Errorf("unsupported work session sort field %q", field.Field)
			}
		}
	}
	filter.Limit, filter.Offset = paginationBounds(options)
	return filter, nil
}

// --- work session breaks ---

type workSessionBreakRepository struct{ workforce workforce.Capability }

// NewWorkSessionBreakRepository serves activeModels.WorkSessionBreakRepository
// from the Workforce capability.
func NewWorkSessionBreakRepository(capability workforce.Capability) activeModels.WorkSessionBreakRepository {
	if capability == nil {
		panic("work session break repository adapter: Workforce capability is required")
	}
	return workSessionBreakRepository{workforce: capability}
}

func (r workSessionBreakRepository) Create(ctx context.Context, entity *activeModels.WorkSessionBreak) error {
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

func (r workSessionBreakRepository) FindByID(ctx context.Context, id any) (*activeModels.WorkSessionBreak, error) {
	breakID, err := legacyID(id)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "find by id", Err: err}
	}
	value, err := r.workforce.FindWorkSessionBreak(ctx, breakID)
	if err != nil {
		return nil, workSessionReadError("find by id", err, workforce.ErrWorkSessionBreakNotFound)
	}
	return workSessionBreakToLegacy(value), nil
}

func (r workSessionBreakRepository) Update(ctx context.Context, entity *activeModels.WorkSessionBreak) error {
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

func (r workSessionBreakRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*activeModels.WorkSessionBreak, error) {
	filter := workforce.WorkSessionBreakFilter{}
	if options != nil {
		if options.Filter != nil {
			if len(options.Filter.OrFilters()) > 0 || len(options.Filter.AndFilters()) > 0 {
				return nil, &modelBase.DatabaseError{Op: "list with options", Err: errors.New("nested work session break filters are not supported")}
			}
			for _, condition := range options.Filter.Conditions() {
				switch {
				case condition.Field == "session_id" && condition.Operator == modelBase.OpEqual:
					id, ok := conditionInt64(condition.Value)
					if !ok {
						return nil, &modelBase.DatabaseError{Op: "list with options", Err: errors.New("session_id filter must be an integer")}
					}
					filter.SessionID = id
				case condition.Field == "session_id" && condition.Operator == modelBase.OpIn:
					ids, ok := conditionInt64List(condition.Value)
					if !ok {
						return nil, &modelBase.DatabaseError{Op: "list with options", Err: errors.New("session_id filter must be a list of integers")}
					}
					filter.SessionIDs = ids
				default:
					return nil, &modelBase.DatabaseError{Op: "list with options", Err: fmt.Errorf("unsupported work session break filter %s %s", condition.Field, condition.Operator)}
				}
			}
		}
		filter.Limit, filter.Offset = paginationBounds(options)
	}
	values, err := r.workforce.ListWorkSessionBreaks(ctx, filter)
	if err != nil {
		return nil, workSessionReadError("list with options", err, nil)
	}
	return workSessionBreaksToLegacy(values), nil
}

func (r workSessionBreakRepository) GetBySessionID(ctx context.Context, sessionID int64) ([]*activeModels.WorkSessionBreak, error) {
	values, err := r.workforce.ListWorkSessionBreaks(ctx, workforce.WorkSessionBreakFilter{SessionID: sessionID})
	if err != nil {
		return nil, workSessionReadError("get breaks by session ID", err, nil)
	}
	return workSessionBreaksToLegacy(values), nil
}

func (r workSessionBreakRepository) GetBySessionIDs(ctx context.Context, sessionIDs []int64) (map[int64][]*activeModels.WorkSessionBreak, error) {
	result := make(map[int64][]*activeModels.WorkSessionBreak, len(sessionIDs))
	if len(sessionIDs) == 0 {
		return result, nil
	}
	values, err := r.workforce.ListWorkSessionBreaks(ctx, workforce.WorkSessionBreakFilter{SessionIDs: sessionIDs})
	if err != nil {
		return nil, workSessionReadError("get breaks by session IDs", err, nil)
	}
	for _, value := range values {
		result[value.SessionID] = append(result[value.SessionID], workSessionBreakToLegacy(value))
	}
	return result, nil
}

func (r workSessionBreakRepository) GetActiveBySessionID(ctx context.Context, sessionID int64) (*activeModels.WorkSessionBreak, error) {
	active := true
	values, err := r.workforce.ListWorkSessionBreaks(ctx, workforce.WorkSessionBreakFilter{SessionID: sessionID, Active: &active, Limit: 1})
	if err != nil {
		return nil, workSessionReadError("get active break by session ID", err, nil)
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

func (r workSessionBreakRepository) GetExpiredBreaks(ctx context.Context, before time.Time) ([]*activeModels.WorkSessionBreak, error) {
	values, err := r.workforce.ExpiredWorkSessionBreaks(ctx, before)
	if err != nil {
		return nil, workSessionReadError("get expired breaks", err, nil)
	}
	return workSessionBreaksToLegacy(values), nil
}

// --- staff balance adjustments ---

type staffBalanceAdjustmentRepository struct{ workforce workforce.Capability }

// NewStaffBalanceAdjustmentRepository serves
// activeModels.StaffBalanceAdjustmentRepository from the Workforce capability.
func NewStaffBalanceAdjustmentRepository(capability workforce.Capability) activeModels.StaffBalanceAdjustmentRepository {
	if capability == nil {
		panic("staff balance adjustment repository adapter: Workforce capability is required")
	}
	return staffBalanceAdjustmentRepository{workforce: capability}
}

func (r staffBalanceAdjustmentRepository) Create(ctx context.Context, entity *activeModels.StaffBalanceAdjustment) error {
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

func (r staffBalanceAdjustmentRepository) FindByID(ctx context.Context, id any) (*activeModels.StaffBalanceAdjustment, error) {
	adjustmentID, err := legacyID(id)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "find by id", Err: err}
	}
	value, err := r.workforce.FindStaffBalanceAdjustment(ctx, adjustmentID)
	if err != nil {
		return nil, workSessionReadError("find by id", err, workforce.ErrStaffBalanceAdjustmentNotFound)
	}
	return adjustmentToLegacy(value), nil
}

func (r staffBalanceAdjustmentRepository) Update(ctx context.Context, entity *activeModels.StaffBalanceAdjustment) error {
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

func (r staffBalanceAdjustmentRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*activeModels.StaffBalanceAdjustment, error) {
	filter := workforce.StaffBalanceAdjustmentFilter{}
	if options != nil {
		if options.Filter != nil {
			if len(options.Filter.OrFilters()) > 0 || len(options.Filter.AndFilters()) > 0 {
				return nil, &modelBase.DatabaseError{Op: "list with options", Err: errors.New("nested balance adjustment filters are not supported")}
			}
			for _, condition := range options.Filter.Conditions() {
				switch {
				case condition.Field == "staff_id" && condition.Operator == modelBase.OpEqual:
					id, ok := conditionInt64(condition.Value)
					if !ok {
						return nil, &modelBase.DatabaseError{Op: "list with options", Err: errors.New("staff_id filter must be an integer")}
					}
					filter.StaffID = id
				case condition.Field == "type" && condition.Operator == modelBase.OpEqual:
					kind, ok := condition.Value.(string)
					if !ok {
						return nil, &modelBase.DatabaseError{Op: "list with options", Err: errors.New("type filter must be a string")}
					}
					filter.Types = []string{kind}
				case condition.Field == "type" && condition.Operator == modelBase.OpIn:
					kinds, ok := conditionStringList(condition.Value)
					if !ok {
						return nil, &modelBase.DatabaseError{Op: "list with options", Err: errors.New("type filter must be a list of strings")}
					}
					filter.Types = kinds
				case condition.Field == "effective_date" && condition.Operator == modelBase.OpGreaterThanOrEqual:
					date, ok := conditionDate(condition.Value)
					if !ok {
						return nil, &modelBase.DatabaseError{Op: "list with options", Err: errors.New("effective_date filter must be a calendar date")}
					}
					filter.EffectiveFrom = date
				case condition.Field == "effective_date" && condition.Operator == modelBase.OpLessThanOrEqual:
					date, ok := conditionDate(condition.Value)
					if !ok {
						return nil, &modelBase.DatabaseError{Op: "list with options", Err: errors.New("effective_date filter must be a calendar date")}
					}
					filter.EffectiveTo = date
				default:
					return nil, &modelBase.DatabaseError{Op: "list with options", Err: fmt.Errorf("unsupported balance adjustment filter %s %s", condition.Field, condition.Operator)}
				}
			}
		}
		// The adapter orders by effective_date then id, which is the order
		// every retained caller asks for; a different sort is not supported.
		if options.Sorting != nil {
			for _, field := range options.Sorting.Fields {
				if (field.Field != "effective_date" && field.Field != "id") || field.Direction == modelBase.SortDesc {
					return nil, &modelBase.DatabaseError{Op: "list with options", Err: fmt.Errorf("unsupported balance adjustment sort %q", field.Field)}
				}
			}
		}
		filter.Limit, filter.Offset = paginationBounds(options)
	}
	values, err := r.workforce.ListStaffBalanceAdjustments(ctx, filter)
	if err != nil {
		return nil, workSessionReadError("list with options", err, nil)
	}
	return adjustmentsToLegacy(values), nil
}

func (r staffBalanceAdjustmentRepository) LockStaffBalanceWrites(ctx context.Context, staffID int64) error {
	return r.workforce.LockStaffBalanceWrites(ctx, staffID)
}

func (r staffBalanceAdjustmentRepository) GetByStaffAndDateRange(ctx context.Context, staffID int64, from, to timezone.Date) ([]*activeModels.StaffBalanceAdjustment, error) {
	values, err := r.workforce.ListStaffBalanceAdjustments(ctx, workforce.StaffBalanceAdjustmentFilter{
		StaffID: staffID, EffectiveFrom: from.String(), EffectiveTo: to.String(),
	})
	if err != nil {
		return nil, workSessionReadError("get balance adjustments by staff+range", err, nil)
	}
	return adjustmentsToLegacy(values), nil
}

func (r staffBalanceAdjustmentRepository) GetByStaffIDsAndDateRange(ctx context.Context, staffIDs []int64, from, to timezone.Date) (map[int64][]*activeModels.StaffBalanceAdjustment, error) {
	result := make(map[int64][]*activeModels.StaffBalanceAdjustment, len(staffIDs))
	if len(staffIDs) == 0 {
		return result, nil
	}
	values, err := r.workforce.ListStaffBalanceAdjustments(ctx, workforce.StaffBalanceAdjustmentFilter{
		StaffIDs: staffIDs, EffectiveFrom: from.String(), EffectiveTo: to.String(),
	})
	if err != nil {
		return nil, workSessionReadError("get balance adjustments by staff IDs+range", err, nil)
	}
	for _, value := range values {
		result[value.StaffID] = append(result[value.StaffID], adjustmentToLegacy(value))
	}
	return result, nil
}

// --- staff vacation openings ---

type staffVacationOpeningRepository struct{ workforce workforce.Capability }

// NewStaffVacationOpeningRepository serves
// activeModels.StaffVacationOpeningRepository from the Workforce capability.
func NewStaffVacationOpeningRepository(capability workforce.Capability) activeModels.StaffVacationOpeningRepository {
	if capability == nil {
		panic("staff vacation opening repository adapter: Workforce capability is required")
	}
	return staffVacationOpeningRepository{workforce: capability}
}

func (r staffVacationOpeningRepository) Create(ctx context.Context, entity *activeModels.StaffVacationOpening) error {
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

func (r staffVacationOpeningRepository) FindByID(ctx context.Context, id any) (*activeModels.StaffVacationOpening, error) {
	openingID, err := legacyID(id)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "find by id", Err: err}
	}
	value, err := r.workforce.FindStaffVacationOpening(ctx, openingID)
	if err != nil {
		return nil, workSessionReadError("find by id", err, workforce.ErrStaffVacationOpeningNotFound)
	}
	return openingToLegacy(value), nil
}

func (r staffVacationOpeningRepository) Update(ctx context.Context, entity *activeModels.StaffVacationOpening) error {
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

func (r staffVacationOpeningRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*activeModels.StaffVacationOpening, error) {
	filter, err := vacationFilterFromOptions(options)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "list with options", Err: err}
	}
	values, err := r.workforce.ListStaffVacationOpenings(ctx, filter)
	if err != nil {
		return nil, workSessionReadError("list with options", err, nil)
	}
	result := make([]*activeModels.StaffVacationOpening, 0, len(values))
	for _, value := range values {
		result = append(result, openingToLegacy(value))
	}
	return result, nil
}

func (r staffVacationOpeningRepository) GetByStaffAndYear(ctx context.Context, staffID int64, year int) (*activeModels.StaffVacationOpening, error) {
	values, err := r.workforce.ListStaffVacationOpenings(ctx, workforce.StaffVacationFilter{StaffID: staffID, Year: year, Limit: 1})
	if err != nil {
		return nil, workSessionReadError("get vacation opening by staff+year", err, nil)
	}
	if len(values) == 0 {
		return nil, nil
	}
	return openingToLegacy(values[0]), nil
}

func (r staffVacationOpeningRepository) GetByStaffIDsAndYear(ctx context.Context, staffIDs []int64, year int) (map[int64]*activeModels.StaffVacationOpening, error) {
	result := make(map[int64]*activeModels.StaffVacationOpening, len(staffIDs))
	if len(staffIDs) == 0 {
		return result, nil
	}
	values, err := r.workforce.ListStaffVacationOpenings(ctx, workforce.StaffVacationFilter{StaffIDs: staffIDs, Year: year})
	if err != nil {
		return nil, workSessionReadError("get vacation openings by staff IDs+year", err, nil)
	}
	for _, value := range values {
		result[value.StaffID] = openingToLegacy(value)
	}
	return result, nil
}

// --- staff vacation quota ---

type staffVacationQuotaRepository struct{ workforce workforce.Capability }

// NewStaffVacationQuotaRepository serves
// activeModels.StaffVacationQuotaRepository from the Workforce capability.
func NewStaffVacationQuotaRepository(capability workforce.Capability) activeModels.StaffVacationQuotaRepository {
	if capability == nil {
		panic("staff vacation quota repository adapter: Workforce capability is required")
	}
	return staffVacationQuotaRepository{workforce: capability}
}

func (r staffVacationQuotaRepository) Create(ctx context.Context, entity *activeModels.StaffVacationQuota) error {
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

func (r staffVacationQuotaRepository) FindByID(ctx context.Context, id any) (*activeModels.StaffVacationQuota, error) {
	quotaID, err := legacyID(id)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "find by id", Err: err}
	}
	value, err := r.workforce.FindStaffVacationQuota(ctx, quotaID)
	if err != nil {
		return nil, workSessionReadError("find by id", err, workforce.ErrStaffVacationQuotaNotFound)
	}
	return quotaToLegacy(value), nil
}

func (r staffVacationQuotaRepository) Update(ctx context.Context, entity *activeModels.StaffVacationQuota) error {
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

// List keeps the legacy contract of the quota repository: an empty match is a
// nil slice, and the failure carries the "list vacation quotas" operation.
func (r staffVacationQuotaRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*activeModels.StaffVacationQuota, error) {
	filter, err := vacationFilterFromOptions(options)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "list vacation quotas", Err: err}
	}
	values, err := r.workforce.ListStaffVacationQuotas(ctx, filter)
	if err != nil {
		return nil, workSessionReadError("list vacation quotas", err, nil)
	}
	if len(values) == 0 {
		return nil, nil
	}
	result := make([]*activeModels.StaffVacationQuota, 0, len(values))
	for _, value := range values {
		result = append(result, quotaToLegacy(value))
	}
	return result, nil
}

func (r staffVacationQuotaRepository) GetByStaffAndYear(ctx context.Context, staffID int64, year int) (*activeModels.StaffVacationQuota, error) {
	values, err := r.workforce.ListStaffVacationQuotas(ctx, workforce.StaffVacationFilter{StaffID: staffID, Year: year, Limit: 1})
	if err != nil {
		return nil, workSessionReadError("get vacation quota by staff+year", err, nil)
	}
	if len(values) == 0 {
		return nil, nil
	}
	return quotaToLegacy(values[0]), nil
}

func (r staffVacationQuotaRepository) GetByStaffIDsAndYear(ctx context.Context, staffIDs []int64, year int) (map[int64]*activeModels.StaffVacationQuota, error) {
	result := make(map[int64]*activeModels.StaffVacationQuota, len(staffIDs))
	if len(staffIDs) == 0 {
		return result, nil
	}
	values, err := r.workforce.ListStaffVacationQuotas(ctx, workforce.StaffVacationFilter{StaffIDs: staffIDs, Year: year})
	if err != nil {
		return nil, workSessionReadError("get vacation quotas by staff IDs+year", err, nil)
	}
	for _, value := range values {
		result[value.StaffID] = quotaToLegacy(value)
	}
	return result, nil
}

func (r staffVacationQuotaRepository) Upsert(ctx context.Context, entity *activeModels.StaffVacationQuota) error {
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

func vacationFilterFromOptions(options *modelBase.QueryOptions) (workforce.StaffVacationFilter, error) {
	filter := workforce.StaffVacationFilter{}
	if options == nil {
		return filter, nil
	}
	if options.Filter != nil {
		if len(options.Filter.OrFilters()) > 0 || len(options.Filter.AndFilters()) > 0 {
			return filter, errors.New("nested vacation filters are not supported")
		}
		for _, condition := range options.Filter.Conditions() {
			switch {
			case condition.Field == "staff_id" && condition.Operator == modelBase.OpEqual:
				id, ok := conditionInt64(condition.Value)
				if !ok {
					return filter, errors.New("staff_id filter must be an integer")
				}
				filter.StaffID = id
			case condition.Field == "staff_id" && condition.Operator == modelBase.OpIn:
				ids, ok := conditionInt64List(condition.Value)
				if !ok {
					return filter, errors.New("staff_id filter must be a list of integers")
				}
				filter.StaffIDs = ids
			case condition.Field == "year" && condition.Operator == modelBase.OpEqual:
				year, ok := conditionInt64(condition.Value)
				if !ok {
					return filter, errors.New("year filter must be an integer")
				}
				filter.Year = int(year)
			default:
				return filter, fmt.Errorf("unsupported vacation filter %s %s", condition.Field, condition.Operator)
			}
		}
	}
	if options.Sorting != nil {
		for _, field := range options.Sorting.Fields {
			switch field.Field {
			case "id", "staff_id", "year", "entitled_days":
				filter.Order = append(filter.Order, workforce.StaffVacationOrder{
					Field: workforce.StaffVacationOrderField(field.Field), Descending: field.Direction == modelBase.SortDesc,
				})
			default:
				return filter, fmt.Errorf("unsupported vacation sort field %q", field.Field)
			}
		}
	}
	filter.Limit, filter.Offset = paginationBounds(options)
	return filter, nil
}

// --- shared helpers ---

// paginationBounds translates a page/page-size pair into limit and offset.
func paginationBounds(options *modelBase.QueryOptions) (int, int) {
	if options == nil || options.Pagination == nil || options.Pagination.PageSize <= 0 {
		return 0, 0
	}
	offset := 0
	if options.Pagination.Page > 1 {
		offset = (options.Pagination.Page - 1) * options.Pagination.PageSize
	}
	return options.Pagination.PageSize, offset
}

func workSessionOrder(fields ...workforce.WorkSessionOrderField) []workforce.WorkSessionOrder {
	order := make([]workforce.WorkSessionOrder, 0, len(fields))
	for _, field := range fields {
		order = append(order, workforce.WorkSessionOrder{Field: field})
	}
	return order
}

// workSessionReadError restores the legacy read error shape: a missing row is
// the persistence-neutral not-found sentinel wrapped in a DatabaseError.
func workSessionReadError(op string, err error, notFound error) error {
	if notFound != nil && errors.Is(err, notFound) {
		return &modelBase.DatabaseError{Op: op, Err: errors.Join(modelBase.ErrNotFound, sql.ErrNoRows)}
	}
	return &modelBase.DatabaseError{Op: op, Err: err}
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

func workSessionToCapability(entity *activeModels.WorkSession) workforce.WorkSession {
	return workforce.WorkSession{
		ID: entity.ID, TenantID: entity.TenantID, StaffID: entity.StaffID, Date: entity.Date.String(), Status: entity.Status,
		Source: entity.Source, CheckInTime: entity.CheckInTime, CheckOutTime: entity.CheckOutTime, ReopenedAt: entity.ReopenedAt,
		BreakMinutes: entity.BreakMinutes, Notes: entity.Notes, AutoCheckedOut: entity.AutoCheckedOut, CreatedBy: entity.CreatedBy,
		UpdatedBy: entity.UpdatedBy, CreatedAt: entity.CreatedAt, UpdatedAt: entity.UpdatedAt,
	}
}

func workSessionToLegacy(value workforce.WorkSession) *activeModels.WorkSession {
	entity := &activeModels.WorkSession{}
	applyWorkSessionToLegacy(entity, value)
	return entity
}

func applyWorkSessionToLegacy(entity *activeModels.WorkSession, value workforce.WorkSession) {
	entity.ID = value.ID
	entity.CreatedAt = value.CreatedAt
	entity.UpdatedAt = value.UpdatedAt
	entity.TenantID = value.TenantID
	entity.StaffID = value.StaffID
	entity.Date = timezone.Date(value.Date)
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

func workSessionsToLegacy(values []workforce.WorkSession) []*activeModels.WorkSession {
	// Empty, not nil: callers serialize the result straight to JSON.
	result := make([]*activeModels.WorkSession, 0, len(values))
	for _, value := range values {
		result = append(result, workSessionToLegacy(value))
	}
	return result
}

func workSessionBreakToCapability(entity *activeModels.WorkSessionBreak) workforce.WorkSessionBreak {
	return workforce.WorkSessionBreak{
		ID: entity.ID, TenantID: entity.TenantID, SessionID: entity.SessionID, StartedAt: entity.StartedAt, EndedAt: entity.EndedAt,
		DurationMinutes: entity.DurationMinutes, PlannedEndTime: entity.PlannedEndTime, CreatedAt: entity.CreatedAt, UpdatedAt: entity.UpdatedAt,
	}
}

func workSessionBreakToLegacy(value workforce.WorkSessionBreak) *activeModels.WorkSessionBreak {
	entity := &activeModels.WorkSessionBreak{}
	applyWorkSessionBreakToLegacy(entity, value)
	return entity
}

func applyWorkSessionBreakToLegacy(entity *activeModels.WorkSessionBreak, value workforce.WorkSessionBreak) {
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

func workSessionBreaksToLegacy(values []workforce.WorkSessionBreak) []*activeModels.WorkSessionBreak {
	result := make([]*activeModels.WorkSessionBreak, 0, len(values))
	for _, value := range values {
		result = append(result, workSessionBreakToLegacy(value))
	}
	return result
}

func adjustmentToCapability(entity *activeModels.StaffBalanceAdjustment) workforce.StaffBalanceAdjustment {
	return workforce.StaffBalanceAdjustment{
		ID: entity.ID, TenantID: entity.TenantID, StaffID: entity.StaffID, Type: entity.Type, MinutesDelta: entity.MinutesDelta,
		EffectiveDate: entity.EffectiveDate.String(), Note: entity.Note, DecidedBy: entity.DecidedBy, DecidedAt: entity.DecidedAt,
		CreatedAt: entity.CreatedAt, UpdatedAt: entity.UpdatedAt,
	}
}

func adjustmentToLegacy(value workforce.StaffBalanceAdjustment) *activeModels.StaffBalanceAdjustment {
	entity := &activeModels.StaffBalanceAdjustment{}
	applyAdjustmentToLegacy(entity, value)
	return entity
}

func applyAdjustmentToLegacy(entity *activeModels.StaffBalanceAdjustment, value workforce.StaffBalanceAdjustment) {
	entity.ID = value.ID
	entity.CreatedAt = value.CreatedAt
	entity.UpdatedAt = value.UpdatedAt
	entity.TenantID = value.TenantID
	entity.StaffID = value.StaffID
	entity.Type = value.Type
	entity.MinutesDelta = value.MinutesDelta
	entity.EffectiveDate = timezone.Date(value.EffectiveDate)
	entity.Note = value.Note
	entity.DecidedBy = value.DecidedBy
	entity.DecidedAt = value.DecidedAt
}

func adjustmentsToLegacy(values []workforce.StaffBalanceAdjustment) []*activeModels.StaffBalanceAdjustment {
	result := make([]*activeModels.StaffBalanceAdjustment, 0, len(values))
	for _, value := range values {
		result = append(result, adjustmentToLegacy(value))
	}
	return result
}

func openingToCapability(entity *activeModels.StaffVacationOpening) workforce.StaffVacationOpening {
	return workforce.StaffVacationOpening{
		ID: entity.ID, TenantID: entity.TenantID, StaffID: entity.StaffID, Year: entity.Year, EffectiveDate: entity.EffectiveDate.String(),
		TakenBeforeDays: entity.TakenBeforeDays, EnteredRemainingDays: entity.EnteredRemainingDays, Note: entity.Note,
		DecidedBy: entity.DecidedBy, DecidedAt: entity.DecidedAt, CreatedAt: entity.CreatedAt, UpdatedAt: entity.UpdatedAt,
	}
}

func openingToLegacy(value workforce.StaffVacationOpening) *activeModels.StaffVacationOpening {
	entity := &activeModels.StaffVacationOpening{}
	applyOpeningToLegacy(entity, value)
	return entity
}

func applyOpeningToLegacy(entity *activeModels.StaffVacationOpening, value workforce.StaffVacationOpening) {
	entity.ID = value.ID
	entity.CreatedAt = value.CreatedAt
	entity.UpdatedAt = value.UpdatedAt
	entity.TenantID = value.TenantID
	entity.StaffID = value.StaffID
	entity.Year = value.Year
	entity.EffectiveDate = timezone.Date(value.EffectiveDate)
	entity.TakenBeforeDays = value.TakenBeforeDays
	entity.EnteredRemainingDays = value.EnteredRemainingDays
	entity.Note = value.Note
	entity.DecidedBy = value.DecidedBy
	entity.DecidedAt = value.DecidedAt
}

func quotaToCapability(entity *activeModels.StaffVacationQuota) workforce.StaffVacationQuota {
	return workforce.StaffVacationQuota{
		ID: entity.ID, TenantID: entity.TenantID, StaffID: entity.StaffID, Year: entity.Year,
		EntitledDays: entity.EntitledDays, CarryoverDays: entity.CarryoverDays, CreatedAt: entity.CreatedAt, UpdatedAt: entity.UpdatedAt,
	}
}

func quotaToLegacy(value workforce.StaffVacationQuota) *activeModels.StaffVacationQuota {
	entity := &activeModels.StaffVacationQuota{}
	applyQuotaToLegacy(entity, value)
	return entity
}

func applyQuotaToLegacy(entity *activeModels.StaffVacationQuota, value workforce.StaffVacationQuota) {
	entity.ID = value.ID
	entity.CreatedAt = value.CreatedAt
	entity.UpdatedAt = value.UpdatedAt
	entity.TenantID = value.TenantID
	entity.StaffID = value.StaffID
	entity.Year = value.Year
	entity.EntitledDays = value.EntitledDays
	entity.CarryoverDays = value.CarryoverDays
}
