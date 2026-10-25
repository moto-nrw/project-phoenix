// The moto team deactivates a Träger and records its erasure order (#4657,
// spec #4611 user story 11): deactivation ends every Träger session at once,
// reactivation lets the Träger-Büro in again, and the erasure order of a
// deactivated Träger opens a grace period in which it can be withdrawn. Every
// step is in the operator audit.
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
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	provisioningAPI "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/inbound/operator"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// traegerLifecycleFixture holds the operator routes over one provisioned
// Träger; the owners they were composed over stay local to the constructor.
type traegerLifecycleFixture struct {
	db             *bun.DB
	ctx            context.Context
	operatorID     int64
	organizationID int64
	operator       http.Handler
}

func newTraegerLifecycleFixture(t *testing.T, label string) (traegerLifecycleFixture, identityaccess.Owner) {
	t.Helper()
	db := testpkg.SetupIsolatedTestDB(t)
	service, identity := buildOperatorProvisioningWithIdentity(t, db)
	operatorID := testpkg.CreateTestOperator(t, db).ID
	organizationID, _ := provisionTestSchool(t, db, service, operatorID, label)
	resource := provisioningAPI.NewProvisioningResource(provisioningAPI.ProvisioningConfig{Service: service})
	operator := chi.NewRouter()
	mountOperatorProvisioningRoutes(operator, resource)
	return traegerLifecycleFixture{
		db: db, ctx: provisioningContext(t, db),
		operatorID: operatorID, organizationID: organizationID, operator: operator,
	}, identity
}

func (f traegerLifecycleFixture) call(t *testing.T, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var payload bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&payload).Encode(body))
	}
	req := httptest.NewRequest(method, path, &payload)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(f.ctx, sessiontoken.CtxClaims, sessiontoken.AppClaims{ID: int(f.operatorID), Scope: "platform"}))
	rec := httptest.NewRecorder()
	f.operator.ServeHTTP(rec, req)
	return rec
}

// setActive switches the Träger on or off through the operator's edit route.
func (f traegerLifecycleFixture) setActive(t *testing.T, active bool) *httptest.ResponseRecorder {
	t.Helper()
	var organization struct {
		Name string `bun:"name"`
		Slug string `bun:"slug"`
	}
	require.NoError(t, f.db.NewRaw(`SELECT name, slug FROM platform.organizations WHERE id = ?`, f.organizationID).Scan(f.ctx, &organization))
	return f.call(t, http.MethodPut, fmt.Sprintf("/organizations/%d", f.organizationID),
		map[string]any{"name": organization.Name, "slug": organization.Slug, "active": active})
}

// openOrder reads the open erasure order; nil without one.
func (f traegerLifecycleFixture) openOrder(t *testing.T, path string) *organizationtenancy.TraegerErasureOrder {
	t.Helper()
	rec := f.call(t, http.MethodGet, path, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var body struct {
		Data *organizationtenancy.TraegerErasureOrder `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	return body.Data
}

func (f traegerLifecycleFixture) audited(t *testing.T, action string) int {
	t.Helper()
	var count int
	require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM platform.operator_audit_log
		WHERE operator_id = ? AND resource_type = 'organization' AND resource_id = ? AND action = ?`,
		f.operatorID, f.organizationID, action).Scan(f.ctx, &count))
	return count
}

func TestOperatorDeactivationEndsEveryTraegerSessionAndReactivationLetsInAgain(t *testing.T) {
	t.Parallel()
	f, identity := newTraegerLifecycleFixture(t, "traeger-aus")
	password := testpkg.NewTestPassword()
	account := traegerAccount(t, f.db, password)
	testpkg.CreateTestTraegerMember(t, f.db, f.organizationID, account.ID, identityaccess.TraegerRoleAdministration)
	unit := testpkg.TenantRuntime(t, f.db)
	traeger, _ := traegerPortalOn(t, f.db, unit)
	code, session := traegerSignIn(t, traeger, account.Email, password, 0)
	require.Equal(t, http.StatusOK, code)
	listed, _ := traegerSchools(t, traeger, session.AccessToken)
	require.Equal(t, http.StatusOK, listed.Code, listed.Body.String())

	rec := f.setActive(t, false)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var sessions int
	require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM auth.tokens WHERE organization_id = ?`, f.organizationID).Scan(f.ctx, &sessions))
	assert.Zero(t, sessions, "deactivation deletes every Träger session")
	var ended string
	require.NoError(t, f.db.NewRaw(`SELECT changes->>'traeger_sessions_ended' FROM platform.operator_audit_log
		WHERE operator_id = ? AND resource_type = 'organization' AND resource_id = ? AND action = 'update'
		ORDER BY id DESC LIMIT 1`, f.operatorID, f.organizationID).Scan(f.ctx, &ended))
	assert.Equal(t, "1", ended, "the operator audit records the ended sessions")

	listed, _ = traegerSchools(t, traeger, session.AccessToken)
	assert.Equal(t, http.StatusUnauthorized, listed.Code, "an access token still in flight reaches no school: %s", listed.Body.String())
	_, _, err := identity.RefreshTokenWithAudit(tenant.WithUnitOfWork(context.Background(), unit), session.RefreshToken, "", "", "")
	assert.Error(t, err, "the ended session cannot be refreshed")
	code, _ = traegerSignIn(t, traeger, account.Email, password, 0)
	assert.Equal(t, http.StatusForbidden, code, "a deactivated Träger lets nobody sign in")

	rec = f.setActive(t, true)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	code, session = traegerSignIn(t, traeger, account.Email, password, 0)
	require.Equal(t, http.StatusOK, code, "reactivation lets the Träger-Büro in again")
	listed, _ = traegerSchools(t, traeger, session.AccessToken)
	assert.Equal(t, http.StatusOK, listed.Code, listed.Body.String())
}

func TestOperatorRecordsAndWithdrawsTheErasureOrderOfADeactivatedTraeger(t *testing.T) {
	t.Parallel()
	f, _ := newTraegerLifecycleFixture(t, "traeger-loeschen")
	path := fmt.Sprintf("/organizations/%d/erasure-order", f.organizationID)
	order := map[string]string{"order_reference": "LOESCH-4657", "confirmed_on": "2026-01-02", "erase_not_before": "2099-12-31"}

	rec := f.call(t, http.MethodPost, path, order)
	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Contains(t, rec.Body.String(), `"code":"provisioning.traeger_active"`, "an active Träger gets no erasure order")

	require.Equal(t, http.StatusOK, f.setActive(t, false).Code)
	for name, invalid := range map[string]map[string]string{
		"personal reference":         {"order_reference": "Max Mustermann", "confirmed_on": "2026-01-02", "erase_not_before": "2099-12-31"},
		"confirmation in the future": {"order_reference": "LOESCH-4657", "confirmed_on": "2099-01-01", "erase_not_before": "2099-12-31"},
		"no grace period":            {"order_reference": "LOESCH-4657", "confirmed_on": "2026-01-02", "erase_not_before": "2026-01-03"},
		"no date":                    {"order_reference": "LOESCH-4657", "confirmed_on": "", "erase_not_before": "2099-12-31"},
	} {
		rec = f.call(t, http.MethodPost, path, invalid)
		assert.Equal(t, http.StatusBadRequest, rec.Code, "%s: %s", name, rec.Body.String())
	}

	assert.Nil(t, f.openOrder(t, path), "no order yet")

	rec = f.call(t, http.MethodPost, path, order)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	recorded := decodeSchoolPortalData[organizationtenancy.TraegerErasureOrder](t, rec.Body.Bytes())
	assert.Equal(t, "LOESCH-4657", recorded.OrderReference)
	assert.Equal(t, "2026-01-02", recorded.ConfirmedOn.String())
	assert.Equal(t, "2099-12-31", recorded.EraseNotBefore.String())
	assert.Equal(t, f.operatorID, recorded.RecordedBy)

	open := f.openOrder(t, path)
	require.NotNil(t, open)
	assert.Equal(t, recorded.ID, open.ID)
	rec = f.call(t, http.MethodPost, path, order)
	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Contains(t, rec.Body.String(), `"code":"provisioning.traeger_erasure_order_open"`, "one open order per Träger")
	rec = f.setActive(t, true)
	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Contains(t, rec.Body.String(), `"code":"provisioning.traeger_erasure_order_open"`, "the order is withdrawn before the Träger comes back")

	// Once the erasure CLI started, a run that failed halfway resumes with
	// the same order: it can no longer be withdrawn.
	_, err := f.db.ExecContext(f.ctx, `UPDATE platform.organization_erasure_orders SET started_at = clock_timestamp() WHERE id = ?`, recorded.ID)
	require.NoError(t, err)
	rec = f.call(t, http.MethodDelete, path, nil)
	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Contains(t, rec.Body.String(), `"code":"provisioning.traeger_erasure_started"`)
	require.NotNil(t, f.openOrder(t, path).StartedAt, "the started order reports its start")
	_, err = f.db.ExecContext(f.ctx, `UPDATE platform.organization_erasure_orders SET started_at = NULL WHERE id = ?`, recorded.ID)
	require.NoError(t, err)

	rec = f.call(t, http.MethodDelete, path, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = f.call(t, http.MethodDelete, path, nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), `"code":"provisioning.traeger_erasure_order_not_found"`)
	assert.Nil(t, f.openOrder(t, path), "the withdrawn order is no longer open")
	assert.Equal(t, http.StatusOK, f.setActive(t, true).Code, "after the withdrawal the Träger can be reactivated")

	assert.Equal(t, 1, f.audited(t, "record_erasure_order"), "recording is in the operator audit")
	assert.Equal(t, 1, f.audited(t, "withdraw_erasure_order"), "the withdrawal is in the operator audit")
	var kept int
	require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM platform.organization_erasure_orders
		WHERE organization_id = ? AND withdrawn_at IS NOT NULL AND withdrawn_by = ?`, f.organizationID, f.operatorID).Scan(f.ctx, &kept))
	assert.Equal(t, 1, kept, "the withdrawn order stays as evidence")

	rec = f.call(t, http.MethodGet, fmt.Sprintf("/organizations/%d/erasure-order", f.organizationID+100000), nil)
	assert.Equal(t, http.StatusNotFound, rec.Code, "an unknown Träger has no order: %s", rec.Body.String())
}
