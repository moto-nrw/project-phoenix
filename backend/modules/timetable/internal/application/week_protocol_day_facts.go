package application

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// dayFacts builds what each protocol records for the day: the attendance of
// its group or of the whole school, its blocks and the staff notices of the
// day.
func (e weekProtocolEvening) dayFacts(ctx context.Context, day calendar.Date, targets []weekProtocolTarget) (map[int64]timetable.WeekProtocolDayFacts, error) {
	kind, err := e.dayKind(ctx, day)
	if err != nil {
		return nil, err
	}
	attendance, err := e.groupAttendance(ctx, day, targets)
	if err != nil {
		return nil, err
	}
	blocks, err := e.dayBlocks(ctx, day)
	if err != nil {
		return nil, err
	}
	notices, err := e.dayNotices(ctx, day)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]timetable.WeekProtocolDayFacts, len(targets))
	for _, target := range targets {
		facts := timetable.WeekProtocolDayFacts{
			Kind: kind, Missing: []timetable.WeekProtocolMissingChild{}, Notices: notices,
			Blocks: blocksOfTarget(blocks, target),
		}
		addWeekProtocolAttendance(&facts, attendance[target.key])
		if kind != timetable.WeekProtocolDayCare {
			// On a holiday or closing day nobody is missing; only the children
			// who came (holiday care) and its blocks count.
			facts.Sick, facts.Excused, facts.ClassTrip, facts.Absent = 0, 0, 0, 0
			facts.Missing = []timetable.WeekProtocolMissingChild{}
		}
		sort.SliceStable(facts.Missing, func(i, j int) bool { return facts.Missing[i].Name < facts.Missing[j].Name })
		result[target.key] = facts
	}
	return result, nil
}

// dayKind tells a care day from a statutory holiday and a closing day of the
// school; a holiday wins over a closing day, as in the timetable.
func (e weekProtocolEvening) dayKind(ctx context.Context, day calendar.Date) (string, error) {
	nonWorking, err := timetable.LoadNonWorkingDays(ctx, e.sources.Calendar, day.String(), day.String())
	if err != nil {
		return "", err
	}
	switch nonWorking.Skip(day.String(), false) {
	case timetable.SkipHoliday:
		return timetable.WeekProtocolDayHoliday, nil
	case timetable.SkipClosingDay:
		return timetable.WeekProtocolDayClosing, nil
	default:
		return timetable.WeekProtocolDayCare, nil
	}
}

// groupAttendance reads the day of every target by its key: a group's
// children from its day log, the whole school's from the school's day log.
func (e weekProtocolEvening) groupAttendance(ctx context.Context, day calendar.Date, targets []weekProtocolTarget) (map[int64]timetable.WeekProtocolGroupDay, error) {
	groupIDs := []int64{}
	result := map[int64]timetable.WeekProtocolGroupDay{}
	for _, target := range targets {
		if target.target.GroupID != nil {
			groupIDs = append(groupIDs, target.key)
			continue
		}
		school, err := e.sources.Attendance.SchoolDayAttendance(ctx, day)
		if err != nil {
			return nil, fmt.Errorf("read the school's day log: %w", err)
		}
		result[target.key] = school
	}
	if len(groupIDs) == 0 {
		return result, nil
	}
	days, err := e.sources.Attendance.GroupDayAttendance(ctx, day, groupIDs)
	if err != nil {
		return nil, fmt.Errorf("read the day log: %w", err)
	}
	for _, groupDay := range days {
		result[groupDay.GroupID] = groupDay
	}
	return result, nil
}

func addWeekProtocolAttendance(facts *timetable.WeekProtocolDayFacts, day timetable.WeekProtocolGroupDay) {
	facts.Present += day.Present
	facts.Sick += day.Sick
	facts.Excused += day.Excused
	facts.ClassTrip += day.ClassTrip
	facts.Absent += day.Absent
	facts.Missing = append(facts.Missing, day.Missing...)
}

// weekProtocolDayBlock is a block of the day with the OGS groups it belongs
// to.
type weekProtocolDayBlock struct {
	block  timetable.WeekProtocolBlock
	groups []int64
}

// dayBlocks are the day's blocks with children, without duties, each with
// its groups as the Betreuungsplan's group filter names them.
func (e weekProtocolEvening) dayBlocks(ctx context.Context, day calendar.Date) ([]weekProtocolDayBlock, error) {
	date := day.String()
	instances, err := e.service.ListActivityInstances(ctx, timetable.ActivityInstanceFilter{Date: &date, OrderByDateAndTime: true})
	if err != nil {
		return nil, fmt.Errorf("list the day's blocks: %w", err)
	}
	duties, err := e.dutyTemplates(ctx, instances)
	if err != nil {
		return nil, err
	}
	kept := make([]timetable.ActivityInstance, 0, len(instances))
	ids := make([]int64, 0, len(instances))
	for _, instance := range instances {
		if instance.ActivityGroupID != nil && duties[*instance.ActivityGroupID] {
			continue
		}
		kept = append(kept, instance)
		ids = append(ids, instance.ID)
	}
	groups, err := e.service.BlockEducationGroups(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("name the groups of the day's blocks: %w", err)
	}
	result := make([]weekProtocolDayBlock, 0, len(kept))
	for _, instance := range kept {
		result = append(result, weekProtocolDayBlock{block: weekProtocolBlock(instance), groups: groups[instance.ID]})
	}
	return result, nil
}

func (e weekProtocolEvening) dutyTemplates(ctx context.Context, instances []timetable.ActivityInstance) (map[int64]bool, error) {
	ids := []int64{}
	for _, instance := range instances {
		if instance.ActivityGroupID != nil && !slices.Contains(ids, *instance.ActivityGroupID) {
			ids = append(ids, *instance.ActivityGroupID)
		}
	}
	duties := map[int64]bool{}
	if len(ids) == 0 {
		return duties, nil
	}
	templates, err := e.service.ListGroups(ctx, timetable.GroupFilter{IDs: ids})
	if err != nil {
		return nil, fmt.Errorf("list the templates of the day's blocks: %w", err)
	}
	for _, template := range templates {
		if !timetable.GroupTypeHasChildren(template.Type) {
			duties[template.ID] = true
		}
	}
	return duties, nil
}

func weekProtocolBlock(instance timetable.ActivityInstance) timetable.WeekProtocolBlock {
	block := timetable.WeekProtocolBlock{
		Title: instance.Title, StartTime: clockOf(instance.StartTime), EndTime: clockOf(instance.EndTime), Status: instance.Status,
	}
	if instance.Status == timetable.InstanceStatusCancelled && instance.CancelReason != nil {
		block.CancelReason = strings.TrimSpace(*instance.CancelReason)
	}
	return block
}

// clockOf shortens a TIME value to HH:MM.
func clockOf(value string) string {
	if len(value) >= 5 {
		return value[:5]
	}
	return value
}

func blocksOfTarget(blocks []weekProtocolDayBlock, target weekProtocolTarget) []timetable.WeekProtocolBlock {
	result := []timetable.WeekProtocolBlock{}
	for _, block := range blocks {
		if target.target.GroupID == nil || slices.Contains(block.groups, target.key) {
			result = append(result, block.block)
		}
	}
	return result
}

// dayNotices are the staff notices of the day as the team read them.
func (e weekProtocolEvening) dayNotices(ctx context.Context, day calendar.Date) ([]timetable.WeekProtocolNotice, error) {
	views, err := e.sources.Notices.StaffNoticesOn(ctx, 0, day.String(), timetable.StaffNoticeAudienceStaff)
	if err != nil {
		return nil, fmt.Errorf("list the day's staff notices: %w", err)
	}
	notices := make([]timetable.WeekProtocolNotice, 0, len(views))
	for _, view := range views {
		notices = append(notices, timetable.WeekProtocolNotice{Title: view.Title, Body: strings.TrimSpace(view.Body)})
	}
	return notices, nil
}
