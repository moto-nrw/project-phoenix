// Package legacy adapts the retained schedule repositories and the Workforce
// Dienstplan overview to the plan export capability's consumer-owned ports
// (#2706). It exists because the materialized blocks and their staff, the
// Schichtarten, the Planungsspuren, the closing days and the holidays still
// live in the retained Timetable & Activities packages; the adapters
// translate their rows into plain records and decide nothing themselves. The
// staff schedule overview arrives through the public Workforce query
// (#3418). Delete this package with the last legacy source once each owner
// exposes the fact through its public capability.
package legacy

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	facilitiesModel "github.com/moto-nrw/project-phoenix/models/facilities"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/planexport"
	"github.com/moto-nrw/project-phoenix/modules/timetable/legacy/timetableplanning"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

// ShiftTypeSource, ActivityInstanceSource and RoomSource are the slices of the retained repositories the adapters read.
type ShiftTypeSource interface {
	ListAll(ctx context.Context) ([]*scheduleModel.ShiftType, error)
}
type ActivityInstanceSource interface {
	FindByTenantAndDateRange(ctx context.Context, from, to scheduleModel.Date) ([]*scheduleModel.ActivityInstance, error)
}
type RoomSource interface {
	FindByIDs(ctx context.Context, ids []int64) ([]*facilitiesModel.Room, error)
}

// ActivityGroupSource is the slice of the retained activity-group repository the adapter reads.
type ActivityGroupSource interface {
	FindByIDs(ctx context.Context, ids []int64) ([]*activitiesModel.Group, error)
}

// PlanningTrackSource is the slice of the retained planning-track repository the adapter reads.
type PlanningTrackSource interface {
	ListAll(ctx context.Context) ([]*scheduleModel.PlanningTrack, error)
}

// InstanceStudentCountSource is the slice of the retained instance-student repository the adapter reads.
type InstanceStudentCountSource interface {
	CountNonAbsentByInstanceIDs(ctx context.Context, instanceIDs []int64) (map[int64]int, error)
}

// Sources are the retained readers the root binds. Every optional source may
// be nil, in which case the matching port stays unbound and the capability
// prints without that detail — the same contract the retained service had.
type Sources struct {
	Overview       workforce.StaffScheduleOverviewQuery
	ShiftTypes     ShiftTypeSource
	Instances      ActivityInstanceSource
	InstanceStaff  timetableplanning.InstanceStaffBatchReader
	Students       InstanceStudentCountSource
	Rooms          RoomSource
	Staff          timetableplanning.StaffWithPersonBatchReader
	ActivityGroups ActivityGroupSource
	PlanningTracks PlanningTrackSource
	ClosingDays    planexport.ClosingDayReader
	Holidays       planexport.HolidayReader
	Renderer       planexport.Renderer
	Logger         *slog.Logger
}

// New binds the retained sources to the plan export ports and returns the
// public capability.
func New(sources Sources) planexport.Service {
	deps := planexport.Dependencies{
		Renderer: sources.Renderer,
	}
	if sources.Students != nil {
		deps.Students = studentCountAdapter{source: sources.Students}
	}
	if sources.Overview != nil {
		deps.Overview = overviewAdapter{source: sources.Overview}
	}
	if sources.ShiftTypes != nil {
		deps.ShiftTypes = shiftTypeAdapter{source: sources.ShiftTypes}
	}
	if sources.Instances != nil {
		deps.Instances = instanceAdapter{source: sources.Instances}
	}
	if sources.InstanceStaff != nil {
		deps.InstanceStaff = instanceStaffAdapter{source: sources.InstanceStaff}
	}
	if sources.Rooms != nil {
		deps.Rooms = roomAdapter{source: sources.Rooms}
	}
	if sources.Staff != nil {
		deps.Staff = staffAdapter{source: sources.Staff}
	}
	if sources.ActivityGroups != nil {
		deps.ActivityGroups = activityGroupAdapter{source: sources.ActivityGroups}
	}
	if sources.PlanningTracks != nil {
		deps.PlanningTracks = planningTrackAdapter{source: sources.PlanningTracks}
	}
	deps.ClosingDays = sources.ClosingDays
	deps.Holidays = sources.Holidays
	return planexport.NewService(deps, sources.Logger)
}

// parseRange resolves the port's day strings. The capability only ever hands
// over days it validated itself, so a failure here is a programming error
// and is reported rather than widened to a zero date.
func parseRange(from, to planexport.Date) (timezone.Date, timezone.Date, error) {
	start, err := timezone.ParseDate(string(from))
	if err != nil {
		return "", "", fmt.Errorf("plan export range start: %w", err)
	}
	end, err := timezone.ParseDate(string(to))
	if err != nil {
		return "", "", fmt.Errorf("plan export range end: %w", err)
	}
	return start, end, nil
}

type overviewAdapter struct {
	source workforce.StaffScheduleOverviewQuery
}

func (a overviewAdapter) StaffScheduleOverview(ctx context.Context, from, to planexport.Date) (*planexport.StaffScheduleOverview, error) {
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

// wallClock restores the normalized wall clock from the public ClockLayout string.
func wallClock(value string) time.Time {
	parsed, _ := time.Parse(workforce.ClockLayout, value)
	return timezone.NormalizeWallClock(parsed)
}

type shiftTypeAdapter struct {
	source ShiftTypeSource
}

func (a shiftTypeAdapter) ListShiftTypes(ctx context.Context) ([]*planexport.ShiftType, error) {
	types, err := a.source.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*planexport.ShiftType, 0, len(types))
	for _, shiftType := range types {
		if shiftType == nil {
			continue
		}
		out = append(out, &planexport.ShiftType{ID: shiftType.ID, Name: shiftType.Name, Color: shiftType.Color})
	}
	return out, nil
}

type instanceAdapter struct {
	source ActivityInstanceSource
}

func (a instanceAdapter) InstancesInRange(ctx context.Context, from, to planexport.Date) ([]*planexport.Instance, error) {
	start, end, err := parseRange(from, to)
	if err != nil {
		return nil, err
	}
	instances, err := a.source.FindByTenantAndDateRange(ctx, scheduleModel.Date(start), scheduleModel.Date(end))
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
			Cancelled:        instance.Status == scheduleModel.InstanceStatusCancelled,
			CancelReason:     instance.CancelReason,
			Notes:            instance.Notes,
			UnderstaffedNote: instance.UnderstaffedNote,
		})
	}
	return out, nil
}

// studentCountAdapter names the head-count binding so a renamed retained method fails here, not by unbinding the port.
type studentCountAdapter struct {
	source InstanceStudentCountSource
}

func (a studentCountAdapter) CountNonAbsentByInstanceIDs(ctx context.Context, instanceIDs []int64) (map[int64]int, error) {
	return a.source.CountNonAbsentByInstanceIDs(ctx, instanceIDs)
}

type instanceStaffAdapter struct {
	source timetableplanning.InstanceStaffBatchReader
}

func (a instanceStaffAdapter) InstanceStaffByInstanceIDs(ctx context.Context, instanceIDs []int64) ([]*planexport.InstanceStaff, error) {
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

type roomAdapter struct {
	source RoomSource
}

func (a roomAdapter) RoomsByIDs(ctx context.Context, ids []int64) ([]*planexport.Room, error) {
	rooms, err := a.source.FindByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]*planexport.Room, 0, len(rooms))
	for _, room := range rooms {
		if room == nil {
			continue
		}
		out = append(out, &planexport.Room{ID: room.ID, Name: room.Name})
	}
	return out, nil
}

type staffAdapter struct {
	source timetableplanning.StaffWithPersonBatchReader
}

func (a staffAdapter) StaffByIDs(ctx context.Context, ids []int64) (map[int64]*planexport.StaffMember, error) {
	members, err := a.source.FindWithPersonByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]*planexport.StaffMember, len(members))
	for id, member := range members {
		if member == nil {
			// A staff row without a record keeps its slot and prints as
			// "Unbekannt", exactly as the retained service did.
			out[id] = nil
			continue
		}
		record := &planexport.StaffMember{ID: member.ID}
		if member.Person != nil {
			record.FirstName, record.LastName = member.Person.FirstName, member.Person.LastName
		}
		out[id] = record
	}
	return out, nil
}

type activityGroupAdapter struct {
	source ActivityGroupSource
}

func (a activityGroupAdapter) ActivityGroupsByIDs(ctx context.Context, ids []int64) ([]*planexport.ActivityGroup, error) {
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

type planningTrackAdapter struct {
	source PlanningTrackSource
}

func (a planningTrackAdapter) ListPlanningTracks(ctx context.Context) ([]*planexport.PlanningTrack, error) {
	tracks, err := a.source.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*planexport.PlanningTrack, 0, len(tracks))
	for _, track := range tracks {
		if track == nil {
			continue
		}
		out = append(out, &planexport.PlanningTrack{ID: track.ID, Color: track.Color})
	}
	return out, nil
}
