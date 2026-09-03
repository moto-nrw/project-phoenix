package enrollment

import (
	"context"
	"errors"
	"fmt"

	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
)

// CareOfferingCalendarPeriodValidator is the narrow cross-domain contract
// used by calendar-period mutations. A nil replacement represents deletion;
// a non-nil replacement is the proposed post-update row.
type CareOfferingCalendarPeriodValidator interface {
	ValidateCalendarPeriodChange(
		ctx context.Context,
		periodID int64,
		replacement *scheduleModels.CalendarPeriod,
	) error
}

type loadedCareOfferingSegment struct {
	group     *activitiesModels.Group
	schedules []*activitiesModels.Schedule
}

// ValidateCalendarPeriodChange simulates the effective period of every
// care-offering-linked template segment after an update or delete. The caller
// holds the tenant recurrence lock and invokes this before changing the period
// row, so a rejected HTTP 409 cannot leave a partially mutated recurrence.
func (s *careOfferingService) ValidateCalendarPeriodChange(
	ctx context.Context,
	periodID int64,
	replacement *scheduleModels.CalendarPeriod,
) error {
	if periodID <= 0 {
		return errors.New("calendar period change validation requires a positive period id")
	}
	if replacement != nil && replacement.ID != periodID {
		return fmt.Errorf(
			"calendar period change validation received replacement id %d for period %d",
			replacement.ID,
			periodID,
		)
	}
	if s.Repo == nil || s.ActivityGroupRepo == nil || s.ActivityScheduleRepo == nil ||
		s.CalendarPeriodRepo == nil || s.PhaseRepo == nil {
		return errors.New("calendar period care-offering validation dependencies are not configured")
	}

	offerings, err := s.Repo.ListByTenant(ctx)
	if err != nil {
		return fmt.Errorf("list care offerings for calendar period validation: %w", err)
	}

	periods := make(map[int64]*scheduleModels.CalendarPeriod)
	for _, offering := range offerings {
		if offering == nil || offering.ActivityGroupID == nil {
			continue
		}
		if err := s.validateOfferingCalendarPeriodChange(
			ctx,
			offering,
			periodID,
			replacement,
			periods,
		); err != nil {
			return err
		}
	}
	return nil
}

func (s *careOfferingService) validateOfferingCalendarPeriodChange(
	ctx context.Context,
	offering *enrollmentModels.CareOffering,
	periodID int64,
	replacement *scheduleModels.CalendarPeriod,
	periods map[int64]*scheduleModels.CalendarPeriod,
) error {
	if offering == nil {
		return nil
	}
	requiresMaterialization, err := s.offeringRequiresMaterialization(ctx, offering)
	if err != nil {
		return fmt.Errorf("inspect care offering %d request selections: %w", offering.ID, err)
	}
	if !requiresMaterialization {
		// Unused inactive offerings are drafts/history and may intentionally be
		// staged against an inactive future period.
		return nil
	}
	group, series, err := s.loadCareOfferingTemplateSeriesForPeriodChange(ctx, offering)
	if err != nil {
		return err
	}
	if len(series) == 0 {
		return nil
	}

	loaded, referencesChangedPeriod, err := s.loadCareOfferingPeriodChangeSegments(
		ctx,
		offering.ID,
		series,
		periodID,
	)
	if err != nil {
		return err
	}
	if !referencesChangedPeriod {
		return nil
	}

	phase, err := s.loadCareOfferingPeriodChangePhase(ctx, offering)
	if err != nil {
		return err
	}

	postChangeSegments, err := s.resolveCareOfferingPostChangeSegments(
		ctx,
		offering,
		group,
		phase,
		loaded,
		periodID,
		replacement,
		periods,
		requiresMaterialization,
	)
	if err != nil {
		return err
	}
	if err := validateCareOfferingTemplateSegments(
		postChangeSegments,
		phase,
		offering.AvailableDays,
		requiresMaterialization,
	); err != nil {
		return careOfferingPeriodConflictf(offering.ID, err)
	}
	if err := validateCareOfferingMaterializability(
		ctx,
		s.materializationDeps(),
		postChangeSegments,
		phase,
		offering.AvailableDays,
		careOfferingMaterializationChange{},
	); err != nil {
		return careOfferingPeriodConflictf(offering.ID, err)
	}
	return nil
}

func (s *careOfferingService) loadCareOfferingTemplateSeriesForPeriodChange(
	ctx context.Context,
	offering *enrollmentModels.CareOffering,
) (*activitiesModels.Group, []*activitiesModels.Group, error) {
	group, err := s.ActivityGroupRepo.FindByID(ctx, *offering.ActivityGroupID)
	if err != nil {
		return nil, nil, fmt.Errorf("load care offering %d timetable template: %w", offering.ID, err)
	}
	if group == nil {
		return nil, nil, fmt.Errorf("load care offering %d timetable template: unavailable", offering.ID)
	}
	// Historical non-template links do not use recurrence calendar periods.
	if !group.IsTemplate {
		return group, nil, nil
	}

	series, err := s.ActivityGroupRepo.FindTemplateSeries(ctx, group.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("load care offering %d timetable series: %w", offering.ID, err)
	}
	// An empty live series represents an archived series. Archive validation
	// separately prevents a live care offering from entering this state.
	return group, series, nil
}

func (s *careOfferingService) loadCareOfferingPeriodChangeSegments(
	ctx context.Context,
	offeringID int64,
	series []*activitiesModels.Group,
	periodID int64,
) ([]loadedCareOfferingSegment, bool, error) {
	loaded := make([]loadedCareOfferingSegment, 0, len(series))
	referencesChangedPeriod := false
	groupIDs := make([]int64, 0, len(series))
	for _, segment := range series {
		if segment != nil {
			groupIDs = append(groupIDs, segment.ID)
		}
	}
	var scheduleRows []*activitiesModels.Schedule
	if len(groupIDs) > 0 {
		var err error
		scheduleRows, err = s.ActivityScheduleRepo.List(ctx, &modelBase.QueryOptions{
			Filter: modelBase.NewFilter().In("activity_group_id", int64FilterArgs(groupIDs)...),
		})
		if err != nil {
			return nil, false, fmt.Errorf("load care offering %d timetable schedules: %w", offeringID, err)
		}
	}
	schedulesByGroup := activitySchedulesByGroup(scheduleRows)
	for _, segment := range series {
		if segment == nil {
			continue
		}
		schedules := schedulesByGroup[segment.ID]
		schedulesReferenceChangedPeriod := schedulesReferencePeriod(schedules, periodID)
		referencesChangedPeriod = referencesChangedPeriod ||
			referencesPeriod(segment.CalendarPeriodID, periodID) ||
			schedulesReferenceChangedPeriod
		loaded = append(loaded, loadedCareOfferingSegment{
			group:     segment,
			schedules: schedules,
		})
	}
	return loaded, referencesChangedPeriod, nil
}

func activitySchedulesByGroup(rows []*activitiesModels.Schedule) map[int64][]*activitiesModels.Schedule {
	result := make(map[int64][]*activitiesModels.Schedule)
	for _, row := range rows {
		if row != nil {
			result[row.ActivityGroupID] = append(result[row.ActivityGroupID], row)
		}
	}
	return result
}

func schedulesReferencePeriod(schedules []*activitiesModels.Schedule, periodID int64) bool {
	for _, schedule := range schedules {
		if schedule != nil && referencesPeriod(schedule.CalendarPeriodID, periodID) {
			return true
		}
	}
	return false
}

func (s *careOfferingService) loadCareOfferingPeriodChangePhase(
	ctx context.Context,
	offering *enrollmentModels.CareOffering,
) (*enrollmentModels.Phase, error) {
	phase, err := s.PhaseRepo.FindByID(ctx, offering.PhaseID)
	if err != nil {
		return nil, fmt.Errorf("load care offering %d phase: %w", offering.ID, err)
	}
	if phase == nil {
		return nil, fmt.Errorf("load care offering %d phase: unavailable", offering.ID)
	}
	return phase, nil
}

func (s *careOfferingService) resolveCareOfferingPostChangeSegments(
	ctx context.Context,
	offering *enrollmentModels.CareOffering,
	root *activitiesModels.Group,
	phase *enrollmentModels.Phase,
	loaded []loadedCareOfferingSegment,
	periodID int64,
	replacement *scheduleModels.CalendarPeriod,
	periods map[int64]*scheduleModels.CalendarPeriod,
	requiresMaterialization bool,
) ([]linkedCareOfferingGroup, error) {
	segments := make([]linkedCareOfferingGroup, 0, len(loaded))
	for _, state := range loaded {
		segment, include, err := s.resolveCareOfferingPostChangeSegment(
			ctx,
			offering,
			root,
			phase,
			state,
			periodID,
			replacement,
			periods,
			requiresMaterialization,
		)
		if err != nil {
			return nil, err
		}
		if include {
			segments = append(segments, segment)
		}
	}
	return segments, nil
}

func (s *careOfferingService) resolveCareOfferingPostChangeSegment(
	ctx context.Context,
	offering *enrollmentModels.CareOffering,
	root *activitiesModels.Group,
	phase *enrollmentModels.Phase,
	state loadedCareOfferingSegment,
	periodID int64,
	replacement *scheduleModels.CalendarPeriod,
	periods map[int64]*scheduleModels.CalendarPeriod,
	requiresMaterialization bool,
) (linkedCareOfferingGroup, bool, error) {
	overlapsPhase := schedulesOverlapEnrollmentPhase(state.schedules, phase)
	mustResolve := overlapsPhase || (replacement == nil && state.group.ID == root.ID)
	resolvedPeriodID, changed, err := effectivePeriodIDAfterChange(
		state.group,
		state.schedules,
		periodID,
		replacement == nil,
	)
	if err != nil {
		// The directly-linked root is resolved before series expansion during
		// approval. Deleting its last effective period therefore breaks the
		// link even when that historic segment does not overlap this phase.
		if mustResolve {
			return linkedCareOfferingGroup{}, false, careOfferingPeriodConflictf(offering.ID, err)
		}
		return linkedCareOfferingGroup{}, false, nil
	}

	period, err := s.resolvePostChangeSegmentPeriod(
		ctx,
		state.group,
		resolvedPeriodID,
		changed,
		overlapsPhase,
		periodID,
		replacement,
		periods,
	)
	if err != nil {
		return linkedCareOfferingGroup{}, false, careOfferingPostChangePeriodError(
			offering.ID,
			state.group.ID,
			err,
			mustResolve,
		)
	}
	if !overlapsPhase {
		return linkedCareOfferingGroup{}, false, nil
	}
	if period == nil || (requiresMaterialization && !period.IsActive) {
		return linkedCareOfferingGroup{}, false, careOfferingPeriodConflictf(
			offering.ID,
			careOfferingInvalidf("a materializable timetable-linked care offering requires an active calendar period"),
		)
	}
	if err := validatePhaseWithinTemplatePeriod(phase, period); err != nil {
		return linkedCareOfferingGroup{}, false, careOfferingPeriodConflictf(offering.ID, err)
	}
	return linkedCareOfferingGroup{
		group:     state.group,
		period:    period,
		schedules: state.schedules,
	}, true, nil
}

func (s *careOfferingService) resolvePostChangeSegmentPeriod(
	ctx context.Context,
	group *activitiesModels.Group,
	resolvedPeriodID int64,
	changed bool,
	overlapsPhase bool,
	periodID int64,
	replacement *scheduleModels.CalendarPeriod,
	periods map[int64]*scheduleModels.CalendarPeriod,
) (*scheduleModels.CalendarPeriod, error) {
	if changed {
		return s.calendarPeriodAfterChange(ctx, resolvedPeriodID, periodID, replacement, periods)
	}
	if !overlapsPhase {
		return nil, nil
	}
	return resolveTemplatePeriodForGroup(ctx, careOfferingTemplateDeps{
		activityGroupRepo:    s.ActivityGroupRepo,
		activityScheduleRepo: s.ActivityScheduleRepo,
		calendarPeriodRepo:   s.CalendarPeriodRepo,
	}, group)
}

func careOfferingPostChangePeriodError(
	offeringID int64,
	groupID int64,
	err error,
	mustResolve bool,
) error {
	if errors.Is(err, ErrCareOfferingInvalid) && mustResolve {
		return careOfferingPeriodConflictf(offeringID, err)
	}
	if !mustResolve {
		return nil
	}
	return fmt.Errorf(
		"resolve care offering %d timetable segment %d period: %w",
		offeringID,
		groupID,
		err,
	)
}

func effectivePeriodIDAfterChange(
	group *activitiesModels.Group,
	schedules []*activitiesModels.Schedule,
	periodID int64,
	deleting bool,
) (resolved int64, changed bool, err error) {
	if group == nil {
		return 0, false, careOfferingInvalidf("linked timetable segment is unavailable")
	}
	groupPeriodID := periodReferenceAfterChange(group.CalendarPeriodID, periodID, deleting)
	changed = referencesPeriod(group.CalendarPeriodID, periodID)
	if len(schedules) == 0 {
		return effectivePeriodWithoutSchedules(changed)
	}

	for _, schedule := range schedules {
		resolved, changed, err = mergeEffectiveSchedulePeriod(
			resolved,
			changed,
			groupPeriodID,
			schedule,
			periodID,
			deleting,
		)
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
	schedule *activitiesModels.Schedule,
	periodID int64,
	deleting bool,
) (int64, bool, error) {
	if schedule == nil {
		return resolved, changed, nil
	}
	changed = changed || referencesPeriod(schedule.CalendarPeriodID, periodID)
	schedulePeriodID := periodReferenceAfterChange(schedule.CalendarPeriodID, periodID, deleting)
	if schedulePeriodID == nil {
		schedulePeriodID = groupPeriodID
	}
	if schedulePeriodID == nil {
		if changed {
			return 0, true, careOfferingInvalidf(
				"timetable template schedules must resolve one calendar_period_id after the change",
			)
		}
		return resolved, false, nil
	}
	if resolved == 0 {
		return *schedulePeriodID, changed, nil
	}
	if resolved != *schedulePeriodID && changed {
		return 0, true, careOfferingInvalidf(
			"timetable template schedules would use different calendar_period_id values after the change",
		)
	}
	return resolved, changed, nil
}

func effectivePeriodChangeResult(resolved int64, changed bool) (int64, bool, error) {
	if !changed {
		return 0, false, nil
	}
	if resolved == 0 {
		return 0, true, careOfferingInvalidf(
			"timetable template schedules must resolve one calendar_period_id after the change",
		)
	}
	return resolved, true, nil
}

func periodReferenceAfterChange(current *int64, periodID int64, deleting bool) *int64 {
	if current == nil {
		return nil
	}
	if deleting && *current == periodID {
		return nil
	}
	id := *current
	return &id
}

func referencesPeriod(current *int64, periodID int64) bool {
	return current != nil && *current == periodID
}

func schedulesOverlapEnrollmentPhase(
	schedules []*activitiesModels.Schedule,
	phase *enrollmentModels.Phase,
) bool {
	if phase == nil {
		return false
	}
	for _, schedule := range schedules {
		if schedule == nil {
			continue
		}
		startsAfterPhase := schedule.ValidFrom != nil && schedule.ValidFrom.After(phase.ServiceEndDate)
		endsBeforeOrOnPhase := schedule.ValidUntil != nil && !schedule.ValidUntil.After(phase.ServiceStartDate)
		if !startsAfterPhase && !endsBeforeOrOnPhase {
			return true
		}
	}
	return false
}

func (s *careOfferingService) calendarPeriodAfterChange(
	ctx context.Context,
	resolvedPeriodID int64,
	changedPeriodID int64,
	replacement *scheduleModels.CalendarPeriod,
	cache map[int64]*scheduleModels.CalendarPeriod,
) (*scheduleModels.CalendarPeriod, error) {
	if replacement != nil && resolvedPeriodID == changedPeriodID {
		return replacement, nil
	}
	if period := cache[resolvedPeriodID]; period != nil {
		return period, nil
	}
	period, err := s.CalendarPeriodRepo.FindByID(ctx, resolvedPeriodID)
	if err != nil {
		if modelBase.IsNoRows(err) {
			return nil, careOfferingInvalidf("calendar period for timetable template not found")
		}
		return nil, fmt.Errorf("load timetable template calendar period: %w", err)
	}
	if period == nil {
		return nil, careOfferingInvalidf("calendar period for timetable template not found")
	}
	cache[resolvedPeriodID] = period
	return period, nil
}

func careOfferingPeriodConflictf(offeringID int64, cause error) error {
	return fmt.Errorf(
		"%w: care offering %d would become incompatible: %w",
		scheduleModels.ErrCalendarPeriodCareOfferingConflict,
		offeringID,
		cause,
	)
}
