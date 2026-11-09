package config

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/moto-nrw/project-phoenix/modules/settings/internal/defaults"
	config "github.com/moto-nrw/project-phoenix/modules/settings/records"
)

// The save side of the DATEV settings (#4694). The same case list drives
// TestDatevReadinessMatchesTheSettingsCases in the Elternabrechnung, so the
// registry never stores a value the export refuses (backend/testdata).
func TestDatevSettingsSaveMatchesTheExportCases(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile("../../../../testdata/datev-settings-cases.json")
	require.NoError(t, err)
	var list struct {
		Cases []struct {
			Key       string `json:"key"`
			Value     any    `json:"value"`
			Save      bool   `json:"save"`
			Export    string `json:"export"`
			DependsOn string `json:"depends_on"`
		} `json:"cases"`
	}
	require.NoError(t, json.Unmarshal(raw, &list))
	require.NotEmpty(t, list.Cases)
	keys := map[string]bool{}
	for _, c := range list.Cases {
		keys[c.Key] = true
		definition := config.GetDefinition(c.Key)
		require.NotNil(t, definition, c.Key)
		err := validateValue(definition, c.Value)
		assert.Equal(t, c.Save, err == nil, "%s = %#v: save %v, got %v", c.Key, c.Value, c.Save, err)
		if c.Save && c.Export == "invalid" {
			assert.NotEmpty(t, c.DependsOn, "%s = %#v is stored but refused by the export without a cross-field reason", c.Key, c.Value)
		}
	}
	for _, key := range []string{
		config.KeyParentBillingDatevConsultantNumber, config.KeyParentBillingDatevClientNumber,
		config.KeyParentBillingDatevAccountLength, config.KeyParentBillingDatevChartOfAccounts,
		config.KeyParentBillingDatevFiscalYearStart, config.KeyParentBillingDatevRevenueAccount,
		config.KeyParentBillingDatevLocked,
	} {
		assert.True(t, keys[key], "the case list covers %s", key)
	}
}
