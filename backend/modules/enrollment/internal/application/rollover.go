package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// Enrollment owner ports of the rollover; the root binds the owner.
type (
	RolloverPhases interface {
		Phase(context.Context, int64) (*enrollment.Phase, error)
		InsertPhase(context.Context, *enrollment.Phase) error
		HasRolloverSuccessor(context.Context, int64) (bool, error)
		PhasesWithExpiredRolloverDeadline(context.Context, time.Time) ([]*enrollment.Phase, error)
	}
	RolloverRequests interface {
		InsertRequest(context.Context, *enrollment.Request) error
		RequestsByID(context.Context, []int64) ([]*enrollment.Request, error)
	}
	RolloverChildren interface {
		enrollment.SubmittedOfferingCommands
		RequestChildOfferingsAtDate(context.Context, int64, enrollment.Date) ([]*enrollment.RequestChildOffering, error)
		RequestChildOfferingsForChildrenAtDate(context.Context, []int64, enrollment.Date) ([]*enrollment.RequestChildOffering, error)
		InsertChild(context.Context, *enrollment.RequestChild) error
		ChildrenByID(context.Context, []int64) ([]*enrollment.RequestChild, error)
		ChildrenByPhaseStatuses(context.Context, int64, []string) ([]*enrollment.RequestChild, error)
		ReviewRolloverChild(context.Context, int64, string, *string, *int16, int64) error
		TransitionPhaseChildren(context.Context, int64, string, string) (int, error)
	}
	// RolloverCatalogCloner copies a phase's care-offering catalog into the
	// follow-up phase (#2249). The Care Plan catalog implements it: every
	// offering of the source phase plus the carried offerings of earlier
	// phases, with the linked timetable templates kept and the auto-add
	// triggers remapped, returning the source→target offering ids.
	RolloverCatalogCloner interface {
		CloneCatalogForRollover(ctx context.Context, sourcePhaseID int64, targetPhaseID int64, carriedOfferingIDs []int64) (map[int64]int64, error)
	}
	// PhaseEligibilityGuard rejects an eligibility restriction the school
	// cannot collect. The phase administration implements it.
	PhaseEligibilityGuard interface {
		CheckEligibilityCollectable(ctx context.Context, phase *enrollment.Phase) error
	}
	// RolloverDecider approves an auto-renewed row on an auto-approve phase.
	RolloverDecider interface {
		Decide(ctx context.Context, input enrollment.DecideInput) (*enrollment.DecideOutcome, error)
	}
)

// RolloverDependencies bind the rollover to its owners. Bookings, Catalog,
// Notifications, PhaseEligibility, Outbox and Decisions are optional where
// the rollover degrades without them.
type RolloverDependencies struct {
	Bookings enrollment.CareBookingCommands
	Phases   RolloverPhases
	Requests RolloverRequests
	Children RolloverChildren
	// Catalog clones the source phase's care-offering catalog into the
	// follow-up phase (#2249). Production always wires the Care Plan
	// catalog; without it (lightweight tests without offerings) the catalog
	// is not cloned and any carried booking fails the rollover instead of
	// persisting a source-phase reference.
	Catalog       RolloverCatalogCloner
	Notifications enrollment.Notifications
	// PhaseEligibility rejects an eligibility restriction the school cannot
	// collect before the rollover activates it.
	PhaseEligibility PhaseEligibilityGuard
	Outbox           MailOutbox
	Settings         CollectionSettings
	// Decisions approves auto-renewed rows on a rollover_auto_approve phase;
	// without it the deadline worker promotes them to submitted.
	Decisions  RolloverDecider
	ParentsURL string
	// Random fills the status tokens of the carried requests.
	Random  func([]byte) error
	Runtime Runtime
	Logger  *slog.Logger
}

// Rollovers creates a new phase from a source phase, carrying every approved
// enrollment forward into the new phase under one of two parent-action modes
// (opt_in / opt_out). Children whose new grade would exceed the tenant's
// grade maximum land in an admin review queue instead of being auto-rolled.
// The rollover orchestrates the phase, request, child and offering-selection
// writes inside a single tenant transaction, plus the outbox.
type Rollovers struct {
	deps RolloverDependencies
}

var _ enrollment.Rollovers = (*Rollovers)(nil)

// NewRollovers composes the rollover over its owners.
func NewRollovers(deps RolloverDependencies) *Rollovers {
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	return &Rollovers{deps: deps}
}

type rolloverRequestInput struct {
	tenantID          int64
	sourcePhase       *enrollment.Phase
	newPhase          *enrollment.Phase
	sourceChildren    []*RequestChild
	maxGrade          int
	collectGradeLevel bool
	// offeringIDMap translates source-phase care offering IDs to their
	// target-phase clones. Every carried booking MUST resolve through it — a
	// source-phase reference in the target phase is exactly the mixed-phase
	// state #2249 forbids.
	offeringIDMap map[int64]int64
	result        *enrollment.RolloverResult
}

// CreatePhaseFromSource is the workhorse:
//  1. Load + validate source phase
//  2. Build the new Phase (Validate(), Insert)
//  3. Clone the source phase's care-offering catalog into the new phase
//  4. List approved source children
//  5. Group by source request_id and create one new request per group
//  6. Per child: compute new grade, decide status, create row
//  7. Copy the effective care-offering booking, remapped to the cloned
//     offerings, days carried, validity reset
//  8. Enqueue one email per new request via the outbox
func (s *Rollovers) CreatePhaseFromSource(ctx context.Context, req enrollment.CreatePhaseFromSourceRequest) (*enrollment.RolloverResult, error) {
	if err := validateCreateRequest(req); err != nil {
		return nil, err
	}
	tenantID := s.deps.Runtime.TenantID(ctx)
	if tenantID == 0 {
		return nil, fmt.Errorf("rollover: tenant not in context")
	}
	maxGrade, err := s.resolveMaxGrade(ctx)
	if err != nil {
		return nil, fmt.Errorf("rollover: %w", err)
	}
	collectGradeLevel, err := s.deps.Settings.CollectGradeLevel(ctx)
	if err != nil {
		return nil, fmt.Errorf("rollover: resolve collect grade level: %w", err)
	}
	result := &enrollment.RolloverResult{ReviewByReason: make(map[string]int)}
	// The whole rollover runs in one tenant tx: if any insert fails the
	// caller sees "atomic" — either everything carried or nothing.
	txErr := s.deps.Runtime.Transactions.RunInTx(ctx, func(txCtx context.Context) error {
		return s.runCreate(txCtx, tenantID, req, maxGrade, collectGradeLevel, result)
	})
	if txErr != nil {
		return nil, txErr
	}
	return result, nil
}

func (s *Rollovers) runCreate(ctx context.Context, tenantID int64, req enrollment.CreatePhaseFromSourceRequest, maxGrade int, collectGradeLevel bool, result *enrollment.RolloverResult) error {
	source, err := s.loadRolloverSourcePhase(ctx, tenantID, req.SourcePhaseID)
	if err != nil {
		return err
	}
	newPhase, err := s.createRolloverPhase(ctx, tenantID, req, source)
	if err != nil {
		return err
	}
	result.Phase = newPhase
	sourceChildren, err := s.childrenByStatuses(ctx, source.ID, []string{enrollment.ChildStatusApproved})
	if err != nil {
		return fmt.Errorf("rollover: list source children: %w", err)
	}
	result.SourceChildCount = len(sourceChildren)
	carriedOfferingIDs, err := s.listCarriedOfferingIDs(ctx, source, sourceChildren)
	if err != nil {
		return err
	}
	offeringIDMap, err := s.cloneOfferingCatalog(ctx, source.ID, newPhase.ID, carriedOfferingIDs)
	if err != nil {
		return err
	}
	result.ClonedOfferingCount = len(offeringIDMap)
	return s.rollSourceRequests(ctx, rolloverRequestInput{
		tenantID:          tenantID,
		sourcePhase:       source,
		newPhase:          newPhase,
		sourceChildren:    sourceChildren,
		maxGrade:          maxGrade,
		collectGradeLevel: collectGradeLevel,
		offeringIDMap:     offeringIDMap,
		result:            result,
	})
}

// cloneOfferingCatalog delegates to the catalog. A nil catalog yields an
// empty map — copyRolloverOfferings then rejects any booking it cannot
// remap, so a mis-wired production setup fails loudly instead of carrying
// source-phase offering references (#2249).
func (s *Rollovers) cloneOfferingCatalog(ctx context.Context, sourcePhaseID, targetPhaseID int64, carriedOfferingIDs []int64) (map[int64]int64, error) {
	if s.deps.Catalog == nil {
		return map[int64]int64{}, nil
	}
	mapping, err := s.deps.Catalog.CloneCatalogForRollover(ctx, sourcePhaseID, targetPhaseID, carriedOfferingIDs)
	if err != nil {
		return nil, fmt.Errorf("rollover: clone offering catalog: %w", err)
	}
	return mapping, nil
}

func (s *Rollovers) listCarriedOfferingIDs(ctx context.Context, sourcePhase *enrollment.Phase, children []*RequestChild) ([]int64, error) {
	childIDs := make([]int64, 0, len(children))
	for _, child := range children {
		childIDs = append(childIDs, child.ID)
	}
	offerings, err := s.deps.Children.RequestChildOfferingsForChildrenAtDate(ctx, childIDs, sourcePhase.ServiceEndDate)
	if err != nil {
		return nil, fmt.Errorf("rollover: list source offerings: %w", err)
	}
	ids := make(map[int64]struct{})
	for _, offering := range offerings {
		ids[offering.CareOfferingID] = struct{}{}
	}
	result := int64SetKeys(ids)
	slices.Sort(result)
	return result, nil
}

func (s *Rollovers) loadRolloverSourcePhase(ctx context.Context, tenantID, sourcePhaseID int64) (*enrollment.Phase, error) {
	source, err := s.deps.Phases.Phase(ctx, sourcePhaseID)
	if err != nil || source == nil {
		return nil, fmt.Errorf("%w: %d", enrollment.ErrRolloverSourceNotFound, sourcePhaseID)
	}
	if source.TenantID != tenantID {
		return nil, fmt.Errorf("%w: source phase belongs to another tenant", enrollment.ErrRolloverSourceNotFound)
	}
	exists, err := s.deps.Phases.HasRolloverSuccessor(ctx, source.ID)
	if err != nil {
		return nil, fmt.Errorf("rollover: check existing follow-up: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("%w: source phase %d", enrollment.ErrRolloverSourceAlreadyRolled, source.ID)
	}
	return source, nil
}

func (s *Rollovers) createRolloverPhase(ctx context.Context, tenantID int64, req enrollment.CreatePhaseFromSourceRequest, source *enrollment.Phase) (*enrollment.Phase, error) {
	phase := rolloverPhaseFromSource(req, source)
	phase.TenantID = tenantID
	if err := phase.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %v", enrollment.ErrRolloverInvalidRequest, err)
	}
	// A rollover copies the source's class-eligibility restriction into an
	// ACTIVE successor. If concrete-class collection was disabled after the
	// source went inactive, activating that restriction here would make every
	// submission fail class_not_eligible — the invariant the admin
	// create/update paths enforce but which this direct write otherwise
	// bypasses. runCreate wraps this in a tenant tx, so the shared lock
	// inside serializes it against a concurrent class-collection toggle
	// (#1663).
	if s.deps.PhaseEligibility != nil {
		if err := s.deps.PhaseEligibility.CheckEligibilityCollectable(ctx, phase); err != nil {
			return nil, fmt.Errorf("%w: %v", enrollment.ErrRolloverInvalidRequest, err)
		}
	}
	if err := s.deps.Phases.InsertPhase(ctx, phase); err != nil {
		if errors.Is(err, enrollment.ErrPhaseNameTaken) {
			return nil, fmt.Errorf("%w: %q", enrollment.ErrRolloverDuplicateName, req.Name)
		}
		return nil, fmt.Errorf("rollover: create phase: %w", err)
	}
	return phase, nil
}

// rolloverPhaseFromSource builds the follow-up phase. Name translations ride
// along as a starting point; parents only see them again once the new name
// matches their German source (#3377). The eligibility config is carried
// forward: without it the successor silently defaults to audience=open with
// no class gate, turning a rolled linked/class-restricted phase public — the
// active successor is what parents actually submit to (#1663). The grade
// restriction is copied verbatim, like the class list: a rollover is a new
// service period for the same target group, and the admin retargets the
// successor in the editor when the group changes. It is deliberately NOT
// shifted by RolloverBumpsGrade — that flag bumps each CHILD's grade, and
// auto-shifting the phase restriction alongside it would silently retarget a
// phase the admin never edited (#1663).
func rolloverPhaseFromSource(req enrollment.CreatePhaseFromSourceRequest, source *enrollment.Phase) *enrollment.Phase {
	formSchemaID := req.FormSchemaID
	if formSchemaID == nil {
		formSchemaID = source.FormSchemaID
	}
	mode := req.RolloverMode
	deadline := req.RolloverDeadline
	return &enrollment.Phase{
		Name:                      req.Name,
		Kind:                      req.Kind,
		ServiceStartDate:          enrollment.Date(req.ServiceStartDate),
		ServiceEndDate:            enrollment.Date(req.ServiceEndDate),
		EnrollmentOpenAt:          req.EnrollmentOpenAt,
		EnrollmentCloseAt:         req.EnrollmentCloseAt,
		FormSchemaID:              formSchemaID,
		ShowStatusReasonToParent:  source.ShowStatusReasonToParent,
		CareOverflowMode:          source.CareOverflowMode,
		CareOfferingSelectionMode: source.CareOfferingSelectionMode,
		AvailableSchoolClasses:    source.AvailableSchoolClasses,
		RequireSchoolClass:        source.RequireSchoolClass,
		Translations:              source.Translations,
		Audience:                  source.Audience,
		EligibleSchoolClasses:     source.EligibleSchoolClasses,
		EligibleGradeLevels:       source.EligibleGradeLevels,
		IsActive:                  true,
		RolloverSourcePhaseID:     &source.ID,
		RolloverMode:              &mode,
		RolloverAutoApprove:       req.RolloverAutoApprove,
		RolloverDeadline:          &deadline,
		RolloverBumpsGrade:        req.RolloverBumpsGrade,
	}
}

func validateCreateRequest(req enrollment.CreatePhaseFromSourceRequest) error {
	switch {
	case req.SourcePhaseID <= 0:
		return fmt.Errorf("%w: source_phase_id is required", enrollment.ErrRolloverInvalidRequest)
	case req.Name == "":
		return fmt.Errorf("%w: name is required", enrollment.ErrRolloverInvalidRequest)
	case req.ServiceStartDate.IsZero() || req.ServiceEndDate.IsZero():
		return fmt.Errorf("%w: service dates are required", enrollment.ErrRolloverInvalidRequest)
	case req.ServiceEndDate.Before(req.ServiceStartDate):
		return fmt.Errorf("%w: service_end_date must be on or after service_start_date", enrollment.ErrRolloverInvalidRequest)
	case req.RolloverDeadline.IsZero():
		return fmt.Errorf("%w: rollover_deadline is required", enrollment.ErrRolloverInvalidRequest)
	case req.RolloverMode != enrollment.PhaseRolloverModeOptIn && req.RolloverMode != enrollment.PhaseRolloverModeOptOut:
		return fmt.Errorf("%w: rollover_mode must be opt_in or opt_out", enrollment.ErrRolloverInvalidRequest)
	}
	return nil
}

// resolveMaxGrade resolves the registry's legitimate default when this
// tenant has no override. A missing settings port, a read failure, or a
// corrupt value must stop the rollover: substituting grade 4 would disagree
// with public enrollment and could silently route valid higher-grade
// children into admin review.
func (s *Rollovers) resolveMaxGrade(ctx context.Context) (int, error) {
	if s.deps.Settings == nil {
		return 0, errors.New("resolve enrollment.grade_level_max: settings service is not configured")
	}
	value, err := s.deps.Settings.GradeLevelMax(ctx)
	if err != nil {
		return 0, fmt.Errorf("resolve enrollment.grade_level_max: %w", err)
	}
	if value < minGradeLevel || value > maxGradeLevel {
		return 0, fmt.Errorf(
			"resolve enrollment.grade_level_max: value %d is outside %d..%d",
			value,
			minGradeLevel,
			maxGradeLevel,
		)
	}
	return value, nil
}

// childrenByStatuses reads and decodes a phase's children in the statuses.
func (s *Rollovers) childrenByStatuses(ctx context.Context, phaseID int64, statuses []string) ([]*RequestChild, error) {
	values, err := s.deps.Children.ChildrenByPhaseStatuses(ctx, phaseID, statuses)
	if err != nil {
		return nil, err
	}
	return childValues(values)
}
