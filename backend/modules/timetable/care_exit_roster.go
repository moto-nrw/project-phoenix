package timetable

import (
	"context"
	"time"
)

// CareExitRosterRow preserves a removed plan verbatim so changing a future
// care exit does not overwrite a supervisor's local decisions.
type CareExitRosterRow struct {
	TenantID           int64      `json:"tenant_id"`
	StudentID          int64      `json:"student_id"`
	InstanceID         int64      `json:"instance_id"`
	RoomID             *int64     `json:"room_id"`
	Status             string     `json:"status"`
	Substatus          *string    `json:"substatus"`
	Note               *string    `json:"note"`
	IsUnplanned        bool       `json:"is_unplanned"`
	NotScheduled       bool       `json:"not_scheduled"`
	ManualStatusAt     *time.Time `json:"manual_status_at"`
	StudentStatusDayID *int64     `json:"student_status_day_id"`
	PickupExceptionID  *int64     `json:"pickup_exception_id"`
}

type CareExitRosterCommand interface {
	LockPlannedRosterForCareExit(context.Context, []int64, string) error
	RemovePlannedRosterForCareExit(context.Context, []int64, string) ([]CareExitRosterRow, error)
	RestoreRosterForCareExit(context.Context, []int64, []CareExitRosterRow) (int, error)
}

func (m *Module) LockPlannedRosterForCareExit(ctx context.Context, studentIDs []int64, after string) error {
	if hasInvalidID(studentIDs) || !validDate(after) {
		return m.reject("lock_planned_roster_for_care_exit", ErrInvalidInstanceStudentQuery)
	}
	return m.engine.LockPlannedRosterForCareExit(ctx, studentIDs, after)
}
func (m *Module) RemovePlannedRosterForCareExit(ctx context.Context, studentIDs []int64, after string) ([]CareExitRosterRow, error) {
	if hasInvalidID(studentIDs) || !validDate(after) {
		return nil, m.reject("remove_planned_roster_for_care_exit", ErrInvalidInstanceStudentQuery)
	}
	return m.engine.RemovePlannedRosterForCareExit(ctx, studentIDs, after)
}
func (m *Module) RestoreRosterForCareExit(ctx context.Context, studentIDs []int64, rows []CareExitRosterRow) (int, error) {
	if hasInvalidID(studentIDs) {
		return 0, m.reject("restore_roster_for_care_exit", ErrInvalidInstanceStudentQuery)
	}
	for _, row := range rows {
		if row.TenantID <= 0 || row.StudentID <= 0 || row.InstanceID <= 0 {
			return 0, m.reject("restore_roster_for_care_exit", ErrInvalidInstanceStudent)
		}
	}
	return m.engine.RestoreRosterForCareExit(ctx, studentIDs, rows)
}
