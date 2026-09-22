package absencerecords

import (
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/careplan/excusedrequests"
)

// ErrExcusedRequestNotPending means a pending-row transition lost a race or the
// row was already terminal under the caller's tenant.
var ErrExcusedRequestNotPending = errors.New("active: excused absence request is not pending")

// ErrExcusedRequestNotFound means no row with the requested id exists in the
// caller's tenant.
var ErrExcusedRequestNotFound = errors.New("active: excused absence request not found")

// ErrExcusedRequestNotDecided means a correction was attempted on a row that
// carries no decision to correct — still pending, or already closed some other
// way (withdrawn, care ended, marked done).
var ErrExcusedRequestNotDecided = errors.New("active: excused absence request is not decided")

// Legacy-named absence-request lifecycle states. A guardian submits a pending
// sick or excused request; a staff decision moves it to approved (and writes
// the requested status days) or rejected; the guardian can withdraw it while
// still pending.
const (
	ExcusedRequestStatusPending   = excusedrequests.StatusPending
	ExcusedRequestStatusApproved  = excusedrequests.StatusApproved
	ExcusedRequestStatusRejected  = excusedrequests.StatusRejected
	ExcusedRequestStatusWithdrawn = excusedrequests.StatusWithdrawn
	// ExcusedRequestStatusDone closes a request that only covered days already
	// gone: nothing to apply, and "abgelehnt" would misstate what happened.
	ExcusedRequestStatusDone = excusedrequests.StatusDone
	// ExcusedRequestStatusCareEnded closes an open request whose child left
	// the OGS before anybody decided it (#2487).
	ExcusedRequestStatusCareEnded = excusedrequests.StatusCareEnded
)

// ExcusedAbsenceRequest is one row of active.excused_absence_requests: a
// parent-initiated absence awaiting office approval. AbsenceStatus
// distinguishes sick from excused submissions; existing rows default to
// excused (#1845, #2447, #2449).
type ExcusedAbsenceRequest struct {
	ID             int64           `json:"id"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
	TenantID       int64           `json:"tenant_id"`
	StudentID      int64           `json:"student_id"`
	SubmittedBy    int64           `json:"submitted_by"`
	Dates          []timezone.Date `json:"dates"`
	Note           string          `json:"note"`
	AbsenceStatus  string          `json:"absence_status"`
	Status         string          `json:"status"`
	DecisionReason *string         `json:"decision_reason,omitempty"`
	ReviewedBy     *int64          `json:"reviewed_by,omitempty"`
	ReviewedAt     *time.Time      `json:"reviewed_at,omitempty"`
	AppliedAt      *time.Time      `json:"applied_at,omitempty"`
}
