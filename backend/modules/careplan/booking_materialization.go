package careplan

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Booking materialization (#3560) turns approved care-offering bookings into
// the Timetable rosters, dated offering adjustments and projected pickup
// times that depend on them. Every write runs in the caller's tenant
// transaction, so an approval, an adjustment and its roster, audit and pickup
// effects commit or roll back together.

var (
	// ErrBookingRequestNotFound means the enrollment request of an adjustment
	// does not exist. Enrollment decisions return the same value.
	ErrBookingRequestNotFound = errors.New("enrollment request not found")
	// ErrBookingChildNotFound means the request child of an adjustment does
	// not exist or belongs to another request. Enrollment decisions return
	// the same value.
	ErrBookingChildNotFound = errors.New("request child not found")
)

// The sources of an offering adjustment. A direct correction by the office is
// gated by the live care-offerings setting and is listed as its own row kind
// in the central history; a request-applied change carries the capability
// frozen when the parent submitted.
const (
	OfferingAdjustmentSourceDirect  = "direct"
	OfferingAdjustmentSourceRequest = "request"
)

// OfferingRosterResync describes one template's offering-source rule for the
// roster resync (#2137): the template's offering-sourced enrollments are
// reconciled with the union of the source offerings' approved bookings from
// EffectiveFrom on.
type OfferingRosterResync struct {
	TemplateID int64
	// OfferingIDs are the source offerings in stored order; later offerings
	// only contribute coverage the earlier ones do not plan. Empty removes the
	// source.
	OfferingIDs []int64
	// GradeLevels and SchoolClasses filter the children; they are mutually
	// exclusive and empty admits everyone.
	GradeLevels   []int
	SchoolClasses []string
	// CalendarPeriodID is the template's period pin.
	CalendarPeriodID *int64
	// EffectiveFrom bounds the rewrite: history before it is never touched.
	EffectiveFrom calendar.Date
	// ScopeRequestChildIDs restricts the rewrite to the given request
	// children (empty = the whole template).
	ScopeRequestChildIDs []int64
	// TolerateDriftedSources loads the source offerings without the active
	// and phase checks. Detach fallback only; never set on a save path.
	TolerateDriftedSources bool
}

// SourcedRosters keeps the rosters of offering-sourced Regeltermine in step
// with the bookings, grade transitions and catalog edits that feed them. The
// caller holds the tenant recurrence lock.
type SourcedRosters interface {
	// ResyncTemplateOfferingRoster reconciles one template's sourced roster
	// and its already-materialized future occurrences.
	ResyncTemplateOfferingRoster(ctx context.Context, in OfferingRosterResync) error
	// ResyncOfferingSourcedTemplates re-reconciles every sourced template of
	// the tenant; drifted templates are skipped with a warning.
	ResyncOfferingSourcedTemplates(ctx context.Context, effectiveFrom calendar.Date) error
	// ResyncTemplatesSourcedFromOffering re-reconciles the templates sourcing
	// one offering; a template the edit would strand is refused.
	ResyncTemplatesSourcedFromOffering(ctx context.Context, offeringID int64, effectiveFrom calendar.Date) error
	// DetachTemplatesSourcedFromOffering removes an offering from every
	// template sourcing it before the offering is deleted.
	DetachTemplatesSourcedFromOffering(ctx context.Context, offeringID int64, effectiveFrom calendar.Date) error
}

// OfferingSourceEditor serves the Regeltermin editor (#2137, #3140).
type OfferingSourceEditor interface {
	// ListOfferingSourceOptions lists the offerings a template may source
	// from, restricted to those fitting the calendar period when one is given.
	ListOfferingSourceOptions(ctx context.Context, calendarPeriodID *int64) ([]OfferingSourceOption, error)
	// CombinedOfferingSourceCounts validates a selection like a save would
	// and counts its distinct approved children.
	CombinedOfferingSourceCounts(ctx context.Context, offeringIDs []int64, calendarPeriodID *int64) (*OfferingSourceCombinedCounts, error)
	// TemplateRosterMaintenanceFeeds resolves the offering-side facts of the
	// roster indicator with a constant number of reads.
	TemplateRosterMaintenanceFeeds(ctx context.Context, templates []TemplateRosterFeedQuery) (map[int64]TemplateRosterFeeds, error)
}

// BookingMaterializer is what an Enrollment decision asks of Care Plan when
// it approves a child.
type BookingMaterializer interface {
	// MaterializeApprovedBookings writes the roster rows the child's
	// bookings plan, without the multi-source union resync: the child is not
	// approved yet, so the union cannot see it.
	MaterializeApprovedBookings(ctx context.Context, requestChildID, studentID int64, phase OfferingPhase) error
	// ResyncMultiSourceTemplatesForChild re-reconciles every multi-source
	// template fed by one of the child's offerings once the child is
	// approved.
	ResyncMultiSourceTemplatesForChild(ctx context.Context, requestChildID int64, phase OfferingPhase) error
	// LockOfferingDerivedWrites takes the class-writes gate and then the
	// recurrence gate, before the caller locks student rows.
	LockOfferingDerivedWrites(ctx context.Context) error
}

// OfferingAdjustments applies a staff correction or an approved change
// request to a child's bookings, rosters and pickup projection.
type OfferingAdjustments interface {
	AdjustOfferings(ctx context.Context, in OfferingAdjustment) (*OfferingAdjustmentResult, error)
}

// OfferingPickupTimes keeps the materialized consumers of the date-aware
// offering pickup projection in step.
type OfferingPickupTimes interface {
	// ReconcileOfferingPickupForStudents refreshes the students' derived
	// pickup effects; it writes no weekly pickup rows.
	ReconcileOfferingPickupForStudents(ctx context.Context, studentIDs []int64) error
	// ReconcileOfferingPickupForOffering refreshes every current or future
	// child of an edited offering.
	ReconcileOfferingPickupForOffering(ctx context.Context, offeringID int64) error
	// ResetStudentPickupDayToOffering removes the manual weekly pickup row of
	// the date's weekday when the offering projects a time for that date.
	ResetStudentPickupDayToOffering(ctx context.Context, studentID int64, date calendar.Date) error
}

// BookingMaterializationCapability is the whole booking materialization.
type BookingMaterializationCapability interface {
	SourcedRosters
	OfferingSourceEditor
	BookingMaterializer
	OfferingAdjustments
	OfferingPickupTimes
}

// CareOfferingCapability is the care-offering catalog together with the
// booking materialization it feeds, the offering-change review deciding
// changes to the bookings (#3561) and the pickup adjustments switching them.
type CareOfferingCapability interface {
	CareOfferingCatalogCapability
	BookingMaterializationCapability
	OfferingChangeCapability
	PickupAdjustments
}

// BookedOffering is one of a request child's offering selections, with the
// validity the booking holds it in (ValidUntil exclusive).
type BookedOffering struct {
	RequestChildID        int64
	CareOfferingID        int64
	SelectedDays          []string
	ManualSelectedDays    []string
	AutomaticSelectedDays []string
	ValidFrom             *calendar.Date
	ValidUntil            *calendar.Date
}

// OfferingSelection is one materialized offering of a selection: the days
// it books, split into the days a person picked and the days a
// Mitbuchungs-Regel or the required lunch derived.
type OfferingSelection struct {
	OfferingID            int64
	SelectedDays          []string
	ManualSelectedDays    []string
	AutomaticSelectedDays []string
}

// OfferingOverride names a co-booking target whose rule-derived days the
// reviewer switched off.
type OfferingOverride struct {
	OfferingID int64
	Name       string
}

// OfferingAdjustmentChoice is one offering of an adjustment with the days
// picked for a parent-choice offering.
type OfferingAdjustmentChoice struct {
	OfferingID   int64
	SelectedDays []string
}

// OfferingAdjustment is a booking switch of one approved child.
type OfferingAdjustment struct {
	RequestID int64
	ChildID   int64
	Offerings []OfferingAdjustmentChoice
	// ExcludedAutoAddTargetIDs switches off the Mitbuchungs-Regel for these
	// target offerings.
	ExcludedAutoAddTargetIDs map[int64]bool
	Reason                   string
	ActorAccountID           int64
	ActorRole                string
	// EffectiveFrom makes the switch dated: history before it stays.
	EffectiveFrom *calendar.Date
	// CompleteWithdrawalConfirmed confirms a switch that removes every care
	// day.
	CompleteWithdrawalConfirmed bool
	// Source is OfferingAdjustmentSourceDirect or
	// OfferingAdjustmentSourceRequest.
	Source string
}

// OfferingAdjustmentResult is the materialization an adjustment persisted, so
// a change-request decision can describe exactly the booking it wrote.
type OfferingAdjustmentResult struct {
	RequestChildID     int64
	Before             []BookedOffering
	Selections         []OfferingSelection
	Offerings          map[int64]CareOffering
	Overridden         []OfferingOverride
	CompleteWithdrawal bool
}

// OfferingAdjustmentSnapshot is one offering in the before and after
// snapshots of an adjustment's audit row. The JSON shape is persisted.
type OfferingAdjustmentSnapshot struct {
	OfferingID            string   `json:"offering_id"`
	OfferingName          string   `json:"offering_name"`
	DaysOfWeekMode        string   `json:"days_of_week_mode"`
	SelectedDays          []string `json:"selected_days,omitempty"`
	ManualSelectedDays    []string `json:"manual_selected_days,omitempty"`
	AutomaticSelectedDays []string `json:"automatic_selected_days,omitempty"`
	AvailableDays         []string `json:"available_days,omitempty"`
}

// OfferingAdjustmentRecord is the audit row of one adjustment.
type OfferingAdjustmentRecord struct {
	RequestID                   int64
	RequestChildID              int64
	StudentID                   int64
	ActorAccountID              int64
	ActorRole                   string
	ActorNameSnapshot           *string
	ActorEmailSnapshot          *string
	Reason                      string
	Source                      string
	Before                      []byte
	After                       []byte
	CompleteWithdrawalConfirmed bool
}
