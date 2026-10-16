package timetracking

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The transfer endpoint's wire contract (#3050).
//
// The export selection travels in the BODY. The first version read it from the
// query string, which the POST proxy route does not forward — every transfer
// arrived without parameters and answered 400. These tests pin the contract on
// both sides of that mistake.

func postTransfer(t *testing.T, tc *testContext, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(
		http.MethodPost,
		"/staff/time-tracking/export/sftp",
		strings.NewReader(body),
	)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	tc.router.ServeHTTP(rec, req)
	return rec
}

func TestTransferExport_ReadsTheSelectionFromTheBody(t *testing.T) {
	t.Parallel()
	tc := setupStaffRoute(t)
	token := authToken(t, permissions.TimeTrackingManage)

	// No query string at all — exactly what the proxy route forwards.
	rec := postTransfer(t, tc, token, `{"year":2026,"month":8,"format":"csv","granularity":"month","time_format":"hhmm"}`)

	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
	// No counterpart is configured in these tests, so the honest answer is
	// "not configured" — and it arrives as a 200 so the journal row of a real
	// attempt would survive the tenant transaction.
	assert.Contains(t, rec.Body.String(), "not_configured")
}

func TestTransferExport_RejectsARequestWithoutASelection(t *testing.T) {
	t.Parallel()
	tc := setupStaffRoute(t)
	token := authToken(t, permissions.TimeTrackingManage)

	for name, body := range map[string]string{
		"empty body":   ``,
		"empty object": `{}`,
		"no year":      `{"month":8,"format":"csv"}`,
		"not json":     `year=2026&format=csv`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			rec := postTransfer(t, tc, token, body)
			assert.Equal(t, http.StatusBadRequest, rec.Code, "body: %s", rec.Body.String())
		})
	}
}

// The transfer discloses the whole staff's working time, so it sits behind the
// same permission as the export itself.
func TestTransferExport_RequiresTheExportPermission(t *testing.T) {
	t.Parallel()
	tc := setupStaffRoute(t)
	token := authToken(t, permissions.UsersRead)

	rec := postTransfer(t, tc, token, `{"year":2026,"month":8,"format":"csv"}`)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestExportSFTPStatus_ReportsAnUnconfiguredCounterpart(t *testing.T) {
	t.Parallel()
	tc := setupStaffRoute(t)
	token := authToken(t, permissions.TimeTrackingManage)

	req := httptest.NewRequest(http.MethodGet, "/staff/time-tracking/export/sftp-status", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	tc.router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, `"ready":false`)
	// The status must never carry credentials, not even masked ones.
	assert.NotContains(t, body, "password")
}
