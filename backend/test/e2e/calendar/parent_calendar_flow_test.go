package calendar_e2e_test

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/go-chi/chi/v5"
	calendarAPI "github.com/moto-nrw/project-phoenix/api/calendar"
	parentAPI "github.com/moto-nrw/project-phoenix/api/parent"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// setupParentE2ERouter mounts both the staff calendar router (to create
// appointments) and the parent router (to read them as a guardian) on one
// router, so a single test can drive the full staff→parent flow over HTTP.
func setupParentE2ERouter(t *testing.T, db *bun.DB) chi.Router {
	t.Helper()
	testutil.SeedTestJWTConfig()
	repos := repositories.NewFactory(db)
	factory, err := services.NewFactory(repos, db, slog.Default())
	require.NoError(t, err)

	staffResource := calendarAPI.NewResource(factory.Calendar, db, slog.Default())
	parentResource := parentAPI.NewResource(
		factory.Auth,
		factory.Parent,
		factory.EnrollmentRequest,
		factory.GuardianProfileLoader,
		factory.Schools,
		db,
	)
	parentResource.SetCalendarService(factory.Calendar)

	router := chi.NewRouter()
	router.Mount("/calendar", staffResource.Router())
	router.Mount("/parent", parentResource.Router())
	return router
}

func parentCalendarToken(t *testing.T, accountID int64) string {
	t.Helper()
	return testutil.MintTestJWT(t, jwt.AppClaims{
		ID:    int(accountID),
		Sub:   "parent-e2e@example.com",
		Roles: []string{"guardian"},
		Scope: tenant.ScopeParent,
	})
}

type feedE2EResponse struct {
	Data struct {
		URL       string `json:"url"`
		WebcalURL string `json:"webcal_url"`
	} `json:"data"`
}

// TestParentCalendarHTTPFlow_ViewICSAndFeed drives the parent side end to end:
// a staff member creates an appointment for a guardian, then the guardian views
// it, downloads its .ics, and fetches their subscription feed URL — all through
// the real routers with a parent-scope JWT.
// Deliberately NOT parallel: the test reaches process-global state (env
// variables, viper keys, the settings registry, os.Stdout) that the whole
// test binary shares.
func TestParentCalendarHTTPFlow_ViewICSAndFeed(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	router := setupParentE2ERouter(t, db)

	organizer, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "E2E", "ParentFlowOrg")
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	t.Cleanup(func() {
		testpkg.CleanupParentGuardianChain(t, db, chain)
		testpkg.CleanupStaffFixtures(t, db, organizer.ID)
		testpkg.CleanupAuthFixtures(t, db, organizerAccount.ID)
	})

	manageToken := calendarToken(t, organizerAccount.ID, permissions.CalendarManage, permissions.CalendarOwn)
	createRR := doJSON(t, router, http.MethodPost, "/calendar/appointments", manageToken, map[string]any{
		"title":         "Elternabend",
		"start_date":    "2026-05-04",
		"end_date":      "2026-05-04",
		"start_time":    "18:00",
		"end_time":      "19:30",
		"delivery_mode": "informational",
		"targets":       []map[string]any{{"type": "guardian_profile", "id": chain.GuardianProfileID}},
	})
	require.Equal(t, http.StatusCreated, createRR.Code, createRR.Body.String())
	var created createAppointmentE2EResponse
	require.NoError(t, json.Unmarshal(createRR.Body.Bytes(), &created))
	apptID := created.Data.Appointment.ID
	require.Greater(t, apptID, int64(0))
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, db, "calendar.appointments", apptID) })

	parentToken := parentCalendarToken(t, chain.AccountID)

	// The guardian sees the appointment in their family calendar.
	listRR := doJSON(t, router, http.MethodGet, "/parent/me/calendar?from=2026-05-04&to=2026-05-04", parentToken, nil)
	require.Equal(t, http.StatusOK, listRR.Code, listRR.Body.String())
	var listed calendarListE2EResponse
	require.NoError(t, json.Unmarshal(listRR.Body.Bytes(), &listed))
	require.Len(t, listed.Data.Events, 1)
	assert.Equal(t, "Elternabend", listed.Data.Events[0].Title)

	// The guardian downloads the .ics for that appointment.
	icsPath := "/parent/me/calendar/appointments/" + strconv.FormatInt(apptID, 10) + "/ics"
	icsReq := httptest.NewRequest(http.MethodGet, icsPath, nil)
	icsReq.Header.Set("Authorization", "Bearer "+parentToken)
	icsRR := httptest.NewRecorder()
	router.ServeHTTP(icsRR, icsReq)
	require.Equal(t, http.StatusOK, icsRR.Code, icsRR.Body.String())
	assert.Contains(t, icsRR.Header().Get("Content-Type"), "text/calendar")
	assert.Contains(t, icsRR.Body.String(), "SUMMARY:Elternabend")

	// The guardian fetches their subscription feed URL.
	feedRR := doJSON(t, router, http.MethodGet, "/parent/me/calendar/feed", parentToken, nil)
	require.Equal(t, http.StatusOK, feedRR.Code, feedRR.Body.String())
	var feed feedE2EResponse
	require.NoError(t, json.Unmarshal(feedRR.Body.Bytes(), &feed))
	assert.Contains(t, feed.Data.URL, "/api/calendar-feed/")
	assert.Contains(t, feed.Data.WebcalURL, "webcal://")

	// A staff token (wrong scope) is rejected by the parent surface.
	staffScopeRR := doJSON(t, router, http.MethodGet, "/parent/me/calendar?from=2026-05-04&to=2026-05-04", manageToken, nil)
	assert.Equal(t, http.StatusUnauthorized, staffScopeRR.Code, staffScopeRR.Body.String())
}
