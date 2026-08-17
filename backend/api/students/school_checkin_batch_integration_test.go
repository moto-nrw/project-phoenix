package students_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Integration tests for POST /api/students/school-checkin/batch (#2359).
// Same conventions as the single-endpoint tests: real DB, real fixtures,
// external _test package, full staff/account chain because the handler
// resolves JWT claims through PersonService.
//
// Student IDs travel as strings in both directions (review: JSON numbers
// corrupt int64 values above 2^53-1 client-side).

type batchCheckinResponse struct {
	Data struct {
		Action  string `json:"action"`
		Results []struct {
			StudentID string `json:"student_id"`
			OK        bool   `json:"ok"`
			Changed   bool   `json:"changed"`
			Status    string `json:"status"`
			Location  string `json:"location"`
			Error     string `json:"error"`
		} `json:"results"`
		Succeeded int `json:"succeeded"`
		Failed    int `json:"failed"`
	} `json:"data"`
}

func idStr(id int64) string { return strconv.FormatInt(id, 10) }

func postBatchCheckin(t *testing.T, tc *testContext, accountID int64, body map[string]any) (*batchCheckinResponse, int, string) {
	t.Helper()
	bodyBytes, err := json.Marshal(body)
	require.NoError(t, err)
	req := testutil.NewRequest("POST", "/school-checkin/batch", bytesReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rr := authExec(t, tc, req, testutil.AdminTestClaims(int(accountID)), []string{"admin:*"})
	var resp batchCheckinResponse
	if rr.Code == http.StatusOK {
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	}
	return &resp, rr.Code, rr.Body.String()
}

func TestSchoolCheckinBatch_CheckInMultiple(t *testing.T) {
	tc := setupTestContext(t)

	first := testpkg.CreateTestStudent(t, tc.db, "BatchIn", "First", "1a")
	second := testpkg.CreateTestStudent(t, tc.db, "BatchIn", "Second", "1a")
	staff, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "BatchIn", "Caller")
	defer testpkg.CleanupActivityFixtures(t, tc.db, first.ID, second.ID, staff.ID, staff.PersonID)
	defer testpkg.CleanupAccount(t, tc.db, account.ID)

	resp, code, body := postBatchCheckin(t, tc, account.ID, map[string]any{
		"action":      "in",
		"student_ids": []string{idStr(first.ID), idStr(second.ID)},
	})
	require.Equal(t, http.StatusOK, code, "body: %s", body)

	require.Len(t, resp.Data.Results, 2)
	assert.Equal(t, 2, resp.Data.Succeeded)
	assert.Equal(t, 0, resp.Data.Failed)
	for _, result := range resp.Data.Results {
		assert.True(t, result.OK)
		assert.True(t, result.Changed)
		assert.Equal(t, "checked_in", result.Status)
		assert.Equal(t, "Anwesend", result.Location)
	}
}

func TestSchoolCheckinBatch_MixedStates_IdempotentPerStudent(t *testing.T) {
	tc := setupTestContext(t)

	present := testpkg.CreateTestStudent(t, tc.db, "BatchMixed", "Present", "2a")
	absent := testpkg.CreateTestStudent(t, tc.db, "BatchMixed", "Absent", "2a")
	staff, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "BatchMixed", "Caller")
	defer testpkg.CleanupActivityFixtures(t, tc.db, present.ID, absent.ID, staff.ID, staff.PersonID)
	defer testpkg.CleanupAccount(t, tc.db, account.ID)

	// Check the first student in beforehand via the single endpoint.
	inBody, _ := json.Marshal(map[string]string{"action": "in"})
	reqIn := testutil.NewRequest("POST", fmt.Sprintf("/%d/school-checkin", present.ID), bytesReader(inBody))
	reqIn.Header.Set("Content-Type", "application/json")
	rrIn := authExec(t, tc, reqIn, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})
	require.Equal(t, http.StatusOK, rrIn.Code, "body: %s", rrIn.Body.String())

	// Batch check-out of both: the present one changes, the absent one is a
	// no-op — both count as succeeded (Swantje's Verabschiedungskreis case).
	resp, code, body := postBatchCheckin(t, tc, account.ID, map[string]any{
		"action":      "out",
		"student_ids": []string{idStr(present.ID), idStr(absent.ID)},
	})
	require.Equal(t, http.StatusOK, code, "body: %s", body)

	require.Len(t, resp.Data.Results, 2)
	assert.Equal(t, 2, resp.Data.Succeeded)
	assert.Equal(t, 0, resp.Data.Failed)

	byID := map[string]bool{}
	for _, result := range resp.Data.Results {
		assert.True(t, result.OK)
		assert.Equal(t, "Abwesend", result.Location)
		byID[result.StudentID] = result.Changed
	}
	assert.True(t, byID[idStr(present.ID)], "present student must be checked out (changed)")
	assert.False(t, byID[idStr(absent.ID)], "absent student must be a no-op (unchanged)")
}

func TestSchoolCheckinBatch_UnknownStudentSkipped_RestProcessed(t *testing.T) {
	tc := setupTestContext(t)

	student := testpkg.CreateTestStudent(t, tc.db, "BatchSkip", "Known", "3a")
	staff, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "BatchSkip", "Caller")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID, staff.ID, staff.PersonID)
	defer testpkg.CleanupAccount(t, tc.db, account.ID)

	// An id that cannot exist: far outside any sequence used by fixtures.
	unknownID := student.ID + 1_000_000

	resp, code, body := postBatchCheckin(t, tc, account.ID, map[string]any{
		"action":      "in",
		"student_ids": []string{idStr(unknownID), idStr(student.ID)},
	})
	require.Equal(t, http.StatusOK, code, "body: %s", body)

	require.Len(t, resp.Data.Results, 2)
	assert.Equal(t, 1, resp.Data.Succeeded)
	assert.Equal(t, 1, resp.Data.Failed)

	for _, result := range resp.Data.Results {
		if result.StudentID == idStr(unknownID) {
			assert.False(t, result.OK)
			assert.Equal(t, "not_found", result.Error)
		} else {
			assert.True(t, result.OK)
			assert.Equal(t, "checked_in", result.Status)
		}
	}
}

func TestSchoolCheckinBatch_DuplicateIDsCollapse(t *testing.T) {
	tc := setupTestContext(t)

	student := testpkg.CreateTestStudent(t, tc.db, "BatchDup", "Target", "4a")
	staff, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "BatchDup", "Caller")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID, staff.ID, staff.PersonID)
	defer testpkg.CleanupAccount(t, tc.db, account.ID)

	resp, code, body := postBatchCheckin(t, tc, account.ID, map[string]any{
		"action":      "in",
		"student_ids": []string{idStr(student.ID), idStr(student.ID), idStr(student.ID)},
	})
	require.Equal(t, http.StatusOK, code, "body: %s", body)

	require.Len(t, resp.Data.Results, 1, "duplicates must collapse to one result")
	assert.Equal(t, 1, resp.Data.Succeeded)
	assert.True(t, resp.Data.Results[0].Changed)
}

func TestSchoolCheckinBatch_InvalidAction_Rejects(t *testing.T) {
	tc := setupTestContext(t)

	student := testpkg.CreateTestStudent(t, tc.db, "BatchInvalid", "Target", "5a")
	staff, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "BatchInvalid", "Caller")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID, staff.ID, staff.PersonID)
	defer testpkg.CleanupAccount(t, tc.db, account.ID)

	_, code, body := postBatchCheckin(t, tc, account.ID, map[string]any{
		"action":      "toggle",
		"student_ids": []string{idStr(student.ID)},
	})
	assert.Equal(t, http.StatusBadRequest, code)
	assert.Contains(t, body, "action must be")
}

func TestSchoolCheckinBatch_MalformedID_Rejects(t *testing.T) {
	tc := setupTestContext(t)

	staff, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "BatchMalformed", "Caller")
	defer testpkg.CleanupActivityFixtures(t, tc.db, staff.ID, staff.PersonID)
	defer testpkg.CleanupAccount(t, tc.db, account.ID)

	_, code, body := postBatchCheckin(t, tc, account.ID, map[string]any{
		"action":      "in",
		"student_ids": []string{"12abc"},
	})
	assert.Equal(t, http.StatusBadRequest, code)
	assert.Contains(t, body, "student_ids must be numeric")
}

func TestSchoolCheckinBatch_EmptyIDs_Rejects(t *testing.T) {
	tc := setupTestContext(t)

	staff, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "BatchEmpty", "Caller")
	defer testpkg.CleanupActivityFixtures(t, tc.db, staff.ID, staff.PersonID)
	defer testpkg.CleanupAccount(t, tc.db, account.ID)

	_, code, body := postBatchCheckin(t, tc, account.ID, map[string]any{
		"action":      "in",
		"student_ids": []string{},
	})
	assert.Equal(t, http.StatusBadRequest, code)
	assert.Contains(t, body, "student_ids must not be empty")
}
