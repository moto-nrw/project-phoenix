package calendar

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	calModels "github.com/moto-nrw/project-phoenix/models/calendar"
	calendarSvc "github.com/moto-nrw/project-phoenix/services/calendar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeCalendarService struct {
	listStaffEvents []calendarSvc.Event
	listStaffErr    error
	gotListFrom     timezone.Date
	gotListTo       timezone.Date

	createDetail *calendarSvc.AppointmentDetail
	createErr    error
	gotCreate    calendarSvc.CreateAppointmentRequest

	updateDetail    *calendarSvc.AppointmentDetail
	updateErr       error
	gotUpdateID     int64
	gotUpdate       calendarSvc.UpdateAppointmentRequest
	cancelDetail    *calendarSvc.AppointmentDetail
	cancelErr       error
	gotCancelID     int64
	deleteErr       error
	gotDeleteID     int64
	cancelOccErr    error
	gotCancelOccID  int64
	gotCancelOccDay timezone.Date

	icsFilename string
	icsContent  string
	icsErr      error
	gotICSID    int64

	respondErr       error
	gotRespondID     int64
	gotRespondStatus string

	options       *calendarSvc.RecipientOptions
	optionsErr    error
	gotOptionsQ   string
	gotOptionsLim int
}

func (f *fakeCalendarService) ListMyStaffEvents(_ context.Context, from, to timezone.Date) ([]calendarSvc.Event, error) {
	f.gotListFrom = from
	f.gotListTo = to
	return f.listStaffEvents, f.listStaffErr
}

func (f *fakeCalendarService) ListMyParentEvents(context.Context, int64, timezone.Date, timezone.Date) ([]calendarSvc.Event, error) {
	return nil, nil
}

func (f *fakeCalendarService) CreateStaffAppointment(_ context.Context, req calendarSvc.CreateAppointmentRequest) (*calendarSvc.AppointmentDetail, error) {
	f.gotCreate = req
	return f.createDetail, f.createErr
}

func (f *fakeCalendarService) GetStaffAppointmentDetail(context.Context, int64) (*calendarSvc.AppointmentDetail, error) {
	return nil, nil
}

func (f *fakeCalendarService) UpdateStaffAppointment(_ context.Context, appointmentID int64, req calendarSvc.UpdateAppointmentRequest) (*calendarSvc.AppointmentDetail, error) {
	f.gotUpdateID = appointmentID
	f.gotUpdate = req
	return f.updateDetail, f.updateErr
}

func (f *fakeCalendarService) CancelStaffAppointment(_ context.Context, appointmentID int64) (*calendarSvc.AppointmentDetail, error) {
	f.gotCancelID = appointmentID
	return f.cancelDetail, f.cancelErr
}

func (f *fakeCalendarService) DeleteStaffAppointment(_ context.Context, appointmentID int64) error {
	f.gotDeleteID = appointmentID
	return f.deleteErr
}

func (f *fakeCalendarService) CancelStaffAppointmentOccurrence(_ context.Context, appointmentID int64, occurrenceDate timezone.Date) error {
	f.gotCancelOccID = appointmentID
	f.gotCancelOccDay = occurrenceDate
	return f.cancelOccErr
}

func (f *fakeCalendarService) StaffAppointmentICS(_ context.Context, appointmentID int64) (string, string, error) {
	f.gotICSID = appointmentID
	return f.icsFilename, f.icsContent, f.icsErr
}

func (f *fakeCalendarService) ParentAppointmentICS(context.Context, int64, int64) (string, string, error) {
	return "", "", nil
}

func (f *fakeCalendarService) ParentCalendarFeedURL(context.Context, int64) (string, string, error) {
	return "", "", nil
}

func (f *fakeCalendarService) RotateParentCalendarFeed(context.Context, int64) (string, string, error) {
	return "", "", nil
}

func (f *fakeCalendarService) ParentCalendarFeedByToken(context.Context, string) (string, string, error) {
	return "", "", nil
}

func (f *fakeCalendarService) GetStaffAppointmentOverview(context.Context, int64) (*calendarSvc.AppointmentOverview, error) {
	return nil, nil
}

func (f *fakeCalendarService) GetParentAppointmentOverview(context.Context, int64, int64) (*calendarSvc.AppointmentOverview, error) {
	return nil, nil
}

func (f *fakeCalendarService) RespondToStaffInvitation(_ context.Context, recipientID int64, status string) error {
	f.gotRespondID = recipientID
	f.gotRespondStatus = status
	return f.respondErr
}

func (f *fakeCalendarService) RespondToParentInvitation(context.Context, int64, int64, string) error {
	return nil
}

func (f *fakeCalendarService) RecipientOptions(_ context.Context, query string, limit int) (*calendarSvc.RecipientOptions, error) {
	f.gotOptionsQ = query
	f.gotOptionsLim = limit
	return f.options, f.optionsErr
}

// EnqueueDueAppointmentReminders exists only to satisfy the service interface:
// the guardian reminder scan (#1671) is driven by the scheduler, never by an
// HTTP handler, so no test in this file calls it.
func (f *fakeCalendarService) EnqueueDueAppointmentReminders(context.Context, time.Time, time.Time) (int, error) {
	return 0, nil
}

func requestWithURLParam(req *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func requestWithURLParams(req *http.Request, kv map[string]string) *http.Request {
	rctx := chi.NewRouteContext()
	for key, value := range kv {
		rctx.URLParams.Add(key, value)
	}
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func TestListMyParsesDateRange(t *testing.T) {
	service := &fakeCalendarService{
		listStaffEvents: []calendarSvc.Event{{ID: "appointment:1:2026-01-05", Source: calModels.EventSourceAppointment, Title: "Planning"}},
	}
	rs := &Resource{service: service}
	req := httptest.NewRequest(http.MethodGet, "/my?from=2026-01-05&to=2026-01-11", nil)
	w := httptest.NewRecorder()

	rs.listMy(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, timezone.NewDate(2026, 1, 5), service.gotListFrom)
	assert.Equal(t, timezone.NewDate(2026, 1, 11), service.gotListTo)
	assert.Contains(t, w.Body.String(), "Planning")
}

func TestListMyRejectsMissingRange(t *testing.T) {
	rs := &Resource{service: &fakeCalendarService{}}
	req := httptest.NewRequest(http.MethodGet, "/my?from=2026-01-05", nil)
	w := httptest.NewRecorder()

	rs.listMy(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateAppointmentParsesPayload(t *testing.T) {
	targetID := int64(42)
	service := &fakeCalendarService{
		createDetail: &calendarSvc.AppointmentDetail{
			Appointment: &calModels.Appointment{Title: "Planning"},
		},
	}
	rs := &Resource{service: service}
	body := `{
		"title":"Planning",
		"description":"Weekly sync",
		"location":"Room 1",
		"start_date":"2026-01-05",
		"end_date":"2026-01-05",
		"start_time":"09:15",
		"end_time":"10:30",
		"delivery_mode":"rsvp_required",
		"recurrence":{"frequency":"weekly","interval_count":2,"weekdays":["monday"]},
		"targets":[{"type":"staff","id":42}]
	}`
	req := httptest.NewRequest(http.MethodPost, "/appointments", strings.NewReader(body))
	w := httptest.NewRecorder()

	rs.createAppointment(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "Planning", service.gotCreate.Title)
	assert.Equal(t, timezone.NewDate(2026, 1, 5), service.gotCreate.StartDate)
	assert.Equal(t, "09:15", service.gotCreate.StartTime.Format("15:04"))
	assert.Equal(t, "10:30", service.gotCreate.EndTime.Format("15:04"))
	require.NotNil(t, service.gotCreate.Recurrence)
	assert.Equal(t, calModels.RecurrenceFrequencyWeekly, service.gotCreate.Recurrence.Frequency)
	require.Len(t, service.gotCreate.Targets, 1)
	require.NotNil(t, service.gotCreate.Targets[0].ID)
	assert.Equal(t, targetID, *service.gotCreate.Targets[0].ID)
}

func TestCreateAppointmentRejectsBadClock(t *testing.T) {
	rs := &Resource{service: &fakeCalendarService{}}
	body := `{"title":"Bad","start_date":"2026-01-05","end_date":"2026-01-05","start_time":"9am","end_time":"10:00"}`
	req := httptest.NewRequest(http.MethodPost, "/appointments", strings.NewReader(body))
	w := httptest.NewRecorder()

	rs.createAppointment(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateAppointmentMapsServiceErrors(t *testing.T) {
	rs := &Resource{service: &fakeCalendarService{createErr: calendarSvc.ErrForbidden}}
	body := `{"title":"Forbidden","start_date":"2026-01-05","end_date":"2026-01-05","start_time":"09:00","end_time":"10:00"}`
	req := httptest.NewRequest(http.MethodPost, "/appointments", strings.NewReader(body))
	w := httptest.NewRecorder()

	rs.createAppointment(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestUpdateAppointmentParsesPayload(t *testing.T) {
	service := &fakeCalendarService{
		updateDetail: &calendarSvc.AppointmentDetail{Appointment: &calModels.Appointment{Title: "Planning v2"}},
	}
	rs := &Resource{service: service}
	body := `{
		"title":"Planning v2",
		"location":"Room 2",
		"start_date":"2026-01-06",
		"end_date":"2026-01-06",
		"start_time":"10:00",
		"end_time":"11:00",
		"overview_visibility":"all"
	}`
	req := requestWithURLParam(
		httptest.NewRequest(http.MethodPut, "/appointments/41", strings.NewReader(body)),
		"appointmentId", "41",
	)
	w := httptest.NewRecorder()

	rs.updateAppointment(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, int64(41), service.gotUpdateID)
	assert.Equal(t, "Planning v2", service.gotUpdate.Title)
	assert.Equal(t, timezone.NewDate(2026, 1, 6), service.gotUpdate.StartDate)
	assert.Equal(t, "10:00", service.gotUpdate.StartTime.Format("15:04"))
	assert.Equal(t, "all", service.gotUpdate.OverviewVisibility)
}

func TestUpdateAppointmentRejectsBadClock(t *testing.T) {
	rs := &Resource{service: &fakeCalendarService{}}
	body := `{"title":"x","start_date":"2026-01-06","end_date":"2026-01-06","start_time":"nope","end_time":"11:00"}`
	req := requestWithURLParam(
		httptest.NewRequest(http.MethodPut, "/appointments/41", strings.NewReader(body)),
		"appointmentId", "41",
	)
	w := httptest.NewRecorder()

	rs.updateAppointment(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateAppointmentMapsNotFound(t *testing.T) {
	rs := &Resource{service: &fakeCalendarService{updateErr: calendarSvc.ErrNotFound}}
	body := `{"title":"x","start_date":"2026-01-06","end_date":"2026-01-06","start_time":"10:00","end_time":"11:00"}`
	req := requestWithURLParam(
		httptest.NewRequest(http.MethodPut, "/appointments/41", strings.NewReader(body)),
		"appointmentId", "41",
	)
	w := httptest.NewRecorder()

	rs.updateAppointment(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestCancelAppointmentPassesID(t *testing.T) {
	service := &fakeCalendarService{cancelDetail: &calendarSvc.AppointmentDetail{Appointment: &calModels.Appointment{}}}
	rs := &Resource{service: service}
	req := requestWithURLParam(
		httptest.NewRequest(http.MethodPost, "/appointments/42/cancel", nil),
		"appointmentId", "42",
	)
	w := httptest.NewRecorder()

	rs.cancelAppointment(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, int64(42), service.gotCancelID)
}

func TestCancelAppointmentMapsForbidden(t *testing.T) {
	rs := &Resource{service: &fakeCalendarService{cancelErr: calendarSvc.ErrForbidden}}
	req := requestWithURLParam(
		httptest.NewRequest(http.MethodPost, "/appointments/42/cancel", nil),
		"appointmentId", "42",
	)
	w := httptest.NewRecorder()

	rs.cancelAppointment(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestDeleteAppointmentPassesID(t *testing.T) {
	service := &fakeCalendarService{}
	rs := &Resource{service: service}
	req := requestWithURLParam(
		httptest.NewRequest(http.MethodDelete, "/appointments/11", nil),
		"appointmentId", "11",
	)
	w := httptest.NewRecorder()

	rs.deleteAppointment(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, int64(11), service.gotDeleteID)
}

func TestCancelOccurrenceParsesIDAndDate(t *testing.T) {
	service := &fakeCalendarService{}
	rs := &Resource{service: service}
	req := requestWithURLParams(
		httptest.NewRequest(http.MethodPost, "/appointments/11/occurrences/2026-01-12/cancel", nil),
		map[string]string{"appointmentId": "11", "occurrenceDate": "2026-01-12"},
	)
	w := httptest.NewRecorder()

	rs.cancelOccurrence(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, int64(11), service.gotCancelOccID)
	assert.Equal(t, timezone.NewDate(2026, 1, 12), service.gotCancelOccDay)
}

func TestCancelOccurrenceRejectsBadDate(t *testing.T) {
	rs := &Resource{service: &fakeCalendarService{}}
	req := requestWithURLParams(
		httptest.NewRequest(http.MethodPost, "/appointments/11/occurrences/nope/cancel", nil),
		map[string]string{"appointmentId": "11", "occurrenceDate": "nope"},
	)
	w := httptest.NewRecorder()

	rs.cancelOccurrence(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAppointmentICSStreamsDownload(t *testing.T) {
	service := &fakeCalendarService{
		icsFilename: "planning.ics",
		icsContent:  "BEGIN:VCALENDAR\r\nEND:VCALENDAR\r\n",
	}
	rs := &Resource{service: service}
	req := requestWithURLParam(
		httptest.NewRequest(http.MethodGet, "/appointments/7/ics", nil),
		"appointmentId", "41",
	)
	w := httptest.NewRecorder()

	rs.appointmentICS(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/calendar; charset=utf-8", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Header().Get("Content-Disposition"), "planning.ics")
	assert.Contains(t, w.Body.String(), "BEGIN:VCALENDAR")
	assert.Equal(t, int64(41), service.gotICSID)
}

func TestRespondPassesRecipientAndStatus(t *testing.T) {
	service := &fakeCalendarService{}
	rs := &Resource{service: service}
	req := requestWithURLParam(
		httptest.NewRequest(http.MethodPost, "/recipients/42/response", strings.NewReader(`{"status":"accepted"}`)),
		"recipientId",
		"42",
	)
	w := httptest.NewRecorder()

	rs.respond(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, int64(42), service.gotRespondID)
	assert.Equal(t, calModels.ResponseStatusAccepted, service.gotRespondStatus)
}

func TestRespondRejectsInvalidRecipientID(t *testing.T) {
	rs := &Resource{service: &fakeCalendarService{}}
	req := requestWithURLParam(
		httptest.NewRequest(http.MethodPost, "/recipients/nope/response", strings.NewReader(`{"status":"accepted"}`)),
		"recipientId",
		"nope",
	)
	w := httptest.NewRecorder()

	rs.respond(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRecipientOptionsParsesQueryAndLimit(t *testing.T) {
	service := &fakeCalendarService{
		options: &calendarSvc.RecipientOptions{
			Classes: []string{"1a"},
		},
	}
	rs := &Resource{service: service}
	req := httptest.NewRequest(http.MethodGet, "/recipient-options?q=anna&limit=12", nil)
	w := httptest.NewRecorder()

	rs.recipientOptions(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "anna", service.gotOptionsQ)
	assert.Equal(t, 12, service.gotOptionsLim)
	assert.Contains(t, w.Body.String(), "1a")
}

func TestRecipientOptionsMapsUnknownError(t *testing.T) {
	rs := &Resource{service: &fakeCalendarService{optionsErr: errors.New("boom")}}
	req := httptest.NewRequest(http.MethodGet, "/recipient-options", nil)
	w := httptest.NewRecorder()

	rs.recipientOptions(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
