package audit

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// Audited attendance fields. These are exactly the three columns the
// attendance PATCH can change on schedule.instance_students; check-in and
// check-out timestamps are written by the operational flow, not corrected by
// hand, and are therefore not audited here.
const (
	AttendanceFieldStatus    = "status"
	AttendanceFieldSubstatus = "substatus"
	AttendanceFieldNote      = "note"
)

// CorrectionReasonMaxLength caps the stated reason. It matches the 500-char
// ceiling of the attendance note itself: a reason needs no more room than the
// entry it explains.
const CorrectionReasonMaxLength = 500

// AttendanceCorrection is the append-only trail for a single field change on a
// child's attendance row in a COMPLETED activity instance (#2898). CreatedAt is
// the moment of the correction.
//
// The catalogue promise behind this table is that entries in the digital class
// book stay correctable AND that every correction is traceable. Three
// consequences shape the fields below:
//
//   - Reason is mandatory. A correction of a closed record without a stated
//     reason is weak evidence. The same rule already governs manual
//     work-session edits, for the same reason.
//   - OldValue/NewValue are kept as text. Unlike guardian contact data these
//     are not third-party PII: status and substatus are closed vocabularies,
//     and the free note is authored by staff about the care day. Retaining
//     them is what makes the trail usable — "note changed" without the
//     before/after would not let anyone reconstruct what happened.
//   - There is no "was it after completion" flag: only the correction endpoint
//     writes here, and it serves completed instances exclusively. Ordinary
//     in-flight edits keep going through the attendance PATCH and are not
//     corrections.
//
// ActorAccountID is nulled when the acting account is deleted later; the name
// snapshot preserves who corrected the entry.
type AttendanceCorrection struct {
	base.Model `bun:"schema:audit,table:attendance_corrections"`
	base.TenantModel
	InstanceID        int64   `bun:"instance_id,notnull" json:"instance_id"`
	StudentID         int64   `bun:"student_id,notnull" json:"student_id"`
	ActorAccountID    *int64  `bun:"actor_account_id" json:"actor_account_id,omitempty"`
	ActorNameSnapshot *string `bun:"actor_name_snapshot" json:"actor_name_snapshot,omitempty"`
	FieldName         string  `bun:"field_name,notnull" json:"field_name"`
	OldValue          *string `bun:"old_value" json:"old_value,omitempty"`
	NewValue          *string `bun:"new_value" json:"new_value,omitempty"`
	Reason            string  `bun:"reason,notnull" json:"reason"`
}

// Validate rejects rows that would make the trail unreadable. It deliberately
// does NOT require a value change: clearing a note to NULL is a correction,
// and both values being nil is the caller's bug, caught by the field check.
func (c *AttendanceCorrection) Validate() error {
	if c.InstanceID <= 0 {
		return errors.New("instance ID is required")
	}
	if c.StudentID <= 0 {
		return errors.New("student ID is required")
	}
	switch c.FieldName {
	case AttendanceFieldStatus, AttendanceFieldSubstatus, AttendanceFieldNote:
	default:
		return errors.New("invalid attendance field name")
	}
	c.Reason = strings.TrimSpace(c.Reason)
	if c.Reason == "" {
		return errors.New("reason is required")
	}
	if utf8.RuneCountInString(c.Reason) > CorrectionReasonMaxLength {
		return errors.New("reason is too long")
	}
	return nil
}

// AttendanceCorrectionRepository is the data-access boundary for the
// attendance correction trail.
type AttendanceCorrectionRepository interface {
	// CreateBatch appends every correction of one write in a single insert.
	// An empty slice is a no-op so callers need no length guard.
	CreateBatch(ctx context.Context, corrections []*AttendanceCorrection) error
	// ListByInstanceAndStudent returns the trail of one child in one instance,
	// newest first — the shape the detail view reads.
	ListByInstanceAndStudent(ctx context.Context, instanceID, studentID int64) ([]*AttendanceCorrection, error)
	// CountByInstanceAndStudents reports how many corrections each child of an
	// instance carries, so a list can mark corrected rows without N queries.
	CountByInstanceAndStudents(ctx context.Context, instanceID int64, studentIDs []int64) (map[int64]int, error)
}
