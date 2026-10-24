package operator_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The moto team records the retention windows of each school and alone can
// halt the nightly deletion (ADR 0048).
func TestOperatorRecordsRetentionWindowsWithinTheirRange(t *testing.T) {
	t.Parallel()
	ctx := setupOperatorSettingsRoute(t)
	path := schoolPath(t, "/settings/values/gdpr.staff_message_retention_days")

	put := newOperatorRequest(t, http.MethodPut, path, map[string]any{"value": 400})
	testutil.AssertSuccessResponse(t, testutil.ExecuteRequest(ctx.router, put), http.StatusOK)

	for _, outOfRange := range []int{29, 731} {
		req := newOperatorRequest(t, http.MethodPut, path, map[string]any{"value": outOfRange})
		testutil.AssertErrorResponse(t, testutil.ExecuteRequest(ctx.router, req), http.StatusBadRequest)
	}

	var stored string
	require.NoError(t, ctx.db.NewSelect().TableExpr("config.setting_values").ColumnExpr("value::text").
		Where("tenant_id = ?", testpkg.Tenant(t)).
		Where("setting_key = ?", "gdpr.staff_message_retention_days").
		Scan(context.Background(), &stored))
	assert.Equal(t, "400", stored, "values outside the range never replace the recorded window")

	reset := newOperatorRequest(t, http.MethodDelete, path, nil)
	testutil.AssertSuccessResponse(t, testutil.ExecuteRequest(ctx.router, reset), http.StatusNoContent)
}

func TestOperatorSchemaShowsTheDeletionSwitchAndWritableRetentionWindows(t *testing.T) {
	t.Parallel()
	ctx := setupOperatorSettingsRoute(t)

	rr := testutil.ExecuteRequest(ctx.router, newOperatorRequest(t, http.MethodGet, schoolPath(t, "/settings/schema"), nil))
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	body := rr.Body.String()

	assert.Contains(t, body, `"key":"gdpr.data_cleanup_enabled"`, "the moto team can halt the nightly deletion")
	assert.Contains(t, body, `"access_policy":"operator_managed"`)

	put := newOperatorRequest(t, http.MethodPut, schoolPath(t, "/settings/values/gdpr.data_cleanup_enabled"), map[string]any{"value": false})
	testutil.AssertSuccessResponse(t, testutil.ExecuteRequest(ctx.router, put), http.StatusOK)
}
