// The moto team grants the release „Personal beim Träger“ with the statement
// the Träger confirmed and withdraws it in the Operator portal (#4705, ADR
// 0089 section 2, ADR 0232). Every read and change is in the operator audit;
// a withdrawal waits until the Träger keeps no record.
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

	"github.com/moto-ogs/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-ogs/project-phoenix/modules/organizationtenancy"
	provisioningAPI "github.com/moto-ogs/project-phoenix/modules/organizationtenancy/inbound/operator"
	"github.com/moto-ogs/project-phoenix/modules/workforce"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

func TestOperatorGrantsTheStaffReleaseWithAudit(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	service, _ := buildOperatorProvisioningWithIdentity(t, db)
	operatorID := testpkg.CreateTestOperator(t, db).ID
	organizationID, _ := provisionTestSchool(t, db, service, operatorID, "traeger-personal")
	ctx := provisioningContext(t, db)
	staffRelease, ok := belegRetentionAuditOf(t, service).(organizationtenancy.TraegerStaffReleaseChangeAudit)
	require.True(t, ok)
	operator := chi.NewRouter()
	mountOperatorTraegerStaffReleaseRoutes(operator, provisioningAPI.NewProvisioningResource(provisioningAPI.ProvisioningConfig{
		Service: service, TraegerStaffReleaseAudit: staffRelease,
	}))
	call := func(method string, body any) *httptest.ResponseRecorder {
		t.Helper()
		var payload bytes.Buffer
		if body != nil {
			require.NoError(t, json.NewEncoder(&payload).Encode(body))
		}
		req := httptest.NewRequest(method, fmt.Sprintf("/organizations/%d/staff-release", organizationID), &payload)
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(context.WithValue(ctx, sessiontoken.CtxClaims, sessiontoken.AppClaims{ID: int(operatorID), Scope: "platform"}))
		rec := httptest.NewRecorder()
		operator.ServeHTTP(rec, req)
		return rec
	}
	audits := func(action string) int {
		var count int
		require.NoError(t, db.NewRaw(`SELECT count(*) FROM platform.operator_audit_log
			WHERE operator_id = ? AND resource_type = 'traeger_staff_release' AND resource_id = ? AND action = ?`,
			operatorID, organizationID, action).Scan(ctx, &count))
		return count
	}
	state := func() workforce.TraegerStaffReleaseState {
		rec := call(http.MethodGet, nil)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return decodeSchoolPortalData[workforce.TraegerStaffReleaseState](t, rec.Body.Bytes())
	}

	assert.Nil(t, state().Release)
	assert.Equal(t, 1, audits("view"), "the read is in the operator audit")

	rec := call(http.MethodPost, map[string]string{"representation": "maybe"})
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Equal(t, "workforce.traeger_staff_release_invalid", refusalOf(t, rec).Code)
	assert.Zero(t, audits("grant"), "a refused grant leaves no audit entry")

	rec = call(http.MethodPost, map[string]string{"representation": workforce.TraegerStaffRepresentationNone})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	assert.Equal(t, 1, audits("grant"))
	released := state()
	require.NotNil(t, released.Release)
	assert.Equal(t, workforce.TraegerStaffRepresentationNone, released.Release.Representation)

	rec = call(http.MethodPost, map[string]string{"representation": workforce.TraegerStaffRepresentationAgreement})
	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "workforce.traeger_staff_release_exists", refusalOf(t, rec).Code)
	assert.Equal(t, 1, audits("grant"), "the refused second grant rolls its audit entry back")

	_, err := db.ExecContext(ctx, `INSERT INTO workforce.staff_records (organization_id, first_name, last_name) VALUES (?, 'Mira', 'Sommer')`, organizationID)
	require.NoError(t, err)
	assert.Equal(t, 1, state().Records)
	rec = call(http.MethodDelete, nil)
	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "workforce.traeger_staff_release_in_use", refusalOf(t, rec).Code)
	assert.Zero(t, audits("revoke"))

	_, err = db.ExecContext(ctx, `DELETE FROM workforce.staff_records WHERE organization_id = ?`, organizationID)
	require.NoError(t, err)
	rec = call(http.MethodDelete, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, 1, audits("revoke"))
	assert.Nil(t, state().Release)
}
