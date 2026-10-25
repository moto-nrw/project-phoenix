// Router-level tests for the Träger settings (ADR 0086, #4652): the Träger
// reads and writes its own values in the Träger-Portal, a role without the
// right gets 403, and no Träger reaches the values of another.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/settings/records"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func traegerSettingsRequest(t *testing.T, router http.Handler, token, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

type traegerSchemaItem struct {
	Key      string `json:"key"`
	Value    any    `json:"value"`
	Source   string `json:"source"`
	Writable bool   `json:"writable"`
}

func traegerSchemaItems(t *testing.T, router http.Handler, token string) (int, map[string]traegerSchemaItem) {
	t.Helper()
	rec := traegerSettingsRequest(t, router, token, http.MethodGet, "/traeger/settings/schema", nil)
	if rec.Code != http.StatusOK {
		return rec.Code, nil
	}
	schema := decodeSchoolPortalData[struct {
		Tabs []struct {
			Categories []struct {
				Items []traegerSchemaItem `json:"items"`
			} `json:"categories"`
		} `json:"tabs"`
	}](t, rec.Body.Bytes())
	items := map[string]traegerSchemaItem{}
	for _, tab := range schema.Tabs {
		for _, category := range tab.Categories {
			for _, item := range category.Items {
				items[item.Key] = item
			}
		}
	}
	return rec.Code, items
}

func TestTraegerSettingsWriteReachesOnlyTheTraegerOfTheSession(t *testing.T) {
	t.Parallel()
	db, router, _, unit := newTraegerPortal(t)
	password := testpkg.NewTestPassword()
	own := traegerOrganization(t, db, "Träger Einstellungen", true)
	foreign := traegerOrganization(t, db, "Träger Nachbar", true)
	ownSchool := testpkg.CreateTestSchoolInOrganization(t, db, own, "OGS Eigen")
	foreignSchool := testpkg.CreateTestSchoolInOrganization(t, db, foreign, "OGS Nachbar")
	admin := traegerAccount(t, db, password)
	testpkg.CreateTestTraegerMember(t, db, own, admin.ID, identityaccess.TraegerRoleAdministration)
	_, login := traegerLogin(t, router, admin.Email, password, 0)
	key := records.KeyParentNewsEnabled

	rec := traegerSettingsRequest(t, router, login.AccessToken, http.MethodPut, "/traeger/settings/values/"+key, map[string]any{"value": false})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	code, items := traegerSchemaItems(t, router, login.AccessToken)
	require.Equal(t, http.StatusOK, code)
	require.Contains(t, items, key)
	assert.Equal(t, false, items[key].Value)
	assert.Equal(t, "traeger", items[key].Source)
	assert.True(t, items[key].Writable)
	assert.NotContains(t, items, records.KeyMFAMode, "school-only settings are no Träger settings")

	module, err := owners.NewSettingsTestModule(testpkg.NewDatabase(t, db), unit)
	require.NoError(t, err)
	ctx := testpkg.WithPackageTenantRuntime(context.Background())
	ownValue, err := module.Settings.ResolveBoolForTenant(ctx, ownSchool, key)
	require.NoError(t, err)
	assert.False(t, ownValue, "the school of the Träger resolves its value")
	foreignValue, err := module.Settings.ResolveBoolForTenant(ctx, foreignSchool, key)
	require.NoError(t, err)
	assert.True(t, foreignValue, "the school of another Träger keeps the registry default")

	rec = traegerSettingsRequest(t, router, login.AccessToken, http.MethodPut, "/traeger/settings/values/"+records.KeyMFAMode, map[string]any{"value": "off"})
	assert.Equal(t, http.StatusNotFound, rec.Code, "a school-only setting has no Träger value")
	rec = traegerSettingsRequest(t, router, login.AccessToken, http.MethodPut, "/traeger/settings/values/"+key, map[string]any{"value": "nein"})
	assert.Equal(t, http.StatusBadRequest, rec.Code, "an invalid value is rejected")

	rec = traegerSettingsRequest(t, router, login.AccessToken, http.MethodDelete, "/traeger/settings/values/"+key, nil)
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	_, items = traegerSchemaItems(t, router, login.AccessToken)
	assert.Equal(t, "default", items[key].Source)
	assert.Equal(t, records.GetDefinition(key).Default, items[key].Value)
}

func TestTraegerSettingsNeedTheRight(t *testing.T) {
	t.Parallel()
	db, router, _, _ := newTraegerPortal(t)
	password := testpkg.NewTestPassword()
	organization := traegerOrganization(t, db, "Träger Rechte", true)
	reader := traegerAccount(t, db, password)
	testpkg.CreateTestTraegerMember(t, db, organization, reader.ID, identityaccess.TraegerRoleReadOnly)
	_, login := traegerLogin(t, router, reader.Email, password, 0)
	key := records.KeyParentNewsEnabled

	code, items := traegerSchemaItems(t, router, login.AccessToken)
	require.Equal(t, http.StatusOK, code, "every member with the overview right reads the Träger settings")
	require.Contains(t, items, key)
	assert.False(t, items[key].Writable)
	for _, method := range []string{http.MethodPut, http.MethodDelete} {
		rec := traegerSettingsRequest(t, router, login.AccessToken, method, "/traeger/settings/values/"+key, map[string]any{"value": false})
		assert.Equal(t, http.StatusForbidden, rec.Code, "%s without the settings right", method)
	}

	billing := traegerAccount(t, db, password)
	testpkg.CreateTestTraegerMember(t, db, organization, billing.ID, identityaccess.TraegerRoleBilling)
	_, billingLogin := traegerLogin(t, router, billing.Email, password, 0)
	rec := traegerSettingsRequest(t, router, billingLogin.AccessToken, http.MethodPut, "/traeger/settings/values/"+key, map[string]any{"value": false})
	assert.Equal(t, http.StatusForbidden, rec.Code, "the billing right does not write a setting that needs the settings right")

	none := traegerAccount(t, db, password)
	testpkg.CreateTestTraegerMember(t, db, organization, none.ID)
	_, noneLogin := traegerLogin(t, router, none.Email, password, 0)
	code, _ = traegerSchemaItems(t, router, noneLogin.AccessToken)
	assert.Equal(t, http.StatusForbidden, code, "a member without a role reads nothing")
	var stored int
	require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM config.traeger_setting_values WHERE organization_id = ?`, organization).Scan(context.Background(), &stored))
	assert.Zero(t, stored)
}
