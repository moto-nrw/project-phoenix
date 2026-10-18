package application

import (
	"context"
	"fmt"

	appointmentcap "github.com/moto-nrw/project-phoenix/modules/appointments"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal/internal/ports"
)

// staffTimetableAssignment pairs a timetable instance with the staff row that
// put this staff member on it — the row carries the per-person room override.
type staffTimetableAssignment struct {
	instance   *ports.ActivityInstance
	assignment *ports.InstanceStaff
}

// timetableRoomID resolves the room the staff member is actually in: the
// per-person override on instance_staff wins over the instance's own room
// (the Lernzeit split pattern), matching the Dienstplan overview in
// services/schedule/staff_schedule_overview.go.
func timetableRoomID(instance *ports.ActivityInstance, assignment *ports.InstanceStaff) int64 {
	if assignment != nil && assignment.RoomID != nil {
		return *assignment.RoomID
	}
	if instance == nil {
		return 0
	}
	return instance.RoomID
}

func distinctTimetableRoomIDs(assigned []staffTimetableAssignment) []int64 {
	seen := make(map[int64]struct{}, len(assigned))
	roomIDs := make([]int64, 0, len(assigned))
	for _, entry := range assigned {
		roomID := timetableRoomID(entry.instance, entry.assignment)
		if roomID <= 0 {
			continue
		}
		if _, ok := seen[roomID]; ok {
			continue
		}
		seen[roomID] = struct{}{}
		roomIDs = append(roomIDs, roomID)
	}
	return roomIDs
}

// timetableRoomNames resolves every room in the window with one query so the
// room name does not cost one read per event (#2078). An unwired room
// repository yields an empty map: events keep an empty Location instead of
// failing the calendar, mirroring the ShiftTypeRepo guard in staffShiftEvents.
func (s *service) timetableRoomNames(ctx context.Context, roomIDs []int64) (map[int64]string, error) {
	names := make(map[int64]string, len(roomIDs))
	if s.cfg.RoomRepo == nil || len(roomIDs) == 0 {
		return names, nil
	}
	rooms, err := s.cfg.RoomRepo.FindByIDs(ctx, roomIDs)
	if err != nil {
		return nil, err
	}
	for _, room := range rooms {
		if room != nil {
			names[room.ID] = room.Name
		}
	}
	return names, nil
}

// collectStaffTimetableAssignments resolves the staff member's assignments and
// their instances in two range queries. It keeps the first assignment per
// instance. Interactive reads drop cancelled instances; subscription feeds
// retain them so external calendars receive STATUS:CANCELLED.
func (s *service) collectStaffTimetableAssignments(ctx context.Context, staffID int64, from, to appointmentcap.Date, includeCancelled bool) ([]staffTimetableAssignment, error) {
	assignments, err := s.cfg.InstanceStaffRepo.FindByStaffAndDateRange(ctx, staffID, appointmentcap.Date(from), appointmentcap.Date(to))
	if err != nil {
		return nil, err
	}
	instanceIDs := make([]int64, 0, len(assignments))
	seen := make(map[int64]struct{})
	for _, assignment := range assignments {
		if _, ok := seen[assignment.InstanceID]; ok {
			continue
		}
		seen[assignment.InstanceID] = struct{}{}
		instanceIDs = append(instanceIDs, assignment.InstanceID)
	}
	instances, err := s.cfg.ActivityInstanceRepo.FindByIDs(ctx, instanceIDs)
	if err != nil {
		return nil, err
	}
	instancesByID := make(map[int64]*ports.ActivityInstance, len(instances))
	for _, instance := range instances {
		instancesByID[instance.ID] = instance
	}

	collected := make([]staffTimetableAssignment, 0, len(instanceIDs))
	seen = make(map[int64]struct{})
	for _, assignment := range assignments {
		if _, ok := seen[assignment.InstanceID]; ok {
			continue
		}
		seen[assignment.InstanceID] = struct{}{}
		instance := instancesByID[assignment.InstanceID]
		if instance == nil || (!includeCancelled && instance.Status == "cancelled") {
			continue
		}
		collected = append(collected, staffTimetableAssignment{instance: instance, assignment: assignment})
	}
	return collected, nil
}

func (s *service) staffTimetableEvents(ctx context.Context, staffID int64, from, to appointmentcap.Date) ([]Event, error) {
	return s.staffTimetableEventsWithCancelled(ctx, staffID, from, to, false)
}

func (s *service) staffTimetableFeedEvents(ctx context.Context, staffID int64, from, to appointmentcap.Date) ([]Event, error) {
	return s.staffTimetableEventsWithCancelled(ctx, staffID, from, to, true)
}

func (s *service) staffTimetableEventsWithCancelled(ctx context.Context, staffID int64, from, to appointmentcap.Date, includeCancelled bool) ([]Event, error) {
	assigned, err := s.collectStaffTimetableAssignments(ctx, staffID, from, to, includeCancelled)
	if err != nil {
		return nil, err
	}
	roomNames, err := s.timetableRoomNames(ctx, distinctTimetableRoomIDs(assigned))
	if err != nil {
		return nil, err
	}
	events := make([]Event, 0, len(assigned))
	for _, entry := range assigned {
		instance := entry.instance
		id := formatID(instance.ID)
		modifiedAt := instance.UpdatedAt
		if entry.assignment.UpdatedAt.After(modifiedAt) {
			modifiedAt = entry.assignment.UpdatedAt
		}
		event := Event{
			ID:          fmt.Sprintf("timetable:%d", instance.ID),
			Source:      EventSourceTimetable,
			TimetableID: &id, ActivityType: instance.ActivityType,
			Title:       instance.Title,
			Description: instance.Description,
			StartDate:   instance.Date.String(),
			EndDate:     instance.Date.String(),
			StartTime:   formatClock(instance.StartTime),
			EndTime:     formatClock(instance.EndTime),
			AllDay:      false,
			Cancelled:   instance.Status == "cancelled",
			ModifiedAt:  modifiedAt,
		}
		// A room deleted between assignment and this read misses the map;
		// Location then stays nil rather than becoming an empty string.
		if name := roomNames[timetableRoomID(instance, entry.assignment)]; name != "" {
			event.Location = &name
		}
		events = append(events, event)
	}
	return events, nil
}
