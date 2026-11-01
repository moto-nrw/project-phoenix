// Router-level tests for the school's own wage types and surcharge rules on
// /payroll (#4807): config:manage like the fixed Lohnart settings, IDs as
// strings on the wire, stable error codes, and a settings audit row per
// change.
package timetracking

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const payrollRulesPath = "/staff/time-tracking/payroll-rules"

func TestPayrollRulesAPI_ConfigManageMaintainsWageTypesAndRules(t *testing.T) {
	t.Parallel()
	ctx := setupOverviewAPI(t)

	rec := ctx.get(payrollRulesPath, "time_tracking:manage")
	require.Equal(t, http.StatusForbidden, rec.Code, "time tracking managers do not maintain the payroll office's wage types")
	rec = ctx.post(payrollRulesPath+"/wage-types", `{"name":"Nachtzuschlag","number":"310","unit":"stunden"}`, "time_tracking:manage")
	require.Equal(t, http.StatusForbidden, rec.Code)

	rec = ctx.post(payrollRulesPath+"/wage-types", `{"name":" Nachtzuschlag ","number":"310","unit":"stunden"}`, "config:manage")
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var created struct {
		Data struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	assert.Equal(t, "Nachtzuschlag", created.Data.Name)
	require.NotEmpty(t, created.Data.ID, "IDs travel as strings")

	rule := fmt.Sprintf(`{"kind":"nacht","rate_type":"prozent","rate_hundredths":2500,"wage_type_id":%q}`, created.Data.ID)
	rec = ctx.post(payrollRulesPath+"/surcharge-rules", rule, "config:manage")
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"window_start_minute":1200`)
	assert.Contains(t, rec.Body.String(), `"window_end_minute":360`)

	rec = ctx.get(payrollRulesPath, "config:manage")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"wage_type_id":"`+created.Data.ID+`"`)

	var audits int
	require.NoError(t, ctx.tc.db.NewRaw(`SELECT count(*) FROM config.setting_audit WHERE tenant_id = ? AND setting_key LIKE 'payroll.%'`,
		testpkg.Tenant(t)).Scan(t.Context(), &audits))
	assert.Equal(t, 2, audits, "each change is one row in the settings audit")
}

func TestPayrollRulesAPI_StableErrorCodes(t *testing.T) {
	t.Parallel()
	ctx := setupOverviewAPI(t)

	rec := ctx.post(payrollRulesPath+"/wage-types", `{"name":"Sonntag","number":"12345","unit":"stunden"}`, "config:manage")
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"workforce.payroll_rule_invalid"`)

	rec = ctx.post(payrollRulesPath+"/wage-types", `{"name":"Sonntag","number":"320","unit":"betrag"}`, "config:manage")
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var created struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))

	rec = ctx.post(payrollRulesPath+"/wage-types", `{"name":"sonntag","unit":"betrag"}`, "config:manage")
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"workforce.payroll_wage_type_name_taken"`)

	rule := fmt.Sprintf(`{"kind":"sonntag","rate_type":"betrag","rate_hundredths":250,"wage_type_id":%q}`, created.Data.ID)
	rec = ctx.post(payrollRulesPath+"/surcharge-rules", rule, "config:manage")
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	req := httptest.NewRequest(http.MethodDelete, payrollRulesPath+"/wage-types/"+created.Data.ID, nil)
	rec = ctx.do(req, "config:manage")
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"workforce.payroll_wage_type_in_use"`)

	rec = ctx.put(payrollRulesPath+"/absence-types/999999999", `{"wage_type_id":null}`, "config:manage")
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"workforce.payroll_rule_not_found"`)
}
