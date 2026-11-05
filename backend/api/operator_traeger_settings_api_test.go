// Router-level tests for the operator's view of the Träger settings (#5748):
// the operator reads and changes the settings of one Träger with the
// validation of the Träger-Portal, the change and its change-log entry name
// the operator, the operator audit records read and change, the Träger-Portal
// and the Träger's schools then read the value, and no other Träger does.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	provisioningAPI "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/inbound/operator"
	"github.com/moto-nrw/project-phoenix/modules/settings/records"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestOperatorChangesTheSettingsOfOneTraeger(t *testing.T) {
	t.Parallel()
	db, portal, _, unit := newTraegerPortal(t)
	ctx := testpkg.WithPackageTenantRuntime(context.Background())
	password := testpkg.NewTestPassword()
	own := traegerOrganization(t, db, "Träger Operator", true)
	foreign := traegerOrganization(t, db, "Träger Operator Nachbar", true)
	ownSchool := testpkg.CreateTestSchoolInOrganization(t, db, own, "OGS Operator Eigen")
	foreignSchool := testpkg.CreateTestSchoolInOrganization(t, db, foreign, "OGS Operator Nachbar")
	admin := traegerAccount(t, db, password)
	testpkg.CreateTestTraegerMember(t, db, own, admin.ID, identityaccess.TraegerRoleAdministration)
	operatorID := testpkg.CreateTestOperator(t, db).ID

	organizations, err := owners.NewOrganizationTenancy(testpkg.NewDatabase(t, db))
	require.NoError(t, err)
	traegerSettings, err := owners.BuildTraegerSettings(testpkg.NewDatabase(t, db), unit, slog.Default(), organizations, nil)
	require.NoError(t, err)
	operator := chi.NewRouter()
	mountOperatorTraegerSettingsRoutes(operator,
		provisioningAPI.NewProvisioningResource(provisioningAPI.ProvisioningConfig{Service: buildOperatorProvisioning(t, db)}),
		traegerSettings)
	call := func(method, path string, body any) *httptest.ResponseRecorder {
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
	base := fmt.Sprintf("/organizations/%d/traeger-settings", own)
	key := records.KeyParentBillingDebtorNumberOnConfirm
	operatorAudit := func(action string) int {
		var count int
		require.NoError(t, db.NewRaw(`SELECT count(*) FROM platform.operator_audit_log
			WHERE operator_id = ? AND resource_type = 'traeger_setting' AND resource_id = ? AND action = ?`, operatorID, own, action).Scan(ctx, &count))
		return count
	}

	rec := call(http.MethodGet, base+"/schema", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"key":"`+key+`"`, "the schema shows the Träger-level keys of the registry")
	assert.Contains(t, rec.Body.String(), `"writable":true`)
	assert.NotContains(t, rec.Body.String(), `"key":"`+records.KeyMFAMode+`"`, "school-only settings are no Träger settings")
	assert.Equal(t, 1, operatorAudit("view"), "reading is in the operator audit")

	rec = call(http.MethodPut, base+"/values/"+key, map[string]any{"value": "ja"})
	assert.Equal(t, http.StatusBadRequest, rec.Code, "the validation of the Träger-Portal applies: %s", rec.Body.String())
	rec = call(http.MethodPut, base+"/values/"+records.KeyMFAMode, map[string]any{"value": "off"})
	assert.Equal(t, http.StatusNotFound, rec.Code, "a school-only setting has no Träger value")
	rec = call(http.MethodPut, fmt.Sprintf("/organizations/%d/traeger-settings/values/%s", foreign+100000, key), map[string]any{"value": true})
	assert.Equal(t, http.StatusNotFound, rec.Code, "an unknown Träger is refused before the change: %s", rec.Body.String())

	rec = call(http.MethodPut, base+"/values/"+key, map[string]any{"value": true})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	// The entry commits after the registry's checks and before the change:
	// the invalid value and the school-only key left none, the unknown
	// Träger neither.
	assert.Equal(t, 1, operatorAudit("update"), "the change is in the operator audit")
	var unknown int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM platform.operator_audit_log
		WHERE operator_id = ? AND resource_type = 'traeger_setting' AND resource_id = ?`, operatorID, foreign+100000).Scan(ctx, &unknown))
	assert.Zero(t, unknown, "an unknown Träger leaves no entry")
	var entry struct {
		ChangedBy         *int64 `bun:"changed_by"`
		ChangedByOperator *int64 `bun:"changed_by_operator"`
		Action            string `bun:"action"`
	}
	require.NoError(t, db.NewRaw(`SELECT changed_by, changed_by_operator, action FROM config.traeger_setting_audit
		WHERE organization_id = ? AND setting_key = ? ORDER BY id DESC LIMIT 1`, own, key).Scan(ctx, &entry))
	assert.Equal(t, "set", entry.Action)
	assert.Nil(t, entry.ChangedBy, "an operator is no account of the Träger-Büro")
	require.NotNil(t, entry.ChangedByOperator, "the change log names the operator")
	assert.Equal(t, operatorID, *entry.ChangedByOperator)

	_, login := traegerSignIn(t, portal, admin.Email, password, 0)
	_, items := traegerSchemaItems(t, portal, login.AccessToken)
	require.Contains(t, items, key)
	assert.Equal(t, true, items[key].Value, "the Träger-Portal shows the value the operator set")
	assert.Equal(t, "traeger", items[key].Source)

	module, err := owners.NewSettingsTestModule(testpkg.NewDatabase(t, db), unit)
	require.NoError(t, err)
	ownValue, err := module.Settings.ResolveBoolForTenant(ctx, ownSchool, key)
	require.NoError(t, err)
	assert.True(t, ownValue, "the school of the Träger resolves the value")
	foreignValue, err := module.Settings.ResolveBoolForTenant(ctx, foreignSchool, key)
	require.NoError(t, err)
	assert.False(t, foreignValue, "the school of another Träger keeps the registry default")

	rec = call(http.MethodDelete, base+"/values/"+key, nil)
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	assert.Equal(t, 1, operatorAudit("delete"), "the reset is in the operator audit")
	_, items = traegerSchemaItems(t, portal, login.AccessToken)
	assert.Equal(t, "default", items[key].Source, "the reset returns the Träger to the default")
	var actions []string
	require.NoError(t, db.NewRaw(`SELECT action FROM config.traeger_setting_audit
		WHERE organization_id = ? AND setting_key = ? AND changed_by_operator = ? ORDER BY id`, own, key, operatorID).Scan(ctx, &actions))
	assert.Equal(t, []string{"set", "reset"}, actions, "both changes stand in the change log with the operator")
	var foreignChanges int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM config.traeger_setting_audit WHERE organization_id = ?`, foreign).Scan(ctx, &foreignChanges))
	assert.Zero(t, foreignChanges, "the other Träger has no change")
}
