package calendar

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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

func requestWithURLParam(req *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
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
		"targets":[{"type":"staff","id":7}]
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
	assert.Equal(t, int64(7), *service.gotCreate.Targets[0].ID)
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
