package schedule

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	facilitiesModel "github.com/moto-nrw/project-phoenix/models/facilities"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModel "github.com/moto-nrw/project-phoenix/models/users"
)

const (
	CoverageStatusCovered       = "covered"
	CoverageStatusUncovered     = "uncovered"
	CoverageStatusNotApplicable = "not_applicable"

	CoverageReasonAbsent            = "absent"
	CoverageReasonDienstplanNotUsed = "dienstplan_not_used"
)

// ShiftCoverageInterval is one uncovered part of an assignment's wall-clock
// window. StartTime and EndTime are normalized through timezone.WallClock.
type ShiftCoverageInterval struct {
	StartTime time.Time
	EndTime   time.Time
}

// StaffScheduleAssignment is the read-only bridge between one concrete
// activity-instance assignment and the staff member's shifts on that day.
type StaffScheduleAssignment struct {
	InstanceID         int64
	StaffID            int64
	Date               timezone.Date
	StartTime          time.Time
	EndTime            time.Time
	ActivityTitle      string
	RoomID             int64
	RoomName           string
	Status             string
	IsAbsent           bool
	IsSubstitute       bool
	AbsenceReason      *string
	CoverageStatus     string
	CoverageReason     *string
	UncoveredIntervals []ShiftCoverageInterval
}

// StaffScheduleOverview is the batched weekly projection consumed by the
// Dienstplan. It is deliberately read-only: no assignment is linked to or
// generated from a StaffShift row.
type StaffScheduleOverview struct {
	From            timezone.Date
	To              timezone.Date
	DienstplanInUse bool
	Staff           []*usersModel.Staff
	Shifts          []*scheduleModel.StaffShift
	Assignments     []StaffScheduleAssignment
}

// Narrow read interfaces keep this projection independently testable while
// the production repositories continue to implement the wider domain
// interfaces used by mutation services.
type StaffShiftRangeReader interface {
	FindByDateRange(ctx context.Context, start, end timezone.Date) ([]*scheduleModel.StaffShift, error)
}

type ActivityInstanceRangeReader interface {
	FindByTenantAndDateRange(ctx context.Context, from, to timezone.Date) ([]*scheduleModel.ActivityInstance, error)
}

type InstanceStaffBatchReader interface {
	FindByInstanceIDs(ctx context.Context, instanceIDs []int64) ([]*scheduleModel.InstanceStaff, error)
}

type RoomBatchReader interface {
	FindByIDs(ctx context.Context, ids []int64) ([]*facilitiesModel.Room, error)
}

type StaffOverviewReader interface {
	ListAllWithPerson(ctx context.Context) ([]*usersModel.Staff, error)
	FindWithPersonByIDs(ctx context.Context, ids []int64) (map[int64]*usersModel.Staff, error)
}

type StaffScheduleOverviewDependencies struct {
	Shifts        StaffShiftRangeReader
	Instances     ActivityInstanceRangeReader
	InstanceStaff InstanceStaffBatchReader
	Rooms         RoomBatchReader
	Staff         StaffOverviewReader
}

type StaffScheduleOverviewGetter interface {
	GetOverview(ctx context.Context, from, to timezone.Date) (*StaffScheduleOverview, error)
}

type staffScheduleOverviewService struct {
	deps StaffScheduleOverviewDependencies
}

func NewStaffScheduleOverviewService(deps StaffScheduleOverviewDependencies) StaffScheduleOverviewGetter {
	return &staffScheduleOverviewService{deps: deps}
}

type staffDateKey struct {
	staffID int64
	date    timezone.Date
}

type staffScheduleOverviewData struct {
	shifts           []*scheduleModel.StaffShift
	visibleInstances []*scheduleModel.ActivityInstance
	assignmentRows   []*scheduleModel.InstanceStaff
	rooms            []*facilitiesModel.Room
	staff            []*usersModel.Staff
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

	dienstplanInUse := len(data.shifts) > 0
	assignments := buildStaffScheduleAssignments(
		data.visibleInstances,
		indexAssignmentRows(data.assignmentRows),
		indexRoomNames(data.rooms),
		indexShifts(data.shifts),
		dienstplanInUse,
	)

	return &StaffScheduleOverview{
		From:            from,
		To:              to,
		DienstplanInUse: dienstplanInUse,
		Staff:           data.staff,
		Shifts:          data.shifts,
		Assignments:     assignments,
	}, nil
}

func (s *staffScheduleOverviewService) loadOverviewData(ctx context.Context, from, to timezone.Date) (*staffScheduleOverviewData, error) {
	shifts, err := s.deps.Shifts.FindByDateRange(ctx, from, to)
	if err != nil {
		return nil, fmt.Errorf("load staff shifts: %w", err)
	}
	instances, err := s.deps.Instances.FindByTenantAndDateRange(ctx, from, to)
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

	return &staffScheduleOverviewData{
		shifts:           shifts,
		visibleInstances: visibleInstances,
		assignmentRows:   assignmentRows,
		rooms:            rooms,
		staff:            staff,
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

func sortOverviewStaff(staff []*usersModel.Staff) {
	sort.Slice(staff, func(i, j int) bool {
		left := staffSortName(staff[i])
		right := staffSortName(staff[j])
		if left == right {
			return staffID(staff[i]) < staffID(staff[j])
		}
		return left < right
	})
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
	dienstplanInUse bool,
) []StaffScheduleAssignment {
	assignments := make([]StaffScheduleAssignment, 0)
	for _, instance := range visibleInstances {
		for _, row := range rowsByInstance[instance.ID] {
			assignment := newStaffScheduleAssignment(instance, row, roomNames)
			applyCoverage(&assignment, dienstplanInUse, shiftIndex[staffDateKey{row.StaffID, instance.Date}])
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
		Date:               instance.Date,
		StartTime:          timezone.WallClock(instance.StartTime),
		EndTime:            timezone.WallClock(instance.EndTime),
		ActivityTitle:      instance.Title,
		RoomID:             roomID,
		RoomName:           roomNames[roomID],
		Status:             instance.Status,
		IsAbsent:           row.IsAbsent,
		IsSubstitute:       row.IsSubstitute,
		AbsenceReason:      row.AbsenceReason,
		UncoveredIntervals: make([]ShiftCoverageInterval, 0),
	}
}

func staffID(staff *usersModel.Staff) int64 {
	if staff == nil {
		return 0
	}
	return staff.ID
}

func staffSortName(staff *usersModel.Staff) string {
	if staff == nil || staff.Person == nil {
		return ""
	}
	return strings.ToLower(staff.Person.LastName + "\x00" + staff.Person.FirstName)
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

func indexShifts(shifts []*scheduleModel.StaffShift) map[staffDateKey][]*scheduleModel.StaffShift {
	index := make(map[staffDateKey][]*scheduleModel.StaffShift)
	for _, shift := range shifts {
		if shift != nil {
			key := staffDateKey{shift.StaffID, shift.Date}
			index[key] = append(index[key], shift)
		}
	}
	return index
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

type shiftCoverageWindow struct {
	start time.Time
	end   time.Time
}

// uncoveredShiftIntervals returns the exact gaps in [start, end) after
// clipping, sorting, and unioning all shift windows. Overlapping and directly
// touching shifts form one continuous interval. BreakMinutes intentionally do
// not create gaps because the model does not locate a break within the shift.
func uncoveredShiftIntervals(start, end time.Time, shifts []*scheduleModel.StaffShift) []ShiftCoverageInterval {
	start = timezone.WallClock(start)
	end = timezone.WallClock(end)
	if !end.After(start) {
		return []ShiftCoverageInterval{}
	}

	covered := coveredShiftWindows(start, end, shifts)
	return gapsBetweenShiftWindows(start, end, covered)
}

func coveredShiftWindows(start, end time.Time, shifts []*scheduleModel.StaffShift) []shiftCoverageWindow {
	covered := make([]shiftCoverageWindow, 0, len(shifts))
	for _, shift := range shifts {
		if window, ok := clipShiftCoverageWindow(start, end, shift); ok {
			covered = append(covered, window)
		}
	}
	sort.Slice(covered, func(i, j int) bool {
		if covered[i].start.Equal(covered[j].start) {
			return covered[i].end.Before(covered[j].end)
		}
		return covered[i].start.Before(covered[j].start)
	})
	return covered
}

func clipShiftCoverageWindow(start, end time.Time, shift *scheduleModel.StaffShift) (shiftCoverageWindow, bool) {
	if shift == nil {
		return shiftCoverageWindow{}, false
	}
	shiftStart := timezone.WallClock(shift.StartTime)
	shiftEnd := timezone.WallClock(shift.EndTime)
	if !shiftEnd.After(start) || !end.After(shiftStart) {
		return shiftCoverageWindow{}, false
	}
	if shiftStart.Before(start) {
		shiftStart = start
	}
	if shiftEnd.After(end) {
		shiftEnd = end
	}
	return shiftCoverageWindow{start: shiftStart, end: shiftEnd}, true
}

func gapsBetweenShiftWindows(start, end time.Time, covered []shiftCoverageWindow) []ShiftCoverageInterval {
	gaps := make([]ShiftCoverageInterval, 0)
	cursor := start
	for _, current := range covered {
		if current.end.Before(cursor) || current.end.Equal(cursor) {
			continue
		}
		if current.start.After(cursor) {
			gaps = append(gaps, ShiftCoverageInterval{StartTime: cursor, EndTime: current.start})
		}
		if current.end.After(cursor) {
			cursor = current.end
		}
		if !cursor.Before(end) {
			return gaps
		}
	}
	if cursor.Before(end) {
		gaps = append(gaps, ShiftCoverageInterval{StartTime: cursor, EndTime: end})
	}
	return gaps
}
