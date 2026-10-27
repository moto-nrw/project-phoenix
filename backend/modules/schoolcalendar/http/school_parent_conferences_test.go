package staffcalendarhttp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	calendarSvc "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal"
)

func (f *fakeCalendarService) ListSchoolParentConferences(context.Context) (*calendarSvc.SchoolParentConferenceList, error) {
	return &calendarSvc.SchoolParentConferenceList{Conferences: []calendarSvc.ParentConferenceView{}, Classes: []string{"1a"}}, f.school.writeErr
}

func (f *fakeCalendarService) GetSchoolParentConference(_ context.Context, id int64) (*calendarSvc.ParentConferenceView, error) {
	f.school.gotConferenceID = id
	return &calendarSvc.ParentConferenceView{ID: "5"}, f.school.writeErr
}

func (f *fakeCalendarService) CreateSchoolParentConference(_ context.Context, req calendarSvc.ParentConferenceRequest) (*calendarSvc.ParentConferenceView, error) {
	f.school.gotConference = req
	return &calendarSvc.ParentConferenceView{ID: "5", SchoolClass: req.SchoolClass}, f.school.writeErr
}

func (f *fakeCalendarService) UpdateSchoolParentConference(_ context.Context, id int64, req calendarSvc.ParentConferenceRequest) (*calendarSvc.ParentConferenceView, error) {
	f.school.gotConferenceID, f.school.gotConference = id, req
	return &calendarSvc.ParentConferenceView{ID: "5"}, f.school.writeErr
}

func (f *fakeCalendarService) DeleteSchoolParentConference(_ context.Context, id int64) error {
	f.school.gotConferenceID = id
	return f.school.writeErr
}

func (f *fakeCalendarService) ListChildParentConferences(context.Context, calendarSvc.ParentConferenceChild) ([]calendarSvc.ParentConferenceOffer, error) {
	return nil, nil
}

func (f *fakeCalendarService) BookParentConference(context.Context, calendarSvc.ParentConferenceChild, int64, string) (*calendarSvc.ParentConferenceOffer, error) {
	return nil, nil
}

func (f *fakeCalendarService) CancelParentConferenceBooking(context.Context, calendarSvc.ParentConferenceChild, int64) error {
	return nil
}

func TestSchoolParentConferenceRoutes(t *testing.T) {
	t.Parallel()
	service := &fakeCalendarService{}
	rs := &Resource{service: service}

	body := `{"school_class":"1a","date":"2026-11-12","start_time":"14:00","end_time":"16:00","slot_minutes":15,"breaks":[{"start_time":"15:00","end_time":"15:15"}]}`
	w := httptest.NewRecorder()
	rs.createSchoolParentConference(w, httptest.NewRequest(http.MethodPost, "/parent-conferences", strings.NewReader(body)))
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	assert.Equal(t, "1a", service.school.gotConference.SchoolClass)
	assert.Equal(t, "2026-11-12", service.school.gotConference.Date.String())
	assert.Equal(t, []calendarSvc.ParentConferenceWindow{{StartTime: "15:00", EndTime: "15:15"}}, service.school.gotConference.Breaks)

	w = httptest.NewRecorder()
	rs.deleteSchoolParentConference(w, conferenceIDRequest(http.MethodDelete, "5"))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, int64(5), service.school.gotConferenceID)

	service.school.writeErr = calendarSvc.ErrForbidden
	w = httptest.NewRecorder()
	rs.createSchoolParentConference(w, httptest.NewRequest(http.MethodPost, "/parent-conferences", strings.NewReader(body)))
	assert.Equal(t, http.StatusForbidden, w.Code, "a class the Lehrkraft does not teach")

	service.school.writeErr = calendarSvc.ErrNotFound
	w = httptest.NewRecorder()
	rs.getSchoolParentConference(w, conferenceIDRequest(http.MethodGet, "6"))
	assert.Equal(t, http.StatusNotFound, w.Code, "another Lehrkraft's day")
}

func conferenceIDRequest(method, id string) *http.Request {
	req := httptest.NewRequest(method, "/parent-conferences/"+id, nil)
	route := chi.NewRouteContext()
	route.URLParams.Add("conferenceId", id)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, route))
}
