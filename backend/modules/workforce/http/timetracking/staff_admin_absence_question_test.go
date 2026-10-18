// Router-level tests for the #1419 Rückfrage endpoint:
// POST /api/staff/absences/{absenceId}/question
//
// Requests flow through the production middleware chain (JWT → tenant →
// permission → tenant tx). Fixtures via testpkg helpers + direct row inserts.
package timetracking

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createRequestedVacation inserts a requested vacation absence for the staff
// member and returns its ID.
func createRequestedVacation(t *testing.T, tc *testContext, staffID int64) int64 {
	t.Helper()
	start := testpkg.TodayDate().AddDays(14)
	return insertAbsenceRow(t, tc, map[string]any{
		"staff_id":     staffID,
		"absence_type": absenceTypeVacation,
		"date_start":   start.String(),
		"date_end":     start.AddDays(1).String(),
		"status":       absenceStatusReqd,
		"created_by":   staffID,
		"requested_at": time.Now(),
	})
	// Audit rows cascade with the absence (FK ON DELETE CASCADE).
}

func postQuestion(t *testing.T, tc *testContext, token string, absenceID int64, note string) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(map[string]any{"decision_note": note})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/staff/absences/%d/question", absenceID), bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	tc.router.ServeHTTP(rec, req)
	return rec
}

func TestQuestionAbsence_Success(t *testing.T) {
	t.Parallel()

	tc, _, subjectID, _ := setupAbsenceAdminTest(t)
	absenceID := createRequestedVacation(t, tc, subjectID)

	token := authToken(t, "vacation:approve")
	rec := postQuestion(t, tc, token, absenceID, "Bitte Vertretung klären")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	updated, err := newWorkforceTestRepositories(t, tc.db).StaffAbsence.FindByID(testpkg.Ctx(t), absenceID)
	require.NoError(t, err)
	assert.Equal(t, absenceStatusQuestn, updated.Status)
	assert.Equal(t, "Bitte Vertretung klären", updated.DecisionNote)
	assert.Nil(t, updated.ApprovedBy)
}

func TestQuestionAbsence_RequiresNote(t *testing.T) {
	t.Parallel()

	tc, _, subjectID, _ := setupAbsenceAdminTest(t)
	absenceID := createRequestedVacation(t, tc, subjectID)

	token := authToken(t, "vacation:approve")
	rec := postQuestion(t, tc, token, absenceID, "")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestQuestionAbsence_RequiresPermission(t *testing.T) {
	t.Parallel()

	tc, _, subjectID, _ := setupAbsenceAdminTest(t)
	absenceID := createRequestedVacation(t, tc, subjectID)

	noPermToken := authToken(t, "schedules:read")
	rec := postQuestion(t, tc, noPermToken, absenceID, "egal")
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestQuestionAbsence_OnlyFromRequested(t *testing.T) {
	t.Parallel()

	tc, _, subjectID, _ := setupAbsenceAdminTest(t)
	absenceID := createRequestedVacation(t, tc, subjectID)

	token := authToken(t, "vacation:approve")
	require.Equal(t, http.StatusOK, postQuestion(t, tc, token, absenceID, "erste Rückfrage").Code)
	// Second question on an already-questioned absence must fail.
	rec := postQuestion(t, tc, token, absenceID, "zweite Rückfrage")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestListPendingAbsences_IncludesQuestionStatus(t *testing.T) {
	t.Parallel()

	tc, _, subjectID, _ := setupAbsenceAdminTest(t)
	absenceID := createRequestedVacation(t, tc, subjectID)

	token := authToken(t, "vacation:approve")
	require.Equal(t, http.StatusOK, postQuestion(t, tc, token, absenceID, "bitte klären").Code)

	req := httptest.NewRequest(http.MethodGet, "/staff/absences/pending", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	tc.router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Data []struct {
			ID     int64  `json:"id"`
			Status string `json:"status"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	found := false
	for _, row := range resp.Data {
		if row.ID == absenceID {
			found = true
			assert.Equal(t, absenceStatusQuestn, row.Status)
		}
	}
	assert.True(t, found, "questioned absence must stay in the pending inbox")
}
