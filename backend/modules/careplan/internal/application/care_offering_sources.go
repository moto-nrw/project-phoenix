package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
)

// ValidateTemplateOfferingSource guards a template's offering-source rule
// (#2137). Vanished ids are tolerated only when they are already stored on
// the template: rejecting those would wedge every edit of a template whose
// array carries a dangling id, while a newly submitted unknown id would
// persist a dead source with a permanently empty roster.
func (c *CareOfferingCatalog) ValidateTemplateOfferingSource(ctx context.Context, offeringIDs, storedOfferingIDs []int64, calendarPeriodID *int64) error {
	sources, err := c.loadValidatedOfferingSources(ctx, offeringIDs, calendarPeriodID, false)
	if err != nil {
		return err
	}
	if len(sources.Dropped) == 0 {
		return nil
	}
	stored := make(map[int64]bool, len(storedOfferingIDs))
	for _, id := range storedOfferingIDs {
		stored[id] = true
	}
	for _, id := range sources.Dropped {
		if !stored[id] {
			return c.deps.SourceRules.Reject(fmt.Sprintf("care offering %d not found", id))
		}
	}
	c.deps.Logger.Warn("offering source validation: ignoring vanished stored source offerings",
		slog.Any("care_offering_ids", sources.Dropped),
	)
	return nil
}

// LoadOfferingSources resolves and validates every source offering of a
// template for the booking materialization's roster resync; see
// loadValidatedOfferingSources.
func (c *CareOfferingCatalog) LoadOfferingSources(ctx context.Context, offeringIDs []int64, calendarPeriodID *int64, tolerateDrift bool) (careplan.OfferingSources, error) {
	return c.loadValidatedOfferingSources(ctx, offeringIDs, calendarPeriodID, tolerateDrift)
}

// loadValidatedOfferingSources runs the offering-source guard (#2137) per id
// and rejects duplicate ids and mixed enrollment phases: the union's seeded
// rows carry ONE phase, and a child holding offerings of two phases would
// surface under two request-child tags. Per offering it must exist in the
// tenant and be active, and the shared phase's service window must lie
// within the calendar period (nil skips the period check). Every declaration
// path uses this guard, so no path can persist a source rule the others
// would reject.
//
// A vanished offering (the jsonb array carries no FK) is dropped and
// returned in Dropped instead of rejected, so the template degrades to its
// surviving sources. The list is capped and the period is loaded once: ids
// arrive from query strings and from the stored array on every resync.
//
// tolerateDrift skips the active and phase-within-period rejections; only
// the detach fallback sets it.
func (c *CareOfferingCatalog) loadValidatedOfferingSources(
	ctx context.Context,
	offeringIDs []int64,
	calendarPeriodID *int64,
	tolerateDrift bool,
) (careplan.OfferingSources, error) {
	if err := c.checkOfferingSourceIDs(offeringIDs); err != nil {
		return careplan.OfferingSources{}, err
	}
	period, err := c.offeringSourcePeriod(ctx, calendarPeriodID)
	if err != nil {
		return careplan.OfferingSources{}, err
	}
	if err := c.checkOfferingSourceDuplicates(offeringIDs); err != nil {
		return careplan.OfferingSources{}, err
	}
	loaded, err := c.listByIDs(ctx, offeringIDs)
	if err != nil {
		return careplan.OfferingSources{}, fmt.Errorf("offering roster resync: load offerings: %w", err)
	}
	byID := make(map[int64]careplan.CareOffering, len(loaded))
	for _, offering := range loaded {
		byID[offering.ID] = offering
	}
	sources := careplan.OfferingSources{Offerings: make([]careplan.CareOffering, 0, len(loaded))}
	for _, offeringID := range offeringIDs {
		offering, ok := byID[offeringID]
		if !ok {
			sources.Dropped = append(sources.Dropped, offeringID)
			continue
		}
		if err := c.acceptOfferingSource(ctx, &sources, offering, period, tolerateDrift); err != nil {
			return careplan.OfferingSources{}, err
		}
	}
	return sources, nil
}

func (c *CareOfferingCatalog) checkOfferingSourceIDs(offeringIDs []int64) error {
	if len(offeringIDs) == 0 {
		return c.deps.SourceRules.Reject("at least one care offering is required")
	}
	if limit := c.deps.SourceRules.MaxSourcesPerTemplate(); len(offeringIDs) > limit {
		return c.deps.SourceRules.Reject(fmt.Sprintf("at most %d source offerings are supported (%d given)", limit, len(offeringIDs)))
	}
	return nil
}

func (c *CareOfferingCatalog) offeringSourcePeriod(ctx context.Context, calendarPeriodID *int64) (*careplan.LinkedPeriod, error) {
	if calendarPeriodID == nil {
		return nil, nil
	}
	period, err := c.deps.Calendar.FindPeriod(ctx, *calendarPeriodID)
	if err != nil {
		if errors.Is(err, ports.ErrCatalogRowNotFound) {
			return nil, c.deps.SourceRules.Reject(fmt.Sprintf("calendar period %d not found", *calendarPeriodID))
		}
		return nil, fmt.Errorf("offering roster resync: load calendar period: %w", err)
	}
	return &period, nil
}

// checkOfferingSourceDuplicates runs after the period lookup, as the guard
// always has.
func (c *CareOfferingCatalog) checkOfferingSourceDuplicates(offeringIDs []int64) error {
	seen := make(map[int64]bool, len(offeringIDs))
	for _, offeringID := range offeringIDs {
		if seen[offeringID] {
			return c.deps.SourceRules.Reject(fmt.Sprintf("care offering %d is listed twice", offeringID))
		}
		seen[offeringID] = true
	}
	return nil
}

func (c *CareOfferingCatalog) acceptOfferingSource(
	ctx context.Context,
	sources *careplan.OfferingSources,
	offering careplan.CareOffering,
	period *careplan.LinkedPeriod,
	tolerateDrift bool,
) error {
	if !offering.IsActive && !tolerateDrift {
		return c.deps.SourceRules.Reject(fmt.Sprintf("care offering %d is inactive", offering.ID))
	}
	if sources.Phase != nil && offering.PhaseID != sources.Phase.ID {
		return c.deps.SourceRules.Reject(fmt.Sprintf(
			"all source offerings must belong to the same enrollment phase (offering %d belongs to %q)",
			offering.ID, c.offeringPhaseName(ctx, offering.PhaseID),
		))
	}
	if sources.Phase == nil {
		phase, err := c.establishSourcePhase(ctx, offering, period, tolerateDrift)
		if err != nil {
			return err
		}
		sources.Phase = &phase
	}
	sources.Offerings = append(sources.Offerings, offering)
	return nil
}

// establishSourcePhase loads the phase of the first surviving offering. It
// becomes the shared phase; the window check runs once since every later
// offering must match it.
func (c *CareOfferingCatalog) establishSourcePhase(
	ctx context.Context,
	offering careplan.CareOffering,
	period *careplan.LinkedPeriod,
	tolerateDrift bool,
) (careplan.OfferingPhase, error) {
	phase, err := c.deps.Phases.Phase(ctx, offering.PhaseID)
	if err != nil {
		if errors.Is(err, ports.ErrCatalogRowNotFound) {
			return careplan.OfferingPhase{}, c.deps.SourceRules.Reject(fmt.Sprintf("enrollment phase of care offering %d not found", offering.ID))
		}
		return careplan.OfferingPhase{}, fmt.Errorf("offering roster resync: load phase: %w", err)
	}
	if period != nil && !tolerateDrift {
		if err := careplan.ValidatePhaseWithinPeriod(phase, period); err != nil {
			return careplan.OfferingPhase{}, c.deps.SourceRules.Reject(err.Error())
		}
	}
	return phase, nil
}

// offeringPhaseName resolves a phase name for the mixed-phase error message;
// error path only, so a failed lookup falls back to the id.
func (c *CareOfferingCatalog) offeringPhaseName(ctx context.Context, phaseID int64) string {
	phase, err := c.deps.Phases.Phase(ctx, phaseID)
	if err != nil {
		return fmt.Sprintf("phase %d", phaseID)
	}
	return phase.Name
}

// ValidateTemplateSeries checks every care offering linked to any live
// segment of groupID's split lineage against the complete post-split series.
// The split calls this after creating the successor but before committing, so
// a recurrence edit cannot turn an accepted catalog link into a later
// approval failure.
func (c *CareOfferingCatalog) ValidateTemplateSeries(ctx context.Context, groupID int64) error {
	if groupID <= 0 {
		return careOfferingInvalidf("template group id must be positive")
	}
	series, err := c.deps.Timetable.TemplateSeries(ctx, groupID)
	if err != nil {
		return fmt.Errorf("load template series for care offering validation: %w", err)
	}
	// Include groupID even when it was provisionally archived in the current
	// transaction: the live series leaves it out, but an offering linked to
	// the row being archived must still be found and rejected before commit.
	offerings, err := c.listByActivityGroupIDs(ctx, careOfferingSeriesGroupIDs(groupID, series))
	if err != nil {
		return fmt.Errorf("list care offerings linked to template series: %w", err)
	}
	phases := make(map[int64]careplan.OfferingPhase)
	for _, offering := range offerings {
		if err := c.validateTemplateSeriesOffering(ctx, phases, offering); err != nil {
			return err
		}
	}
	return nil
}

func careOfferingSeriesGroupIDs(groupID int64, series []careplan.LinkedGroup) []int64 {
	groupIDs := make([]int64, 0, len(series)+1)
	groupIDs = append(groupIDs, groupID)
	seen := map[int64]bool{groupID: true}
	for _, segment := range series {
		if seen[segment.ID] {
			continue
		}
		groupIDs = append(groupIDs, segment.ID)
		seen[segment.ID] = true
	}
	return groupIDs
}

func (c *CareOfferingCatalog) validateTemplateSeriesOffering(ctx context.Context, phases map[int64]careplan.OfferingPhase, offering careplan.CareOffering) error {
	if offering.ActivityGroupID == nil {
		return nil
	}
	requiresMaterialization, err := c.offeringRequiresMaterialization(ctx, offering)
	if err != nil {
		return fmt.Errorf("inspect care offering %d request selections: %w", offering.ID, err)
	}
	if !requiresMaterialization {
		return nil
	}
	phase, err := c.careOfferingSeriesPhase(ctx, phases, offering)
	if err != nil {
		return err
	}
	if err := c.validateLinkMaterializable(ctx, *offering.ActivityGroupID, phase, offering.AvailableDays, careOfferingMaterializationChange{}); err != nil {
		return fmt.Errorf("care offering %d is incompatible with the split template series: %w", offering.ID, err)
	}
	return nil
}

func (c *CareOfferingCatalog) careOfferingSeriesPhase(ctx context.Context, phases map[int64]careplan.OfferingPhase, offering careplan.CareOffering) (careplan.OfferingPhase, error) {
	if phase, ok := phases[offering.PhaseID]; ok {
		return phase, nil
	}
	phase, err := c.deps.Phases.Phase(ctx, offering.PhaseID)
	if err != nil {
		if errors.Is(err, ports.ErrCatalogRowNotFound) {
			return careplan.OfferingPhase{}, careOfferingInvalidf("care offering %d references an unavailable phase", offering.ID)
		}
		return careplan.OfferingPhase{}, fmt.Errorf("load phase for care offering %d: %w", offering.ID, err)
	}
	phases[offering.PhaseID] = phase
	return phase, nil
}

// ResolveLinkedSegments expands a link to every live split-series segment
// that can produce an occurrence during the phase. A non-template activity
// stays one segment without a period.
func (c *CareOfferingCatalog) ResolveLinkedSegments(ctx context.Context, activityGroupID int64, phase careplan.OfferingPhase) ([]careplan.LinkedSegment, error) {
	return c.resolveCareOfferingLinkedGroupsForPhase(ctx, activityGroupID, phase)
}

// ValidateMaterializable checks the weekday coverage with an active period
// required and the materializability of every selected weekday.
func (c *CareOfferingCatalog) ValidateMaterializable(ctx context.Context, segments []careplan.LinkedSegment, phase careplan.OfferingPhase, days []string) error {
	if err := validateCareOfferingTemplateSegments(c.deps.Calendar, segments, phase, days, true); err != nil {
		return err
	}
	return c.validateCareOfferingMaterializability(ctx, segments, phase, days, careOfferingMaterializationChange{})
}

// ResolveTemplatePeriod resolves the one calendar period a template's
// schedules share.
func (c *CareOfferingCatalog) ResolveTemplatePeriod(ctx context.Context, group careplan.LinkedGroup) (careplan.LinkedPeriod, error) {
	return c.resolveTemplatePeriodForGroup(ctx, group)
}
