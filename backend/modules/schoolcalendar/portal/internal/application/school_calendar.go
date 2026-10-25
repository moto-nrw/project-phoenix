package application

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	appointmentcap "github.com/moto-nrw/project-phoenix/modules/appointments"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal"
)

type SchoolCalendarView = portal.SchoolCalendarView

type SchoolDays = portal.SchoolDays

type SchoolAppointmentRequest = portal.SchoolAppointmentRequest

// ListSchoolCalendar is one window of the moto schule calendar (#4798): the
// appointments addressed to or organized by the Lehrkraft, and the closing
// days and holiday periods of the school. A Lehrkraft edits only the
// Schultermine they organize; there is no participant overview here.
func (s *service) ListSchoolCalendar(ctx context.Context, from, to appointmentcap.Date) (*SchoolCalendarView, error) {
	if err := validateWindow(from, to); err != nil {
		return nil, err
	}
	if s.cfg.SchoolDays == nil {
		return nil, errors.New("school calendar days are not configured")
	}
	staff, err := s.cfg.UserContext.GetCurrentStaff(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: current staff required", ErrForbidden)
	}
	appointments, err := s.listAppointmentsVisibleToStaff(ctx, staff.ID, toCalendarDate(from), toCalendarDate(to))
	if err != nil {
		return nil, err
	}
	events, err := s.expandAppointmentEvents(ctx, appointments, staff.ID, from, to)
	if err != nil {
		return nil, err
	}
	for i := range events {
		events[i].CanEdit = events[i].CanEdit && events[i].SchoolEventType != nil
		events[i].CanViewOverview = false
	}
	sortEvents(events)

	closingDays, err := s.cfg.SchoolDays.ListClosingDays(ctx, schoolcalendar.ClosingDayFilter{
		OverlappingFrom: from.String(), OverlappingTo: to.String(),
	})
	if err != nil {
		return nil, err
	}
	holidays, err := s.cfg.SchoolDays.ListCalendarPeriods(ctx, schoolcalendar.HolidaysOverlapping(from.String(), to.String()))
	if err != nil {
		return nil, err
	}
	classes, err := s.taughtClasses(ctx)
	if err != nil {
		return nil, err
	}
	view := &SchoolCalendarView{
		Events:      events,
		ClosingDays: make([]SchoolDays, 0, len(closingDays)),
		Holidays:    make([]SchoolDays, 0, len(holidays)),
		Classes:     classes,
	}
	for _, day := range closingDays {
		view.ClosingDays = append(view.ClosingDays, SchoolDays{
			ID: formatID(day.ID), Name: day.Reason, StartDate: day.StartDate, EndDate: day.EndDate,
		})
	}
	for _, period := range holidays {
		view.Holidays = append(view.Holidays, SchoolDays{
			ID: formatID(period.ID), Name: period.Name, StartDate: period.StartDate, EndDate: period.EndDate,
		})
	}
	return view, nil
}

// taughtClasses are the classes whose parents a Lehrkraft may show a
// Schultermin to: the classes they teach, trimmed and sorted.
func (s *service) taughtClasses(ctx context.Context) ([]string, error) {
	classes, err := s.cfg.UserContext.MySchoolClasses(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(classes))
	for _, class := range classes {
		if trimmed := strings.TrimSpace(class); trimmed != "" && !slices.Contains(result, trimmed) {
			result = append(result, trimmed)
		}
	}
	slices.Sort(result)
	return result, nil
}

// CreateSchoolAppointment enters a Schultermin. The OGS staff and the
// Lehrkräfte always see it; parents only when the Lehrkraft chooses them.
// It is informational: nobody answers it.
func (s *service) CreateSchoolAppointment(ctx context.Context, req SchoolAppointmentRequest) (*AppointmentDetail, error) {
	if !appointmentcap.ValidSchoolEventType(req.EventType) {
		return nil, fmt.Errorf("%w: invalid school event type", ErrInvalidRequest)
	}
	var result *AppointmentDetail
	err := s.withinAppointmentWrite(ctx, func(txCtx context.Context) error {
		targets, err := s.schoolAppointmentTargets(txCtx, req)
		if err != nil {
			return err
		}
		eventType := req.EventType
		result, err = s.createAppointment(txCtx, CreateAppointmentRequest{
			Title: req.Title, Description: req.Description, Location: req.Location,
			StartDate: req.StartDate, EndDate: req.EndDate, StartTime: req.StartTime, EndTime: req.EndTime,
			AllDay:             req.AllDay,
			DeliveryMode:       appointmentcap.DeliveryModeInformational,
			OverviewVisibility: appointmentcap.OverviewVisibilityOrganizer,
			Targets:            targets,
			SchoolEventType:    &eventType,
		}, true)
		return err
	})
	return result, err
}

// schoolAppointmentTargets builds the fixed audience of a Schultermin. A
// Lehrkraft may address the parents of the classes they teach or of the
// whole school; a class whose families have no portal access adds nobody
// instead of failing the entry.
func (s *service) schoolAppointmentTargets(ctx context.Context, req SchoolAppointmentRequest) ([]AppointmentTarget, error) {
	targets := []AppointmentTarget{
		{Type: appointmentcap.TargetTypeAllStaff},
		{Type: appointmentcap.TargetTypeAllTeachers},
	}
	switch req.Parents {
	case "", portal.SchoolParentsNone:
		return targets, nil
	case portal.SchoolParentsSchool:
		return append(targets, AppointmentTarget{Type: appointmentcap.TargetTypeAllSchoolParents}), nil
	case portal.SchoolParentsClasses:
	default:
		return nil, fmt.Errorf("%w: invalid parents audience", ErrInvalidRequest)
	}
	if len(req.Classes) == 0 {
		return nil, fmt.Errorf("%w: choose at least one class", ErrInvalidRequest)
	}
	taught, err := s.taughtClasses(ctx)
	if err != nil {
		return nil, err
	}
	taughtSet := make(map[string]bool, len(taught))
	for _, class := range taught {
		taughtSet[normalizeCalendarClass(class)] = true
	}
	for _, class := range req.Classes {
		if !taughtSet[normalizeCalendarClass(class)] {
			return nil, fmt.Errorf("%w: a Lehrkraft addresses only the parents of their own classes", ErrForbidden)
		}
		value := strings.TrimSpace(class)
		targets = append(targets, AppointmentTarget{Type: appointmentcap.TargetTypeParentsByClass, Value: &value})
	}
	return targets, nil
}

// UpdateSchoolAppointment edits a Schultermin the Lehrkraft organizes. The
// audience stays as created.
func (s *service) UpdateSchoolAppointment(ctx context.Context, appointmentID int64, req SchoolAppointmentRequest) (*AppointmentDetail, error) {
	if !appointmentcap.ValidSchoolEventType(req.EventType) {
		return nil, fmt.Errorf("%w: invalid school event type", ErrInvalidRequest)
	}
	var result *AppointmentDetail
	err := s.withinAppointmentWrite(ctx, func(txCtx context.Context) error {
		appointment, err := s.loadSchoolAppointmentForWrite(txCtx, appointmentID)
		if err != nil {
			return err
		}
		eventType := req.EventType
		appointment.SchoolEventType = &eventType
		result, err = s.updateLoadedAppointment(txCtx, appointment, UpdateAppointmentRequest{
			Title: req.Title, Description: req.Description, Location: req.Location,
			StartDate: req.StartDate, EndDate: req.EndDate, StartTime: req.StartTime, EndTime: req.EndTime,
			AllDay: req.AllDay,
		})
		return err
	})
	return result, err
}

// DeleteSchoolAppointment removes a Schultermin the Lehrkraft organizes.
func (s *service) DeleteSchoolAppointment(ctx context.Context, appointmentID int64) error {
	return s.withinAppointmentWrite(ctx, func(txCtx context.Context) error {
		appointment, err := s.loadSchoolAppointmentForWrite(txCtx, appointmentID)
		if err != nil {
			return err
		}
		return s.softDeleteLoadedAppointment(txCtx, appointment)
	})
}

// RespondToSchoolInvitation answers an OGS appointment that asks the
// Lehrkraft for a reply.
func (s *service) RespondToSchoolInvitation(ctx context.Context, recipientID int64, status string) error {
	return s.RespondToStaffInvitation(ctx, recipientID, status)
}

// SchoolCalendarFeedURL and RotateSchoolCalendarFeed hand out the same
// account-bound subscription feed as the OGS calendar: one person, one feed.
func (s *service) SchoolCalendarFeedURL(ctx context.Context) (string, string, error) {
	return s.StaffCalendarFeedURL(ctx)
}

func (s *service) RotateSchoolCalendarFeed(ctx context.Context) (string, string, error) {
	return s.RotateStaffCalendarFeed(ctx)
}

// loadSchoolAppointmentForWrite locks a Schultermin the current Lehrkraft
// organizes. OGS appointments stay out of reach of moto schule, even for an
// account that organizes them in the OGS portal.
func (s *service) loadSchoolAppointmentForWrite(ctx context.Context, appointmentID int64) (*appointmentcap.Appointment, error) {
	appointment, err := s.loadAppointmentForWrite(ctx, appointmentID)
	if err != nil {
		return nil, err
	}
	if appointment.SchoolEventType == nil {
		return nil, fmt.Errorf("%w: only Schultermine can be changed in moto schule", ErrForbidden)
	}
	return appointment, nil
}
