package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	appointmentcap "github.com/moto-nrw/project-phoenix/modules/appointments"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/sharedkernel/collation"
)

// The families' side of the Elternsprechtag (#4801). The parent portal has
// already checked the guardian relationship of the child; here the child
// must still belong to the class of the day. A family sees the free slots
// only, books one per child and can move or cancel it until 24 hours before
// it starts. Every booking is an appointment of the Lehrkraft addressed to
// the child's families, so the existing appointment mail and push tell them
// about it and both calendars show it.

type (
	ParentConferenceChild = portal.ParentConferenceChild
	ParentConferenceOffer = portal.ParentConferenceOffer
)

// Kinds of the notice to the Lehrkraft.
const (
	parentConferenceBooked    = "booked"
	parentConferenceMoved     = "moved"
	parentConferenceCancelled = "cancelled"
)

// ListChildParentConferences lists the coming days of the child's class.
func (s *service) ListChildParentConferences(ctx context.Context, child ParentConferenceChild) ([]ParentConferenceOffer, error) {
	var offers []ParentConferenceOffer
	err := s.withinChildTenant(ctx, child, func(txCtx context.Context, store schoolcalendar.ParentConferences, class string) error {
		conferences, err := store.ListParentConferences(txCtx, schoolcalendar.ParentConferenceFilter{From: s.conferenceToday()})
		if err != nil {
			return err
		}
		mine := conferencesOfClass(conferences, class)
		offers, err = s.parentConferenceOffers(txCtx, mine, child.StudentID)
		return err
	})
	return offers, err
}

// BookParentConference books the slot for the child, or moves the child's
// booking there.
func (s *service) BookParentConference(ctx context.Context, child ParentConferenceChild, conferenceID int64, startTime string) (*ParentConferenceOffer, error) {
	minute, err := parseClockMinute(startTime)
	if err != nil {
		return nil, err
	}
	var offer *ParentConferenceOffer
	err = s.withinChildTenant(ctx, child, func(txCtx context.Context, store schoolcalendar.ParentConferences, class string) error {
		conference, booking, err := s.childParentConference(txCtx, store, conferenceID, class, child.StudentID)
		if err != nil {
			return err
		}
		if !schoolcalendar.HasParentConferenceSlot(conference.ParentConferenceFields, minute) {
			return fmt.Errorf("%w: the day has no slot at %s", ErrInvalidRequest, startTime)
		}
		if booking != nil && booking.SlotStartMinute == minute {
			offer, err = s.parentConferenceOffer(txCtx, conference, child.StudentID)
			return err
		}
		if err := s.requireBookingChangeable(conference, booking, minute); err != nil {
			return err
		}
		if booking == nil {
			err = s.bookParentConferenceSlot(txCtx, conference, child, minute)
		} else {
			err = s.moveParentConferenceBooking(txCtx, conference, booking, minute)
		}
		if err != nil {
			return err
		}
		offer, err = s.parentConferenceOffer(txCtx, conference, child.StudentID)
		return err
	})
	return offer, err
}

// CancelParentConferenceBooking cancels the child's booking of the day.
func (s *service) CancelParentConferenceBooking(ctx context.Context, child ParentConferenceChild, conferenceID int64) error {
	return s.withinChildTenant(ctx, child, func(txCtx context.Context, store schoolcalendar.ParentConferences, class string) error {
		conference, booking, err := s.childParentConference(txCtx, store, conferenceID, class, child.StudentID)
		if err != nil {
			return err
		}
		if booking == nil {
			return ErrNotFound
		}
		if err := s.requireChangeable(conference, booking.SlotStartMinute); err != nil {
			return err
		}
		if err := s.cancelParentConferenceAppointment(txCtx, booking); err != nil {
			return err
		}
		if err := store.DeleteParentConferenceBookings(txCtx, []int64{booking.ID}); err != nil {
			return err
		}
		return s.notifyParentConferenceOrganizer(txCtx, noticeFor(conference), parentConferenceCancelled, booking.SlotStartMinute)
	})
}

// withinChildTenant runs fn in the transaction the caller opened for the
// child's school, with the child's class. A child without a class, or one
// no longer active, has no Sprechtag.
func (s *service) withinChildTenant(ctx context.Context, child ParentConferenceChild, fn func(context.Context, schoolcalendar.ParentConferences, string) error) error {
	store, err := s.parentConferences()
	if err != nil {
		return err
	}
	if child.TenantID <= 0 || child.StudentID <= 0 {
		return fmt.Errorf("%w: child is required", ErrForbidden)
	}
	return s.withinAppointmentWrite(ctx, func(txCtx context.Context) error {
		if s.cfg.Runtime.TenantID(txCtx) != child.TenantID {
			return fmt.Errorf("%w: the child belongs to another school", ErrForbidden)
		}
		class, err := s.activeStudentClass(txCtx, child.StudentID)
		if err != nil {
			return err
		}
		return fn(txCtx, store, class)
	})
}

func (s *service) activeStudentClass(ctx context.Context, studentID int64) (string, error) {
	students, err := s.cfg.StudentRepo.ListActive(ctx)
	if err != nil {
		return "", err
	}
	for _, student := range students {
		if student.ID == studentID {
			return strings.TrimSpace(student.SchoolClass), nil
		}
	}
	return "", nil
}

// childParentConference locks a coming day of the child's class and finds
// the child's booking on it. A day of another class is not found.
func (s *service) childParentConference(ctx context.Context, store schoolcalendar.ParentConferences, conferenceID int64, class string, studentID int64) (*schoolcalendar.ParentConference, *schoolcalendar.ParentConferenceBooking, error) {
	conference, err := store.FindParentConference(ctx, conferenceID, true)
	if err != nil {
		return nil, nil, mapParentConferenceError(err)
	}
	if class == "" || normalizeCalendarClass(conference.SchoolClass) != normalizeCalendarClass(class) || conference.Date.Before(s.conferenceToday()) {
		return nil, nil, ErrNotFound
	}
	bookings, err := store.ListParentConferenceBookings(ctx, []int64{conference.ID})
	if err != nil {
		return nil, nil, err
	}
	for _, booking := range bookings {
		if booking.StudentID == studentID {
			return conference, booking, nil
		}
	}
	return conference, nil, nil
}

// requireBookingChangeable checks the destination and, when moving, the
// existing booking so a later slot cannot bypass the original deadline.
func (s *service) requireBookingChangeable(conference *schoolcalendar.ParentConference, booking *schoolcalendar.ParentConferenceBooking, minute int) error {
	if err := s.requireChangeable(conference, minute); err != nil {
		return err
	}
	if booking != nil {
		return s.requireChangeable(conference, booking.SlotStartMinute)
	}
	return nil
}

// requireChangeable refuses a slot that starts in less than 24 hours.
func (s *service) requireChangeable(conference *schoolcalendar.ParentConference, minute int) error {
	if !s.changeable(conference, minute) {
		return portal.ErrParentConferenceChangeClosed
	}
	return nil
}

func (s *service) changeable(conference *schoolcalendar.ParentConference, minute int) bool {
	start := schoolcalendar.ParentConferenceSlotStart(conference.Date, minute)
	return start.Sub(s.now()) >= portal.ParentConferenceChangeWindow
}

func (s *service) bookParentConferenceSlot(ctx context.Context, conference *schoolcalendar.ParentConference, child ParentConferenceChild, minute int) error {
	guardianProfileID := child.GuardianProfileID
	booking, err := s.cfg.ParentConferences.CreateParentConferenceBooking(ctx, schoolcalendar.ParentConferenceBooking{
		ConferenceID: conference.ID, StudentID: child.StudentID, GuardianProfileID: &guardianProfileID, SlotStartMinute: minute,
	})
	if err != nil {
		return mapParentConferenceError(err)
	}
	appointmentID, err := s.createParentConferenceAppointment(ctx, conference, child.StudentID, minute)
	if err != nil {
		return err
	}
	if err := s.cfg.ParentConferences.LinkParentConferenceBookingAppointment(ctx, booking.ID, appointmentID); err != nil {
		return err
	}
	return s.notifyParentConferenceOrganizer(ctx, noticeFor(conference), parentConferenceBooked, minute)
}

func (s *service) moveParentConferenceBooking(ctx context.Context, conference *schoolcalendar.ParentConference, booking *schoolcalendar.ParentConferenceBooking, minute int) error {
	if err := s.cfg.ParentConferences.MoveParentConferenceBooking(ctx, booking.ID, minute); err != nil {
		return mapParentConferenceError(err)
	}
	booking.SlotStartMinute = minute
	if err := s.moveParentConferenceAppointment(ctx, conference, booking); err != nil {
		return err
	}
	return s.notifyParentConferenceOrganizer(ctx, noticeFor(conference), parentConferenceMoved, minute)
}

// createParentConferenceAppointment writes the appointment that shows the
// booking: organized by the Lehrkraft, addressed to the child's families,
// informational and announced by mail and push. The title names the class,
// not the child.
func (s *service) createParentConferenceAppointment(ctx context.Context, conference *schoolcalendar.ParentConference, studentID int64, minute int) (int64, error) {
	eventType := appointmentcap.SchoolEventTypeParentConference
	request := parentConferenceAppointmentRequest(conference, minute)
	detail, err := s.createAppointmentFor(ctx, conference.OrganizerStaffID, CreateAppointmentRequest{
		Title: request.Title, Location: request.Location,
		StartDate: request.StartDate, EndDate: request.EndDate, StartTime: request.StartTime, EndTime: request.EndTime,
		DeliveryMode:       appointmentcap.DeliveryModeInformational,
		OverviewVisibility: appointmentcap.OverviewVisibilityOrganizer,
		Targets:            []AppointmentTarget{{Type: appointmentcap.TargetTypeParentsByStudent, ID: &studentID}},
		SendEmail:          true,
		SchoolEventType:    &eventType,
	}, true)
	if err != nil {
		return 0, err
	}
	return detail.Appointment.ID, nil
}

// moveParentConferenceAppointment moves the booking's appointment to the
// booking's slot and announces the change. A booking whose appointment is
// gone gets a new one.
func (s *service) moveParentConferenceAppointment(ctx context.Context, conference *schoolcalendar.ParentConference, booking *schoolcalendar.ParentConferenceBooking) error {
	appointment, err := s.liveParentConferenceAppointment(ctx, booking)
	if err != nil {
		return err
	}
	if appointment == nil {
		appointmentID, err := s.createParentConferenceAppointment(ctx, conference, booking.StudentID, booking.SlotStartMinute)
		if err != nil {
			return err
		}
		return s.cfg.ParentConferences.LinkParentConferenceBookingAppointment(ctx, booking.ID, appointmentID)
	}
	request := parentConferenceAppointmentRequest(conference, booking.SlotStartMinute)
	if sameParentConferenceAppointment(appointment, request) {
		return nil
	}
	_, err = s.updateLoadedAppointment(ctx, appointment, request)
	return err
}

// cancelParentConferenceAppointment cancels the booking's appointment and
// tells the families.
func (s *service) cancelParentConferenceAppointment(ctx context.Context, booking *schoolcalendar.ParentConferenceBooking) error {
	appointment, err := s.liveParentConferenceAppointment(ctx, booking)
	if err != nil || appointment == nil {
		return err
	}
	transitioned, err := s.cfg.Appointments.CancelAppointment(ctx, appointment.ID)
	if err != nil || !transitioned {
		return err
	}
	now := s.now()
	appointment.CancelledAt = &now
	if err := s.cancelPendingNotifications(ctx, appointment.ID, "parent conference booking cancelled"); err != nil {
		return err
	}
	return s.notifyGuardianLifecycle(ctx, appointment, portal.EmailKindAppointmentCancelled)
}

// liveParentConferenceAppointment locks the booking's appointment; nil when
// it is gone, cancelled or deleted.
func (s *service) liveParentConferenceAppointment(ctx context.Context, booking *schoolcalendar.ParentConferenceBooking) (*appointmentcap.Appointment, error) {
	if booking.AppointmentID == nil {
		return nil, nil
	}
	appointment, err := s.cfg.Appointments.FindAppointmentForUpdate(ctx, *booking.AppointmentID)
	if errors.Is(err, appointmentcap.ErrAppointmentNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if appointment.CancelledAt != nil || appointment.DeletedAt != nil {
		return nil, nil
	}
	return appointment, nil
}

func parentConferenceAppointmentRequest(conference *schoolcalendar.ParentConference, minute int) UpdateAppointmentRequest {
	return UpdateAppointmentRequest{
		Title:     "Elternsprechtag " + collation.SchoolClassLabel(conference.SchoolClass),
		Location:  conference.Location,
		StartDate: appointmentcap.Date(conference.Date.String()),
		EndDate:   appointmentcap.Date(conference.Date.String()),
		StartTime: clockMinuteTime(minute),
		EndTime:   clockMinuteTime(minute + conference.SlotMinutes),
	}
}

func sameParentConferenceAppointment(appointment *appointmentcap.Appointment, request UpdateAppointmentRequest) bool {
	sameLocation := (appointment.Location == nil && request.Location == nil) ||
		(appointment.Location != nil && request.Location != nil && *appointment.Location == *request.Location)
	return appointment.Title == request.Title && sameLocation &&
		appointment.StartDate == request.StartDate && appointment.EndDate == request.EndDate &&
		formatClock(appointment.StartTime) == formatClock(request.StartTime) &&
		formatClock(appointment.EndTime) == formatClock(request.EndTime)
}

// isParentConferenceAppointment reports whether the appointment shows an
// Elternsprechtag booking.
func isParentConferenceAppointment(appointment *appointmentcap.Appointment) bool {
	return appointment.SchoolEventType != nil && *appointment.SchoolEventType == appointmentcap.SchoolEventTypeParentConference
}

func clockMinuteTime(minute int) time.Time {
	return time.Date(1, time.January, 1, minute/60, minute%60, 0, 0, time.UTC)
}

func conferencesOfClass(conferences []*schoolcalendar.ParentConference, class string) []*schoolcalendar.ParentConference {
	if class == "" {
		return nil
	}
	result := make([]*schoolcalendar.ParentConference, 0, len(conferences))
	for _, conference := range conferences {
		if normalizeCalendarClass(conference.SchoolClass) == normalizeCalendarClass(class) {
			result = append(result, conference)
		}
	}
	return result
}

func (s *service) parentConferenceOffer(ctx context.Context, conference *schoolcalendar.ParentConference, studentID int64) (*ParentConferenceOffer, error) {
	offers, err := s.parentConferenceOffers(ctx, []*schoolcalendar.ParentConference{conference}, studentID)
	if err != nil {
		return nil, err
	}
	return &offers[0], nil
}

// parentConferenceOffers builds the families' view of several days with one
// booking and one staff read. Other children's bookings show only as a
// missing slot.
func (s *service) parentConferenceOffers(ctx context.Context, conferences []*schoolcalendar.ParentConference, studentID int64) ([]ParentConferenceOffer, error) {
	offers := make([]ParentConferenceOffer, 0, len(conferences))
	if len(conferences) == 0 {
		return offers, nil
	}
	ids := make([]int64, 0, len(conferences))
	staffIDs := make([]int64, 0, len(conferences))
	for _, conference := range conferences {
		ids = append(ids, conference.ID)
		staffIDs = append(staffIDs, conference.OrganizerStaffID)
	}
	bookings, err := s.cfg.ParentConferences.ListParentConferenceBookings(ctx, ids)
	if err != nil {
		return nil, err
	}
	teachers, err := s.cfg.StaffRepo.FindWithPersonByIDs(ctx, staffIDs)
	if err != nil {
		return nil, err
	}
	taken := make(map[int64]map[int]*schoolcalendar.ParentConferenceBooking, len(conferences))
	for _, booking := range bookings {
		if taken[booking.ConferenceID] == nil {
			taken[booking.ConferenceID] = map[int]*schoolcalendar.ParentConferenceBooking{}
		}
		taken[booking.ConferenceID][booking.SlotStartMinute] = booking
	}
	for _, conference := range conferences {
		offers = append(offers, s.buildParentConferenceOffer(conference, taken[conference.ID], teachers[conference.OrganizerStaffID], studentID))
	}
	return offers, nil
}

func (s *service) buildParentConferenceOffer(conference *schoolcalendar.ParentConference, taken map[int]*schoolcalendar.ParentConferenceBooking, teacher *ports.Staff, studentID int64) ParentConferenceOffer {
	offer := ParentConferenceOffer{
		ID: formatID(conference.ID), SchoolClass: conference.SchoolClass, TeacherName: staffDisplayName(teacher),
		Date: conference.Date, Location: conference.Location, FreeSlots: []portal.ParentConferenceWindow{},
	}
	for _, start := range schoolcalendar.ParentConferenceSlots(conference.ParentConferenceFields) {
		window := portal.ParentConferenceWindow{
			StartTime: formatClockMinute(start), EndTime: formatClockMinute(start + conference.SlotMinutes),
		}
		booking, booked := taken[start]
		switch {
		case booked && booking.StudentID == studentID:
			offer.Booking = &window
			offer.CanChange = s.changeable(conference, start)
		case !booked && s.changeable(conference, start):
			offer.FreeSlots = append(offer.FreeSlots, window)
		}
	}
	if offer.Booking == nil {
		offer.CanChange = len(offer.FreeSlots) > 0
	}
	return offer
}

func staffDisplayName(staff *ports.Staff) string {
	if staff == nil || staff.Person == nil {
		return ""
	}
	return strings.TrimSpace(staff.Person.FirstName + " " + staff.Person.LastName)
}

// parentConferenceNotice is what the notice to the Lehrkraft needs of a day.
type parentConferenceNotice struct {
	ConferenceID       int64
	OrganizerAccountID int64
	SchoolClass        string
	Date               calendar.Date
}

func noticeFor(conference *schoolcalendar.ParentConference) parentConferenceNotice {
	return parentConferenceNotice{
		ConferenceID: conference.ID, OrganizerAccountID: conference.OrganizerAccountID,
		SchoolClass: conference.SchoolClass, Date: conference.Date,
	}
}

// notifyParentConferenceOrganizer tells the Lehrkraft in moto schule that a
// family booked, moved or cancelled a slot, when she agreed to it. The
// notice names class, day and time, never the child: it lands on a lock
// screen.
func (s *service) notifyParentConferenceOrganizer(ctx context.Context, conference parentConferenceNotice, kind string, minute int) error {
	if s.cfg.Notifier == nil || s.cfg.Preferences == nil || conference.OrganizerAccountID <= 0 {
		return nil
	}
	accountIDs, err := s.cfg.Preferences.FilterOptedIn(ctx, portal.NotificationTypeParentConferenceBooking, []int64{conference.OrganizerAccountID})
	if err != nil {
		return fmt.Errorf("filter opted-in Lehrkraft: %w", err)
	}
	if len(accountIDs) == 0 {
		return nil
	}
	err = s.cfg.Notifier.Notify(ctx, ports.Event{
		Type:           portal.NotificationTypeParentConferenceBooking,
		IdempotencyKey: fmt.Sprintf("parent-conference:%d:%s:%d:%d", conference.ConferenceID, kind, minute, s.now().UnixNano()),
		RelatedType:    "parent_conference", RelatedID: conference.ConferenceID,
		Title:          "Elternsprechtag " + collation.SchoolClassLabel(conference.SchoolClass),
		Body:           parentConferenceNoticeBody(conference, kind, minute),
		SchoolDeepLink: fmt.Sprintf("/school/elternsprechtag?sprechtag=%d", conference.ConferenceID),
		Priority:       "normal",
		Audience: ports.Audience{
			TenantID: s.cfg.Runtime.TenantID(ctx), Scope: "staff", StaffAccountIDs: accountIDs,
		},
	})
	if s.cfg.NoDelivery(err) {
		s.logger().Info("calendar: parent conference notice not delivered",
			slog.Int64("parent_conference_id", conference.ConferenceID),
			slog.String("reason", err.Error()),
		)
		return nil
	}
	return err
}

func parentConferenceNoticeBody(conference parentConferenceNotice, kind string, minute int) string {
	when := fmt.Sprintf("%s um %s Uhr", conference.Date.Format("02.01."), formatClockMinute(minute))
	switch kind {
	case parentConferenceMoved:
		return "Eine Familie hat ihren Termin verschoben auf " + when + "."
	case parentConferenceCancelled:
		return "Eine Familie hat ihren Termin am " + when + " abgesagt."
	default:
		return "Eine Familie hat einen Termin gebucht: " + when + "."
	}
}

// conferenceToday is the calendar's day as the School Calendar writes it.
func (s *service) conferenceToday() calendar.Date {
	return calendar.Date(s.today().String())
}
