// Package compose binds the plan export capability's consumer-owned ports to
// reads in the owners' public vocabulary (#2706) and only translates their
// records: Workforce for the staff week and the Schichtarten, Timetable &
// Activities for the blocks, their staff, head counts, Angebote and
// Planungsspuren, Facilities for the room names. The root supplies those
// reads (the Timetable block, staff, Angebot and head-count reads still run on
// its retained repositories, as for the Workforce Dienstplan), plus the staff
// names, closing days and holidays it already binds for other consumers.
package compose

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

// InstanceReader is the Timetable read of the blocks of a window,
// cancelled ones and the state of their sessions included.
type InstanceReader interface {
	FindByTenantAndDateRange(ctx context.Context, from, to calendar.Date) ([]*timetable.ScheduledInstance, error)
}

// InstanceStaffReader is the Timetable read of the staff planned
// on blocks.
type InstanceStaffReader interface {
	FindByInstanceIDs(ctx context.Context, instanceIDs []int64) ([]*timetable.InstanceStaff, error)
}

// ActivityGroupReader is the Timetable read of the Angebote blocks were
// materialized from.
type ActivityGroupReader interface {
	FindByIDs(ctx context.Context, ids []int64) ([]*timetable.Group, error)
}

// ShiftTypeReader is the Workforce read of the Schichtarten.
type ShiftTypeReader interface {
	ListShiftTypes(ctx context.Context) ([]workforce.ShiftType, error)
}

// RoomReader is the Facilities read of the printed rooms.
type RoomReader interface {
	ListRoomsByID(ctx context.Context, ids []int64) ([]facilities.Room, error)
}

// PlanningTrackReader is the Timetable read of the Planungsspuren.
type PlanningTrackReader interface {
	ListPlanningTracks(ctx context.Context, filter timetable.PlanningTrackFilter) ([]timetable.PlanningTrack, error)
}

// Sources are the owner reads the root binds to the plan export. Every
// source is required; the root binds all of them.
type Sources struct {
	Overview       workforce.StaffScheduleOverviewQuery
	ShiftTypes     ShiftTypeReader
	Instances      InstanceReader
	InstanceStaff  InstanceStaffReader
	Students       planexport.InstanceStudentCountReader
	Rooms          RoomReader
	Staff          planexport.StaffNameReader
	ActivityGroups ActivityGroupReader
	PlanningTracks PlanningTrackReader
	ClosingDays    planexport.ClosingDayReader
	Holidays       planexport.HolidayReader
	Renderer       planexport.Renderer
	Logger         *slog.Logger
}

// New binds the owner reads to the plan export ports and returns
// the public capability.
func New(sources Sources) planexport.Service {
	return planexport.NewService(planexport.Dependencies{
		Overview:       overviewBinding{source: sources.Overview},
		ShiftTypes:     shiftTypeBinding{source: sources.ShiftTypes},
		Instances:      instanceBinding{source: sources.Instances},
		InstanceStaff:  instanceStaffBinding{source: sources.InstanceStaff},
		Students:       sources.Students,
		Rooms:          roomBinding{source: sources.Rooms},
		Staff:          sources.Staff,
		ActivityGroups: activityGroupBinding{source: sources.ActivityGroups},
		PlanningTracks: planningTrackBinding{source: sources.PlanningTracks},
		ClosingDays:    sources.ClosingDays,
		Holidays:       sources.Holidays,
		Renderer:       sources.Renderer,
	}, sources.Logger)
}

// parseRange resolves the port's day strings. The capability only ever
// hands over days it validated itself, so a failure here is a programming
// error and is reported rather than widened to a zero date.
func parseRange(from, to planexport.Date) (calendar.Date, calendar.Date, error) {
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

type overviewBinding struct {
	source workforce.StaffScheduleOverviewQuery
}

func (a overviewBinding) StaffScheduleOverview(ctx context.Context, from, to planexport.Date) (*planexport.StaffScheduleOverview, error) {
	start, end, err := parseRange(from, to)
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
			StartTime:     wallClock(shift.StartTime),
			EndTime:       wallClock(shift.EndTime),
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
			intervals = append(intervals, planexport.Interval{StartTime: wallClock(gap.StartTime), EndTime: wallClock(gap.EndTime)})
		}
		out.Assignments = append(out.Assignments, planexport.Assignment{
			StaffID:            assignment.StaffID,
			Date:               planexport.Date(assignment.Date),
			StartTime:          wallClock(assignment.StartTime),
			EndTime:            wallClock(assignment.EndTime),
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

// wallClock restores the normalized wall clock from the public
// Workforce ClockLayout string.
func wallClock(value string) time.Time {
	parsed, _ := time.Parse(workforce.ClockLayout, value)
	return calendar.NormalizeWallClock(parsed)
}

type shiftTypeBinding struct {
	source ShiftTypeReader
}

func (a shiftTypeBinding) ListShiftTypes(ctx context.Context) ([]*planexport.ShiftType, error) {
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

type instanceBinding struct {
	source InstanceReader
}

func (a instanceBinding) InstancesInRange(ctx context.Context, from, to planexport.Date) ([]*planexport.Instance, error) {
	start, end, err := parseRange(from, to)
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
			Cancelled:        instance.Status == timetable.InstanceStatusCancelled,
			IsDuty:           instance.IsDuty,
			CancelReason:     instance.CancelReason,
			Notes:            instance.Notes,
			UnderstaffedNote: instance.UnderstaffedNote,
		})
	}
	return out, nil
}

type instanceStaffBinding struct {
	source InstanceStaffReader
}

func (a instanceStaffBinding) InstanceStaffByInstanceIDs(ctx context.Context, instanceIDs []int64) ([]*planexport.InstanceStaff, error) {
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

type roomBinding struct {
	source RoomReader
}

func (a roomBinding) RoomsByIDs(ctx context.Context, ids []int64) ([]*planexport.Room, error) {
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

type activityGroupBinding struct {
	source ActivityGroupReader
}

func (a activityGroupBinding) ActivityGroupsByIDs(ctx context.Context, ids []int64) ([]*planexport.ActivityGroup, error) {
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

type planningTrackBinding struct {
	source PlanningTrackReader
}

func (a planningTrackBinding) ListPlanningTracks(ctx context.Context) ([]*planexport.PlanningTrack, error) {
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
