package students_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Early-checkout note (#3324) through POST /students/{id}/school-checkin and
// back out through the student detail ("Heutige Abholung").

func postSchoolCheckin(t *testing.T, tc *testContext, studentID, accountID int64, body map[string]string) int {
	t.Helper()
	bodyBytes, err := json.Marshal(body)
	require.NoError(t, err)
	req := testutil.NewRequest("POST", fmt.Sprintf("/%d/school-checkin", studentID), bytesReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rr := authExec(t, tc, req, testutil.AdminTestClaims(int(accountID)), []string{"admin:*"})
	return rr.Code
}

func TestSchoolCheckin_CheckOutNote_ShowsInStudentDetail(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)
	_ = testpkg.EnsureWebManualDevice(t, tc.db)

	student := testpkg.CreateTestStudent(t, tc.db, "Notiz", "Kind", "2a")
	_, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "Notiz", "Caller")

	require.Equal(t, http.StatusOK, postSchoolCheckin(t, tc, student.ID, account.ID, map[string]string{"action": "in"}))
	require.Equal(t, http.StatusOK, postSchoolCheckin(t, tc, student.ID, account.ID, map[string]string{
		"action": "out",
		"note":   " Arzttermin ",
	}))

	req := testutil.NewRequest("GET", fmt.Sprintf("/%d", student.ID), nil)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})
	require.Equal(t, http.StatusOK, rr.Code, "body: %s", rr.Body.String())

	var resp struct {
		Data struct {
			ActualPickupTime *string `json:"actual_pickup_time"`
			ActualPickupNote *string `json:"actual_pickup_note"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	require.NotNil(t, resp.Data.ActualPickupTime)
	require.NotNil(t, resp.Data.ActualPickupNote)
	assert.Equal(t, "Arzttermin", *resp.Data.ActualPickupNote)
}

func TestSchoolCheckin_CheckOutNote_RejectsInvalidInput(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)
	_ = testpkg.EnsureWebManualDevice(t, tc.db)

	student := testpkg.CreateTestStudent(t, tc.db, "Notiz", "Ungueltig", "2b")
	_, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "Notiz", "Pruefer")

	assert.Equal(t, http.StatusBadRequest, postSchoolCheckin(t, tc, student.ID, account.ID, map[string]string{
		"action": "in",
		"note":   "Arzttermin",
	}), "a note only belongs to a checkout")

	require.Equal(t, http.StatusOK, postSchoolCheckin(t, tc, student.ID, account.ID, map[string]string{"action": "in"}))
	assert.Equal(t, http.StatusBadRequest, postSchoolCheckin(t, tc, student.ID, account.ID, map[string]string{
		"action": "out",
		"note":   strings.Repeat("x", 501),
	}), "a note over 500 characters is rejected")

	// The rejected checkout left the child checked in, so a valid one still changes state.
	req := testutil.NewRequest("GET", fmt.Sprintf("/%d", student.ID), nil)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})
	require.Equal(t, http.StatusOK, rr.Code)
	var resp struct {
		Data struct {
			ActualPickupTime *string `json:"actual_pickup_time"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Nil(t, resp.Data.ActualPickupTime, "rejected checkout must not close the stay")
}
