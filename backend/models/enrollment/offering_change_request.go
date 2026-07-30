package enrollment

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
)

// Offering-change-request lifecycle. A guardian submits a pending row; staff
// approve it (which applies the switch on its effective date) or reject it; the
// submitting guardian may withdraw it while it is still pending.
const (
	OfferingChangeStatusPending   = "pending"
	OfferingChangeStatusApproved  = "approved"
	OfferingChangeStatusRejected  = "rejected"
	OfferingChangeStatusWithdrawn = "withdrawn"
)

var (
	// ErrOfferingChangeNotFound means no such row exists in the caller's tenant.
	ErrOfferingChangeNotFound = errors.New("enrollment: offering change request not found")
	// ErrOfferingChangeNotPending means the row was already decided or
	// withdrawn — a lost race or a double submit.
	ErrOfferingChangeNotPending = errors.New("enrollment: offering change request is not pending")
	// ErrOfferingChangeAlreadyPending means the child already has an open
	// request (enforced by a partial unique index as well).
	ErrOfferingChangeAlreadyPending = errors.New("enrollment: offering change request already pending")
)

// OfferingChangeRequest is one parent-initiated change to the care offerings a
// child is booked into, to take effect on EffectiveFrom. The payload carries
// the complete desired selection, not a delta: staff decide one coherent
// booking, and re-validating a full selection against capacity and the phase
// rules at approval time is the same operation the enrollment form performs.
type OfferingChangeRequest struct {
	base.Model `bun:"schema:enrollment,table:offering_change_requests"`
	base.TenantModel

	StudentID int64 `bun:"student_id,notnull" json:"student_id"`
	// RequestChildID anchors the apply: the enrollment rows the switch replaces
	// are exactly the ones materialized from this approved request child.
	RequestChildID int64 `bun:"request_child_id,notnull" json:"request_child_id"`
	SubmittedBy    int64 `bun:"submitted_by,notnull" json:"submitted_by"`
	// Payload is {"offerings":[{"offering_id":<int>,"selected_days":["mon",...]}]}.
	Payload        map[string]any `bun:"payload,type:jsonb,notnull" json:"payload"`
	EffectiveFrom  timezone.Date  `bun:"effective_from,notnull,type:date" json:"effective_from"`
	ParentNote     *string        `bun:"parent_note" json:"parent_note,omitempty"`
	Status         string         `bun:"status,notnull,default:'pending'" json:"status"`
	DecisionReason *string        `bun:"decision_reason" json:"decision_reason,omitempty"`
	ReviewedBy     *int64         `bun:"reviewed_by" json:"reviewed_by,omitempty"`
	ReviewedAt     *time.Time     `bun:"reviewed_at" json:"reviewed_at,omitempty"`
	AppliedAt      *time.Time     `bun:"applied_at" json:"applied_at,omitempty"`
}

// IsTerminal reports whether the row can still be decided or withdrawn.
func (r *OfferingChangeRequest) IsTerminal() bool {
	switch r.Status {
	case OfferingChangeStatusApproved, OfferingChangeStatusRejected, OfferingChangeStatusWithdrawn:
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
	ListPendingForTenant(ctx context.Context) ([]*OfferingChangeRequest, error)

	// FindByIDForUpdate locks a row by id regardless of status, so an ownership
	// check can run on the locked row before the pending check (a foreign id
	// stays reported as not-found).
	FindByIDForUpdate(ctx context.Context, id int64) (*OfferingChangeRequest, error)

	// UpdateEffectiveFrom records the date an approved request was actually
	// applied on when a delayed review moves it forward to today.
	UpdateEffectiveFrom(ctx context.Context, id int64, effectiveFrom timezone.Date) error

	// Decide moves a pending row to a terminal status, guarding the transition
	// in SQL so two reviewers cannot both decide it. Returns
	// ErrOfferingChangeNotPending when the row was no longer pending. Guardian
	// withdrawals pass reviewedBy = nil; applied is set only when the switch was
	// actually written.
	Decide(ctx context.Context, id int64, status string, reason *string, reviewedBy *int64, applied bool) error
}
