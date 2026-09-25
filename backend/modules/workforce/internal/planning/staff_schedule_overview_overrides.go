package planning

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	usersModel "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

// TargetOverrideDaysReader answers the days a Sonderarbeitszeit (#3259) sets
// a target on: Monday to Friday, never a statutory holiday.
type TargetOverrideDaysReader interface {
	StaffTargetOverrideDays(ctx context.Context, staffIDs []int64, from, to string) (workforce.TargetOverrideDays, error)
}

// baseDayTarget prices one day of one staff member without a
// Sonderarbeitszeit.
type baseDayTarget func(member *usersModel.Staff, day timezone.Date) int

// weeklyTargets resolves the contractual weekly targets and then corrects
// them by the Sonderarbeitszeiten of the summarized weeks.
func (s *staffScheduleOverviewService) weeklyTargets(
	ctx context.Context,
	staff []*usersModel.Staff,
	schedules []*configModel.StaffWorkSchedule,
	weekStarts []timezone.Date,
) (map[staffDateKey]int, error) {
	targets, err := s.resolveWeeklyTargets(ctx, staff, schedules, weekStarts)
	if err != nil {
		return nil, err
	}
	if err := s.applyTargetOverrides(ctx, staff, schedules, weekStarts, targets); err != nil {
		return nil, err
	}
	return targets, nil
}

// applyTargetOverrides replaces, for every Sonderarbeitszeit day, what the
// schedule, the model or a closing day contributed to that week by the
// override's minutes. Nothing more is read when nobody has one.
func (s *staffScheduleOverviewService) applyTargetOverrides(
	ctx context.Context,
	staff []*usersModel.Staff,
	schedules []*configModel.StaffWorkSchedule,
	weekStarts []timezone.Date,
	targets map[staffDateKey]int,
) error {
	if s.deps.TargetOverrides == nil || len(weekStarts) == 0 || len(staff) == 0 {
		return nil
	}
	from, to := weekStarts[0], weekStarts[len(weekStarts)-1].AddDays(6)
	days, err := s.deps.TargetOverrides.StaffTargetOverrideDays(ctx, overviewStaffIDs(staff), from.String(), to.String())
	if err != nil {
		return fmt.Errorf("load target overrides: %w", err)
	}
	if len(days) == 0 {
		return nil
	}
	base, err := s.baseDayTargets(ctx, staff, schedules, weekStarts)
	if err != nil {
		return err
	}
	for _, member := range staff {
		if member != nil && len(days[member.ID]) > 0 {
			addOverrideDeltas(member, days[member.ID], weekStarts, base, targets)
		}
	}
	return nil
}

// addOverrideDeltas corrects the weekly targets that resolveWeeklyTargets
// produced. A week it left out has no Soll at all — neither a schedule row
// nor the model covers it — so a Sonderarbeitszeit inside that week must not
// create one: the overview would otherwise show the override minutes as if
// they were the whole week's Soll.
func addOverrideDeltas(member *usersModel.Staff, days map[string]int, weekStarts []timezone.Date, base baseDayTarget, targets map[staffDateKey]int) {
	for _, weekStart := range weekStarts {
		key := staffDateKey{StaffID: member.ID, Date: weekStart}
		if _, resolved := targets[key]; !resolved {
			continue
		}
		delta, touched := 0, false
		for offset := range 7 {
			day := weekStart.AddDays(offset)
			if minutes, ok := days[day.String()]; ok {
				delta += minutes - base(member, day)
				touched = true
			}
		}
		if touched {
			targets[key] += delta
		}
	}
}

func overviewStaffIDs(staff []*usersModel.Staff) []int64 {
	ids := make([]int64, 0, len(staff))
	for _, member := range staff {
		if member != nil {
			ids = append(ids, member.ID)
		}
	}
	return ids
}

// baseDayTargets prices a day the way resolveWeeklyTargets does: a closing
// day or holiday is zero, the schedule prices the day of everyone it covers
// in at least one summarized week, the assigned model prices the rest.
func (s *staffScheduleOverviewService) baseDayTargets(
	ctx context.Context,
	staff []*usersModel.Staff,
	schedules []*configModel.StaffWorkSchedule,
	weekStarts []timezone.Date,
) (baseDayTarget, error) {
	holidaySet, err := s.holidayDatesForWeeks(ctx, weekStarts)
	if err != nil {
		return nil, err
	}
	entriesByStaff := make(map[int64][]*configModel.StaffWorkSchedule)
	for _, entry := range schedules {
		if entry != nil {
			entriesByStaff[entry.StaffID] = append(entriesByStaff[entry.StaffID], entry)
		}
	}
	scheduled := scheduleDrivenStaff(staff, entriesByStaff, weekStarts)
	modelsByID, err := s.overrideModels(ctx, staff, scheduled)
	if err != nil {
		return nil, err
	}
	return func(member *usersModel.Staff, day timezone.Date) int {
		if holidaySet[day] {
			return 0
		}
		if scheduled[member.ID] {
			target, _ := configModel.DailyTargetFromSchedule(entriesByStaff[member.ID], workforceDatePointer(member.RotationAnchorDate), workforceDate(day))
			return target
		}
		return modelDayTarget(member, modelsByID, day)
	}, nil
}

// scheduleDrivenStaff names the staff members whose schedule rows cover at
// least one summarized week, the same condition under which
// resolveWeeklyTargets prices their weeks from the schedule instead of the
// model. Both sides must agree, or a Sonderarbeitszeit would subtract a day
// the week's Soll never contained.
func scheduleDrivenStaff(
	staff []*usersModel.Staff,
	entriesByStaff map[int64][]*configModel.StaffWorkSchedule,
	weekStarts []timezone.Date,
) map[int64]bool {
	scheduled := make(map[int64]bool, len(staff))
	for _, member := range staff {
		if member == nil || len(entriesByStaff[member.ID]) == 0 {
			continue
		}
		for _, weekStart := range weekStarts {
			if _, ok := configModel.WeeklyTargetFromSchedule(entriesByStaff[member.ID], workforceDatePointer(member.RotationAnchorDate), workforceDate(weekStart)); ok {
				scheduled[member.ID] = true
				break
			}
		}
	}
	return scheduled
}

func modelDayTarget(member *usersModel.Staff, modelsByID map[int64]*configModel.WorkTimeModel, day timezone.Date) int {
	if member.WorkTimeModelID == nil || modelsByID[*member.WorkTimeModelID] == nil {
		return 0
	}
	model := modelsByID[*member.WorkTimeModelID]
	anchor := model.RotationAnchorDate
	if member.RotationAnchorDate != nil {
		anchor = workforceDate(*member.RotationAnchorDate)
	}
	target, _ := configModel.DailyTargetFromModel(model, anchor, workforceDate(day))
	return target
}

// overrideModels loads the work-time models of the staff members whose
// schedule does not cover the summarized weeks.
func (s *staffScheduleOverviewService) overrideModels(
	ctx context.Context,
	staff []*usersModel.Staff,
	scheduled map[int64]bool,
) (map[int64]*configModel.WorkTimeModel, error) {
	modelsByID := make(map[int64]*configModel.WorkTimeModel)
	modelIDs := make([]int64, 0)
	for _, member := range staff {
		if member != nil && member.WorkTimeModelID != nil && !scheduled[member.ID] {
			modelIDs = append(modelIDs, *member.WorkTimeModelID)
		}
	}
	if len(modelIDs) == 0 || s.deps.WorkModels == nil {
		return modelsByID, nil
	}
	models, err := s.deps.WorkModels.FindByIDs(ctx, modelIDs)
	if err != nil {
		return nil, fmt.Errorf("load work-time models: %w", err)
	}
	for _, model := range models {
		if model != nil {
			modelsByID[model.ID] = model
		}
	}
	return modelsByID, nil
}
