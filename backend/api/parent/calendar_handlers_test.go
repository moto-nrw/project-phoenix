package parent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	calModels "github.com/moto-nrw/project-phoenix/models/calendar"
	calendarSvc "github.com/moto-nrw/project-phoenix/services/calendar"
	"github.com/stretchr/testify/assert"
)

type fakeParentCalendarService struct {
	parentEvents []calendarSvc.Event
	parentErr    error
	gotAccountID int64
	gotFrom      timezone.Date
	gotTo        timezone.Date

	respondErr        error
	gotRespondAccount int64
	gotRespondID      int64
	gotRespondStatus  string

	icsFilename       string
	icsContent        string
	icsErr            error
	gotICSAccount     int64
	gotICSAppointment int64

	feedURL          string
	feedWebcal       string
	feedErr          error
	gotFeedAccount   int64
	gotRotateAccount int64
}

func (f *fakeParentCalendarService) ListMyStaffEvents(context.Context, timezone.Date, timezone.Date) ([]calendarSvc.Event, error) {
	return nil, nil
}

func (f *fakeParentCalendarService) ListMyParentEvents(_ context.Context, accountID int64, from, to timezone.Date) ([]calendarSvc.Event, error) {
	f.gotAccountID = accountID
	f.gotFrom = from
	f.gotTo = to
	return f.parentEvents, f.parentErr
}

func (f *fakeParentCalendarService) CreateStaffAppointment(context.Context, calendarSvc.CreateAppointmentRequest) (*calendarSvc.AppointmentDetail, error) {
	return nil, nil
}

func (f *fakeParentCalendarService) GetStaffAppointmentDetail(context.Context, int64) (*calendarSvc.AppointmentDetail, error) {
	return nil, nil
}

func (f *fakeParentCalendarService) UpdateStaffAppointment(context.Context, int64, calendarSvc.UpdateAppointmentRequest) (*calendarSvc.AppointmentDetail, error) {
	return nil, nil
}

func (f *fakeParentCalendarService) CancelStaffAppointment(context.Context, int64) (*calendarSvc.AppointmentDetail, error) {
	return nil, nil
}

func (f *fakeParentCalendarService) DeleteStaffAppointment(context.Context, int64) error {
	return nil
}

func (f *fakeParentCalendarService) CancelStaffAppointmentOccurrence(context.Context, int64, timezone.Date) error {
	return nil
}

func (f *fakeParentCalendarService) StaffAppointmentICS(context.Context, int64) (string, string, error) {
	return "", "", nil
}

func (f *fakeParentCalendarService) ParentAppointmentICS(_ context.Context, accountID, appointmentID int64) (string, string, error) {
	f.gotICSAccount = accountID
	f.gotICSAppointment = appointmentID
	return f.icsFilename, f.icsContent, f.icsErr
}

func (f *fakeParentCalendarService) ParentCalendarFeedURL(_ context.Context, accountID int64) (string, string, error) {
	f.gotFeedAccount = accountID
	return f.feedURL, f.feedWebcal, f.feedErr
}

func (f *fakeParentCalendarService) RotateParentCalendarFeed(_ context.Context, accountID int64) (string, string, error) {
	f.gotRotateAccount = accountID
	return f.feedURL, f.feedWebcal, f.feedErr
}

func (f *fakeParentCalendarService) ParentCalendarFeedByToken(context.Context, string) (string, string, error) {
	return "", "", nil
}

func (f *fakeParentCalendarService) GetStaffAppointmentOverview(context.Context, int64) (*calendarSvc.AppointmentOverview, error) {
	return nil, nil
}

func (f *fakeParentCalendarService) GetParentAppointmentOverview(context.Context, int64, int64) (*calendarSvc.AppointmentOverview, error) {
	return nil, nil
}

func (f *fakeParentCalendarService) RespondToStaffInvitation(context.Context, int64, string) error {
	return nil
}

func (f *fakeParentCalendarService) RespondToParentInvitation(_ context.Context, accountID, recipientID int64, status string) error {
	f.gotRespondAccount = accountID
	f.gotRespondID = recipientID
	f.gotRespondStatus = status
	return f.respondErr
}

func (f *fakeParentCalendarService) RecipientOptions(context.Context, string, int) (*calendarSvc.RecipientOptions, error) {
	return nil, nil
}

func parentRequestWithURLParam(req *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func TestCalendarAppointmentICSStreamsDownload(t *testing.T) {
	service := &fakeParentCalendarService{
		icsFilename: "elternabend.ics",
		icsContent:  "BEGIN:VCALENDAR\r\nEND:VCALENDAR\r\n",
	}
	rs := &Resource{CalendarService: service}
	req := parentRequestWithURLParam(
		withClaims(httptest.NewRequest(http.MethodGet, "/me/calendar/appointments/45/ics", nil), 77),
		"appointmentId", "45",
	)
	w := httptest.NewRecorder()

	rs.calendarAppointmentICS(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/calendar; charset=utf-8", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Header().Get("Content-Disposition"), "elternabend.ics")
	assert.Contains(t, w.Body.String(), "BEGIN:VCALENDAR")
	assert.Equal(t, int64(77), service.gotICSAccount)
	assert.Equal(t, int64(45), service.gotICSAppointment)
}

func TestCalendarFeedURLReturnsURLs(t *testing.T) {
	service := &fakeParentCalendarService{
		feedURL:    "https://parents.test/api/calendar-feed/abc",
		feedWebcal: "webcal://parents.test/api/calendar-feed/abc",
	}
	rs := &Resource{CalendarService: service}
	req := withClaims(httptest.NewRequest(http.MethodGet, "/me/calendar/feed", nil), 77)
	w := httptest.NewRecorder()

	rs.calendarFeedURL(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, int64(77), service.gotFeedAccount)
	assert.Contains(t, w.Body.String(), "webcal://parents.test/api/calendar-feed/abc")
}

func TestRotateCalendarFeedPassesAccount(t *testing.T) {
	service := &fakeParentCalendarService{
		feedURL:    "https://parents.test/api/calendar-feed/new",
		feedWebcal: "webcal://parents.test/api/calendar-feed/new",
	}
	rs := &Resource{CalendarService: service}
	req := withClaims(httptest.NewRequest(http.MethodPost, "/me/calendar/feed/rotate", nil), 77)
	w := httptest.NewRecorder()

	rs.rotateCalendarFeed(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, int64(77), service.gotRotateAccount)
	assert.Contains(t, w.Body.String(), "calendar-feed/new")
}

func TestListMyCalendarRejectsMissingClaims(t *testing.T) {
	rs := &Resource{CalendarService: &fakeParentCalendarService{}}
	req := httptest.NewRequest(http.MethodGet, "/me/calendar?from=2026-01-05&to=2026-01-11", nil)
	w := httptest.NewRecorder()

	rs.listMyCalendar(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestListMyCalendarRejectsMissingCalendarService(t *testing.T) {
	rs := &Resource{}
	req := withClaims(httptest.NewRequest(http.MethodGet, "/me/calendar?from=2026-01-05&to=2026-01-11", nil), 77)
	w := httptest.NewRecorder()

	rs.listMyCalendar(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestListMyCalendarParsesRangeAndAccount(t *testing.T) {
	service := &fakeParentCalendarService{
		parentEvents: []calendarSvc.Event{{ID: "appointment:1:2026-01-05", Source: calModels.EventSourceAppointment, Title: "Parent meeting"}},
	}
	rs := &Resource{CalendarService: service}
	req := withClaims(httptest.NewRequest(http.MethodGet, "/me/calendar?from=2026-01-05&to=2026-01-11", nil), 77)
	w := httptest.NewRecorder()

	rs.listMyCalendar(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, int64(77), service.gotAccountID)
	assert.Equal(t, timezone.NewDate(2026, 1, 5), service.gotFrom)
	assert.Equal(t, timezone.NewDate(2026, 1, 11), service.gotTo)
	assert.Contains(t, w.Body.String(), "Parent meeting")
}

func TestListMyCalendarRejectsBadRange(t *testing.T) {
	rs := &Resource{CalendarService: &fakeParentCalendarService{}}
	req := withClaims(httptest.NewRequest(http.MethodGet, "/me/calendar?from=bad&to=2026-01-11", nil), 77)
	w := httptest.NewRecorder()

	rs.listMyCalendar(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRespondToCalendarInvitationPassesAccountRecipientAndStatus(t *testing.T) {
	service := &fakeParentCalendarService{}
	rs := &Resource{CalendarService: service}
	req := withClaims(
		parentRequestWithURLParam(
			httptest.NewRequest(http.MethodPost, "/me/calendar/recipients/42/response", strings.NewReader(`{"status":"declined"}`)),
			"recipientId",
			"42",
		),
		77,
	)
	w := httptest.NewRecorder()

	rs.respondToCalendarInvitation(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, int64(77), service.gotRespondAccount)
	assert.Equal(t, int64(42), service.gotRespondID)
	assert.Equal(t, calModels.ResponseStatusDeclined, service.gotRespondStatus)
}

func TestRespondToCalendarInvitationRejectsInvalidRecipient(t *testing.T) {
	rs := &Resource{CalendarService: &fakeParentCalendarService{}}
	req := withClaims(
		parentRequestWithURLParam(
			httptest.NewRequest(http.MethodPost, "/me/calendar/recipients/nope/response", strings.NewReader(`{"status":"accepted"}`)),
			"recipientId",
			"nope",
		),
		77,
	)
	w := httptest.NewRecorder()

	rs.respondToCalendarInvitation(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
