package parent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	calendarService "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	parentService "github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

func (f *fakeParentService) ParentConferenceChild(_ context.Context, _, _ int64, book bool) (*parentService.Child, error) {
	f.gotConferenceBook = append(f.gotConferenceBook, book)
	return f.conferenceChild, f.conferenceChildErr
}

// fakeConferences records what the Elternsprechtag routes handed to the
// calendar (#4801).
type fakeConferences struct {
	gotChild    calendarService.ParentConferenceChild
	gotID       int64
	gotStart    string
	calls       int
	bookErr     error
	listOffers  []calendarService.ParentConferenceOffer
	cancelCalls int
}

func (f *fakeConferences) ListChildParentConferences(_ context.Context, child calendarService.ParentConferenceChild) ([]calendarService.ParentConferenceOffer, error) {
	f.calls++
	f.gotChild = child
	return f.listOffers, nil
}

func (f *fakeConferences) BookParentConference(_ context.Context, child calendarService.ParentConferenceChild, id int64, start string) (*calendarService.ParentConferenceOffer, error) {
	f.calls++
	f.gotChild, f.gotID, f.gotStart = child, id, start
	if f.bookErr != nil {
		return nil, f.bookErr
	}
	return &calendarService.ParentConferenceOffer{ID: "7", Booking: &calendarService.ParentConferenceWindow{StartTime: start, EndTime: "14:15"}}, nil
}

func (f *fakeConferences) CancelParentConferenceBooking(_ context.Context, child calendarService.ParentConferenceChild, id int64) error {
	f.cancelCalls++
	f.gotChild, f.gotID = child, id
	return f.bookErr
}

// conferenceRequest carries the tenant runtime the flow opens the school's
// transaction with.
func conferenceRequest(t *testing.T, method, body string) *http.Request {
	t.Helper()
	req := parentRequestWithStudentID(method, "/me/children/42/parent-conferences/7/booking", body, "42")
	chi.RouteContext(req.Context()).URLParams.Add("conferenceId", "7")
	return req.WithContext(tenant.WithUnitOfWork(req.Context(), testpkg.TenantRuntime(t, testpkg.SetupTestDB(t))))
}

func TestParentConferenceRoutesNeedTheRelationship(t *testing.T) {
	t.Parallel()
	fake := &fakeParentService{conferenceChildErr: parentService.ErrChildNotLinked}
	conferences := &fakeConferences{}
	rs := &Resource{ParentService: fake, Conferences: conferences}

	w := httptest.NewRecorder()
	rs.listChildParentConferences(w, parentRequestWithStudentID(http.MethodGet, "/me/children/42/parent-conferences", "", "42"))
	assert.Equal(t, http.StatusForbidden, w.Code, "a parent without a relationship sees nothing")
	assert.Contains(t, w.Body.String(), `"code":"care.child_not_linked"`)

	w = httptest.NewRecorder()
	rs.bookChildParentConference(w, conferenceRequest(t, http.MethodPut, `{"start_time":"14:00"}`))
	assert.Equal(t, http.StatusForbidden, w.Code, "and books nothing")
	assert.Zero(t, conferences.calls, "the calendar is never asked")
	assert.Equal(t, []bool{false, true}, fake.gotConferenceBook, "seeing needs access, booking the booking permission")
}

func TestParentConferenceBookingWithoutBookPermissionIsForbidden(t *testing.T) {
	t.Parallel()
	fake := &fakeParentService{conferenceChildErr: parentService.ErrGuardianPermissionDenied}
	conferences := &fakeConferences{}
	rs := &Resource{ParentService: fake, Conferences: conferences}

	w := httptest.NewRecorder()
	rs.cancelChildParentConference(w, conferenceRequest(t, http.MethodDelete, ""))
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), `"code":"care.guardian_permission_denied"`)
	assert.Zero(t, conferences.cancelCalls)
}

func TestParentConferenceBookingPassesTheResolvedChild(t *testing.T) {
	t.Parallel()
	fake := &fakeParentService{conferenceChild: &parentService.Child{TenantID: testpkg.Tenant(t), GuardianProfileID: 9}}
	conferences := &fakeConferences{}
	rs := &Resource{ParentService: fake, Conferences: conferences}

	w := httptest.NewRecorder()
	rs.bookChildParentConference(w, conferenceRequest(t, http.MethodPut, `{"start_time":"14:00"}`))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, calendarService.ParentConferenceChild{TenantID: testpkg.Tenant(t), StudentID: 42, GuardianProfileID: 9}, conferences.gotChild)
	assert.Equal(t, int64(7), conferences.gotID)
	assert.Equal(t, "14:00", conferences.gotStart)
	assert.True(t, strings.Contains(w.Body.String(), `"start_time":"14:00"`))
}

func TestParentConferenceBookingConflictsCarryTheirCodes(t *testing.T) {
	t.Parallel()
	for err, code := range map[error]string{
		calendarService.ErrParentConferenceSlotTaken:    "school.parent_conference_slot_taken",
		calendarService.ErrParentConferenceChangeClosed: "school.parent_conference_change_closed",
	} {
		fake := &fakeParentService{conferenceChild: &parentService.Child{TenantID: testpkg.Tenant(t), GuardianProfileID: 9}}
		rs := &Resource{ParentService: fake, Conferences: &fakeConferences{bookErr: err}}
		w := httptest.NewRecorder()
		rs.bookChildParentConference(w, conferenceRequest(t, http.MethodPut, `{"start_time":"14:00"}`))
		assert.Equal(t, http.StatusConflict, w.Code)
		assert.Contains(t, w.Body.String(), `"code":"`+code+`"`)
	}
}
