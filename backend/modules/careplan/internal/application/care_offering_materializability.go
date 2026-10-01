package application

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// careOfferingMaterializationChange is the resource edit a guard simulates:
// a room being deleted or a timeframe being edited (replacement set) or
// deleted (replacement nil).
type careOfferingMaterializationChange struct {
	deletedRoomID        int64
	changedTimeframeID   int64
	timeframeReplacement *careplan.LinkedTimeframe
}

type careOfferingExceptionKey struct {
	groupID int64
	date    calendar.Date
}

type careOfferingMaterializationState struct {
	timeframes map[int64]careplan.LinkedTimeframe
	exceptions map[careOfferingExceptionKey]careplan.LinkedException
	change     careOfferingMaterializationChange
}

func (c *CareOfferingCatalog) loadCareOfferingMaterializationState(
	ctx context.Context,
	phase careplan.OfferingPhase,
	change careOfferingMaterializationChange,
) (*careOfferingMaterializationState, error) {
	timeframes, err := c.deps.Timetable.Timeframes(ctx)
	if err != nil {
		return nil, fmt.Errorf("load timeframes for care offering materialization: %w", err)
	}
	exceptions, err := c.deps.Timetable.ExceptionsBetween(ctx, phase.ServiceStart, phase.ServiceEnd)
	if err != nil {
		return nil, fmt.Errorf("load exceptions for care offering materialization: %w", err)
	}
	state := &careOfferingMaterializationState{
		timeframes: make(map[int64]careplan.LinkedTimeframe, len(timeframes)),
		exceptions: make(map[careOfferingExceptionKey]careplan.LinkedException, len(exceptions)),
		change:     change,
	}
	for _, timeframe := range timeframes {
		if timeframe.ID == change.changedTimeframeID {
			if change.timeframeReplacement != nil {
				state.timeframes[timeframe.ID] = *change.timeframeReplacement
			}
			continue
		}
		state.timeframes[timeframe.ID] = timeframe
	}
	for _, exception := range exceptions {
		state.exceptions[careOfferingExceptionKey{groupID: exception.GroupID, date: exception.Date}] = exception
	}
	return state, nil
}

// validateCareOfferingMaterializability checks that every date of a selected
// weekday yields a complete occurrence: a timeframe with an end, an
// effective room and valid effective times — unless every candidate is
// cancelled.
func (c *CareOfferingCatalog) validateCareOfferingMaterializability(
	ctx context.Context,
	segments []careplan.LinkedSegment,
	phase careplan.OfferingPhase,
	days []string,
	change careOfferingMaterializationChange,
) error {
	if len(segments) == 0 || !segments[0].Group.IsTemplate {
		return nil
	}
	weekdays, err := parseCareOfferingWeekdays(days)
	if err != nil {
		return err
	}
	state, err := c.loadCareOfferingMaterializationState(ctx, phase, change)
	if err != nil {
		return err
	}
	for weekday := isoMonday; weekday <= isoSunday; weekday++ {
		if !weekdays[weekday] {
			continue
		}
		if err := c.validateCareOfferingWeekdayMaterializability(segments, phase, weekday, state); err != nil {
			return err
		}
	}
	return nil
}

func (c *CareOfferingCatalog) validateCareOfferingWeekdayMaterializability(
	segments []careplan.LinkedSegment,
	phase careplan.OfferingPhase,
	weekday int,
	state *careOfferingMaterializationState,
) error {
	for date := phase.ServiceStart; !date.After(phase.ServiceEnd); date = date.AddDays(1) {
		if careOfferingISOWeekday(date) != weekday {
			continue
		}
		occurrence := c.careOfferingOccurrenceMaterializable(segments, date, weekday, state)
		if occurrence.complete || occurrence.allCancelled {
			continue
		}
		switch {
		case !occurrence.hasCompleteTimeframe:
			return careOfferingInvalidf("timetable occurrence on %s has no complete timeframe", date.String())
		case !occurrence.hasEffectiveRoom:
			return careOfferingInvalidf("timetable occurrence on %s has no effective room", date.String())
		default:
			return careOfferingInvalidf("timetable occurrence on %s has invalid effective start/end time", date.String())
		}
	}
	return nil
}

type careOfferingOccurrence struct {
	complete             bool
	allCancelled         bool
	hasCompleteTimeframe bool
	hasEffectiveRoom     bool
}

func (c *CareOfferingCatalog) careOfferingOccurrenceMaterializable(
	segments []careplan.LinkedSegment,
	date calendar.Date,
	weekday int,
	state *careOfferingMaterializationState,
) careOfferingOccurrence {
	var occurrence careOfferingOccurrence
	candidates := 0
	cancelled := 0
	for _, segment := range segments {
		for _, schedule := range segment.Schedules {
			evaluation, candidate := c.evaluateCareOfferingMaterializationCandidate(segment, schedule, date, weekday, state)
			if !candidate {
				continue
			}
			candidates++
			if evaluation.cancelled {
				cancelled++
				continue
			}
			occurrence.hasCompleteTimeframe = occurrence.hasCompleteTimeframe || evaluation.completeTimeframe
			occurrence.hasEffectiveRoom = occurrence.hasEffectiveRoom || evaluation.effectiveRoom
			if evaluation.complete {
				return careOfferingOccurrence{complete: true, hasCompleteTimeframe: true, hasEffectiveRoom: true}
			}
		}
	}
	occurrence.allCancelled = candidates > 0 && candidates == cancelled
	return occurrence
}

type careOfferingCandidateEvaluation struct {
	cancelled         bool
	completeTimeframe bool
	effectiveRoom     bool
	complete          bool
}

func (c *CareOfferingCatalog) evaluateCareOfferingMaterializationCandidate(
	segment careplan.LinkedSegment,
	schedule careplan.LinkedSchedule,
	date calendar.Date,
	weekday int,
	state *careOfferingMaterializationState,
) (careOfferingCandidateEvaluation, bool) {
	if !c.isCareOfferingMaterializationCandidate(segment, schedule, date, weekday) {
		return careOfferingCandidateEvaluation{}, false
	}
	exception, hasException := state.exceptions[careOfferingExceptionKey{groupID: segment.Group.ID, date: date}]
	var modification *careplan.LinkedException
	if hasException {
		if exception.Cancelled {
			return careOfferingCandidateEvaluation{cancelled: true}, true
		}
		if exception.Modified {
			modification = &exception
		}
	}
	timeframe, ok := careOfferingScheduleTimeframe(schedule, state)
	if !ok || timeframe.EndTime == nil {
		return careOfferingCandidateEvaluation{}, true
	}
	evaluation := careOfferingCandidateEvaluation{completeTimeframe: true}
	evaluation.effectiveRoom = careOfferingEffectiveRoomID(segment.Group, modification, state.change.deletedRoomID) > 0
	if !evaluation.effectiveRoom {
		return evaluation, true
	}
	evaluation.complete = careOfferingEffectiveTimesValid(timeframe, modification)
	return evaluation, true
}

func (c *CareOfferingCatalog) isCareOfferingMaterializationCandidate(
	segment careplan.LinkedSegment,
	schedule careplan.LinkedSchedule,
	date calendar.Date,
	weekday int,
) bool {
	if segment.Period == nil || !segment.Period.IsActive {
		return false
	}
	return schedule.Weekday == weekday &&
		scheduleCoversDate(schedule, date) &&
		weekPatternApplies(c.deps.Calendar, schedule.WeekPattern, date, segment.Period)
}

func careOfferingScheduleTimeframe(schedule careplan.LinkedSchedule, state *careOfferingMaterializationState) (careplan.LinkedTimeframe, bool) {
	if schedule.TimeframeID == nil || *schedule.TimeframeID <= 0 {
		return careplan.LinkedTimeframe{}, false
	}
	timeframe, ok := state.timeframes[*schedule.TimeframeID]
	return timeframe, ok
}

// careOfferingEffectiveRoomID resolves the room an occurrence runs in: the
// modification's room over the planned room, ignoring the room being deleted.
func careOfferingEffectiveRoomID(group careplan.LinkedGroup, modification *careplan.LinkedException, deletedRoomID int64) int64 {
	roomID := int64(0)
	if group.PlannedRoomID != nil && *group.PlannedRoomID != deletedRoomID {
		roomID = *group.PlannedRoomID
	}
	if modification != nil && modification.RoomID != nil && *modification.RoomID != deletedRoomID {
		roomID = *modification.RoomID
	}
	return roomID
}

func careOfferingEffectiveTimesValid(timeframe careplan.LinkedTimeframe, modification *careplan.LinkedException) bool {
	if timeframe.EndTime == nil {
		return false
	}
	start := timeframe.StartTime
	end := *timeframe.EndTime
	if modification != nil {
		if modification.StartTime != nil {
			start = *modification.StartTime
		}
		if modification.EndTime != nil {
			end = *modification.EndTime
		}
	}
	return calendar.NormalizeWallClock(end).After(calendar.NormalizeWallClock(start))
}

// ValidateRoomDeletion rejects a deletion only when the room participates in
// a materializable care-offering series and clearing its planned or
// exception references would leave a non-cancelled occurrence without a room.
func (c *CareOfferingCatalog) ValidateRoomDeletion(ctx context.Context, roomID int64) error {
	if roomID <= 0 {
		return careOfferingInvalidf("room id must be positive")
	}
	return c.validateMaterializationResourceDeletion(ctx, careOfferingMaterializationChange{deletedRoomID: roomID})
}

// ValidateTimeframeChange simulates an edit (replacement non-nil) or delete
// (replacement nil) before the Timetable owner mutates the timeframe row.
// activities.schedules.timeframe_id is cleared by ON DELETE SET NULL, so a
// deletion is simulated as that clearing.
func (c *CareOfferingCatalog) ValidateTimeframeChange(ctx context.Context, timeframeID int64, replacement *careplan.TimeframeReplacement) error {
	change := careOfferingMaterializationChange{changedTimeframeID: timeframeID}
	if replacement != nil {
		proposed, err := proposedTimeframe(timeframeID, *replacement)
		if err != nil {
			return err
		}
		change.timeframeReplacement = &proposed
	}
	if timeframeID <= 0 {
		return fmt.Errorf("validate timeframe %d change: %w", timeframeID, careOfferingInvalidf("timeframe id must be positive"))
	}
	if err := c.validateMaterializationResourceDeletion(ctx, change); err != nil {
		return fmt.Errorf("validate timeframe %d change: %w", timeframeID, err)
	}
	return nil
}

func proposedTimeframe(timeframeID int64, replacement careplan.TimeframeReplacement) (careplan.LinkedTimeframe, error) {
	start, err := parseTimeframeClock(replacement.StartTime)
	if err != nil {
		return careplan.LinkedTimeframe{}, careOfferingInvalidf("timeframe start time %q is not a clock value", replacement.StartTime)
	}
	proposed := careplan.LinkedTimeframe{ID: timeframeID, StartTime: start}
	if replacement.EndTime != nil {
		end, parseErr := parseTimeframeClock(*replacement.EndTime)
		if parseErr != nil {
			return careplan.LinkedTimeframe{}, careOfferingInvalidf("timeframe end time %q is not a clock value", *replacement.EndTime)
		}
		proposed.EndTime = &end
	}
	return proposed, nil
}

// parseTimeframeClock reads the Timetable owner's HH:MM:SS clock string, with
// or without the fractional second a stored row may carry.
func parseTimeframeClock(value string) (time.Time, error) {
	clock, err := time.Parse("15:04:05", value)
	if err == nil {
		return clock, nil
	}
	return time.Parse("15:04:05.999999999", value)
}

// ValidatePhaseChange evaluates every materializable offering of the phase
// against the proposed service window before the phase is saved.
func (c *CareOfferingCatalog) ValidatePhaseChange(ctx context.Context, phaseID int64, replacement *careplan.OfferingPhase) error {
	if phaseID <= 0 || replacement == nil || replacement.ID != phaseID {
		return careOfferingInvalidf("phase replacement must match a positive phase id")
	}
	offerings, err := c.listRecords(ctx, phaseFilter(phaseID), "failed to list care offerings by phase")
	if err != nil {
		return fmt.Errorf("list care offerings for phase change validation: %w", err)
	}
	for _, offering := range offerings {
		if err := c.validateOfferingPhaseChange(ctx, offering, *replacement); err != nil {
			return err
		}
	}
	return nil
}

func (c *CareOfferingCatalog) validateOfferingPhaseChange(ctx context.Context, offering careplan.CareOffering, replacement careplan.OfferingPhase) error {
	if offering.ActivityGroupID == nil {
		return nil
	}
	required, err := c.offeringRequiresMaterialization(ctx, offering)
	if err != nil {
		return fmt.Errorf("inspect care offering %d request selections: %w", offering.ID, err)
	}
	if !required {
		return nil
	}
	if err := c.validateLinkMaterializable(ctx, *offering.ActivityGroupID, replacement, offering.AvailableDays, careOfferingMaterializationChange{}); err != nil {
		return fmt.Errorf("care offering %d is incompatible with the updated phase: %w", offering.ID, err)
	}
	return nil
}

// validateLinkMaterializable resolves a link for the phase and checks both
// the weekday coverage (active period required) and the materializability.
func (c *CareOfferingCatalog) validateLinkMaterializable(
	ctx context.Context,
	activityGroupID int64,
	phase careplan.OfferingPhase,
	days []string,
	change careOfferingMaterializationChange,
) error {
	segments, err := c.resolveCareOfferingLinkedGroupsForPhase(ctx, activityGroupID, phase)
	if err != nil {
		return err
	}
	if err := validateCareOfferingTemplateSegments(c.deps.Calendar, segments, phase, days, true); err != nil {
		return err
	}
	return c.validateCareOfferingMaterializability(ctx, segments, phase, days, change)
}

func (c *CareOfferingCatalog) validateMaterializationResourceDeletion(ctx context.Context, change careOfferingMaterializationChange) error {
	offerings, err := c.listTenantRecords(ctx)
	if err != nil {
		return fmt.Errorf("list care offerings for materialization resource deletion: %w", err)
	}
	for _, offering := range offerings {
		if err := c.validateOfferingResourceDeletion(ctx, offering, change); err != nil {
			return err
		}
	}
	return nil
}

func (c *CareOfferingCatalog) validateOfferingResourceDeletion(ctx context.Context, offering careplan.CareOffering, change careOfferingMaterializationChange) error {
	if offering.ActivityGroupID == nil {
		return nil
	}
	required, err := c.offeringRequiresMaterialization(ctx, offering)
	if err != nil {
		return fmt.Errorf("inspect care offering %d request selections: %w", offering.ID, err)
	}
	if !required {
		return nil
	}
	impacted, err := c.offeringReferencesMaterializationChange(ctx, *offering.ActivityGroupID, change)
	if err != nil {
		return fmt.Errorf("inspect care offering %d timetable resources: %w", offering.ID, err)
	}
	if !impacted {
		return nil
	}
	phase, err := c.deps.Phases.Phase(ctx, offering.PhaseID)
	if err != nil {
		return fmt.Errorf("load care offering %d phase for resource deletion: %w", offering.ID, err)
	}
	if err := c.validateLinkMaterializable(ctx, *offering.ActivityGroupID, phase, offering.AvailableDays, change); err != nil {
		return fmt.Errorf("care offering %d would become non-materializable: %w", offering.ID, err)
	}
	return nil
}

func (c *CareOfferingCatalog) offeringReferencesMaterializationChange(ctx context.Context, groupID int64, change careOfferingMaterializationChange) (bool, error) {
	group, err := c.deps.Timetable.FindGroup(ctx, groupID)
	if err != nil {
		return false, err
	}
	if !group.IsTemplate {
		return false, nil
	}
	series, err := c.deps.Timetable.TemplateSeries(ctx, groupID)
	if err != nil {
		return false, err
	}
	for _, segment := range ensureCareOfferingSeriesContainsGroup(series, group) {
		references, err := c.segmentReferencesMaterializationChange(ctx, segment, change)
		if err != nil || references {
			return references, err
		}
	}
	return false, nil
}

func ensureCareOfferingSeriesContainsGroup(series []careplan.LinkedGroup, group careplan.LinkedGroup) []careplan.LinkedGroup {
	for _, segment := range series {
		if segment.ID == group.ID {
			return series
		}
	}
	return append(series, group)
}

func (c *CareOfferingCatalog) segmentReferencesMaterializationChange(ctx context.Context, group careplan.LinkedGroup, change careOfferingMaterializationChange) (bool, error) {
	if change.deletedRoomID > 0 && group.PlannedRoomID != nil && *group.PlannedRoomID == change.deletedRoomID {
		return true, nil
	}
	schedules, err := c.deps.Timetable.GroupSchedules(ctx, []int64{group.ID})
	if err != nil {
		return false, err
	}
	for _, schedule := range schedules {
		if schedule.TimeframeID != nil && *schedule.TimeframeID == change.changedTimeframeID {
			return true, nil
		}
	}
	if change.deletedRoomID <= 0 {
		return false, nil
	}
	exceptions, err := c.deps.Timetable.GroupExceptions(ctx, group.ID)
	if err != nil {
		return false, err
	}
	for _, exception := range exceptions {
		if exception.RoomID != nil && *exception.RoomID == change.deletedRoomID {
			return true, nil
		}
	}
	return false, nil
}
