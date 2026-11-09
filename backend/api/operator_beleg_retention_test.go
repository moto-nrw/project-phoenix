// The moto team records the instructions of a Träger for its Belege and
// sees the clarification list in the Operator portal (#4720, ADR 0184).
// Every read and every change is in the operator audit; only an operator
// session reaches the routes.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil/routetest"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken/tokentest"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	organizationCompose "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"
	provisioningAPI "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/inbound/operator"
	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestOperatorRecordsBelegRetentionWithAudit(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	service, _ := buildOperatorProvisioningWithIdentity(t, db)
	operatorID := testpkg.CreateTestOperator(t, db).ID
	organizationID, _ := provisionTestSchool(t, db, service, operatorID, "traeger-belege")
	ctx := provisioningContext(t, db)
	operator := chi.NewRouter()
	mountOperatorBelegRetentionRoutes(operator, provisioningAPI.NewProvisioningResource(provisioningAPI.ProvisioningConfig{Service: service, BelegRetentionAudit: belegRetentionAuditOf(t, service)}))
	call := func(method, path string, body any) *httptest.ResponseRecorder {
		t.Helper()
		var payload bytes.Buffer
		if body != nil {
			require.NoError(t, json.NewEncoder(&payload).Encode(body))
		}
		req := httptest.NewRequest(method, path, &payload)
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(context.WithValue(ctx, sessiontoken.CtxClaims, sessiontoken.AppClaims{ID: int(operatorID), Scope: "platform"}))
		rec := httptest.NewRecorder()
		operator.ServeHTTP(rec, req)
		return rec
	}
	audits := func(action string) int {
		var count int
		require.NoError(t, db.NewRaw(`SELECT count(*) FROM platform.operator_audit_log
			WHERE operator_id = ? AND resource_type = 'beleg_retention' AND resource_id = ? AND action = ?`,
			operatorID, organizationID, action).Scan(ctx, &count))
		return count
	}
	base := fmt.Sprintf("/organizations/%d/beleg-retention", organizationID)

	rec := call(http.MethodGet, base, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, 1, audits("view"), "the read is in the operator audit")

	invalid := map[string]any{"beleg_type": "care_contract", "trigger_event": "issued", "period": 6,
		"legal_basis": "§ 257 HGB", "instructed_by": "Kämmerei", "instructed_on": "2026-11-09", "instruction_reference": "AVV-Nachtrag"}
	rec = call(http.MethodPost, base+"/instructions", invalid)
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"code":"general.input"`)
	assert.Zero(t, audits("create"), "a refused change leaves no audit entry")

	valid := invalid
	valid["trigger_event"] = "contract_ended"
	rec = call(http.MethodPost, base+"/instructions", valid)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	valid["period"] = 8
	rec = call(http.MethodPost, base+"/instructions", valid)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	assert.Equal(t, 2, audits("create"))

	rec = call(http.MethodPost, base+"/suspensions", map[string]any{"beleg_type": "sepa_debit", "reason": "Betriebsprüfung",
		"scope": "alle Lastschriften 2026", "responsible": "Kämmerei", "review_on": "2099-01-01"})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	suspension := decodeSchoolPortalData[parentbilling.RetentionSuspension](t, rec.Body.Bytes())

	rec = call(http.MethodGet, base, nil)
	require.Equal(t, http.StatusOK, rec.Code)
	overview := decodeSchoolPortalData[parentbilling.BelegRetentionOverview](t, rec.Body.Bytes())
	require.Len(t, overview.Instructions, 1, "a new instruction replaces the active one of its type")
	assert.Equal(t, 8, overview.Instructions[0].Period)
	require.Len(t, overview.Suspensions, 1)

	path := fmt.Sprintf("%s/suspensions/%d", base, suspension.ID)
	require.Equal(t, http.StatusNoContent, call(http.MethodDelete, path, nil).Code)
	assert.Equal(t, 1, audits("delete"))
	rec = call(http.MethodDelete, path, nil)
	assert.Equal(t, http.StatusNotFound, rec.Code, "a lifted suspension cannot be lifted again")
}

// TestBelegRetentionRoutesRefuseASchoolSession: the routes sit in the
// operator chain, so a tenant token gets 403 before any audit or read.
func TestBelegRetentionRoutesRefuseASchoolSession(t *testing.T) {
	t.Parallel()
	authService := &mockOperatorAuthService{}
	provisioningService := &protectedRouteProvisioningService{}
	router := testOperatorRouter(operatorIdentity{access: authService, sessions: operatorTestSessions(t, authService)}, provisioningService)
	token, err := tokentest.CreateJWT(routetest.TestTokenAuth(t), routetest.Claims{
		ID: 42, Sub: "school-user", Roles: []string{"admin"}, Scope: "tenant", FamilyID: "school-family",
	})
	require.NoError(t, err)
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		path := "/organizations/7/beleg-retention"
		if method == http.MethodPost {
			path += "/instructions"
		}
		req := httptest.NewRequest(method, path, bytes.NewBufferString(`{}`))
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusForbidden, rr.Code, "%s %s", method, rr.Body.String())
	}
	assert.Empty(t, provisioningService.reads, "no audit and no read for a school session")
}

func belegRetentionAuditOf(t *testing.T, provisioning organizationtenancy.Provisioning) organizationtenancy.BelegRetentionChangeAudit {
	t.Helper()
	audit, err := organizationCompose.BelegRetentionAudit(provisioning)
	require.NoError(t, err)
	return audit
}
