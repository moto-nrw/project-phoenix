// Router-level tests for the cross-staff time-tracking export (#1417 2b):
// GET /api/staff/time-tracking/export. They pin the permission gate (a bulk
// export of per-person working-time data must not be reachable with
// users:read), the parameter validation, and the download response shape.
package staff_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupExportAPI(t *testing.T) *overviewAPIContext {
	t.Helper()
	ctx := setupOverviewAPI(t)
	// The export writes an audit.data_access_log row per download; remove them
	// before the account fixture cleanup or its FK blocks the delete.
	t.Cleanup(func() {
		_, _ = ctx.tc.db.NewDelete().
			ModelTableExpr("audit.data_access_log AS t").
			Where("t.resource_type = ?", "time_tracking_export").
			Exec(context.Background())
	})
	return ctx
}

func TestTimeTrackingExportAPI_PermissionSplit(t *testing.T) {
	t.Parallel()

	ctx := setupExportAPI(t)
	path := "/staff/time-tracking/export?year=2026&month=1"

	// users:read sees the staff list but must NOT get the working-time file.
	rec := ctx.get(path, "users:read")
	assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())

	rec = ctx.get(path, "time_tracking:manage")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "text/csv; charset=utf-8", rec.Header().Get("Content-Type"))
	assert.Contains(t, rec.Header().Get("Content-Disposition"), "zeiterfassung_alle_2026-01.csv")
	body := rec.Body.String()
	assert.True(t, strings.HasPrefix(body, "\xEF\xBB\xBF"), "UTF-8 BOM for Excel")
	assert.Contains(t, body, "Übertrag Monatsende")
}

func TestTimeTrackingExportAPI_Validation(t *testing.T) {
	t.Parallel()

	ctx := setupExportAPI(t)

	for _, path := range []string{
		"/staff/time-tracking/export",                              // year missing
		"/staff/time-tracking/export?year=abc",                     // year not an int
		"/staff/time-tracking/export?year=2026&month=13",           // month out of range
		"/staff/time-tracking/export?year=2026&month=1&format=pdf", // unknown format
		"/staff/time-tracking/export?year=2026&month=1&granularity=weekly",
		"/staff/time-tracking/export?year=2026&month=1&time_format=seconds",
		"/staff/time-tracking/export?year=2999&month=1", // future
	} {
		rec := ctx.get(path, "time_tracking:manage")
		assert.Equal(t, http.StatusBadRequest, rec.Code, "path %s: %s", path, rec.Body.String())
	}
}

func TestTimeTrackingExportAPI_Formats(t *testing.T) {
	t.Parallel()

	ctx := setupExportAPI(t)

	rec := ctx.get("/staff/time-tracking/export?year=2026&month=1&format=xlsx", "time_tracking:manage")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", rec.Header().Get("Content-Type"))
	assert.Contains(t, rec.Header().Get("Content-Disposition"), ".xlsx")

	// Day granularity + whole-year selection.
	rec = ctx.get("/staff/time-tracking/export?year=2026&granularity=day", "time_tracking:manage")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "Wochentag")
	assert.Contains(t, rec.Header().Get("Content-Disposition"), "zeiterfassung_alle_tage_2026.csv")
}

// The DATEV routes share the export permission gate; with the tenant's
// payroll configuration empty (the legitimate initial state) both the report
// and the download refuse with the stable payroll_config_incomplete code
// instead of producing an empty file.
func TestTimeTrackingExportAPI_Datev(t *testing.T) {
	t.Parallel()

	ctx := setupExportAPI(t)
	reportPath := "/staff/time-tracking/export/datev-report?year=2026&month=1&format=datev_lodas"

	rec := ctx.get(reportPath, "users:read")
	assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())

	rec = ctx.get(reportPath, "time_tracking:manage")
	assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "payroll_config_incomplete")

	rec = ctx.get("/staff/time-tracking/export?year=2026&month=1&format=datev_lug", "time_tracking:manage")
	assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())

	// A DATEV request without a single month is a caller mistake.
	rec = ctx.get("/staff/time-tracking/export/datev-report?year=2026&format=datev_lodas", "time_tracking:manage")
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())

	for _, format := range []string{"", "csv", "xlsx"} {
		path := "/staff/time-tracking/export/datev-report?year=2026&month=1"
		if format != "" {
			path += "&format=" + format
		}
		rec = ctx.get(path, "time_tracking:manage")
		assert.Equal(t, http.StatusBadRequest, rec.Code, "format %q: %s", format, rec.Body.String())
	}
}
