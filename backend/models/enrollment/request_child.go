package enrollment

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

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

// RequestChild is a row in enrollment.request_children - one row per
// child included in a parent submission. Status transitions per child
// independently; the parent request's overall status is derived from the
// per-child set (see Request.RequestStatus* constants).
type RequestChild struct {
	base.Model `bun:"schema:enrollment,table:request_children"`
	base.TenantModel
	RequestID        int64          `bun:"request_id,notnull" json:"request_id"`
	FirstName        string         `bun:"first_name,notnull" json:"first_name"`
	LastName         string         `bun:"last_name,notnull" json:"last_name"`
	DateOfBirth      time.Time      `bun:"date_of_birth,notnull,type:date" json:"date_of_birth"`
	TargetGradeLevel *int16         `bun:"target_grade_level" json:"target_grade_level,omitempty"`
	CustomData       map[string]any `bun:"custom_data,type:jsonb,notnull,default:'{}'" json:"custom_data"`
	Status           string         `bun:"status,notnull,default:'submitted'" json:"status"`
	StatusReason     *string        `bun:"status_reason" json:"status_reason,omitempty"`
	ActivationMode   string         `bun:"activation_mode,notnull,default:'scheduled'" json:"activation_mode"`
	ActivateOn       *time.Time     `bun:"activate_on,type:date" json:"activate_on,omitempty"`
	ReviewedAt       *time.Time     `bun:"reviewed_at" json:"reviewed_at,omitempty"`
	ReviewedBy       *int64         `bun:"reviewed_by" json:"reviewed_by,omitempty"`
	CreatedStudentID *int64         `bun:"created_student_id" json:"created_student_id,omitempty"`
	SortOrder        int            `bun:"sort_order,notnull,default:0" json:"sort_order"`

	// Rollover columns (migration 1.15.62). NULL on rows created via
	// the public form; set by RolloverService when a previous-year
	// approved child is carried forward into a new phase.
	//
	// RolloverSourceChildID — the previous-year row this one was
	// derived from. Unique index ensures one-to-one mapping.
	// ReviewReason — populated only when Status == pending_admin_review;
	// drives the localised label in the admin review queue.
	RolloverSourceChildID *int64  `bun:"rollover_source_child_id" json:"rollover_source_child_id,omitempty"`
	ReviewReason          *string `bun:"review_reason" json:"review_reason,omitempty"`
}

func (c *RequestChild) TableName() string {
	return "enrollment.request_children"
}

// IsTerminal returns true when this child's status is approved, rejected,
// or withdrawn - i.e., no further admin decision can change it (other than
// promotion of a waitlisted child, but waitlisted is non-terminal).
func (c *RequestChild) IsTerminal() bool {
	switch c.Status {
	case ChildStatusApproved, ChildStatusRejected, ChildStatusWithdrawn:
		return true
	default:
		return false
	}
}

// IsRenewalPending returns true while this child is waiting on a
// parent action (opt-in confirm) or deadline (opt-out decline). Used
// by the deadline worker to scope its scan.
func (c *RequestChild) IsRenewalPending() bool {
	return c.Status == ChildStatusPendingRenewal || c.Status == ChildStatusAutoRenewed
}

// IsRollover returns true when this row was created by RolloverService
// from a previous-year approval. The decision service uses this to
// update the existing student record instead of creating a new one.
func (c *RequestChild) IsRollover() bool {
	return c.RolloverSourceChildID != nil
}

// RequestChildRepository describes the DB operations PR 5/7/8 need. PR 5
// only implements + tests Create/ListByRequestID/UpdateStatus; PR 8
// adds LinkCreatedStudent to back-link the row to the student record
// created on approval. Rollover (this PR) adds two list+update
// helpers scoped to phase + status.
type RequestChildRepository interface {
	Create(ctx context.Context, child *RequestChild) error
	FindByID(ctx context.Context, id int64) (*RequestChild, error)
	ListByRequestID(ctx context.Context, requestID int64) ([]*RequestChild, error)
	UpdateStatus(ctx context.Context, id int64, newStatus string, reason *string, reviewedBy int64) error
	LinkCreatedStudent(ctx context.Context, requestChildID, studentID int64) error

	// ListByPhaseAndStatuses returns every child row in the given
	// phase whose status is in the provided set, sorted by request
	// and sort_order. Tenant-scoped via RLS. Used by RolloverService
	// to enumerate the source children and by the deadline worker
	// to find pending_renewal / auto_renewed rows.
	ListByPhaseAndStatuses(ctx context.Context, phaseID int64, statuses []string) ([]*RequestChild, error)

	// BulkUpdateStatusByPhaseAndStatus is the deadline-worker
	// primitive: flip every row in (phaseID, currentStatus) to
	// newStatus in one UPDATE. Returns the count actually changed.
	BulkUpdateStatusByPhaseAndStatus(ctx context.Context, phaseID int64, currentStatus, newStatus string) (int, error)

	// UpdateRolloverReview is called by the admin review queue. Sets
	// status, optionally also bumps target_grade_level. reviewedBy
	// stamps the audit columns just like UpdateStatus.
	UpdateRolloverReview(ctx context.Context, id int64, newStatus string, reason *string, newGradeLevel *int16, reviewedBy int64) error
}
