package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

const (
	isoMonday = 1
	isoFriday = 5
	isoSunday = 7
)

// resolveCareOfferingTemplatePeriod loads a linked group that must be a live
// timetable template and resolves the one period its schedules share.
func (c *CareOfferingCatalog) resolveCareOfferingTemplatePeriod(ctx context.Context, activityGroupID int64) (careplan.LinkedPeriod, error) {
	if activityGroupID <= 0 {
		return careplan.LinkedPeriod{}, careOfferingInvalidf("activity_group_id must be positive when set")
	}
	group, err := c.deps.Timetable.FindGroup(ctx, activityGroupID)
	if err != nil {
		if errors.Is(err, ports.ErrCatalogRowNotFound) {
			return careplan.LinkedPeriod{}, careOfferingInvalidf("activity_group_id does not reference a template in this tenant")
		}
		return careplan.LinkedPeriod{}, fmt.Errorf("load linked activity group: %w", err)
	}
	if !group.IsTemplate {
		return careplan.LinkedPeriod{}, careOfferingInvalidf("activity_group_id must reference a timetable template")
	}
	if group.Archived {
		return careplan.LinkedPeriod{}, careOfferingInvalidf("activity_group_id references an archived timetable template")
	}
	return c.resolveTemplatePeriodForGroup(ctx, group)
}

func (c *CareOfferingCatalog) resolveTemplatePeriodForGroup(ctx context.Context, group careplan.LinkedGroup) (careplan.LinkedPeriod, error) {
	if !group.IsTemplate {
		return careplan.LinkedPeriod{}, careOfferingInvalidf("activity group must be a timetable template")
	}
	schedules, err := c.deps.Timetable.GroupSchedules(ctx, []int64{group.ID})
	if err != nil {
		return careplan.LinkedPeriod{}, fmt.Errorf("load timetable template schedules: %w", err)
	}
	if len(schedules) == 0 {
		return careplan.LinkedPeriod{}, careOfferingInvalidf("timetable template must have at least one schedule")
	}
	periodID, err := resolveTemplateSchedulePeriodID(group, schedules)
	if err != nil {
		return careplan.LinkedPeriod{}, err
	}
	period, err := c.deps.Calendar.FindPeriod(ctx, periodID)
	if err != nil {
		if errors.Is(err, ports.ErrCatalogRowNotFound) {
			return careplan.LinkedPeriod{}, careOfferingInvalidf("calendar period for timetable template not found")
		}
		return careplan.LinkedPeriod{}, fmt.Errorf("load timetable template calendar period: %w", err)
	}
	return period, nil
}

func resolveTemplateSchedulePeriodID(group careplan.LinkedGroup, schedules []careplan.LinkedSchedule) (int64, error) {
	var periodID *int64
	for _, schedule := range schedules {
		resolvedPeriodID := schedule.CalendarPeriodID
		if resolvedPeriodID == nil {
			resolvedPeriodID = group.CalendarPeriodID
		}
		if resolvedPeriodID == nil {
			return 0, careOfferingInvalidf("timetable template schedules must resolve one calendar_period_id from the schedule or template")
		}
		if periodID == nil {
			id := *resolvedPeriodID
			periodID = &id
			continue
		}
		if *periodID != *resolvedPeriodID {
			return 0, careOfferingInvalidf("timetable template schedules must use one calendar_period_id")
		}
	}
	return *periodID, nil
}

// resolveCareOfferingLinkedGroupsForPhase expands a template link to every
// live split-series segment whose recurrence window can produce an
// occurrence during the enrollment phase. A non-template activity remains
// one group.
func (c *CareOfferingCatalog) resolveCareOfferingLinkedGroupsForPhase(
	ctx context.Context,
	activityGroupID int64,
	phase careplan.OfferingPhase,
) ([]careplan.LinkedSegment, error) {
	group, period, err := c.resolveCareOfferingLinkedGroupPeriod(ctx, activityGroupID)
	if err != nil {
		return nil, err
	}
	if !group.IsTemplate {
		return []careplan.LinkedSegment{{Group: group}}, nil
	}
	series, err := c.deps.Timetable.TemplateSeries(ctx, activityGroupID)
	if err != nil {
		return nil, fmt.Errorf("load timetable template split series: %w", err)
	}
	segments := make([]careplan.LinkedSegment, 0, len(series))
	for _, segment := range series {
		linked, include, resolveErr := c.resolveCareOfferingSeriesSegment(ctx, group, period, segment, phase)
		if resolveErr != nil {
			return nil, resolveErr
		}
		if include {
			segments = append(segments, linked)
		}
	}
	if len(segments) == 0 {
		return nil, careOfferingInvalidf("timetable template has no recurrence segment during the enrollment phase")
	}
	return segments, nil
}

func (c *CareOfferingCatalog) resolveCareOfferingSeriesSegment(
	ctx context.Context,
	root careplan.LinkedGroup,
	rootPeriod *careplan.LinkedPeriod,
	segment careplan.LinkedGroup,
	phase careplan.OfferingPhase,
) (careplan.LinkedSegment, bool, error) {
	schedules, err := c.deps.Timetable.GroupSchedules(ctx, []int64{segment.ID})
	if err != nil {
		return careplan.LinkedSegment{}, false, fmt.Errorf("load split-series segment %d schedules: %w", segment.ID, err)
	}
	if !careplan.SchedulesOverlapPhase(schedules, phase) {
		return careplan.LinkedSegment{}, false, nil
	}
	period := rootPeriod
	if segment.ID != root.ID {
		resolved, err := c.resolveTemplatePeriodForGroup(ctx, segment)
		if err != nil {
			return careplan.LinkedSegment{}, false, fmt.Errorf("resolve split-series segment %d period: %w", segment.ID, err)
		}
		period = &resolved
	}
	if err := careplan.ValidatePhaseWithinPeriod(phase, period); err != nil {
		return careplan.LinkedSegment{}, false, err
	}
	return careplan.LinkedSegment{Group: segment, Period: period, Schedules: schedules}, true, nil
}

func (c *CareOfferingCatalog) resolveCareOfferingLinkedGroupPeriod(
	ctx context.Context,
	activityGroupID int64,
) (careplan.LinkedGroup, *careplan.LinkedPeriod, error) {
	if activityGroupID <= 0 {
		return careplan.LinkedGroup{}, nil, careOfferingInvalidf("activity_group_id must be positive when set")
	}
	group, err := c.deps.Timetable.FindGroup(ctx, activityGroupID)
	if err != nil {
		if errors.Is(err, ports.ErrCatalogRowNotFound) {
			return careplan.LinkedGroup{}, nil, careOfferingInvalidf("activity_group_id does not reference a group in this tenant")
		}
		return careplan.LinkedGroup{}, nil, fmt.Errorf("load linked activity group: %w", err)
	}
	if !group.IsTemplate {
		return group, nil, nil
	}
	period, err := c.resolveCareOfferingTemplatePeriod(ctx, activityGroupID)
	if err != nil {
		return careplan.LinkedGroup{}, nil, err
	}
	return group, &period, nil
}

// validateCareOfferingTemplateSegments checks that every selected weekday
// is covered by a segment on each of its dates in the phase.
func validateCareOfferingTemplateSegments(
	calendarPort ports.CatalogCalendar,
	segments []careplan.LinkedSegment,
	phase careplan.OfferingPhase,
	days []string,
	requireActivePeriod bool,
) error {
	if len(segments) == 0 || !segments[0].Group.IsTemplate {
		return nil
	}
	weekdays, err := parseCareOfferingWeekdays(days)
	if err != nil {
		return err
	}
	for weekday := isoMonday; weekday <= isoSunday; weekday++ {
		if !weekdays[weekday] {
			continue
		}
		if err := validateCareOfferingWeekdayCoverage(calendarPort, segments, phase, weekday, requireActivePeriod); err != nil {
			return err
		}
	}
	return nil
}

func parseCareOfferingWeekdays(days []string) (map[int]bool, error) {
	weekdays := make(map[int]bool, len(days))
	for _, day := range days {
		weekday, ok := offeringDayWeekday(day)
		if !ok {
			return nil, careOfferingInvalidf("care offering day %q is invalid", day)
		}
		weekdays[weekday] = true
	}
	if len(weekdays) == 0 {
		return nil, careOfferingInvalidf("a timetable-linked care offering must define at least one available day")
	}
	return weekdays, nil
}

func validateCareOfferingWeekdayCoverage(
	calendarPort ports.CatalogCalendar,
	segments []careplan.LinkedSegment,
	phase careplan.OfferingPhase,
	weekday int,
	requireActivePeriod bool,
) error {
	occurrences := 0
	for date := phase.ServiceStart; !date.After(phase.ServiceEnd); date = date.AddDays(1) {
		if careOfferingISOWeekday(date) != weekday {
			continue
		}
		occurrences++
		covered, inactiveOnly := careOfferingOccurrenceCovered(calendarPort, segments, date, weekday, requireActivePeriod)
		if covered {
			continue
		}
		if inactiveOnly {
			return careOfferingInvalidf("an active timetable-linked care offering requires an active calendar period")
		}
		return careOfferingInvalidf("timetable recurrence does not cover care offering weekday %d on %s", weekday, date.String())
	}
	if occurrences == 0 {
		return careOfferingInvalidf("care offering weekday %d has no occurrence during the enrollment phase", weekday)
	}
	return nil
}

func careOfferingISOWeekday(date calendar.Date) int {
	weekday := int(date.Weekday())
	if weekday == 0 {
		return isoSunday
	}
	return weekday
}

func careOfferingOccurrenceCovered(
	calendarPort ports.CatalogCalendar,
	segments []careplan.LinkedSegment,
	date calendar.Date,
	weekday int,
	requireActivePeriod bool,
) (covered bool, inactiveOnly bool) {
	for _, segment := range segments {
		for _, schedule := range segment.Schedules {
			if schedule.Weekday != weekday || !scheduleCoversDate(schedule, date) ||
				!weekPatternApplies(calendarPort, schedule.WeekPattern, date, segment.Period) {
				continue
			}
			if requireActivePeriod && (segment.Period == nil || !segment.Period.IsActive) {
				inactiveOnly = true
				continue
			}
			return true, false
		}
	}
	return false, inactiveOnly
}

func scheduleCoversDate(schedule careplan.LinkedSchedule, date calendar.Date) bool {
	if schedule.ValidFrom != nil && schedule.ValidFrom.After(date) {
		return false
	}
	return schedule.ValidUntil == nil || schedule.ValidUntil.After(date)
}

// weekPatternApplies asks the School Calendar's A/B-week engine whether a
// schedule with weekPattern occurs on date inside period, so a catalog link
// is never accepted for a day the materializer would skip. A nil period
// means no alternation is configured.
func weekPatternApplies(calendarPort ports.CatalogCalendar, weekPattern int, date calendar.Date, period *careplan.LinkedPeriod) bool {
	if period == nil {
		return true
	}
	return calendarPort.WeekPatternApplies(weekPattern, date, *period)
}

// validateLinkedTemplate checks the admin catalog's template-only link
// contract. Decision materialization separately supports historical
// non-template links.
func (c *CareOfferingCatalog) validateLinkedTemplate(ctx context.Context, offering careplan.CareOffering) error {
	if offering.ActivityGroupID == nil {
		return nil
	}
	if _, err := c.resolveCareOfferingTemplatePeriod(ctx, *offering.ActivityGroupID); err != nil {
		return err
	}
	phase, err := c.deps.Phases.Phase(ctx, offering.PhaseID)
	if err != nil {
		if errors.Is(err, ports.ErrCatalogRowNotFound) {
			return careOfferingInvalidf("phase_id does not reference a phase in this tenant")
		}
		return fmt.Errorf("load care offering phase: %w", err)
	}
	requiresMaterialization, err := c.offeringRequiresMaterialization(ctx, offering)
	if err != nil {
		return err
	}
	return c.validateLinkedTemplateForMaterialization(ctx, offering, phase, requiresMaterialization)
}

func (c *CareOfferingCatalog) validateLinkedTemplateForMaterialization(
	ctx context.Context,
	offering careplan.CareOffering,
	phase careplan.OfferingPhase,
	requiresMaterialization bool,
) error {
	segments, err := c.resolveCareOfferingLinkedGroupsForPhase(ctx, *offering.ActivityGroupID, phase)
	if err != nil {
		return err
	}
	if err := validateCareOfferingTemplateSegments(c.deps.Calendar, segments, phase, offering.AvailableDays, requiresMaterialization); err != nil {
		return err
	}
	if !requiresMaterialization {
		return nil
	}
	return c.validateCareOfferingMaterializability(ctx, segments, phase, offering.AvailableDays, careOfferingMaterializationChange{})
}

// offeringRequiresMaterialization protects catalog-visible offerings and
// inactive offerings still selected by a non-terminal enrollment request.
// Deactivation is not a cancellation: a later decision materializes the
// persisted selection, so recurrence mutations must keep that path valid.
func (c *CareOfferingCatalog) offeringRequiresMaterialization(ctx context.Context, offering careplan.CareOffering) (bool, error) {
	if offering.IsActive {
		return true, nil
	}
	// A not-yet-created inactive draft cannot have request selections.
	if offering.ID <= 0 {
		return false, nil
	}
	count, err := c.deps.Bookings.MaterializableOfferingCount(ctx, offering.ID, calendar.TodayDate())
	if err != nil {
		return false, fmt.Errorf("count materializable care offering selections: %w", err)
	}
	return count > 0, nil
}
