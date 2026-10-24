package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/modules/communication/http/operatorannouncements"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	provisioningAPI "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/inbound/operator"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOperatorAuditAnnouncementViews(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := provisioningContext(t, db)
	operatorID := testpkg.CreateTestOperator(t, db).ID
	_, account := testpkg.CreateTestPersonWithAccount(t, db, "Audit", "Viewer")
	module := buildCommunication(t, db, &auditRecorder{})
	announcement := newAnnouncement("Audit view fixture", testpkg.Tenant(t))
	require.NoError(t, module.CreateAnnouncement(ctx, announcement, operatorID, nil))
	require.NoError(t, module.MarkSeen(ctx, account.ID, announcement.ID))
	resource := provisioningAPI.NewProvisioningResource(provisioningAPI.ProvisioningConfig{Service: buildOperatorProvisioning(t, db)})
	router := chi.NewRouter()
	mountOperatorAnnouncementRoutes(router, operatorannouncements.NewAnnouncementsResource(module), resource)
	request := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/announcements/%d/views?search=private-filter", announcement.ID), nil)
		req.RemoteAddr = "198.51.100.45:12345"
		req = req.WithContext(context.WithValue(ctx, sessiontoken.CtxClaims, sessiontoken.AppClaims{ID: int(operatorID), Scope: "platform"}))
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		return response
	}

	response := request()
	require.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), `"user_name":"Audit Viewer"`)
	var entries []struct {
		ResourceType string
		ResourceID   int64
		RequestIP    string
		Changes      json.RawMessage
	}
	require.NoError(t, db.NewRaw(`SELECT resource_type, resource_id, host(request_ip) AS request_ip, changes
		FROM platform.operator_audit_log WHERE operator_id = ? AND action = 'view'`, operatorID).Scan(ctx, &entries))
	if assert.Len(t, entries, 1, "one audit entry must precede disclosure") {
		assert.Equal(t, "announcement", entries[0].ResourceType)
		assert.Equal(t, announcement.ID, entries[0].ResourceID)
		assert.Equal(t, "198.51.100.45", entries[0].RequestIP)
		assert.JSONEq(t, `{"view":"announcement_views"}`, string(entries[0].Changes))
	}

	for _, tc := range []struct{ path, view, changes string }{
		{"/announcements/", "announcements", `{"view":"announcements"}`},
		{fmt.Sprintf("/announcements/%d", announcement.ID), "announcement", `{"view":"announcement"}`},
	} {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		req.RemoteAddr = "198.51.100.45:12345"
		req = req.WithContext(context.WithValue(ctx, sessiontoken.CtxClaims, sessiontoken.AppClaims{ID: int(operatorID), Scope: "platform"}))
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		require.Equal(t, http.StatusOK, response.Code)
		var entries []struct {
			ResourceID *int64
			RequestIP  string
			Changes    json.RawMessage
		}
		require.NoError(t, db.NewRaw(`SELECT resource_id, host(request_ip) AS request_ip, changes FROM platform.operator_audit_log
			WHERE operator_id = ? AND action = 'view' AND changes->>'view' = ?`, operatorID, tc.view).Scan(ctx, &entries))
		require.Len(t, entries, 1)
		assert.JSONEq(t, tc.changes, string(entries[0].Changes))
		assert.Equal(t, "198.51.100.45", entries[0].RequestIP)
		if tc.view == "announcement" {
			require.NotNil(t, entries[0].ResourceID)
			assert.Equal(t, announcement.ID, *entries[0].ResourceID)
		} else {
			assert.Nil(t, entries[0].ResourceID)
		}
	}
	_, err := db.ExecContext(ctx, `CREATE FUNCTION platform.reject_operator_audit() RETURNS trigger LANGUAGE plpgsql AS $$
	BEGIN RAISE EXCEPTION 'injected audit failure'; END $$;
	CREATE TRIGGER reject_operator_audit BEFORE INSERT ON platform.operator_audit_log
	FOR EACH ROW EXECUTE FUNCTION platform.reject_operator_audit()`)
	require.NoError(t, err)
	response = request()
	assert.Equal(t, http.StatusServiceUnavailable, response.Code)
	assert.Contains(t, response.Body.String(), `"code":"operator.audit_unavailable"`)
	for _, private := range []string{"user_id", "user_name", "seen_at", "dismissed", "Audit Viewer", account.Email, "private-filter", "injected audit failure"} {
		assert.NotContains(t, response.Body.String(), private)
	}
	// Announcement content is free text, not necessarily data about the
	// internal author only. Its list/detail and write responses need the
	// same disclosure barrier as the recipient view details.
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/announcements/"},
		{http.MethodGet, fmt.Sprintf("/announcements/%d", announcement.ID)},
		{http.MethodPost, "/announcements/"},
		{http.MethodPut, fmt.Sprintf("/announcements/%d", announcement.ID)},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		req = req.WithContext(context.WithValue(ctx, sessiontoken.CtxClaims, sessiontoken.AppClaims{ID: int(operatorID), Scope: "platform"}))
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		assert.Equal(t, http.StatusServiceUnavailable, response.Code, tc.method+" "+tc.path)
		assert.Contains(t, response.Body.String(), `"code":"operator.audit_unavailable"`)
		assert.NotContains(t, response.Body.String(), `"data"`)
	}
}
