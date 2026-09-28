package careplan

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan/offeringrequests"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Post-enrollment offering changes (#1665, #3561): a guardian requests a
// different set of care offerings or AGs for an enrolled child, effective on
// a chosen date; staff decide it on the central "Änderungsanfragen" page, and
// an approval applies the switch through the dated offering adjustment of the
// booking materialization (#3560). Every method runs inside the tenant
// transaction its caller opened.

// OfferingChangeRequestType is the request_type stamped on the chat pills,
// so a localized client can tell this apart from care_schedule and
// master_data.
const OfferingChangeRequestType = "care_offering"

// ParentRequestTypeOffering is the request type of an offering change in the
// parent-request ledger and share records.
const ParentRequestTypeOffering = "offering"

// OfferingChangeLeadDaysDefault mirrors the registry default of the notice
// period.
const OfferingChangeLeadDaysDefault = 14

// EarliestOfferingChangeDate is the first date a requested switch may take
// effect: tomorrow at the earliest, plus the school's lead time.
func EarliestOfferingChangeDate(today calendar.Date, leadDays int) calendar.Date {
	if leadDays < 0 {
		leadDays = 0
	}
	return today.AddDays(1 + leadDays)
}

// OfferingChangeSelection is one desired offering with its chosen days.
type OfferingChangeSelection = offeringrequests.Selection

// OfferingChangeDiffEntry is one "current → requested" line. State and day
// keys stay stable so the parents portal can localize them.
type OfferingChangeDiffEntry = offeringrequests.DiffEntry

// OfferingChangeRequestedItem is one offering of a decided request, resolved
// to its name and the requested days at read time.
type OfferingChangeRequestedItem = offeringrequests.RequestedItem

// OfferingChangeCatalogItem is one offering a guardian can pick, with the
// booking state and the capacity signal the modal shows.
type OfferingChangeCatalogItem struct {
	OfferingID      int64
	Name            string
	Description     string
	DaysOfWeekMode  string
	AvailableDays   []string
	SelectionGroup  string
	SelectionRule   string
	IsRequired      bool
	PriceCents      *int
	IncludesLunch   bool
	IncludesHoliday bool
	// CountsAsCare and PickupTimes let staff-side editors compare a complete
	// proposed weekly plan with the active catalog. Parent responses omit
	// them.
	CountsAsCare bool
	PickupTimes  map[string]string
	// Selected marks the child's current booking, so the modal opens
	// prefilled.
	Selected     bool
	SelectedDays []string
	// Automatic marks a current booking derived solely from another offering.
	Automatic bool
	// IsActive is false only for a held offering removed from the catalog.
	IsActive bool
	// Capacity is nil for unlimited; FreeSlots is nil then too. FreeSlots can
	// be zero for an offering the child already holds.
	Capacity  *int
	FreeSlots *int
	// ActivityGroupID is set when the offering is bound to an AG, the shape a
	// Kurs has (#3075).
	ActivityGroupID *int64
}

// OfferingChangeCatalog is everything the parent modal needs.
type OfferingChangeCatalog struct {
	PhaseID   int64
	PhaseName string
	// SelectionMode is the phase's care_offering_selection_mode.
	SelectionMode string
	// EarliestEffectiveFrom and LatestEffectiveFrom bound the date picker.
	EarliestEffectiveFrom calendar.Date
	LatestEffectiveFrom   calendar.Date
	// TargetGradeLevel and TargetSchoolClass narrow sourced courses to a
	// grade or a concrete class.
	TargetGradeLevel  *int16
	TargetSchoolClass string
	// CourseCapacityUntil is the exclusive end of the phase window course
	// capacity is counted in. It never leaves the owner.
	CourseCapacityUntil calendar.Date `json:"-"`
	Items               []OfferingChangeCatalogItem
}

// OfferingChangeView is the child's open request with its live diff, plus
// the most recent decision inside the recency window.
type OfferingChangeView struct {
	Request      *OfferingChangeRequest
	Diff         []OfferingChangeDiffEntry
	LastDecision *OfferingChangeDecision
}

// OfferingChangeDecision is a decided request as the guardian sees it.
type OfferingChangeDecision struct {
	ID int64
	// SubmittedBy keeps request details private to the submitting guardian.
	SubmittedBy        int64
	SubmittedBySelf    bool
	Status             string
	CompleteWithdrawal bool
	DecidedAt          time.Time
	// EffectiveFrom is the date the switch took (or would have taken) effect.
	EffectiveFrom calendar.Date
	Reason        string
	// Requested is what the family asked for, read back from the payload.
	Requested []OfferingChangeRequestedItem
	// AppliedDiff is the frozen review diff (ADR 0002). Nil for rows decided
	// before the snapshot existed.
	AppliedDiff []OfferingChangeDiffEntry
	// OverriddenOfferings lists the Mitbuchungs-Regel targets staff excluded
	// for this one request (#2370).
	OverriddenOfferings []OfferingOverride
}

// OfferingChangePreviewSelection is one offering of the materialization for
// a review card's current override selection.
type OfferingChangePreviewSelection struct {
	OfferingID int64
	State      string
	Days       []string
}

// ManualPlanningConflict groups future occurrences of one recurring roster
// the proposed bookings no longer cover.
type ManualPlanningConflict struct {
	ActivityGroupID   int64
	ActivityGroupName string
	Days              []string
	FirstDate         calendar.Date
	OccurrenceCount   int
}

// OfferingChangePreview is the complete, non-writing approval projection.
type OfferingChangePreview struct {
	Selections                        []OfferingChangePreviewSelection
	ManualPlanningConflicts           []ManualPlanningConflict
	ArrivalExpectationsFollowBookings bool
}

// CreateOfferingChangeInput is a guardian's submission.
type CreateOfferingChangeInput struct {
	StudentID                   int64
	AccountID                   int64
	Selections                  []OfferingChangeSelection
	EffectiveFrom               calendar.Date
	Note                        string
	CompleteWithdrawalConfirmed bool
}

// OfferingChangeDecisionInput is a staff decision.
type OfferingChangeDecisionInput struct {
	RequestID  int64
	Approve    bool
	Reason     string
	ReviewedBy int64
	ActorRole  string
	// ExcludedAutoOfferingIDs overrides the Mitbuchungs-Regel for these
	// target offerings on this one approval (#2370).
	ExcludedAutoOfferingIDs []int64
	// EffectiveFrom is the date the reviewer confirmed (#2484). Nil keeps the
	// guardian's date, moved forward to the first date it can still apply on.
	EffectiveFrom *calendar.Date
	// CompleteWithdrawalConfirmed confirms the fully materialized target.
	CompleteWithdrawalConfirmed bool
	// ExpectedVersion pins the row the caller decided on. Empty skips the
	// check.
	ExpectedVersion string
	// ReasonRequired says the school's reason policy asks for a reason on an
	// approval; a rejection always needs one (#2267).
	ReasonRequired bool
}

// OfferingChangeRequests is the offering-change lifecycle of the parents
// portal and the staff review.
type OfferingChangeRequests interface {
	// Catalog returns the offerings a guardian may choose from for the child.
	Catalog(ctx context.Context, studentID int64) (*OfferingChangeCatalog, error)
	// CatalogAt returns the offerings and booking state at a chosen date.
	CatalogAt(ctx context.Context, studentID int64, effectiveFrom calendar.Date) (*OfferingChangeCatalog, error)
	// GetForStudent returns the child's open request (nil when none) with
	// its live diff and the last decision.
	GetForStudent(ctx context.Context, studentID int64) (*OfferingChangeView, error)
	// SubmitOfferingChange stores a pending request after validating it exactly as an
	// approval would.
	SubmitOfferingChange(ctx context.Context, input CreateOfferingChangeInput) (*OfferingChangeRequest, error)
	// Edit lets the submitting guardian correct their own pending request.
	Edit(ctx context.Context, requestID int64, input CreateOfferingChangeInput, expectedVersion string) (*OfferingChangeRequest, error)
	// EarliestEffectiveFrom is the first date a switch may take effect.
	EarliestEffectiveFrom(ctx context.Context) (calendar.Date, error)
	// PreviewDecision materializes a pending approval without writing it.
	PreviewDecision(ctx context.Context, requestID int64, excludedIDs []int64, effectiveFrom *calendar.Date) (*OfferingChangePreview, error)
	// Decide approves (and applies) or rejects a pending request.
	Decide(ctx context.Context, input OfferingChangeDecisionInput) error
	// MarkDone closes a request whose effective date has passed without
	// applying it.
	MarkDone(ctx context.Context, requestID int64, expectedVersion, reason string, reviewedBy int64) error
}

// OfferingConflictCandidate is a pending request the parent-request conflict
// resolver groups by child.
type OfferingConflictCandidate struct {
	StudentID int64
	UpdatedAt time.Time
}

// OfferingConflictDecision is one verdict of the conflict resolver.
type OfferingConflictDecision struct {
	RequestID       int64
	Approve         bool
	Reason          string
	ReviewerID      int64
	ActorRole       string
	ExpectedVersion string
}

// OfferingStaffValueWrite books the offerings a staff member chose instead of
// the rejected wishes from EffectiveFrom on.
type OfferingStaffValueWrite struct {
	StudentID     int64
	EffectiveFrom calendar.Date
	Selections    []OfferingChangeSelection
	Reason        string
	ReviewerID    int64
	ActorRole     string
}

// OfferingChangeConflicts serves the parent-request conflict resolver
// (#2267, stories 6-10).
type OfferingChangeConflicts interface {
	ConflictCandidate(ctx context.Context, requestID int64) (*OfferingConflictCandidate, error)
	LockConflictRequest(ctx context.Context, requestID int64) error
	DecideConflictRequest(ctx context.Context, decision OfferingConflictDecision) error
	WriteStaffValue(ctx context.Context, write OfferingStaffValueWrite) error
}

// DirectOfferingAdjustmentInput is a staff-side booking replacement.
type DirectOfferingAdjustmentInput struct {
	StudentID                   int64
	EffectiveFrom               calendar.Date
	Selections                  []OfferingChangeSelection
	ExcludedAutoOfferingIDs     []int64
	Reason                      string
	ActorAccountID              int64
	ActorRole                   string
	CompleteWithdrawalConfirmed bool
}

// DirectOfferingAdjustmentPreview is the non-writing staff projection the
// permanent pickup-time editor uses; it reuses the offering-change catalog
// and materialization instead of a second booking path.
type DirectOfferingAdjustmentPreview struct {
	RequestID               int64
	RequestChildID          int64
	Catalog                 *OfferingChangeCatalog
	Consequences            *OfferingChangePreview
	MaterializedPickupTimes map[string]string
}

// DirectOfferingAdjustments is the staff-only seam of permanent pickup-time
// changes. Parent requests keep their own enablement and notice settings.
type DirectOfferingAdjustments interface {
	PrepareDirectOfferingAdjustment(ctx context.Context, input DirectOfferingAdjustmentInput) error
	PreviewDirectOfferingAdjustment(ctx context.Context, input DirectOfferingAdjustmentInput) (*DirectOfferingAdjustmentPreview, error)
	ApplyDirectOfferingAdjustment(ctx context.Context, input DirectOfferingAdjustmentInput) error
}

// OfferingChangeCapability is the whole offering-change review: requests,
// courses, the conflict resolver side and the direct adjustments.
type OfferingChangeCapability interface {
	OfferingChangeRequests
	CourseRequests
	OfferingChangeConflicts
	DirectOfferingAdjustments
}
