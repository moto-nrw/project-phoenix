// Router-level tests for GET/PUT /api/staff/{id}/payroll-number (#1417):
// the Personalnummer feeds payroll, so it sits behind time_tracking:manage —
// deliberately NOT the users:read/users:update directory tier.
package staff_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPayrollNumberAPI_PermissionSplit(t *testing.T) {
	t.Parallel()

	ctx := setupOverviewAPI(t)
	target := testpkg.CreateTestStaff(t, ctx.tc.db, "Payroll", fmt.Sprintf("API-%d", time.Now().UnixNano()))
	path := fmt.Sprintf("/staff/%d/payroll-number", target.ID)

	// The directory tiers never see or write the payroll identifier.
	rec := ctx.get(path, "users:read")
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	rec = ctx.put(path, `{"personnel_number":"90100"}`, "users:update")
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())

	// time_tracking:manage writes and reads back.
	rec = ctx.put(path, `{"personnel_number":"90100","note":"Ersteinrichtung"}`, "time_tracking:manage")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"personnel_number":"90100"`)

	rec = ctx.get(path, "time_tracking:manage")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"personnel_number":"90100"`)
}

func TestPayrollNumberAPI_StableErrorCodes(t *testing.T) {
	t.Parallel()

	ctx := setupOverviewAPI(t)
	suffix := time.Now().UnixNano()
	first := testpkg.CreateTestStaff(t, ctx.tc.db, "Payroll", fmt.Sprintf("Erste-%d", suffix))
	second := testpkg.CreateTestStaff(t, ctx.tc.db, "Payroll", fmt.Sprintf("Zweite-%d", suffix))

	rec := ctx.put(fmt.Sprintf("/staff/%d/payroll-number", first.ID), `{"personnel_number":"90110"}`, "time_tracking:manage")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// Omitting the field is not a clear operation. Only explicit null clears.
	rec = ctx.put(fmt.Sprintf("/staff/%d/payroll-number", first.ID), `{"note":"fehlt"}`, "time_tracking:manage")
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	rec = ctx.get(fmt.Sprintf("/staff/%d/payroll-number", first.ID), "time_tracking:manage")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"personnel_number":"90110"`)

	// Duplicate within the tenant → 409 with a stable code on the wire.
	rec = ctx.put(fmt.Sprintf("/staff/%d/payroll-number", second.ID), `{"personnel_number":"90110"}`, "time_tracking:manage")
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"personnel_number_taken"`)

	// Malformed value → 400 with a stable code.
	rec = ctx.put(fmt.Sprintf("/staff/%d/payroll-number", second.ID), `{"personnel_number":"12a"}`, "time_tracking:manage")
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"personnel_number_invalid"`)

	// Clearing via null succeeds.
	rec = ctx.put(fmt.Sprintf("/staff/%d/payroll-number", first.ID), `{"personnel_number":null,"note":"Austritt"}`, "time_tracking:manage")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"personnel_number":null`)
}
