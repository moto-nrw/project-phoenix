// Router-level tests of the check of the DATEV settings (#4694): the right
// "Abrechnung" reads it, it names the keys of missing or invalid settings of
// the session's Träger and never their values, and another Träger's
// settings do not count.
package api

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

const datevReadinessPath = "/traeger/datev-export/readiness"

type datevReadinessBody struct {
	Ready           bool     `json:"ready"`
	MissingSettings []string `json:"missing_settings"`
	InvalidSettings []string `json:"invalid_settings"`
}

// datevMember signs in one more person of the Träger with role.
func datevMember(t *testing.T, db *bun.DB, router *traegerPortalHandler, organization int64, role string) string {
	t.Helper()
	password := testpkg.NewTestPassword()
	account := traegerAccount(t, db, password)
	testpkg.CreateTestTraegerMember(t, db, organization, account.ID, role)
	code, login := traegerSignIn(t, router, account.Email, password, 0)
	require.Equal(t, http.StatusOK, code)
	return login.AccessToken
}

func datevReadiness(t *testing.T, router http.Handler, token string) datevReadinessBody {
	t.Helper()
	rec := traegerJSON(t, router, token, http.MethodGet, datevReadinessPath, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.NotContains(t, rec.Body.String(), "29098", "the answer names keys, never values")
	return decodeSchoolPortalData[datevReadinessBody](t, rec.Body.Bytes())
}

func TestDatevReadinessNamesTheSettingsOfTheSessionsTraeger(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	router, _ := traegerPortalOn(t, db, testpkg.TenantRuntime(t, db))
	own := newTraegerOffice(t, db, router, "DATEV Träger", identityaccess.TraegerRoleAdministration)
	neighbour := newTraegerOffice(t, db, router, "DATEV Nachbar", identityaccess.TraegerRoleBilling)
	billing := datevMember(t, db, router, own.organization, identityaccess.TraegerRoleBilling)

	before := datevReadiness(t, router, billing)
	assert.False(t, before.Ready)
	assert.Equal(t, []string{
		configModels.KeyParentBillingDatevAccountLength, configModels.KeyParentBillingDatevClientNumber,
		configModels.KeyParentBillingDatevConsultantNumber, configModels.KeyParentBillingDatevRevenueAccount,
	}, before.MissingSettings, "the registry leaves the values of the customer empty")
	assert.Empty(t, before.InvalidSettings)

	for key, value := range map[string]string{
		configModels.KeyParentBillingDatevConsultantNumber: "29098",
		configModels.KeyParentBillingDatevClientNumber:     "55003",
		configModels.KeyParentBillingDatevAccountLength:    "4",
		configModels.KeyParentBillingDatevChartOfAccounts:  "04",
		configModels.KeyParentBillingDatevRevenueAccount:   "44000",
	} {
		rec := traegerJSON(t, router, own.token, http.MethodPut, "/traeger/settings/values/"+key, map[string]any{"value": value})
		require.Equal(t, http.StatusOK, rec.Code, "%s: %s", key, rec.Body.String())
	}
	invalid := datevReadiness(t, router, billing)
	assert.False(t, invalid.Ready)
	assert.Empty(t, invalid.MissingSettings)
	assert.Equal(t, []string{configModels.KeyParentBillingDatevRevenueAccount}, invalid.InvalidSettings,
		"a revenue account with five digits does not fit accounts of four")

	rec := traegerJSON(t, router, own.token, http.MethodPut, "/traeger/settings/values/"+configModels.KeyParentBillingDatevRevenueAccount,
		map[string]any{"value": "4400"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	ready := datevReadiness(t, router, billing)
	assert.True(t, ready.Ready)
	assert.Empty(t, ready.MissingSettings)
	assert.Empty(t, ready.InvalidSettings)

	foreign := datevReadiness(t, router, neighbour.token)
	assert.False(t, foreign.Ready, "the settings of another Träger do not count")
	assert.Len(t, foreign.MissingSettings, 4)

	rec = traegerJSON(t, router, own.token, http.MethodGet, datevReadinessPath, nil)
	assert.Equal(t, http.StatusForbidden, rec.Code, "the check needs the right Abrechnung: %s", rec.Body.String())
}
