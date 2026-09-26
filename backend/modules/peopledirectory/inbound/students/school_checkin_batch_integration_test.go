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
	t.Parallel()

	tc := setupStudentsRoute(t)
	// The virtual WEB-MANUAL device is per tenant, so this test's own
	// tenant (#2419) needs its own row before a manual check-in.
	_ = testpkg.EnsureWebManualDevice(t, tc.db)

	first := testpkg.CreateTestStudent(t, tc.db, "BatchIn", "First", "1a")
	second := testpkg.CreateTestStudent(t, tc.db, "BatchIn", "Second", "1a")
	_, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "BatchIn", "Caller")

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
	t.Parallel()

	tc := setupStudentsRoute(t)
	// The virtual WEB-MANUAL device is per tenant, so this test's own
	// tenant (#2419) needs its own row before a manual check-in.
	_ = testpkg.EnsureWebManualDevice(t, tc.db)

	present := testpkg.CreateTestStudent(t, tc.db, "BatchMixed", "Present", "2a")
	absent := testpkg.CreateTestStudent(t, tc.db, "BatchMixed", "Absent", "2a")
	_, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "BatchMixed", "Caller")

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
	t.Parallel()

	tc := setupStudentsRoute(t)
	// The virtual WEB-MANUAL device is per tenant, so this test's own
	// tenant (#2419) needs its own row before a manual check-in.
	_ = testpkg.EnsureWebManualDevice(t, tc.db)

	student := testpkg.CreateTestStudent(t, tc.db, "BatchSkip", "Known", "3a")
	_, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "BatchSkip", "Caller")

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
	t.Parallel()

	tc := setupStudentsRoute(t)
	// The virtual WEB-MANUAL device is per tenant, so this test's own
	// tenant (#2419) needs its own row before a manual check-in.
	_ = testpkg.EnsureWebManualDevice(t, tc.db)

	student := testpkg.CreateTestStudent(t, tc.db, "BatchDup", "Target", "4a")
	_, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "BatchDup", "Caller")

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
	t.Parallel()

	tc := setupStudentsRoute(t)

	student := testpkg.CreateTestStudent(t, tc.db, "BatchInvalid", "Target", "5a")
	_, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "BatchInvalid", "Caller")

	_, code, body := postBatchCheckin(t, tc, account.ID, map[string]any{
		"action":      "toggle",
		"student_ids": []string{idStr(student.ID)},
	})
	assert.Equal(t, http.StatusBadRequest, code)
	assert.Contains(t, body, "action must be")
}

func TestSchoolCheckinBatch_MalformedID_Rejects(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	_, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "BatchMalformed", "Caller")

	_, code, body := postBatchCheckin(t, tc, account.ID, map[string]any{
		"action":      "in",
		"student_ids": []string{"12abc"},
	})
	assert.Equal(t, http.StatusBadRequest, code)
	assert.Contains(t, body, "student_ids must be numeric")
}

// TestSchoolCheckinBatch_TooManyIDs_Rejects pins that the size cap applies to
// the RAW list before deduplication (review #2372): a payload of duplicates
// must not slip past the limit by collapsing to a single student.
func TestSchoolCheckinBatch_TooManyIDs_Rejects(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	student := testpkg.CreateTestStudent(t, tc.db, "BatchCap", "Target", "6a")
	_, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "BatchCap", "Caller")

	// One over maxSchoolCheckinBatchSize, all the same student.
	ids := make([]string, 1001)
	for i := range ids {
		ids[i] = idStr(student.ID)
	}

	_, code, body := postBatchCheckin(t, tc, account.ID, map[string]any{
		"action":      "in",
		"student_ids": ids,
	})
	assert.Equal(t, http.StatusBadRequest, code)
	assert.Contains(t, body, "too many students")
}

// TestSchoolCheckinBatch_OversizedBody_Rejects pins the byte bound in front of
// the JSON decoder (review #2372): a body past maxSchoolCheckinBatchBytes
// fails during decode instead of being read into memory whole.
func TestSchoolCheckinBatch_OversizedBody_Rejects(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	student := testpkg.CreateTestStudent(t, tc.db, "BatchBytes", "Target", "6b")
	_, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "BatchBytes", "Caller")

	// Enough duplicate entries to exceed the 64KB body cap regardless of how
	// long the fixture's id happens to be (each entry costs len+3 bytes for
	// the quotes and comma).
	entry := idStr(student.ID)
	ids := make([]string, (64<<10)/(len(entry)+3)+100)
	for i := range ids {
		ids[i] = entry
	}

	_, code, body := postBatchCheckin(t, tc, account.ID, map[string]any{
		"action":      "in",
		"student_ids": ids,
	})
	assert.Equal(t, http.StatusBadRequest, code)
	assert.Contains(t, body, "too large")
}

// TestSchoolCheckinBatch_TrailingBody_Rejects pins that the bounded body is
// consumed to EOF (review #2372): the decoder stops after the first JSON
// value, so trailing bytes would otherwise stay unread and an oversized or
// malformed tail would slip past the advertised size limit.
func TestSchoolCheckinBatch_TrailingBody_Rejects(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	student := testpkg.CreateTestStudent(t, tc.db, "BatchTrail", "Target", "6c")
	_, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "BatchTrail", "Caller")

	payload := fmt.Sprintf(`{"action":"in","student_ids":[%q]}{"trailing":"junk"}`, idStr(student.ID))
	req := testutil.NewRequest("POST", "/school-checkin/batch", bytesReader([]byte(payload)))
	req.Header.Set("Content-Type", "application/json")
	rr := authExec(t, tc, req, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "trailing data")
}

func TestSchoolCheckinBatch_EmptyIDs_Rejects(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	_, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "BatchEmpty", "Caller")

	_, code, body := postBatchCheckin(t, tc, account.ID, map[string]any{
		"action":      "in",
		"student_ids": []string{},
	})
	assert.Equal(t, http.StatusBadRequest, code)
	assert.Contains(t, body, "student_ids must not be empty")
}
