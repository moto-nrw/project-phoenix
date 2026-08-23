// Post-enrollment offering changes (#1665): a guardian requests a different
// set of care offerings / AGs for a child that is already enrolled, effective
// on a chosen date; staff decide it on the central "Änderungsanfragen" page,
// and an approval applies the switch through the same adjustment path an admin
// correction uses — dated, so the groups attended before the switch stay on the
// record.
//
// Why not enrollment.change_requests: those rows are whole-form corrections of
// a submission whose review window is still open, carrying a form snapshot and
// the submission-window rules. A mid-year switch shares neither.
package enrollment

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/parentmessaging"
	usersService "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// OfferingChangeRequestType is the request_type stamped on the chat pills, so
// a localized client can tell this apart from care_schedule / master_data.
const OfferingChangeRequestType = "care_offering"

// offeringChangeMaxNoteLen bounds free text (in runes) from either side so a
// direct API client cannot persist an unbounded payload. Matches the limit the
// care-schedule requests use for the same fields.
const offeringChangeMaxNoteLen = 2000

// German pill texts. The staff portal renders these directly; the parents
// portal localizes from the structured event fields.
const (
	offeringChangeCreatedBody   = "Anfrage: Betreuungsangebote ändern"
	offeringChangeRejectedBody  = "Anfrage abgelehnt"
	offeringChangeWithdrawnBody = "Anfrage zurückgezogen"
)

// offeringChangeApprovedBody names the date the office confirmed, so the pill
// answers the question a family asks next: from when (#2484). The parents
// portal renders its own localized text from the payload date beside it.
func offeringChangeApprovedBody(effectiveFrom timezone.Date) string {
	return "Anfrage bestätigt, Betreuungsangebote werden ab " +
		effectiveFrom.Format("02.01.2006") + " umgestellt"
}

var (
	// ErrOfferingChangeDisabled means the school has post-enrollment changes
	// switched off.
	ErrOfferingChangeDisabled = errors.New("enrollment: post-enrollment offering changes are disabled")
	// ErrOfferingChangeInvalid marks a request the caller can correct: no
	// selection, an unknown offering, a bad effective date.
	ErrOfferingChangeInvalid = errors.New("enrollment: invalid offering change request")
	// ErrOfferingChangeNoEnrollment means the child has no approved enrollment
	// an offering change could be applied to.
	ErrOfferingChangeNoEnrollment = errors.New("enrollment: child has no approved enrollment")
	// ErrOfferingChangeForbidden means the caller does not own the request.
	ErrOfferingChangeForbidden = errors.New("enrollment: offering change request forbidden")
	// ErrOfferingChangeCapacityFull means an offering in the request has no free
	// slot left. Raised at approval time, when it actually matters.
	ErrOfferingChangeCapacityFull = errors.New("enrollment: care offering is at capacity")
	// ErrOfferingChangeDateOutOfRange means the reviewer confirmed a date the
	// switch cannot take effect on: before today, or outside the care period the
	// request belongs to.
	ErrOfferingChangeDateOutOfRange = errors.New("enrollment: confirmed effective date is out of range")
)

// OfferingChangeSelection is one desired offering with its chosen days.
type OfferingChangeSelection struct {
	OfferingID   int64
	SelectedDays []string
}

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
	// CountsAsCare and PickupTimes let staff-side care-plan editors compare a
	// complete proposed weekly plan with the same active catalog that an
	// offering adjustment validates. Parent responses deliberately omit these
	// internal matching fields in their response shaper.
	CountsAsCare bool
	PickupTimes  map[string]string
	// Selected marks the child's current booking, so the modal opens prefilled.
	Selected     bool
	SelectedDays []string
	// Automatic marks a current booking that is derived solely from another
	// offering. It remains visible, but is not a guardian selection.
	Automatic bool
	// IsActive is false only for a held offering that was removed from the
	// catalog. It remains visible to preserve the existing booking.
	IsActive bool
	// Capacity is nil for unlimited; FreeSlots is nil then too. FreeSlots can be
	// zero for an offering the child already holds — losing the slot by editing
	// an unrelated day would be a nasty surprise, so the frontend shows "voll"
	// but keeps an already-selected offering selectable.
	Capacity  *int
	FreeSlots *int
}

// OfferingChangeCatalog is everything the parent modal needs.
type OfferingChangeCatalog struct {
	PhaseID   int64
	PhaseName string
	// SelectionMode is the phase's care_offering_selection_mode, which decides
	// whether picking nothing is allowed.
	SelectionMode string
	// EarliestEffectiveFrom / LatestEffectiveFrom bound the date picker.
	// LatestEffectiveFrom spans all approved care periods so guardians can
	// choose a later approved period, but not a day after their enrollment.
	EarliestEffectiveFrom timezone.Date
	LatestEffectiveFrom   timezone.Date
	Items                 []OfferingChangeCatalogItem
}

// OfferingChangeDiffEntry is one "current → requested" line, shared by the
// parent read view and the staff review card. State and day keys stay stable so
// the parents portal can localize them in the guardian's chosen language.
type OfferingChangeDiffEntry struct {
	OfferingID int64
	Label      string
	OldState   string
	OldDays    []string
	NewState   string
	NewDays    []string
	// NewAutomaticDays is the share of NewDays that a Mitbuchungs-Regel (or the
	// required-lunch derivation) adds rather than the parents (#2365). Empty for
	// a purely parent-chosen line.
	NewAutomaticDays []string
	// NewRuleDays is the share of NewAutomaticDays added specifically by a
	// Mitbuchungs-Regel. Required-lunch days are not included.
	NewRuleDays []string
	// NewDaysWithoutRules is the materialized NEW side after suppressing this
	// target's Mitbuchungs-Regeln. It keeps manual and required-lunch days.
	NewDaysWithoutRules []string
	// AutoTriggerIDs / AutoTriggerNames identify the selected offerings whose
	// Mitbuchungs-Regel produced the automatic share; both empty when the share
	// comes only from the required-lunch derivation. Rule-triggered lines are the
	// ones staff may override per request (#2370).
	AutoTriggerIDs   []int64
	AutoTriggerNames []string
}

// OfferingChangeHistoryItem is one decided request for the staff history. The
// diff comes exclusively from the frozen decision snapshot (ADR 0002) — never
// recomputed against current bookings, which have moved on since the decision.
// Requested supplies a payload-derived recap for withdrawn and legacy rows
// that have no decision snapshot.
type OfferingChangeHistoryItem struct {
	Request      *enrollmentModels.OfferingChangeRequest
	StudentName  string
	ReviewerName string // "" when the row carries no reviewer (withdrawn)
	Diff         []OfferingChangeDiffEntry
	Requested    []OfferingChangeRequestedItem
}

// OfferingChangeView is a request as both sides see it.
type OfferingChangeView struct {
	Request *enrollmentModels.OfferingChangeRequest
	// StudentName / GroupLabel are filled for the staff queue only; the parents
	// portal already knows whose child it is.
	StudentName string
	Diff        []OfferingChangeDiffEntry
	// Unchanged lists the bookings this request leaves exactly as they are, so
	// the staff review card can show the child's complete picture after the
	// decision instead of only the changed lines (#2434). Staff queue only.
	Unchanged []OfferingChangeDiffEntry
	// FullWithdrawal is true when approving would leave the child with no
	// offering at all — the case that looked like an ordinary request in the
	// OGS-am-Berg incident (#2434). Staff queue only.
	FullWithdrawal bool
	// EarliestEffectiveFrom / LatestEffectiveFrom bound the date the reviewer
	// may confirm the switch for (#2484), so the card cannot offer a date the
	// approval would refuse. Zero when the care period could not be resolved;
	// the decision then validates alone. Staff queue only.
	EarliestEffectiveFrom timezone.Date
	LatestEffectiveFrom   timezone.Date
	// RequestedEffectiveFrom is the date the family actually asked for. It
	// differs from EffectiveFrom once that date has passed (or falls before the
	// care period starts) while the request waited, and the queue then shows the
	// earliest date an approval can still use. The card names both, so the
	// reviewer is not left wondering where the date came from (#2484). Staff
	// queue only.
	RequestedEffectiveFrom timezone.Date
	// LastDecision is the most recent decided request inside the recency window,
	// so a guardian learns the outcome (and a rejection's reason) on the page
	// they submitted from instead of only in the chat.
	LastDecision *OfferingChangeDecision
}

// OfferingChangeDecision is a decided request as the guardian sees it. It
// carries no diff: after an approval was applied the "current → requested"
// comparison is empty, which would read as "nothing changed".
type OfferingChangeDecision struct {
	ID     int64
	Status string
	// DecidedAt is when staff decided it.
	DecidedAt time.Time
	// EffectiveFrom is the date the switch took (or would have taken) effect.
	EffectiveFrom timezone.Date
	Reason        string
	// Requested is what the family asked for, read back from the stored payload.
	// A rejection is only understandable next to the request it refers to, and a
	// live "current → requested" comparison cannot serve here: after an approval
	// was applied it is empty, and after a rejection the payload is the only
	// record of the wish.
	Requested []OfferingChangeRequestedItem
	// AppliedDiff is the frozen review diff from the decision snapshot (ADR
	// 0002): what the decision actually compared and, for an approval, applied.
	// Nil for rows decided before the snapshot column existed; a non-nil empty
	// slice represents a frozen zero-change decision.
	AppliedDiff []OfferingChangeDiffEntry
	// OverriddenOfferings lists the Mitbuchungs-Regel targets staff excluded for
	// this one request at approval time (#2370).
	OverriddenOfferings []enrollmentModels.OfferingChangeSnapshotOffering
}

// OfferingChangeRequestedItem is one offering of a decided request, resolved to
// its name and the booked days at read time.
type OfferingChangeRequestedItem struct {
	OfferingID int64
	Name       string
	// Days are the requested day abbreviations ("mon"), empty when the offering
	// covers every care day.
	Days []string
}

// OfferingChangePreviewSelection is one offering from the authoritative
// materialization for a review card's current override selection.
type OfferingChangePreviewSelection struct {
	OfferingID int64
	State      string
	Days       []string
}

// ManualPlanningConflict groups future occurrences from one recurring roster
// that the proposed care bookings no longer cover.
type ManualPlanningConflict struct {
	ActivityGroupID   int64
	ActivityGroupName string
	Days              []string
	FirstDate         timezone.Date
	OccurrenceCount   int
}

// OfferingChangePreview is the complete, non-writing approval projection.
type OfferingChangePreview struct {
	Selections                        []OfferingChangePreviewSelection
	ManualPlanningConflicts           []ManualPlanningConflict
	ArrivalExpectationsFollowBookings bool
}

// DirectOfferingAdjustmentPreview is the non-writing staff projection used
// by the permanent pickup-time editor. It deliberately reuses the offering
// change catalog and materializer instead of creating a second booking path.
type DirectOfferingAdjustmentPreview struct {
	RequestID               int64
	RequestChildID          int64
	Catalog                 *OfferingChangeCatalog
	Consequences            *OfferingChangePreview
	MaterializedPickupTimes map[string]string
}

type DirectOfferingAdjustmentInput struct {
	StudentID               int64
	EffectiveFrom           timezone.Date
	Selections              []OfferingChangeSelection
	ExcludedAutoOfferingIDs []int64
	Reason                  string
	ActorAccountID          int64
	ActorRole               string
}

// DirectOfferingAdjustmentCoordinator is the narrow staff-only seam used by
// permanent pickup-time changes. The parents request feature continues to use
// OfferingChangeRequestService and its own enablement/notice settings.
type DirectOfferingAdjustmentCoordinator interface {
	PrepareDirectOfferingAdjustment(ctx context.Context, input DirectOfferingAdjustmentInput) error
	PreviewDirectOfferingAdjustment(ctx context.Context, input DirectOfferingAdjustmentInput) (*DirectOfferingAdjustmentPreview, error)
	ApplyDirectOfferingAdjustment(ctx context.Context, input DirectOfferingAdjustmentInput) error
}

type DirectOfferingAdjustmentApplier interface {
	LockOfferingDerivedWrites(ctx context.Context) error
	UpdateChildOfferings(ctx context.Context, input UpdateChildOfferingsInput) (*enrollmentModels.RequestChild, error)
}

// offeringDecisionRecencyDays bounds how long a decided request keeps being
// reported. Same window the excused-absence requests use for the same purpose:
// long enough that a guardian sees the outcome on their next visit, short
// enough that the card does not turn into a history list.
const offeringDecisionRecencyDays = 14

// CreateOfferingChangeInput is a guardian's submission.
type CreateOfferingChangeInput struct {
	StudentID     int64
	AccountID     int64
	Selections    []OfferingChangeSelection
	EffectiveFrom timezone.Date
	Note          string
}

// DecideOfferingChangeInput is a staff decision.
type DecideOfferingChangeInput struct {
	RequestID  int64
	Approve    bool
	Reason     string
	ReviewedBy int64
	ActorRole  string
	// ExcludedAutoOfferingIDs overrides the Mitbuchungs-Regel for these target
	// offerings on this one approval (#2370): their rule-derived days are not
	// applied, the rule itself stays active. Only valid with Approve, and only
	// for offerings the diff marks as rule-triggered.
	ExcludedAutoOfferingIDs []int64
	// EffectiveFrom is the date the reviewer confirmed the switch takes effect
	// on (#2484). Nil keeps the guardian's requested date, moved forward to the
	// first date it can still apply on. Only valid together with Approve.
	EffectiveFrom *timezone.Date
}

// OfferingChangeRequestService is the lifecycle contract. Every method runs
// inside a tenant transaction opened by its caller.
type OfferingChangeRequestService interface {
	// Catalog returns the offerings a guardian may choose from for this child.
	Catalog(ctx context.Context, studentID int64) (*OfferingChangeCatalog, error)
	// CatalogAt returns the offerings and booking state at a chosen effective
	// date, so a complete replacement selection cannot be based on stale rows.
	CatalogAt(ctx context.Context, studentID int64, effectiveFrom timezone.Date) (*OfferingChangeCatalog, error)

	// GetForStudent returns the child's open request (nil when none) plus its
	// live diff against the current booking.
	GetForStudent(ctx context.Context, studentID int64) (*OfferingChangeView, error)

	// Create stores a pending request after validating it exactly as an
	// approval would, so a request that cannot be applied is never accepted.
	Create(ctx context.Context, input CreateOfferingChangeInput) (*enrollmentModels.OfferingChangeRequest, error)

	// Withdraw flips the submitting guardian's own pending request to withdrawn.
	Withdraw(ctx context.Context, requestID, accountID, studentID int64) error

	// ListPending backs the working list, newest submission first. The filters
	// narrow and page the query in SQL; their zero value returns the whole
	// queue.
	ListPending(ctx context.Context, filters modelBase.RequestQueueFilters) (items []*OfferingChangeView, next *usersService.HistoryCursor, err error)
	// ListHistory returns decided requests newest-decision-first, keyset
	// paginated on (updated_at, id). A zero BeforeInstant returns the first
	// page; next is nil when no older rows exist beyond this page.
	ListHistory(ctx context.Context, filters modelBase.RequestQueueFilters) (items []*OfferingChangeHistoryItem, next *usersService.HistoryCursor, err error)
	// ListDirectCorrections returns the office's own corrections to bookings,
	// newest change first, keyset paginated on (changed_at, id). They are not
	// requests: no pending state, nothing to decide (#2436).
	ListDirectCorrections(ctx context.Context, filters modelBase.RequestQueueFilters) (items []*DirectCorrectionItem, next *usersService.HistoryCursor, err error)
	// PendingCount backs the staff sidebar badge without constructing queue diffs.
	PendingCount(ctx context.Context) (int, error)
	// PreviewDecision materializes a pending approval with the supplied
	// Mitbuchungs-Regel overrides and confirmed effective date without writing
	// it. A nil date previews the guardian's requested date.
	PreviewDecision(
		ctx context.Context,
		requestID int64,
		excludedIDs []int64,
		effectiveFrom *timezone.Date,
	) (*OfferingChangePreview, error)

	// Decide approves (and applies) or rejects a pending request.
	Decide(ctx context.Context, input DecideOfferingChangeInput) error

	// EarliestEffectiveFrom is the first date a switch may take effect under
	// the school's configured lead time.
	EarliestEffectiveFrom(ctx context.Context) (timezone.Date, error)
}

// OfferingChangeRequestServiceConfig wires the service.
type OfferingChangeRequestServiceConfig struct {
	ChangeRepo               enrollmentModels.OfferingChangeRequestRepository
	RequestChildRepo         enrollmentModels.RequestChildRepository
	RequestRepo              enrollmentModels.RequestRepository
	PhaseRepo                enrollmentModels.PhaseRepository
	CareOfferingRepo         enrollmentModels.CareOfferingRepository
	ImpactRepo               enrollmentModels.OfferingChangeImpactRepository
	RequestChildOfferingRepo enrollmentModels.RequestChildOfferingRepository
	StudentRepo              usersModels.StudentRepository
	PersonRepo               usersModels.PersonRepository
	// OfferingAdjustmentRepo backs the direct-correction feed of the central
	// history (#2436); the same append-only log the decision service writes.
	OfferingAdjustmentRepo auditModels.EnrollmentOfferingAdjustmentRepository
	UserContext            authorize.StudentAccessUserContext
	// Applier performs the dated adjustment on approval. It is the decision
	// service, reached through the same narrow interface the change-request
	// service uses.
	Applier       ChangeRequestDecisionApplier
	DirectApplier DirectOfferingAdjustmentApplier
	Settings      DecisionSettingsResolver
	Emitter       *parentmessaging.Emitter
	Logger        *slog.Logger
}

type offeringChangeRequestService struct {
	OfferingChangeRequestServiceConfig
}

// NewOfferingChangeRequestService wires a fresh service.
func NewOfferingChangeRequestService(cfg OfferingChangeRequestServiceConfig) OfferingChangeRequestService {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &offeringChangeRequestService{OfferingChangeRequestServiceConfig: cfg}
}

// OfferingChangeLeadDaysDefault mirrors the registry default so a caller
// without a settings service still gets the intended notice period.
const OfferingChangeLeadDaysDefault = 14

// EarliestOfferingChangeDate is the first date a requested switch may take
// effect: tomorrow at the earliest, plus the school's lead time. Exported so
// the parents portal can bound its date picker with the same rule the server
// enforces.
func EarliestOfferingChangeDate(today timezone.Date, leadDays int) timezone.Date {
	if leadDays < 0 {
		leadDays = 0
	}
	return today.AddDays(1 + leadDays)
}

func (s *offeringChangeRequestService) EarliestEffectiveFrom(ctx context.Context) (timezone.Date, error) {
	leadDays := OfferingChangeLeadDaysDefault
	if s.Settings != nil {
		resolved, err := s.resolveLeadDays(ctx)
		if err != nil {
			return timezone.Date{}, err
		}
		leadDays = resolved
	}
	return EarliestOfferingChangeDate(timezone.TodayDate(), leadDays), nil
}

// resolveLeadDays reads the configured notice period. DecisionSettingsResolver
// deliberately exposes only string/bool, so the number is read as a string and
// parsed here rather than widening the interface for one setting.
func (s *offeringChangeRequestService) resolveLeadDays(ctx context.Context) (int, error) {
	raw, err := s.Settings.ResolveString(ctx, configModel.KeyEnrollmentOfferingChangesLeadDays)
	if err != nil {
		return 0, fmt.Errorf("offering change: resolve lead days: %w", err)
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, fmt.Errorf("offering change: lead-days setting is empty")
	}
	days, convErr := strconv.Atoi(raw)
	if convErr != nil || days < 0 {
		return 0, fmt.Errorf("offering change: lead-days setting must be a non-negative integer")
	}
	return days, nil
}

func (s *offeringChangeRequestService) changesEnabled(ctx context.Context) error {
	if s.Settings == nil {
		return ErrOfferingChangeDisabled
	}
	enabled, err := s.Settings.ResolveBool(ctx, configModel.KeyEnrollmentOfferingChangesEnabled)
	if err != nil {
		// Fail closed: a school that never switched this on must not collect
		// requests because a config read blipped.
		s.Logger.Warn("offering change: resolve enabled failed, failing closed",
			slog.String("error", err.Error()),
		)
		return ErrOfferingChangeDisabled
	}
	if !enabled {
		return ErrOfferingChangeDisabled
	}
	careOfferingsEnabled, err := s.Settings.ResolveBool(ctx, configModel.KeyEnrollmentCareOfferingsEnabled)
	if err != nil {
		s.Logger.Warn("offering change: resolve care offerings enabled failed, failing closed",
			slog.String("error", err.Error()),
		)
		return ErrCareOfferingsDisabled
	}
	if !careOfferingsEnabled {
		return ErrCareOfferingsDisabled
	}
	return nil
}

// carePeriodAt resolves the approved enrollment that covers onDate. A request
// may have to take effect in the next approved period when the notice period
// crosses a care-period boundary.
func (s *offeringChangeRequestService) carePeriodAt(
	ctx context.Context,
	studentID int64,
	onDate timezone.Date,
) (*enrollmentModels.StudentCarePeriod, *enrollmentModels.Phase, error) {
	periods, err := s.RequestChildRepo.ListCarePeriodsByStudentID(ctx, studentID)
	if err != nil {
		return nil, nil, fmt.Errorf("offering change: list care periods: %w", err)
	}
	for _, candidate := range periods {
		if !candidate.ServiceStartDate.After(onDate) && !candidate.ServiceEndDate.Before(onDate) {
			phase, phaseErr := s.PhaseRepo.FindByID(ctx, candidate.PhaseID)
			if phaseErr != nil || phase == nil {
				return nil, nil, fmt.Errorf("offering change: load phase: %w", phaseErr)
			}
			return candidate, phase, nil
		}
	}
	return nil, nil, ErrOfferingChangeNoEnrollment
}

// carePeriodAtOrNext resolves the period covering onDate, or the nearest
// approved period beginning after it. The catalog needs the latter so it can
// offer the first valid date when a lead time falls before the next period.
func (s *offeringChangeRequestService) carePeriodAtOrNext(
	ctx context.Context,
	studentID int64,
	onDate timezone.Date,
) (*enrollmentModels.StudentCarePeriod, *enrollmentModels.Phase, error) {
	periods, err := s.RequestChildRepo.ListCarePeriodsByStudentID(ctx, studentID)
	if err != nil {
		return nil, nil, fmt.Errorf("offering change: list care periods: %w", err)
	}
	var next *enrollmentModels.StudentCarePeriod
	for _, candidate := range periods {
		if !candidate.ServiceStartDate.After(onDate) && !candidate.ServiceEndDate.Before(onDate) {
			phase, phaseErr := s.PhaseRepo.FindByID(ctx, candidate.PhaseID)
			if phaseErr != nil || phase == nil {
				return nil, nil, fmt.Errorf("offering change: load phase: %w", phaseErr)
			}
			return candidate, phase, nil
		}
		if candidate.ServiceStartDate.After(onDate) &&
			(next == nil || candidate.ServiceStartDate.Before(next.ServiceStartDate)) {
			next = candidate
		}
	}
	if next == nil {
		return nil, nil, ErrOfferingChangeNoEnrollment
	}
	phase, err := s.PhaseRepo.FindByID(ctx, next.PhaseID)
	if err != nil || phase == nil {
		return nil, nil, fmt.Errorf("offering change: load phase: %w", err)
	}
	return next, phase, nil
}

func (s *offeringChangeRequestService) carePeriodForEarliestEffectiveDate(
	ctx context.Context,
	studentID int64,
) (*enrollmentModels.StudentCarePeriod, *enrollmentModels.Phase, timezone.Date, error) {
	earliest, err := s.EarliestEffectiveFrom(ctx)
	if err != nil {
		return nil, nil, timezone.Date{}, err
	}
	period, phase, err := s.carePeriodAtOrNext(ctx, studentID, earliest)
	if err != nil {
		return nil, nil, timezone.Date{}, err
	}
	if earliest.Before(phase.ServiceStartDate) {
		earliest = phase.ServiceStartDate
	}
	return period, phase, earliest, nil
}

// latestContiguousApprovedCarePeriodEnd returns the end of the uninterrupted
// approved care period beginning with initial. The date picker cannot offer a
// date in a gap because CatalogAt requires an enrollment on the chosen date.
func (s *offeringChangeRequestService) latestContiguousApprovedCarePeriodEnd(
	ctx context.Context,
	studentID int64,
	initial *enrollmentModels.StudentCarePeriod,
) (timezone.Date, error) {
	periods, err := s.RequestChildRepo.ListCarePeriodsByStudentID(ctx, studentID)
	if err != nil {
		return timezone.Date{}, fmt.Errorf("offering change: list care periods: %w", err)
	}
	if initial == nil {
		return timezone.Date{}, ErrOfferingChangeNoEnrollment
	}
	return contiguousCarePeriodEnd(periods, initial), nil
}

func contiguousCarePeriodEnd(
	periods []*enrollmentModels.StudentCarePeriod,
	initial *enrollmentModels.StudentCarePeriod,
) timezone.Date {
	latest := initial.ServiceEndDate
	// Periods are returned newest first. Repeat the scan because an earlier
	// overlapping period can extend the range far enough to join another one.
	for extended := true; extended; {
		extended = false
		for _, period := range periods {
			if period != nil &&
				!period.ServiceStartDate.After(latest.AddDays(1)) &&
				latest.Before(period.ServiceEndDate) {
				latest = period.ServiceEndDate
				extended = true
			}
		}
	}
	return latest
}

func (s *offeringChangeRequestService) Catalog(
	ctx context.Context,
	studentID int64,
) (*OfferingChangeCatalog, error) {
	if err := s.changesEnabled(ctx); err != nil {
		return nil, err
	}
	period, phase, earliest, err := s.carePeriodForEarliestEffectiveDate(ctx, studentID)
	if err != nil {
		return nil, err
	}
	latest, err := s.latestContiguousApprovedCarePeriodEnd(ctx, studentID, period)
	if err != nil {
		return nil, err
	}
	return s.catalogAt(ctx, studentID, period, phase, earliest, latest, earliest)
}

func (s *offeringChangeRequestService) CatalogAt(
	ctx context.Context,
	studentID int64,
	effectiveFrom timezone.Date,
) (*OfferingChangeCatalog, error) {
	if effectiveFrom.IsZero() {
		return s.Catalog(ctx, studentID)
	}
	if err := s.changesEnabled(ctx); err != nil {
		return nil, err
	}
	earliest, err := s.EarliestEffectiveFrom(ctx)
	if err != nil {
		return nil, err
	}
	if effectiveFrom.Before(earliest) {
		return nil, fmt.Errorf("%w: effective date is earlier than the school's notice period", ErrOfferingChangeInvalid)
	}
	period, phase, err := s.carePeriodAt(ctx, studentID, effectiveFrom)
	if err != nil {
		return nil, err
	}
	if earliest.Before(phase.ServiceStartDate) {
		earliest = phase.ServiceStartDate
	}
	latest, err := s.latestContiguousApprovedCarePeriodEnd(ctx, studentID, period)
	if err != nil {
		return nil, err
	}
	return s.catalogAt(ctx, studentID, period, phase, earliest, latest, effectiveFrom)
}

func (s *offeringChangeRequestService) catalogAt(
	ctx context.Context,
	studentID int64,
	period *enrollmentModels.StudentCarePeriod,
	phase *enrollmentModels.Phase,
	earliest, latest, onDate timezone.Date,
) (*OfferingChangeCatalog, error) {
	active, err := s.CareOfferingRepo.ListActiveByPhase(ctx, phase.ID)
	if err != nil {
		return nil, fmt.Errorf("offering change: list active offerings: %w", err)
	}
	current, err := s.RequestChildOfferingRepo.ListByRequestChildIDAtDate(ctx, period.RequestChildID, onDate)
	if err != nil {
		return nil, fmt.Errorf("offering change: list current offerings: %w", err)
	}
	currentByID := make(map[int64]*enrollmentModels.RequestChildOffering, len(current))
	for _, link := range current {
		if link != nil {
			currentByID[link.CareOfferingID] = link
		}
	}
	child, err := s.RequestChildRepo.FindByID(ctx, period.RequestChildID)
	if err != nil || child == nil {
		return nil, fmt.Errorf("offering change: load request child: %w", err)
	}
	activeByID := offeringsByID(active)
	allowed, err := availableCareOfferingsForGrade(activeByID, child.TargetGradeLevel)
	if err != nil {
		return nil, fmt.Errorf("offering change: filter eligible offerings: %w", err)
	}
	if _, err := s.addHeldOfferingsAtDate(ctx, period.RequestChildID, onDate, allowed); err != nil {
		return nil, err
	}
	catalog := &OfferingChangeCatalog{
		PhaseID:               phase.ID,
		PhaseName:             phase.Name,
		SelectionMode:         phase.CareOfferingSelectionMode,
		EarliestEffectiveFrom: earliest,
		LatestEffectiveFrom:   latest,
		Items:                 make([]OfferingChangeCatalogItem, 0, len(allowed)),
	}
	for _, offering := range allowed {
		if offering == nil {
			continue
		}
		item, itemErr := s.catalogItem(ctx, offering, currentByID[offering.ID], onDate, phase.ServiceEndDate.AddDays(1))
		if itemErr != nil {
			return nil, itemErr
		}
		item.IsActive = activeByID[offering.ID] != nil
		catalog.Items = append(catalog.Items, item)
	}
	sort.SliceStable(catalog.Items, func(i, j int) bool {
		return catalog.Items[i].OfferingID < catalog.Items[j].OfferingID
	})
	return catalog, nil
}

func (s *offeringChangeRequestService) catalogItem(
	ctx context.Context,
	offering *enrollmentModels.CareOffering,
	current *enrollmentModels.RequestChildOffering,
	onDate, phaseEndExclusive timezone.Date,
) (OfferingChangeCatalogItem, error) {
	item := OfferingChangeCatalogItem{
		OfferingID:      offering.ID,
		Name:            offering.Name,
		DaysOfWeekMode:  offering.DaysOfWeekMode,
		AvailableDays:   append([]string(nil), offering.AvailableDays...),
		SelectionGroup:  offering.SelectionGroup,
		SelectionRule:   offering.SelectionRule,
		IsRequired:      offering.IsRequired,
		PriceCents:      offering.PriceCents,
		IncludesLunch:   offering.IncludesLunch,
		IncludesHoliday: offering.IncludesHolidayCare,
		CountsAsCare:    offering.CountsAsCare,
		PickupTimes:     maps.Clone(offering.PickupTimes),
	}
	if offering.Description != nil {
		item.Description = *offering.Description
	}
	if current != nil {
		item.Selected = true
		item.SelectedDays = append([]string(nil), current.SelectedDays...)
		item.Automatic = len(current.ManualSelectedDays) == 0 && len(current.AutomaticSelectedDays) > 0
	}
	if offering.Capacity == nil {
		return item, nil
	}
	capacity := *offering.Capacity
	item.Capacity = &capacity
	taken, err := s.RequestChildOfferingRepo.CountMaxActiveByCareOfferingInRange(ctx, offering.ID, onDate, phaseEndExclusive)
	if err != nil {
		return OfferingChangeCatalogItem{}, fmt.Errorf("offering change: count offering occupancy: %w", err)
	}
	free := max(capacity-taken, 0)
	item.FreeSlots = &free
	return item, nil
}

func (s *offeringChangeRequestService) GetForStudent(
	ctx context.Context,
	studentID int64,
) (*OfferingChangeView, error) {
	pending, err := s.ChangeRepo.GetPendingForStudent(ctx, studentID)
	if err != nil {
		return nil, fmt.Errorf("offering change: get pending: %w", err)
	}
	decision, err := s.lastDecisionForStudent(ctx, studentID)
	if err != nil {
		return nil, err
	}
	if pending == nil && decision == nil {
		return nil, nil
	}
	view := &OfferingChangeView{Request: pending, LastDecision: decision}
	if pending == nil {
		return view, nil
	}
	diff, diffErr := s.diffForRequest(ctx, pending)
	if diffErr != nil {
		// A diff we cannot build must not hide the request itself: the guardian
		// still needs to see that one is open (and be able to withdraw it).
		s.Logger.Warn("offering change: build diff failed",
			slog.Int64("request_id", pending.ID),
			slog.String("error", diffErr.Error()),
		)
		return view, nil
	}
	view.Diff = diff
	return view, nil
}

// lastDecisionForStudent returns the newest approved/rejected request decided
// within the recency window. Withdrawals are skipped: the guardian withdrew it
// themselves and does not need to be told about it.
func (s *offeringChangeRequestService) lastDecisionForStudent(
	ctx context.Context,
	studentID int64,
) (*OfferingChangeDecision, error) {
	rows, err := s.ChangeRepo.ListByStudent(ctx, studentID)
	if err != nil {
		return nil, fmt.Errorf("offering change: list requests: %w", err)
	}
	cutoff := time.Now().AddDate(0, 0, -offeringDecisionRecencyDays)
	today := timezone.TodayDate()
	for _, row := range rows {
		if row == nil || row.ReviewedAt == nil {
			continue
		}
		if row.Status != enrollmentModels.OfferingChangeStatusApproved &&
			row.Status != enrollmentModels.OfferingChangeStatusRejected {
			continue
		}
		isFutureApproval := row.Status == enrollmentModels.OfferingChangeStatusApproved && today.Before(row.EffectiveFrom)
		if row.ReviewedAt.Before(cutoff) && !isFutureApproval {
			// Rows are newest-first, so everything below is older too.
			return nil, nil
		}
		decision := &OfferingChangeDecision{
			ID:            row.ID,
			Status:        row.Status,
			DecidedAt:     *row.ReviewedAt,
			EffectiveFrom: row.EffectiveFrom,
		}
		if row.DecisionReason != nil {
			decision.Reason = *row.DecisionReason
		}
		if row.DecisionSnapshot != nil {
			decision.AppliedDiff = diffEntriesFromSnapshot(row.DecisionSnapshot.Diff)
			decision.OverriddenOfferings = row.DecisionSnapshot.OverriddenOfferings
		}
		requested, reqErr := s.requestedItems(ctx, row)
		if reqErr != nil {
			// The outcome matters more than the recap: report the decision even
			// when the payload cannot be resolved to names.
			s.Logger.Warn("offering change: resolve requested offerings failed",
				slog.Int64("request_id", row.ID),
				slog.String("error", reqErr.Error()),
			)
		} else {
			decision.Requested = requested
		}
		return decision, nil
	}
	return nil, nil
}

// requestedItems reads the stored payload back into named offerings, sorted by
// catalog order so the recap matches the booking list above it. An offering
// deleted from the catalog since is still listed, by id, rather than dropped:
// the family asked for it, and hiding it would make the decision unreadable.
func (s *offeringChangeRequestService) requestedItems(
	ctx context.Context,
	row *enrollmentModels.OfferingChangeRequest,
) ([]OfferingChangeRequestedItem, error) {
	selections, err := selectionsFromPayload(row.Payload)
	if err != nil {
		return nil, err
	}
	if len(selections) == 0 {
		return []OfferingChangeRequestedItem{}, nil
	}
	ids := make([]int64, 0, len(selections))
	for _, selection := range selections {
		ids = append(ids, selection.OfferingID)
	}
	offerings, err := s.CareOfferingRepo.ListByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("list requested offerings: %w", err)
	}
	nameByID := make(map[int64]string, len(offerings))
	sortByID := make(map[int64]int, len(offerings))
	for _, offering := range offerings {
		if offering != nil {
			nameByID[offering.ID] = offering.Name
			sortByID[offering.ID] = offering.SortOrder
		}
	}
	items := make([]OfferingChangeRequestedItem, 0, len(selections))
	for _, selection := range selections {
		name := nameByID[selection.OfferingID]
		if name == "" {
			name = fmt.Sprintf("Angebot %d", selection.OfferingID)
		}
		items = append(items, OfferingChangeRequestedItem{
			OfferingID: selection.OfferingID,
			Name:       name,
			Days:       append([]string(nil), selection.SelectedDays...),
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		left, right := sortByID[items[i].OfferingID], sortByID[items[j].OfferingID]
		if left == right {
			return items[i].Name < items[j].Name
		}
		return left < right
	})
	return items, nil
}

func (s *offeringChangeRequestService) Create(
	ctx context.Context,
	input CreateOfferingChangeInput,
) (*enrollmentModels.OfferingChangeRequest, error) {
	if err := s.changesEnabled(ctx); err != nil {
		return nil, err
	}
	if input.StudentID <= 0 || input.AccountID <= 0 {
		return nil, fmt.Errorf("%w: student and account are required", ErrOfferingChangeInvalid)
	}
	note := strings.TrimSpace(input.Note)
	if utf8.RuneCountInString(note) > offeringChangeMaxNoteLen {
		return nil, fmt.Errorf("%w: note is too long", ErrOfferingChangeInvalid)
	}
	earliest, err := s.EarliestEffectiveFrom(ctx)
	if err != nil {
		return nil, err
	}
	if input.EffectiveFrom.Before(earliest) {
		return nil, fmt.Errorf("%w: effective date is earlier than the school's notice period", ErrOfferingChangeInvalid)
	}
	period, phase, err := s.carePeriodAt(ctx, input.StudentID, input.EffectiveFrom)
	if err != nil {
		return nil, err
	}
	if input.EffectiveFrom.Before(phase.ServiceStartDate) {
		return nil, fmt.Errorf("%w: effective date is before the care period starts", ErrOfferingChangeInvalid)
	}
	if input.EffectiveFrom.After(phase.ServiceEndDate) {
		return nil, fmt.Errorf("%w: effective date is after the care period ends", ErrOfferingChangeInvalid)
	}
	current, err := s.RequestChildOfferingRepo.ListByRequestChildIDAtDate(ctx, period.RequestChildID, input.EffectiveFrom)
	if err != nil {
		return nil, fmt.Errorf("offering change: list current offerings: %w", err)
	}
	selections, err := s.validateSelections(ctx, phase, period.RequestChildID, input.EffectiveFrom,
		withoutAutomaticSelections(current, input.Selections))
	if err != nil {
		return nil, err
	}
	materialized, err := s.materializedSelections(ctx, phase, period.RequestChildID, input.EffectiveFrom, selections)
	if err != nil {
		return nil, err
	}
	if sameMaterializedOfferingSelections(current, materialized) {
		return nil, fmt.Errorf("%w: offerings are unchanged", ErrOfferingChangeInvalid)
	}
	existing, err := s.ChangeRepo.GetPendingForStudent(ctx, input.StudentID)
	if err != nil {
		return nil, fmt.Errorf("offering change: check pending: %w", err)
	}
	if existing != nil {
		return nil, enrollmentModels.ErrOfferingChangeAlreadyPending
	}
	row := &enrollmentModels.OfferingChangeRequest{
		StudentID:      input.StudentID,
		RequestChildID: period.RequestChildID,
		SubmittedBy:    input.AccountID,
		Payload:        payloadFromSelections(selections),
		EffectiveFrom:  input.EffectiveFrom,
		Status:         enrollmentModels.OfferingChangeStatusPending,
	}
	if note != "" {
		row.ParentNote = &note
	}
	if err := s.ChangeRepo.Create(ctx, row); err != nil {
		if isPendingOfferingChangeConflict(err) {
			return nil, enrollmentModels.ErrOfferingChangeAlreadyPending
		}
		return nil, fmt.Errorf("offering change: create: %w", err)
	}
	s.emitPillAfterCommit(ctx, row, parentmessaging.ChildEvent{
		EventType:      usersModels.ParentMessageEventRequestCreated,
		ActorKind:      usersModels.ParentMessageSenderGuardian,
		ActorAccountID: input.AccountID,
		Body:           offeringChangeCreatedBody,
		RequestType:    OfferingChangeRequestType,
		RequestStatus:  usersModels.ParentMessageRequestStatusOpen,
	})
	s.Logger.Info("offering change request created",
		slog.Int64("student_id", input.StudentID),
		slog.Int64("request_child_id", period.RequestChildID),
		slog.String("effective_from", input.EffectiveFrom.String()),
		slog.Int("offering_count", len(selections)),
	)
	return row, nil
}

func (s *offeringChangeRequestService) Withdraw(ctx context.Context, requestID, accountID, studentID int64) error {
	row, err := s.ChangeRepo.FindByIDForUpdate(ctx, requestID)
	if err != nil {
		return err
	}
	// Ownership first, so a foreign request's id cannot be probed for status.
	if row.StudentID != studentID || row.SubmittedBy != accountID {
		return ErrOfferingChangeForbidden
	}
	if row.IsTerminal() {
		return enrollmentModels.ErrOfferingChangeNotPending
	}
	if err := s.ChangeRepo.Decide(ctx, requestID, enrollmentModels.OfferingChangeStatusWithdrawn, nil, nil, false); err != nil {
		return err
	}
	s.emitPillAfterCommit(ctx, row, parentmessaging.ChildEvent{
		EventType:      usersModels.ParentMessageEventRequestStatus,
		ActorKind:      usersModels.ParentMessageSenderGuardian,
		ActorAccountID: accountID,
		Body:           offeringChangeWithdrawnBody,
		RequestType:    OfferingChangeRequestType,
		RequestStatus:  usersModels.ParentMessageRequestStatusWithdrawn,
	})
	return nil
}

func (s *offeringChangeRequestService) ListPending(ctx context.Context, filters modelBase.RequestQueueFilters) ([]*OfferingChangeView, *usersService.HistoryCursor, error) {
	// limit+1 probes for an older page without a second count query.
	rows, err := s.ChangeRepo.ListPendingForTenant(ctx, probeLimit(filters))
	if err != nil {
		return nil, nil, fmt.Errorf("offering change: list pending: %w", err)
	}
	rows, next := usersService.NextCursor(rows, filters.Limit, func(r *enrollmentModels.OfferingChangeRequest) (time.Time, int64) {
		return r.CreatedAt, r.ID
	})
	studentIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		if row != nil {
			studentIDs = append(studentIDs, row.StudentID)
		}
	}
	students, err := s.StudentRepo.FindByIDs(ctx, studentIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("offering change: load students: %w", err)
	}
	writable := authorize.WritableStudentFilter(ctx, jwt.PermissionsFromCtx(ctx), s.UserContext)
	visibleRows := make([]*enrollmentModels.OfferingChangeRequest, 0, len(rows))
	personIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		if row == nil {
			continue
		}
		student := students[row.StudentID]
		// A child whose care has ended leaves the pending queue: the
		// effect-day pass closes their open requests, and until it runs the
		// queue must not offer a decision on a departed child (#2487).
		if student == nil || !writable(student) || student.IsAlumnus() ||
			student.CareEndedOn(timezone.TodayDate()) {
			continue
		}
		visibleRows = append(visibleRows, row)
		if student.PersonID > 0 {
			personIDs = append(personIDs, student.PersonID)
		}
	}
	persons, err := s.PersonRepo.FindByIDs(ctx, personIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("offering change: load student persons: %w", err)
	}
	reviews, diffErr := s.pendingReviews(ctx, visibleRows)
	if diffErr != nil {
		s.Logger.Warn("offering change: preload pending diffs failed", slog.String("error", diffErr.Error()))
	}
	views := make([]*OfferingChangeView, 0, len(visibleRows))
	for _, row := range visibleRows {
		student := students[row.StudentID]
		reviewRow := *row
		// The queue must name the date an approval would actually apply. That
		// date is clamped to the care period's start as well as to today, which
		// diverges once a phase's service start moves after the request was
		// filed.
		reviewRow.EffectiveFrom = appliedOfferingChangeDate(row.EffectiveFrom)
		review := reviews[row.ID]
		if review != nil {
			reviewRow.EffectiveFrom = review.AppliedDate
		}
		view := &OfferingChangeView{Request: &reviewRow, RequestedEffectiveFrom: row.EffectiveFrom}
		if review != nil {
			view.EarliestEffectiveFrom = review.EarliestDate
			view.LatestEffectiveFrom = review.LatestDate
		}
		if person := persons[student.PersonID]; person != nil {
			view.StudentName = strings.TrimSpace(person.GetFullName())
		}
		if diffErr == nil && review != nil {
			view.Diff = review.Diff
			view.Unchanged = review.Unchanged
			view.FullWithdrawal = review.FullWithdrawal
		}
		views = append(views, view)
	}
	return views, next, nil
}

// probeLimit asks the repository for one row more than the caller wants, so a
// present extra row proves an older page exists. An unbounded page stays
// unbounded.
func probeLimit(filters modelBase.RequestQueueFilters) modelBase.RequestQueueFilters {
	if filters.Limit > 0 {
		filters.Limit++
	}
	return filters
}

func (s *offeringChangeRequestService) ListHistory(ctx context.Context, filters modelBase.RequestQueueFilters) ([]*OfferingChangeHistoryItem, *usersService.HistoryCursor, error) {
	// limit+1 probes for an older page without a second count query.
	rows, err := s.ChangeRepo.ListDecidedForTenant(ctx, probeLimit(filters))
	if err != nil {
		return nil, nil, fmt.Errorf("offering change: list decided: %w", err)
	}
	// The cursor points at the last DB row (not the last visible item): the
	// per-child scope filters after the DB limit, so a cursor built from the
	// filtered page would skip rows.
	rows, next := usersService.NextCursor(rows, filters.Limit, func(r *enrollmentModels.OfferingChangeRequest) (time.Time, int64) {
		return r.UpdatedAt, r.ID
	})
	if len(rows) == 0 {
		return []*OfferingChangeHistoryItem{}, next, nil
	}

	studentIDs := make([]int64, 0, len(rows))
	reviewerIDs := make([]int64, 0, len(rows))
	seenReviewers := make(map[int64]struct{}, len(rows))
	for _, row := range rows {
		studentIDs = append(studentIDs, row.StudentID)
		if row.ReviewedBy != nil && *row.ReviewedBy > 0 {
			if _, ok := seenReviewers[*row.ReviewedBy]; !ok {
				seenReviewers[*row.ReviewedBy] = struct{}{}
				reviewerIDs = append(reviewerIDs, *row.ReviewedBy)
			}
		}
	}
	students, err := s.StudentRepo.FindByIDs(ctx, studentIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("offering change: load students for history: %w", err)
	}
	personIDs := make([]int64, 0, len(students))
	for _, st := range students {
		personIDs = append(personIDs, st.PersonID)
	}
	persons, err := s.PersonRepo.FindByIDs(ctx, personIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("offering change: load student persons for history: %w", err)
	}
	reviewers, err := s.PersonRepo.FindByAccountIDs(ctx, reviewerIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("offering change: load reviewers for history: %w", err)
	}

	// Same per-child scope as ListPending: write gate + alumnus skip.
	writable := authorize.WritableStudentFilter(ctx, jwt.PermissionsFromCtx(ctx), s.UserContext)

	items := make([]*OfferingChangeHistoryItem, 0, len(rows))
	for _, row := range rows {
		student := students[row.StudentID]
		if student == nil || !writable(student) || student.IsAlumnus() {
			continue
		}
		item := &OfferingChangeHistoryItem{
			Request:      row,
			ReviewerName: usersService.ReviewerDisplayName(reviewers, row.ReviewedBy),
		}
		if row.DecisionSnapshot != nil {
			item.Diff = diffEntriesFromSnapshot(row.DecisionSnapshot.Diff)
		} else {
			requested, requestedErr := s.requestedItems(ctx, row)
			if requestedErr != nil {
				s.Logger.Warn("offering change: resolve requested offerings for history failed",
					slog.Int64("request_id", row.ID),
					slog.String("error", requestedErr.Error()),
				)
			} else {
				item.Requested = requested
			}
		}
		if person := persons[student.PersonID]; person != nil {
			item.StudentName = strings.TrimSpace(person.GetFullName())
		}
		items = append(items, item)
	}
	return items, next, nil
}

func (s *offeringChangeRequestService) PendingCount(ctx context.Context) (int, error) {
	rows, err := s.ChangeRepo.ListPendingForTenant(ctx, modelBase.RequestQueueFilters{})
	if err != nil {
		return 0, fmt.Errorf("offering change: list pending for count: %w", err)
	}
	studentIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		if row != nil {
			studentIDs = append(studentIDs, row.StudentID)
		}
	}
	students, err := s.StudentRepo.FindByIDs(ctx, studentIDs)
	if err != nil {
		return 0, fmt.Errorf("offering change: load students for count: %w", err)
	}
	writable := authorize.WritableStudentFilter(ctx, jwt.PermissionsFromCtx(ctx), s.UserContext)
	count := 0
	for _, row := range rows {
		if row == nil {
			continue
		}
		student := students[row.StudentID]
		if student != nil && writable(student) && !student.IsAlumnus() &&
			!student.CareEndedOn(timezone.TodayDate()) {
			count++
		}
	}
	return count, nil
}

// pendingReview is what the staff queue shows for one pending row: the changed
// lines, the bookings that stay untouched, whether approving would leave the
// child with nothing (#2434), and the date the approval would actually apply.
type pendingReview struct {
	Diff           []OfferingChangeDiffEntry
	Unchanged      []OfferingChangeDiffEntry
	FullWithdrawal bool
	AppliedDate    timezone.Date
	// EarliestDate / LatestDate bound the date a reviewer may confirm the
	// switch for (#2484). Zero when the care period could not be resolved.
	EarliestDate timezone.Date
	LatestDate   timezone.Date
}

// pendingReviews preloads the queue's aggregate data in bounded queries. The
// review page and its badge both call ListPending, so loading a complete
// request aggregate per row would turn an ordinary queue into an N+1 query.
func (s *offeringChangeRequestService) pendingReviews(
	ctx context.Context,
	rows []*enrollmentModels.OfferingChangeRequest,
) (map[int64]*pendingReview, error) {
	childIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		if row == nil || row.RequestChildID <= 0 {
			continue
		}
		childIDs = append(childIDs, row.RequestChildID)
	}
	children, err := s.RequestChildRepo.ListByIDs(ctx, childIDs)
	if err != nil {
		return nil, fmt.Errorf("load request children: %w", err)
	}
	childrenByID := make(map[int64]*enrollmentModels.RequestChild, len(children))
	requestIDs := make([]int64, 0, len(children))
	for _, child := range children {
		if child == nil {
			continue
		}
		childrenByID[child.ID] = child
		requestIDs = append(requestIDs, child.RequestID)
	}
	requests, err := s.RequestRepo.ListByIDs(ctx, requestIDs)
	if err != nil {
		return nil, fmt.Errorf("load enrollment requests: %w", err)
	}
	requestsByID := make(map[int64]*enrollmentModels.Request, len(requests))
	phaseIDs := make([]int64, 0, len(requests))
	for _, request := range requests {
		if request == nil {
			continue
		}
		requestsByID[request.ID] = request
		phaseIDs = append(phaseIDs, request.PhaseID)
	}
	phases, err := s.PhaseRepo.ListByIDs(ctx, phaseIDs)
	if err != nil {
		return nil, fmt.Errorf("load phases: %w", err)
	}
	phasesByID := make(map[int64]*enrollmentModels.Phase, len(phases))
	for _, phase := range phases {
		if phase != nil {
			phasesByID[phase.ID] = phase
		}
	}
	// The snapshot has to be read on the date the approval would use, phase
	// clamp included, or the diff describes a booking outside the care period.
	dates := make(map[int64]timezone.Date, len(rows))
	reviews := make(map[int64]*pendingReview, len(rows))
	for _, row := range rows {
		if row == nil || row.RequestChildID <= 0 {
			continue
		}
		child := childrenByID[row.RequestChildID]
		var phase *enrollmentModels.Phase
		if child != nil {
			if request := requestsByID[child.RequestID]; request != nil {
				phase = phasesByID[request.PhaseID]
			}
		}
		date := appliedOfferingChangeDateForPhase(row.EffectiveFrom, phase)
		dates[row.RequestChildID] = date
		review := &pendingReview{AppliedDate: date}
		if phase != nil {
			review.EarliestDate = appliedOfferingChangeDateForPhase(timezone.TodayDate(), phase)
			review.LatestDate = phase.ServiceEndDate
		}
		reviews[row.ID] = review
	}
	active, err := s.CareOfferingRepo.ListActiveByPhaseIDs(ctx, phaseIDs)
	if err != nil {
		return nil, fmt.Errorf("load active offerings: %w", err)
	}
	activeByPhase := make(map[int64]map[int64]*enrollmentModels.CareOffering)
	allOfferingIDs := make([]int64, 0, len(active))
	for _, offering := range active {
		if offering == nil {
			continue
		}
		if activeByPhase[offering.PhaseID] == nil {
			activeByPhase[offering.PhaseID] = make(map[int64]*enrollmentModels.CareOffering)
		}
		activeByPhase[offering.PhaseID][offering.ID] = offering
		allOfferingIDs = append(allOfferingIDs, offering.ID)
	}
	current, err := s.RequestChildOfferingRepo.ListByRequestChildIDsAtDates(ctx, dates)
	if err != nil {
		return nil, fmt.Errorf("load current offerings: %w", err)
	}
	currentByChild := make(map[int64][]*enrollmentModels.RequestChildOffering)
	for _, link := range current {
		if link != nil {
			currentByChild[link.RequestChildID] = append(currentByChild[link.RequestChildID], link)
			allOfferingIDs = append(allOfferingIDs, link.CareOfferingID)
		}
	}
	requestedByRow := make(map[int64][]OfferingChangeSelection, len(rows))
	for _, row := range rows {
		if row == nil {
			continue
		}
		requested, payloadErr := selectionsFromPayload(row.Payload)
		if payloadErr != nil {
			continue
		}
		requestedByRow[row.ID] = requested
		for _, selection := range requested {
			allOfferingIDs = append(allOfferingIDs, selection.OfferingID)
		}
	}
	offerings, err := s.CareOfferingRepo.ListByIDs(ctx, allOfferingIDs)
	if err != nil {
		return nil, fmt.Errorf("load queue offerings: %w", err)
	}
	offeringsByID := make(map[int64]*enrollmentModels.CareOffering, len(offerings)+len(active))
	for _, offering := range offerings {
		if offering != nil {
			offeringsByID[offering.ID] = offering
		}
	}
	for _, offering := range active {
		if offering != nil {
			offeringsByID[offering.ID] = offering
		}
	}

	for _, row := range rows {
		if row == nil {
			continue
		}
		child := childrenByID[row.RequestChildID]
		requested, ok := requestedByRow[row.ID]
		if child == nil || !ok {
			continue
		}
		request := requestsByID[child.RequestID]
		if request == nil {
			continue
		}
		phase := phasesByID[request.PhaseID]
		if phase == nil {
			continue
		}
		allowed := make(map[int64]*enrollmentModels.CareOffering, len(activeByPhase[phase.ID])+len(currentByChild[child.ID]))
		for id, offering := range activeByPhase[phase.ID] {
			allowed[id] = offering
		}
		for _, link := range currentByChild[child.ID] {
			if offering := offeringsByID[link.CareOfferingID]; offering != nil {
				allowed[offering.ID] = offering
			}
		}
		_, submit, normalizeErr := normalizeOfferingSelections(requested, allowed)
		if normalizeErr != nil {
			continue
		}
		submit.TargetGradeLevel = child.TargetGradeLevel
		materialized, materializeErr := materializeAndValidateChildrenOfferingSelectionsGrandfathering(
			[]SubmitChild{submit}, allowed, phase.CareOfferingSelectionMode,
			grandfatheredOfferingsFromLinks(currentByChild[child.ID]),
		)
		if materializeErr != nil {
			continue
		}
		ids, currentByID, requestedByID := offeringChangeSides(currentByChild[child.ID], offeringChangeSelections(materialized[0]))
		entries := make([]OfferingChangeDiffEntry, 0, len(ids))
		changedByOfferingID := make(map[int64]bool, len(ids))
		for _, id := range ids {
			name := ""
			if offering := offeringsByID[id]; offering != nil {
				name = offering.Name
			}
			entry, changed := offeringDiffEntry(id, name, currentByID[id], requestedByID)
			changedByOfferingID[id] = changed
			entries = append(entries, entry)
		}
		annotateAutomaticShares(entries, materialized[0], offeringsByID)
		review := reviews[row.ID]
		if review == nil {
			continue
		}
		review.FullWithdrawal = leavesNoOfferings(entries)
		review.Unchanged = unchangedBookings(entries, changedByOfferingID)
		entries = slices.DeleteFunc(entries, func(entry OfferingChangeDiffEntry) bool {
			return !changedByOfferingID[entry.OfferingID] && len(entry.NewRuleDays) == 0
		})
		sort.SliceStable(entries, func(i, j int) bool { return entries[i].Label < entries[j].Label })
		review.Diff = entries
	}
	return reviews, nil
}

// leavesNoOfferings reports the Komplett-Abmeldung: the child holds bookings
// today and would hold none after the change. Judged on the full entry list
// (both sides of every offering), never on the changed lines alone — a request
// that drops one of two offerings also has only "removed" lines in its diff.
func leavesNoOfferings(entries []OfferingChangeDiffEntry) bool {
	held := false
	for _, entry := range entries {
		if entry.NewState == "booked" {
			return false
		}
		if entry.OldState == "booked" {
			held = true
		}
	}
	return held
}

// unchangedBookings returns the bookings the request leaves exactly as they
// are. Lines with a rule-added share stay in the diff itself (they carry the
// Mitbuchungs-Erklärung), so they are not repeated here.
func unchangedBookings(
	entries []OfferingChangeDiffEntry,
	changedByOfferingID map[int64]bool,
) []OfferingChangeDiffEntry {
	unchanged := make([]OfferingChangeDiffEntry, 0, len(entries))
	for _, entry := range entries {
		if changedByOfferingID[entry.OfferingID] || len(entry.NewRuleDays) > 0 {
			continue
		}
		if entry.NewState != "booked" {
			continue
		}
		unchanged = append(unchanged, entry)
	}
	sort.SliceStable(unchanged, func(i, j int) bool { return unchanged[i].Label < unchanged[j].Label })
	return unchanged
}

func (s *offeringChangeRequestService) Decide(ctx context.Context, input DecideOfferingChangeInput) error {
	if input.RequestID <= 0 || input.ReviewedBy <= 0 {
		return fmt.Errorf("%w: request and reviewer are required", ErrOfferingChangeInvalid)
	}
	reason := strings.TrimSpace(input.Reason)
	if utf8.RuneCountInString(reason) > offeringChangeMaxNoteLen {
		return fmt.Errorf("%w: reason is too long", ErrOfferingChangeInvalid)
	}
	if !input.Approve && reason == "" {
		// A rejection the family cannot understand generates the phone call the
		// whole feature exists to avoid.
		return fmt.Errorf("%w: a rejection needs a reason", ErrOfferingChangeInvalid)
	}
	row, err := s.ChangeRepo.FindByIDForUpdate(ctx, input.RequestID)
	if err != nil {
		return err
	}
	if row.IsTerminal() {
		return enrollmentModels.ErrOfferingChangeNotPending
	}
	student, err := s.StudentRepo.FindByIDForUpdate(ctx, row.StudentID)
	if err != nil {
		return fmt.Errorf("offering change: load student for decision: %w", err)
	}
	if student.IsAlumnus() {
		return enrollmentModels.ErrOfferingChangeNotFound
	}
	// The child left the OGS after filing this request; approving it would
	// book offerings for days they are no longer in care (#2487).
	if student.CareEndedOn(timezone.TodayDate()) {
		return enrollmentModels.ErrOfferingChangeNotFound
	}
	if ok, _ := authorize.CanUpdateStudent(ctx, jwt.PermissionsFromCtx(ctx), student, s.UserContext); !ok {
		return ErrOfferingChangeForbidden
	}
	if !input.Approve {
		if len(input.ExcludedAutoOfferingIDs) > 0 {
			return fmt.Errorf("%w: co-booking overrides only apply to an approval", ErrOfferingChangeInvalid)
		}
		if input.EffectiveFrom != nil {
			return fmt.Errorf("%w: a confirmed date only applies to an approval", ErrOfferingChangeInvalid)
		}
		diff, err := s.rejectionDecisionDiff(ctx, row)
		if err != nil {
			return err
		}
		if err := s.ChangeRepo.Decide(
			ctx, row.ID, enrollmentModels.OfferingChangeStatusRejected, &reason, &input.ReviewedBy, false,
		); err != nil {
			return err
		}
		if err := s.storeDecisionSnapshot(ctx, row.ID, diff); err != nil {
			return err
		}
		s.emitDecisionPill(ctx, row, input.ReviewedBy, offeringChangeRejectedBody,
			usersModels.ParentMessageRequestStatusRejected, reason, nil)
		return nil
	}
	applied, err := s.applyApproved(ctx, row, input)
	if err != nil {
		return err
	}
	diff := materializedDecisionDiff(
		applied.Before, applied.Selections, nil, applied.Offerings, applied.Overridden,
	)
	if err := s.ChangeRepo.Decide(
		ctx, row.ID, enrollmentModels.OfferingChangeStatusApproved, optionalString(reason), &input.ReviewedBy, true,
	); err != nil {
		return err
	}
	if err := s.storeDecisionSnapshot(ctx, row.ID, diff); err != nil {
		return err
	}
	s.emitDecisionPill(ctx, row, input.ReviewedBy, offeringChangeApprovedBody(row.EffectiveFrom),
		usersModels.ParentMessageRequestStatusDone, reason,
		map[string]any{"effective_from": row.EffectiveFrom.String()})
	s.Logger.Info("offering change request approved",
		slog.Int64("request_id", row.ID),
		slog.Int64("student_id", row.StudentID),
		slog.String("effective_from", row.EffectiveFrom.String()),
	)
	return nil
}

func (s *offeringChangeRequestService) PreviewDecision(
	ctx context.Context,
	requestID int64,
	excludedIDs []int64,
	effectiveFrom *timezone.Date,
) (*OfferingChangePreview, error) {
	if requestID <= 0 {
		return nil, fmt.Errorf("%w: request is required", ErrOfferingChangeInvalid)
	}
	row, err := s.ChangeRepo.FindByID(ctx, requestID)
	if err != nil {
		return nil, err
	}
	if row.IsTerminal() {
		return nil, enrollmentModels.ErrOfferingChangeNotPending
	}
	student, err := s.StudentRepo.FindByID(ctx, row.StudentID)
	if err != nil {
		return nil, fmt.Errorf("offering change: load student for preview: %w", err)
	}
	if student == nil || student.IsAlumnus() || student.CareEndedOn(timezone.TodayDate()) {
		return nil, enrollmentModels.ErrOfferingChangeNotFound
	}
	if ok, _ := authorize.CanUpdateStudent(ctx, jwt.PermissionsFromCtx(ctx), student, s.UserContext); !ok {
		return nil, ErrOfferingChangeForbidden
	}
	diff, err := s.decisionDiff(ctx, row, excludedIDs, effectiveFrom)
	if err != nil {
		return nil, err
	}
	// The preview is what the office decides from, so it has to fail on
	// everything the approval would fail on — at the same date (#2484).
	if err := s.assertApplicableAt(
		ctx, diff.phase, row.RequestChildID, diff.effectiveFrom, diff.requested, offeringIDSet(excludedIDs),
	); err != nil {
		return nil, err
	}
	ids, _, _ := offeringChangeSides(diff.current, offeringChangeSelections(diff.base))
	selectedByID := materializedSelectionPointers(diff.selected)
	preview := make([]OfferingChangePreviewSelection, 0, len(ids))
	for _, offeringID := range ids {
		selection := selectedByID[offeringID]
		if selection == nil {
			preview = append(preview, OfferingChangePreviewSelection{
				OfferingID: offeringID,
				State:      "removed",
			})
			continue
		}
		preview = append(preview, OfferingChangePreviewSelection{
			OfferingID: offeringID,
			State:      "booked",
			Days:       append([]string(nil), selection.SelectedDays...),
		})
	}
	conflicts, err := s.manualPlanningConflicts(ctx, row.StudentID, diff)
	if err != nil {
		return nil, err
	}
	if s.Settings == nil {
		return nil, fmt.Errorf("offering change: settings are required for preview")
	}
	arrivalExpectationsFollowBookings, err := s.Settings.ResolveBool(
		ctx, configModel.KeyEnrollmentBookingsAuthoritative,
	)
	if err != nil {
		return nil, fmt.Errorf("offering change: resolve booking authority for preview: %w", err)
	}
	return &OfferingChangePreview{
		Selections:                        preview,
		ManualPlanningConflicts:           conflicts,
		ArrivalExpectationsFollowBookings: arrivalExpectationsFollowBookings,
	}, nil
}

func (s *offeringChangeRequestService) manualPlanningConflicts(
	ctx context.Context,
	studentID int64,
	diff *offeringDecisionDiff,
) ([]ManualPlanningConflict, error) {
	if s.ImpactRepo == nil {
		return nil, fmt.Errorf("offering change: impact repository is required for preview")
	}
	occurrences, err := s.ImpactRepo.ListManualPlanningOccurrences(
		ctx, studentID, diff.effectiveFrom, diff.phase.ServiceEndDate,
	)
	if err != nil {
		return nil, fmt.Errorf("offering change: list manual planning conflicts: %w", err)
	}
	return aggregateManualPlanningConflicts(occurrences, diff), nil
}

func proposedCareCoversOccurrence(diff *offeringDecisionDiff, occurrence enrollmentModels.ManualPlanningOccurrence) bool {
	for _, selection := range diff.selected {
		if proposedSelectionCoversOccurrence(diff, selection, occurrence) {
			return true
		}
	}
	return false
}

func proposedSelectionCoversOccurrence(
	diff *offeringDecisionDiff,
	selection materializedOfferingSelection,
	occurrence enrollmentModels.ManualPlanningOccurrence,
) bool {
	if diff == nil || diff.phase == nil || occurrence.Date.Before(diff.effectiveFrom) || occurrence.Date.After(diff.phase.ServiceEndDate) {
		return false
	}
	offering := diff.offeringByID[selection.OfferingID]
	if offering == nil || !offering.CountsAsCare {
		return false
	}
	days := selection.SelectedDays
	if offering.DaysOfWeekMode == enrollmentModels.DaysOfWeekModeFixed {
		days = offering.AvailableDays
	}
	return slices.Contains(days, canonicalDayForWeekday(occurrence.Date.Weekday()))
}

func proposedLegacyPlanningCoversOccurrence(diff *offeringDecisionDiff, occurrence enrollmentModels.ManualPlanningOccurrence) bool {
	if diff == nil {
		return false
	}
	for _, selection := range diff.selected {
		offering := diff.offeringByID[selection.OfferingID]
		if offering == nil || offering.ActivityGroupID == nil || *offering.ActivityGroupID != occurrence.ActivityGroupID {
			continue
		}
		if proposedSelectionCoversOccurrence(diff, selection, occurrence) {
			return true
		}
	}
	return false
}

func aggregateManualPlanningConflicts(
	occurrences []enrollmentModels.ManualPlanningOccurrence,
	diff *offeringDecisionDiff,
) []ManualPlanningConflict {
	conflicts := make([]ManualPlanningConflict, 0)
	groupIndexes := make(map[int64]int)
	seenDays := make(map[int64]map[string]bool)
	for _, occurrence := range occurrences {
		day := canonicalDayForWeekday(occurrence.Date.Weekday())
		if proposedLegacyPlanningCoversOccurrence(diff, occurrence) || proposedCareCoversOccurrence(diff, occurrence) {
			continue
		}
		groupIndex, exists := groupIndexes[occurrence.ActivityGroupID]
		if !exists {
			conflicts = append(conflicts, ManualPlanningConflict{
				ActivityGroupID:   occurrence.ActivityGroupID,
				ActivityGroupName: occurrence.ActivityGroupName,
				FirstDate:         occurrence.Date,
			})
			groupIndex = len(conflicts) - 1
			groupIndexes[occurrence.ActivityGroupID] = groupIndex
			seenDays[occurrence.ActivityGroupID] = make(map[string]bool)
		}
		conflict := &conflicts[groupIndex]
		conflict.OccurrenceCount++
		if occurrence.Date.Before(conflict.FirstDate) {
			conflict.FirstDate = occurrence.Date
		}
		if !seenDays[occurrence.ActivityGroupID][day] {
			seenDays[occurrence.ActivityGroupID][day] = true
			conflict.Days = append(conflict.Days, day)
		}
	}
	for i := range conflicts {
		conflicts[i].Days = canonicalDays(conflicts[i].Days)
	}
	return conflicts
}

func canonicalDayForWeekday(weekday time.Weekday) string {
	return [...]string{"sun", "mon", "tue", "wed", "thu", "fri", "sat"}[weekday]
}

func (s *offeringChangeRequestService) rejectionDecisionDiff(
	ctx context.Context,
	row *enrollmentModels.OfferingChangeRequest,
) (*offeringDecisionDiff, error) {
	diff, err := s.decisionDiff(ctx, row, nil, nil)
	if err == nil {
		return diff, nil
	}
	fallback, fallbackErr := s.payloadDecisionDiff(ctx, row)
	if fallbackErr != nil {
		return nil, fmt.Errorf("offering change: build rejection snapshot: %v; payload fallback: %w", err, fallbackErr)
	}
	s.Logger.Warn("offering change: rejection snapshot uses request payload",
		slog.Int64("request_id", row.ID),
		slog.String("materialization_error", err.Error()),
	)
	return fallback, nil
}

func (s *offeringChangeRequestService) payloadDecisionDiff(
	ctx context.Context,
	row *enrollmentModels.OfferingChangeRequest,
) (*offeringDecisionDiff, error) {
	requested, err := selectionsFromPayload(row.Payload)
	if err != nil {
		return nil, err
	}
	current, err := s.RequestChildOfferingRepo.ListByRequestChildIDAtDate(
		ctx, row.RequestChildID, appliedOfferingChangeDate(row.EffectiveFrom),
	)
	if err != nil {
		return nil, fmt.Errorf("list current offerings: %w", err)
	}
	current = explicitOfferingLinks(current)
	ids, currentByID, requestedByID := offeringChangeSides(current, requested)
	offerings, err := s.CareOfferingRepo.ListByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("list snapshot offerings: %w", err)
	}
	entries := offeringDiffEntries(ids, offeringsByID(offerings), currentByID, requestedByID)
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].Label < entries[j].Label })
	return &offeringDecisionDiff{entries: entries, current: current}, nil
}

func explicitOfferingLinks(
	links []*enrollmentModels.RequestChildOffering,
) []*enrollmentModels.RequestChildOffering {
	explicit := make([]*enrollmentModels.RequestChildOffering, 0, len(links))
	for _, link := range links {
		if link == nil || (len(link.ManualSelectedDays) == 0 && len(link.AutomaticSelectedDays) > 0) {
			continue
		}
		clone := *link
		if len(link.AutomaticSelectedDays) > 0 {
			clone.SelectedDays = append([]string(nil), link.ManualSelectedDays...)
		}
		explicit = append(explicit, &clone)
	}
	return explicit
}

// storeDecisionSnapshot freezes the diff on the decided row (ADR 0002).
func (s *offeringChangeRequestService) storeDecisionSnapshot(
	ctx context.Context,
	requestID int64,
	diff *offeringDecisionDiff,
) error {
	if diff == nil {
		return fmt.Errorf("offering change: decision snapshot diff is required")
	}
	snapshot := &enrollmentModels.OfferingChangeDecisionSnapshot{
		Diff:                make([]enrollmentModels.OfferingChangeSnapshotEntry, 0, len(diff.entries)),
		OverriddenOfferings: diff.overridden,
	}
	for _, entry := range diff.entries {
		snapshot.Diff = append(snapshot.Diff, enrollmentModels.OfferingChangeSnapshotEntry{
			OfferingID:       entry.OfferingID,
			Label:            entry.Label,
			OldState:         entry.OldState,
			OldDays:          entry.OldDays,
			NewState:         entry.NewState,
			NewDays:          entry.NewDays,
			NewAutomaticDays: entry.NewAutomaticDays,
			NewRuleDays:      entry.NewRuleDays,
			AutoTriggerNames: entry.AutoTriggerNames,
		})
	}
	if err := s.ChangeRepo.UpdateDecisionSnapshot(ctx, requestID, snapshot); err != nil {
		return fmt.Errorf("offering change: store decision snapshot: %w", err)
	}
	return nil
}

// diffEntriesFromSnapshot maps the frozen model entries back into the service
// diff shape shared by the pending and recap views.
func diffEntriesFromSnapshot(entries []enrollmentModels.OfferingChangeSnapshotEntry) []OfferingChangeDiffEntry {
	out := make([]OfferingChangeDiffEntry, 0, len(entries))
	for _, entry := range entries {
		out = append(out, OfferingChangeDiffEntry{
			OfferingID:       entry.OfferingID,
			Label:            entry.Label,
			OldState:         entry.OldState,
			OldDays:          entry.OldDays,
			NewState:         entry.NewState,
			NewDays:          entry.NewDays,
			NewAutomaticDays: entry.NewAutomaticDays,
			NewRuleDays:      entry.NewRuleDays,
			AutoTriggerNames: entry.AutoTriggerNames,
		})
	}
	return out
}

// applyApproved re-validates the request against today's catalog and capacity
// and then performs the dated switch. Re-validation is the point: the request
// may be weeks old, and an offering can have been deactivated or filled up in
// the meantime. A failure here leaves the row pending, so the office sees an
// actionable error instead of a request marked done that never applied.
func (s *offeringChangeRequestService) applyApproved(
	ctx context.Context,
	row *enrollmentModels.OfferingChangeRequest,
	input DecideOfferingChangeInput,
) (*appliedOfferingAdjustment, error) {
	if s.Settings == nil {
		return nil, ErrCareOfferingsDisabled
	}
	careOfferingsEnabled, err := s.Settings.ResolveBool(ctx, configModel.KeyEnrollmentCareOfferingsEnabled)
	if err != nil {
		return nil, fmt.Errorf("offering change: resolve care offerings setting: %w", err)
	}
	if !careOfferingsEnabled {
		return nil, ErrCareOfferingsDisabled
	}
	child, err := s.RequestChildRepo.FindByID(ctx, row.RequestChildID)
	if err != nil || child == nil {
		return nil, fmt.Errorf("offering change: load request child: %w", err)
	}
	request, err := s.RequestRepo.FindByID(ctx, child.RequestID)
	if err != nil || request == nil {
		return nil, fmt.Errorf("offering change: load request: %w", err)
	}
	phase, err := s.PhaseRepo.FindByID(ctx, request.PhaseID)
	if err != nil || phase == nil {
		return nil, fmt.Errorf("offering change: load phase: %w", err)
	}
	selections, err := selectionsFromPayload(row.Payload)
	if err != nil {
		return nil, err
	}
	// A request whose date has passed while it waited applies as soon as
	// possible instead of being refused: the family asked for a change, the
	// office agreed, and a date in the past would be rejected by the adjustment
	// validator. A date the office confirmed itself is never moved — see
	// confirmedEffectiveFrom.
	effectiveFrom, err := confirmedEffectiveFrom(input.EffectiveFrom, row.EffectiveFrom, phase)
	if err != nil {
		return nil, err
	}
	if effectiveFrom.After(phase.ServiceEndDate) {
		return nil, fmt.Errorf("%w: the care period ended before this request was decided", ErrOfferingChangeInvalid)
	}
	student, err := s.StudentRepo.FindByIDForUpdate(ctx, row.StudentID)
	if err != nil {
		return nil, fmt.Errorf("offering change: load student for effective date: %w", err)
	}
	if student.EnrolledUntil != nil && effectiveFrom.After(*student.EnrolledUntil) {
		return nil, fmt.Errorf("%w: care ends before the approved effective date", ErrOfferingChangeInvalid)
	}
	excluded := offeringIDSet(input.ExcludedAutoOfferingIDs)
	if err := s.assertApplicableAt(ctx, phase, row.RequestChildID, effectiveFrom, selections, excluded); err != nil {
		return nil, err
	}
	// Keep the actual date in memory for the adjustment audit. Persist it only
	// after the switch succeeds, so a client error leaves the pending row intact.
	requestedFrom := row.EffectiveFrom
	row.EffectiveFrom = effectiveFrom
	reason := offeringChangeAdjustmentReason(row, requestedFrom, input.Reason)
	adjustment := UpdateChildOfferingsInput{
		RequestID:                request.ID,
		ChildID:                  row.RequestChildID,
		Offerings:                adjustmentSelections(selections),
		Reason:                   reason,
		ActorAccountID:           input.ReviewedBy,
		ActorRole:                cmpOr(strings.TrimSpace(input.ActorRole), "admin"),
		EffectiveFrom:            &effectiveFrom,
		ExcludedAutoAddTargetIDs: excluded,
	}
	applied, err := s.Applier.applyApprovedChangeRequestOfferingsWithResult(ctx, adjustment)
	if err != nil {
		return nil, fmt.Errorf("offering change: apply: %w", err)
	}
	if err := s.ChangeRepo.UpdateEffectiveFrom(ctx, row.ID, effectiveFrom); err != nil {
		return nil, fmt.Errorf("offering change: update applied effective date: %w", err)
	}
	return applied, nil
}

// confirmedEffectiveFrom resolves the date an approval applies on. Without a
// date from the reviewer the guardian's date is moved forward as before. A date
// the reviewer confirmed is never moved: one outside the selectable range is
// refused, so the office never applies a switch on a different day than the one
// it just confirmed (#2484).
func confirmedEffectiveFrom(
	confirmed *timezone.Date,
	requested timezone.Date,
	phase *enrollmentModels.Phase,
) (timezone.Date, error) {
	if confirmed == nil {
		return appliedOfferingChangeDateForPhase(requested, phase), nil
	}
	earliest := appliedOfferingChangeDateForPhase(timezone.TodayDate(), phase)
	if confirmed.Before(earliest) {
		return timezone.Date{}, fmt.Errorf("%w: %s is before %s",
			ErrOfferingChangeDateOutOfRange, confirmed, earliest)
	}
	if phase != nil && confirmed.After(phase.ServiceEndDate) {
		return timezone.Date{}, fmt.Errorf("%w: %s is after the care period ends on %s",
			ErrOfferingChangeDateOutOfRange, confirmed, phase.ServiceEndDate)
	}
	return *confirmed, nil
}

// assertApplicableAt re-runs the checks an approval performs, without writing
// anything: the selection has to be valid in the phase catalog on that date and
// every offering has to have a free slot. Preview and approval share it, so a
// preview that shows a switch is a switch the approval can actually make.
func (s *offeringChangeRequestService) assertApplicableAt(
	ctx context.Context,
	phase *enrollmentModels.Phase,
	requestChildID int64,
	effectiveFrom timezone.Date,
	selections []OfferingChangeSelection,
	excluded map[int64]bool,
) error {
	if _, err := s.validateSelections(ctx, phase, requestChildID, effectiveFrom, selections); err != nil {
		return err
	}
	return s.assertCapacityAvailable(ctx, phase, requestChildID, effectiveFrom, selections, excluded)
}

func appliedOfferingChangeDate(effectiveFrom timezone.Date) timezone.Date {
	if today := timezone.TodayDate(); effectiveFrom.Before(today) {
		return today
	}
	return effectiveFrom
}

func appliedOfferingChangeDateForPhase(effectiveFrom timezone.Date, phase *enrollmentModels.Phase) timezone.Date {
	effectiveFrom = appliedOfferingChangeDate(effectiveFrom)
	if phase != nil && effectiveFrom.Before(phase.ServiceStartDate) {
		return phase.ServiceStartDate
	}
	return effectiveFrom
}

// assertCapacityAvailable refuses an approval that would overbook an offering.
// It counts every selected offering after removing this child's old intervals,
// then reserves one replacement interval through the end of the care period.
func (s *offeringChangeRequestService) assertCapacityAvailable(
	ctx context.Context,
	phase *enrollmentModels.Phase,
	requestChildID int64,
	effectiveFrom timezone.Date,
	selections []OfferingChangeSelection,
	excluded map[int64]bool,
) error {
	current, err := s.RequestChildOfferingRepo.ListByRequestChildIDAtDate(ctx, requestChildID, effectiveFrom)
	if err != nil {
		return fmt.Errorf("offering change: list current offerings: %w", err)
	}
	held := heldOfferingIDs(current)
	replacementOfferingIDs, err := s.materializedOfferingIDs(ctx, phase, requestChildID, effectiveFrom, selections, excluded)
	if err != nil {
		return err
	}
	allOfferingIDs := append([]int64(nil), replacementOfferingIDs...)
	// A concurrent approval that releases one of this child's current offerings
	// must hold that offering's row lock too. Otherwise this approval can count
	// the still-occupied row before the releasing transaction commits and reject
	// a slot that is about to become available.
	for offeringID := range held {
		allOfferingIDs = append(allOfferingIDs, offeringID)
	}
	for offeringID := range excluded {
		allOfferingIDs = append(allOfferingIDs, offeringID)
	}
	sort.Slice(allOfferingIDs, func(i, j int) bool { return allOfferingIDs[i] < allOfferingIDs[j] })
	allOfferingIDs = slices.Compact(allOfferingIDs)
	locked, err := s.CareOfferingRepo.ListByIDsForUpdate(ctx, allOfferingIDs)
	if err != nil {
		return fmt.Errorf("offering change: lock offering capacity: %w", err)
	}
	lockedByID := offeringsByID(locked)
	phaseEndExclusive := phase.ServiceEndDate.AddDays(1)
	for _, offeringID := range replacementOfferingIDs {
		offering := lockedByID[offeringID]
		if offering == nil || (!held[offeringID] && !offering.IsActive) {
			return fmt.Errorf("%w: care offering %d is unavailable", ErrOfferingChangeInvalid, offeringID)
		}
		if heldOfferingCoversRange(current, offeringID, phaseEndExclusive) {
			continue
		}
		if offering.Capacity == nil {
			continue
		}
		taken, countErr := s.RequestChildOfferingRepo.CountMaxActiveByCareOfferingInRangeExcludingRequestChild(
			ctx, offering.ID, requestChildID, effectiveFrom, phaseEndExclusive,
		)
		if countErr != nil {
			return fmt.Errorf("offering change: count offering occupancy: %w", countErr)
		}
		if taken >= *offering.Capacity {
			return fmt.Errorf("%w: %s", ErrOfferingChangeCapacityFull, offering.Name)
		}
	}
	return nil
}

func heldOfferingCoversRange(
	links []*enrollmentModels.RequestChildOffering,
	offeringID int64,
	until timezone.Date,
) bool {
	for _, link := range links {
		if link != nil && link.CareOfferingID == offeringID &&
			(link.ValidUntil == nil || !link.ValidUntil.Before(until)) {
			return true
		}
	}
	return false
}

// materializedOfferingIDs includes required and auto-added offerings, which
// are not present in a guardian's raw selection but consume capacity too.
func (s *offeringChangeRequestService) materializedOfferingIDs(
	ctx context.Context,
	phase *enrollmentModels.Phase,
	requestChildID int64,
	effectiveFrom timezone.Date,
	selections []OfferingChangeSelection,
	excluded map[int64]bool,
) ([]int64, error) {
	materialized, err := s.materializedSelectionsExcluding(
		ctx, phase, requestChildID, effectiveFrom, selections, excluded,
	)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(materialized))
	for _, selection := range materialized {
		ids = append(ids, selection.OfferingID)
	}
	return ids, nil
}

func heldOfferingIDs(links []*enrollmentModels.RequestChildOffering) map[int64]bool {
	held := make(map[int64]bool, len(links))
	for _, link := range links {
		if link != nil {
			held[link.CareOfferingID] = true
		}
	}
	return held
}

// withoutAutomaticSelections ignores guardian input for offerings that are
// currently derived from another selection. Automatic offerings are visible in
// the form but cannot be independently changed or turned into manual bookings.
func withoutAutomaticSelections(current []*enrollmentModels.RequestChildOffering, selections []OfferingChangeSelection) []OfferingChangeSelection {
	automatic := make(map[int64]*enrollmentModels.RequestChildOffering, len(current))
	for _, link := range current {
		if link != nil && len(link.ManualSelectedDays) == 0 && len(link.AutomaticSelectedDays) > 0 {
			automatic[link.CareOfferingID] = link
		}
	}
	manual := make([]OfferingChangeSelection, 0, len(selections))
	for _, selection := range selections {
		if automatic[selection.OfferingID] != nil {
			continue
		}
		manual = append(manual, selection)
	}
	return manual
}

func sameMaterializedOfferingSelections(current []*enrollmentModels.RequestChildOffering, selections []materializedOfferingSelection) bool {
	if len(current) != len(selections) {
		return false
	}
	byID := make(map[int64]*enrollmentModels.RequestChildOffering, len(current))
	for _, link := range current {
		if link == nil {
			return false
		}
		byID[link.CareOfferingID] = link
	}
	for _, selection := range selections {
		link := byID[selection.OfferingID]
		if link == nil || !slices.Equal(canonicalDays(link.SelectedDays), canonicalDays(selection.SelectedDays)) {
			return false
		}
	}
	return true
}

func (s *offeringChangeRequestService) materializedSelections(
	ctx context.Context,
	phase *enrollmentModels.Phase,
	requestChildID int64,
	effectiveFrom timezone.Date,
	selections []OfferingChangeSelection,
) ([]materializedOfferingSelection, error) {
	return s.materializedSelectionsExcluding(ctx, phase, requestChildID, effectiveFrom, selections, nil)
}

// materializedSelectionsExcluding is the variant behind a staff approval with
// per-request Mitbuchungs-Regel overrides (#2370): excluded target offerings
// keep their manual days but gain no rule-derived ones.
func (s *offeringChangeRequestService) materializedSelectionsExcluding(
	ctx context.Context,
	phase *enrollmentModels.Phase,
	requestChildID int64,
	effectiveFrom timezone.Date,
	selections []OfferingChangeSelection,
	excluded map[int64]bool,
) ([]materializedOfferingSelection, error) {
	active, err := s.CareOfferingRepo.ListActiveByPhase(ctx, phase.ID)
	if err != nil {
		return nil, fmt.Errorf("offering change: list active offerings: %w", err)
	}
	allowed := offeringsByID(active)
	current, err := s.addHeldOfferingsAtDate(ctx, requestChildID, effectiveFrom, allowed)
	if err != nil {
		return nil, err
	}
	_, submit, err := normalizeOfferingSelections(selections, allowed)
	if err != nil {
		return nil, err
	}
	child, err := s.RequestChildRepo.FindByID(ctx, requestChildID)
	if err != nil || child == nil {
		return nil, fmt.Errorf("offering change: load request child: %w", err)
	}
	submit.TargetGradeLevel = child.TargetGradeLevel
	submit.ExcludedAutoAddTargetIDs = excluded
	materialized, err := materializeAndValidateChildrenOfferingSelectionsGrandfathering(
		[]SubmitChild{submit}, allowed, phase.CareOfferingSelectionMode,
		grandfatheredOfferingsFromLinks(current),
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrOfferingChangeInvalid, err)
	}
	return materialized[0], nil
}

// validateSelections checks the desired selection against the phase catalog and
// its selection rules using the very same materializer the enrollment form and
// the admin adjustment use, so a request cannot be accepted that an approval
// would then refuse.
func (s *offeringChangeRequestService) validateSelections(
	ctx context.Context,
	phase *enrollmentModels.Phase,
	requestChildID int64,
	effectiveFrom timezone.Date,
	selections []OfferingChangeSelection,
) ([]OfferingChangeSelection, error) {
	active, err := s.CareOfferingRepo.ListActiveByPhase(ctx, phase.ID)
	if err != nil {
		return nil, fmt.Errorf("offering change: list active offerings: %w", err)
	}
	allowed := offeringsByID(active)
	// Offerings the child already holds stay selectable even when the admin
	// deactivated them in the catalog: keeping the status quo must never be
	// blocked by a catalog edit.
	current, err := s.addHeldOfferingsAtDate(ctx, requestChildID, effectiveFrom, allowed)
	if err != nil {
		return nil, err
	}
	normalized, submit, err := normalizeOfferingSelections(selections, allowed)
	if err != nil {
		return nil, err
	}
	child, err := s.RequestChildRepo.FindByID(ctx, requestChildID)
	if err != nil || child == nil {
		return nil, fmt.Errorf("offering change: load request child: %w", err)
	}
	submit.TargetGradeLevel = child.TargetGradeLevel
	_, err = materializeAndValidateChildrenOfferingSelectionsGrandfathering(
		[]SubmitChild{submit}, allowed, phase.CareOfferingSelectionMode,
		grandfatheredOfferingsFromLinks(current),
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrOfferingChangeInvalid, err)
	}
	// Keep the guardian's explicit selection in the request payload. The
	// materializer adds dependent offerings for validation and capacity, but
	// storing those additions would turn them into manual bookings on approval.
	sort.SliceStable(normalized, func(i, j int) bool {
		return normalized[i].OfferingID < normalized[j].OfferingID
	})
	return normalized, nil
}

func offeringsByID(offerings []*enrollmentModels.CareOffering) map[int64]*enrollmentModels.CareOffering {
	byID := make(map[int64]*enrollmentModels.CareOffering, len(offerings))
	for _, offering := range offerings {
		if offering != nil {
			byID[offering.ID] = offering
		}
	}
	return byID
}

func (s *offeringChangeRequestService) addHeldOfferingsAtDate(
	ctx context.Context,
	requestChildID int64,
	onDate timezone.Date,
	allowed map[int64]*enrollmentModels.CareOffering,
) ([]*enrollmentModels.RequestChildOffering, error) {
	if requestChildID <= 0 {
		return nil, nil
	}
	current, err := s.RequestChildOfferingRepo.ListByRequestChildIDAtDate(ctx, requestChildID, onDate)
	if err != nil {
		return nil, fmt.Errorf("offering change: list current offerings: %w", err)
	}
	heldIDs := make([]int64, 0, len(current))
	for _, link := range current {
		if link != nil && allowed[link.CareOfferingID] == nil {
			heldIDs = append(heldIDs, link.CareOfferingID)
		}
	}
	if len(heldIDs) == 0 {
		return current, nil
	}
	held, err := s.CareOfferingRepo.ListByIDs(ctx, heldIDs)
	if err != nil {
		return nil, fmt.Errorf("offering change: list held offerings: %w", err)
	}
	for id, offering := range offeringsByID(held) {
		allowed[id] = offering
	}
	return current, nil
}

func normalizeOfferingSelections(
	selections []OfferingChangeSelection,
	allowed map[int64]*enrollmentModels.CareOffering,
) ([]OfferingChangeSelection, SubmitChild, error) {
	normalized := make([]OfferingChangeSelection, 0, len(selections))
	submit := SubmitChild{
		OfferingIDs:  make([]int64, 0, len(selections)),
		OfferingDays: make([]SubmitOfferingDays, 0, len(selections)),
	}
	seen := make(map[int64]bool, len(selections))
	for _, selection := range selections {
		if selection.OfferingID <= 0 {
			return nil, SubmitChild{}, fmt.Errorf("%w: offering id is required", ErrOfferingChangeInvalid)
		}
		if seen[selection.OfferingID] {
			continue
		}
		if allowed[selection.OfferingID] == nil {
			return nil, SubmitChild{}, fmt.Errorf("%w: care offering %d cannot be booked for this child", ErrOfferingChangeInvalid, selection.OfferingID)
		}
		seen[selection.OfferingID] = true
		for _, day := range selection.SelectedDays {
			normalizedDay := strings.ToLower(strings.TrimSpace(day))
			if normalizedDay == "" {
				continue
			}
			if _, ok := enrollmentModels.CanonicalDayToISOWeekday(normalizedDay); !ok {
				return nil, SubmitChild{}, fmt.Errorf("%w: unknown selected day %q", ErrOfferingChangeInvalid, day)
			}
		}
		days := canonicalDays(selection.SelectedDays)
		normalized = append(normalized, OfferingChangeSelection{
			OfferingID:   selection.OfferingID,
			SelectedDays: days,
		})
		submit.OfferingIDs = append(submit.OfferingIDs, selection.OfferingID)
		if len(days) > 0 {
			submit.OfferingDays = append(submit.OfferingDays, SubmitOfferingDays{
				OfferingID:   selection.OfferingID,
				SelectedDays: days,
			})
		}
	}
	return normalized, submit, nil
}

// diffForRequest renders "current → requested" per offering, including the ones
// that are dropped, so a reviewer never has to open two screens.
func (s *offeringChangeRequestService) diffForRequest(
	ctx context.Context,
	row *enrollmentModels.OfferingChangeRequest,
) ([]OfferingChangeDiffEntry, error) {
	diff, err := s.decisionDiff(ctx, row, nil, nil)
	if err != nil {
		return nil, err
	}
	return diff.entries, nil
}

// offeringDecisionDiff is a request's review diff plus the override bookkeeping
// a staff approval with exclusions needs.
type offeringDecisionDiff struct {
	entries      []OfferingChangeDiffEntry
	overridden   []enrollmentModels.OfferingChangeSnapshotOffering
	current      []*enrollmentModels.RequestChildOffering
	base         []materializedOfferingSelection
	selected     []materializedOfferingSelection
	offeringByID map[int64]*enrollmentModels.CareOffering
	// phase, requested and effectiveFrom are the context the materialization
	// ran against, so a caller can re-check applicability without resolving the
	// same aggregate twice.
	phase         *enrollmentModels.Phase
	requested     []OfferingChangeSelection
	effectiveFrom timezone.Date
}

type offeringDecisionMaterialization struct {
	row       *enrollmentModels.OfferingChangeRequest
	base      []materializedOfferingSelection
	selected  []materializedOfferingSelection
	phase     *enrollmentModels.Phase
	requested []OfferingChangeSelection
}

// decisionDiff builds the "current → requested" diff. With exclusions it first
// validates each excluded id against the UNexcluded materialization — only a
// target that actually gains rule-derived days can be overridden (#2370) — and
// then diffs the materialization with the Mitbuchungs-Regel switched off for
// those targets.
func (s *offeringChangeRequestService) decisionDiff(
	ctx context.Context,
	row *enrollmentModels.OfferingChangeRequest,
	excludedIDs []int64,
	effectiveFrom *timezone.Date,
) (*offeringDecisionDiff, error) {
	materialization, err := s.materializeDecisionSelections(ctx, row, excludedIDs, effectiveFrom)
	if err != nil {
		return nil, err
	}
	current, err := s.RequestChildOfferingRepo.ListByRequestChildIDAtDate(
		ctx, materialization.row.RequestChildID, materialization.row.EffectiveFrom,
	)
	if err != nil {
		return nil, fmt.Errorf("offering change: list current offerings: %w", err)
	}
	ids, currentByID, requestedByID := offeringChangeSides(current, offeringChangeSelections(materialization.selected))
	diff, err := s.buildDecisionDiff(
		ctx, excludedIDs, current, materialization.base, materialization.selected, ids, currentByID, requestedByID,
	)
	if err != nil {
		return nil, err
	}
	diff.phase = materialization.phase
	diff.requested = materialization.requested
	diff.effectiveFrom = materialization.row.EffectiveFrom
	return diff, nil
}

func (s *offeringChangeRequestService) materializeDecisionSelections(
	ctx context.Context,
	row *enrollmentModels.OfferingChangeRequest,
	excludedIDs []int64,
	effectiveFrom *timezone.Date,
) (*offeringDecisionMaterialization, error) {
	rowCopy := *row
	requested, phase, err := s.decisionRequestContext(ctx, &rowCopy)
	if err != nil {
		return nil, err
	}
	rowCopy.EffectiveFrom, err = confirmedEffectiveFrom(effectiveFrom, rowCopy.EffectiveFrom, phase)
	if err != nil {
		return nil, err
	}
	base, err := s.materializedSelections(ctx, phase, rowCopy.RequestChildID, rowCopy.EffectiveFrom, requested)
	if err != nil {
		return nil, err
	}
	result := &offeringDecisionMaterialization{
		row: &rowCopy, base: base, selected: base, phase: phase, requested: requested,
	}
	excluded := offeringIDSet(excludedIDs)
	if len(excluded) == 0 {
		return result, nil
	}
	result.selected, err = s.materializedSelectionsExcluding(
		ctx, phase, rowCopy.RequestChildID, rowCopy.EffectiveFrom, requested, excluded,
	)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *offeringChangeRequestService) decisionRequestContext(
	ctx context.Context,
	row *enrollmentModels.OfferingChangeRequest,
) ([]OfferingChangeSelection, *enrollmentModels.Phase, error) {
	requested, err := selectionsFromPayload(row.Payload)
	if err != nil {
		return nil, nil, err
	}
	child, err := s.RequestChildRepo.FindByID(ctx, row.RequestChildID)
	if err != nil || child == nil {
		return nil, nil, fmt.Errorf("offering change: load request child for diff: %w", err)
	}
	request, err := s.RequestRepo.FindByID(ctx, child.RequestID)
	if err != nil || request == nil {
		return nil, nil, fmt.Errorf("offering change: load request for diff: %w", err)
	}
	phase, err := s.PhaseRepo.FindByID(ctx, request.PhaseID)
	if err != nil || phase == nil {
		return nil, nil, fmt.Errorf("offering change: load phase for diff: %w", err)
	}
	return requested, phase, nil
}

func offeringIDSet(ids []int64) map[int64]bool {
	result := make(map[int64]bool, len(ids))
	for _, id := range ids {
		result[id] = true
	}
	return result
}

func appendMissingOfferingIDs(ids []int64, selections []materializedOfferingSelection) []int64 {
	// The base materialization can contain offerings the excluded diff no longer
	// has; their names are still needed for the override record.
	result := append([]int64(nil), ids...)
	for _, selection := range selections {
		if !slices.Contains(result, selection.OfferingID) {
			result = append(result, selection.OfferingID)
		}
	}
	return result
}

func (s *offeringChangeRequestService) buildDecisionDiff(
	ctx context.Context,
	excludedIDs []int64,
	current []*enrollmentModels.RequestChildOffering,
	base, materialized []materializedOfferingSelection,
	ids []int64,
	currentByID map[int64]*enrollmentModels.RequestChildOffering,
	requestedByID map[int64]OfferingChangeSelection,
) (*offeringDecisionDiff, error) {
	offeringIDs := appendMissingOfferingIDs(ids, base)
	offerings, err := s.CareOfferingRepo.ListByIDs(ctx, offeringIDs)
	if err != nil {
		return nil, fmt.Errorf("offering change: list offerings for diff: %w", err)
	}
	offeringByID := offeringsByID(offerings)
	overridden, err := validateExcludedAutoTargets(excludedIDs, base, offeringByID)
	if err != nil {
		return nil, err
	}
	return materializedDecisionDiffFromSides(
		currentByID, requestedByID, ids, current, base, materialized, offeringByID, overridden,
	), nil
}

func materializedDecisionDiff(
	current []*enrollmentModels.RequestChildOffering,
	materialized, base []materializedOfferingSelection,
	offeringByID map[int64]*enrollmentModels.CareOffering,
	overridden []enrollmentModels.OfferingChangeSnapshotOffering,
) *offeringDecisionDiff {
	ids, currentByID, requestedByID := offeringChangeSides(current, offeringChangeSelections(materialized))
	return materializedDecisionDiffFromSides(
		currentByID, requestedByID, ids, current, base, materialized, offeringByID, overridden,
	)
}

func materializedDecisionDiffFromSides(
	currentByID map[int64]*enrollmentModels.RequestChildOffering,
	requestedByID map[int64]OfferingChangeSelection,
	ids []int64,
	current []*enrollmentModels.RequestChildOffering,
	base, materialized []materializedOfferingSelection,
	offeringByID map[int64]*enrollmentModels.CareOffering,
	overridden []enrollmentModels.OfferingChangeSnapshotOffering,
) *offeringDecisionDiff {
	entries := offeringDiffEntries(ids, offeringByID, currentByID, requestedByID)
	annotateAutomaticShares(entries, materialized, offeringByID)
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].Label < entries[j].Label })
	return &offeringDecisionDiff{
		entries: entries, overridden: overridden, current: current, base: base,
		selected: materialized, offeringByID: offeringByID,
	}
}

func offeringDiffEntries(
	ids []int64,
	offeringByID map[int64]*enrollmentModels.CareOffering,
	currentByID map[int64]*enrollmentModels.RequestChildOffering,
	requestedByID map[int64]OfferingChangeSelection,
) []OfferingChangeDiffEntry {
	entries := make([]OfferingChangeDiffEntry, 0, len(ids))
	for _, id := range ids {
		name := ""
		if offering := offeringByID[id]; offering != nil {
			name = offering.Name
		}
		entry, changed := offeringDiffEntry(id, name, currentByID[id], requestedByID)
		if !changed {
			continue
		}
		entries = append(entries, entry)
	}
	return entries
}

// validateExcludedAutoTargets accepts an exclusion only for an offering that
// the unexcluded materialization marks as a Mitbuchungs-Regel target with
// rule-derived days. Anything else — a parent-chosen offering, a pure
// required-lunch derivation, an unknown id — cannot be overridden away.
func validateExcludedAutoTargets(
	excludedIDs []int64,
	base []materializedOfferingSelection,
	offeringByID map[int64]*enrollmentModels.CareOffering,
) ([]enrollmentModels.OfferingChangeSnapshotOffering, error) {
	if len(excludedIDs) == 0 {
		return nil, nil
	}
	baseByID := materializedSelectionPointers(base)
	overridden := make([]enrollmentModels.OfferingChangeSnapshotOffering, 0, len(excludedIDs))
	seen := make(map[int64]bool, len(excludedIDs))
	for _, id := range excludedIDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		sel, ok := baseByID[id]
		offering := offeringByID[id]
		if !ok || offering == nil || len(ruleContributionForTarget(offering, sel, baseByID, offeringByID)) == 0 {
			return nil, fmt.Errorf(
				"%w: offering %d is not added by a co-booking rule and cannot be excluded", ErrOfferingChangeInvalid, id,
			)
		}
		overridden = append(overridden, enrollmentModels.OfferingChangeSnapshotOffering{
			OfferingID: id,
			Name:       offering.Name,
		})
	}
	return overridden, nil
}

func offeringChangeSelections(materialized []materializedOfferingSelection) []OfferingChangeSelection {
	selections := make([]OfferingChangeSelection, 0, len(materialized))
	for _, selection := range materialized {
		selections = append(selections, OfferingChangeSelection{
			OfferingID:   selection.OfferingID,
			SelectedDays: append([]string(nil), selection.SelectedDays...),
		})
	}
	return selections
}

func offeringChangeSides(
	current []*enrollmentModels.RequestChildOffering,
	requested []OfferingChangeSelection,
) (
	[]int64,
	map[int64]*enrollmentModels.RequestChildOffering,
	map[int64]OfferingChangeSelection,
) {
	ids := make([]int64, 0, len(current)+len(requested))
	seen := make(map[int64]bool, len(current)+len(requested))
	addID := func(id int64) {
		if id > 0 && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	currentByID := make(map[int64]*enrollmentModels.RequestChildOffering, len(current))
	for _, link := range current {
		if link != nil {
			currentByID[link.CareOfferingID] = link
			addID(link.CareOfferingID)
		}
	}
	requestedByID := make(map[int64]OfferingChangeSelection, len(requested))
	for _, selection := range requested {
		requestedByID[selection.OfferingID] = selection
		addID(selection.OfferingID)
	}
	return ids, currentByID, requestedByID
}

// annotateAutomaticShares marks the diff lines whose NEW side carries days a
// Mitbuchungs-Regel (or the required-lunch derivation) added (#2365). The
// trigger names come from the offerings that are actually part of the
// materialized selection, so the card can say WHY a line appeared.
func annotateAutomaticShares(
	entries []OfferingChangeDiffEntry,
	materialized []materializedOfferingSelection,
	offeringByID map[int64]*enrollmentModels.CareOffering,
) {
	selByID := materializedSelectionPointers(materialized)
	for i := range entries {
		annotateAutomaticShare(&entries[i], selByID, offeringByID)
	}
}

func annotateAutomaticShare(
	entry *OfferingChangeDiffEntry,
	selections map[int64]*materializedOfferingSelection,
	offerings map[int64]*enrollmentModels.CareOffering,
) {
	selection, ok := selections[entry.OfferingID]
	if !ok || selection == nil || len(selection.AutomaticSelectedDays) == 0 {
		return
	}
	entry.NewAutomaticDays = append([]string(nil), selection.AutomaticSelectedDays...)
	target := offerings[entry.OfferingID]
	if target == nil {
		return
	}
	ruleDays := ruleContributionForTarget(target, selection, selections, offerings)
	if len(ruleDays) == 0 {
		return
	}
	entry.NewRuleDays = append([]string(nil), ruleDays...)
	entry.NewDaysWithoutRules = nonRuleDaysForTarget(target, selection, selections, offerings)
	for _, triggerID := range target.AutoAddTriggerOfferingIDs {
		triggerDays := autoDaysForTarget(target, []int64{triggerID}, selections, offerings)
		if !daysOverlap(triggerDays, ruleDays) {
			continue
		}
		name := fmt.Sprintf("Angebot %d", triggerID)
		if trigger := offerings[triggerID]; trigger != nil && trigger.Name != "" {
			name = trigger.Name
		}
		entry.AutoTriggerIDs = append(entry.AutoTriggerIDs, triggerID)
		entry.AutoTriggerNames = append(entry.AutoTriggerNames, name)
	}
}

func materializedSelectionPointers(
	materialized []materializedOfferingSelection,
) map[int64]*materializedOfferingSelection {
	byID := make(map[int64]*materializedOfferingSelection, len(materialized))
	for i := range materialized {
		byID[materialized[i].OfferingID] = &materialized[i]
	}
	return byID
}

func ruleContributionForTarget(
	target *enrollmentModels.CareOffering,
	selection *materializedOfferingSelection,
	selections map[int64]*materializedOfferingSelection,
	offerings map[int64]*enrollmentModels.CareOffering,
) []string {
	if target == nil || selection == nil {
		return nil
	}
	ruleDays := autoDaysForTarget(target, target.AutoAddTriggerOfferingIDs, selections, offerings)
	nonRuleDays := nonRuleDaysForTarget(target, selection, selections, offerings)
	return daysExcept(ruleDays, nonRuleDays)
}

func nonRuleDaysForTarget(
	target *enrollmentModels.CareOffering,
	selection *materializedOfferingSelection,
	selections map[int64]*materializedOfferingSelection,
	offerings map[int64]*enrollmentModels.CareOffering,
) []string {
	return unionDaysInOfferingOrder(
		target.AvailableDays,
		selection.ManualSelectedDays,
		autoLunchDaysForTarget(target, selections, offerings),
	)
}

func daysExcept(days, excluded []string) []string {
	return slices.DeleteFunc(slices.Clone(days), func(day string) bool {
		return slices.Contains(excluded, day)
	})
}

func daysOverlap(left, right []string) bool {
	for _, day := range left {
		if slices.Contains(right, day) {
			return true
		}
	}
	return false
}

func offeringDiffEntry(
	id int64,
	name string,
	current *enrollmentModels.RequestChildOffering,
	requestedByID map[int64]OfferingChangeSelection,
) (OfferingChangeDiffEntry, bool) {
	if name == "" {
		name = fmt.Sprintf("Angebot %d", id)
	}
	oldState := "not_booked"
	var oldDays []string
	if current != nil {
		oldState = "booked"
		oldDays = canonicalDays(current.SelectedDays)
	}
	newState := "removed"
	var newDays []string
	if requested, wanted := requestedByID[id]; wanted {
		newState = "booked"
		newDays = canonicalDays(requested.SelectedDays)
	}
	changed := oldState != newState || !slices.Equal(oldDays, newDays)
	return OfferingChangeDiffEntry{
		OfferingID: id,
		Label:      name,
		OldState:   oldState,
		OldDays:    oldDays,
		NewState:   newState,
		NewDays:    newDays,
	}, changed
}

func (s *offeringChangeRequestService) emitDecisionPill(
	ctx context.Context,
	row *enrollmentModels.OfferingChangeRequest,
	reviewedBy int64,
	body, status, reason string,
	payload map[string]any,
) {
	s.emitPillAfterCommit(ctx, row, parentmessaging.ChildEvent{
		EventType:      usersModels.ParentMessageEventRequestStatus,
		ActorKind:      usersModels.ParentMessageSenderStaff,
		ActorAccountID: reviewedBy,
		Body:           body,
		RequestType:    OfferingChangeRequestType,
		RequestStatus:  status,
		DecisionReason: reason,
		Payload:        payload,
	})
}

// emitPillAfterCommit posts the notification pill into the child's parent-OGS
// thread once the surrounding transaction committed. Best-effort: a lost pill
// costs a notification, never the decision.
func (s *offeringChangeRequestService) emitPillAfterCommit(
	ctx context.Context,
	row *enrollmentModels.OfferingChangeRequest,
	ev parentmessaging.ChildEvent,
) {
	if s.Emitter == nil {
		return
	}
	tenantID := row.TenantID
	if tenantID <= 0 {
		tenantID = tenant.FromContext(ctx)
	}
	refID := row.ID
	ev.RefTable = "enrollment.offering_change_requests"
	ev.RefID = &refID
	studentID := row.StudentID
	guardianAccountID := row.SubmittedBy
	tenant.RegisterAfterCommit(ctx, func() {
		s.Emitter.EmitChildEvent(tenantID, studentID, guardianAccountID, ev)
		s.Emitter.BroadcastChildUpdateToGuardians(tenantID, studentID)
	})
}

// offeringChangeAdjustmentReason writes the line the Änderungsprotokoll shows
// for an approved request. A date the office confirmed instead of the one the
// family asked for is named at the end: the request row only keeps the
// confirmed date, so without this the shift would leave no trace anywhere
// (#2484). The generated prefix keeps its exact shape — migration 1.15.309
// identifies request-applied rows by it.
func offeringChangeAdjustmentReason(
	row *enrollmentModels.OfferingChangeRequest,
	requested timezone.Date,
	staffReason string,
) string {
	reason := fmt.Sprintf("Elternanfrage #%d freigegeben (gültig ab %s)",
		row.ID, row.EffectiveFrom.Format("02.01.2006"))
	if trimmed := strings.TrimSpace(staffReason); trimmed != "" {
		reason += ": " + trimmed
	}
	if requested != row.EffectiveFrom {
		reason += " · Wunschdatum der Eltern war " + requested.Format("02.01.2006")
	}
	return reason
}

func adjustmentSelections(selections []OfferingChangeSelection) []OfferingAdjustmentSelection {
	out := make([]OfferingAdjustmentSelection, 0, len(selections))
	for _, selection := range selections {
		out = append(out, OfferingAdjustmentSelection{
			OfferingID:   selection.OfferingID,
			SelectedDays: append([]string(nil), selection.SelectedDays...),
		})
	}
	return out
}

func payloadFromSelections(selections []OfferingChangeSelection) map[string]any {
	rows := make([]any, 0, len(selections))
	for _, selection := range selections {
		row := map[string]any{"offering_id": selection.OfferingID}
		if len(selection.SelectedDays) > 0 {
			days := make([]any, 0, len(selection.SelectedDays))
			for _, day := range selection.SelectedDays {
				days = append(days, day)
			}
			row["selected_days"] = days
		}
		rows = append(rows, row)
	}
	return map[string]any{"offerings": rows}
}

// selectionsFromPayload parses the stored payload. It is strict: a payload we
// cannot read must not be silently applied as "no offerings", which would
// unbook the child entirely.
func selectionsFromPayload(payload map[string]any) ([]OfferingChangeSelection, error) {
	raw, ok := payload["offerings"]
	if !ok {
		return nil, fmt.Errorf("%w: payload has no offerings", ErrOfferingChangeInvalid)
	}
	list, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("%w: payload offerings must be a list", ErrOfferingChangeInvalid)
	}
	selections := make([]OfferingChangeSelection, 0, len(list))
	for _, entry := range list {
		selection, err := selectionFromPayloadEntry(entry)
		if err != nil {
			return nil, err
		}
		selections = append(selections, selection)
	}
	return selections, nil
}

func selectionFromPayloadEntry(entry any) (OfferingChangeSelection, error) {
	row, ok := entry.(map[string]any)
	if !ok {
		return OfferingChangeSelection{}, fmt.Errorf("%w: payload offering entry is not an object", ErrOfferingChangeInvalid)
	}
	id, err := payloadInt64(row["offering_id"])
	if err != nil {
		return OfferingChangeSelection{}, err
	}
	selection := OfferingChangeSelection{OfferingID: id}
	daysRaw, hasDays := row["selected_days"]
	if !hasDays || daysRaw == nil {
		return selection, nil
	}
	days, ok := daysRaw.([]any)
	if !ok {
		return OfferingChangeSelection{}, fmt.Errorf("%w: payload selected_days must be a list", ErrOfferingChangeInvalid)
	}
	values, err := payloadStrings(days)
	if err != nil {
		return OfferingChangeSelection{}, err
	}
	selection.SelectedDays = canonicalDays(values)
	return selection, nil
}

func payloadStrings(values []any) ([]string, error) {
	strings := make([]string, 0, len(values))
	for _, value := range values {
		text, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("%w: payload selected_days must contain strings", ErrOfferingChangeInvalid)
		}
		strings = append(strings, text)
	}
	return strings, nil
}

// payloadInt64 accepts the shapes jsonb round-trips through Go: float64 from
// encoding/json, plus int64/string for hand-written payloads.
func payloadInt64(value any) (int64, error) {
	switch typed := value.(type) {
	case float64:
		return int64(typed), nil
	case int64:
		return typed, nil
	case int:
		return int64(typed), nil
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("%w: offering id %q is not a number", ErrOfferingChangeInvalid, typed)
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("%w: offering id is missing", ErrOfferingChangeInvalid)
	}
}

// canonicalDays lowercases, de-duplicates and week-orders day abbreviations.
func canonicalDays(days []string) []string {
	out := make([]string, 0, len(days))
	seen := make(map[string]bool, len(days))
	for _, day := range days {
		normalized := strings.ToLower(strings.TrimSpace(day))
		if normalized == "" || seen[normalized] {
			continue
		}
		if _, ok := enrollmentModels.CanonicalDayToISOWeekday(normalized); !ok {
			continue
		}
		seen[normalized] = true
		out = append(out, normalized)
	}
	sort.SliceStable(out, func(i, j int) bool {
		left, _ := enrollmentModels.CanonicalDayToISOWeekday(out[i])
		right, _ := enrollmentModels.CanonicalDayToISOWeekday(out[j])
		return left < right
	})
	return out
}

func optionalString(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return &value
}

func cmpOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

// isPendingOfferingChangeConflict recognises the partial unique index violation
// from a concurrent second submit.
func isPendingOfferingChangeConflict(err error) bool {
	return err != nil && strings.Contains(err.Error(), "uq_offering_change_requests_pending")
}
