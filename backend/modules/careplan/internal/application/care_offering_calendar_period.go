package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
)

// periodChange is a calendar-period edit being simulated: the period's id
// and its proposed state, nil for a deletion.
type periodChange struct {
	periodID    int64
	replacement *careplan.LinkedPeriod
	periods     map[int64]careplan.LinkedPeriod
}

func (change periodChange) deleting() bool { return change.replacement == nil }

// ValidateCalendarPeriodChange simulates the effective period of every
// care-offering-linked template segment after an update or delete. The
// caller holds the tenant recurrence lock and invokes this before changing
// the period row, so a refusal cannot leave a partially mutated recurrence.
func (c *CareOfferingCatalog) ValidateCalendarPeriodChange(ctx context.Context, periodID int64, replacement *careplan.CalendarPeriodReplacement) error {
	if periodID <= 0 {
		return errors.New("calendar period change validation requires a positive period id")
	}
	change := periodChange{periodID: periodID, periods: make(map[int64]careplan.LinkedPeriod)}
	if replacement != nil {
		change.replacement = &careplan.LinkedPeriod{
			ID: periodID, StartDate: replacement.StartDate, EndDate: replacement.EndDate, IsActive: replacement.IsActive,
			WeekCycleLength: replacement.WeekCycleLength, WeekCycleAnchor: replacement.WeekCycleAnchor,
		}
	}
	offerings, err := c.listTenantRecords(ctx)
	if err != nil {
		return fmt.Errorf("list care offerings for calendar period validation: %w", err)
	}
	for _, offering := range offerings {
		if offering.ActivityGroupID == nil {
			continue
		}
		if err := c.validateOfferingCalendarPeriodChange(ctx, offering, change); err != nil {
			return err
		}
	}
	return nil
}

func (c *CareOfferingCatalog) validateOfferingCalendarPeriodChange(ctx context.Context, offering careplan.CareOffering, change periodChange) error {
	requiresMaterialization, err := c.offeringRequiresMaterialization(ctx, offering)
	if err != nil {
		return fmt.Errorf("inspect care offering %d request selections: %w", offering.ID, err)
	}
	if !requiresMaterialization {
		// Unused inactive offerings are drafts or history and may be staged
		// against an inactive future period.
		return nil
	}
	group, series, err := c.loadCareOfferingTemplateSeriesForPeriodChange(ctx, offering)
	if err != nil || len(series) == 0 {
		return err
	}
	loaded, referencesChangedPeriod, err := c.loadCareOfferingPeriodChangeSegments(ctx, offering.ID, series, change.periodID)
	if err != nil || !referencesChangedPeriod {
		return err
	}
	phase, err := c.deps.Phases.Phase(ctx, offering.PhaseID)
	if err != nil {
		return fmt.Errorf("load care offering %d phase: %w", offering.ID, err)
	}
	segments, err := c.resolveCareOfferingPostChangeSegments(ctx, offering, group, phase, loaded, change)
	if err != nil {
		return err
	}
	if err := validateCareOfferingTemplateSegments(c.deps.Calendar, segments, phase, offering.AvailableDays, true); err != nil {
		return careOfferingPeriodConflictf(offering.ID, err)
	}
	if err := c.validateCareOfferingMaterializability(ctx, segments, phase, offering.AvailableDays, careOfferingMaterializationChange{}); err != nil {
		return careOfferingPeriodConflictf(offering.ID, err)
	}
	return nil
}

func (c *CareOfferingCatalog) loadCareOfferingTemplateSeriesForPeriodChange(
	ctx context.Context,
	offering careplan.CareOffering,
) (careplan.LinkedGroup, []careplan.LinkedGroup, error) {
	group, err := c.deps.Timetable.FindGroup(ctx, *offering.ActivityGroupID)
	if err != nil {
		return careplan.LinkedGroup{}, nil, fmt.Errorf("load care offering %d timetable template: %w", offering.ID, err)
	}
	// Historical non-template links do not use recurrence calendar periods.
	if !group.IsTemplate {
		return group, nil, nil
	}
	series, err := c.deps.Timetable.TemplateSeries(ctx, group.ID)
	if err != nil {
		return careplan.LinkedGroup{}, nil, fmt.Errorf("load care offering %d timetable series: %w", offering.ID, err)
	}
	// An empty live series is an archived series. Archive validation keeps a
	// live care offering out of this state.
	return group, series, nil
}

func (c *CareOfferingCatalog) loadCareOfferingPeriodChangeSegments(
	ctx context.Context,
	offeringID int64,
	series []careplan.LinkedGroup,
	periodID int64,
) ([]careplan.LinkedSegment, bool, error) {
	groupIDs := make([]int64, 0, len(series))
	for _, segment := range series {
		groupIDs = append(groupIDs, segment.ID)
	}
	var scheduleRows []careplan.LinkedSchedule
	if len(groupIDs) > 0 {
		var err error
		scheduleRows, err = c.deps.Timetable.GroupSchedules(ctx, groupIDs)
		if err != nil {
			return nil, false, fmt.Errorf("load care offering %d timetable schedules: %w", offeringID, err)
		}
	}
	schedulesByGroup := linkedSchedulesByGroup(scheduleRows)
	loaded := make([]careplan.LinkedSegment, 0, len(series))
	referencesChangedPeriod := false
	for _, segment := range series {
		schedules := schedulesByGroup[segment.ID]
		referencesChangedPeriod = referencesChangedPeriod ||
			referencesPeriod(segment.CalendarPeriodID, periodID) ||
			schedulesReferencePeriod(schedules, periodID)
		loaded = append(loaded, careplan.LinkedSegment{Group: segment, Schedules: schedules})
	}
	return loaded, referencesChangedPeriod, nil
}

func linkedSchedulesByGroup(rows []careplan.LinkedSchedule) map[int64][]careplan.LinkedSchedule {
	result := make(map[int64][]careplan.LinkedSchedule)
	for _, row := range rows {
		result[row.GroupID] = append(result[row.GroupID], row)
	}
	return result
}

func schedulesReferencePeriod(schedules []careplan.LinkedSchedule, periodID int64) bool {
	for _, schedule := range schedules {
		if referencesPeriod(schedule.CalendarPeriodID, periodID) {
			return true
		}
	}
	return false
}

func (c *CareOfferingCatalog) resolveCareOfferingPostChangeSegments(
	ctx context.Context,
	offering careplan.CareOffering,
	root careplan.LinkedGroup,
	phase careplan.OfferingPhase,
	loaded []careplan.LinkedSegment,
	change periodChange,
) ([]careplan.LinkedSegment, error) {
	segments := make([]careplan.LinkedSegment, 0, len(loaded))
	for _, state := range loaded {
		segment, include, err := c.resolveCareOfferingPostChangeSegment(ctx, offering, root, phase, state, change)
		if err != nil {
			return nil, err
		}
		if include {
			segments = append(segments, segment)
		}
	}
	return segments, nil
}

func (c *CareOfferingCatalog) resolveCareOfferingPostChangeSegment(
	ctx context.Context,
	offering careplan.CareOffering,
	root careplan.LinkedGroup,
	phase careplan.OfferingPhase,
	state careplan.LinkedSegment,
	change periodChange,
) (careplan.LinkedSegment, bool, error) {
	overlapsPhase := careplan.SchedulesOverlapPhase(state.Schedules, phase)
	mustResolve := overlapsPhase || (change.deleting() && state.Group.ID == root.ID)
	resolvedPeriodID, changed, err := effectivePeriodIDAfterChange(state.Group, state.Schedules, change.periodID, change.deleting())
	if err != nil {
		// The directly linked root is resolved before series expansion during
		// approval. Deleting its last effective period breaks the link even
		// when that historic segment does not overlap this phase.
		if mustResolve {
			return careplan.LinkedSegment{}, false, careOfferingPeriodConflictf(offering.ID, err)
		}
		return careplan.LinkedSegment{}, false, nil
	}
	period, err := c.resolvePostChangeSegmentPeriod(ctx, state.Group, resolvedPeriodID, changed, overlapsPhase, change)
	if err != nil {
		return careplan.LinkedSegment{}, false, careOfferingPostChangePeriodError(offering.ID, state.Group.ID, err, mustResolve)
	}
	if !overlapsPhase {
		return careplan.LinkedSegment{}, false, nil
	}
	if period == nil || !period.IsActive {
		return careplan.LinkedSegment{}, false, careOfferingPeriodConflictf(
			offering.ID,
			careOfferingInvalidf("a materializable timetable-linked care offering requires an active calendar period"),
		)
	}
	if err := careplan.ValidatePhaseWithinPeriod(phase, period); err != nil {
		return careplan.LinkedSegment{}, false, careOfferingPeriodConflictf(offering.ID, err)
	}
	return careplan.LinkedSegment{Group: state.Group, Period: period, Schedules: state.Schedules}, true, nil
}

func (c *CareOfferingCatalog) resolvePostChangeSegmentPeriod(
	ctx context.Context,
	group careplan.LinkedGroup,
	resolvedPeriodID int64,
	changed bool,
	overlapsPhase bool,
	change periodChange,
) (*careplan.LinkedPeriod, error) {
	if changed {
		return c.calendarPeriodAfterChange(ctx, resolvedPeriodID, change)
	}
	if !overlapsPhase {
		return nil, nil
	}
	period, err := c.resolveTemplatePeriodForGroup(ctx, group)
	if err != nil {
		return nil, err
	}
	return &period, nil
}

func careOfferingPostChangePeriodError(offeringID, groupID int64, err error, mustResolve bool) error {
	if !mustResolve {
		return nil
	}
	if errors.Is(err, careplan.ErrCareOfferingConfigInvalid) {
		return careOfferingPeriodConflictf(offeringID, err)
	}
	return fmt.Errorf("resolve care offering %d timetable segment %d period: %w", offeringID, groupID, err)
}

func effectivePeriodIDAfterChange(
	group careplan.LinkedGroup,
	schedules []careplan.LinkedSchedule,
	periodID int64,
	deleting bool,
) (resolved int64, changed bool, err error) {
	groupPeriodID := periodReferenceAfterChange(group.CalendarPeriodID, periodID, deleting)
	changed = referencesPeriod(group.CalendarPeriodID, periodID)
	if len(schedules) == 0 {
		return effectivePeriodWithoutSchedules(changed)
	}
	for _, schedule := range schedules {
		resolved, changed, err = mergeEffectiveSchedulePeriod(resolved, changed, groupPeriodID, schedule, periodID, deleting)
		if err != nil {
			return resolved, changed, err
		}
	}
	return effectivePeriodChangeResult(resolved, changed)
}

func effectivePeriodWithoutSchedules(changed bool) (int64, bool, error) {
	if !changed {
		return 0, false, nil
	}
	return 0, true, careOfferingInvalidf("timetable template must have at least one schedule")
}

func mergeEffectiveSchedulePeriod(
	resolved int64,
	changed bool,
	groupPeriodID *int64,
	schedule careplan.LinkedSchedule,
	periodID int64,
	deleting bool,
) (int64, bool, error) {
	changed = changed || referencesPeriod(schedule.CalendarPeriodID, periodID)
	schedulePeriodID := periodReferenceAfterChange(schedule.CalendarPeriodID, periodID, deleting)
	if schedulePeriodID == nil {
		schedulePeriodID = groupPeriodID
	}
	if schedulePeriodID == nil {
		if changed {
			return 0, true, careOfferingInvalidf("timetable template schedules must resolve one calendar_period_id after the change")
		}
		return resolved, false, nil
	}
	if resolved == 0 {
		return *schedulePeriodID, changed, nil
	}
	if resolved != *schedulePeriodID && changed {
		return 0, true, careOfferingInvalidf("timetable template schedules would use different calendar_period_id values after the change")
	}
	return resolved, changed, nil
}

func effectivePeriodChangeResult(resolved int64, changed bool) (int64, bool, error) {
	if !changed {
		return 0, false, nil
	}
	if resolved == 0 {
		return 0, true, careOfferingInvalidf("timetable template schedules must resolve one calendar_period_id after the change")
	}
	return resolved, true, nil
}

func periodReferenceAfterChange(current *int64, periodID int64, deleting bool) *int64 {
	if current == nil || (deleting && *current == periodID) {
		return nil
	}
	id := *current
	return &id
}

func referencesPeriod(current *int64, periodID int64) bool {
	return current != nil && *current == periodID
}

func (c *CareOfferingCatalog) calendarPeriodAfterChange(ctx context.Context, resolvedPeriodID int64, change periodChange) (*careplan.LinkedPeriod, error) {
	if change.replacement != nil && resolvedPeriodID == change.periodID {
		return change.replacement, nil
	}
	if period, ok := change.periods[resolvedPeriodID]; ok {
		return &period, nil
	}
	period, err := c.deps.Calendar.FindPeriod(ctx, resolvedPeriodID)
	if err != nil {
		if errors.Is(err, ports.ErrCatalogRowNotFound) {
			return nil, careOfferingInvalidf("calendar period for timetable template not found")
		}
		return nil, fmt.Errorf("load timetable template calendar period: %w", err)
	}
	change.periods[resolvedPeriodID] = period
	return &period, nil
}

func careOfferingPeriodConflictf(offeringID int64, cause error) error {
	return fmt.Errorf("%w: care offering %d would become incompatible: %w",
		careplan.ErrCalendarPeriodCareOfferingConflict, offeringID, cause)
}
