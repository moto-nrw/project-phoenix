package audit

import (
	"context"
	"errors"
	"time"
)

// WorkSessionEdit records a single field change on a work session for audit trail
type WorkSessionEdit struct {
	ID int64 `bun:"id,pk,autoincrement" json:"id"`
	TenantModel
	SessionID int64     `bun:"session_id,notnull" json:"session_id"`
	StaffID   int64     `bun:"staff_id,notnull" json:"staff_id"`
	EditedBy  int64     `bun:"edited_by,notnull" json:"edited_by"`
	FieldName string    `bun:"field_name,notnull" json:"field_name"`
	OldValue  *string   `bun:"old_value" json:"old_value"`
	NewValue  *string   `bun:"new_value" json:"new_value"`
	Notes     *string   `bun:"notes" json:"notes"`
	CreatedAt time.Time `bun:"created_at,notnull,default:now()" json:"created_at"`
}

// SystemEditorID is the sentinel edited_by value for changes performed by
// the system itself (e.g. the automatic checkout at planned shift end,
// #1798). The column has no FK, so 0 is storable; display code renders it
// as "System".
const SystemEditorID int64 = 0

// Valid field names for audit entries
const (
	FieldDate          = "date"
	FieldCheckInTime   = "check_in_time"
	FieldCheckOutTime  = "check_out_time"
	FieldBreakMinutes  = "break_minutes"
	FieldBreakDuration = "break_duration"
	FieldStatus        = "status"
	FieldNotes         = "notes"
	// FieldDeviationReason records the mandatory reason for stamping outside
	// the tolerance window around the planned shift window (F9). OldValue
	// holds the planned wall-clock time, NewValue the actual one, Notes the
	// reason given by the staff member.
	FieldDeviationReason = "deviation_reason"
)

// Validate ensures the edit record is valid
func (e *WorkSessionEdit) Validate() error {
	if e.SessionID <= 0 {
		return errors.New("session ID is required")
	}
	if e.StaffID <= 0 {
		return errors.New("staff ID is required")
	}
	// SystemEditorID (0) marks system actions (#1798); negative IDs stay invalid.
	if e.EditedBy < 0 {
		return errors.New("edited by is required")
	}
	if e.FieldName == "" {
		return errors.New("field name is required")
	}

	switch e.FieldName {
	case FieldDate, FieldCheckInTime, FieldCheckOutTime, FieldBreakMinutes, FieldBreakDuration, FieldStatus, FieldNotes, FieldDeviationReason:
		// Valid field names
	default:
		return errors.New("invalid field name")
	}

	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now()
	}

	return nil
}

// WorkSessionEditRepository defines operations for managing work session edit audit records
type WorkSessionEditRepository interface {
	CreateBatch(ctx context.Context, edits []*WorkSessionEdit) error
	GetBySessionID(ctx context.Context, sessionID int64) ([]*WorkSessionEdit, error)
	CountBySessionID(ctx context.Context, sessionID int64) (int, error)
	CountBySessionIDs(ctx context.Context, sessionIDs []int64) (map[int64]int, error)
	// CountManualBySessionIDs excludes system-authored edits (edited_by =
	// SystemEditorID) and FieldDeviationReason rows — a stamp-time deviation
	// reason is audit trail, not a manual correction (#1844).
	CountManualBySessionIDs(ctx context.Context, sessionIDs []int64) (map[int64]int, error)
}
