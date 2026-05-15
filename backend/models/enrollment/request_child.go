package enrollment

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// Per-child status values matching the column CHECK constraint.
const (
	ChildStatusSubmitted   = "submitted"
	ChildStatusUnderReview = "under_review"
	ChildStatusApproved    = "approved"
	ChildStatusWaitlisted  = "waitlisted"
	ChildStatusRejected    = "rejected"
	ChildStatusWithdrawn   = "withdrawn"
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

// RequestChildRepository describes the DB operations PR 5/7/8 need. PR 5
// only implements + tests Create/ListByRequestID/UpdateStatus; PR 8
// adds LinkCreatedStudent to back-link the row to the student record
// created on approval.
type RequestChildRepository interface {
	Create(ctx context.Context, child *RequestChild) error
	ListByRequestID(ctx context.Context, requestID int64) ([]*RequestChild, error)
	UpdateStatus(ctx context.Context, id int64, newStatus string, reason *string, reviewedBy int64) error
	LinkCreatedStudent(ctx context.Context, requestChildID, studentID int64) error
}
