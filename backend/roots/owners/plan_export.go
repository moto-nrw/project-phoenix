package owners

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/modules/planexport"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// planExportInstanceReader is the Timetable read of the blocks of a window,
// cancelled ones and the state of their sessions included.
type planExportInstanceReader interface {
	FindByTenantAndDateRange(ctx context.Context, from, to calendar.Date) ([]*timetable.ScheduledInstance, error)
}

// planExportInstanceStaffReader is the Timetable read of the staff planned
// on blocks.
type planExportInstanceStaffReader interface {
	FindByInstanceIDs(ctx context.Context, instanceIDs []int64) ([]*timetable.InstanceStaff, error)
}

// planExportActivityGroupReader is the Timetable read of the Angebote blocks were
// materialized from.
type planExportActivityGroupReader interface {
	FindByIDs(ctx context.Context, ids []int64) ([]*timetable.Group, error)
}

// planExportOverviewReader is the Workforce read of the staff week.
type planExportOverviewReader interface {
	Overview(ctx context.Context, from, to string) (workforce.StaffScheduleOverview, error)
}

// planExportShiftTypeReader is the Workforce read of the Schichtarten.
type planExportShiftTypeReader interface {
	ListShiftTypes(ctx context.Context) ([]workforce.ShiftType, error)
}

// planExportRoomReader is the Facilities read of the printed rooms.
type planExportRoomReader interface {
	ListRoomsByID(ctx context.Context, ids []int64) ([]facilities.Room, error)
}

// planExportPlanningTrackReader is the Timetable read of the Planungsspuren.
type planExportPlanningTrackReader interface {
	ListPlanningTracks(ctx context.Context, filter timetable.PlanningTrackFilter) ([]timetable.PlanningTrack, error)
}

// planExportSources are the owner reads the root binds to the plan export. Every
// source is required; the root binds all of them.
type planExportSources struct {
	Overview       planExportOverviewReader
	ShiftTypes     planExportShiftTypeReader
	Instances      planExportInstanceReader
	InstanceStaff  planExportInstanceStaffReader
	Students       planexport.InstanceStudentCountReader
	Rooms          planExportRoomReader
	Staff          planexport.StaffNameReader
	ActivityGroups planExportActivityGroupReader
	PlanningTracks planExportPlanningTrackReader
	ClosingDays    planexport.ClosingDayReader
	Holidays       planexport.HolidayReader
	Renderer       planexport.Renderer
	Logger         *slog.Logger
}

// newPlanExport binds the owner reads to the plan export ports and returns
// the public capability.
func newPlanExport(sources planExportSources) planexport.Service {
	return planexport.NewService(planexport.Dependencies{
		Overview:       planExportOverview{source: sources.Overview},
		ShiftTypes:     planExportShiftTypes{source: sources.ShiftTypes},
		Instances:      planExportInstances{source: sources.Instances},
		InstanceStaff:  planExportInstanceStaff{source: sources.InstanceStaff},
		Students:       sources.Students,
		Rooms:          planExportRooms{source: sources.Rooms},
		Staff:          sources.Staff,
		ActivityGroups: planExportActivityGroups{source: sources.ActivityGroups},
		PlanningTracks: planExportPlanningTracks{source: sources.PlanningTracks},
		ClosingDays:    sources.ClosingDays,
		Holidays:       sources.Holidays,
		Renderer:       sources.Renderer,
	}, sources.Logger)
}

// planExportRange resolves the port's day strings. The capability only ever
// hands over days it validated itself, so a failure here is a programming
// error and is reported rather than widened to a zero date.
func planExportRange(from, to planexport.Date) (calendar.Date, calendar.Date, error) {
	start, err := calendar.ParseDate(string(from))
	if err != nil {
		return "", "", fmt.Errorf("plan export range start: %w", err)
	}
	end, err := calendar.ParseDate(string(to))
	if err != nil {
		return "", "", fmt.Errorf("plan export range end: %w", err)
	}
	return start, end, nil
}

type planExportOverview struct {
	source planExportOverviewReader
}

func (a planExportOverview) StaffScheduleOverview(ctx context.Context, from, to planexport.Date) (*planexport.StaffScheduleOverview, error) {
	start, end, err := planExportRange(from, to)
	if err != nil {
		return nil, err
	}
	overview, err := a.source.Overview(ctx, start.String(), end.String())
	if err != nil {
		return nil, err
	}
	if len(overview.Staff) == 0 && len(overview.Shifts) == 0 && len(overview.Assignments) == 0 {
		return &planexport.StaffScheduleOverview{}, nil
	}
	out := &planexport.StaffScheduleOverview{
		Staff:       make([]*planexport.StaffMember, 0, len(overview.Staff)),
		Shifts:      make([]*planexport.Shift, 0, len(overview.Shifts)),
		Assignments: make([]planexport.Assignment, 0, len(overview.Assignments)),
	}
	for _, member := range overview.Staff {
		out.Staff = append(out.Staff, &planexport.StaffMember{ID: member.ID, FirstName: member.FirstName, LastName: member.LastName})
	}
	for _, shift := range overview.Shifts {
		out.Shifts = append(out.Shifts, &planexport.Shift{
			ID:            shift.ID,
			StaffID:       shift.StaffID,
			Date:          planexport.Date(shift.Date),
			StartTime:     planExportWallClock(shift.StartTime),
			EndTime:       planExportWallClock(shift.EndTime),
			ShiftTypeID:   shift.ShiftTypeID,
			OriginShiftID: shift.OriginShiftID,
			Cancelled:     shift.Cancelled,
			ChangeReason:  shift.ChangeReason,
			Notes:         shift.Notes,
		})
	}
	for _, assignment := range overview.Assignments {
		intervals := make([]planexport.Interval, 0, len(assignment.UncoveredIntervals))
		for _, gap := range assignment.UncoveredIntervals {
			intervals = append(intervals, planexport.Interval{StartTime: planExportWallClock(gap.StartTime), EndTime: planExportWallClock(gap.EndTime)})
		}
		out.Assignments = append(out.Assignments, planexport.Assignment{
			StaffID:            assignment.StaffID,
			Date:               planexport.Date(assignment.Date),
			StartTime:          planExportWallClock(assignment.StartTime),
			EndTime:            planExportWallClock(assignment.EndTime),
			ActivityTitle:      assignment.ActivityTitle,
			ActivityGroupID:    assignment.ActivityGroupID,
			RoomName:           assignment.RoomName,
			IsSubstitute:       assignment.IsSubstitute,
			IsAbsent:           assignment.IsAbsent,
			UncoveredIntervals: intervals,
		})
	}
	return out, nil
}

// planExportWallClock restores the normalized wall clock from the public
// Workforce ClockLayout string.
func planExportWallClock(value string) time.Time {
	parsed, _ := time.Parse(workforce.ClockLayout, value)
	return calendar.NormalizeWallClock(parsed)
}

type planExportShiftTypes struct {
	source planExportShiftTypeReader
}

func (a planExportShiftTypes) ListShiftTypes(ctx context.Context) ([]*planexport.ShiftType, error) {
	types, err := a.source.ListShiftTypes(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*planexport.ShiftType, 0, len(types))
	for _, shiftType := range types {
		out = append(out, &planexport.ShiftType{ID: shiftType.ID, Name: shiftType.Name, Color: shiftType.Color})
	}
	return out, nil
}

type planExportInstances struct {
	source planExportInstanceReader
}

func (a planExportInstances) InstancesInRange(ctx context.Context, from, to planexport.Date) ([]*planexport.Instance, error) {
	start, end, err := planExportRange(from, to)
	if err != nil {
		return nil, err
	}
	instances, err := a.source.FindByTenantAndDateRange(ctx, start, end)
	if err != nil {
		return nil, err
	}
	out := make([]*planexport.Instance, 0, len(instances))
	for _, instance := range instances {
		if instance == nil {
			continue
		}
		out = append(out, &planexport.Instance{
			ID:               instance.ID,
			Date:             planexport.Date(instance.Date),
			StartTime:        instance.StartTime,
			EndTime:          instance.EndTime,
			Title:            instance.Title,
			ActivityGroupID:  instance.ActivityGroupID,
			RoomID:           instance.RoomID,
			Cancelled:        instance.IsCancelled(),
			IsDuty:           instance.IsDuty,
			CancelReason:     instance.CancelReason,
			Notes:            instance.Notes,
			UnderstaffedNote: instance.UnderstaffedNote,
		})
	}
	return out, nil
}

type planExportInstanceStaff struct {
	source planExportInstanceStaffReader
}

func (a planExportInstanceStaff) InstanceStaffByInstanceIDs(ctx context.Context, instanceIDs []int64) ([]*planexport.InstanceStaff, error) {
	rows, err := a.source.FindByInstanceIDs(ctx, instanceIDs)
	if err != nil {
		return nil, err
	}
	out := make([]*planexport.InstanceStaff, 0, len(rows))
	for _, row := range rows {
		if row == nil {
			continue
		}
		out = append(out, &planexport.InstanceStaff{
			InstanceID:   row.InstanceID,
			StaffID:      row.StaffID,
			RoomID:       row.RoomID,
			IsSubstitute: row.IsSubstitute,
			IsAbsent:     row.IsAbsent,
		})
	}
	return out, nil
}

type planExportRooms struct {
	source planExportRoomReader
}

func (a planExportRooms) RoomsByIDs(ctx context.Context, ids []int64) ([]*planexport.Room, error) {
	rooms, err := a.source.ListRoomsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]*planexport.Room, 0, len(rooms))
	for _, room := range rooms {
		out = append(out, &planexport.Room{ID: room.ID, Name: room.Name})
	}
	return out, nil
}

type planExportActivityGroups struct {
	source planExportActivityGroupReader
}

func (a planExportActivityGroups) ActivityGroupsByIDs(ctx context.Context, ids []int64) ([]*planexport.ActivityGroup, error) {
	groups, err := a.source.FindByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]*planexport.ActivityGroup, 0, len(groups))
	for _, group := range groups {
		if group == nil {
			continue
		}
		out = append(out, &planexport.ActivityGroup{ID: group.ID, PlanningTrackID: group.PlanningTrackID})
	}
	return out, nil
}

type planExportPlanningTracks struct {
	source planExportPlanningTrackReader
}

func (a planExportPlanningTracks) ListPlanningTracks(ctx context.Context) ([]*planexport.PlanningTrack, error) {
	tracks, err := a.source.ListPlanningTracks(ctx, timetable.PlanningTrackFilter{Ordered: true})
	if err != nil {
		return nil, err
	}
	out := make([]*planexport.PlanningTrack, 0, len(tracks))
	for _, track := range tracks {
		out = append(out, &planexport.PlanningTrack{ID: track.ID, Color: track.Color})
	}
	return out, nil
}
