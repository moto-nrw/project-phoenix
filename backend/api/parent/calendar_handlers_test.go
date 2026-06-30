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
