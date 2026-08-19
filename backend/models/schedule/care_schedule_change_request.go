package schedule

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// ErrCareRequestNotPending means a pending-row transition lost a race or the
// row was already terminal under the caller's tenant.
var ErrCareRequestNotPending = errors.New("schedule: care schedule change request is not pending")

// ErrCareRequestNotFound means no row with the requested id exists in the
// caller's tenant.
var ErrCareRequestNotFound = errors.New("schedule: care schedule change request not found")

// Care-schedule change-request lifecycle states. A guardian submits a pending
// request; a staff decision moves it to approved (and applies the weekly plan)
// or rejected; the guardian can withdraw it while still pending.
const (
	CareRequestStatusPending   = "pending"
	CareRequestStatusApproved  = "approved"
	CareRequestStatusRejected  = "rejected"
	CareRequestStatusWithdrawn = "withdrawn"
)

const (
	CareRequestKindWeeklySchedule = "weekly_schedule"
	CareRequestKindPickupChange   = "pickup_change"
)

// CareScheduleChangeRequest is one parent-initiated change to a child's
// recurring weekly care plan (departure mode + arrival/pickup times per
// weekday). One row per request carrying the whole canonical payload
// {"weekdays":[{"weekday","mode","arrival","pickup"}]} — the request is
// decided atomically, and apply merges only the filled aspects onto the live
// plan. At most one pending request exists per student (partial unique index).
type CareScheduleChangeRequest struct {
	base.Model `bun:"schema:schedule,table:care_schedule_change_requests"`
	base.TenantModel

	StudentID      int64          `bun:"student_id,notnull" json:"student_id"`
	SubmittedBy    int64          `bun:"submitted_by,notnull" json:"submitted_by"`
	RequestKind    string         `bun:"request_kind,notnull,default:'weekly_schedule'" json:"request_kind"`
	Payload        map[string]any `bun:"payload,type:jsonb,notnull" json:"payload"`
	Status         string         `bun:"status,notnull,default:'pending'" json:"status"`
	DecisionReason *string        `bun:"decision_reason" json:"decision_reason,omitempty"`
	ReviewedBy     *int64         `bun:"reviewed_by" json:"reviewed_by,omitempty"`
	ReviewedAt     *time.Time     `bun:"reviewed_at" json:"reviewed_at,omitempty"`
	AppliedAt      *time.Time     `bun:"applied_at" json:"applied_at,omitempty"`
}

// IsTerminal reports whether the row is in a final state. Only pending rows
// accept a staff decision or a guardian withdrawal.
func (c *CareScheduleChangeRequest) IsTerminal() bool {
	switch c.Status {
	case CareRequestStatusApproved, CareRequestStatusRejected, CareRequestStatusWithdrawn:
		return true
	default:
		return false
	}
}

// CareScheduleChangeRequestRepository is the tenant-scoped data-access
// contract. All methods MUST run inside a tenant transaction (the parent
// service resolves the child's tenant first, then wraps the call in
// tenant.WithTenantTx); the staff review queue runs under the request's
// tenant middleware.
type CareScheduleChangeRequestRepository interface {
	Create(ctx context.Context, req *CareScheduleChangeRequest) error
	// FindByID takes `any` to match the embedded generic base.Repository.
	FindByID(ctx context.Context, id any) (*CareScheduleChangeRequest, error)

	// GetPendingForStudent returns the student's open request, or nil when
	// none exists (at most one can exist — partial unique index).
	GetPendingForStudent(ctx context.Context, studentID int64) (*CareScheduleChangeRequest, error)

	// ListPendingForTenant returns every pending request for the current
	// tenant, newest-first — the staff review queue.
	ListPendingForTenant(ctx context.Context) ([]*CareScheduleChangeRequest, error)
	GetPendingForStudentAndKind(ctx context.Context, studentID int64, requestKind string) (*CareScheduleChangeRequest, error)
	ListPendingForTenantAndKind(ctx context.Context, requestKind string) ([]*CareScheduleChangeRequest, error)
	ListRecentForStudentAndKind(ctx context.Context, studentID int64, requestKind string, since time.Time) ([]*CareScheduleChangeRequest, error)

	// ListDecidedForTenant returns the tenant's decided weekly-schedule rows
	// (approved, rejected, withdrawn) newest-decision-first via keyset
	// pagination on (updated_at, id); a zero beforeUpdatedAt returns the first
	// page.
	ListDecidedForTenant(ctx context.Context, beforeUpdatedAt time.Time, beforeID int64, limit int) ([]*CareScheduleChangeRequest, error)

	// FindPendingByIDForUpdate locks a request row for decision processing.
	// It returns ErrCareRequestNotFound when the row is missing in the
	// current tenant and ErrCareRequestNotPending when it exists but is
	// already decided or withdrawn.
	FindPendingByIDForUpdate(ctx context.Context, id int64) (*CareScheduleChangeRequest, error)

	// FindByIDForUpdate locks a request row for the current tenant regardless
	// of status, returning ErrCareRequestNotFound when it is missing. The
	// guardian withdraw path uses it to verify ownership BEFORE distinguishing
	// a terminal status, so a stranger cannot probe a foreign child's decided
	// request id via a not-pending 409 (FindPendingByIDForUpdate leaks that the
	// row exists before ownership is checked).
	FindByIDForUpdate(ctx context.Context, id int64) (*CareScheduleChangeRequest, error)

	// Decide moves a pending row to approved/rejected/withdrawn, stamping
	// decision_reason, reviewed_by, reviewed_at and (for approvals)
	// applied_at. Withdrawals carry no reviewer stamp.
	Decide(ctx context.Context, id int64, newStatus string, reason *string, reviewedBy *int64, applied bool) error
}
