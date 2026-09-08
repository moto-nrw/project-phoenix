// Router-level tests for the Anfragen module's Mitarbeitende tab (#2433):
// GET /api/staff/absences/requests
//
// Requests flow through the production middleware chain (JWT → tenant →
// permission → tenant tx). Fixtures via testpkg helpers + direct row inserts.
package timetracking

import (
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

// Wire values of the absence contract. Spelled out here because the tests may
// not import the model package; they mirror models/active/staff_absence.go.
const (
	absenceTypeSick      = "sick"
	absenceTypeVacation  = "vacation"
	absenceTypeTraining  = "training"
	absenceTypeOther     = "other"
	absenceTypeCompTime  = "comp_time"
	absenceStatusRepd    = "reported"
	absenceStatusReqd    = "requested"
	absenceStatusQuestn  = "question"
	absenceStatusApprovd = "approved"
)

type absenceRequestItem struct {
	ID            int64  `json:"id,string"`
	StaffID       int64  `json:"staff_id,string"`
	AbsenceType   string `json:"absence_type"`
	Status        string `json:"status"`
	StaffName     string `json:"staff_name"`
	DecidedByName string `json:"decided_by_name"`
	ApprovedAt    string `json:"approved_at"`
	DecisionNote  string `json:"decision_note"`
}

// insertAbsenceRow writes one active.staff_absences row in the test's tenant
// and returns its ID. The columns mirror the model's bun tags; the row is
// written directly because no public capability creates absences in the
// pre-decision states these list views are about.
func insertAbsenceRow(t *testing.T, tc *testContext, row map[string]any) int64 {
	t.Helper()
	row["tenant_id"] = testpkg.Tenant(t)
	var id int64
	require.NoError(t, tc.db.NewInsert().
		Model(&row).
		TableExpr("active.staff_absences").
		Returning("id").
		Scan(testpkg.Ctx(t), &id))
	return id
}

// createAbsence inserts one absence for the staff member and returns its ID.
func createAbsence(t *testing.T, tc *testContext, staffID int64, absenceType, status string, decidedBy *int64) int64 {
	t.Helper()
	// The tenant comes from the test, not from the claims struct: claims are
	// rebased when they are USED, so reading TenantID off it yields the
	// bootstrap tenant while the staff row belongs to this test's (#2419).
	start := testpkg.TodayDate().AddDays(21)
	now := time.Now()
	row := map[string]any{
		"staff_id":     staffID,
		"absence_type": absenceType,
		"date_start":   start.String(),
		"date_end":     start.AddDays(1).String(),
		"status":       status,
		"created_by":   staffID,
		"requested_at": now,
	}
	if decidedBy != nil {
		row["approved_by"] = *decidedBy
		row["approved_at"] = now
		row["decision_note"] = "Passt so"
	}
	return insertAbsenceRow(t, tc, row)
}

func getAbsenceRequests(t *testing.T, tc *testContext, token, query string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/staff/absences/requests"+query, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	tc.router.ServeHTTP(rec, req)
	return rec
}

func decodeAbsenceRequests(t *testing.T, rec *httptest.ResponseRecorder) []absenceRequestItem {
	t.Helper()
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp struct {
		Data []absenceRequestItem `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	return resp.Data
}

func findAbsenceRequest(items []absenceRequestItem, id int64) *absenceRequestItem {
	for i := range items {
		if items[i].ID == id {
			return &items[i]
		}
	}
	return nil
}

// setupAbsenceRequestTest creates two staff members with distinct names plus a
// decider, and returns the router context and a vacation:approve token.
func setupAbsenceRequestTest(t *testing.T) (tc *testContext, token string, mueller, schmidt, decider int64) {
	t.Helper()
	tc = setupStaffRoute(t)
	suffix := time.Now().UnixNano()

	a := testpkg.CreateTestStaff(t, tc.db, "Mira", fmt.Sprintf("Muellerson-%d", suffix))
	b := testpkg.CreateTestStaff(t, tc.db, "Sven", fmt.Sprintf("Schmidtke-%d", suffix))
	d := testpkg.CreateTestStaff(t, tc.db, "Lea", fmt.Sprintf("Leitung-%d", suffix))
	return tc, authToken(t, "vacation:approve"), a.ID, b.ID, d.ID
}

func TestListAbsenceRequests_OpenCarriesStaffName(t *testing.T) {
	t.Parallel()

	tc, token, mueller, _, decider := setupAbsenceRequestTest(t)
	open := createAbsence(t, tc, mueller, absenceTypeVacation, absenceStatusReqd, nil)
	decided := createAbsence(t, tc, mueller, absenceTypeVacation, absenceStatusApprovd, &decider)

	items := decodeAbsenceRequests(t, getAbsenceRequests(t, tc, token, "?view=open"))

	row := findAbsenceRequest(items, open)
	require.NotNil(t, row, "open request must be listed")
	assert.Contains(t, row.StaffName, "Mira")
	assert.Nil(t, findAbsenceRequest(items, decided), "decided request must not be in the work list")
}

func TestListAbsenceRequests_QuestionStaysOpen(t *testing.T) {
	t.Parallel()

	tc, token, mueller, _, _ := setupAbsenceRequestTest(t)
	questioned := createAbsence(t, tc, mueller, absenceTypeVacation, absenceStatusQuestn, nil)

	items := decodeAbsenceRequests(t, getAbsenceRequests(t, tc, token, "?view=open"))
	assert.NotNil(t, findAbsenceRequest(items, questioned))
}

func TestListAbsenceRequests_HistoryCarriesDecider(t *testing.T) {
	t.Parallel()

	tc, token, mueller, _, decider := setupAbsenceRequestTest(t)
	open := createAbsence(t, tc, mueller, absenceTypeVacation, absenceStatusReqd, nil)
	decided := createAbsence(t, tc, mueller, absenceTypeSick, absenceStatusApprovd, &decider)

	items := decodeAbsenceRequests(t, getAbsenceRequests(t, tc, token, "?view=history"))

	row := findAbsenceRequest(items, decided)
	require.NotNil(t, row, "decided request must be in the history")
	assert.Contains(t, row.DecidedByName, "Lea")
	assert.NotEmpty(t, row.ApprovedAt)
	assert.Equal(t, "Passt so", row.DecisionNote)
	assert.Nil(t, findAbsenceRequest(items, open), "open request must not be in the history")
}

func TestListAbsenceRequests_SearchesByStaffName(t *testing.T) {
	t.Parallel()

	tc, token, mueller, schmidt, _ := setupAbsenceRequestTest(t)
	mine := createAbsence(t, tc, mueller, absenceTypeVacation, absenceStatusReqd, nil)
	other := createAbsence(t, tc, schmidt, absenceTypeVacation, absenceStatusReqd, nil)

	items := decodeAbsenceRequests(t, getAbsenceRequests(t, tc, token, "?view=open&search=muellerson"))
	assert.NotNil(t, findAbsenceRequest(items, mine), "case-insensitive name match must hit")
	assert.Nil(t, findAbsenceRequest(items, other))
}

func TestListAbsenceRequests_TreatsWildcardsAsLiterals(t *testing.T) {
	t.Parallel()

	tc, token, mueller, _, _ := setupAbsenceRequestTest(t)
	mine := createAbsence(t, tc, mueller, absenceTypeVacation, absenceStatusReqd, nil)

	// A typed "%" is part of a name, not a wildcard — it must match nothing.
	items := decodeAbsenceRequests(t, getAbsenceRequests(t, tc, token, "?view=open&search=%25"))
	assert.Nil(t, findAbsenceRequest(items, mine))
}

func TestListAbsenceRequests_FiltersByType(t *testing.T) {
	t.Parallel()

	tc, token, mueller, _, _ := setupAbsenceRequestTest(t)
	vacation := createAbsence(t, tc, mueller, absenceTypeVacation, absenceStatusReqd, nil)
	training := createAbsence(t, tc, mueller, absenceTypeTraining, absenceStatusReqd, nil)

	items := decodeAbsenceRequests(t, getAbsenceRequests(t, tc, token, "?view=open&types=training"))
	assert.NotNil(t, findAbsenceRequest(items, training))
	assert.Nil(t, findAbsenceRequest(items, vacation))
}

func TestListAbsenceRequests_RequiresPermission(t *testing.T) {
	t.Parallel()

	tc, _, _, _, _ := setupAbsenceRequestTest(t)
	rec := getAbsenceRequests(t, tc, authToken(t, "schedules:read"), "?view=open")
	assert.Equal(t, http.StatusForbidden, rec.Code)
}
