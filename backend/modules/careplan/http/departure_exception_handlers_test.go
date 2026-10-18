package careplanhttp_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The OGS sets the Abholart of one day directly (#3945). The value shows in
// the pickup data of the child and replaces the weekly plan on the
// Kinderkarte of that day only; deleting it returns the day to the plan.
func TestStudentDepartureExceptionRoundTrip(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)
	// A fixed Monday, so the list's planning date is a care weekday.
	tc.resource.Now = func() time.Time { return time.Date(2026, time.June, 1, 10, 0, 0, 0, time.UTC) }
	student := testpkg.CreateTestStudent(t, tc.db, "Heimweg", "Tag", "HT1")
	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Heimweg", "Teacher")
	claims := testutil.AdminTestClaims(int(account.ID))
	date := calendar.NewDate(2026, time.June, 1)
	path := fmt.Sprintf("/%d/departure-exceptions/%s", student.ID, date.String())

	put := testutil.NewAuthenticatedRequest(t, http.MethodPut, path, map[string]any{
		"departure_mode": "pickup", "reason": "Vater hat angerufen",
	})
	rr := authExec(t, tc, put, claims, []string{"admin:*"})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.Contains(t, rr.Body.String(), `"departure_mode":"pickup"`)

	// Saving the day again replaces its value; there is one per day.
	put = testutil.NewAuthenticatedRequest(t, http.MethodPut, path, map[string]any{"departure_mode": "bus"})
	rr = authExec(t, tc, put, claims, []string{"admin:*"})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	data := testutil.NewRequest(http.MethodGet, fmt.Sprintf("/%d/pickup-schedules?from=%s&to=%s", student.ID, date, date.AddDays(6)), nil)
	rr = authExec(t, tc, data, claims, []string{"admin:*"})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var pickupData struct {
		Data struct {
			DepartureExceptions []struct {
				ExceptionDate string `json:"exception_date"`
				DepartureMode string `json:"departure_mode"`
			} `json:"departure_exceptions"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &pickupData))
	require.Len(t, pickupData.Data.DepartureExceptions, 1)
	assert.Equal(t, "2026-06-01", pickupData.Data.DepartureExceptions[0].ExceptionDate)
	assert.Equal(t, "bus", pickupData.Data.DepartureExceptions[0].DepartureMode)

	assert.Equal(t, []string{"bus"}, listedDepartureModes(t, tc, claims, student.ID, date, true))
	assert.Empty(t, listedDepartureModes(t, tc, claims, student.ID, date.AddDays(1), false),
		"the next day keeps the weekly plan, which this child does not have")

	del := testutil.NewAuthenticatedRequest(t, http.MethodDelete, path, nil)
	rr = authExec(t, tc, del, claims, []string{"admin:*"})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.Empty(t, listedDepartureModes(t, tc, claims, student.ID, date, false))
}

func TestStudentDepartureExceptionRejectsUnknownMode(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)
	student := testpkg.CreateTestStudent(t, tc.db, "Heimweg", "Falsch", "HT2")
	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Heimweg", "Falsch")

	put := testutil.NewAuthenticatedRequest(t, http.MethodPut, fmt.Sprintf("/%d/departure-exceptions/2026-06-01", student.ID), map[string]any{
		"departure_mode": "taxi",
	})
	rr := authExec(t, tc, put, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})
	assert.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())

	put = testutil.NewAuthenticatedRequest(t, http.MethodPut, fmt.Sprintf("/%d/departure-exceptions/kein-datum", student.ID), map[string]any{
		"departure_mode": "bus",
	})
	rr = authExec(t, tc, put, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})
	assert.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
}

func TestStudentDepartureExceptionRejectsNonCareWeekend(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t, fixedCalendarClock)
	student := testpkg.CreateTestStudent(t, tc.db, "Heimweg", "Wochenende", "HT3")
	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Heimweg", "Wochenende")
	date := studentsTestToday.AddDays(5)

	put := testutil.NewAuthenticatedRequest(t, http.MethodPut, fmt.Sprintf("/%d/departure-exceptions/%s", student.ID, date), map[string]any{
		"departure_mode": "bus",
	})
	rr := authExec(t, tc, put, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})
	assert.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
	assert.Empty(t, listedDepartureModes(t, tc, testutil.AdminTestClaims(int(account.ID)), student.ID, date, false))
}

// listedDepartureModes reads the Kinderkarte projection of one child on date.
func listedDepartureModes(t *testing.T, tc *testContext, claims sessiontoken.AppClaims, studentID int64, date calendar.Date, wantException bool) []string {
	t.Helper()
	req := testutil.NewAuthenticatedRequest(t, http.MethodGet, "/?view=slim&date="+date.String(), nil)
	rr := authExec(t, tc, req, claims, []string{"admin:*"})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var response struct {
		Data []struct {
			ID                   int64    `json:"id"`
			DepartureModes       []string `json:"departure_modes"`
			DepartureIsException bool     `json:"departure_is_exception"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &response))
	for _, row := range response.Data {
		if row.ID == studentID {
			assert.Equal(t, wantException, row.DepartureIsException)
			return row.DepartureModes
		}
	}
	t.Fatalf("student %d not listed", studentID)
	return nil
}
