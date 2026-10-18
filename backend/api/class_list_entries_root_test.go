package api

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

// checkClassListEntryAdministration drives the audited class-list writes
// (#2382) through the production router: the serving root must mount
// /class-list-entries over the administration composed with the People
// Directory lookup and the Audit command, not over a School Membership value
// without them, which refuses every audited write.
func checkClassListEntryAdministration(t *testing.T, graph *goldenRuntime) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("cle-root-%d@test.local", time.Now().UnixNano()))
	token := testutil.MintTestJWT(t, testutil.AdminTestClaimsForTenant(int(account.ID), testpkg.Tenant(t)))
	className := fmt.Sprintf("cler%d", time.Now().UnixNano()%100000)

	created := checkpointRequest(graph, checkpointScenario{
		Method: http.MethodPost, Path: "/api/class-list-entries/", Authenticated: true,
		Body: fmt.Sprintf(`{"first_name":"Rosa","last_name":"Wurzel","school_class":"%s"}`, className),
	}, token)
	require.Equal(t, http.StatusCreated, created.Code, created.Body.String())
	var entry struct {
		Data struct {
			ID int64 `json:"id,string"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(created.Body.Bytes(), &entry))
	require.NotZero(t, entry.Data.ID)

	updated := checkpointRequest(graph, checkpointScenario{
		Method: http.MethodPut, Path: fmt.Sprintf("/api/class-list-entries/%d", entry.Data.ID), Authenticated: true,
		Body: fmt.Sprintf(`{"first_name":"Rosa","last_name":"Wurzel","school_class":"%s-b"}`, className),
	}, token)
	require.Equal(t, http.StatusOK, updated.Code, updated.Body.String())

	student := testpkg.CreateTestStudent(t, db, "Rosa", "Wurzel", className+"-b")
	resolved := checkpointRequest(graph, checkpointScenario{
		Method: http.MethodPost, Path: fmt.Sprintf("/api/class-list-entries/%d/assign", entry.Data.ID), Authenticated: true,
		Body: fmt.Sprintf(`{"student_id":"%d"}`, student.ID),
	}, token)
	require.Equal(t, http.StatusOK, resolved.Code, resolved.Body.String())

	assert.Equal(t, []classListEntryChangeRow{
		{Action: "created", NewValue: "Rosa Wurzel (" + className + ")", ChangedBy: account.ID},
		{Action: "updated", OldValue: "Rosa Wurzel (" + className + ")", NewValue: "Rosa Wurzel (" + className + "-b)", ChangedBy: account.ID},
		{Action: "assigned", OldValue: "Rosa Wurzel (" + className + "-b)", MatchedStudentID: &student.ID, ChangedBy: account.ID},
	}, classListEntryTrail(t, db, entry.Data.ID), "every write ran the audited path")
}
