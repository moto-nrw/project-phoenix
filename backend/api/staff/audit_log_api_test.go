// Router-level tests for GET /api/staff/time-tracking/audit-log (#1417):
// the whole feed is personal data (affected person, acting person, free-text
// reasons), so it sits behind time_tracking:manage with no users:read tier.
package staff_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTimeTrackingAuditLogAPI_PermissionSplit(t *testing.T) {
	ctx := setupOverviewAPI(t)

	rec := ctx.get("/staff/time-tracking/audit-log", "time_tracking:manage")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	// Response envelope fields are the frontend contract.
	for _, field := range []string{`"events"`, `"retention_cutoff"`} {
		assert.Contains(t, rec.Body.String(), field)
	}

	// users:read sees the staff list, but never the audit feed.
	rec = ctx.get("/staff/time-tracking/audit-log", "users:read")
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())

	// vacation:approve alone is not enough either.
	rec = ctx.get("/staff/time-tracking/audit-log", "vacation:approve")
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
}

func TestTimeTrackingAuditLogAPI_Validation(t *testing.T) {
	ctx := setupOverviewAPI(t)

	for _, tc := range []struct {
		name string
		path string
	}{
		{"bad from date", "/staff/time-tracking/audit-log?from=2025-13-01"},
		{"bad staff id", "/staff/time-tracking/audit-log?staff_id=abc"},
		{"bad actor id", "/staff/time-tracking/audit-log?actor_id=-1"},
		{"bad limit", "/staff/time-tracking/audit-log?limit=0"},
		{"limit over max", "/staff/time-tracking/audit-log?limit=999"},
		{"unknown source", "/staff/time-tracking/audit-log?sources=payroll"},
		{"broken cursor", "/staff/time-tracking/audit-log?cursor=kaputt"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := ctx.get(tc.path, "time_tracking:manage")
			require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
		})
	}

	// Valid filters pass through.
	rec := ctx.get("/staff/time-tracking/audit-log?from=2025-08-01&to=2025-08-31&sources=session_edit,adjustment&limit=10", "time_tracking:manage")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}
