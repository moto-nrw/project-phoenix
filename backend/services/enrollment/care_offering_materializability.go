package enrollment

import (
	"context"
	"errors"
	"fmt"

	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
)

type careOfferingMaterializationDeps struct {
	timeframeRepo         scheduleModels.TimeframeRepository
	activityExceptionRepo scheduleModels.ActivityExceptionRepository
}

func (d careOfferingMaterializationDeps) validate() error {
	if d.timeframeRepo == nil || d.activityExceptionRepo == nil {
		return errors.New("care offering materialization dependencies are not configured")
	}
	return nil
}

func (s *careOfferingService) materializationDeps() careOfferingMaterializationDeps {
	return careOfferingMaterializationDeps{
		timeframeRepo:         s.TimeframeRepo,
		activityExceptionRepo: s.ActivityExceptionRepo,
	}
}

type careOfferingMaterializationChange struct {
	deletedRoomID        int64
	changedTimeframeID   int64
	timeframeReplacement *scheduleModels.Timeframe
}

type careOfferingExceptionKey struct {
	groupID int64
	date    timezone.Date
}

type careOfferingMaterializationState struct {
	timeframes map[int64]*scheduleModels.Timeframe
	exceptions map[careOfferingExceptionKey]*scheduleModels.ActivityException
	change     careOfferingMaterializationChange
}

func loadCareOfferingMaterializationState(
	ctx context.Context,
	deps careOfferingMaterializationDeps,
	phase *enrollmentOwner.Phase,
	change careOfferingMaterializationChange,
) (*careOfferingMaterializationState, error) {
	if err := deps.validate(); err != nil {
		return nil, err
	}
	if phase == nil {
		return nil, errors.New("care offering materialization phase is required")
	}
	timeframes, err := deps.timeframeRepo.ListAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("load timeframes for care offering materialization: %w", err)
	}
	exceptions, err := deps.activityExceptionRepo.FindByDateRange(
		ctx,
		scheduleModels.Date(timezone.Date(phase.ServiceStartDate)),
		scheduleModels.Date(timezone.Date(phase.ServiceEndDate)),
	)
	if err != nil {
		return nil, fmt.Errorf("load exceptions for care offering materialization: %w", err)
	}

	state := &careOfferingMaterializationState{
		timeframes: make(map[int64]*scheduleModels.Timeframe, len(timeframes)),
		exceptions: make(map[careOfferingExceptionKey]*scheduleModels.ActivityException, len(exceptions)),
		change:     change,
	}
	for _, timeframe := range timeframes {
		if timeframe == nil {
			continue
		}
		if timeframe.ID == change.changedTimeframeID {
			if change.timeframeReplacement != nil {
				state.timeframes[timeframe.ID] = change.timeframeReplacement
			}
			continue
		}
		state.timeframes[timeframe.ID] = timeframe
	}
	for _, exception := range exceptions {
		if exception == nil {
			continue
		}
		state.exceptions[careOfferingExceptionKey{
			groupID: exception.ActivityGroupID,
			date:    timezone.Date(exception.ExceptionDate),
		}] = exception
	}
	return state, nil
}

func validateCareOfferingMaterializability(
	ctx context.Context,
	deps careOfferingMaterializationDeps,
	segments []linkedCareOfferingGroup,
	phase *enrollmentOwner.Phase,
	days []string,
	change careOfferingMaterializationChange,
) error {
	if len(segments) == 0 || phase == nil || segments[0].group == nil || !segments[0].group.IsTemplate {
		return nil
	}
	weekdays, err := parseCareOfferingWeekdays(days)
	if err != nil {
		return err
	}
	state, err := loadCareOfferingMaterializationState(ctx, deps, phase, change)
	if err != nil {
		return err
	}
	for weekday := activitiesModels.WeekdayMonday; weekday <= activitiesModels.WeekdaySunday; weekday++ {
		if !weekdays[weekday] {
			continue
		}
		if err := validateCareOfferingWeekdayMaterializability(segments, phase, weekday, state); err != nil {
			return err
		}
	}
	return nil
}

func validateCareOfferingWeekdayMaterializability(
	segments []linkedCareOfferingGroup,
	phase *enrollmentOwner.Phase,
	weekday int,
	state *careOfferingMaterializationState,
) error {
	for date := timezone.Date(phase.ServiceStartDate); !date.After(timezone.Date(phase.ServiceEndDate)); date = date.AddDays(1) {
		if careOfferingISOWeekday(date) != weekday {
			continue
		}
		complete, allCancelled, hasCompleteTimeframe, hasEffectiveRoom :=
			careOfferingOccurrenceMaterializable(segments, date, weekday, state)
		if complete || allCancelled {
			continue
		}
		switch {
		case !hasCompleteTimeframe:
			return careOfferingInvalidf("timetable occurrence on %s has no complete timeframe", date.String())
		case !hasEffectiveRoom:
			return careOfferingInvalidf("timetable occurrence on %s has no effective room", date.String())
		default:
			return careOfferingInvalidf("timetable occurrence on %s has invalid effective start/end time", date.String())
		}
	}
	return nil
}

func careOfferingOccurrenceMaterializable(
	segments []linkedCareOfferingGroup,
	date timezone.Date,
	weekday int,
	state *careOfferingMaterializationState,
) (complete, allCancelled, hasCompleteTimeframe, hasEffectiveRoom bool) {
	candidates := 0
	cancelled := 0
	for _, segment := range segments {
		for _, schedule := range segment.schedules {
			evaluation, candidate := evaluateCareOfferingMaterializationCandidate(
				segment,
				schedule,
				date,
				weekday,
				state,
			)
			if !candidate {
				continue
			}
			candidates++
			if evaluation.cancelled {
				cancelled++
				continue
			}
			hasCompleteTimeframe = hasCompleteTimeframe || evaluation.completeTimeframe
			hasEffectiveRoom = hasEffectiveRoom || evaluation.effectiveRoom
			if evaluation.complete {
				return true, false, true, true
			}
		}
	}
	return false, candidates > 0 && candidates == cancelled, hasCompleteTimeframe, hasEffectiveRoom
}

type careOfferingCandidateEvaluation struct {
	cancelled         bool
	completeTimeframe bool
	effectiveRoom     bool
	complete          bool
}

func evaluateCareOfferingMaterializationCandidate(
	segment linkedCareOfferingGroup,
	schedule *activitiesModels.Schedule,
	date timezone.Date,
	weekday int,
	state *careOfferingMaterializationState,
) (careOfferingCandidateEvaluation, bool) {
	if !isCareOfferingMaterializationCandidate(segment, schedule, date, weekday) {
		return careOfferingCandidateEvaluation{}, false
	}
	exception := state.exceptions[careOfferingExceptionKey{groupID: segment.group.ID, date: date}]
	if exception != nil && exception.IsCancellation() {
		return careOfferingCandidateEvaluation{cancelled: true}, true
	}
	timeframe := careOfferingScheduleTimeframe(schedule, state)
	if timeframe == nil || timeframe.EndTime == nil {
		return careOfferingCandidateEvaluation{}, true
	}
	evaluation := careOfferingCandidateEvaluation{completeTimeframe: true}
	evaluation.effectiveRoom = careOfferingEffectiveRoomID(
		segment.group,
		exception,
		state.change.deletedRoomID,
	) > 0
	if !evaluation.effectiveRoom {
		return evaluation, true
	}
	evaluation.complete = careOfferingEffectiveTimesValid(timeframe, exception)
	return evaluation, true
}

func isCareOfferingMaterializationCandidate(
	segment linkedCareOfferingGroup,
	schedule *activitiesModels.Schedule,
	date timezone.Date,
	weekday int,
) bool {
	if segment.group == nil || segment.period == nil || !segment.period.IsActive || schedule == nil {
		return false
	}
	return schedule.Weekday == weekday &&
		scheduleCoversDate(schedule, date) &&
		scheduleService.ShouldMaterializeWeekPattern(schedule.WeekPattern, date, segment.period)
}

func careOfferingScheduleTimeframe(
	schedule *activitiesModels.Schedule,
	state *careOfferingMaterializationState,
) *scheduleModels.Timeframe {
	if schedule == nil || schedule.TimeframeID == nil || *schedule.TimeframeID <= 0 {
		return nil
	}
	return state.timeframes[*schedule.TimeframeID]
}

func careOfferingEffectiveRoomID(
	group *activitiesModels.Group,
	exception *scheduleModels.ActivityException,
	deletedRoomID int64,
) int64 {
	roomID := int64(0)
	if group != nil && group.PlannedRoomID != nil && *group.PlannedRoomID != deletedRoomID {
		roomID = *group.PlannedRoomID
	}
	if exception != nil && exception.ExceptionType == scheduleModels.ActivityExceptionModified &&
		exception.RoomID != nil && *exception.RoomID != deletedRoomID {
		roomID = *exception.RoomID
	}
	return roomID
}

func careOfferingEffectiveTimesValid(
	timeframe *scheduleModels.Timeframe,
	exception *scheduleModels.ActivityException,
) bool {
	if timeframe == nil || timeframe.EndTime == nil {
		return false
	}
	start := timeframe.StartTime
	end := *timeframe.EndTime
	if exception != nil && exception.ExceptionType == scheduleModels.ActivityExceptionModified {
		if exception.StartTime != nil {
			start = *exception.StartTime
		}
		if exception.EndTime != nil {
			end = *exception.EndTime
		}
	}
	return timezone.NormalizeWallClock(end).After(timezone.NormalizeWallClock(start))
}

// ValidateRoomDeletion rejects deletion only when the room participates in a
// materializable care-offering series and clearing its planned/exception room
// references would leave at least one non-cancelled occurrence without a room.
func (s *careOfferingService) ValidateRoomDeletion(ctx context.Context, roomID int64) error {
	if roomID <= 0 {
		return careOfferingInvalidf("room id must be positive")
	}
	return s.validateMaterializationResourceDeletion(ctx, careOfferingMaterializationChange{deletedRoomID: roomID})
}

// ValidateTimeframeDeletion simulates activities.schedules.timeframe_id being
// cleared by ON DELETE SET NULL before the owning schedule service deletes it.
func (s *careOfferingService) ValidateTimeframeDeletion(ctx context.Context, timeframeID int64) error {
	return s.ValidateTimeframeChange(ctx, timeframeID, nil)
}

// ValidateTimeframeChange simulates an update (replacement non-nil) or delete
// (replacement nil) before the schedule service mutates the timeframe row.
func (s *careOfferingService) ValidateTimeframeChange(
	ctx context.Context,
	timeframeID int64,
	replacement *scheduleModels.Timeframe,
) error {
	if timeframeID <= 0 {
		return careOfferingInvalidf("timeframe id must be positive")
	}
	if replacement != nil && replacement.ID != timeframeID {
		return careOfferingInvalidf(
			"timeframe replacement id %d does not match changed timeframe %d",
			replacement.ID,
			timeframeID,
		)
	}
	return s.validateMaterializationResourceDeletion(ctx, careOfferingMaterializationChange{
		changedTimeframeID:   timeframeID,
		timeframeReplacement: replacement,
	})
}

// ValidatePhaseChange evaluates every materializable offering owned by the
// phase against the proposed service window before PhaseService persists it.
func (s *careOfferingService) ValidatePhaseChange(
	ctx context.Context,
	phaseID int64,
	replacement *enrollmentOwner.Phase,
) error {
	if phaseID <= 0 || replacement == nil || replacement.ID != phaseID {
		return careOfferingInvalidf("phase replacement must match a positive phase id")
	}
	if s.Repo == nil {
		return errors.New("care offering phase validation repository is not configured")
	}
	offerings, err := s.Repo.ListByPhase(ctx, phaseID)
	if err != nil {
		return fmt.Errorf("list care offerings for phase change validation: %w", err)
	}
	for _, offering := range offerings {
		if err := s.validateOfferingPhaseChange(ctx, offering, replacement); err != nil {
			return err
		}
	}
	return nil
}

func (s *careOfferingService) validateOfferingPhaseChange(
	ctx context.Context,
	offering *enrollmentModels.CareOffering,
	replacement *enrollmentOwner.Phase,
) error {
	if offering == nil || offering.ActivityGroupID == nil {
		return nil
	}
	required, err := s.offeringRequiresMaterialization(ctx, offering)
	if err != nil {
		return fmt.Errorf("inspect care offering %d request selections: %w", offering.ID, err)
	}
	if !required {
		return nil
	}
	segments, err := resolveCareOfferingLinkedGroupsForPhase(ctx, careOfferingTemplateDeps{
		activityGroupRepo:    s.ActivityGroupRepo,
		activityScheduleRepo: s.ActivityScheduleRepo,
		calendarPeriodRepo:   s.CalendarPeriodRepo,
	}, *offering.ActivityGroupID, replacement)
	if err != nil {
		return fmt.Errorf("care offering %d is incompatible with the updated phase: %w", offering.ID, err)
	}
	if err := validateCareOfferingTemplateSegments(segments, replacement, offering.AvailableDays, true); err != nil {
		return fmt.Errorf("care offering %d is incompatible with the updated phase: %w", offering.ID, err)
	}
	if err := validateCareOfferingMaterializability(
		ctx,
		s.materializationDeps(),
		segments,
		replacement,
		offering.AvailableDays,
		careOfferingMaterializationChange{},
	); err != nil {
		return fmt.Errorf("care offering %d is incompatible with the updated phase: %w", offering.ID, err)
	}
	return nil
}

func (s *careOfferingService) validateMaterializationResourceDeletion(
	ctx context.Context,
	change careOfferingMaterializationChange,
) error {
	if s.Repo == nil || s.ActivityGroupRepo == nil || s.ActivityScheduleRepo == nil ||
		s.Phases == nil {
		return errors.New("care offering resource deletion validation dependencies are not configured")
	}
	offerings, err := s.Repo.ListByTenant(ctx)
	if err != nil {
		return fmt.Errorf("list care offerings for materialization resource deletion: %w", err)
	}
	for _, offering := range offerings {
		if err := s.validateOfferingResourceDeletion(ctx, offering, change); err != nil {
			return err
		}
	}
	return nil
}

func (s *careOfferingService) validateOfferingResourceDeletion(
	ctx context.Context,
	offering *enrollmentModels.CareOffering,
	change careOfferingMaterializationChange,
) error {
	if offering == nil || offering.ActivityGroupID == nil {
		return nil
	}
	required, err := s.offeringRequiresMaterialization(ctx, offering)
	if err != nil {
		return fmt.Errorf("inspect care offering %d request selections: %w", offering.ID, err)
	}
	if !required {
		return nil
	}
	impacted, err := s.offeringReferencesMaterializationChange(ctx, *offering.ActivityGroupID, change)
	if err != nil {
		return fmt.Errorf("inspect care offering %d timetable resources: %w", offering.ID, err)
	}
	if !impacted {
		return nil
	}
	phase, err := s.Phases.Phase(ctx, offering.PhaseID)
	if err != nil {
		return fmt.Errorf("load care offering %d phase for resource deletion: %w", offering.ID, err)
	}
	segments, err := resolveCareOfferingLinkedGroupsForPhase(ctx, careOfferingTemplateDeps{
		activityGroupRepo:    s.ActivityGroupRepo,
		activityScheduleRepo: s.ActivityScheduleRepo,
		calendarPeriodRepo:   s.CalendarPeriodRepo,
	}, *offering.ActivityGroupID, phase)
	if err != nil {
		return fmt.Errorf("care offering %d would become non-materializable: %w", offering.ID, err)
	}
	if err := validateCareOfferingTemplateSegments(segments, phase, offering.AvailableDays, true); err != nil {
		return fmt.Errorf("care offering %d would become non-materializable: %w", offering.ID, err)
	}
	if err := validateCareOfferingMaterializability(
		ctx,
		s.materializationDeps(),
		segments,
		phase,
		offering.AvailableDays,
		change,
	); err != nil {
		return fmt.Errorf("care offering %d would become non-materializable: %w", offering.ID, err)
	}
	return nil
}

func (s *careOfferingService) offeringReferencesMaterializationChange(
	ctx context.Context,
	groupID int64,
	change careOfferingMaterializationChange,
) (bool, error) {
	group, err := s.ActivityGroupRepo.FindByID(ctx, groupID)
	if err != nil {
		return false, err
	}
	if group == nil || !group.IsTemplate {
		return false, nil
	}
	series, err := s.ActivityGroupRepo.FindTemplateSeries(ctx, groupID)
	if err != nil {
		return false, err
	}
	series = ensureCareOfferingSeriesContainsGroup(series, group)
	for _, segment := range series {
		references, err := s.segmentReferencesMaterializationChange(ctx, segment, change)
		if err != nil {
			return false, err
		}
		if references {
			return true, nil
		}
	}
	return false, nil
}

func ensureCareOfferingSeriesContainsGroup(
	series []*activitiesModels.Group,
	group *activitiesModels.Group,
) []*activitiesModels.Group {
	for _, segment := range series {
		if segment != nil && group != nil && segment.ID == group.ID {
			return series
		}
	}
	return append(series, group)
}

func (s *careOfferingService) segmentReferencesMaterializationChange(
	ctx context.Context,
	group *activitiesModels.Group,
	change careOfferingMaterializationChange,
) (bool, error) {
	if group == nil {
		return false, nil
	}
	if change.deletedRoomID > 0 && group.PlannedRoomID != nil && *group.PlannedRoomID == change.deletedRoomID {
		return true, nil
	}
	schedules, err := s.ActivityScheduleRepo.FindByGroupID(ctx, group.ID)
	if err != nil {
		return false, err
	}
	for _, schedule := range schedules {
		if schedule != nil && schedule.TimeframeID != nil && *schedule.TimeframeID == change.changedTimeframeID {
			return true, nil
		}
	}
	if change.deletedRoomID <= 0 {
		return false, nil
	}
	exceptions, err := s.ActivityExceptionRepo.FindByActivityGroupID(ctx, group.ID)
	if err != nil {
		return false, err
	}
	for _, exception := range exceptions {
		if exception != nil && exception.RoomID != nil && *exception.RoomID == change.deletedRoomID {
			return true, nil
		}
	}
	return false, nil
}
