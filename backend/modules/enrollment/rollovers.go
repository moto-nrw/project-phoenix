package enrollment

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Sentinel errors for the rollover flow. Mapped to HTTP status codes
// by the handlers.
var (
	ErrRolloverSourceNotFound = errors.New("rollover source phase not found")
	ErrRolloverInvalidRequest = errors.New("rollover request invalid")
	ErrRolloverReviewNotFound = errors.New("rollover review item not found")
	ErrRolloverReviewInvalid  = errors.New("rollover review decision invalid")
	// ErrRolloverDuplicateName is returned when the new phase name
	// collides with an existing phase for the same tenant. Driven by
	// the enrollment_phases_unique_name UNIQUE(tenant_id, name)
	// constraint — the admin sees a 409 with a clear message instead
	// of a raw Postgres error.
	ErrRolloverDuplicateName = errors.New("phase name already exists")
	// ErrRolloverSourceAlreadyRolled is returned when the source phase
	// has already been rolled forward into another phase. Enforced by
	// the uq_enrollment_request_children_rollover_source partial unique
	// index (migration 1.15.73) — each source child may live in at
	// most one follow-up phase. The rollover checks up front so the admin
	// sees a clear 409 instead of the raw DB error on the first
	// request_child insert.
	ErrRolloverSourceAlreadyRolled = errors.New("source phase already rolled forward")
	// ErrRolloverSourceChildTaken marks a child insert the database refused
	// because its rollover source already lives in another follow-up phase.
	ErrRolloverSourceChildTaken = errors.New("rollover source child already rolled forward")
)

// ReviewDecision values the admin can pick on a queued review row.
const (
	ReviewDecisionKeep  = "keep"
	ReviewDecisionDrop  = "drop"
	ReviewDecisionDefer = "defer"
)

// DeadlineWorkerSummary is what the deadline worker returns so callers (the
// scheduler tick log, eventually a CLI cleanup command) can report what
// changed.
type DeadlineWorkerSummary struct {
	PhasesProcessed           int
	AutoRenewedToSubmitted    int
	AutoRenewedToApproved     int // populated only on rollover_auto_approve=true phases
	PendingRenewalToWithdrawn int
	AutoApproveErrors         int
}

// CreatePhaseFromSourceRequest is the admin-facing input for the
// "create next year's phase" form. The tenant comes from context.
type CreatePhaseFromSourceRequest struct {
	SourcePhaseID     int64
	Name              string
	Kind              string
	ServiceStartDate  calendar.Date
	ServiceEndDate    calendar.Date
	EnrollmentOpenAt  *time.Time
	EnrollmentCloseAt *time.Time

	// Copy the source's form_schema_id by default (nil here means
	// "copy"); set to a different schema id to pin a new one.
	FormSchemaID *int64

	RolloverMode        string // opt_in or opt_out
	RolloverAutoApprove bool
	RolloverDeadline    time.Time
	RolloverBumpsGrade  bool

	// AdminAccountID is stamped on reviewed_by for any rolled rows
	// the admin's action causes us to write (the source children are
	// not modified). Optional — zero falls back to "system".
	AdminAccountID int64
}

// RolloverResult summarises what the rollover did so the admin UI can
// confirm "you carried 27 children forward, 2 need review".
type RolloverResult struct {
	Phase               *Phase
	SourceChildCount    int            // approved children scanned in the source phase
	RolledCount         int            // child rows created in renewal state
	ClonedOfferingCount int            // care offerings cloned into the new phase (#2249)
	ReviewCount         int            // child rows created in pending_admin_review
	ReviewByReason      map[string]int // per-reason breakdown for the UI
	RequestCount        int            // distinct parent requests created in the new phase
	EnqueuedEmails      int
	SkippedEmptyEmail   int // rows whose parent had no email (no enqueue)
}

// RolloverPreview summarises what a rollover WOULD do. Only approved
// source children are carry candidates; everything else counts as
// excluded (they are never silently carried — issue #2251's guarantee).
type RolloverPreview struct {
	CarryCandidateCount int            // approved children in the source phase
	CarriedCount        int            // would land in renewal state
	ReviewCount         int            // would land in pending_admin_review
	ReviewByReason      map[string]int // per-reason breakdown for the UI
	ExcludedCount       int            // non-approved children, never carried
	ExcludedByStatus    map[string]int // per-status breakdown for the UI
	RequestCount        int            // distinct parent requests among candidates
}

// RolloverReviewItem is one row in the admin review UI. SourceChild is
// the previous-year child the rollover tried to roll forward — used
// by the admin to see "this is Lina, last year she was in 4a, now
// she'd be in 5a which is above the cap".
type RolloverReviewItem struct {
	Child       *RequestChild
	Request     *Request
	SourceChild *RequestChild
}

// DecideReviewRequest carries the admin's decision plus the optional
// class override for the "keep" action.
type DecideReviewRequest struct {
	RequestChildID int64
	Decision       string
	// NewGradeLevel — when Decision == keep, optionally rewrites the
	// child's target_grade_level. Set this to keep a repeater at the
	// same grade as last year.
	NewGradeLevel *int16
	// AdminAccountID — stamped on reviewed_by for the audit trail.
	AdminAccountID int64
}

// Rollovers creates a new phase from a source phase, carrying every approved
// enrollment forward into the new phase under one of two parent-action modes
// (opt_in / opt_out). Children whose new grade would exceed the tenant's
// grade maximum land in an admin review queue instead of being auto-rolled.
type Rollovers interface {
	// CreatePhaseFromSource creates the new phase + all carried-forward
	// request rows + child rows + care-offering selections, and
	// enqueues one parent email per carried request. Runs inside a
	// tenant tx so the whole rollover is atomic.
	CreatePhaseFromSource(ctx context.Context, req CreatePhaseFromSourceRequest) (*RolloverResult, error)
	// PreviewPhaseFromSource is the read-only dry run of
	// CreatePhaseFromSource (#2251): it classifies every child of the
	// source phase — carried, needing admin review, or excluded — so the
	// admin sees the blast radius BEFORE executing the rollover.
	PreviewPhaseFromSource(ctx context.Context, sourcePhaseID int64, bumpsGrade bool) (*RolloverPreview, error)
	// ListReviewQueue returns every request_children row in the new
	// phase that landed in pending_admin_review, along with enough
	// context (parent + source child) for the admin UI to render.
	ListReviewQueue(ctx context.Context, phaseID int64) ([]*RolloverReviewItem, error)
	// DecideReview applies the admin's decision (keep / drop / defer)
	// to one pending_admin_review row. "Keep" optionally rewrites
	// target_grade_level to handle repeaters.
	DecideReview(ctx context.Context, req DecideReviewRequest) error
	// RunDeadlineWorker scans the current tenant's phases for any
	// whose rollover_deadline has passed, and resolves their pending
	// renewal rows: auto_renewed → submitted (admin still approves
	// through the existing flow), pending_renewal → withdrawn.
	// pending_admin_review rows are intentionally left alone — admin
	// must decide each one through the review queue.
	//
	// Idempotent: re-running after all rows have transitioned is a
	// no-op. Per-phase failures are logged but do not abort the
	// batch — the next tick retries.
	RunDeadlineWorker(ctx context.Context, asOf time.Time) (*DeadlineWorkerSummary, error)
}
