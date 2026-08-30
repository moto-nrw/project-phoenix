// Router-level tests for GET /api/settings/payroll-status (#1417): the
// payroll configuration state sits behind config:manage — the same tier that
// writes the payroll settings. config:read/config:update never see it (the
// response carries a staff-without-Personalnummer count).
package config_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/stretchr/testify/assert"
)

func TestPayrollStatusAPI_ManageOnly(t *testing.T) {
	t.Parallel()

	ctx := setupSettingsModule(t)
	router := ctx.resource.SettingsRouter()

	req := testutil.NewAuthenticatedRequest(t, "GET", "/payroll-status", nil,
		testutil.WithTestTenant(t),
	)
	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, 200)
	body := rr.Body.String()
	for _, field := range []string{
		`"categories"`, `"beraternummer"`, `"mandantennummer"`,
		`"lodas_header_complete"`, `"configured_categories"`, `"total_categories"`,
		`"staff_total"`, `"staff_without_personnel_number"`,
	} {
		assert.Contains(t, body, field)
	}
	// Fresh tenant configuration is empty by design — nothing invented.
	assert.Contains(t, body, `"lodas_header_complete":false`)
}
