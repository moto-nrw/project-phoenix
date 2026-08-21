package schedule

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModel "github.com/moto-nrw/project-phoenix/models/active"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
)

// StudentWeekPreload holds the range-wide batch-query results used to assemble
// the per-student day/week view. All lookups are keyed by Berlin-local
// YYYY-MM-DD except VisitsByActiveGroup, which keys by active_group_id.
type StudentWeekPreload struct {
	EnrolledByDate       map[string][]*scheduleModel.ScheduledInstanceRow
	InstancesByDate      map[string][]*scheduleModel.ActivityInstance
	VisitsByActiveGroup  map[int64][]*activeModel.Visit
	ArrivalSchedByWeekly map[int]*scheduleModel.StudentArrivalSchedule
	ArrivalExcByDate     map[string]*scheduleModel.StudentArrivalException
	PickupSchedByDate    map[string]*scheduleModel.StudentPickupSchedule
	PickupExcByDate      map[string]*scheduleModel.StudentPickupException
}

// timetableDateKey formats a calendar date as the YYYY-MM-DD map key used
// across the read-side preloads.
func timetableDateKey(d timezone.Date) string { return d.String() }

// PreloadStudentWeek issues one DB query per category for the full [from, to]
// range and returns the data indexed by date. Constant query count regardless
// of range length — no per-day N+1.
func (s *TimetableDataService) PreloadStudentWeek(ctx context.Context, studentID int64, from, to timezone.Date) (*StudentWeekPreload, error) {
	out := &StudentWeekPreload{
		EnrolledByDate:       map[string][]*scheduleModel.ScheduledInstanceRow{},
		InstancesByDate:      map[string][]*scheduleModel.ActivityInstance{},
		VisitsByActiveGroup:  map[int64][]*activeModel.Visit{},
		ArrivalSchedByWeekly: map[int]*scheduleModel.StudentArrivalSchedule{},
		ArrivalExcByDate:     map[string]*scheduleModel.StudentArrivalException{},
		PickupSchedByDate:    map[string]*scheduleModel.StudentPickupSchedule{},
		PickupExcByDate:      map[string]*scheduleModel.StudentPickupException{},
	}

	enrolledRows, err := s.deps.InstanceStudentRepo.FindInstancesWithAttendanceByStudentAndDateRange(ctx, studentID, from, to)
	if err != nil {
		return nil, fmt.Errorf("load enrolled instances: %w", err)
	}
	for _, row := range enrolledRows {
		k := timetableDateKey(row.Instance.Date)
		out.EnrolledByDate[k] = append(out.EnrolledByDate[k], row)
	}

	allInstances, err := s.deps.ActivityInstanceRepo.FindByTenantAndDateRange(ctx, from, to)
	if err != nil {
		return nil, fmt.Errorf("load all tenant instances: %w", err)
	}

	// Collect active_group_ids across the range for a single visits lookup.
	// Only active/completed instances are relevant — planned/cancelled never
	// have visits bridged.
	activeGroupIDs := make([]int64, 0, len(allInstances))
	seenAGID := make(map[int64]bool, len(allInstances))
	for _, inst := range allInstances {
		k := timetableDateKey(inst.Date)
		out.InstancesByDate[k] = append(out.InstancesByDate[k], inst)
		if inst.ActiveGroupID == nil {
			continue
		}
		if inst.Status != scheduleModel.InstanceStatusActive &&
			inst.Status != scheduleModel.InstanceStatusCompleted {
			continue
		}
		if !seenAGID[*inst.ActiveGroupID] {
			seenAGID[*inst.ActiveGroupID] = true
			activeGroupIDs = append(activeGroupIDs, *inst.ActiveGroupID)
		}
	}

	if len(activeGroupIDs) > 0 {
		visits, err := s.deps.VisitRepo.FindByStudentAndActiveGroupIDs(ctx, studentID, activeGroupIDs)
		if err != nil {
			return nil, fmt.Errorf("load student visits: %w", err)
		}
		for _, v := range visits {
			if v == nil {
				continue
			}
			out.VisitsByActiveGroup[v.ActiveGroupID] = append(out.VisitsByActiveGroup[v.ActiveGroupID], v)
		}
	}

	// Arrival/pickup weekly schedules: one query each (up to 5 rows/student).
	arrivalSchedules, err := s.deps.ArrivalScheduleRepo.FindByStudentID(ctx, studentID)
	if err != nil {
		return nil, fmt.Errorf("load arrival schedules: %w", err)
	}
	for _, sc := range arrivalSchedules {
		out.ArrivalSchedByWeekly[sc.Weekday] = sc
	}
	if s.deps.PickupBaselines == nil {
		return nil, fmt.Errorf("load pickup schedules: baseline projection is not configured")
	}
	pickupSchedules, err := s.deps.PickupBaselines.Project(ctx, []int64{studentID}, from, to)
	if err != nil {
		return nil, fmt.Errorf("load pickup schedules: %w", err)
	}
	for date := from; !date.After(to); date = date.AddDays(1) {
		if row := pickupSchedules.ForDate(studentID, date); row != nil {
			out.PickupSchedByDate[timetableDateKey(date)] = row
		}
	}

	// Arrival/pickup exceptions: range-scoped (avoid unbounded history).
	arrivalExcs, err := s.deps.ArrivalExceptionRepo.FindByStudentIDAndDateRange(ctx, studentID, from, to)
	if err != nil {
		return nil, fmt.Errorf("load arrival exceptions: %w", err)
	}
	for _, e := range arrivalExcs {
		out.ArrivalExcByDate[timetableDateKey(e.ExceptionDate)] = e
	}
	pickupExcs, err := s.deps.PickupExceptionRepo.FindByStudentIDAndDateRange(ctx, studentID, from, to)
	if err != nil {
		return nil, fmt.Errorf("load pickup exceptions: %w", err)
	}
	for _, e := range pickupExcs {
		out.PickupExcByDate[timetableDateKey(e.ExceptionDate)] = e
	}

	return out, nil
}
