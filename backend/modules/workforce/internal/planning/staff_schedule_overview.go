package planning

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	facilitiesModel "github.com/moto-nrw/project-phoenix/models/facilities"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModel "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/timetable/legacy/timetableplanning"
)

const (
	CoverageStatusCovered       = "covered"
	CoverageStatusUncovered     = "uncovered"
	CoverageStatusNotApplicable = "not_applicable"

	CoverageReasonAbsent            = "absent"
	CoverageReasonDienstplanNotUsed = "dienstplan_not_used"
)

// StaffScheduleAssignment is the read-only bridge between one concrete
// activity-instance assignment and the staff member's shifts on that day.
type StaffScheduleAssignment struct {
	InstanceID    int64
	StaffID       int64
	Date          timezone.Date
	StartTime     time.Time
	EndTime       time.Time
	ActivityTitle string
	// ActivityGroupID identifies the Angebot this block was materialized from,
	// nil for a spontaneous block. Titles are not unique — two separately
	// configured Angebote may share a name — so a consumer grouping
	// assignments needs the identity, not the label (#2079).
	ActivityGroupID    *int64
	RoomID             int64
	RoomName           string
	Status             string
	IsAbsent           bool
	IsSubstitute       bool
	AbsenceReason      *string
	CoverageStatus     string
	CoverageReason     *string
	UncoveredIntervals []timetableplanning.ShiftCoverageInterval
}

// StaffWeeklySummary aggregates one staff member's planned shift minutes for
// one Monday-anchored calendar week and compares them against the contractual
// weekly target (Arbeitszeitmodell) when one resolves. Planned minutes come
// exclusively from schedule.staff_shifts — timetable assignments are the
// tasks inside a shift and are never added on top. Weeks without any tenant
// shift produce no summary at all (Dienstplan unused, matching coverage).
type StaffWeeklySummary struct {
	StaffID        int64
	WeekStart      timezone.Date
	PlannedMinutes int
	TargetMinutes  *int
	DeltaMinutes   *int
}

// StaffScheduleOverview is the batched weekly projection consumed by the
// Dienstplan. It is deliberately read-only: no assignment is linked to or
// generated from a StaffShift row.
type StaffScheduleOverview struct {
	From            timezone.Date
	To              timezone.Date
	DienstplanInUse bool
	UsedWeeks       []timezone.Date
	Staff           []*usersModel.Staff
	Shifts          []*scheduleModel.StaffShift
	Assignments     []StaffScheduleAssignment
	WeeklySummaries []StaffWeeklySummary
}

// Narrow read interfaces keep this projection independently testable while
// the production repositories continue to implement the wider domain
// interfaces used by mutation services.
type StaffShiftRangeReader interface {
	FindByDateRange(ctx context.Context, start, end scheduleModel.Date) ([]*scheduleModel.StaffShift, error)
}

type StaffShiftWeekUsageReader interface {
	FindUsedCalendarWeeks(ctx context.Context, start, end scheduleModel.Date) ([]scheduleModel.Date, error)
}

type ActivityInstanceRangeReader interface {
	FindByTenantAndDateRange(ctx context.Context, from, to scheduleModel.Date) ([]*scheduleModel.ActivityInstance, error)
}

type RoomBatchReader interface {
	FindByIDs(ctx context.Context, ids []int64) ([]*facilitiesModel.Room, error)
}

type StaffOverviewReader interface {
	ListAllWithPerson(ctx context.Context) ([]*usersModel.Staff, error)
	FindWithPersonByIDs(ctx context.Context, ids []int64) (map[int64]*usersModel.Staff, error)
}

type StaffWorkScheduleBatchReader interface {
	FindByStaffIDsValidInRange(ctx context.Context, staffIDs []int64, from, to configModel.CalendarDate) ([]*configModel.StaffWorkSchedule, error)
}

type WorkTimeModelBatchReader interface {
	FindByIDs(ctx context.Context, ids []int64) ([]*configModel.WorkTimeModel, error)
}

type StaffScheduleOverviewDependencies struct {
	Shifts        StaffShiftRangeReader
	ShiftWeeks    StaffShiftWeekUsageReader
	Instances     ActivityInstanceRangeReader
	InstanceStaff timetableplanning.InstanceStaffBatchReader
	Rooms         RoomBatchReader
	Staff         StaffOverviewReader
	// WorkSchedules and WorkModels feed the contractual weekly-target side of
	// WeeklySummaries. Both are optional: a nil reader degrades to
	// planned-minutes-only summaries (TargetMinutes stays nil).
	WorkSchedules StaffWorkScheduleBatchReader
	WorkModels    WorkTimeModelBatchReader
	// Holidays reduces the weekly targets by the non-working-day Soll (#1418
	// 3a/3b), bound to the School Calendar; nil skips it (unit fixtures).
	Holidays HolidayDatesReader
}

// HolidayDatesReader answers the tenant's non-working days as a date set.
type HolidayDatesReader interface {
	HolidayDates(ctx context.Context, from, to timezone.Date) (map[timezone.Date]bool, error)
}

type StaffScheduleOverviewGetter interface {
	GetOverview(ctx context.Context, from, to timezone.Date) (*StaffScheduleOverview, error)
}

type staffScheduleOverviewService struct {
	deps StaffScheduleOverviewDependencies
}

func NewStaffScheduleOverviewService(deps StaffScheduleOverviewDependencies) StaffScheduleOverviewGetter {
	if deps.ShiftWeeks == nil {
		if reader, ok := deps.Shifts.(StaffShiftWeekUsageReader); ok {
			deps.ShiftWeeks = reader
		}
	}
	return &staffScheduleOverviewService{deps: deps}
}

type staffScheduleOverviewData struct {
	// shifts is scoped to the requested from..to range and feeds the grid
	// payload plus assignment coverage. weekShifts covers the full
	// Monday–Sunday span of every summarized week so weekly planned minutes
	// include shifts outside a partial viewport (e.g. weekend shifts on a
	// Mon–Fri request).
	shifts           []*scheduleModel.StaffShift
	weekShifts       []*scheduleModel.StaffShift
	usedWeeks        []timezone.Date
	visibleInstances []*scheduleModel.ActivityInstance
	assignmentRows   []*scheduleModel.InstanceStaff
	rooms            []*facilitiesModel.Room
	staff            []*usersModel.Staff
	workSchedules    []*configModel.StaffWorkSchedule
}

func (s *staffScheduleOverviewService) GetOverview(ctx context.Context, from, to timezone.Date) (*StaffScheduleOverview, error) {
	if err := validateShiftRange(from, to); err != nil {
		return nil, err
	}

	data, err := s.loadOverviewData(ctx, from, to)
	if err != nil {
		return nil, err
	}
	sortOverviewStaff(data.staff)

	usedWeeks := indexCalendarWeeks(data.usedWeeks)
	dienstplanInUse := len(usedWeeks) > 0
	assignments := buildStaffScheduleAssignments(
		data.visibleInstances,
		indexAssignmentRows(data.assignmentRows),
		indexRoomNames(data.rooms),
		indexShifts(data.shifts),
		usedWeeks,
	)
	weeklySummaries, err := s.buildWeeklySummaries(ctx, data)
	if err != nil {
		return nil, err
	}

	return &StaffScheduleOverview{
		From:            from,
		To:              to,
		DienstplanInUse: dienstplanInUse,
		UsedWeeks:       data.usedWeeks,
		Staff:           data.staff,
		Shifts:          data.shifts,
		Assignments:     assignments,
		WeeklySummaries: weeklySummaries,
	}, nil
}

func (s *staffScheduleOverviewService) loadOverviewData(ctx context.Context, from, to timezone.Date) (*staffScheduleOverviewData, error) {
	if s.deps.ShiftWeeks == nil {
		return nil, errors.New("staff-shift week usage reader is not wired")
	}
	firstWeekFrom, _ := containingCalendarWeek(from)
	_, lastWeekTo := containingCalendarWeek(to)
	// Load the full summarized weeks in one query; the viewport subset for the
	// grid is filtered locally so weekly summaries never undercount shifts
	// outside a partial requested range.
	weekShifts, err := s.deps.Shifts.FindByDateRange(ctx, scheduleModel.Date(firstWeekFrom), scheduleModel.Date(lastWeekTo))
	if err != nil {
		return nil, fmt.Errorf("load staff shifts: %w", err)
	}
	shifts := shiftsWithinRange(weekShifts, from, to)
	usedScheduleWeeks, err := s.deps.ShiftWeeks.FindUsedCalendarWeeks(ctx, scheduleModel.Date(firstWeekFrom), scheduleModel.Date(lastWeekTo))
	if err != nil {
		return nil, fmt.Errorf("load used staff-shift weeks: %w", err)
	}
	usedWeeks := make([]timezone.Date, len(usedScheduleWeeks))
	for index, week := range usedScheduleWeeks {
		usedWeeks[index] = timezone.Date(week)
	}
	instances, err := s.deps.Instances.FindByTenantAndDateRange(ctx, scheduleModel.Date(from), scheduleModel.Date(to))
	if err != nil {
		return nil, fmt.Errorf("load activity instances: %w", err)
	}

	visibleInstances, instanceIDs := visibleActivityInstances(instances)

	assignmentRows, err := s.deps.InstanceStaff.FindByInstanceIDs(ctx, instanceIDs)
	if err != nil {
		return nil, fmt.Errorf("load instance staff: %w", err)
	}
	roomIDs := effectiveAssignmentRoomIDs(visibleInstances, assignmentRows)
	rooms, err := s.deps.Rooms.FindByIDs(ctx, roomIDs)
	if err != nil {
		return nil, fmt.Errorf("load assignment rooms: %w", err)
	}
	staff, err := s.deps.Staff.ListAllWithPerson(ctx)
	if err != nil {
		return nil, fmt.Errorf("load staff directory: %w", err)
	}

	var workSchedules []*configModel.StaffWorkSchedule
	if s.deps.WorkSchedules != nil && len(staff) > 0 && len(usedWeeks) > 0 {
		staffIDs := make([]int64, 0, len(staff))
		for _, member := range staff {
			if member != nil {
				staffIDs = append(staffIDs, member.ID)
			}
		}
		workSchedules, err = s.deps.WorkSchedules.FindByStaffIDsValidInRange(ctx, staffIDs, workforceDate(firstWeekFrom), workforceDate(lastWeekTo))
		if err != nil {
			return nil, fmt.Errorf("load staff work schedules: %w", err)
		}
	}

	return &staffScheduleOverviewData{
		shifts:           shifts,
		weekShifts:       weekShifts,
		usedWeeks:        usedWeeks,
		visibleInstances: visibleInstances,
		assignmentRows:   assignmentRows,
		rooms:            rooms,
		staff:            staff,
		workSchedules:    workSchedules,
	}, nil
}

func visibleActivityInstances(instances []*scheduleModel.ActivityInstance) ([]*scheduleModel.ActivityInstance, []int64) {
	visible := make([]*scheduleModel.ActivityInstance, 0, len(instances))
	ids := make([]int64, 0, len(instances))
	for _, instance := range instances {
		if instance == nil || instance.Status == scheduleModel.InstanceStatusCancelled {
			continue
		}
		visible = append(visible, instance)
		ids = append(ids, instance.ID)
	}
	return visible, ids
}

func indexRoomNames(rooms []*facilitiesModel.Room) map[int64]string {
	roomNames := make(map[int64]string, len(rooms))
	for _, room := range rooms {
		if room != nil {
			roomNames[room.ID] = room.Name
		}
	}
	return roomNames
}

func indexAssignmentRows(rows []*scheduleModel.InstanceStaff) map[int64][]*scheduleModel.InstanceStaff {
	rowsByInstance := make(map[int64][]*scheduleModel.InstanceStaff)
	for _, row := range rows {
		if row != nil {
			rowsByInstance[row.InstanceID] = append(rowsByInstance[row.InstanceID], row)
		}
	}
	return rowsByInstance
}

func buildStaffScheduleAssignments(
	visibleInstances []*scheduleModel.ActivityInstance,
	rowsByInstance map[int64][]*scheduleModel.InstanceStaff,
	roomNames map[int64]string,
	shiftIndex map[staffDateKey][]*scheduleModel.StaffShift,
	usedWeeks map[timezone.Date]bool,
) []StaffScheduleAssignment {
	assignments := make([]StaffScheduleAssignment, 0)
	for _, instance := range visibleInstances {
		for _, row := range rowsByInstance[instance.ID] {
			assignment := newStaffScheduleAssignment(instance, row, roomNames)
			instanceDate := timezone.Date(instance.Date)
			weekFrom, _ := containingCalendarWeek(instanceDate)
			applyCoverage(&assignment, usedWeeks[weekFrom], shiftIndex[staffDateKey{StaffID: row.StaffID, Date: instanceDate}])
			assignments = append(assignments, assignment)
		}
	}
	return assignments
}

func newStaffScheduleAssignment(
	instance *scheduleModel.ActivityInstance,
	row *scheduleModel.InstanceStaff,
	roomNames map[int64]string,
) StaffScheduleAssignment {
	roomID := instance.RoomID
	if row.RoomID != nil {
		roomID = *row.RoomID
	}
	return StaffScheduleAssignment{
		InstanceID:         instance.ID,
		StaffID:            row.StaffID,
		Date:               timezone.Date(instance.Date),
		StartTime:          timezone.NormalizeWallClock(instance.StartTime),
		EndTime:            timezone.NormalizeWallClock(instance.EndTime),
		ActivityTitle:      instance.Title,
		ActivityGroupID:    instance.ActivityGroupID,
		RoomID:             roomID,
		RoomName:           roomNames[roomID],
		Status:             instance.Status,
		IsAbsent:           row.IsAbsent,
		IsSubstitute:       row.IsSubstitute,
		AbsenceReason:      row.AbsenceReason,
		UncoveredIntervals: make([]timetableplanning.ShiftCoverageInterval, 0),
	}
}

func effectiveAssignmentRoomIDs(instances []*scheduleModel.ActivityInstance, rows []*scheduleModel.InstanceStaff) []int64 {
	roomByInstance := make(map[int64]int64, len(instances))
	for _, instance := range instances {
		roomByInstance[instance.ID] = instance.RoomID
	}
	seen := make(map[int64]struct{})
	roomIDs := make([]int64, 0)
	for _, row := range rows {
		if row == nil {
			continue
		}
		roomID := roomByInstance[row.InstanceID]
		if row.RoomID != nil {
			roomID = *row.RoomID
		}
		if roomID <= 0 {
			continue
		}
		if _, exists := seen[roomID]; exists {
			continue
		}
		seen[roomID] = struct{}{}
		roomIDs = append(roomIDs, roomID)
	}
	return roomIDs
}

// shiftsWithinRange filters the full-week shift load back down to the
// requested viewport for the grid payload and assignment coverage.
func shiftsWithinRange(shifts []*scheduleModel.StaffShift, from, to timezone.Date) []*scheduleModel.StaffShift {
	filtered := make([]*scheduleModel.StaffShift, 0, len(shifts))
	for _, shift := range shifts {
		if shift == nil || shift.Date.Before(from) || shift.Date.After(to) {
			continue
		}
		filtered = append(filtered, shift)
	}
	return filtered
}

func applyCoverage(assignment *StaffScheduleAssignment, dienstplanInUse bool, shifts []*scheduleModel.StaffShift) {
	if assignment.IsAbsent {
		assignment.CoverageStatus = CoverageStatusNotApplicable
		assignment.CoverageReason = stringPointer(CoverageReasonAbsent)
		return
	}
	if !dienstplanInUse {
		assignment.CoverageStatus = CoverageStatusNotApplicable
		assignment.CoverageReason = stringPointer(CoverageReasonDienstplanNotUsed)
		return
	}
	assignment.UncoveredIntervals = uncoveredShiftIntervals(assignment.StartTime, assignment.EndTime, shifts)
	if len(assignment.UncoveredIntervals) == 0 {
		assignment.CoverageStatus = CoverageStatusCovered
		return
	}
	assignment.CoverageStatus = CoverageStatusUncovered
}

func stringPointer(value string) *string {
	return &value
}

// buildWeeklySummaries derives per-staff weekly planned minutes from the
// full-week shift load and pairs them with the contractual weekly target.
// Planned minutes intentionally use weekShifts (full Monday–Sunday weeks), not
// the viewport subset — a Mon–Fri request must still count weekend shifts.
// A summary row exists only for calendar weeks that have at least one tenant
// shift, and only for staff with either shift minutes or a resolved target in
// that week (a contracted person without shifts surfaces as underplanned).
func (s *staffScheduleOverviewService) buildWeeklySummaries(ctx context.Context, data *staffScheduleOverviewData) ([]StaffWeeklySummary, error) {
	summaries := make([]StaffWeeklySummary, 0)
	weekStarts := sortedCalendarWeeks(data.usedWeeks)
	if len(weekStarts) == 0 || len(data.staff) == 0 {
		return summaries, nil
	}

	planned := plannedShiftMinutes(data.weekShifts)
	targets, err := s.resolveWeeklyTargets(ctx, data.staff, data.workSchedules, weekStarts)
	if err != nil {
		return nil, err
	}

	for _, member := range data.staff {
		if member == nil {
			continue
		}
		for _, weekStart := range weekStarts {
			key := staffDateKey{StaffID: member.ID, Date: weekStart}
			plannedMinutes, hasShifts := planned[key]
			target, hasTarget := targets[key]
			if !hasShifts && !hasTarget {
				continue
			}
			summary := StaffWeeklySummary{
				StaffID:        member.ID,
				WeekStart:      weekStart,
				PlannedMinutes: plannedMinutes,
			}
			if hasTarget {
				targetMinutes := target
				delta := plannedMinutes - targetMinutes
				summary.TargetMinutes = &targetMinutes
				summary.DeltaMinutes = &delta
			}
			summaries = append(summaries, summary)
		}
	}
	return summaries, nil
}

// resolveWeeklyTargets mirrors the time-tracking Soll resolution: date-valid
// staff_work_schedules rows win per staff; the assigned work-time model is
// the fallback only when the schedule yields no target for any week at all.
func (s *staffScheduleOverviewService) resolveWeeklyTargets(
	ctx context.Context,
	staff []*usersModel.Staff,
	schedules []*configModel.StaffWorkSchedule,
	weekStarts []timezone.Date,
) (map[staffDateKey]int, error) {
	targets := make(map[staffDateKey]int)

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

	residual := make([]*usersModel.Staff, 0)
	modelIDs := make([]int64, 0)
	seenModels := make(map[int64]struct{})
	for _, member := range staff {
		if member == nil {
			continue
		}
		found := false
		if entries := entriesByStaff[member.ID]; len(entries) > 0 {
			for _, weekStart := range weekStarts {
				if target, ok := configModel.WeeklyTargetFromSchedule(entries, workforceDatePointer(member.RotationAnchorDate), workforceDate(weekStart)); ok {
					for offset := 0; offset < 7; offset++ {
						day := weekStart.AddDays(offset)
						if holidaySet[day] {
							dayTarget, _ := configModel.DailyTargetFromSchedule(entries, workforceDatePointer(member.RotationAnchorDate), workforceDate(day))
							target -= dayTarget
						}
					}
					targets[staffDateKey{StaffID: member.ID, Date: weekStart}] = target
					found = true
				}
			}
		}
		if found || member.WorkTimeModelID == nil {
			continue
		}
		residual = append(residual, member)
		if _, exists := seenModels[*member.WorkTimeModelID]; !exists {
			seenModels[*member.WorkTimeModelID] = struct{}{}
			modelIDs = append(modelIDs, *member.WorkTimeModelID)
		}
	}

	if len(residual) == 0 || s.deps.WorkModels == nil {
		return targets, nil
	}
	models, err := s.deps.WorkModels.FindByIDs(ctx, modelIDs)
	if err != nil {
		return nil, fmt.Errorf("load work-time models: %w", err)
	}
	modelsByID := make(map[int64]*configModel.WorkTimeModel, len(models))
	for _, model := range models {
		if model != nil {
			modelsByID[model.ID] = model
		}
	}
	for _, member := range residual {
		model := modelsByID[*member.WorkTimeModelID]
		if model == nil {
			continue
		}
		anchor := model.RotationAnchorDate
		if member.RotationAnchorDate != nil {
			anchor = workforceDate(*member.RotationAnchorDate)
		}
		for weekStart, target := range configModel.WeeklyTargetsFromModel(model, anchor, workforceDates(weekStarts)) {
			for offset := 0; offset < 7; offset++ {
				day := calendarDate(weekStart.AddDays(offset))
				if holidaySet[day] {
					dayTarget, _ := configModel.DailyTargetFromModel(model, anchor, workforceDate(day))
					target -= dayTarget
				}
			}
			targets[staffDateKey{StaffID: member.ID, Date: calendarDate(weekStart)}] = target
		}
	}
	return targets, nil
}

// holidayDatesForWeeks loads the public holidays covering the given weeks.
// A resolver error fails the overview read — a silently unreduced Soll
// would fake minus hours in holiday weeks.
func (s *staffScheduleOverviewService) holidayDatesForWeeks(ctx context.Context, weekStarts []timezone.Date) (map[timezone.Date]bool, error) {
	if s.deps.Holidays == nil || len(weekStarts) == 0 {
		return nil, nil
	}
	from, to := weekStarts[0], weekStarts[0]
	for _, weekStart := range weekStarts[1:] {
		if weekStart.Before(from) {
			from = weekStart
		}
		if weekStart.After(to) {
			to = weekStart
		}
	}
	set, err := s.deps.Holidays.HolidayDates(ctx, from, to.AddDays(6))
	if err != nil {
		return nil, fmt.Errorf("load public holidays: %w", err)
	}
	return set, nil
}

func plannedShiftMinutes(shifts []*scheduleModel.StaffShift) map[staffDateKey]int {
	planned := make(map[staffDateKey]int)
	for _, shift := range shifts {
		if shift == nil || shift.Cancelled {
			// A cancelled shift does not take place (#1841): it contributes no
			// planned minutes, so the weekly delta reflects the real gap and a
			// replacement's minutes land on the covering person instead.
			continue
		}
		weekFrom, _ := containingCalendarWeek(timezone.Date(shift.Date))
		planned[staffDateKey{StaffID: shift.StaffID, Date: weekFrom}] += staffShiftNetMinutes(shift)
	}
	return planned
}

// staffShiftNetMinutes is the planned working time of one shift: wall-clock
// span minus the break duration (validation caps the break at the span, but
// legacy rows are clamped defensively).
func staffShiftNetMinutes(shift *scheduleModel.StaffShift) int {
	start := timezone.NormalizeWallClock(shift.StartTime)
	end := timezone.NormalizeWallClock(shift.EndTime)
	minutes := int(end.Sub(start)/time.Minute) - shift.BreakMinutes
	if minutes < 0 {
		return 0
	}
	return minutes
}

func sortedCalendarWeeks(weeks []timezone.Date) []timezone.Date {
	sorted := make([]timezone.Date, 0, len(weeks))
	for _, week := range weeks {
		if !week.IsZero() {
			sorted = append(sorted, week)
		}
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Before(sorted[j]) })
	return sorted
}
