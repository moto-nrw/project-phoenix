package compose

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// errPickupBaselineMissing reports a composition without the pickup
// baseline; errArrivalBaselineMissing is the arrival twin.
var errPickupBaselineMissing = errors.New("load pickup schedules: baseline projection is not configured")

// StudentWeek reads a child's timetable over the window with one read per
// kind, whatever the window's length: the planned blocks with the child's
// slot, every block of the tenant (for the walk-ins), the child's visits in
// running or ended sessions, the regular arrival and pickup through Care
// Plan's baselines and the dated exceptions.
func (d *timetableData) StudentWeek(ctx context.Context, studentID int64, from, to timezone.Date) (*timetable.StudentWeek, error) {
	week := &timetable.StudentWeek{
		EnrolledByDate:      map[string][]timetable.StudentWeekEntry{},
		InstancesByDate:     map[string][]timetable.ScheduledInstance{},
		VisitsByActiveGroup: map[int64][]timetable.StudentWeekVisit{},
		ArrivalByDate:       map[string]timetable.StudentWeekTimes{},
		PickupByDate:        map[string]timetable.StudentWeekTimes{},
	}
	if err := d.loadStudentWeekBlocks(ctx, week, studentID, from, to); err != nil {
		return nil, err
	}
	if err := d.loadStudentWeekBaselines(ctx, week, studentID, from, to); err != nil {
		return nil, err
	}
	if err := d.loadStudentWeekExceptions(ctx, week, studentID, from, to); err != nil {
		return nil, err
	}
	return week, nil
}

func (d *timetableData) loadStudentWeekBlocks(ctx context.Context, week *timetable.StudentWeek, studentID int64, from, to timezone.Date) error {
	enrolled, err := d.deps.Participants.FindInstancesWithAttendanceByStudentAndDateRange(ctx, studentID, scheduleModels.Date(from), scheduleModels.Date(to))
	if err != nil {
		return fmt.Errorf("load enrolled instances: %w", err)
	}
	for _, row := range enrolled {
		key := timezone.Date(row.Instance.Date).String()
		week.EnrolledByDate[key] = append(week.EnrolledByDate[key], timetable.StudentWeekEntry{
			Instance: ScheduledInstanceOf(row.Instance), Attendance: ScheduledParticipantOf(row.Attendance),
		})
	}
	instances, err := d.deps.Instances.FindByTenantAndDateRange(ctx, scheduleModels.Date(from), scheduleModels.Date(to))
	if err != nil {
		return fmt.Errorf("load all tenant instances: %w", err)
	}
	// One visit read covers every session the window's running or ended
	// blocks were bridged to; planned and cancelled blocks have none.
	activeGroupIDs := make([]int64, 0, len(instances))
	seen := make(map[int64]bool, len(instances))
	for _, inst := range instances {
		key := timezone.Date(inst.Date).String()
		week.InstancesByDate[key] = append(week.InstancesByDate[key], ScheduledInstanceOf(inst))
		if inst.ActiveGroupID == nil || seen[*inst.ActiveGroupID] ||
			(inst.Status != scheduleModels.InstanceStatusActive && inst.Status != scheduleModels.InstanceStatusCompleted) {
			continue
		}
		seen[*inst.ActiveGroupID] = true
		activeGroupIDs = append(activeGroupIDs, *inst.ActiveGroupID)
	}
	if len(activeGroupIDs) == 0 {
		return nil
	}
	visits, err := d.deps.Visits.ListVisits(ctx, studentpresence.VisitFilter{StudentIDs: []int64{studentID}, ActiveGroupIDs: activeGroupIDs})
	if err != nil {
		return fmt.Errorf("load student visits: %w", err)
	}
	for _, visit := range visits {
		week.VisitsByActiveGroup[visit.ActiveGroupID] = append(week.VisitsByActiveGroup[visit.ActiveGroupID], timetable.StudentWeekVisit{
			ID: visit.ID, EntryTime: visit.EntryTime, ExitTime: visit.ExitTime,
		})
	}
	return nil
}

// loadStudentWeekBaselines projects the regular arrival and pickup onto the
// window: with enrollment.bookings_authoritative on, a weekday stops being a
// care day the moment the booking ends, so the plan is keyed by date, not by
// weekday (#2414, ADR 0005).
func (d *timetableData) loadStudentWeekBaselines(ctx context.Context, week *timetable.StudentWeek, studentID int64, from, to timezone.Date) error {
	if d.deps.ArrivalBaselines == nil {
		return errArrivalBaselineMissing
	}
	arrivals, err := d.deps.ArrivalBaselines.ProjectArrivals(ctx, []int64{studentID}, from, to)
	if err != nil {
		return fmt.Errorf("load arrival schedules: %w", err)
	}
	for date := from; !date.After(to); date = date.AddDays(1) {
		if arrival, ok := arrivals.ExpectedArrival(studentID, date); ok {
			week.ArrivalByDate[date.String()] = timetable.StudentWeekTimes{HasSchedule: true, Time: arrival}
		}
	}
	if d.deps.PickupBaselines == nil {
		return errPickupBaselineMissing
	}
	pickups, err := d.deps.PickupBaselines.ProjectPickups(ctx, []int64{studentID}, from, to)
	if err != nil {
		return fmt.Errorf("load pickup schedules: %w", err)
	}
	for date := from; !date.After(to); date = date.AddDays(1) {
		if pickup, ok := pickups.RegularPickup(studentID, date); ok {
			week.PickupByDate[date.String()] = timetable.StudentWeekTimes{HasSchedule: true, Time: pickup}
		}
	}
	return nil
}

// loadStudentWeekExceptions reads the window's dated arrival and pickup
// exceptions, never the unbounded history.
func (d *timetableData) loadStudentWeekExceptions(ctx context.Context, week *timetable.StudentWeek, studentID int64, from, to timezone.Date) error {
	arrivals, err := d.deps.ArrivalExceptions.FindByStudentIDAndDateRange(ctx, studentID, scheduleModels.Date(from), scheduleModels.Date(to))
	if err != nil {
		return fmt.Errorf("load arrival exceptions: %w", err)
	}
	for _, exception := range arrivals {
		key := timezone.Date(exception.ExceptionDate).String()
		times := week.ArrivalByDate[key]
		times.Exception = &timetable.StudentDayException{Time: exception.ExpectedArrival, Reason: exception.Reason}
		week.ArrivalByDate[key] = times
	}
	pickups, err := d.deps.PickupExceptions.FindByStudentIDAndDateRange(ctx, studentID, scheduleModels.Date(from), scheduleModels.Date(to))
	if err != nil {
		return fmt.Errorf("load pickup exceptions: %w", err)
	}
	for _, exception := range pickups {
		key := timezone.Date(exception.ExceptionDate).String()
		times := week.PickupByDate[key]
		times.Exception = &timetable.StudentDayException{Time: exception.PickupTime, Reason: exception.Reason}
		week.PickupByDate[key] = times
	}
	return nil
}
