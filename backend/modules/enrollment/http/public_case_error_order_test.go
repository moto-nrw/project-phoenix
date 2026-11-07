package enrollmenthttp_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	enrollment "github.com/moto-nrw/project-phoenix/modules/enrollment"
	compose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// A stored malformed child must win over a later message-read failure. The
// permission fault is confined to this fixture's disposable database clone.
func TestEnrollmentPublicCaseChildErrorPrecedesMessages(t *testing.T) {
	t.Parallel()
	f := newEnrollmentGrowthFixture(t)
	env := f.env
	ctx := testpkg.Ctx(t)
	submitted, err := f.resource.RequestService.Submit(ctx, validSubmission(t, env.phaseID))
	require.NoError(t, err)
	row := &enrollment.ChangeRequest{RequestID: submitted.Request.ID, Origin: enrollment.ChangeRequestOriginParent, Status: enrollment.ChangeRequestStatusRejected, BaseSnapshot: json.RawMessage(`{}`), ProposedSnapshot: json.RawMessage(`{}`), Diff: json.RawMessage(`{}`)}
	require.NoError(t, testpkg.TenantCommand1(t, ctx, env.db, env.repos.Enrollment().InsertChangeRequest, row))
	_, err = env.db.NewRaw("UPDATE enrollment.request_children SET custom_data = '[]'::jsonb WHERE tenant_id = ? AND request_id = ?", testpkg.Tenant(t), submitted.Request.ID).Exec(ctx)
	require.NoError(t, err)
	_, err = env.db.NewRaw("REVOKE SELECT ON enrollment.change_request_messages FROM phoenix_tenant").Exec(ctx)
	require.NoError(t, err)
	f.resource.ChangeRequestService = newTestChangeRequestService(owners.EnrollmentChangeRequestSources{Requests: env.repos.Enrollment(), Children: env.repos.Enrollment(), Guardians: env.repos.Enrollment(), Catalog: env.repos.Enrollment(), CareOfferingRepo: compose.NewCareOfferingRecords(env.reposCare.CarePlan), Settings: f.settings.Settings, OutboxEnqueuer: env.outbox, ParentsURL: "http://parents.localhost:3000", FrontendURL: "http://localhost:3000"})
	rec := httptest.NewRecorder()
	f.router(t).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/requests/"+submitted.Request.StatusToken+"/change-requests", nil))
	// The shared error envelope uses the (empty here) correlation ID as instance.
	var body map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	wire, err := json.Marshal(body)
	require.NoError(t, err)
	t.Logf("wire status=%d headers=%v body=%s", rec.Code, rec.Header(), wire)
	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.Equal(t, http.Header{"Content-Type": []string{"application/json"}}, rec.Header())
	require.JSONEq(t, `{"type":"https://moto-app.de/help/fehlermeldungen#anleitung-unerwarteter-fehler","title":"Internal Server Error","status":500,"detail":"enrollment operation failed","instance":"","code":"general.server"}`, string(wire))
	var detail string
	require.NoError(t, json.Unmarshal(body["detail"], &detail))
	require.Equal(t, "enrollment operation failed", detail)
	_, aggregateErr := f.resource.ChangeRequestService.ListPublic(ctx, submitted.Request.StatusToken)
	t.Logf("capability error=%v", aggregateErr)
	require.EqualError(t, aggregateErr, "change request: list aggregate children: json: cannot unmarshal array into Go value of type map[string]interface {}")
	require.JSONEq(t, `"general.server"`, string(body["code"]))
}
