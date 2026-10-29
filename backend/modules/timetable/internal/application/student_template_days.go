package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sort"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"
	schedule "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// StudentTemplateDays names and removes the hand planning a booking change
// leaves behind (#5102). A roster row a booking created carries
// enrollment_request_child_id and follows the booking; everything else — a
// hand row, a Klasse/Jahrgang/Gruppe target, an occurrence roster — stays
// until somebody removes it.
type StudentTemplateDays struct {
	deps ports.StudentTemplateDaysDependencies
}

var _ timetable.StudentTemplateDays = (*StudentTemplateDays)(nil)

// NewStudentTemplateDays constructs the capability; every row dependency is
// required.
func NewStudentTemplateDays(deps ports.StudentTemplateDaysDependencies) (*StudentTemplateDays, error) {
	if deps.Groups == nil || deps.Schedules == nil || deps.Timeframes == nil || deps.Periods == nil || deps.Enrollments == nil ||
		deps.Instances == nil || deps.Participants.Reads == nil || deps.Participants.Plans == nil || deps.RecurrenceLock == nil {
		return nil, errors.New("student template days: groups, schedules, timeframes, periods, enrollments, instances, participants and recurrence lock are required")
	}
	if deps.Today == nil {
		deps.Today = calendar.TodayDate
	}
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	return &StudentTemplateDays{deps: deps}, nil
}

// plannedOccurrence is one planned, template-backed occurrence that lists the
// student, with the plan weekday its date runs on.
type plannedOccurrence struct {
	instance    *schedule.ActivityInstance
	participant *schedule.InstanceStudent
	planWeekday int
}

// templateDays collects ISO weekdays per template.
type templateDays map[int64]map[int]bool

func (d templateDays) add(templateID int64, weekday int) {
	if d[templateID] == nil {
		d[templateID] = make(map[int]bool)
	}
	d[templateID][weekday] = true
}

// ManualStudentPlan lists the Regeltermin series that keep planning the
// student on the asked weekdays from the date on, other than through a roster
// row a booking created, where no remaining booking carries the series
// (#5342).
func (s *StudentTemplateDays) ManualStudentPlan(ctx context.Context, query timetable.ManualStudentPlanQuery) ([]timetable.ManualStudentPlan, error) {
	wanted, err := validStudentTemplateDays(query.StudentID, query.From, query.Weekdays)
	if err != nil {
		return nil, err
	}
	carry, err := newBookingCarry(query)
	if err != nil {
		return nil, err
	}
	rows, err := studentEnrollments(ctx, s.deps.Enrollments, query.StudentID)
	if err != nil {
		return nil, err
	}
	days, ends := make(templateDays), make(templateDayEnds)
	occurrences, err := s.plannedOccurrences(ctx, query.StudentID, query.From)
	if err != nil {
		return nil, err
	}
	for _, occurrence := range occurrences {
		// A row recording attendance or a hand decision is not a plan the
		// removal may take back, so it is not named either.
		if !wanted[occurrence.planWeekday] || !instanceRowIsStillPlanned(occurrence.participant) ||
			bookingPlansOccurrence(rows, occurrence) {
			continue
		}
		days.add(*occurrence.instance.ActivityGroupID, occurrence.planWeekday)
		ends.addClock(*occurrence.instance.ActivityGroupID, occurrence.planWeekday, occurrence.instance.EndTime.Format("15:04:05"))
	}
	if err := s.addHandRowDays(ctx, rows, query.From, wanted, days, ends); err != nil {
		return nil, err
	}
	if len(days) == 0 {
		return []timetable.ManualStudentPlan{}, nil
	}
	return s.manualPlansBySeries(ctx, query.StudentID, days, ends, carry)
}

// validStudentTemplateDays checks the shared input and returns the weekday
// set.
func validStudentTemplateDays(studentID int64, from calendar.Date, weekdays []int) (map[int]bool, error) {
	if studentID <= 0 {
		return nil, fmt.Errorf("%w: student_id is required", timetable.ErrStudentTemplateDaysInvalid)
	}
	if from.IsZero() {
		return nil, fmt.Errorf("%w: from is required", timetable.ErrStudentTemplateDaysInvalid)
	}
	set := make(map[int]bool, len(weekdays))
	for _, weekday := range weekdays {
		if !isISOWeekday(weekday) {
			return nil, fmt.Errorf("%w: weekday %d is not an ISO weekday", timetable.ErrStudentTemplateDaysInvalid, weekday)
		}
		set[weekday] = true
	}
	return set, nil
}

// plannedOccurrences reads the student's participant rows from the date on and
// keeps those on planned, template-backed occurrences nobody started.
func (s *StudentTemplateDays) plannedOccurrences(ctx context.Context, studentID int64, from calendar.Date) ([]plannedOccurrence, error) {
	fromText := from.String()
	participants, err := listLegacyParticipants(ctx, s.deps.Participants.Reads, timetable.InstanceStudentFilter{
		StudentIDs: []int64{studentID}, FromDate: &fromText,
	}, "find student participants from date")
	if err != nil || len(participants) == 0 {
		return nil, err
	}
	byInstance := make(map[int64]*schedule.InstanceStudent, len(participants))
	ids := make([]int64, 0, len(participants))
	for _, participant := range participants {
		byInstance[participant.InstanceID] = participant
		ids = append(ids, participant.InstanceID)
	}
	instances, err := instancesByID(ctx, s.deps.Instances, ids)
	if err != nil {
		return nil, err
	}
	occurrences := make([]plannedOccurrence, 0, len(instances))
	for _, instance := range instances {
		if !isPlannedTemplateOccurrence(instance, from) {
			continue
		}
		planWeekday, err := calendar.PlanWeekday(ctx, instance.Date)
		if err != nil {
			return nil, fmt.Errorf("student template days: resolve weekend plan: %w", err)
		}
		occurrences = append(occurrences, plannedOccurrence{instance: instance, participant: byInstance[instance.ID], planWeekday: planWeekday})
	}
	return occurrences, nil
}

// isPlannedTemplateOccurrence keeps materialized occurrences of a template
// that have not started; history and running blocks stay untouched.
func isPlannedTemplateOccurrence(instance *schedule.ActivityInstance, from calendar.Date) bool {
	return instance.ActivityGroupID != nil && instance.CalendarPeriodID != nil && !instance.IsSpontaneous &&
		instance.Status == schedule.InstanceStatusPlanned && !instance.Date.Before(from)
}

// bookingPlansOccurrence reports whether a booking-created row of the
// occurrence's template plans the student on its date.
func bookingPlansOccurrence(rows []*activityContract.StudentEnrollment, occurrence plannedOccurrence) bool {
	for _, row := range rows {
		if row.EnrollmentRequestChildID == nil || row.ActivityGroupID != *occurrence.instance.ActivityGroupID {
			continue
		}
		if isEnrollmentValidOn(row, occurrence.instance.Date, calendarPeriodID(occurrence.instance), occurrence.planWeekday) {
			return true
		}
	}
	return false
}

// addHandRowDays adds the weekdays the student's hand rows still plan from
// the date on, unless a booking-created row of the same template covers the
// weekday as well, with the end of the template's schedule on that weekday.
// Only schedules still valid from the date on count.
func (s *StudentTemplateDays) addHandRowDays(
	ctx context.Context, rows []*activityContract.StudentEnrollment, from calendar.Date, wanted map[int]bool, days templateDays, ends templateDayEnds,
) error {
	hand := reachingRows(rows, from, false)
	if len(hand) == 0 {
		return nil
	}
	schedules, err := s.listSchedules(ctx, rowTemplateIDs(hand))
	if err != nil {
		return err
	}
	if schedules, err = s.runningSchedules(ctx, schedules, from); err != nil {
		return err
	}
	scheduleDays := weekdaysBySchedule(schedules)
	scheduleEnds, err := s.scheduleEnds(ctx, schedules)
	if err != nil {
		return err
	}
	booked := reachingRows(rows, from, true)
	for _, row := range hand {
		for weekday := range rowWeekdays(row, scheduleDays[row.ActivityGroupID]) {
			if wanted[weekday] && !rowsCoverWeekday(booked, row.ActivityGroupID, weekday, scheduleDays[row.ActivityGroupID]) {
				days.add(row.ActivityGroupID, weekday)
				ends.merge(row.ActivityGroupID, weekday, scheduleEnds)
			}
		}
	}
	return nil
}

// reachingRows keeps the hand rows (tagged false) or the booking-created rows
// (tagged true) that are still valid on or after the date.
func reachingRows(rows []*activityContract.StudentEnrollment, from calendar.Date, tagged bool) []*activityContract.StudentEnrollment {
	result := make([]*activityContract.StudentEnrollment, 0, len(rows))
	for _, row := range rows {
		if (row.EnrollmentRequestChildID != nil) != tagged {
			continue
		}
		if row.ValidUntil != nil && !calendar.Date(*row.ValidUntil).After(from) {
			continue
		}
		result = append(result, row)
	}
	return result
}

func rowTemplateIDs(rows []*activityContract.StudentEnrollment) []int64 {
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ActivityGroupID)
	}
	slices.Sort(ids)
	return slices.Compact(ids)
}

// scheduleWeekdays reads the weekdays each template runs on.
func (s *StudentTemplateDays) scheduleWeekdays(ctx context.Context, templateIDs []int64) (map[int64]map[int]bool, error) {
	schedules, err := s.listSchedules(ctx, templateIDs)
	if err != nil {
		return nil, err
	}
	return weekdaysBySchedule(schedules), nil
}

func (s *StudentTemplateDays) listSchedules(ctx context.Context, templateIDs []int64) ([]timetable.Schedule, error) {
	if len(templateIDs) == 0 {
		return nil, nil
	}
	schedules, err := s.deps.Schedules.ListSchedules(ctx, timetable.ScheduleFilter{GroupIDs: templateIDs})
	if err != nil {
		return nil, fmt.Errorf("student template days: list schedules: %w", err)
	}
	return schedules, nil
}

func weekdaysBySchedule(schedules []timetable.Schedule) map[int64]map[int]bool {
	result := make(map[int64]map[int]bool)
	for _, sch := range schedules {
		if result[sch.ActivityGroupID] == nil {
			result[sch.ActivityGroupID] = make(map[int]bool)
		}
		result[sch.ActivityGroupID][sch.Weekday] = true
	}
	return result
}

// rowWeekdays is the weekday set a row plans on a template running on the
// schedule weekdays: its single weekday scope, its selected weekdays, or every
// weekday of the template.
func rowWeekdays(row *activityContract.StudentEnrollment, scheduleDays map[int]bool) map[int]bool {
	result := make(map[int]bool)
	switch {
	case row.Weekday != nil:
		result[*row.Weekday] = true
	case len(row.SelectedWeekdays) > 0:
		for _, weekday := range row.SelectedWeekdays {
			result[weekday] = true
		}
	default:
		for weekday := range scheduleDays {
			result[weekday] = true
		}
	}
	return result
}

func rowsCoverWeekday(rows []*activityContract.StudentEnrollment, templateID int64, weekday int, scheduleDays map[int]bool) bool {
	for _, row := range rows {
		if row.ActivityGroupID == templateID && rowWeekdays(row, scheduleDays)[weekday] {
			return true
		}
	}
	return false
}

// manualPlansBySeries names the templates, drops the weekdays a remaining
// booking carries and merges split segments into one entry per series,
// pointing at the latest segment.
func (s *StudentTemplateDays) manualPlansBySeries(
	ctx context.Context, studentID int64, days templateDays, ends templateDayEnds, carry bookingCarry,
) ([]timetable.ManualStudentPlan, error) {
	groups, err := s.deps.Groups.ListGroups(ctx, timetable.GroupFilter{IDs: sortedTemplateIDs(days)})
	if err != nil {
		return nil, fmt.Errorf("student template days: list templates: %w", err)
	}
	groups = carry.dropCarriedDays(groups, days, ends)
	if len(groups) == 0 {
		return []timetable.ManualStudentPlan{}, nil
	}
	targets, err := s.deps.Groups.ListTargetStudentIDs(ctx, sortedTemplateIDs(days))
	if err != nil {
		return nil, fmt.Errorf("student template days: list target students: %w", err)
	}
	bySeries := make(map[int64]*timetable.ManualStudentPlan)
	weekdays := make(map[int64]map[int]bool)
	for _, group := range groups {
		root := group.ID
		if group.SeriesRootID != nil {
			root = *group.SeriesRootID
		}
		plan := bySeries[root]
		if plan == nil {
			plan = &timetable.ManualStudentPlan{}
			bySeries[root] = plan
			weekdays[root] = make(map[int]bool)
		}
		if group.ID > plan.TemplateID {
			plan.TemplateID, plan.Name = group.ID, group.Name
		}
		plan.ViaTarget = plan.ViaTarget || slices.Contains(targets[group.ID], studentID)
		for weekday := range days[group.ID] {
			weekdays[root][weekday] = true
		}
	}
	return sortedManualPlans(bySeries, weekdays), nil
}

func sortedTemplateIDs(days templateDays) []int64 {
	ids := make([]int64, 0, len(days))
	for id := range days {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

func sortedManualPlans(bySeries map[int64]*timetable.ManualStudentPlan, weekdays map[int64]map[int]bool) []timetable.ManualStudentPlan {
	plans := make([]timetable.ManualStudentPlan, 0, len(bySeries))
	for root, plan := range bySeries {
		plan.Weekdays = sortedWeekdays(weekdays[root])
		plans = append(plans, *plan)
	}
	sort.Slice(plans, func(i, j int) bool {
		if left, right := strings.ToLower(plans[i].Name), strings.ToLower(plans[j].Name); left != right {
			return left < right
		}
		return plans[i].TemplateID < plans[j].TemplateID
	})
	return plans
}

func sortedWeekdays(set map[int]bool) []int {
	result := make([]int, 0, len(set))
	for weekday := range set {
		result = append(result, weekday)
	}
	slices.Sort(result)
	return result
}
