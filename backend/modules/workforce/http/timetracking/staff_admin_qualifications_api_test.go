// Router-level tests for the Nachweise (#4808): kinds and the list sit behind
// staff:stammdaten, a qualification row can be assigned a kind, and a kind of
// another school stays invisible.
package timetracking

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type qualificationTypesBody struct {
	Data []QualificationTypeResponse `json:"data"`
}

func TestQualificationTypesAPI_RequiresStammdatenAndStaysInTheSchool(t *testing.T) {
	t.Parallel()
	ctx := setupOverviewAPI(t)

	rec := ctx.get("/staff/qualification-types", "users:read", "time_tracking:manage")
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	rec = ctx.get("/staff/qualifications", "users:read", "users:update")
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	rec = ctx.post("/staff/qualification-types", `{"name":"Hygiene"}`, "users:update")
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())

	rec = ctx.get("/staff/qualification-types", "staff:stammdaten")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var listed qualificationTypesBody
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &listed))
	require.Len(t, listed.Data, 3, "the standard list")
	assert.Equal(t, "first_aid_child", listed.Data[0].StandardKey)

	name := fmt.Sprintf("Hygiene %d", time.Now().UnixNano())
	rec = ctx.post("/staff/qualification-types", `{"name":"`+name+`","validity_months":12,"mandatory":true}`, "staff:stammdaten")
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"reminder_lead_weeks":8`, "the agreed eight weeks are the default lead")
	rec = ctx.post("/staff/qualification-types", `{"name":"`+name+`"}`, "staff:stammdaten")
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "workforce.qualification_type_name_taken")
	rec = ctx.post("/staff/qualification-types", `{"name":"Zu lang","reminder_lead_weeks":99}`, "staff:stammdaten")
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())

	// A kind of another school is not found here.
	foreignTenant := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, ctx.tc.db, foreignTenant)
	var foreignID int64
	require.NoError(t, ctx.tc.db.NewRaw(`INSERT INTO users.staff_qualification_types (tenant_id, name) VALUES (?, 'Fremd') RETURNING id`, foreignTenant).Scan(t.Context(), &foreignID))
	rec = ctx.put(fmt.Sprintf("/staff/qualification-types/%d", foreignID), `{"name":"Übernommen"}`, "staff:stammdaten")
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "workforce.qualification_type_not_found")
}

func TestQualificationsAPI_AssignKindAndReadStatus(t *testing.T) {
	t.Parallel()
	ctx := setupOverviewAPI(t)
	target := testpkg.CreateTestStaff(t, ctx.tc.db, "Nachweis", fmt.Sprintf("API-%d", time.Now().UnixNano()))

	rec := ctx.get("/staff/qualification-types", "staff:stammdaten")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var listed qualificationTypesBody
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &listed))
	firstAid := listed.Data[0].ID

	base := fmt.Sprintf("/staff/%d", target.ID)
	rec = ctx.put(base+"/stammdaten/qualifikationen", fmt.Sprintf(`{"qualifikationen":[{"type_id":%d,"name":"","expires_on":"2020-03-01"}],"note":"Test"}`, firstAid), "staff:stammdaten")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = ctx.put(base+"/stammdaten/qualifikationen", `{"qualifikationen":[{"type_id":999999999,"name":""}],"note":"Test"}`, "staff:stammdaten")
	require.Equal(t, http.StatusBadRequest, rec.Code, "an unknown kind is refused: "+rec.Body.String())

	rec = ctx.get(base+"/stammdaten", "staff:stammdaten")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), fmt.Sprintf(`"type_id":%d`, firstAid))
	assert.Contains(t, rec.Body.String(), `"name":"Erste Hilfe am Kind"`, "a row of a kind takes the kind's name")

	rec = ctx.get(base+"/qualifications", "users:read")
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	rec = ctx.get(base+"/qualifications", "staff:stammdaten")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"status":"expired"`)

	rec = ctx.get("/staff/qualifications", "staff:stammdaten")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = ctx.get("/staff/me/qualifications")
	require.Equal(t, http.StatusOK, rec.Code, "every signed-in person reads their own Nachweise: "+rec.Body.String())
}

// Rule 15 (#2940): the Nachweise list and the kinds read one batch each,
// however many people and qualifications exist.
func TestQualificationsAPI_ListQueryBudgets(t *testing.T) {
	t.Parallel()
	ctx := setupOverviewAPI(t)
	addQualifications := func() {
		staff := testpkg.CreateTestStaff(t, ctx.tc.db, "Budget", fmt.Sprintf("Nachweis-%d", time.Now().UnixNano()))
		body := `{"qualifikationen":[{"name":"Erste Hilfe","expires_on":"2027-01-01"},{"name":"Hygiene","expires_on":"2020-01-01"}],"note":"Test"}`
		rec := ctx.put(fmt.Sprintf("/staff/%d/stammdaten/qualifikationen", staff.ID), body, "staff:stammdaten")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	}
	measure := func(path string) []string {
		counter := testpkg.CaptureQueriesForContext(t, ctx.tc.db)
		req := httptest.NewRequest(http.MethodGet, path, nil).WithContext(counter.Context(context.Background()))
		req.Header.Set("Authorization", "Bearer "+authToken(t, "staff:stammdaten"))
		rec := httptest.NewRecorder()
		ctx.tc.router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		queries := counter.Queries()
		counter.Stop()
		return queries
	}
	addQualifications()
	smallTypes, smallList := measure("/staff/qualification-types"), measure("/staff/qualifications")
	addQualifications()
	addQualifications()
	largeTypes, largeList := measure("/staff/qualification-types"), measure("/staff/qualifications")

	assert.Equal(t, len(smallList), len(largeList), "the list must not issue a statement per person or row")
	assert.Equal(t, len(smallTypes), len(largeTypes))
	querybudgets.AssertQueryBudget(t, "workforce.staff_qualifications.list", largeList)
	querybudgets.AssertQueryBudget(t, "workforce.staff_qualification_types.list", largeTypes)
}

func TestQualificationsAPI_ReadsStayInTheSchool(t *testing.T) {
	t.Parallel()
	ctx := setupOverviewAPI(t)
	foreignTenant := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, ctx.tc.db, foreignTenant)
	foreign := testpkg.CreateTestStaffForTenant(t, ctx.tc.db, foreignTenant, "Fremd", fmt.Sprintf("Schule-%d", time.Now().UnixNano()))
	_, err := ctx.tc.db.ExecContext(t.Context(),
		`INSERT INTO users.staff_qualifications (tenant_id, staff_id, name, expires_on) VALUES (?, ?, 'Fremder Nachweis', '2020-01-01')`,
		foreignTenant, foreign.ID)
	require.NoError(t, err)

	rec := ctx.get("/staff/qualifications", "staff:stammdaten")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.NotContains(t, rec.Body.String(), "Fremder Nachweis")

	rec = ctx.get(fmt.Sprintf("/staff/%d/qualifications", foreign.ID), "staff:stammdaten")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.NotContains(t, rec.Body.String(), "Fremder Nachweis", "another school's staff member shows nothing here")

	rec = ctx.get(fmt.Sprintf("/staff/%d/qualifications", foreign.ID), "time_tracking:manage")
	require.Equal(t, http.StatusForbidden, rec.Code, "the Nachweise read needs staff:stammdaten")
}
