package enrollment

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Sentinels of the admin decision flow. The HTTP adapters classify them; the
// messages are part of the wire text of a refused request.
var (
	ErrDecisionStudentNotFound = errors.New("student not found")
	ErrDecisionInvalidStatus   = errors.New("invalid decision status")
	ErrDecisionAlreadyTerminal = errors.New("child is already in a terminal status")
	// ErrDecisionInvalidData marks an approval that failed because the
	// parent-supplied request data (e.g. guardian phone) doesn't pass the
	// student/person validators. Mapped to 400, not 500 — submit/edit now
	// validate up front, so this is defense-in-depth for legacy rows.
	ErrDecisionInvalidData = errors.New("enrollment request data is invalid")
	// ErrGuardianAccountMismatch marks an approval whose authenticated
	// submitter account (request.guardian_account_id) conflicts with the
	// guardian profile the request's email resolves to: the email already
	// belongs to a DIFFERENT account's guardian profile at this school.
	// Approving would link the child to that other account, so we fail closed
	// rather than silently misattribute it (#1663). Mapped to 400.
	ErrGuardianAccountMismatch = errors.New("enrollment guardian email belongs to a different account")
	ErrWaitlistDisabled        = errors.New("waitlist decisions are disabled for this tenant")
	// ErrExportTooLarge guards the phase export against assembling an
	// unbounded payload in memory. At OGS scale a phase holds hundreds of
	// requests (a few MB); this cap only trips on a pathological phase,
	// turning "theoretically unbounded" into an explicit, mapped 400 (the
	// handler treats an over-cap phase as a client-side limit, not a
	// server fault).
	ErrExportTooLarge = errors.New("phase has too many registrations for a single export")
)

// Sentinel errors for the admin restore flow (#2157). Mapped to HTTP
// statuses by the admin handler.
var (
	ErrRestoreNothingWithdrawn = errors.New("request has no withdrawn children to restore")
	ErrRestorePhaseInactive    = errors.New("enrollment phase is not active")
	ErrRestoreDuplicateActive  = errors.New("an active enrollment already exists for a child of this request")
)

// DecisionStatus enumerates the per-child decisions an admin can apply.
// Mirrors the request_children.status CHECK constraint subset that
// admins are allowed to write (parent-initiated 'withdrawn' goes
// through a different path).
type DecisionStatus string

const (
	DecisionApproved    DecisionStatus = ChildStatusApproved
	DecisionWaitlisted  DecisionStatus = ChildStatusWaitlisted
	DecisionRejected    DecisionStatus = ChildStatusRejected
	DecisionUnderReview DecisionStatus = ChildStatusUnderReview
)

// DecideInput carries the per-child decision the admin makes.
type DecideInput struct {
	RequestID                  int64
	ChildID                    int64
	Status                     DecisionStatus
	Reason                     string // optional; surfaced to parent only when phase.show_status_reason_to_parent
	ReviewedBy                 int64  // admin's auth account id
	SuppressParentEmail        bool
	SuppressGuardianInvitation bool
}

// DecideOutcome is what the admin handler gets back from Decide. It
// carries the refreshed child plus an optional follow-up instruction asking
// the handler to issue a guardian invitation post-commit (after the tenant
// tx the handler owns completes).
//
// We surface the invitation as a side-effect rather than firing it from
// inside the service so:
//   - the invitation flow's own DB writes happen only if the approval
//     tx committed cleanly
//   - the handler can apply best-effort error handling without rolling
//     back the approval
type DecideOutcome struct {
	Child         *RequestChild
	PendingInvite *PendingGuardianInvite
}

// PendingGuardianInvite is the post-commit hook for fresh approvals
// where the guardian doesn't yet have a portal account. The handler is
// expected to create the guardian invitation with these values once the
// tenant tx commits.
type PendingGuardianInvite struct {
	GuardianProfileID int64
	CreatedBy         int64 // admin auth account id (for audit)
}

// DecisionRequestFilters narrows the admin list. Zero-value fields are
// ignored.
type DecisionRequestFilters struct {
	PhaseID     int64
	ChildStatus string // matches when ANY child carries this status
}

// DecisionSummary is the admin-list shape: one row per request with its
// children so the admin can scan the queue without expanding every detail
// page.
type DecisionSummary struct {
	Request    *Request
	Phase      *Phase
	Children   []*RequestChild
	Guardians  []*RequestGuardian
	LateInvite *LateInvite
}

// ChildOfferingRow is one care-offering selection for a child, as
// surfaced by ListChildOfferings. SelectedDays mirrors the DB column
// — nil when the offering runs in admin-fixed mode, non-nil only when
// the parent picked specific days.
//
// The attribute fields (price, lunch, holiday care) and the validity
// interval mirror what the parent portal already shows for the same
// booking (#2185): staff answering a guardian's question must see the
// same facts, or they contradict the family's own app.
type ChildOfferingRow struct {
	OfferingID            int64
	OfferingName          string
	DaysOfWeekMode        string
	SelectedDays          []string
	ManualSelectedDays    []string
	AutomaticSelectedDays []string
	AvailableDays         []string
	IncludesLunch         bool
	IncludesHolidayCare   bool
	// PriceCents is nil when the school configured no price.
	PriceCents *int
	// ValidFrom is nil for a booking effective since the start of the
	// care period; ValidUntil is exclusive, mirroring the stored interval.
	ValidFrom  *calendar.Date
	ValidUntil *calendar.Date
	// StartsLater marks a booking that has not taken effect yet — an
	// approved change scheduled for a future date. Those rows are
	// informational: they describe what will be booked, not what is.
	StartsLater bool
}

// ChildOfferingSet separates the two questions a reader asks about a
// child's bookings. Two slices rather than one slice plus a flag: a
// forgotten flag would silently route a booking into the wrong group,
// and the wrong group is either a deletion (the editor seeds empty) or
// an early application (a future change is pre-checked). A row cannot
// be in no slice and cannot be in both.
type ChildOfferingSet struct {
	// Current is the selection on file — exactly what UpdateChildOfferings
	// replaces. Seed a correction editor from this and nothing else.
	Current []ChildOfferingRow
	// Upcoming only takes effect on a later date. Display only.
	Upcoming []ChildOfferingRow
}

// OfferingAdjustmentSelection is one offering an admin books for a child.
type OfferingAdjustmentSelection struct {
	OfferingID   int64
	SelectedDays []string
}

// UpdateChildOfferingsInput is an admin's correction of a child's bookings.
type UpdateChildOfferingsInput struct {
	RequestID      int64
	ChildID        int64
	Offerings      []OfferingAdjustmentSelection
	Reason         string
	ActorAccountID int64
	ActorRole      string
	// EffectiveFrom turns the adjustment into a dated switch instead of a
	// retroactive correction: enrollment rows that already started keep their
	// history and are capped at this date, and the new selection starts here.
	// Nil keeps the correction semantics (replace the whole phase window),
	// which is what an admin fixing a typo in the original submission wants.
	EffectiveFrom *calendar.Date
	// ExcludedAutoAddTargetIDs switches off the Mitbuchungs-Regel for these
	// target offerings in this one adjustment (#2370). The shared applier
	// validates them against the same materialization it persists.
	ExcludedAutoAddTargetIDs map[int64]bool
	// CompleteWithdrawalConfirmed is the explicit confirmation shown only when
	// an authoritative adjustment removes the child's final care day.
	CompleteWithdrawalConfirmed bool
}

// OfferingAdjustmentRecord is one entry of a child's offering adjustment
// trail (audit.enrollment_offering_adjustments).
type OfferingAdjustmentRecord struct {
	ID                          int64           `json:"id"`
	TenantID                    int64           `json:"tenant_id"`
	RequestID                   int64           `json:"request_id"`
	RequestChildID              int64           `json:"request_child_id"`
	StudentID                   int64           `json:"student_id"`
	ActorAccountID              int64           `json:"actor_account_id"`
	ActorRole                   string          `json:"actor_role"`
	ActorNameSnapshot           *string         `json:"actor_name_snapshot,omitempty"`
	ActorEmailSnapshot          *string         `json:"actor_email_snapshot,omitempty"`
	Reason                      string          `json:"reason"`
	Source                      string          `json:"source"`
	Before                      json.RawMessage `json:"before"`
	After                       json.RawMessage `json:"after"`
	CompleteWithdrawalConfirmed bool            `json:"complete_withdrawal_confirmed"`
	ChangedAt                   time.Time       `json:"changed_at"`
}

// RestoreOutcome is what the admin handler gets back from RestoreWithdrawn.
// WaitlistedChildIDs is the subset of RestoredChildIDs that came back as
// waitlisted instead of submitted because an offering is meanwhile full.
type RestoreOutcome struct {
	RestoredChildIDs   []int64
	WaitlistedChildIDs []int64
}

// Decisions backs the admin review: the request list and detail, the per-child
// decision with the downstream record creation of an approval, the restore of
// a withdrawn request, the booking correction and the audited exports. Every
// write runs in the caller's tenant transaction, like the approval's
// materialization and pickup sync in Care Plan.
type Decisions interface {
	// DecisionRequests lists the requests of the admin queue.
	DecisionRequests(ctx context.Context, filters DecisionRequestFilters) ([]*DecisionSummary, error)
	// StudentDecisionRequests lists the requests whose children created the
	// student, keeping only those children.
	StudentDecisionRequests(ctx context.Context, studentID int64) ([]*DecisionSummary, error)
	// DecisionRequest loads one request with its children, guardians and the
	// late invite it was submitted through.
	DecisionRequest(ctx context.Context, requestID int64) (*DecisionSummary, error)
	// Decide applies one child's decision; an approval creates the student,
	// guardian and booking records in the same transaction.
	Decide(ctx context.Context, input DecideInput) (*DecideOutcome, error)
	// RestoreWithdrawn undoes a parent-initiated withdraw: withdrawn
	// children go back to submitted, review metadata and withdrawn_at are
	// cleared, and an append-only audit row records the restore (#2157).
	// Guards: phase must be active, and the submit-time duplicate checks
	// re-run under the submit advisory locks. Joins the handler-provided
	// tenant transaction, like Decide.
	RestoreWithdrawn(ctx context.Context, requestID, restoredBy int64) (*RestoreOutcome, error)
	// UpdateChildOfferings corrects a child's bookings through Care Plan.
	UpdateChildOfferings(ctx context.Context, input UpdateChildOfferingsInput) (*RequestChild, error)
	// ListOfferingAdjustments lists the audit trail of one child's offering
	// adjustments for the admin request detail.
	ListOfferingAdjustments(ctx context.Context, requestID, requestChildID int64) ([]*OfferingAdjustmentRecord, error)
	// ListChildOfferings returns the care offering bookings for every child
	// under requestID, joined to the offering's name and attributes so the
	// admin detail page can render labels without a second per-offering
	// fetch. Map key is request_child_id.
	ListChildOfferings(ctx context.Context, requestID int64) (map[int64]ChildOfferingSet, error)
	// ExportPhase loads every request of a phase with its fully-resolved
	// children + offerings + form schema(s) in a fixed handful of
	// queries (N+1-free) AND records the GDPR access-log row in the same
	// call, so no caller can disclose the phase's PII without leaving an
	// audit trail. If the audit write fails the whole call fails (no trail,
	// no disclosure). Must run inside the request's tenant transaction so
	// both the reads and the audit row land under the correct tenant (RLS).
	//
	// childStatusFilter, when non-empty, keeps only children whose own
	// status equals it (requests with no matching child are dropped) —
	// mirroring the admin list's per-child status dropdown. Empty means
	// "all". The audit row's counts reflect the filtered (disclosed) set.
	ExportPhase(ctx context.Context, phaseID, actorAccountID int64, actorRole, format, childStatusFilter string) (*PhaseExport, error)
	// ExportStudent exports the requests of one student's kartei tab and
	// records the GDPR access-log row.
	ExportStudent(ctx context.Context, studentID, actorAccountID int64, actorRole, format string) (*StudentEnrollmentExport, error)
	// RecordPhaseExportAudit appends one append-only row to
	// audit.data_access_log recording that an admin exported the full
	// PII of a phase. The caller MUST refuse the export if this returns
	// an error (no trail, no disclosure).
	RecordPhaseExportAudit(ctx context.Context, actorAccountID int64, actorRole string, phase *Phase, format, statusFilter string, requestCount, childCount int) error
}

// ApprovedChildSync carries a confirmed change request onto the student an
// approval created.
type ApprovedChildSync struct {
	RequestID           int64
	ChildID             int64
	ActorAccountID      int64
	ReplaceTargetedData bool
	// PreviousSnapshot is the JSON snapshot of the request before the
	// change, as the change request stored it.
	PreviousSnapshot         json.RawMessage
	PreviousRequestGuardians []*RequestGuardian
}

// ApprovedChildChanges is the side of the decision flow the change requests
// apply their approvals through, in the caller's tenant transaction.
type ApprovedChildChanges interface {
	// ApplyChangeRequestOfferings applies an offering proposal whose
	// capability was validated and pinned when the parent created the change
	// request.
	ApplyChangeRequestOfferings(ctx context.Context, input UpdateChildOfferingsInput) (*RequestChild, error)
	// SyncApprovedChildData carries the confirmed request and child data onto
	// the student the approval created.
	SyncApprovedChildData(ctx context.Context, input ApprovedChildSync) (*RequestChild, error)
}
