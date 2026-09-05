// Package schedule — staff availability pool for one care block (#1884).
//
// GetStaffPoolForInstance answers "who could cover this block's time window?"
// for the Betreuungsplan: every staff member categorized against the block's
// window from the Dienstplan (schedule.staff_shifts) and the same-day block
// assignments (schedule.instance_staff). Read-only; the atomic move endpoint
// (instance_move_staff.go) consumes the same facts to validate the write.
package schedule

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModel "github.com/moto-nrw/project-phoenix/models/users"
)

// Staff pool categories, mutually exclusive per staff member. Precedence: a
// NON-absent assignment on the target block wins (assigned_here); otherwise
// any absent row on the day wins (absent is day-wide, #1840 — including when
// the target's own row carries the absence); then assigned_elsewhere >
// on_shift_free > not_on_shift.
const (
	StaffPoolAssignedHere      = "assigned_here"
	StaffPoolAbsent            = "absent"
	StaffPoolAssignedElsewhere = "assigned_elsewhere"
	StaffPoolOnShiftFree       = "on_shift_free"
	StaffPoolNotOnShift        = "not_on_shift"
)

// StaffPoolAssignment is one same-day block assignment overlapping the target
// window — the move source the frontend offers.
type StaffPoolAssignment struct {
	InstanceID   int64
	Title        string
	StartTime    string // HH:MM
	EndTime      string // HH:MM
	IsSubstitute bool
}

// StaffPoolEntry is one staff member categorized against the target window.
type StaffPoolEntry struct {
	StaffID     int64
	DisplayName string
	Category    string
	// OnShift reports whether any non-cancelled shift overlaps the window;
	// CoversWindow whether the union of that day's shifts covers it entirely.
	OnShift      bool
	CoversWindow bool
	ShiftWindows []string // "HH:MM–HH:MM" per non-cancelled shift that day
	// AbsenceReason carries the day-wide absence reason for the absent
	// category (first non-empty reason of the day's absent rows).
	AbsenceReason *string
	// Assignments lists the overlapping same-day blocks the person is already
	// planned on (assigned_elsewhere; empty otherwise).
	Assignments []StaffPoolAssignment
}

// StaffPoolResult is the categorized pool for one instance's window.
type StaffPoolResult struct {
	Instance *scheduleModel.ActivityInstance
	// DienstplanInUse mirrors the #1873 semantics: false when the instance's
	// calendar week has no shifts at all, so "not on shift" carries no signal.
	DienstplanInUse bool
	Entries         []StaffPoolEntry
}

// GetStaffPoolForInstance categorizes every staff member against the block's
// window (#1884). Same-day semantics mirror the deviation flow: an absence is
// day-wide, and only planned/active blocks count as occupying.
func (s *TimetableDataService) GetStaffPoolForInstance(ctx context.Context, instanceID int64) (*StaffPoolResult, error) {
	instance, err := s.deps.ActivityInstanceRepo.FindByID(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	date := instance.Date

	sameDay, err := s.deps.ActivityInstanceRepo.FindByTenantAndDateRange(ctx, date, date)
	if err != nil {
		return nil, fmt.Errorf("load same-day instances: %w", err)
	}
	plannable := make([]*scheduleModel.ActivityInstance, 0, len(sameDay))
	allIDs := make([]int64, 0, len(sameDay))
	for _, inst := range sameDay {
		if inst == nil {
			continue
		}
		// Absence is day-wide and remains a fact even when the row that carries
		// it belongs to a completed/cancelled historical block. Occupancy below
		// still uses only planned/active blocks.
		allIDs = append(allIDs, inst.ID)
		if !isPlannableInstance(inst) {
			continue
		}
		plannable = append(plannable, inst)
	}
	rows, err := s.deps.InstanceStaffRepo.FindByInstanceIDs(ctx, allIDs)
	if err != nil {
		return nil, fmt.Errorf("load same-day assignments: %w", err)
	}
	shifts, err := s.deps.StaffShiftRepo.FindByDateRange(ctx, date, date)
	if err != nil {
		return nil, fmt.Errorf("load shifts: %w", err)
	}
	weekFrom, weekTo := containingCalendarWeek(timezone.Date(date))
	usedWeeks, err := s.deps.StaffShiftRepo.FindUsedCalendarWeeks(ctx, scheduleModel.Date(weekFrom), scheduleModel.Date(weekTo))
	if err != nil {
		return nil, fmt.Errorf("load used shift weeks: %w", err)
	}
	staff, err := s.deps.StaffRepo.ListAllWithPerson(ctx)
	if err != nil {
		return nil, fmt.Errorf("load staff directory: %w", err)
	}
	sortOverviewStaff(staff)

	facts := buildStaffPoolFacts(instance, plannable, rows, shifts)
	entries := make([]StaffPoolEntry, 0, len(staff))
	for _, member := range staff {
		if member == nil {
			continue
		}
		entries = append(entries, buildStaffPoolEntry(instance, member.ID, staffDisplayName(member), facts))
	}

	return &StaffPoolResult{
		Instance:        instance,
		DienstplanInUse: len(usedWeeks) > 0,
		Entries:         entries,
	}, nil
}

// staffPoolFacts indexes the per-day raw material once per request.
type staffPoolFacts struct {
	onTarget      map[int64]bool
	absentReason  map[int64]*string
	absent        map[int64]bool
	elsewhere     map[int64][]StaffPoolAssignment
	shiftsByStaff map[int64][]*scheduleModel.StaffShift
}

func buildStaffPoolFacts(
	target *scheduleModel.ActivityInstance,
	plannable []*scheduleModel.ActivityInstance,
	rows []*scheduleModel.InstanceStaff,
	shifts []*scheduleModel.StaffShift,
) *staffPoolFacts {
	instanceByID := make(map[int64]*scheduleModel.ActivityInstance, len(plannable))
	for _, inst := range plannable {
		instanceByID[inst.ID] = inst
	}
	facts := &staffPoolFacts{
		onTarget:      make(map[int64]bool),
		absentReason:  make(map[int64]*string),
		absent:        make(map[int64]bool),
		elsewhere:     make(map[int64][]StaffPoolAssignment),
		shiftsByStaff: make(map[int64][]*scheduleModel.StaffShift),
	}
	for _, row := range rows {
		if row == nil {
			continue
		}
		if row.IsAbsent {
			// Absence is day-wide (#1840): one absent row marks the person
			// absent for the whole pool, whichever block carries it.
			facts.absent[row.StaffID] = true
			if facts.absentReason[row.StaffID] == nil && row.AbsenceReason != nil {
				facts.absentReason[row.StaffID] = row.AbsenceReason
			}
			continue
		}
		if row.InstanceID == target.ID {
			facts.onTarget[row.StaffID] = true
			continue
		}
		inst := instanceByID[row.InstanceID]
		if inst == nil || !clockWindowsOverlap(inst.StartTime, inst.EndTime, target.StartTime, target.EndTime) {
			continue
		}
		facts.elsewhere[row.StaffID] = append(facts.elsewhere[row.StaffID], StaffPoolAssignment{
			InstanceID:   inst.ID,
			Title:        inst.Title,
			StartTime:    timezone.NormalizeWallClock(inst.StartTime).Format("15:04"),
			EndTime:      timezone.NormalizeWallClock(inst.EndTime).Format("15:04"),
			IsSubstitute: row.IsSubstitute,
		})
	}
	for staffID := range facts.elsewhere {
		list := facts.elsewhere[staffID]
		sort.Slice(list, func(i, j int) bool { return list[i].StartTime < list[j].StartTime })
	}
	for _, shift := range shifts {
		if shift == nil || shift.Cancelled {
			continue
		}
		facts.shiftsByStaff[shift.StaffID] = append(facts.shiftsByStaff[shift.StaffID], shift)
	}
	return facts
}

func buildStaffPoolEntry(target *scheduleModel.ActivityInstance, staffID int64, name string, facts *staffPoolFacts) StaffPoolEntry {
	entry := StaffPoolEntry{
		StaffID:      staffID,
		DisplayName:  name,
		ShiftWindows: make([]string, 0),
		Assignments:  make([]StaffPoolAssignment, 0),
	}
	staffShifts := facts.shiftsByStaff[staffID]
	for _, shift := range staffShifts {
		entry.ShiftWindows = append(entry.ShiftWindows, fmt.Sprintf("%s–%s",
			timezone.NormalizeWallClock(shift.StartTime).Format("15:04"),
			timezone.NormalizeWallClock(shift.EndTime).Format("15:04"),
		))
		if clockWindowsOverlap(shift.StartTime, shift.EndTime, target.StartTime, target.EndTime) {
			entry.OnShift = true
		}
	}
	sort.Strings(entry.ShiftWindows)
	if entry.OnShift {
		entry.CoversWindow = len(uncoveredShiftIntervals(
			timezone.NormalizeWallClock(target.StartTime), timezone.NormalizeWallClock(target.EndTime), staffShifts,
		)) == 0
	}

	switch {
	case facts.onTarget[staffID]:
		entry.Category = StaffPoolAssignedHere
	case facts.absent[staffID]:
		entry.Category = StaffPoolAbsent
		entry.AbsenceReason = facts.absentReason[staffID]
	case len(facts.elsewhere[staffID]) > 0:
		entry.Category = StaffPoolAssignedElsewhere
		entry.Assignments = facts.elsewhere[staffID]
	case entry.OnShift:
		entry.Category = StaffPoolOnShiftFree
	default:
		entry.Category = StaffPoolNotOnShift
	}
	return entry
}

// clockWindowsOverlap reports whether two wall-clock windows intersect;
// touching boundaries do not overlap (same rule as StaffShift.Overlaps).
func clockWindowsOverlap(aStart, aEnd, bStart, bEnd time.Time) bool {
	as, ae := timezone.NormalizeWallClock(aStart), timezone.NormalizeWallClock(aEnd)
	bs, be := timezone.NormalizeWallClock(bStart), timezone.NormalizeWallClock(bEnd)
	return as.Before(be) && bs.Before(ae)
}

// staffDisplayName renders "First Last"; a person-less staff row degrades to
// an empty name (frontend renders a fallback).
func staffDisplayName(member *usersModel.Staff) string {
	if member == nil || member.Person == nil {
		return ""
	}
	return member.Person.FirstName + " " + member.Person.LastName
}
