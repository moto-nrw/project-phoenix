package compose

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// Staff availability pool for one care block (#1884): who could cover this
// block's time window? Every staff member is categorized against the window
// from the Dienstplan and the same-day block assignments. Read-only; the
// atomic move endpoint consumes the same facts to validate its write.

// StaffPoolForInstance categorizes every staff member against the block's
// window. Same-day semantics mirror the deviation flow: an absence is
// day-wide, and only planned or running blocks count as occupying.
func (d *conflictDetection) StaffPoolForInstance(ctx context.Context, instanceID int64) (timetable.StaffPool, error) {
	instance, err := d.deps.Instances.FindByID(ctx, instanceID)
	if err != nil {
		if modelBase.IsNoRows(err) {
			return timetable.StaffPool{}, fmt.Errorf("%w: %w", timetable.ErrActivityInstanceNotFound, err)
		}
		return timetable.StaffPool{}, err
	}
	facts, dienstplanInUse, staff, err := d.loadStaffPoolFacts(ctx, instance)
	if err != nil {
		return timetable.StaffPool{}, err
	}
	entries := make([]timetable.StaffPoolEntry, 0, len(staff))
	for _, member := range staff {
		if member != nil {
			entries = append(entries, buildStaffPoolEntry(instance, member.ID, staffDisplayName(member), facts))
		}
	}
	return timetable.StaffPool{
		InstanceID:      instance.ID,
		Title:           instance.Title,
		Date:            timezone.Date(instance.Date),
		StartTime:       instance.StartTime,
		EndTime:         instance.EndTime,
		DienstplanInUse: dienstplanInUse,
		Entries:         entries,
	}, nil
}

// loadStaffPoolFacts reads the day's blocks, their staff rows, the day's
// shifts, the week's Dienstplan usage and the staff directory.
func (d *conflictDetection) loadStaffPoolFacts(ctx context.Context, instance *scheduleModels.ActivityInstance) (*staffPoolFacts, bool, []*usersModels.Staff, error) {
	date := instance.Date
	sameDay, err := d.deps.Instances.FindByTenantAndDateRange(ctx, date, date)
	if err != nil {
		return nil, false, nil, fmt.Errorf("load same-day instances: %w", err)
	}
	plannable := make([]*scheduleModels.ActivityInstance, 0, len(sameDay))
	allIDs := make([]int64, 0, len(sameDay))
	for _, candidate := range sameDay {
		if candidate == nil {
			continue
		}
		// Absence is day-wide and remains a fact even when the row that
		// carries it belongs to a completed or cancelled historical block.
		// Occupancy below still uses only planned or running blocks.
		allIDs = append(allIDs, candidate.ID)
		if isPlannableInstance(candidate) {
			plannable = append(plannable, candidate)
		}
	}
	rows, err := d.deps.InstanceStaff.FindByInstanceIDs(ctx, allIDs)
	if err != nil {
		return nil, false, nil, fmt.Errorf("load same-day assignments: %w", err)
	}
	shifts, err := d.deps.Shifts.FindByDateRange(ctx, date, date)
	if err != nil {
		return nil, false, nil, fmt.Errorf("load shifts: %w", err)
	}
	weekFrom, weekTo := timetable.ContainingCalendarWeek(timezone.Date(date))
	usedWeeks, err := d.deps.Shifts.FindUsedCalendarWeeks(ctx, scheduleModels.Date(weekFrom), scheduleModels.Date(weekTo))
	if err != nil {
		return nil, false, nil, fmt.Errorf("load used shift weeks: %w", err)
	}
	staff, err := d.deps.Staff.ListAllWithPerson(ctx)
	if err != nil {
		return nil, false, nil, fmt.Errorf("load staff directory: %w", err)
	}
	sortStaffByName(staff)
	return buildStaffPoolFacts(instance, plannable, rows, shifts), len(usedWeeks) > 0, staff, nil
}

// staffPoolFacts indexes the per-day raw material once per request.
type staffPoolFacts struct {
	onTarget      map[int64]bool
	absentReason  map[int64]*string
	absent        map[int64]bool
	elsewhere     map[int64][]timetable.StaffPoolAssignment
	shiftsByStaff map[int64][]*scheduleModels.StaffShift
}

func buildStaffPoolFacts(
	target *scheduleModels.ActivityInstance,
	plannable []*scheduleModels.ActivityInstance,
	rows []*scheduleModels.InstanceStaff,
	shifts []*scheduleModels.StaffShift,
) *staffPoolFacts {
	instanceByID := make(map[int64]*scheduleModels.ActivityInstance, len(plannable))
	for _, instance := range plannable {
		instanceByID[instance.ID] = instance
	}
	facts := &staffPoolFacts{
		onTarget:      make(map[int64]bool),
		absentReason:  make(map[int64]*string),
		absent:        make(map[int64]bool),
		elsewhere:     make(map[int64][]timetable.StaffPoolAssignment),
		shiftsByStaff: make(map[int64][]*scheduleModels.StaffShift),
	}
	for _, row := range rows {
		if row != nil {
			facts.addAssignment(target, instanceByID, row)
		}
	}
	for staffID := range facts.elsewhere {
		list := facts.elsewhere[staffID]
		sort.Slice(list, func(i, j int) bool { return list[i].StartTime < list[j].StartTime })
	}
	for _, shift := range shifts {
		if shift != nil && !shift.Cancelled {
			facts.shiftsByStaff[shift.StaffID] = append(facts.shiftsByStaff[shift.StaffID], shift)
		}
	}
	return facts
}

// addAssignment records one same-day staff row: an absence (day-wide,
// #1840 — one absent row marks the person absent for the whole pool,
// whichever block carries it), the target's own assignment, or an
// overlapping assignment elsewhere.
func (f *staffPoolFacts) addAssignment(
	target *scheduleModels.ActivityInstance,
	instanceByID map[int64]*scheduleModels.ActivityInstance,
	row *scheduleModels.InstanceStaff,
) {
	if row.IsAbsent {
		f.absent[row.StaffID] = true
		if f.absentReason[row.StaffID] == nil && row.AbsenceReason != nil {
			f.absentReason[row.StaffID] = row.AbsenceReason
		}
		return
	}
	if row.InstanceID == target.ID {
		f.onTarget[row.StaffID] = true
		return
	}
	instance := instanceByID[row.InstanceID]
	if instance == nil || !clockWindowsOverlap(instance.StartTime, instance.EndTime, target.StartTime, target.EndTime) {
		return
	}
	f.elsewhere[row.StaffID] = append(f.elsewhere[row.StaffID], timetable.StaffPoolAssignment{
		InstanceID:   instance.ID,
		Title:        instance.Title,
		StartTime:    clockLabel(instance.StartTime),
		EndTime:      clockLabel(instance.EndTime),
		IsSubstitute: row.IsSubstitute,
	})
}

func buildStaffPoolEntry(target *scheduleModels.ActivityInstance, staffID int64, name string, facts *staffPoolFacts) timetable.StaffPoolEntry {
	entry := timetable.StaffPoolEntry{
		StaffID:      staffID,
		DisplayName:  name,
		ShiftWindows: make([]string, 0),
		Assignments:  make([]timetable.StaffPoolAssignment, 0),
	}
	staffShifts := facts.shiftsByStaff[staffID]
	for _, shift := range staffShifts {
		entry.ShiftWindows = append(entry.ShiftWindows, fmt.Sprintf("%s–%s", clockLabel(shift.StartTime), clockLabel(shift.EndTime)))
		if clockWindowsOverlap(shift.StartTime, shift.EndTime, target.StartTime, target.EndTime) {
			entry.OnShift = true
		}
	}
	sort.Strings(entry.ShiftWindows)
	if entry.OnShift {
		entry.CoversWindow = len(timetable.UncoveredShiftIntervals(
			timezone.NormalizeWallClock(target.StartTime), timezone.NormalizeWallClock(target.EndTime), shiftWindowsOf(staffShifts),
		)) == 0
	}
	switch {
	case facts.onTarget[staffID]:
		entry.Category = timetable.StaffPoolAssignedHere
	case facts.absent[staffID]:
		entry.Category = timetable.StaffPoolAbsent
		entry.AbsenceReason = facts.absentReason[staffID]
	case len(facts.elsewhere[staffID]) > 0:
		entry.Category = timetable.StaffPoolAssignedElsewhere
		entry.Assignments = facts.elsewhere[staffID]
	case entry.OnShift:
		entry.Category = timetable.StaffPoolOnShiftFree
	default:
		entry.Category = timetable.StaffPoolNotOnShift
	}
	return entry
}

// staffDisplayName renders "First Last"; a person-less staff row degrades to
// an empty name (the planner renders a fallback).
func staffDisplayName(member *usersModels.Staff) string {
	if member == nil || member.Person == nil {
		return ""
	}
	return member.Person.FirstName + " " + member.Person.LastName
}

// sortStaffByName orders staff by last name, first name and ID, the order the
// staff pool and the Dienstplan overview present them in.
func sortStaffByName(staff []*usersModels.Staff) {
	sort.Slice(staff, func(i, j int) bool {
		left, right := staffSortName(staff[i]), staffSortName(staff[j])
		if left == right {
			return staffSortID(staff[i]) < staffSortID(staff[j])
		}
		return left < right
	})
}

func staffSortID(staff *usersModels.Staff) int64 {
	if staff == nil {
		return 0
	}
	return staff.ID
}

func staffSortName(staff *usersModels.Staff) string {
	if staff == nil || staff.Person == nil {
		return ""
	}
	return strings.ToLower(staff.Person.LastName + "\x00" + staff.Person.FirstName)
}
