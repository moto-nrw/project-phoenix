package timetable

import "context"

// CareExitRosterRow is one planned participant a care exit removes or
// restores. The attendance the participant carried belongs to Student
// Presence; the caller reads it before the removal and hands it back after
// the restore, keyed by ParticipantID.
type CareExitRosterRow struct {
	ParticipantID int64  `json:"participant_id"`
	TenantID      int64  `json:"tenant_id"`
	StudentID     int64  `json:"student_id"`
	InstanceID    int64  `json:"instance_id"`
	RoomID        *int64 `json:"room_id"`
}

// CareExitRoster is what ending a child's care needs from the timetable: the
// roster rows it removes and restores, and the baseline its preview counts.
type CareExitRoster interface {
	CareExitRosterCommand
	CareExitBaselineQuery
}

type CareExitRosterCommand interface {
	LockPlannedRosterForCareExit(context.Context, []int64, string) error
	// PreviewPlannedRosterForCareExit lists the rows the removal would take,
	// so the caller can read their attendance while they still exist.
	PreviewPlannedRosterForCareExit(context.Context, []int64, string) ([]CareExitRosterRow, error)
	RemovePlannedRosterForCareExit(context.Context, []int64, string) ([]CareExitRosterRow, error)
	// RestoreRosterForCareExit puts removed rows back onto rosters that have
	// not ended or been cancelled and returns them with their new ids.
	RestoreRosterForCareExit(context.Context, []int64, []CareExitRosterRow) ([]CareExitRosterRow, error)
}

func (m *Module) LockPlannedRosterForCareExit(ctx context.Context, studentIDs []int64, after string) error {
	if hasInvalidID(studentIDs) || !validDate(after) {
		return m.reject("lock_planned_roster_for_care_exit", ErrInvalidInstanceStudentQuery)
	}
	return m.engine.LockPlannedRosterForCareExit(ctx, studentIDs, after)
}

func (m *Module) PreviewPlannedRosterForCareExit(ctx context.Context, studentIDs []int64, after string) ([]CareExitRosterRow, error) {
	if hasInvalidID(studentIDs) || !validDate(after) {
		return nil, m.reject("preview_planned_roster_for_care_exit", ErrInvalidInstanceStudentQuery)
	}
	return m.engine.PreviewPlannedRosterForCareExit(ctx, studentIDs, after)
}

func (m *Module) RemovePlannedRosterForCareExit(ctx context.Context, studentIDs []int64, after string) ([]CareExitRosterRow, error) {
	if hasInvalidID(studentIDs) || !validDate(after) {
		return nil, m.reject("remove_planned_roster_for_care_exit", ErrInvalidInstanceStudentQuery)
	}
	return m.engine.RemovePlannedRosterForCareExit(ctx, studentIDs, after)
}

func (m *Module) RestoreRosterForCareExit(ctx context.Context, studentIDs []int64, rows []CareExitRosterRow) ([]CareExitRosterRow, error) {
	if hasInvalidID(studentIDs) {
		return nil, m.reject("restore_roster_for_care_exit", ErrInvalidInstanceStudentQuery)
	}
	for _, row := range rows {
		if row.TenantID <= 0 || row.StudentID <= 0 || row.InstanceID <= 0 {
			return nil, m.reject("restore_roster_for_care_exit", ErrInvalidInstanceStudent)
		}
	}
	return m.engine.RestoreRosterForCareExit(ctx, studentIDs, rows)
}
