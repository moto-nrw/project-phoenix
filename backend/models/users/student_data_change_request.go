package users

import (
	"encoding/json"
	"errors"
	"time"
)

// ErrChangeRequestNotPending means a pending-row transition lost a race or the
// row was already terminal under the caller's tenant.
var ErrChangeRequestNotPending = errors.New("users: change request is not pending")

// ErrChangeRequestNotFound means no row with the requested id exists in the
// caller's tenant.
var ErrChangeRequestNotFound = errors.New("users: change request not found")

// ErrChangeRequestNotDecided means a correction was attempted on a row that
// carries no decision to correct — still pending, audit-only, or closed some
// other way.
var ErrChangeRequestNotDecided = errors.New("users: change request is not decided")

// Change-request lifecycle states. Track A (direct edit) writes rows directly
// as auto_applied — the live record was already updated, the row is the audit
// trail. Track B (review) writes pending rows that a staff decision moves to
// approved (and applies) or rejected.
const (
	DataChangeStatusAutoApplied = "auto_applied"
	DataChangeStatusPending     = "pending"
	DataChangeStatusApproved    = "approved"
	DataChangeStatusRejected    = "rejected"
	// DataChangeStatusDone closes a request that only covered days already
	// gone: nothing to apply, and "abgelehnt" would misstate what happened.
	DataChangeStatusDone = "done"
	// DataChangeStatusCareEnded closes an open request whose child left the
	// OGS before anybody decided it (#2487). Deliberately its own value:
	// "abgelehnt" would tell the family the office looked at the change and
	// said no, which is not what happened.
	DataChangeStatusCareEnded = "care_ended"
)

// Target identifies which underlying record a change applies to. For person /
// student / departure the student_id is enough to locate the row; for
// guardian_profile / guardian_phone the TargetRefID carries the specific
// sub-record id (the submitting guardian's own profile / phone).
const (
	DataChangeTargetPerson          = "person"
	DataChangeTargetStudent         = "student"
	DataChangeTargetGuardianProfile = "guardian_profile"
	DataChangeTargetGuardianPhone   = "guardian_phone"
	DataChangeTargetDeparture       = "departure"
)

// StudentDataChangeRequest is one parent-initiated change to a child's
// Stammdaten. One row per changed field, so Track B requests can be approved or
// rejected field-by-field and the audit log reads as a flat history of changes.
type StudentDataChangeRequest struct {
	Model `bun:"schema:users,table:student_data_change_requests"`
	TenantModel
	StudentID    int64           `bun:"student_id,notnull" json:"student_id"`
	SubmittedBy  int64           `bun:"submitted_by,notnull" json:"submitted_by"`
	Target       string          `bun:"target,notnull" json:"target"`
	TargetRefID  *int64          `bun:"target_ref_id" json:"target_ref_id,omitempty"`
	FieldKey     string          `bun:"field_key,notnull" json:"field_key"`
	OldValue     json.RawMessage `bun:"old_value,type:jsonb" json:"old_value,omitempty"`
	NewValue     json.RawMessage `bun:"new_value,type:jsonb,notnull" json:"new_value"`
	Status       string          `bun:"status,notnull,default:'pending'" json:"status"`
	ReviewReason *string         `bun:"review_reason" json:"review_reason,omitempty"`
	ReviewedBy   *int64          `bun:"reviewed_by" json:"reviewed_by,omitempty"`
	ReviewedAt   *time.Time      `bun:"reviewed_at" json:"reviewed_at,omitempty"`
	AppliedAt    *time.Time      `bun:"applied_at" json:"applied_at,omitempty"`
}

// IsTerminal reports whether the row is in a final state — a Track A audit row
// (auto_applied) or a decided Track B row (approved/rejected). Only pending
// rows accept a staff decision.
func (c *StudentDataChangeRequest) IsTerminal() bool {
	switch c.Status {
	case DataChangeStatusAutoApplied, DataChangeStatusApproved, DataChangeStatusRejected,
		DataChangeStatusCareEnded, DataChangeStatusDone:
		return true
	default:
		return false
	}
}
