package presenceprojectionhttp_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetStudentCurrentVisit(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	student := testpkg.CreateTestStudent(t, tc.db, "Visit", "Test", "VT1")

	t.Run("error_when_no_current_visit", func(t *testing.T) {
		req := testutil.NewRequest("GET", fmt.Sprintf("/%d/current-visit", student.ID), nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		// No visit should return error or empty
		assert.Contains(t, []int{http.StatusOK, http.StatusInternalServerError}, rr.Code)
	})

	t.Run("bad_request_for_invalid_id", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/invalid/current-visit", nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})
}

func TestGetStudentVisitHistory(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	student := testpkg.CreateTestStudent(t, tc.db, "History", "Test", "HT1")

	t.Run("success_returns_empty_history", func(t *testing.T) {
		req := testutil.NewRequest("GET", fmt.Sprintf("/%d/visit-history", student.ID), nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
	})

	t.Run("bad_request_for_invalid_id", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/invalid/visit-history", nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})
}

func TestGetStudentVisitHistory_WithDateRange(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	student := testpkg.CreateTestStudent(t, tc.db, "DateRange", "Test", "DR1")

	t.Run("with_start_date", func(t *testing.T) {
		req := testutil.NewRequest("GET", fmt.Sprintf("/%d/visit-history?from=2024-01-01", student.ID), nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
	})

	t.Run("with_date_range", func(t *testing.T) {
		req := testutil.NewRequest("GET", fmt.Sprintf("/%d/visit-history?from=2024-01-01&to=2024-12-31", student.ID), nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
	})
}

func TestGetStudentVisitHistory_WithVisits(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	// Create a student with an active visit to test visit history
	student := testpkg.CreateTestStudent(t, tc.db, "Visit", "History", "VH1")
	room := testpkg.CreateTestRoom(t, tc.db, "HistoryRoom")
	activity := testpkg.CreateTestActivityGroup(t, tc.db, "HistoryActivity")
	group := testpkg.CreateTestActiveGroup(t, tc.db, activity.ID, room.ID)
	entry := testpkg.TodayDate().BerlinMidnight().Add(8 * time.Hour)
	visit := testpkg.CreateTestVisit(t, tc.db, student.ID, group.ID, entry, nil)

	req := testutil.NewRequest("GET", fmt.Sprintf("/%d/visit-history", student.ID), nil)

	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

	assert.Equal(t, http.StatusOK, rr.Code, "Should return visit history. Body: %s", rr.Body.String())
	var response struct {
		Data []json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &response))
	require.Len(t, response.Data, 1)
	expectedVisit := fmt.Sprintf(`{"id":%d,"created_at":%q,"updated_at":%q,"tenant_id":%d,"student_id":%d,"active_group_id":%d,"entry_time":%q}`,
		visit.ID, visit.CreatedAt.Format(time.RFC3339Nano), visit.UpdatedAt.Format(time.RFC3339Nano),
		testpkg.Tenant(t), student.ID, group.ID, entry.Format(time.RFC3339Nano))
	assert.JSONEq(t, expectedVisit, string(response.Data[0]))
	currentRequest := testutil.NewRequest("GET", fmt.Sprintf("/%d/current-visit", student.ID), nil)
	currentResponse := authExec(t, tc, currentRequest, testutil.AdminTestClaims(1), []string{"admin:*"})
	require.Equal(t, http.StatusOK, currentResponse.Code)
	var currentBody struct {
		Data json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(currentResponse.Body.Bytes(), &currentBody))
	assert.JSONEq(t, expectedVisit, string(currentBody.Data))
}

func TestGetStudentCurrentVisit_WithActiveVisit(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	t.Run("returns_current_visit", func(t *testing.T) {
		// Create a student with an active visit
		room := testpkg.CreateTestRoom(t, tc.db, "VisitTestRoom")
		student := testpkg.CreateTestStudent(t, tc.db, "Current", "Visit", "CV1")
		activityGroup := testpkg.CreateTestActivityGroup(t, tc.db, "VisitTestActivity")
		staff := testpkg.CreateTestStaff(t, tc.db, "Visit", "Staff")
		device := testpkg.CreateTestDevice(t, tc.db, "visit-test-device")

		// Create attendance
		now := time.Now()
		testpkg.CreateTestAttendance(t, tc.db, student.ID, staff.ID, device.ID, now, nil)

		// Create active group and visit
		activeGroup := testpkg.CreateTestActiveGroup(t, tc.db, activityGroup.ID, room.ID)
		testpkg.CreateTestVisit(t, tc.db, student.ID, activeGroup.ID, now, nil)

		req := testutil.NewRequest("GET", fmt.Sprintf("/%d/current-visit", student.ID), nil)

		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		// Should return the visit data
		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
	})
}

// TestGetStudent_CheckedOutReadsAbsentSinceCheckout pins the checked-out
// location on the student payload, which replaced the retired
// /students/{id}/current-location endpoint (#4431): a full-access reader sees
// "Abwesend" with the checkout time.
func TestGetStudent_CheckedOutReadsAbsentSinceCheckout(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)
	student := testpkg.CreateTestStudent(t, tc.db, "CheckedOut", "Payload", "CO2")
	staff := testpkg.CreateTestStaff(t, tc.db, "CheckedOut", "PayloadStaff")
	device := testpkg.CreateTestDevice(t, tc.db, "checkout-payload-device")
	// Minutes, not hours, keep both stamps on today's date.
	now := time.Now()
	checkOut := now.Add(-1 * time.Minute).Truncate(time.Second)
	testpkg.CreateTestAttendance(t, tc.db, student.ID, staff.ID, device.ID, now.Add(-2*time.Minute), &checkOut)

	req := testutil.NewRequest("GET", fmt.Sprintf("/%d", student.ID), nil)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})
	require.Equal(t, http.StatusOK, rr.Code, "body: %s", rr.Body.String())

	var response struct {
		Data struct {
			Location      string     `json:"current_location"`
			LocationSince *time.Time `json:"location_since"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &response))
	assert.Equal(t, "Abwesend", response.Data.Location)
	require.NotNil(t, response.Data.LocationSince)
	assert.True(t, checkOut.Equal(*response.Data.LocationSince), "location_since %s, want %s", response.Data.LocationSince, checkOut)
}
