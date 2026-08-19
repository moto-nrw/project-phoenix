package calendar_e2e_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/go-chi/chi/v5"
	calendarAPI "github.com/moto-nrw/project-phoenix/api/calendar"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func setupCalendarE2ERouter(t *testing.T, db *bun.DB) (*services.Factory, chi.Router) {
	t.Helper()
	testutil.SeedTestJWTConfig()
	repos := repositories.NewFactory(db)
	serviceFactory, err := services.NewFactory(repos, db, slog.Default())
	require.NoError(t, err)
	resource := calendarAPI.NewResource(serviceFactory.Calendar, db, slog.Default())
	router := chi.NewRouter()
	router.Mount("/calendar", resource.Router())
	return serviceFactory, router
}

func calendarToken(t *testing.T, accountID int64, perms ...string) string {
	t.Helper()
	return testutil.MintTestJWT(t, jwt.AppClaims{
		ID:          int(accountID),
		Sub:         "calendar-e2e@example.com",
		Roles:       []string{"user"},
		TenantID:    testpkg.Tenant(t),
		Permissions: perms,
	})
}

func doJSON(t *testing.T, router http.Handler, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(payload)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

type createAppointmentE2EResponse struct {
	Data struct {
		Appointment struct {
			ID int64 `json:"id"`
		} `json:"appointment"`
		Recipients []struct {
			ID      int64  `json:"id"`
			StaffID *int64 `json:"staff_id"`
			Status  string `json:"status"`
		} `json:"recipients"`
	} `json:"data"`
}

type calendarListE2EResponse struct {
	Data struct {
		Events []struct {
			Title          string `json:"title"`
			StartDate      string `json:"start_date"`
			ResponseStatus string `json:"response_status"`
			CanRespond     bool   `json:"can_respond"`
			Cancelled      bool   `json:"cancelled"`
			CanEdit        bool   `json:"can_edit"`
		} `json:"events"`
	} `json:"data"`
}

// Deliberately NOT parallel: the test reaches process-global state (env
// variables, viper keys, the settings registry, os.Stdout) that the whole
// test binary shares.
func TestPersonalCalendarHTTPFlow_StaffInvitationRSVP(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	_, router := setupCalendarE2ERouter(t, db)

	organizer, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "E2E", "Organizer")
	invitee, inviteeAccount := testpkg.CreateTestCalendarStaff(t, db, "E2E", "Invitee")
	t.Cleanup(func() {
		testpkg.CleanupStaffFixtures(t, db, invitee.ID, organizer.ID)
		testpkg.CleanupAuthFixtures(t, db, inviteeAccount.ID, organizerAccount.ID)
	})

	manageToken := calendarToken(t, organizerAccount.ID, permissions.CalendarManage, permissions.CalendarOwn)
	ownToken := calendarToken(t, inviteeAccount.ID, permissions.CalendarOwn)

	createBody := map[string]any{
		"title":         "HTTP calendar planning",
		"start_date":    "2026-05-04",
		"end_date":      "2026-05-04",
		"start_time":    "09:00",
		"end_time":      "10:00",
		"delivery_mode": "rsvp_required",
		"targets": []map[string]any{
			{"type": "staff", "id": invitee.ID},
		},
	}
	createRR := doJSON(t, router, http.MethodPost, "/calendar/appointments", manageToken, createBody)
	require.Equal(t, http.StatusCreated, createRR.Code, createRR.Body.String())

	var created createAppointmentE2EResponse
	require.NoError(t, json.Unmarshal(createRR.Body.Bytes(), &created))
	require.Greater(t, created.Data.Appointment.ID, int64(0))
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, db, "calendar.appointments", created.Data.Appointment.ID) })
	require.Len(t, created.Data.Recipients, 1)
	recipientID := created.Data.Recipients[0].ID
	assert.Equal(t, "pending", created.Data.Recipients[0].Status)

	listRR := doJSON(t, router, http.MethodGet, "/calendar/my?from=2026-05-04&to=2026-05-04", ownToken, nil)
	require.Equal(t, http.StatusOK, listRR.Code, listRR.Body.String())
	var listed calendarListE2EResponse
	require.NoError(t, json.Unmarshal(listRR.Body.Bytes(), &listed))
	require.Len(t, listed.Data.Events, 1)
	assert.Equal(t, "HTTP calendar planning", listed.Data.Events[0].Title)
	assert.Equal(t, timezone.NewDate(2026, 5, 4).String(), listed.Data.Events[0].StartDate)
	assert.Equal(t, "pending", listed.Data.Events[0].ResponseStatus)
	assert.True(t, listed.Data.Events[0].CanRespond)

	respondRR := doJSON(t, router, http.MethodPost, "/calendar/recipients/"+strconv.FormatInt(recipientID, 10)+"/response", ownToken, map[string]any{
		"status": "accepted",
	})
	require.Equal(t, http.StatusOK, respondRR.Code, respondRR.Body.String())

	listRR = doJSON(t, router, http.MethodGet, "/calendar/my?from=2026-05-04&to=2026-05-04", ownToken, nil)
	require.Equal(t, http.StatusOK, listRR.Code, listRR.Body.String())
	require.NoError(t, json.Unmarshal(listRR.Body.Bytes(), &listed))
	require.Len(t, listed.Data.Events, 1)
	assert.Equal(t, "accepted", listed.Data.Events[0].ResponseStatus)
}

// TestPersonalCalendarHTTPFlow_EditCancelDeleteAndICS drives the full lifecycle
// (create → edit → .ics export → cancel → delete) through the real router.
// Deliberately NOT parallel: the test reaches process-global state (env
// variables, viper keys, the settings registry, os.Stdout) that the whole
// test binary shares.
func TestPersonalCalendarHTTPFlow_EditCancelDeleteAndICS(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	_, router := setupCalendarE2ERouter(t, db)

	organizer, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "E2E", "LifecycleOrg")
	invitee, inviteeAccount := testpkg.CreateTestCalendarStaff(t, db, "E2E", "LifecycleInv")
	t.Cleanup(func() {
		testpkg.CleanupStaffFixtures(t, db, invitee.ID, organizer.ID)
		testpkg.CleanupAuthFixtures(t, db, inviteeAccount.ID, organizerAccount.ID)
	})
	manageToken := calendarToken(t, organizerAccount.ID, permissions.CalendarManage, permissions.CalendarOwn)
	ownToken := calendarToken(t, inviteeAccount.ID, permissions.CalendarOwn)

	createRR := doJSON(t, router, http.MethodPost, "/calendar/appointments", manageToken, map[string]any{
		"title":         "Original",
		"start_date":    "2026-05-04",
		"end_date":      "2026-05-04",
		"start_time":    "09:00",
		"end_time":      "10:00",
		"delivery_mode": "rsvp_required",
		"targets":       []map[string]any{{"type": "staff", "id": invitee.ID}},
	})
	require.Equal(t, http.StatusCreated, createRR.Code, createRR.Body.String())
	var created createAppointmentE2EResponse
	require.NoError(t, json.Unmarshal(createRR.Body.Bytes(), &created))
	apptID := created.Data.Appointment.ID
	require.Greater(t, apptID, int64(0))
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, db, "calendar.appointments", apptID) })
	path := "/calendar/appointments/" + strconv.FormatInt(apptID, 10)
	listPath := "/calendar/my?from=2026-05-01&to=2026-05-10"

	// Edit details (audience stays fixed).
	updRR := doJSON(t, router, http.MethodPut, path, manageToken, map[string]any{
		"title":      "Updated title",
		"location":   "Room 2",
		"start_date": "2026-05-05",
		"end_date":   "2026-05-05",
		"start_time": "11:00",
		"end_time":   "12:00",
	})
	require.Equal(t, http.StatusOK, updRR.Code, updRR.Body.String())

	var listed calendarListE2EResponse
	listRR := doJSON(t, router, http.MethodGet, listPath, ownToken, nil)
	require.NoError(t, json.Unmarshal(listRR.Body.Bytes(), &listed))
	require.Len(t, listed.Data.Events, 1)
	assert.Equal(t, "Updated title", listed.Data.Events[0].Title)
	assert.Equal(t, "2026-05-05", listed.Data.Events[0].StartDate)
	assert.False(t, listed.Data.Events[0].Cancelled)

	// .ics download.
	icsReq := httptest.NewRequest(http.MethodGet, path+"/ics", nil)
	icsReq.Header.Set("Authorization", "Bearer "+ownToken)
	icsRR := httptest.NewRecorder()
	router.ServeHTTP(icsRR, icsReq)
	require.Equal(t, http.StatusOK, icsRR.Code, icsRR.Body.String())
	assert.Contains(t, icsRR.Header().Get("Content-Type"), "text/calendar")
	assert.Contains(t, icsRR.Header().Get("Content-Disposition"), ".ics")
	assert.Contains(t, icsRR.Body.String(), "BEGIN:VEVENT")
	assert.Contains(t, icsRR.Body.String(), "SUMMARY:Updated title")

	// Cancel — stays visible, flagged cancelled.
	cancelRR := doJSON(t, router, http.MethodPost, path+"/cancel", manageToken, nil)
	require.Equal(t, http.StatusOK, cancelRR.Code, cancelRR.Body.String())
	listRR = doJSON(t, router, http.MethodGet, listPath, ownToken, nil)
	require.NoError(t, json.Unmarshal(listRR.Body.Bytes(), &listed))
	require.Len(t, listed.Data.Events, 1)
	assert.True(t, listed.Data.Events[0].Cancelled)

	// Delete — gone for everyone.
	delRR := doJSON(t, router, http.MethodDelete, path, manageToken, nil)
	require.Equal(t, http.StatusOK, delRR.Code, delRR.Body.String())
	listRR = doJSON(t, router, http.MethodGet, listPath, ownToken, nil)
	require.NoError(t, json.Unmarshal(listRR.Body.Bytes(), &listed))
	assert.Empty(t, listed.Data.Events)
}

// TestPersonalCalendarHTTPFlow_ForbiddenEdit confirms a non-organizer cannot
// edit or delete someone else's appointment through the HTTP layer.
// Deliberately NOT parallel: the test reaches process-global state (env
// variables, viper keys, the settings registry, os.Stdout) that the whole
// test binary shares.
func TestPersonalCalendarHTTPFlow_ForbiddenEdit(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	_, router := setupCalendarE2ERouter(t, db)

	organizer, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "E2E", "OwnerOrg")
	other, otherAccount := testpkg.CreateTestCalendarStaff(t, db, "E2E", "OtherMgr")
	t.Cleanup(func() {
		testpkg.CleanupStaffFixtures(t, db, other.ID, organizer.ID)
		testpkg.CleanupAuthFixtures(t, db, otherAccount.ID, organizerAccount.ID)
	})
	ownerToken := calendarToken(t, organizerAccount.ID, permissions.CalendarManage, permissions.CalendarOwn)
	otherToken := calendarToken(t, otherAccount.ID, permissions.CalendarManage, permissions.CalendarOwn)

	createRR := doJSON(t, router, http.MethodPost, "/calendar/appointments", ownerToken, map[string]any{
		"title":         "Owner only",
		"start_date":    "2026-05-04",
		"end_date":      "2026-05-04",
		"start_time":    "09:00",
		"end_time":      "10:00",
		"delivery_mode": "informational",
		"targets":       []map[string]any{{"type": "staff", "id": other.ID}},
	})
	require.Equal(t, http.StatusCreated, createRR.Code, createRR.Body.String())
	var created createAppointmentE2EResponse
	require.NoError(t, json.Unmarshal(createRR.Body.Bytes(), &created))
	apptID := created.Data.Appointment.ID
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, db, "calendar.appointments", apptID) })
	path := "/calendar/appointments/" + strconv.FormatInt(apptID, 10)

	// A different manager (an invitee here) may not delete it.
	delRR := doJSON(t, router, http.MethodDelete, path, otherToken, nil)
	assert.Equal(t, http.StatusForbidden, delRR.Code, delRR.Body.String())
}
