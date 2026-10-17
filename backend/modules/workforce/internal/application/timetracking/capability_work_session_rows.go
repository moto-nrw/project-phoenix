package timetracking

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The row adapters in this file serve the work-session and break row
// contracts from the Workforce capability, like the compose shift rows do for
// the planning rows (#3413); capability_balance_rows.go serves the balance
// adjustments and vacation rows. They perform no persistence of their own.

// --- work sessions ---

type workSessionRepository struct {
	workforce workforce.Capability
	shapes    RowErrorShapes
}

// NewWorkSessionRows serves WorkSessionRepository from
// the Workforce capability.
func NewWorkSessionRows(capability workforce.Capability, shapes RowErrorShapes) WorkSessionRepository {
	if capability == nil {
		panic("work session repository adapter: Workforce capability is required")
	}
	return workSessionRepository{workforce: capability, shapes: shapes}
}

func (r workSessionRepository) Create(ctx context.Context, entity *WorkSession) error {
	if entity == nil {
		return errors.New("WorkSession cannot be nil or zero value")
	}
	if err := entity.Validate(); err != nil {
		return err
	}
	created, err := r.workforce.CreateWorkSession(ctx, WorkSessionToCapability(entity))
	if err != nil {
		return r.shapes.workSessionWrite("create", err)
	}
	applyWorkSessionToLegacy(entity, created)
	return nil
}

func (r workSessionRepository) FindByID(ctx context.Context, id any) (*WorkSession, error) {
	sessionID, err := legacyID(id)
	if err != nil {
		return nil, r.shapes.Database("find by id", err)
	}
	value, err := r.workforce.FindWorkSession(ctx, sessionID)
	if err != nil {
		return nil, r.shapes.read("find by id", err, workforce.ErrWorkSessionNotFound)
	}
	return workSessionToLegacy(value), nil
}

func (r workSessionRepository) Update(ctx context.Context, entity *WorkSession) error {
	if entity == nil {
		return errors.New("WorkSession cannot be nil or zero value")
	}
	if err := entity.Validate(); err != nil {
		return err
	}
	updated, err := r.workforce.UpdateWorkSession(ctx, WorkSessionToCapability(entity))
	if err != nil {
		if errors.Is(err, workforce.ErrWorkSessionNotFound) {
			return r.shapes.rowsAffected("update WorkSession")
		}
		return r.shapes.workSessionWrite("update", err)
	}
	applyWorkSessionToLegacy(entity, updated)
	return nil
}

func (r workSessionRepository) Delete(ctx context.Context, id any) error {
	sessionID, err := legacyID(id)
	if err != nil {
		return r.shapes.Database("delete", err)
	}
	if err := r.workforce.DeleteWorkSession(ctx, sessionID); err != nil {
		return r.shapes.workSessionWrite("delete", err)
	}
	return nil
}

func (r workSessionRepository) ListWithFilter(ctx context.Context, filter workforce.WorkSessionFilter) ([]*WorkSession, error) {
	return r.list(ctx, "list with options", filter)
}

func (r workSessionRepository) CountWithFilter(ctx context.Context, filter workforce.WorkSessionFilter) (int, error) {
	count, err := r.workforce.CountWorkSessions(ctx, filter)
	if err != nil {
		return 0, r.shapes.read("count with options", err, nil)
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
		return nil, r.shapes.read("oldest before", err, nil)
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
		return 0, r.shapes.workSessionWrite("delete older than", err)
	}
	return deleted, nil
}

func (r workSessionRepository) LockStaffBalanceWrites(ctx context.Context, staffID int64) error {
	return r.workforce.LockStaffBalanceWrites(ctx, staffID)
}

func (r workSessionRepository) GetCurrentByStaffID(ctx context.Context, staffID int64) (*WorkSession, error) {
	value, err := r.workforce.TodayOpenWorkSession(ctx, staffID)
	if err != nil {
		return nil, r.shapes.read("get current by staff ID", err, workforce.ErrWorkSessionNotFound)
	}
	return workSessionToLegacy(value), nil
}

func (r workSessionRepository) GetLatestOpenByStaffID(ctx context.Context, staffID int64) (*WorkSession, error) {
	value, err := r.workforce.LatestOpenWorkSession(ctx, staffID)
	if err != nil {
		return nil, r.shapes.read("get latest open by staff ID", err, workforce.ErrWorkSessionNotFound)
	}
	return workSessionToLegacy(value), nil
}

func (r workSessionRepository) GetOpenByStaffAndDate(ctx context.Context, staffID int64, date calendar.Date) (*WorkSession, error) {
	value, err := r.workforce.OpenWorkSessionOn(ctx, staffID, date.String())
	if err != nil {
		return nil, r.shapes.read("get current by staff ID", err, workforce.ErrWorkSessionNotFound)
	}
	return workSessionToLegacy(value), nil
}

func (r workSessionRepository) GetOpenByStaffAndDateForUpdate(ctx context.Context, staffID int64, date calendar.Date) (*WorkSession, error) {
	value, err := r.workforce.LockOpenWorkSessionOn(ctx, staffID, date.String())
	if err != nil {
		return nil, r.shapes.read("get current by staff ID", err, workforce.ErrWorkSessionNotFound)
	}
	return workSessionToLegacy(value), nil
}

func (r workSessionRepository) LockOpenByIDForUpdate(ctx context.Context, id int64) (*WorkSession, error) {
	value, err := r.workforce.LockOpenWorkSession(ctx, id)
	if err != nil {
		return nil, r.shapes.read("lock open session by ID", err, workforce.ErrWorkSessionNotFound)
	}
	return workSessionToLegacy(value), nil
}

func (r workSessionRepository) ListOverlappingByStaffID(ctx context.Context, staffID int64, from time.Time, to *time.Time) ([]*WorkSession, error) {
	values, err := r.workforce.ListOverlappingWorkSessions(ctx, []int64{staffID}, from, to)
	if err != nil {
		return nil, r.shapes.read("list overlapping by staff ID", err, nil)
	}
	return workSessionsToLegacy(values), nil
}

func (r workSessionRepository) ListOverlappingByStaffIDs(ctx context.Context, staffIDs []int64, from time.Time, to *time.Time) (map[int64][]*WorkSession, error) {
	result := make(map[int64][]*WorkSession, len(staffIDs))
	if len(staffIDs) == 0 {
		return result, nil
	}
	values, err := r.workforce.ListOverlappingWorkSessions(ctx, staffIDs, from, to)
	if err != nil {
		return nil, r.shapes.read("list overlapping by staff IDs", err, nil)
	}
	for _, value := range values {
		result[value.StaffID] = append(result[value.StaffID], workSessionToLegacy(value))
	}
	return result, nil
}

func (r workSessionRepository) GetHistoryByStaffID(ctx context.Context, staffID int64, from, to calendar.Date) ([]*WorkSession, error) {
	return r.list(ctx, "get history by staff ID", workforce.WorkSessionFilter{
		StaffID: staffID, DateFrom: from.String(), DateTo: to.String(),
		Order: workSessionOrder(workforce.WorkSessionOrderDate, workforce.WorkSessionOrderCheckInTime),
	})
}

func (r workSessionRepository) GetHistoryByStaffIDs(ctx context.Context, staffIDs []int64, from, to calendar.Date) (map[int64][]*WorkSession, error) {
	result := make(map[int64][]*WorkSession, len(staffIDs))
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

func (r workSessionRepository) GetOpenSessions(ctx context.Context, beforeDate calendar.Date) ([]*WorkSession, error) {
	open := true
	return r.list(ctx, "get open sessions", workforce.WorkSessionFilter{DateBefore: beforeDate.String(), Open: &open})
}

func (r workSessionRepository) GetTodayPresenceMap(ctx context.Context) (map[int64]string, error) {
	presence, err := r.workforce.WorkPresenceMap(ctx)
	if err != nil {
		return nil, r.shapes.read("get today presence map", err, nil)
	}
	return presence, nil
}

func (r workSessionRepository) CloseSession(ctx context.Context, id int64, checkOutTime time.Time, autoCheckedOut bool) (bool, error) {
	closed, err := r.workforce.CloseWorkSession(ctx, id, checkOutTime, autoCheckedOut)
	if err != nil {
		return false, r.shapes.workSessionWrite("close session", err)
	}
	return closed, nil
}

func (r workSessionRepository) UpdateBreakMinutes(ctx context.Context, id int64, breakMinutes int) error {
	affected, err := r.workforce.SetWorkSessionBreakMinutes(ctx, id, breakMinutes)
	if err != nil {
		return r.shapes.workSessionWrite("update columns WorkSession", err)
	}
	if affected != 1 {
		return r.shapes.Database("update break minutes", fmt.Errorf("expected 1 rows affected, got %d", affected))
	}
	return nil
}

func (r workSessionRepository) list(ctx context.Context, op string, filter workforce.WorkSessionFilter) ([]*WorkSession, error) {
	values, err := r.workforce.ListWorkSessions(ctx, filter)
	if err != nil {
		return nil, r.shapes.read(op, err, nil)
	}
	return workSessionsToLegacy(values), nil
}

// --- work session breaks ---

type workSessionBreakRepository struct {
	workforce workforce.Capability
	shapes    RowErrorShapes
}

// NewWorkSessionBreakRows serves WorkSessionBreakRepository
// from the Workforce capability.
func NewWorkSessionBreakRows(capability workforce.Capability, shapes RowErrorShapes) WorkSessionBreakRepository {
	if capability == nil {
		panic("work session break repository adapter: Workforce capability is required")
	}
	return workSessionBreakRepository{workforce: capability, shapes: shapes}
}

func (r workSessionBreakRepository) Create(ctx context.Context, entity *WorkSessionBreak) error {
	if entity == nil {
		return errors.New("WorkSessionBreak cannot be nil or zero value")
	}
	if err := entity.Validate(); err != nil {
		return err
	}
	created, err := r.workforce.CreateWorkSessionBreak(ctx, WorkSessionBreakToCapability(entity))
	if err != nil {
		return r.shapes.workSessionWrite("create", err)
	}
	applyWorkSessionBreakToLegacy(entity, created)
	return nil
}

func (r workSessionBreakRepository) FindByID(ctx context.Context, id any) (*WorkSessionBreak, error) {
	breakID, err := legacyID(id)
	if err != nil {
		return nil, r.shapes.Database("find by id", err)
	}
	value, err := r.workforce.FindWorkSessionBreak(ctx, breakID)
	if err != nil {
		return nil, r.shapes.read("find by id", err, workforce.ErrWorkSessionBreakNotFound)
	}
	return workSessionBreakToLegacy(value), nil
}

func (r workSessionBreakRepository) Update(ctx context.Context, entity *WorkSessionBreak) error {
	if entity == nil {
		return errors.New("WorkSessionBreak cannot be nil or zero value")
	}
	if err := entity.Validate(); err != nil {
		return err
	}
	updated, err := r.workforce.UpdateWorkSessionBreak(ctx, WorkSessionBreakToCapability(entity))
	if err != nil {
		if errors.Is(err, workforce.ErrWorkSessionBreakNotFound) {
			return r.shapes.rowsAffected("update WorkSessionBreak")
		}
		return r.shapes.workSessionWrite("update", err)
	}
	applyWorkSessionBreakToLegacy(entity, updated)
	return nil
}

func (r workSessionBreakRepository) Delete(ctx context.Context, id any) error {
	breakID, err := legacyID(id)
	if err != nil {
		return r.shapes.Database("delete", err)
	}
	if err := r.workforce.DeleteWorkSessionBreak(ctx, breakID); err != nil {
		return r.shapes.workSessionWrite("delete", err)
	}
	return nil
}

func (r workSessionBreakRepository) GetBySessionID(ctx context.Context, sessionID int64) ([]*WorkSessionBreak, error) {
	values, err := r.workforce.ListWorkSessionBreaks(ctx, workforce.WorkSessionBreakFilter{SessionID: sessionID})
	if err != nil {
		return nil, r.shapes.read("get breaks by session ID", err, nil)
	}
	return workSessionBreaksToLegacy(values), nil
}

func (r workSessionBreakRepository) GetBySessionIDs(ctx context.Context, sessionIDs []int64) (map[int64][]*WorkSessionBreak, error) {
	result := make(map[int64][]*WorkSessionBreak, len(sessionIDs))
	if len(sessionIDs) == 0 {
		return result, nil
	}
	values, err := r.workforce.ListWorkSessionBreaks(ctx, workforce.WorkSessionBreakFilter{SessionIDs: sessionIDs})
	if err != nil {
		return nil, r.shapes.read("get breaks by session IDs", err, nil)
	}
	for _, value := range values {
		result[value.SessionID] = append(result[value.SessionID], workSessionBreakToLegacy(value))
	}
	return result, nil
}

func (r workSessionBreakRepository) GetActiveBySessionID(ctx context.Context, sessionID int64) (*WorkSessionBreak, error) {
	active := true
	values, err := r.workforce.ListWorkSessionBreaks(ctx, workforce.WorkSessionBreakFilter{SessionID: sessionID, Active: &active, Limit: 1})
	if err != nil {
		return nil, r.shapes.read("get active break by session ID", err, nil)
	}
	if len(values) == 0 {
		return nil, nil
	}
	return workSessionBreakToLegacy(values[0]), nil
}

func (r workSessionBreakRepository) EndBreak(ctx context.Context, id int64, endedAt time.Time, durationMinutes int) error {
	affected, err := r.workforce.EndWorkSessionBreak(ctx, id, endedAt, durationMinutes)
	if err != nil {
		return r.shapes.workSessionWrite("end break", err)
	}
	if affected != 1 {
		return r.shapes.Database("end break", fmt.Errorf("expected 1 rows affected, got %d", affected))
	}
	return nil
}

func (r workSessionBreakRepository) UpdateDuration(ctx context.Context, id int64, durationMinutes int, endedAt time.Time) error {
	if _, err := r.workforce.SetWorkSessionBreakDuration(ctx, id, durationMinutes, endedAt); err != nil {
		return r.shapes.workSessionWrite("update columns WorkSessionBreak", err)
	}
	return nil
}

func (r workSessionBreakRepository) GetExpiredBreaks(ctx context.Context, before time.Time) ([]*WorkSessionBreak, error) {
	values, err := r.workforce.ExpiredWorkSessionBreaks(ctx, before)
	if err != nil {
		return nil, r.shapes.read("get expired breaks", err, nil)
	}
	return workSessionBreaksToLegacy(values), nil
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
func (s RowErrorShapes) workSessionWrite(op string, err error) error {
	if errors.Is(err, workforce.ErrInvalidWorkSession) {
		return errors.New(err.Error())
	}
	return s.Database(op, err)
}

// --- mapping ---

func workSessionToLegacy(value workforce.WorkSession) *WorkSession {
	entity := &WorkSession{}
	applyWorkSessionToLegacy(entity, value)
	return entity
}

func applyWorkSessionToLegacy(entity *WorkSession, value workforce.WorkSession) {
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

func workSessionsToLegacy(values []workforce.WorkSession) []*WorkSession {
	// Empty, not nil: callers serialize the result straight to JSON.
	result := make([]*WorkSession, 0, len(values))
	for _, value := range values {
		result = append(result, workSessionToLegacy(value))
	}
	return result
}

func workSessionBreakToLegacy(value workforce.WorkSessionBreak) *WorkSessionBreak {
	entity := &WorkSessionBreak{}
	applyWorkSessionBreakToLegacy(entity, value)
	return entity
}

func applyWorkSessionBreakToLegacy(entity *WorkSessionBreak, value workforce.WorkSessionBreak) {
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

func workSessionBreaksToLegacy(values []workforce.WorkSessionBreak) []*WorkSessionBreak {
	result := make([]*WorkSessionBreak, 0, len(values))
	for _, value := range values {
		result = append(result, workSessionBreakToLegacy(value))
	}
	return result
}

// WorkSessionToCapability maps a time-tracking work session onto the
// Workforce capability record.
func WorkSessionToCapability(session *WorkSession) workforce.WorkSession {
	if session == nil {
		return workforce.WorkSession{}
	}
	return workforce.WorkSession{
		ID: session.ID, TenantID: session.TenantID, StaffID: session.StaffID, Date: session.Date.String(),
		Status: session.Status, Source: session.Source, CheckInTime: session.CheckInTime, CheckOutTime: session.CheckOutTime,
		ReopenedAt: session.ReopenedAt, BreakMinutes: session.BreakMinutes, Notes: session.Notes,
		AutoCheckedOut: session.AutoCheckedOut, CreatedBy: session.CreatedBy, UpdatedBy: session.UpdatedBy,
		CreatedAt: session.CreatedAt, UpdatedAt: session.UpdatedAt,
	}
}

// WorkSessionBreakToCapability maps a time-tracking break onto the
// Workforce capability record.
func WorkSessionBreakToCapability(workBreak *WorkSessionBreak) workforce.WorkSessionBreak {
	if workBreak == nil {
		return workforce.WorkSessionBreak{}
	}
	return workforce.WorkSessionBreak{
		ID: workBreak.ID, TenantID: workBreak.TenantID, SessionID: workBreak.SessionID, StartedAt: workBreak.StartedAt,
		EndedAt: workBreak.EndedAt, DurationMinutes: workBreak.DurationMinutes, PlannedEndTime: workBreak.PlannedEndTime,
		CreatedAt: workBreak.CreatedAt, UpdatedAt: workBreak.UpdatedAt,
	}
}
