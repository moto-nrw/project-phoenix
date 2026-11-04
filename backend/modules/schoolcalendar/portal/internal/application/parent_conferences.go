package application

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal"
)

// The Lehrkraft's side of the Elternsprechtag in moto schule (#4801). A
// Lehrkraft offers a day for a class she teaches and changes only her own
// days. A change of the day keeps every booking whose class and slot remain
// and moves its appointment along; other bookings are cancelled.
// Both tell the family through the appointment mail and push.

type (
	ParentConferenceRequest    = portal.ParentConferenceRequest
	ParentConferenceView       = portal.ParentConferenceView
	SchoolParentConferenceList = portal.SchoolParentConferenceList
)

func (s *service) parentConferences() (schoolcalendar.ParentConferences, error) {
	if s.cfg.ParentConferences == nil {
		return nil, errors.New("parent conferences are not configured")
	}
	return s.cfg.ParentConferences, nil
}

// ListSchoolParentConferences lists the Lehrkraft's days with their
// occupancy and the classes she may offer a new day for.
func (s *service) ListSchoolParentConferences(ctx context.Context) (*SchoolParentConferenceList, error) {
	store, err := s.parentConferences()
	if err != nil {
		return nil, err
	}
	staff, err := s.cfg.UserContext.GetCurrentStaff(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: current staff required", ErrForbidden)
	}
	classes, err := s.taughtClasses(ctx)
	if err != nil {
		return nil, err
	}
	conferences, err := store.ListParentConferences(ctx, schoolcalendar.ParentConferenceFilter{OrganizerStaffID: staff.ID})
	if err != nil {
		return nil, err
	}
	views, err := s.parentConferenceViews(ctx, conferences)
	if err != nil {
		return nil, err
	}
	return &SchoolParentConferenceList{Conferences: views, Classes: classes}, nil
}

// GetSchoolParentConference returns one of the Lehrkraft's days with the
// names of the booked children, for the occupancy list and its print.
func (s *service) GetSchoolParentConference(ctx context.Context, id int64) (*ParentConferenceView, error) {
	conference, err := s.loadOwnParentConference(ctx, id, false)
	if err != nil {
		return nil, err
	}
	return s.parentConferenceView(ctx, conference)
}

// CreateSchoolParentConference offers a new day for a class the Lehrkraft
// teaches.
func (s *service) CreateSchoolParentConference(ctx context.Context, req ParentConferenceRequest) (*ParentConferenceView, error) {
	store, err := s.parentConferences()
	if err != nil {
		return nil, err
	}
	staff, err := s.cfg.UserContext.GetCurrentStaff(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: current staff required", ErrForbidden)
	}
	account, err := s.cfg.UserContext.GetCurrentUser(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: current account required", ErrForbidden)
	}
	fields, err := s.parentConferenceFields(ctx, req, "")
	if err != nil {
		return nil, err
	}
	fields.OrganizerStaffID, fields.OrganizerAccountID = staff.ID, account.ID
	var view *ParentConferenceView
	err = s.withinAppointmentWrite(ctx, func(txCtx context.Context) error {
		created, err := store.CreateParentConference(txCtx, fields)
		if err != nil {
			return mapParentConferenceError(err)
		}
		view, err = s.parentConferenceView(txCtx, created)
		return err
	})
	return view, err
}

// UpdateSchoolParentConference changes one of the Lehrkraft's days. The
// bookings follow the change: kept where their class and slot remain,
// cancelled otherwise.
func (s *service) UpdateSchoolParentConference(ctx context.Context, id int64, req ParentConferenceRequest) (*ParentConferenceView, error) {
	store, err := s.parentConferences()
	if err != nil {
		return nil, err
	}
	var view *ParentConferenceView
	err = s.withinAppointmentWrite(ctx, func(txCtx context.Context) error {
		current, err := s.loadOwnParentConference(txCtx, id, true)
		if err != nil {
			return err
		}
		fields, err := s.parentConferenceFields(txCtx, req, current.SchoolClass)
		if err != nil {
			return err
		}
		fields.OrganizerStaffID, fields.OrganizerAccountID = current.OrganizerStaffID, current.OrganizerAccountID
		updated, err := store.UpdateParentConference(txCtx, id, fields)
		if err != nil {
			return mapParentConferenceError(err)
		}
		if err := s.followParentConferenceChange(txCtx, updated, current.SchoolClass); err != nil {
			return err
		}
		view, err = s.parentConferenceView(txCtx, updated)
		return err
	})
	return view, err
}

// DeleteSchoolParentConference removes one of the Lehrkraft's days and
// cancels its bookings.
func (s *service) DeleteSchoolParentConference(ctx context.Context, id int64) error {
	store, err := s.parentConferences()
	if err != nil {
		return err
	}
	return s.withinAppointmentWrite(ctx, func(txCtx context.Context) error {
		conference, err := s.loadOwnParentConference(txCtx, id, true)
		if err != nil {
			return err
		}
		bookings, err := store.ListParentConferenceBookings(txCtx, []int64{conference.ID})
		if err != nil {
			return err
		}
		for _, booking := range bookings {
			if err := s.cancelParentConferenceAppointment(txCtx, booking); err != nil {
				return err
			}
		}
		return mapParentConferenceError(store.DeleteParentConference(txCtx, conference.ID))
	})
}

// followParentConferenceChange moves the appointment of every booking whose
// class and slot remain; all other bookings are cancelled.
func (s *service) followParentConferenceChange(ctx context.Context, conference *schoolcalendar.ParentConference, previousClass string) error {
	bookings, err := s.cfg.ParentConferences.ListParentConferenceBookings(ctx, []int64{conference.ID})
	if err != nil {
		return err
	}
	sameClass := normalizeCalendarClass(conference.SchoolClass) == normalizeCalendarClass(previousClass)
	var gone []int64
	for _, booking := range bookings {
		if sameClass && schoolcalendar.HasParentConferenceSlot(conference.ParentConferenceFields, booking.SlotStartMinute) {
			if err := s.moveParentConferenceAppointment(ctx, conference, booking); err != nil {
				return err
			}
			continue
		}
		if err := s.cancelParentConferenceAppointment(ctx, booking); err != nil {
			return err
		}
		gone = append(gone, booking.ID)
	}
	return s.cfg.ParentConferences.DeleteParentConferenceBookings(ctx, gone)
}

// loadOwnParentConference loads a day the current Lehrkraft organizes.
// Another Lehrkraft's day is not found: its existence is none of her
// business.
func (s *service) loadOwnParentConference(ctx context.Context, id int64, lock bool) (*schoolcalendar.ParentConference, error) {
	store, err := s.parentConferences()
	if err != nil {
		return nil, err
	}
	staff, err := s.cfg.UserContext.GetCurrentStaff(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: current staff required", ErrForbidden)
	}
	conference, err := store.FindParentConference(ctx, id, lock)
	if err != nil {
		return nil, mapParentConferenceError(err)
	}
	if conference.OrganizerStaffID != staff.ID {
		return nil, ErrNotFound
	}
	return conference, nil
}

// parentConferenceFields turns the request into stored values. A new day or
// a changed class must be a class the Lehrkraft teaches; keptClass is the
// class of the day being changed, which she keeps even after the class
// moved on. A day cannot lie in the past.
func (s *service) parentConferenceFields(ctx context.Context, req ParentConferenceRequest, keptClass string) (schoolcalendar.ParentConferenceFields, error) {
	fields := schoolcalendar.ParentConferenceFields{
		SchoolClass: strings.TrimSpace(req.SchoolClass), Date: req.Date,
		SlotMinutes: req.SlotMinutes, Location: req.Location,
	}
	if fields.Date.IsZero() || fields.Date.Before(s.conferenceToday()) {
		return fields, fmt.Errorf("%w: the day must be today or later", ErrInvalidRequest)
	}
	var err error
	if fields.StartMinute, err = parseClockMinute(req.StartTime); err != nil {
		return fields, err
	}
	if fields.EndMinute, err = parseClockMinute(req.EndTime); err != nil {
		return fields, err
	}
	for _, pause := range req.Breaks {
		start, err := parseClockMinute(pause.StartTime)
		if err != nil {
			return fields, err
		}
		end, err := parseClockMinute(pause.EndTime)
		if err != nil {
			return fields, err
		}
		fields.Breaks = append(fields.Breaks, schoolcalendar.ParentConferenceBreak{StartMinute: start, EndMinute: end})
	}
	if keptClass != "" && normalizeCalendarClass(fields.SchoolClass) == normalizeCalendarClass(keptClass) {
		fields.SchoolClass = keptClass
		return fields, nil
	}
	return fields, s.requireTaughtClass(ctx, fields.SchoolClass)
}

func (s *service) requireTaughtClass(ctx context.Context, class string) error {
	taught, err := s.taughtClasses(ctx)
	if err != nil {
		return err
	}
	for _, candidate := range taught {
		if normalizeCalendarClass(candidate) == normalizeCalendarClass(class) {
			return nil
		}
	}
	return fmt.Errorf("%w: a Lehrkraft offers a Sprechtag only for her own classes", ErrForbidden)
}

// parentConferenceViews builds the views of several days with one booking
// and one student read.
func (s *service) parentConferenceViews(ctx context.Context, conferences []*schoolcalendar.ParentConference) ([]ParentConferenceView, error) {
	ids := make([]int64, 0, len(conferences))
	for _, conference := range conferences {
		ids = append(ids, conference.ID)
	}
	bookings, err := s.cfg.ParentConferences.ListParentConferenceBookings(ctx, ids)
	if err != nil {
		return nil, err
	}
	names, err := s.bookedStudentNames(ctx, bookings)
	if err != nil {
		return nil, err
	}
	byConference := make(map[int64][]*schoolcalendar.ParentConferenceBooking, len(conferences))
	for _, booking := range bookings {
		byConference[booking.ConferenceID] = append(byConference[booking.ConferenceID], booking)
	}
	views := make([]ParentConferenceView, 0, len(conferences))
	for _, conference := range conferences {
		views = append(views, buildParentConferenceView(conference, byConference[conference.ID], names))
	}
	return views, nil
}

func (s *service) parentConferenceView(ctx context.Context, conference *schoolcalendar.ParentConference) (*ParentConferenceView, error) {
	views, err := s.parentConferenceViews(ctx, []*schoolcalendar.ParentConference{conference})
	if err != nil {
		return nil, err
	}
	return &views[0], nil
}

// bookedStudentNames names the booked children for the Lehrkraft.
func (s *service) bookedStudentNames(ctx context.Context, bookings []*schoolcalendar.ParentConferenceBooking) (map[int64]string, error) {
	names := make(map[int64]string, len(bookings))
	if len(bookings) == 0 {
		return names, nil
	}
	students, err := s.cfg.StudentRepo.FindAllWithGroups(ctx)
	if err != nil {
		return nil, err
	}
	for _, student := range students {
		if student.Person != nil {
			names[student.ID] = strings.TrimSpace(student.Person.FirstName + " " + student.Person.LastName)
		}
	}
	return names, nil
}

func buildParentConferenceView(conference *schoolcalendar.ParentConference, bookings []*schoolcalendar.ParentConferenceBooking, names map[int64]string) ParentConferenceView {
	booked := make(map[int]*schoolcalendar.ParentConferenceBooking, len(bookings))
	for _, booking := range bookings {
		booked[booking.SlotStartMinute] = booking
	}
	view := ParentConferenceView{
		ID: formatID(conference.ID), SchoolClass: conference.SchoolClass, Date: conference.Date,
		StartTime: formatClockMinute(conference.StartMinute), EndTime: formatClockMinute(conference.EndMinute),
		SlotMinutes: conference.SlotMinutes, Breaks: parentConferenceBreakWindows(conference.Breaks),
		Location: conference.Location, BookedCount: len(bookings), Slots: []portal.ParentConferenceSlot{},
	}
	for _, start := range schoolcalendar.ParentConferenceSlots(conference.ParentConferenceFields) {
		slot := portal.ParentConferenceSlot{
			StartTime: formatClockMinute(start), EndTime: formatClockMinute(start + conference.SlotMinutes),
		}
		if booking, ok := booked[start]; ok {
			slot.Booked = true
			studentID := formatID(booking.StudentID)
			slot.StudentID = &studentID
			if name, ok := names[booking.StudentID]; ok {
				slot.StudentName = &name
			}
		}
		view.Slots = append(view.Slots, slot)
	}
	return view
}

func parentConferenceBreakWindows(breaks []schoolcalendar.ParentConferenceBreak) []portal.ParentConferenceWindow {
	windows := make([]portal.ParentConferenceWindow, 0, len(breaks))
	for _, pause := range breaks {
		windows = append(windows, portal.ParentConferenceWindow{
			StartTime: formatClockMinute(pause.StartMinute), EndTime: formatClockMinute(pause.EndMinute),
		})
	}
	return windows
}

// mapParentConferenceError translates the owner's errors into the portal's.
func mapParentConferenceError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, schoolcalendar.ErrParentConferenceNotFound):
		return ErrNotFound
	case errors.Is(err, schoolcalendar.ErrParentConferenceSlotTaken):
		return portal.ErrParentConferenceSlotTaken
	case errors.Is(err, schoolcalendar.ErrInvalidParentConference):
		return fmt.Errorf("%w: %v", ErrInvalidRequest, err)
	default:
		return err
	}
}

// parseClockMinute reads "HH:MM" as minutes after midnight.
func parseClockMinute(value string) (int, error) {
	hours, minutes, ok := strings.Cut(strings.TrimSpace(value), ":")
	h, hErr := strconv.Atoi(hours)
	m, mErr := strconv.Atoi(minutes)
	if !ok || hErr != nil || mErr != nil || len(minutes) != 2 || h < 0 || m < 0 || m > 59 || h*60+m >= 24*60 {
		return 0, fmt.Errorf("%w: invalid time %q", ErrInvalidRequest, value)
	}
	return h*60 + m, nil
}

func formatClockMinute(minute int) string {
	return fmt.Sprintf("%02d:%02d", minute/60, minute%60)
}

func (s *service) now() time.Time {
	if s.cfg.Now != nil {
		return s.cfg.Now()
	}
	return time.Now()
}
