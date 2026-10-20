package application

import (
	"context"
	"fmt"

	appointmentcap "github.com/moto-nrw/project-phoenix/modules/appointments"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal/internal/ports"
)

// shiftsReferenceTypes reports whether any shift carries a ShiftTypeID —
// the guard that keeps windows without typed shifts free of the ListAll query.
func shiftsReferenceTypes(shifts []*ports.StaffShift) bool {
	for _, shift := range shifts {
		if shift.ShiftTypeID != nil {
			return true
		}
	}
	return false
}

// staffShiftEvents maps the staff member's Dienstplan shifts
// (schedule.staff_shifts) in the window to calendar events. Cancelled shifts
// stay hidden, mirroring how cancelled timetable instances are skipped. The
// range finder does not load the ShiftType relation, so names come from one
// batch ListAll (the tenant's shift-type table is small); a type missing from
// the map (concurrently deleted) falls back to the generic title.
// Location stays deliberately empty: schedule.staff_shifts carries no room
// column, so there is nothing to resolve (#2078).
func (s *service) staffShiftEvents(ctx context.Context, staffID int64, from, to appointmentcap.Date) ([]Event, error) {
	return s.staffShiftEventsWithCancelled(ctx, staffID, from, to, false)
}

func (s *service) staffShiftFeedEvents(ctx context.Context, staffID int64, from, to appointmentcap.Date) ([]Event, error) {
	return s.staffShiftEventsWithCancelled(ctx, staffID, from, to, true)
}

func (s *service) staffShiftEventsWithCancelled(ctx context.Context, staffID int64, from, to appointmentcap.Date, includeCancelled bool) ([]Event, error) {
	if s.cfg.StaffShiftRepo == nil {
		return []Event{}, nil
	}
	shifts, err := s.cfg.StaffShiftRepo.FindByStaffAndDateRange(ctx, staffID, appointmentcap.Date(from), appointmentcap.Date(to))
	if err != nil {
		return nil, err
	}
	typeNames, err := s.shiftTypeNames(ctx, shifts)
	if err != nil {
		return nil, err
	}
	events := []Event{}
	for _, shift := range shifts {
		if shift.Cancelled && !includeCancelled {
			continue
		}
		events = append(events, shiftEvent(shift, typeNames))
	}
	return events, nil
}

// shiftTypeNames maps shift type IDs to names, reading the tenant's shift
// types only when a shift references one.
func (s *service) shiftTypeNames(ctx context.Context, shifts []*ports.StaffShift) (map[int64]string, error) {
	typeNames := map[int64]string{}
	if s.cfg.ShiftTypeRepo == nil || !shiftsReferenceTypes(shifts) {
		return typeNames, nil
	}
	shiftTypes, err := s.cfg.ShiftTypeRepo.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	for _, shiftType := range shiftTypes {
		typeNames[shiftType.ID] = shiftType.Name
	}
	return typeNames, nil
}

// shiftEvent maps one shift to a calendar event titled by its shift type,
// or the generic "Dienst" when the type is unknown.
func shiftEvent(shift *ports.StaffShift, typeNames map[int64]string) Event {
	title := "Dienst"
	if shift.ShiftTypeID != nil {
		if name := typeNames[*shift.ShiftTypeID]; name != "" {
			title = name
		}
	}
	event := Event{
		ID:         fmt.Sprintf("shift:%d", shift.ID),
		Source:     EventSourceShift,
		Title:      title,
		StartDate:  shift.Date.String(),
		EndDate:    shift.Date.String(),
		StartTime:  formatClock(shift.StartTime),
		EndTime:    formatClock(shift.EndTime),
		AllDay:     false,
		Cancelled:  shift.Cancelled,
		ModifiedAt: shift.UpdatedAt,
	}
	if shift.Notes != "" {
		notes := shift.Notes
		event.Description = &notes
	}
	return event
}
