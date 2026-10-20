package timetracking

import (
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The clock uses calendar-date records for its calculations. These functions
// translate values only; native reads and writes are issued by the callers.
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
	return recordErrors.WrapRecordError(op, err)
}

// --- mapping ---

func workSessionToRecord(value workforce.WorkSession) *WorkSession {
	entity := &WorkSession{}
	applyWorkSessionToRecord(entity, value)
	return entity
}

func applyWorkSessionToRecord(entity *WorkSession, value workforce.WorkSession) {
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

func workSessionsToRecords(values []workforce.WorkSession) []*WorkSession {
	// Empty, not nil: callers serialize the result straight to JSON.
	result := make([]*WorkSession, 0, len(values))
	for _, value := range values {
		result = append(result, workSessionToRecord(value))
	}
	return result
}

func workSessionBreakToRecord(value workforce.WorkSessionBreak) *WorkSessionBreak {
	entity := &WorkSessionBreak{}
	applyWorkSessionBreakToRecord(entity, value)
	return entity
}

func applyWorkSessionBreakToRecord(entity *WorkSessionBreak, value workforce.WorkSessionBreak) {
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

func workSessionBreaksToRecords(values []workforce.WorkSessionBreak) []*WorkSessionBreak {
	result := make([]*WorkSessionBreak, 0, len(values))
	for _, value := range values {
		result = append(result, workSessionBreakToRecord(value))
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

func boolPointer(value bool) *bool { return &value }

func workSessionsByStaff(values []workforce.WorkSession) map[int64][]*WorkSession {
	result := make(map[int64][]*WorkSession)
	for _, value := range values {
		result[value.StaffID] = append(result[value.StaffID], workSessionToRecord(value))
	}
	return result
}

func workBreaksBySession(values []workforce.WorkSessionBreak) map[int64][]*WorkSessionBreak {
	result := make(map[int64][]*WorkSessionBreak)
	for _, value := range values {
		result[value.SessionID] = append(result[value.SessionID], workSessionBreakToRecord(value))
	}
	return result
}

func firstWorkSessionBreak(values []workforce.WorkSessionBreak) *WorkSessionBreak {
	if len(values) == 0 {
		return nil
	}
	return workSessionBreakToRecord(values[0])
}

// breakMutationError preserves the conditional update's affected-row contract.
func breakMutationError(op string, affected int64, err error) error {
	if err != nil {
		return workSessionWriteError(op, err)
	}
	if affected != 1 {
		return recordErrors.WrapRecordError(op, fmt.Errorf("expected 1 rows affected, got %d", affected))
	}
	return nil
}

// workSessionReadRecord keeps failed lookups nil: some clock flows explicitly
// ignore not-found before deciding whether to create today's first block.
func workSessionReadRecord(value workforce.WorkSession, err error) *WorkSession {
	if err != nil {
		return nil
	}
	return workSessionToRecord(value)
}

// workSessionDateResult preserves the native read's envelope and the calendar
// parser's own failure, as the former retention row port did.
func workSessionDateResult(value string, err error) (*calendar.Date, error) {
	if err != nil {
		return nil, rowReadError("oldest before", err, nil)
	}
	if value == "" {
		return nil, nil
	}
	date, err := calendar.ParseDate(value)
	if err != nil {
		return nil, err
	}
	return &date, nil
}
