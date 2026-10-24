package staffcalendarhttp

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	calendarSvc "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// schoolCalls records what the moto schule calendar routes handed to the
// service (#4798).
type schoolCalls struct {
	view        *calendarSvc.SchoolCalendarView
	viewErr     error
	gotFrom     calendarSvc.Date
	gotTo       calendarSvc.Date
	writeErr    error
	gotCreate   calendarSvc.SchoolAppointmentRequest
	gotUpdateID int64
	gotUpdate   calendarSvc.SchoolAppointmentRequest
	gotDeleteID int64
}

func (f *fakeCalendarService) ListSchoolCalendar(_ context.Context, from, to calendarSvc.Date) (*calendarSvc.SchoolCalendarView, error) {
	f.school.gotFrom, f.school.gotTo = from, to
	return f.school.view, f.school.viewErr
}

func (f *fakeCalendarService) CreateSchoolAppointment(_ context.Context, req calendarSvc.SchoolAppointmentRequest) (*calendarSvc.AppointmentDetail, error) {
	f.school.gotCreate = req
	return &calendarSvc.AppointmentDetail{}, f.school.writeErr
}

func (f *fakeCalendarService) UpdateSchoolAppointment(_ context.Context, appointmentID int64, req calendarSvc.SchoolAppointmentRequest) (*calendarSvc.AppointmentDetail, error) {
	f.school.gotUpdateID, f.school.gotUpdate = appointmentID, req
	return &calendarSvc.AppointmentDetail{}, f.school.writeErr
}

func (f *fakeCalendarService) DeleteSchoolAppointment(_ context.Context, appointmentID int64) error {
	f.school.gotDeleteID = appointmentID
	return f.school.writeErr
}

func (f *fakeCalendarService) RespondToSchoolInvitation(ctx context.Context, recipientID int64, status string) error {
	return f.RespondToStaffInvitation(ctx, recipientID, status)
}

func (f *fakeCalendarService) SchoolCalendarFeedURL(ctx context.Context) (string, string, error) {
	return f.StaffCalendarFeedURL(ctx)
}

func (f *fakeCalendarService) RotateSchoolCalendarFeed(ctx context.Context) (string, string, error) {
	return f.RotateStaffCalendarFeed(ctx)
}

func TestSchoolCalendarListsEventsWithClosingDaysAndHolidays(t *testing.T) {
	t.Parallel()

	eventType := "ausflug"
	service := &fakeCalendarService{school: schoolCalls{view: &calendarSvc.SchoolCalendarView{
		Events:      []calendarSvc.Event{{ID: "appointment:1:2026-03-02", Title: "Zoo", SchoolEventType: &eventType}},
		ClosingDays: []calendarSvc.SchoolDays{{ID: "4", Name: "Pädagogischer Tag", StartDate: "2026-03-04", EndDate: "2026-03-04"}},
		Holidays:    []calendarSvc.SchoolDays{{ID: "5", Name: "Osterferien", StartDate: "2026-03-30", EndDate: "2026-04-10"}},
		Classes:     []string{"3a"},
	}}}
	rs := &Resource{service: service}
	w := httptest.NewRecorder()

	rs.listSchoolCalendar(w, httptest.NewRequest(http.MethodGet, "/my?from=2026-03-01&to=2026-04-11", nil))

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, calendarSvc.Date(timezone.NewDate(2026, 3, 1)), service.school.gotFrom)
	assert.Equal(t, calendarSvc.Date(timezone.NewDate(2026, 4, 11)), service.school.gotTo)
	body := w.Body.String()
	assert.Contains(t, body, `"school_event_type":"ausflug"`)
	assert.Contains(t, body, `"closing_days":[{"id":"4","name":"Pädagogischer Tag"`)
	assert.Contains(t, body, `"holidays":[{"id":"5","name":"Osterferien"`)
	assert.Contains(t, body, `"classes":["3a"]`)
}

func TestSchoolCalendarRejectsAMissingRange(t *testing.T) {
	t.Parallel()

	service := &fakeCalendarService{}
	rs := &Resource{service: service}
	w := httptest.NewRecorder()

	rs.listSchoolCalendar(w, httptest.NewRequest(http.MethodGet, "/my?from=2026-03-01", nil))

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.True(t, service.school.gotFrom.IsZero(), "the service is not asked without a full range")
}

func TestCreateSchoolAppointmentPassesTypeAndParents(t *testing.T) {
	t.Parallel()

	service := &fakeCalendarService{}
	rs := &Resource{service: service}
	body := `{"title":"Wandertag","start_date":"2026-05-06","end_date":"2026-05-06","start_time":"08:00","end_time":"12:30",` +
		`"school_event_type":"ausflug","parents":"classes","classes":["3a"]}`
	w := httptest.NewRecorder()

	rs.createSchoolAppointment(w, httptest.NewRequest(http.MethodPost, "/appointments", strings.NewReader(body)))

	require.Equal(t, http.StatusCreated, w.Code)
	got := service.school.gotCreate
	assert.Equal(t, "Wandertag", got.Title)
	assert.Equal(t, "ausflug", got.EventType)
	assert.Equal(t, calendarSvc.SchoolParentsClasses, got.Parents)
	assert.Equal(t, []string{"3a"}, got.Classes)
	assert.Equal(t, "08:00", got.StartTime.Format("15:04"))
	assert.Equal(t, "12:30", got.EndTime.Format("15:04"))
}

func TestCreateSchoolAppointmentRejectsAnInvalidClock(t *testing.T) {
	t.Parallel()

	service := &fakeCalendarService{}
	rs := &Resource{service: service}
	body := `{"title":"Wandertag","start_date":"2026-05-06","start_time":"8 Uhr","end_time":"12:30","school_event_type":"ausflug"}`
	w := httptest.NewRecorder()

	rs.createSchoolAppointment(w, httptest.NewRequest(http.MethodPost, "/appointments", strings.NewReader(body)))

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Empty(t, service.school.gotCreate.Title, "an invalid clock never reaches the service")
}

func TestUpdateSchoolAppointmentOfAnotherLehrkraftIsForbidden(t *testing.T) {
	t.Parallel()

	service := &fakeCalendarService{school: schoolCalls{writeErr: errors.Join(calendarSvc.ErrForbidden, errors.New("only the organizer"))}}
	rs := &Resource{service: service}
	body := `{"title":"Wandertag","start_date":"2026-05-06","start_time":"08:00","end_time":"12:30","school_event_type":"ausflug"}`
	const appointmentID = "7"
	req := requestWithURLParams(httptest.NewRequest(http.MethodPut, "/appointments/"+appointmentID, strings.NewReader(body)), map[string]string{"appointmentId": appointmentID})
	w := httptest.NewRecorder()

	rs.updateSchoolAppointment(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Equal(t, appointmentID, strconv.FormatInt(service.school.gotUpdateID, 10))
}

func TestDeleteSchoolAppointmentReachesTheService(t *testing.T) {
	t.Parallel()

	service := &fakeCalendarService{}
	rs := &Resource{service: service}
	const appointmentID = "9"
	req := requestWithURLParams(httptest.NewRequest(http.MethodDelete, "/appointments/"+appointmentID, nil), map[string]string{"appointmentId": appointmentID})
	w := httptest.NewRecorder()

	rs.deleteSchoolAppointment(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, appointmentID, strconv.FormatInt(service.school.gotDeleteID, 10))
}
