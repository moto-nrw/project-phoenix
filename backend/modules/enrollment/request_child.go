package enrollment

import (
	"encoding/json"
	"time"
)

// RequestChild is the intake record, distinct from any materialized student.
type RequestChild struct {
	ID                    int64           `json:"id"`
	TenantID              int64           `json:"tenant_id"`
	CreatedAt             time.Time       `json:"created_at"`
	UpdatedAt             time.Time       `json:"updated_at"`
	RequestID             int64           `json:"request_id"`
	FirstName             string          `json:"first_name"`
	LastName              string          `json:"last_name"`
	DateOfBirth           Date            `json:"date_of_birth"`
	TargetGradeLevel      *int16          `json:"target_grade_level,omitempty"`
	TargetSchoolClass     *string         `json:"target_school_class,omitempty"`
	CustomData            json.RawMessage `json:"custom_data"`
	Status                string          `json:"status"`
	StatusReason          *string         `json:"status_reason,omitempty"`
	ActivationMode        string          `json:"activation_mode"`
	ActivateOn            *Date           `json:"activate_on,omitempty"`
	ReviewedAt            *time.Time      `json:"reviewed_at,omitempty"`
	ReviewedBy            *int64          `json:"reviewed_by,omitempty"`
	CreatedStudentID      *int64          `json:"created_student_id,omitempty"`
	MatchedStudentID      *int64          `json:"matched_student_id,omitempty"`
	SortOrder             int             `json:"sort_order"`
	RolloverSourceChildID *int64          `json:"rollover_source_child_id,omitempty"`
	ReviewReason          *string         `json:"review_reason,omitempty"`
}

// Per-child status values matching the column CHECK constraint.
//
// The rollover statuses (pending_renewal, auto_renewed,
// pending_admin_review) were added in migration 1.15.62 for the annual
// phase rollover flow:
//   - pending_renewal: opt-in rollover, waiting for parent to confirm.
//     Demoted to withdrawn by the deadline worker.
//   - auto_renewed: opt-out rollover, waiting for parent to decline.
//     Promoted to submitted (or approved when rollover_auto_approve)
//     by the deadline worker.
//   - pending_admin_review: rollover couldn't be auto-resolved (grade
//     above max, missing grade level, etc). Never resolved by the
//     deadline worker — admin must decide via the review queue.
const (
	ChildStatusSubmitted          = "submitted"
	ChildStatusUnderReview        = "under_review"
	ChildStatusApproved           = "approved"
	ChildStatusWaitlisted         = "waitlisted"
	ChildStatusRejected           = "rejected"
	ChildStatusWithdrawn          = "withdrawn"
	ChildStatusPendingRenewal     = "pending_renewal"
	ChildStatusAutoRenewed        = "auto_renewed"
	ChildStatusPendingAdminReview = "pending_admin_review"
)

// ReviewReason codes set when a rollover row lands in
// pending_admin_review. Free text would be tempting but a constant
// set lets the frontend show localised labels without parsing.
const (
	ReviewReasonGradeAboveMax = "grade_above_max"
	ReviewReasonNoGradeLevel  = "no_grade_level"
)

// Activation mode values matching the column CHECK constraint.
const (
	ChildActivationImmediate = "immediate"
	ChildActivationScheduled = "scheduled"
)

// IsTerminal reports whether a child has an approved, rejected, or withdrawn status.
func (c *RequestChild) IsTerminal() bool {
	switch c.Status {
	case ChildStatusApproved, ChildStatusRejected, ChildStatusWithdrawn:
		return true
	default:
		return false
	}
}
