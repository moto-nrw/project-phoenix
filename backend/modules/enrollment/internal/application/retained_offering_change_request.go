package application

import (
	"context"
	"errors"
	"time"

	offeringContract "github.com/moto-nrw/project-phoenix/modules/enrollment/offeringcontract"
)

var (
	// ErrOfferingChangeNotFound means no such row exists in the caller's tenant.
	ErrOfferingChangeNotFound = errors.New("enrollment: offering change request not found")
	// ErrOfferingChangeNotPending means the row was already decided or
	// withdrawn — a lost race or a double submit.
	ErrOfferingChangeNotPending = errors.New("enrollment: offering change request is not pending")
)

// OfferingChangeRequest is one parent-initiated change to the care offerings a
// child is booked into, to take effect on EffectiveFrom. The payload carries
// the complete desired selection, not a delta: staff decide one coherent
// booking, and re-validating a full selection against capacity and the phase
// rules at approval time is the same operation the enrollment form performs.
type OfferingChangeRequest struct {
	ID        int64     `json:"id"`
	TenantID  int64     `json:"tenant_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	StudentID int64 `json:"student_id"`
	// RequestChildID anchors the apply: the enrollment rows the switch replaces
	// are exactly the ones materialized from this approved request child.
	RequestChildID int64 `json:"request_child_id"`
	SubmittedBy    int64 `json:"submitted_by"`
	// CompleteWithdrawalConfirmed records the guardian's explicit confirmation
	// when the fully materialized request removes every care day.
	CompleteWithdrawalConfirmed bool       `json:"complete_withdrawal_confirmed"`
	WithdrawalConfirmedBy       *int64     `json:"withdrawal_confirmed_by,omitempty"`
	WithdrawalConfirmedAt       *time.Time `json:"withdrawal_confirmed_at,omitempty"`
	// ApprovedCompleteWithdrawal records the result staff actually applied.
	// It is deliberately separate from the guardian's submission confirmation:
	// catalog and rule changes can leave care days in place at review time.
	ApprovedCompleteWithdrawal bool `json:"approved_complete_withdrawal"`
	// Payload is {"offerings":[{"offering_id":<int>,"selected_days":["mon",...]}]}.
	Payload        map[string]any                      `json:"payload"`
	EffectiveFrom  offeringContract.OfferingChangeDate `json:"effective_from"`
	ParentNote     *string                             `json:"parent_note,omitempty"`
	Status         string                              `json:"status"`
	DecisionReason *string                             `json:"decision_reason,omitempty"`
	// DecisionSnapshot freezes the "current → requested" diff (including which
	// lines a Mitbuchungs-Regel added and any per-request overrides) at the
	// moment the request was decided. Recap views read only this snapshot, so a
	// later rule change cannot rewrite history (ADR 0002). Nil on rows decided
	// before the column existed; recap readers then fall back to the stored
	// payload (the guardian's own selection), exactly as before the column.
	DecisionSnapshot *offeringContract.OfferingChangeDecisionSnapshot `json:"decision_snapshot,omitempty"`
	ReviewedBy       *int64                                           `json:"reviewed_by,omitempty"`
	ReviewedAt       *time.Time                                       `json:"reviewed_at,omitempty"`
	AppliedAt        *time.Time                                       `json:"applied_at,omitempty"`
}

// IsTerminal reports whether the row can still be decided or withdrawn.
func (r *OfferingChangeRequest) IsTerminal() bool {
	switch r.Status {
	case offeringContract.OfferingChangeStatusApproved, offeringContract.OfferingChangeStatusRejected, offeringContract.OfferingChangeStatusWithdrawn,
		offeringContract.OfferingChangeStatusCareEnded, offeringContract.OfferingChangeStatusDone:
		return true
	default:
		return false
	}
}

// OfferingChangeRequestSelection is one desired offering inside the payload.
type OfferingChangeRequestSelection struct {
	OfferingID   int64    `json:"offering_id"`
	SelectedDays []string `json:"selected_days,omitempty"`
}

// OfferingChangeRequestRepository is the tenant-scoped data-access contract.
// Every method must run inside a tenant transaction: the parents portal
// resolves the child's tenant first, the staff review queue runs under tenant
// middleware.
type OfferingChangeRequestRepository interface {
	Create(ctx context.Context, row *OfferingChangeRequest) error

	// FindByID takes `any` to match the embedded generic base.Repository.
	FindByID(ctx context.Context, id any) (*OfferingChangeRequest, error)

	// GetPendingForStudent returns the child's open request, or nil when there
	// is none. The partial unique index guarantees at most one.
	GetPendingForStudent(ctx context.Context, studentID int64) (*OfferingChangeRequest, error)

	// ListByStudent returns the child's requests with the newest decision first.
	ListByStudent(ctx context.Context, studentID int64) ([]*OfferingChangeRequest, error)

	// ListPendingForTenant backs the staff review queue, ordered by effective
	// date so the switch that lands soonest is decided first.
	ListPendingForTenant(ctx context.Context, filters OfferingChangeQueueFilters) ([]*OfferingChangeRequest, error)

	// ListDecidedForTenant returns the tenant's decided rows (approved,
	// rejected, withdrawn) newest-decision-first via keyset pagination on
	// (updated_at, id); a zero beforeUpdatedAt returns the first page.
	ListDecidedForTenant(ctx context.Context, filters OfferingChangeQueueFilters) ([]*OfferingChangeRequest, error)

	// FindByIDForUpdate locks a row by id regardless of status, so an ownership
	// check can run on the locked row before the pending check (a foreign id
	// stays reported as not-found).
	FindByIDForUpdate(ctx context.Context, id int64) (*OfferingChangeRequest, error)

	// UpdateEffectiveFrom records the date an approved request was actually
	// applied on when a delayed review moves it forward to today.
	UpdateEffectiveFrom(ctx context.Context, id int64, effectiveFrom offeringContract.OfferingChangeDate) error
	UpdateApprovedCompleteWithdrawal(ctx context.Context, id int64, complete bool) error

	// UpdatePending rewrites a still-pending row's selections, effective date
	// and parent note — the guardian edit path (#2267). It refuses a decided
	// row (ErrOfferingChangeNotPending) and bumps updated_at.
	UpdatePending(ctx context.Context, id int64, payload map[string]any, effectiveFrom offeringContract.OfferingChangeDate, note *string) error

	// Decide moves a pending row to a terminal status, guarding the transition
	// in SQL so two reviewers cannot both decide it. Returns
	// ErrOfferingChangeNotPending when the row was no longer pending. Guardian
	// withdrawals pass reviewedBy = nil; applied is set only when the switch was
	// actually written.
	Decide(ctx context.Context, id int64, status string, reason *string, reviewedBy *int64, applied bool) error

	// UpdateDecisionSnapshot stores the frozen review diff on a decided row.
	UpdateDecisionSnapshot(ctx context.Context, id int64, snapshot *offeringContract.OfferingChangeDecisionSnapshot) error
}
