package compose

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// plannedNowAccess is what the caller may see and do on the day plan.
type plannedNowAccess struct {
	staffID        int64
	hasStaff       bool
	allOperational bool
	adminActions   bool
}

// plannedNowCandidate is one block that survived the planned-now filters,
// with the rows already loaded for it.
type plannedNowCandidate struct {
	instance    *scheduleModels.ActivityInstance
	staffRows   []*scheduleModels.InstanceStaff
	studentRows []*scheduleModels.InstanceStudent
	roomName    *string
	canOperate  bool
}

func (s *operations) PlannedNow(ctx context.Context, accountID int64, isAdmin bool, date timezone.Date, now time.Time, opts timetable.PlannedNowOptions) ([]timetable.OperationPlannedInstance, error) {
	startLead, err := s.deps.Settings.StartLeadMinutes(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: resolve start lead: %v", timetable.ErrLifecycleSettings, err)
	}
	access, err := s.plannedNowAccess(ctx, accountID, isAdmin)
	if err != nil {
		return nil, err
	}
	instances, err := s.deps.Instances.FindByTenantAndDate(ctx, scheduleModels.Date(date))
	if err != nil {
		return nil, err
	}
	roomNames, err := s.roomNameMap(ctx)
	if err != nil {
		return nil, err
	}
	candidates, err := s.plannedNowCandidates(ctx, eligiblePlannedNow(instances, now, opts, startLead), access, opts.Limit, roomNames)
	if err != nil {
		return nil, err
	}
	// Every block falls on the same date, so one care-day resolution covers
	// them all; resolving per block would be a query burst (#1747).
	careDay, err := s.deps.CareDays.ResolveForDate(ctx, plannedNowStudentIDs(candidates), date)
	if err != nil {
		return nil, err
	}
	out := make([]timetable.OperationPlannedInstance, 0, len(candidates))
	for _, candidate := range candidates {
		mapped, err := s.plannedNowEntry(ctx, candidate, now, opts, startLead, access.staffID, careDay)
		if err != nil {
			return nil, err
		}
		out = append(out, mapped)
	}
	if opts.Scope == timetable.PlannedNowScopeDay {
		if err := s.enrichDayPlan(ctx, candidates, out); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// plannedNowAccess resolves the caller's staff profile and overview rights.
// A caller without staff profile sees the plan only under the all-staff
// operational overview.
func (s *operations) plannedNowAccess(ctx context.Context, accountID int64, isAdmin bool) (plannedNowAccess, error) {
	staffID, hasStaff, err := s.resolveStaffID(ctx, accountID)
	if err != nil {
		return plannedNowAccess{}, err
	}
	access := plannedNowAccess{
		staffID:        staffID,
		hasStaff:       hasStaff,
		allOperational: s.operationalOverview(ctx, isAdmin, hasStaff),
		adminActions:   s.hasAdministrativeActionAccess(ctx, isAdmin),
	}
	if !hasStaff && !access.allOperational {
		return plannedNowAccess{}, timetable.ErrTimetableOperationForbidden
	}
	return access, nil
}

// eligiblePlannedNow keeps the blocks of the requested scope: the whole day
// in every state, today's finished blocks, or the planned blocks of the
// upcoming window (at least the start lead wide).
func eligiblePlannedNow(instances []*scheduleModels.ActivityInstance, now time.Time, opts timetable.PlannedNowOptions, startLead int) []*scheduleModels.ActivityInstance {
	horizon := opts.HorizonMinutes
	if horizon <= 0 {
		horizon = 15
	}
	if startLead > horizon {
		horizon = startLead
	}
	eligible := make([]*scheduleModels.ActivityInstance, 0, len(instances))
	for _, inst := range instances {
		switch opts.Scope {
		case timetable.PlannedNowScopeDay:
			// Every state stays in: a cancelled block ("fällt aus") and a
			// finished one are both answers the person came for.
		case timetable.PlannedNowScopePast:
			if !plannedPastToday(inst, now) {
				continue
			}
		default:
			if inst.Status != scheduleModels.InstanceStatusPlanned || !plannedNowWindow(inst, now, horizon) {
				continue
			}
		}
		eligible = append(eligible, inst)
	}
	return eligible
}

// plannedNowCandidates keeps the blocks the caller may see (all of them
// under the operational overview, otherwise their own assignments) up to
// the limit and loads their staff and participant rows in one read each.
func (s *operations) plannedNowCandidates(
	ctx context.Context,
	eligible []*scheduleModels.ActivityInstance,
	access plannedNowAccess,
	limit int,
	roomNames map[int64]*string,
) ([]plannedNowCandidate, error) {
	staffRows, err := s.deps.InstanceStaff.FindByInstanceIDs(ctx, retainedInstanceIDs(eligible))
	if err != nil {
		return nil, err
	}
	staffByInstance := indexInstanceStaffRows(staffRows)
	visible := make([]*scheduleModels.ActivityInstance, 0, len(eligible))
	for _, inst := range eligible {
		if !access.allOperational && !staffAssigned(staffByInstance[inst.ID], access.staffID) {
			continue
		}
		visible = append(visible, inst)
		if limit > 0 && len(visible) >= limit {
			break
		}
	}
	studentRows, err := s.deps.Participants.FindByInstanceIDs(ctx, retainedInstanceIDs(visible))
	if err != nil {
		return nil, err
	}
	studentsByInstance := indexInstanceStudentRows(studentRows)
	candidates := make([]plannedNowCandidate, 0, len(visible))
	for _, inst := range visible {
		assigned := staffAssigned(staffByInstance[inst.ID], access.staffID)
		candidates = append(candidates, plannedNowCandidate{
			instance:    inst,
			staffRows:   staffByInstance[inst.ID],
			studentRows: studentsByInstance[inst.ID],
			roomName:    roomNames[inst.RoomID],
			canOperate:  access.hasStaff && (access.adminActions || assigned),
		})
	}
	return candidates, nil
}

func plannedNowStudentIDs(candidates []plannedNowCandidate) []int64 {
	seen := map[int64]bool{}
	ids := make([]int64, 0)
	for _, candidate := range candidates {
		for _, row := range candidate.studentRows {
			if !seen[row.StudentID] {
				seen[row.StudentID] = true
				ids = append(ids, row.StudentID)
			}
		}
	}
	return ids
}

// plannedNowEntry maps one candidate with its start lifecycle and, when
// asked, its roster preview. Past blocks are read-only; in the whole-day
// scope only a still planned block carries a start lifecycle.
func (s *operations) plannedNowEntry(
	ctx context.Context,
	candidate plannedNowCandidate,
	now time.Time,
	opts timetable.PlannedNowOptions,
	startLead int,
	staffID int64,
	careDay map[int64]timetable.CareDayStatus,
) (timetable.OperationPlannedInstance, error) {
	mapped := s.mapPlannedInstance(candidate, now, staffID, careDay)
	past := opts.Scope == timetable.PlannedNowScopePast
	wholeDay := opts.Scope == timetable.PlannedNowScopeDay
	if candidate.canOperate && !past && (!wholeDay || candidate.instance.Status == scheduleModels.InstanceStatusPlanned) {
		availability := timetable.EvaluateLifecycleAvailability(lifecycleWindow(candidate.instance), now, startLead, true)
		mapped.CanStart = availability.CanStart
		mapped.StartAvailableAt = availability.StartAvailableAt.Format(time.RFC3339)
		if !candidate.instance.IsSpontaneous {
			mapped.StartExpiresAt = availability.CompleteAvailableAt.Format(time.RFC3339)
		}
	}
	if opts.IncludeRoster {
		roster, err := s.buildRosterWithCareDay(ctx, candidate.instance.ID, careDay)
		if err != nil {
			return timetable.OperationPlannedInstance{}, err
		}
		mapped.RosterPreview = roster.Rows
		mapped.PickupTimesLoaded = roster.PickupTimesLoaded
	}
	return mapped, nil
}

// mapPlannedInstance maps a block with the caller's own assignment and the
// expected, present and not-booked children of the day.
func (s *operations) mapPlannedInstance(candidate plannedNowCandidate, now time.Time, currentStaffID int64, careDay map[int64]timetable.CareDayStatus) timetable.OperationPlannedInstance {
	inst := candidate.instance
	counts := s.plannedAttendanceCounts(inst, candidate.studentRows, careDay)
	start := instanceStartAt(inst, now.Location())
	mapped := timetable.OperationPlannedInstance{
		ID:                    inst.ID,
		Title:                 inst.Title,
		Date:                  inst.Date.Format("2006-01-02"),
		StartTime:             inst.StartTime.Format("15:04"),
		EndTime:               inst.EndTime.Format("15:04"),
		RoomID:                inst.RoomID,
		RoomName:              candidate.roomName,
		Status:                inst.Status,
		IsOverdue:             start.Before(now),
		MinutesUntilStart:     int(start.Sub(now).Minutes()),
		ExpectedStudentsCount: counts.expected,
		PresentStudentsCount:  counts.present,
		NotScheduledCount:     counts.notScheduled,
		Warnings:              []timetable.InstanceConflictWarning{},
		ActiveGroupID:         inst.ActiveGroupID,
		CancelReason:          inst.CancelReason,
	}
	applyPlannedStaff(&mapped, candidate.staffRows, currentStaffID)
	return mapped
}

// applyPlannedStaff fills the assigned staff and the caller's own role.
func applyPlannedStaff(mapped *timetable.OperationPlannedInstance, staffRows []*scheduleModels.InstanceStaff, currentStaffID int64) {
	mapped.AssignedStaffIDs = make([]int64, 0, len(staffRows))
	for _, row := range staffRows {
		if !row.IsAbsent {
			mapped.AssignedStaffIDs = append(mapped.AssignedStaffIDs, row.StaffID)
		}
		if currentStaffID > 0 && row.StaffID == currentStaffID {
			mapped.IsAssigned = !row.IsAbsent
			mapped.IsPrimary = row.IsPrimary
			mapped.IsSubstitute = row.IsSubstitute
			mapped.IsAbsent = row.IsAbsent
		}
	}
}

type plannedAttendanceCounts struct {
	expected, present, notScheduled int
}

// plannedAttendanceCounts counts the day's children. An assignment alone
// does not make a child expected: the care plan has to place them here on
// the weekday and nobody may have cancelled the day (#1747); an unknown
// verdict keeps the child expected. An absence a broad day status wrote onto
// an unbooked day counts as not scheduled, so the card explains the child
// the way the planner list does.
func (s *operations) plannedAttendanceCounts(inst *scheduleModels.ActivityInstance, rows []*scheduleModels.InstanceStudent, careDay map[int64]timetable.CareDayStatus) plannedAttendanceCounts {
	var counts plannedAttendanceCounts
	completed := inst.Status == scheduleModels.InstanceStatusCompleted
	for _, row := range rows {
		verdict := s.deps.CareDays.AttendanceRowCareDay(completed, careDayAttendance(row), careDay[row.StudentID])
		switch row.Status {
		case scheduleModels.AttendanceStatusExpected:
			if !s.deps.CareDays.Expected(verdict) {
				counts.notScheduled++
				continue
			}
			counts.expected++
		case scheduleModels.AttendanceStatusPresent:
			counts.present++
		case scheduleModels.AttendanceStatusAbsent:
			if verdict == timetable.CareDayNotScheduled {
				counts.notScheduled++
			}
		}
	}
	return counts
}

func plannedNowWindow(inst *scheduleModels.ActivityInstance, now time.Time, horizonMinutes int) bool {
	if horizonMinutes <= 0 {
		horizonMinutes = 15
	}
	start := instanceStartAt(inst, now.Location())
	end := instanceEndAt(inst, now.Location())
	if !inst.IsSpontaneous && !now.Before(end) {
		return false
	}
	return (start.After(now.Add(-15*time.Minute)) && start.Before(now.Add(time.Duration(horizonMinutes)*time.Minute))) || start.Before(now)
}

// plannedPastToday is the scope=past complement of plannedNowWindow
// (#2335): completed blocks, plus non-spontaneous planned blocks whose end
// has passed — those never started and stay "planned" forever. Spontaneous
// planned blocks stay in the default scope; cancelled and running blocks
// belong to neither list.
func plannedPastToday(inst *scheduleModels.ActivityInstance, now time.Time) bool {
	switch inst.Status {
	case scheduleModels.InstanceStatusCompleted:
		return true
	case scheduleModels.InstanceStatusPlanned:
		return !inst.IsSpontaneous && !now.Before(instanceEndAt(inst, now.Location()))
	default:
		return false
	}
}

func instanceStartAt(inst *scheduleModels.ActivityInstance, loc *time.Location) time.Time {
	return time.Date(inst.Date.Year(), inst.Date.Month(), inst.Date.Day(), inst.StartTime.Hour(), inst.StartTime.Minute(), inst.StartTime.Second(), 0, loc)
}

func instanceEndAt(inst *scheduleModels.ActivityInstance, loc *time.Location) time.Time {
	return time.Date(inst.Date.Year(), inst.Date.Month(), inst.Date.Day(), inst.EndTime.Hour(), inst.EndTime.Minute(), inst.EndTime.Second(), 0, loc)
}

// roomNameMap names every room of the tenant.
func (s *operations) roomNameMap(ctx context.Context) (map[int64]*string, error) {
	rooms, err := s.deps.Rooms.AllRoomNames(ctx)
	if err != nil {
		return nil, err
	}
	names := make(map[int64]*string, len(rooms))
	for id, name := range rooms {
		name := name
		names[id] = &name
	}
	return names, nil
}
