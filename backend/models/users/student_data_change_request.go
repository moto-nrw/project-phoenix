package users

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// ErrChangeRequestNotPending means a pending-row transition lost a race or the
// row was already terminal under the caller's tenant.
var ErrChangeRequestNotPending = errors.New("users: change request is not pending")

// ErrChangeRequestNotFound means no row with the requested id exists in the
// caller's tenant.
var ErrChangeRequestNotFound = errors.New("users: change request not found")

// Change-request lifecycle states. Track A (direct edit) writes rows directly
// as auto_applied — the live record was already updated, the row is the audit
// trail. Track B (review) writes pending rows that a staff decision moves to
// approved (and applies) or rejected.
const (
	DataChangeStatusAutoApplied = "auto_applied"
	DataChangeStatusPending     = "pending"
	DataChangeStatusApproved    = "approved"
	DataChangeStatusRejected    = "rejected"
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
	base.Model `bun:"schema:users,table:student_data_change_requests"`
	base.TenantModel
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
	case DataChangeStatusAutoApplied, DataChangeStatusApproved, DataChangeStatusRejected:
		return true
	default:
		return false
	}
}

// StudentDataChangeRequestRepository is the tenant-scoped data-access contract.
// All methods MUST run inside a tenant transaction (the parent service resolves
// the child's tenant first, then wraps the call in tenant.WithTenantTx); the
// staff review queue runs under the request's tenant middleware.
type StudentDataChangeRequestRepository interface {
	Create(ctx context.Context, req *StudentDataChangeRequest) error
	// FindByID takes `any` to match the embedded generic base.Repository.
	FindByID(ctx context.Context, id any) (*StudentDataChangeRequest, error)

	// ListByStudent returns the student's change rows newest-first. When
	// statuses is non-empty the result is filtered to that set; limit <= 0
	// returns all matching rows.
	ListByStudent(ctx context.Context, studentID int64, statuses []string, limit int) ([]*StudentDataChangeRequest, error)

	// ListParentVisibleByStudent returns only child-level Track B request rows
	// that may be shown in the parent portal. It deliberately excludes Track A
	// guardian contact audit rows because those can contain another guardian's
	// private email, phone, or address values.
	ListParentVisibleByStudent(ctx context.Context, studentID int64, limit int) ([]*StudentDataChangeRequest, error)

	// ListPendingForTenant returns every pending Track B row for the current
	// tenant, newest-first — the staff review queue.
	ListPendingForTenant(ctx context.Context, filters base.RequestQueueFilters) ([]*StudentDataChangeRequest, error)

	// ListDecidedForTenant returns the tenant's decided rows (auto-applied,
	// approved, rejected) newest-decision-first via keyset pagination on
	// (updated_at, id); a zero beforeUpdatedAt returns the first page.
	ListDecidedForTenant(ctx context.Context, filters base.RequestQueueFilters) ([]*StudentDataChangeRequest, error)

	// HasPendingForField reports whether an undecided pending row already
	// exists for the same student/target/field, so the parent flow can reject
	// duplicate requests instead of stacking them.
	HasPendingForField(ctx context.Context, studentID int64, target, fieldKey string) (bool, error)

	// FindPendingByIDForUpdate locks a request row for staff decision
	// processing. It returns ErrChangeRequestNotFound when the row is missing
	// in the current tenant and ErrChangeRequestNotPending when it exists but
	// is already decided / audit-only.
	FindPendingByIDForUpdate(ctx context.Context, id int64) (*StudentDataChangeRequest, error)

	// Decide moves a pending row to approved/rejected, stamping review_reason,
	// reviewed_by, reviewed_at and (for approvals) applied_at.
	Decide(ctx context.Context, id int64, newStatus string, reason *string, reviewedBy int64, applied bool) error
}
