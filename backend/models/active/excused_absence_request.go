package active

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
)

// ErrExcusedRequestNotPending means a pending-row transition lost a race or the
// row was already terminal under the caller's tenant.
var ErrExcusedRequestNotPending = errors.New("active: excused absence request is not pending")

// ErrExcusedRequestNotFound means no row with the requested id exists in the
// caller's tenant.
var ErrExcusedRequestNotFound = errors.New("active: excused absence request not found")

// Legacy-named absence-request lifecycle states. A guardian submits a pending
// sick or excused request; a staff decision moves it to approved (and writes
// the requested status days) or rejected; the guardian can withdraw it while
// still pending.
const (
	ExcusedRequestStatusPending   = "pending"
	ExcusedRequestStatusApproved  = "approved"
	ExcusedRequestStatusRejected  = "rejected"
	ExcusedRequestStatusWithdrawn = "withdrawn"
	// ExcusedRequestStatusCareEnded closes an open request whose child left
	// the OGS before anybody decided it (#2487).
	ExcusedRequestStatusCareEnded = "care_ended"
)

// ExcusedAbsenceRequest is the legacy-named store for one parent-initiated
// absence awaiting office approval. AbsenceStatus distinguishes sick from
// excused submissions; existing rows default to excused (#1845, #2447, #2449).
type ExcusedAbsenceRequest struct {
	base.Model `bun:"schema:active,table:excused_absence_requests"`
	base.TenantModel

	StudentID      int64           `bun:"student_id,notnull" json:"student_id"`
	SubmittedBy    int64           `bun:"submitted_by,notnull" json:"submitted_by"`
	Dates          []timezone.Date `bun:"dates,type:jsonb,notnull" json:"dates"`
	Note           string          `bun:"note,notnull" json:"note"`
	AbsenceStatus  string          `bun:"absence_status,notnull,default:'excused'" json:"absence_status"`
	Status         string          `bun:"status,notnull,default:'pending'" json:"status"`
	DecisionReason *string         `bun:"decision_reason" json:"decision_reason,omitempty"`
	ReviewedBy     *int64          `bun:"reviewed_by" json:"reviewed_by,omitempty"`
	ReviewedAt     *time.Time      `bun:"reviewed_at" json:"reviewed_at,omitempty"`
	AppliedAt      *time.Time      `bun:"applied_at" json:"applied_at,omitempty"`
}

// IsTerminal reports whether the row is in a final state. Only pending rows
// accept a staff decision or a guardian withdrawal.
func (e *ExcusedAbsenceRequest) IsTerminal() bool {
	switch e.Status {
	case ExcusedRequestStatusApproved, ExcusedRequestStatusRejected, ExcusedRequestStatusWithdrawn, ExcusedRequestStatusCareEnded:
		return true
	default:
		return false
	}
}

// ExcusedAbsenceRequestRepository is the tenant-scoped data-access contract.
// All methods MUST run inside a tenant transaction (the parent service resolves
// the child's tenant first, then wraps the call in tenant.WithTenantTx; the
// staff review queue runs under the request's tenant middleware).
type ExcusedAbsenceRequestRepository interface {
	Create(ctx context.Context, req *ExcusedAbsenceRequest) error

	// LockStudentRequests takes a per-student advisory lock (released at the end
	// of the ambient transaction) so concurrent CreateRequest calls for the same
	// child serialize. Without it the idempotent-retry / overlap checks in the
	// service (read pending, then insert) would race two rows through.
	LockStudentRequests(ctx context.Context, studentID int64) error
	// FindByID takes `any` to match the embedded generic base.Repository.
	FindByID(ctx context.Context, id any) (*ExcusedAbsenceRequest, error)

	// ListPendingForStudent returns the student's open requests, newest-first
	// (there can be several — no one-pending-per-student constraint).
	ListPendingForStudent(ctx context.Context, studentID int64) ([]*ExcusedAbsenceRequest, error)

	// ListRecentForStudent returns the student's requests decided (approved,
	// rejected or withdrawn) on or after `since` — filtered by decision time, not
	// created_at — newest-first, regardless of status. Filtering on the decision
	// time is what surfaces a request that sat pending past the window and was
	// only just rejected; created_at would drop it the moment it left the pending
	// list. See the repository implementation for the COALESCE detail.
	ListRecentForStudent(ctx context.Context, studentID int64, since time.Time) ([]*ExcusedAbsenceRequest, error)

	// ListPendingForTenant returns every pending request for the current tenant,
	// newest-first — the staff review queue and the inline planning surface.
	ListPendingForTenant(ctx context.Context, filters base.RequestQueueFilters) ([]*ExcusedAbsenceRequest, error)

	// ListDecidedForTenant returns the tenant's decided rows (approved,
	// rejected, withdrawn) newest-decision-first via keyset pagination on
	// (updated_at, id); a zero beforeUpdatedAt returns the first page.
	ListDecidedForTenant(ctx context.Context, filters base.RequestQueueFilters) ([]*ExcusedAbsenceRequest, error)

	// FindPendingByIDForUpdate locks a request row for decision processing. It
	// returns ErrExcusedRequestNotFound when the row is missing in the current
	// tenant and ErrExcusedRequestNotPending when it exists but is already
	// decided or withdrawn.
	FindPendingByIDForUpdate(ctx context.Context, id int64) (*ExcusedAbsenceRequest, error)

	// FindByIDForUpdate locks a request row for the current tenant regardless of
	// status, returning ErrExcusedRequestNotFound when it is missing. The
	// guardian withdraw path uses it to verify ownership BEFORE distinguishing a
	// terminal status, so a stranger cannot probe a foreign child's decided
	// request id via a not-pending 409.
	FindByIDForUpdate(ctx context.Context, id int64) (*ExcusedAbsenceRequest, error)

	// Decide moves a pending row to approved/rejected/withdrawn, stamping
	// decision_reason, reviewed_by, reviewed_at and (for approvals) applied_at.
	// Withdrawals carry no reviewer stamp.
	Decide(ctx context.Context, id int64, newStatus string, reason *string, reviewedBy *int64, applied bool) error
}
